package git

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/scm"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// goGitClient implements the Gitter interface using the go-git library (pure Go).
type goGitClient struct{}

// GoGitClient creates a new goGitClient instance.
func GoGitClient() goGitClient {
	return goGitClient{}
}

// getAuth returns the appropriate authentication method based on the clone URL.
func (g goGitClient) getAuth(cloneURL string) transport.AuthMethod {
	// Check if it's an SSH URL
	if strings.HasPrefix(cloneURL, "git@") || strings.Contains(cloneURL, "ssh://") {
		// Try to use SSH agent or default SSH key
		auth, err := ssh.NewSSHAgentAuth("git")
		if err == nil {
			return auth
		}
		// If SSH agent fails, try default key locations
		homeDir, _ := os.UserHomeDir()
		keyPath := homeDir + "/.ssh/id_rsa"
		if _, err := os.Stat(keyPath); err == nil {
			auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
			if err == nil {
				return auth
			}
		}
		// Try ed25519 key
		keyPath = homeDir + "/.ssh/id_ed25519"
		if _, err := os.Stat(keyPath); err == nil {
			auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
			if err == nil {
				return auth
			}
		}
		return nil
	}

	// For HTTPS, check for token in URL or environment
	// The CloneURL may already contain credentials embedded
	return nil
}

// getHTTPAuth returns HTTP basic auth if credentials are available.
func (g goGitClient) getHTTPAuth() *http.BasicAuth {
	// Check for various token environment variables
	tokens := []string{
		"GHORG_GITHUB_TOKEN",
		"GHORG_GITLAB_TOKEN",
		"GHORG_GITEA_TOKEN",
		"GHORG_BITBUCKET_APP_PASSWORD",
	}

	for _, tokenEnv := range tokens {
		if token := os.Getenv(tokenEnv); token != "" {
			username := "x-access-token"
			if tokenEnv == "GHORG_BITBUCKET_APP_PASSWORD" {
				username = os.Getenv("GHORG_BITBUCKET_USERNAME")
			}
			return &http.BasicAuth{
				Username: username,
				Password: token,
			}
		}
	}
	return nil
}

// debugLog prints debug information if debug mode is enabled.
func (g goGitClient) debugLog(operation string, repo scm.Repo, details ...string) {
	if !isDebugMode() {
		return
	}
	fmt.Println("------------- GO-GIT DEBUG -------------")
	fmt.Printf("%s=%v\n", envOutputDir, os.Getenv(envOutputDir))
	fmt.Printf("%s=%v\n", envAbsolutePathTo, os.Getenv(envAbsolutePathTo))
	fmt.Printf("Operation: %s\n", operation)
	fmt.Printf("Repo: %s (Path: %s)\n", repo.Name, repo.HostPath)
	for _, detail := range details {
		fmt.Printf("  %s\n", detail)
	}
	fmt.Println("")
}

// HasRemoteHeads checks if the remote repository has any heads (branches).
// Returns false if the repository is empty.
func (g goGitClient) HasRemoteHeads(repo scm.Repo) (bool, error) {
	g.debugLog("HasRemoteHeads", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return false, err
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return false, err
	}

	refs, err := remote.List(&gogit.ListOptions{
		Auth: g.getAuth(repo.CloneURL),
	})
	if err != nil {
		return false, err
	}

	for _, ref := range refs {
		if ref.Name().IsBranch() {
			return true, nil
		}
	}

	return false, nil
}

