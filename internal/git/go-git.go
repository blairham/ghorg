package git

import (
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
		if err != nil && err != gogit.NoErrAlreadyUpToDate {
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
		d, err := strconv.Atoi(depth)
		if err == nil && d > 0 {
			pullOpts.Depth = d
		}
	}

	err = w.Pull(pullOpts)
	if err == gogit.NoErrAlreadyUpToDate {
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
		d, err := strconv.Atoi(depth)
		if err == nil && d > 0 {
			fetchOpts.Depth = d
		}
	}

	err = r.Fetch(fetchOpts)
	if err == gogit.NoErrAlreadyUpToDate {
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
		isAncestor, err := c.IsAncestor(remoteCommit)
		if err != nil {
			return err
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
		d, err := strconv.Atoi(depth)
		if err == nil && d > 0 {
			fetchOpts.Depth = d
		}
	}

	err = r.Fetch(fetchOpts)
	if err == gogit.NoErrAlreadyUpToDate {
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
		return false, nil
	}

	// Count commits between HEAD and upstream
	headCommit, err := r.CommitObject(head.Hash())
	if err != nil {
		return false, err
	}

	upstreamCommit, err := r.CommitObject(upstream.Hash())
	if err != nil {
		return false, err
	}

	// Check if HEAD is ahead of upstream
	isAncestor, err := upstreamCommit.IsAncestor(headCommit)
	if err != nil {
		return false, err
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
// This implementation delegates to the shared sync logic.
func (g goGitClient) SyncDefaultBranch(repo scm.Repo) error {
	// Check if sync is disabled via configuration
	syncEnabled := os.Getenv("GHORG_SYNC_DEFAULT_BRANCH")
	if syncEnabled != "true" {
		m := fmt.Sprintf("Skipping sync for %s: GHORG_SYNC_DEFAULT_BRANCH is not set to true\n", repo.Name)
		colorlog.PrintInfo(m)
		return nil
	}

	// First check if the remote exists and is accessible
	_, err := g.GetRemoteURL(repo, "origin")
	if err != nil {
		return nil
	}

	// Check if the working directory has any uncommitted changes
	hasWorkingDirChanges, err := g.HasLocalChanges(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to check working directory status for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to check working directory status: %w", err)
	}

	if hasWorkingDirChanges {
		m := fmt.Sprintf("Skipping sync for %s: working directory has uncommitted changes\n", repo.Name)
		colorlog.PrintInfo(m)
		return nil
	}

	// Check if the current branch has unpushed commits
	hasUnpushedCommits, err := g.HasUnpushedCommits(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to check for unpushed commits for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to check for unpushed commits: %w", err)
	}

	if hasUnpushedCommits {
		m := fmt.Sprintf("Skipping sync for %s: branch has unpushed commits\n", repo.Name)
		colorlog.PrintInfo(m)
		return nil
	}

	// Check if we're on the correct branch
	currentBranch, err := g.GetCurrentBranch(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to get current branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if current branch has commits not on the default branch
	hasCommitsNotOnDefault, err := g.HasCommitsNotOnDefaultBranch(repo, currentBranch)
	if err != nil {
		m := fmt.Sprintf("Failed to check for commits not on default branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to check for commits not on default branch: %w", err)
	}

	// Check if the default branch is behind HEAD
	isDefaultBehindHead, err := g.IsDefaultBranchBehindHead(repo, currentBranch)
	if err != nil {
		m := fmt.Sprintf("Failed to check if default branch is behind HEAD for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to check if default branch is behind HEAD: %w", err)
	}

	if hasCommitsNotOnDefault && !isDefaultBehindHead {
		m := fmt.Sprintf("Skipping sync for %s: current branch has commits not on default branch and default is not behind\n", repo.Name)
		colorlog.PrintInfo(m)
		return nil
	}

	// Switch to the target branch if we're not already on it
	if currentBranch != repo.CloneBranch {
		err := g.Checkout(repo)
		if err != nil {
			m := fmt.Sprintf("Could not checkout %s for %s: %v", repo.CloneBranch, repo.Name, err)
			colorlog.PrintError(m)
			return nil
		}
	}

	// Fetch the latest changes from the remote
	err = g.FetchCloneBranch(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to fetch default branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to fetch default branch: %w", err)
	}

	// If the default branch is behind HEAD and we have commits to merge,
	// perform a fast-forward merge
	if isDefaultBehindHead && hasCommitsNotOnDefault {
		m := fmt.Sprintf("Default branch is behind HEAD for %s, performing fast-forward merge", repo.Name)
		colorlog.PrintInfo(m)

		err = g.MergeIntoDefaultBranch(repo, currentBranch)
		if err != nil {
			m := fmt.Sprintf("Failed to merge into default branch for %s: %v", repo.Name, err)
			colorlog.PrintError(m)
			return fmt.Errorf("failed to merge into default branch: %w", err)
		}

		m = fmt.Sprintf("Successfully updated default branch %s by merging %s for %s", repo.CloneBranch, currentBranch, repo.Name)
		colorlog.PrintSuccess(m)
		return nil
	}

	// Update the local branch reference to match the remote
	refName := fmt.Sprintf("refs/heads/%s", repo.CloneBranch)
	commitRef := fmt.Sprintf("refs/remotes/origin/%s", repo.CloneBranch)
	err = g.UpdateRef(repo, refName, commitRef)
	if err != nil {
		m := fmt.Sprintf("Failed to update branch reference for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to update branch reference: %w", err)
	}

	// Reset the working directory to match the updated branch
	err = g.Reset(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to reset working directory to remote branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return fmt.Errorf("failed to reset working directory to remote branch: %w", err)
	}

	m := fmt.Sprintf("Successfully updated default branch %s for %s", repo.CloneBranch, repo.Name)
	colorlog.PrintSuccess(m)

	return nil
}