// Clone clones a repository to the specified path.
// Respects configuration for submodules, depth, and backup mode.
// Note: git filter is not fully supported by go-git.
func (g goGitClient) Clone(repo scm.Repo) error {
	g.debugLog("Clone", repo, fmt.Sprintf("URL: %s", repo.CloneURL))

	cloneOpts := &gogit.CloneOptions{
		URL:      repo.CloneURL,
		Progress: nil, // Set to os.Stdout for verbose output
	}

	// Set authentication
	if auth := g.getAuth(repo.CloneURL); auth != nil {
		cloneOpts.Auth = auth
	} else if httpAuth := g.getHTTPAuth(); httpAuth != nil {
		cloneOpts.Auth = httpAuth
	}

	// Handle submodules
	if includeSubmodules() {
		cloneOpts.RecurseSubmodules = gogit.DefaultSubmoduleRecursionDepth
	}

	// Handle depth
	if depth := getCloneDepth(); depth != "" {
		d, err := strconv.Atoi(depth)
		if err == nil && d > 0 {
			cloneOpts.Depth = d
		}
	}

	// Handle backup/mirror mode
	if isBackupMode() {
		cloneOpts.Mirror = true
	}

	// Note: git filter (--filter=blob:none) is not supported by go-git
	// If git filter is set, log a warning
	if filter := getGitFilter(); filter != "" {
		colorlog.PrintInfo(fmt.Sprintf("Warning: git filter '%s' is not supported by go-git backend, ignoring\n", filter))
	}

	_, err := gogit.PlainClone(repo.HostPath, false, cloneOpts)
	return err
}

// SetOriginWithCredentials sets the origin remote URL using the clone URL (which may include credentials).
func (g goGitClient) SetOriginWithCredentials(repo scm.Repo) error {
	g.debugLog("SetOriginWithCredentials", repo, fmt.Sprintf("URL: %s", repo.CloneURL))
	return g.setRemoteURL(repo, "origin", repo.CloneURL)
}

// SetOrigin sets the origin remote URL to the repository's base URL.
func (g goGitClient) SetOrigin(repo scm.Repo) error {
	g.debugLog("SetOrigin", repo, fmt.Sprintf("URL: %s", repo.URL))
	return g.setRemoteURL(repo, "origin", repo.URL)
}

// setRemoteURL is a helper to set the URL of a remote.
func (g goGitClient) setRemoteURL(repo scm.Repo, remoteName, url string) error {
	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	cfg, err := r.Config()
	if err != nil {
		return err
	}

	if remote, ok := cfg.Remotes[remoteName]; ok {
		remote.URLs = []string{url}
	} else {
		cfg.Remotes[remoteName] = &config.RemoteConfig{
			Name: remoteName,
			URLs: []string{url},
		}
	}

	return r.SetConfig(cfg)
}

// Checkout checks out the specified branch in the repository.
func (g goGitClient) Checkout(repo scm.Repo) error {
	g.debugLog("Checkout", repo, fmt.Sprintf("Branch: %s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Try to checkout the branch
	err = w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(repo.CloneBranch),
	})
	if err != nil {
		// If branch doesn't exist locally, try to create it from remote
		err = w.Checkout(&gogit.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(repo.CloneBranch),
			Create: true,
		})
	}

	return err
}

// Clean removes untracked files and directories from the working tree.
func (g goGitClient) Clean(repo scm.Repo) error {
	g.debugLog("Clean", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	return w.Clean(&gogit.CleanOptions{
		Dir: true,
	})
}

// UpdateRemote fetches updates from all remotes.
func (g goGitClient) UpdateRemote(repo scm.Repo) error {
	g.debugLog("UpdateRemote", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	remotes, err := r.Remotes()
	if err != nil {
		return err
	}

	for _, remote := range remotes {
		fetchOpts := &gogit.FetchOptions{
			RemoteName: remote.Config().Name,
		}
		if auth := g.getAuth(repo.CloneURL); auth != nil {
			fetchOpts.Auth = auth
		} else if httpAuth := g.getHTTPAuth(); httpAuth != nil {
			fetchOpts.Auth = httpAuth
		}

		err := remote.Fetch(fetchOpts)
		if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return err
		}
	}

	return nil
}

// Pull pulls the latest changes from the origin for the specified branch.
// Respects configuration for submodules and depth.
func (g goGitClient) Pull(repo scm.Repo) error {
	g.debugLog("Pull", repo, fmt.Sprintf("Branch: %s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	pullOpts := &gogit.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(repo.CloneBranch),
	}

	// Set authentication
	if auth := g.getAuth(repo.CloneURL); auth != nil {
		pullOpts.Auth = auth
	} else if httpAuth := g.getHTTPAuth(); httpAuth != nil {
		pullOpts.Auth = httpAuth
	}

	// Handle submodules
	if includeSubmodules() {
		pullOpts.RecurseSubmodules = gogit.DefaultSubmoduleRecursionDepth
	}

	// Handle depth
	if depth := getCloneDepth(); depth != "" {
		depthInt, convErr := strconv.Atoi(depth)
		if convErr == nil && depthInt > 0 {
			pullOpts.Depth = depthInt
		}
	}

	err = w.Pull(pullOpts)
	if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// Reset performs a hard reset to the origin branch.
func (g goGitClient) Reset(repo scm.Repo) error {
	g.debugLog("Reset", repo, fmt.Sprintf("Target: origin/%s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	// Get the remote branch reference
	refName := plumbing.NewRemoteReferenceName("origin", repo.CloneBranch)
	ref, err := r.Reference(refName, true)
	if err != nil {
		return fmt.Errorf("failed to find remote branch origin/%s: %w", repo.CloneBranch, err)
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	return w.Reset(&gogit.ResetOptions{
		Commit: ref.Hash(),
		Mode:   gogit.HardReset,
	})
}

// FetchAll fetches from all remotes.
// Respects configuration for depth.
func (g goGitClient) FetchAll(repo scm.Repo) error {
	g.debugLog("FetchAll", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	fetchOpts := &gogit.FetchOptions{
		RefSpecs: []config.RefSpec{"+refs/heads/*:refs/remotes/origin/*"},
	}

	// Set authentication
	if auth := g.getAuth(repo.CloneURL); auth != nil {
		fetchOpts.Auth = auth
	} else if httpAuth := g.getHTTPAuth(); httpAuth != nil {
		fetchOpts.Auth = httpAuth
	}

	// Handle depth
	if depth := getCloneDepth(); depth != "" {
		depthInt, convErr := strconv.Atoi(depth)
		if convErr == nil && depthInt > 0 {
			fetchOpts.Depth = depthInt
		}
	}

	err = r.Fetch(fetchOpts)
	if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// Branch returns the list of branches in the repository.
func (g goGitClient) Branch(repo scm.Repo) (string, error) {
	g.debugLog("Branch", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	branches, err := r.Branches()
	if err != nil {
		return "", err
	}

	var result strings.Builder
	head, _ := r.Head()
	headBranch := ""
	if head != nil {
		headBranch = head.Name().Short()
	}

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		if branchName == headBranch {
			result.WriteString("* ")
		} else {
			result.WriteString("  ")
		}
		result.WriteString(branchName)
		result.WriteString("\n")
		return nil
	})

	return strings.TrimSpace(result.String()), err
}

// RevListCompare returns the list of commits in the local branch that are not in the remote branch.
func (g goGitClient) RevListCompare(repo scm.Repo, localBranch string, remoteBranch string) (string, error) {
	g.debugLog("RevListCompare", repo, fmt.Sprintf("Local: %s, Remote: %s", localBranch, remoteBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	// Get local branch reference
	localRef, err := r.Reference(plumbing.NewBranchReferenceName(localBranch), true)
	if err != nil {
		return "", err
	}

	// Get remote branch reference
	remoteRef, err := r.Reference(plumbing.ReferenceName(remoteBranch), true)
	if err != nil {
		return "", err
	}

	// Get commits reachable from local but not from remote
	localCommit, err := r.CommitObject(localRef.Hash())
	if err != nil {
		return "", err
	}

	remoteCommit, err := r.CommitObject(remoteRef.Hash())
	if err != nil {
		return "", err
	}

	// Walk local commits and check if they're ancestors of remote
	var result strings.Builder
	iter := object.NewCommitIterCTime(localCommit, nil, nil)
	defer iter.Close()

	err = iter.ForEach(func(c *object.Commit) error {
		// Check if this commit is an ancestor of the remote branch
		isAncestor, ancestorErr := c.IsAncestor(remoteCommit)
		if ancestorErr != nil {
			return ancestorErr
		}
		if !isAncestor && c.Hash != remoteCommit.Hash {
			result.WriteString(c.Hash.String())
			result.WriteString("\n")
		}
		return nil
	})

	return strings.TrimSpace(result.String()), err
}

// FetchCloneBranch fetches the specified branch from origin.
// Respects configuration for depth.
func (g goGitClient) FetchCloneBranch(repo scm.Repo) error {
	g.debugLog("FetchCloneBranch", repo, fmt.Sprintf("Branch: %s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	fetchOpts := &gogit.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", repo.CloneBranch, repo.CloneBranch)),
		},
	}

	// Set authentication
	if auth := g.getAuth(repo.CloneURL); auth != nil {
		fetchOpts.Auth = auth
	} else if httpAuth := g.getHTTPAuth(); httpAuth != nil {
		fetchOpts.Auth = httpAuth
	}

	// Handle depth
	if depth := getCloneDepth(); depth != "" {
		depthInt, convErr := strconv.Atoi(depth)
		if convErr == nil && depthInt > 0 {
			fetchOpts.Depth = depthInt
		}
	}

	err = r.Fetch(fetchOpts)
	if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// ShortStatus returns the short status of the repository.
func (g goGitClient) ShortStatus(repo scm.Repo) (string, error) {
	g.debugLog("ShortStatus", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}

	status, err := w.Status()
	if err != nil {
		return "", err
	}

	// Format status similar to git status --short
	var result strings.Builder
	for file, s := range status {
		if s.Staging != gogit.Unmodified || s.Worktree != gogit.Unmodified {
			staging := statusCodeToChar(s.Staging)
			worktree := statusCodeToChar(s.Worktree)
			result.WriteString(fmt.Sprintf("%s%s %s\n", staging, worktree, file))
		}
	}

	return strings.TrimSpace(result.String()), nil
}

// statusCodeToChar converts a go-git status code to a single character.
func statusCodeToChar(code gogit.StatusCode) string {
	switch code {
	case gogit.Unmodified:
		return " "
	case gogit.Untracked:
		return "?"
	case gogit.Modified:
		return "M"
	case gogit.Added:
		return "A"
	case gogit.Deleted:
		return "D"
	case gogit.Renamed:
		return "R"
	case gogit.Copied:
		return "C"
	case gogit.UpdatedButUnmerged:
		return "U"
	default:
		return " "
	}
}

// RepoCommitCount returns the number of commits in the specified branch.
func (g goGitClient) RepoCommitCount(repo scm.Repo) (int, error) {
	g.debugLog("RepoCommitCount", repo, fmt.Sprintf("Branch: %s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return 0, err
	}

	ref, err := r.Reference(plumbing.NewBranchReferenceName(repo.CloneBranch), true)
	if err != nil {
		return 0, err
	}

	iter, err := r.Log(&gogit.LogOptions{
		From: ref.Hash(),
	})
	if err != nil {
		return 0, err
	}

	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})

	return count, err
}

// GetRemoteURL returns the URL for the given remote name (e.g., "origin").
func (g goGitClient) GetRemoteURL(repo scm.Repo, remote string) (string, error) {
	g.debugLog("GetRemoteURL", repo, fmt.Sprintf("Remote: %s", remote))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	rem, err := r.Remote(remote)
	if err != nil {
		return "", err
	}

	cfg := rem.Config()
	if len(cfg.URLs) == 0 {
		return "", fmt.Errorf("no URLs configured for remote %s", remote)
	}

	return cfg.URLs[0], nil
}

// GetRemoteDefaultBranch returns the default branch name for the remote repository
func (g goGitClient) GetRemoteDefaultBranch(repo scm.Repo) (string, error) {
	g.debugLog("GetRemoteDefaultBranch", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	// Try to get the symbolic reference for origin/HEAD
	headRef, err := r.Reference(plumbing.NewRemoteHEADReferenceName("origin"), true)
	if err == nil && headRef.Type() == plumbing.SymbolicReference {
		// Extract branch name from refs/remotes/origin/<branch>
		target := headRef.Target().Short()
		// The target will be like "origin/main", extract just "main"
		parts := strings.Split(target, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: query the remote directly using go-git
	rem, err := r.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("failed to get origin remote: %w", err)
	}

	// List remote references
	refs, err := rem.List(&gogit.ListOptions{
		Auth: g.getAuth(repo.URL),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list remote references: %w", err)
	}

	// Look for the HEAD symbolic reference
	for _, ref := range refs {
		if ref.Type() == plumbing.SymbolicReference && ref.Name() == plumbing.HEAD {
			// The target will be like "refs/heads/main"
			target := ref.Target().String()
			if strings.HasPrefix(target, "refs/heads/") {
				return strings.TrimPrefix(target, "refs/heads/"), nil
			}
		}
	}

	return "", fmt.Errorf("could not determine default branch from remote")
}

// HasLocalChanges returns true if there are uncommitted changes in the working tree.
func (g goGitClient) HasLocalChanges(repo scm.Repo) (bool, error) {
	status, err := g.ShortStatus(repo)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// HasUnpushedCommits returns true if there are commits present locally that are not pushed to upstream.
func (g goGitClient) HasUnpushedCommits(repo scm.Repo) (bool, error) {
	g.debugLog("HasUnpushedCommits", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return false, err
	}

	// Get current HEAD
	head, err := r.Head()
	if err != nil {
		return false, err
	}

	// Get upstream reference
	branchName := head.Name().Short()
	upstreamRef := plumbing.NewRemoteReferenceName("origin", branchName)
	upstream, err := r.Reference(upstreamRef, true)
	if err != nil {
		// No upstream, could mean unpushed or just not set
		return false, nil //nolint:nilerr // No upstream tracking branch is not an error, just means no tracking
	}

	// Count commits between HEAD and upstream
	headCommit, err := r.CommitObject(head.Hash())
	if err != nil {
		return false, err
	}

	upstreamCommit, err := r.CommitObject(upstream.Hash())
	if err != nil {
		// Object not found or other error - can't determine, return error
		return false, fmt.Errorf("failed to get upstream commit object: %w", err)
	}

	// Check if HEAD is ahead of upstream
	isAncestor, err := upstreamCommit.IsAncestor(headCommit)
	if err != nil {
		return false, fmt.Errorf("failed to check ancestor relationship: %w", err)
	}

	// If upstream is ancestor of HEAD, there are unpushed commits
	return isAncestor && head.Hash() != upstream.Hash(), nil
}

// GetCurrentBranch returns the currently checked-out branch name.
func (g goGitClient) GetCurrentBranch(repo scm.Repo) (string, error) {
	g.debugLog("GetCurrentBranch", repo)

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	head, err := r.Head()
	if err != nil {
		return "", err
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}

	// Detached HEAD - return the hash
	return head.Hash().String()[:7], nil
}

// GetRefHash returns the commit hash for the given ref.
func (g goGitClient) GetRefHash(repo scm.Repo, ref string) (string, error) {
	g.debugLog("GetRefHash", repo, fmt.Sprintf("Ref: %s", ref))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return "", err
	}

	refObj, err := r.Reference(plumbing.ReferenceName(ref), true)
	if err != nil {
		return "", err
	}

	return refObj.Hash().String(), nil
}

// HasCommitsNotOnDefaultBranch returns true if currentBranch contains commits not present on the default branch.
func (g goGitClient) HasCommitsNotOnDefaultBranch(repo scm.Repo, currentBranch string) (bool, error) {
	g.debugLog("HasCommitsNotOnDefaultBranch", repo, fmt.Sprintf("Current: %s, Default: %s", currentBranch, repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return false, err
	}

	// Get current branch reference
	currentRef, err := r.Reference(plumbing.NewBranchReferenceName(currentBranch), true)
	if err != nil {
		return false, err
	}

	// Get default branch reference
	defaultRef, err := r.Reference(plumbing.NewBranchReferenceName(repo.CloneBranch), true)
	if err != nil {
		return false, err
	}

	if currentRef.Hash() == defaultRef.Hash() {
		return false, nil
	}

	// Get default branch commit
	defaultCommit, err := r.CommitObject(defaultRef.Hash())
	if err != nil {
		return false, err
	}

	// Get current branch commit
	currentCommit, err := r.CommitObject(currentRef.Hash())
	if err != nil {
		return false, err
	}

	// Check if current is ancestor of default - if so, no unique commits
	isAncestor, err := currentCommit.IsAncestor(defaultCommit)
	if err != nil {
		return false, err
	}

	return !isAncestor, nil
}

// IsDefaultBranchBehindHead returns true if the default branch is an ancestor of the current branch.
func (g goGitClient) IsDefaultBranchBehindHead(repo scm.Repo, currentBranch string) (bool, error) {
	g.debugLog("IsDefaultBranchBehindHead", repo, fmt.Sprintf("Current: %s, Default: %s", currentBranch, repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return false, err
	}

	// Get default branch reference
	defaultRef, err := r.Reference(plumbing.NewBranchReferenceName(repo.CloneBranch), true)
	if err != nil {
		return false, err
	}

	// Get current branch reference
	currentRef, err := r.Reference(plumbing.NewBranchReferenceName(currentBranch), true)
	if err != nil {
		return false, err
	}

	if defaultRef.Hash() == currentRef.Hash() {
		return false, nil
	}

	// Get commits
	defaultCommit, err := r.CommitObject(defaultRef.Hash())
	if err != nil {
		return false, err
	}

	currentCommit, err := r.CommitObject(currentRef.Hash())
	if err != nil {
		return false, err
	}

	// Check if default is ancestor of current
	return defaultCommit.IsAncestor(currentCommit)
}

// MergeIntoDefaultBranch attempts a fast-forward merge of currentBranch into the default branch locally.
func (g goGitClient) MergeIntoDefaultBranch(repo scm.Repo, currentBranch string) error {
	g.debugLog("MergeIntoDefaultBranch", repo, fmt.Sprintf("Source: %s, Target: %s", currentBranch, repo.CloneBranch))

	// First checkout the default branch
	if err := g.Checkout(repo); err != nil {
		return fmt.Errorf("failed to checkout default branch: %w", err)
	}

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	// Get the source branch commit
	sourceRef, err := r.Reference(plumbing.NewBranchReferenceName(currentBranch), true)
	if err != nil {
		return fmt.Errorf("failed to get source branch: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Fast-forward by resetting to the source branch commit
	return w.Reset(&gogit.ResetOptions{
		Commit: sourceRef.Hash(),
		Mode:   gogit.HardReset,
	})
}

// MergeFastForward merges the remote branch into the current branch using fast-forward only.
// This is used during sync to update the local branch with remote changes.
func (g goGitClient) MergeFastForward(repo scm.Repo) error {
	g.debugLog("MergeFastForward", repo, fmt.Sprintf("Target: origin/%s", repo.CloneBranch))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	// Get the remote branch reference
	remoteBranch := fmt.Sprintf("refs/remotes/origin/%s", repo.CloneBranch)
	remoteRef, err := r.Reference(plumbing.ReferenceName(remoteBranch), true)
	if err != nil {
		return fmt.Errorf("failed to get remote branch: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Fast-forward by resetting to the remote branch commit
	return w.Reset(&gogit.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   gogit.HardReset,
	})
}

// UpdateRef updates a local ref to point to the given remote ref.
func (g goGitClient) UpdateRef(repo scm.Repo, refName string, commitRef string) error {
	g.debugLog("UpdateRef", repo, fmt.Sprintf("Ref: %s, Target: %s", refName, commitRef))

	r, err := gogit.PlainOpen(repo.HostPath)
	if err != nil {
		return err
	}

	// Resolve the commit ref to a hash
	ref, err := r.Reference(plumbing.ReferenceName(commitRef), true)
	if err != nil {
		return fmt.Errorf("failed to resolve ref %s: %w", commitRef, err)
	}

	// Create or update the reference
	newRef := plumbing.NewHashReference(plumbing.ReferenceName(refName), ref.Hash())
	return r.Storer.SetReference(newRef)
}

// SyncDefaultBranch synchronizes the local default branch with the remote.
// Returns (wasUpdated, error) where wasUpdated indicates if the branch was actually changed
func (g goGitClient) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	// Check if sync is disabled via configuration
	syncEnabled := os.Getenv("GHORG_SYNC_DEFAULT_BRANCH")
	if syncEnabled != "true" {
		return false, nil
	}

	// First check if the remote exists and is accessible
	_, err := g.GetRemoteURL(repo, "origin")
	if err != nil {
		return false, nil //nolint:nilerr // Remote doesn't exist, nothing to sync
	}

	// Check if the working directory has any uncommitted changes
	hasWorkingDirChanges, err := g.HasLocalChanges(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to check working directory status for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return false, fmt.Errorf("failed to check working directory status: %w", err)
	}

	if hasWorkingDirChanges {
		return false, nil
	}

	// Check what branch we're currently on first
	currentBranch, err := g.GetCurrentBranch(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to get current branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get the actual default branch from the remote
	defaultBranch, err := g.GetRemoteDefaultBranch(repo)
	if err != nil {
		defaultBranch = repo.CloneBranch
		if defaultBranch == "" {
			m := fmt.Sprintf("Failed to determine default branch for %s: %v", repo.Name, err)
			colorlog.PrintError(m)
			return false, fmt.Errorf("failed to determine default branch: %w", err)
		}
	}

	// Only check for unpushed commits if we're on the default branch
	if currentBranch == defaultBranch {
		hasUnpushedCommits, unpushedErr := g.HasUnpushedCommits(repo)
		if unpushedErr != nil {
			// If we can't check for unpushed commits (e.g., no tracking branch set up),
			// skip the sync to be safe - we don't want to potentially lose commits
			return false, nil //nolint:nilerr // Cannot check unpushed commits, skip sync to be safe
		}

		if hasUnpushedCommits {
			return false, nil
		}
	}

	// Get the commit hash before sync to check if changes were made
	refName := fmt.Sprintf("refs/heads/%s", defaultBranch)
	beforeHash, err := g.GetRefHash(repo, refName)
	if err != nil {
		// Ref might not exist yet, that's okay
		beforeHash = ""
	}

	// Fetch the latest changes from the remote using the detected default branch
	originalCloneBranch := repo.CloneBranch
	repo.CloneBranch = defaultBranch
	err = g.FetchCloneBranch(repo)
	if err != nil {
		repo.CloneBranch = originalCloneBranch
		m := fmt.Sprintf("Failed to fetch default branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return false, fmt.Errorf("failed to fetch default branch: %w", err)
	}

	// If we're on the default branch, merge the remote changes
	if currentBranch == defaultBranch {
		err = g.MergeFastForward(repo)
		repo.CloneBranch = originalCloneBranch
		if err != nil {
			m := fmt.Sprintf("Failed to merge remote changes for %s: %v", repo.Name, err)
			colorlog.PrintError(m)
			return false, fmt.Errorf("failed to merge remote changes: %w", err)
		}
	} else {
		repo.CloneBranch = originalCloneBranch
		// If we're on a different branch, just update the default branch ref without checking it out
		commitRef := fmt.Sprintf("refs/remotes/origin/%s", defaultBranch)
		err = g.UpdateRef(repo, refName, commitRef)
		if err != nil {
			m := fmt.Sprintf("Failed to update branch reference for %s: %v", repo.Name, err)
			colorlog.PrintError(m)
			return false, fmt.Errorf("failed to update branch reference: %w", err)
		}
	}

	// Check if the hash changed
	afterHash, err := g.GetRefHash(repo, refName)
	if err != nil {
		// If we can't verify, assume it changed
		return true, nil //nolint:nilerr // Cannot verify hash, optimistically assume it changed
	}

	wasUpdated := beforeHash != afterHash
	return wasUpdated, nil
}
