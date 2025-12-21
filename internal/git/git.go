package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/blairham/ghorg/internal/scm"
	"github.com/davecgh/go-spew/spew"
)

// Gitter defines the interface for git operations.
// All git operations used by ghorg should be defined here to enable
// testing with mock implementations.
type Gitter interface {
	// Core cloning and syncing operations
	Clone(scm.Repo) error
	Reset(scm.Repo) error
	Pull(scm.Repo) error
	SetOrigin(scm.Repo) error
	SetOriginWithCredentials(scm.Repo) error
	Clean(scm.Repo) error
	Checkout(scm.Repo) error

	// Remote operations
	UpdateRemote(scm.Repo) error
	FetchAll(scm.Repo) error
	FetchCloneBranch(scm.Repo) error
	HasRemoteHeads(scm.Repo) (bool, error)
	GetRemoteURL(scm.Repo, string) (string, error)

	// Branch and status operations
	Branch(scm.Repo) (string, error)
	GetCurrentBranch(scm.Repo) (string, error)
	ShortStatus(scm.Repo) (string, error)
	HasLocalChanges(scm.Repo) (bool, error)
	HasUnpushedCommits(scm.Repo) (bool, error)

	// Commit comparison operations
	RevListCompare(scm.Repo, string, string) (string, error)
	RepoCommitCount(scm.Repo) (int, error)
	HasCommitsNotOnDefaultBranch(scm.Repo, string) (bool, error)
	IsDefaultBranchBehindHead(scm.Repo, string) (bool, error)

	// Sync and merge operations
	SyncDefaultBranch(scm.Repo) (bool, error)
	MergeIntoDefaultBranch(scm.Repo, string) error
	UpdateRef(scm.Repo, string, string) error
}

// Environment variable names used for git configuration
const (
	envDebug             = "GHORG_DEBUG"
	envCloneDepth        = "GHORG_CLONE_DEPTH"
	envIncludeSubmodules = "GHORG_INCLUDE_SUBMODULES"
	envGitFilter         = "GHORG_GIT_FILTER"
	envBackup            = "GHORG_BACKUP"
	envOutputDir         = "GHORG_OUTPUT_DIR"
	envAbsolutePathTo    = "GHORG_ABSOLUTE_PATH_TO_CLONE_TO"
	envGitBackend        = "GHORG_GIT_BACKEND"
)

// Git backend types
const (
	BackendExec   = "exec"
	BackendGolang = "golang"
)

// isDebugMode returns true if debug mode is enabled
func isDebugMode() bool {
	return os.Getenv(envDebug) != ""
}

// getCloneDepth returns the configured clone depth, or empty string if not set
func getCloneDepth() string {
	return os.Getenv(envCloneDepth)
}

// includeSubmodules returns true if submodules should be included
func includeSubmodules() bool {
	return os.Getenv(envIncludeSubmodules) == "true"
}

// getGitFilter returns the configured git filter, or empty string if not set
func getGitFilter() string {
	return os.Getenv(envGitFilter)
}

// isBackupMode returns true if backup mode is enabled
func isBackupMode() bool {
	return os.Getenv(envBackup) == "true"
}

// insertArg inserts an argument at the specified index in the args slice.
// This is used to add optional flags like --depth or --recursive at specific positions.
func insertArg(args []string, index int, arg string) []string {
	args = append(args[:index+1], args[index:]...)
	args[index] = arg
	return args
}

// runGitCommand executes a git command and returns any error.
// If debug mode is enabled, it will print debug information including any error.
func runGitCommand(cmd *exec.Cmd, repo scm.Repo) error {
	if isDebugMode() {
		return printDebugCmd(cmd, repo)
	}
	return cmd.Run()
}

// runGitCommandWithOutput executes a git command and returns its output.
// If debug mode is enabled, it will print debug information including any error.
func runGitCommandWithOutput(cmd *exec.Cmd, repo scm.Repo) (string, error) {
	if isDebugMode() {
		return printDebugCmdWithOutput(cmd, repo)
	}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GitClient implements the Gitter interface for executing git commands via exec.
type GitClient struct{}

// NewGit creates a new Gitter instance based on the GHORG_GIT_BACKEND environment variable.
// Supported backends: "golang" (default) uses pure Go implementation, "exec" uses system git binary.
func NewGit() Gitter {
	backend := os.Getenv(envGitBackend)
	switch backend {
	case BackendExec:
		return GitClient{}
	default:
		return GoGitClient()
	}
}

// NewExecGit creates a GitClient that uses exec to run git commands.
// This is useful when you specifically need the exec-based implementation.
func NewExecGit() GitClient {
	return GitClient{}
}

func printDebugCmd(cmd *exec.Cmd, repo scm.Repo) error {
	fmt.Println("------------- GIT DEBUG -------------")
	fmt.Printf("%s=%v\n", envOutputDir, os.Getenv(envOutputDir))
	fmt.Printf("%s=%v\n", envAbsolutePathTo, os.Getenv(envAbsolutePathTo))
	fmt.Print("Repo Data: ")
	spew.Dump(repo)
	fmt.Print("Command Ran: ")
	spew.Dump(*cmd)
	fmt.Println("")
	output, err := cmd.CombinedOutput()
	fmt.Printf("Command Output: %s\n", string(output))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	return err
}

// printDebugCmdWithOutput executes a command with debug output and returns the output string.
func printDebugCmdWithOutput(cmd *exec.Cmd, repo scm.Repo) (string, error) {
	fmt.Println("------------- GIT DEBUG -------------")
	fmt.Printf("%s=%v\n", envOutputDir, os.Getenv(envOutputDir))
	fmt.Printf("%s=%v\n", envAbsolutePathTo, os.Getenv(envAbsolutePathTo))
	fmt.Print("Repo Data: ")
	spew.Dump(repo)
	fmt.Print("Command Ran: ")
	spew.Dump(*cmd)
	fmt.Println("")
	output, err := cmd.CombinedOutput()
	fmt.Printf("Command Output: %s\n", string(output))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HasRemoteHeads checks if the remote repository has any heads (branches).
// Returns false if the repository is empty.
func (g GitClient) HasRemoteHeads(repo scm.Repo) (bool, error) {
	cmd := exec.Command("git", "ls-remote", "--heads", "--quiet", "--exit-code")
	cmd.Dir = repo.HostPath

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return false, err
	}

	// Exit code 2 means the repository is empty
	if exitError.ExitCode() == 2 {
		return false, nil
	}
	return false, err
}

// Clone clones a repository to the specified path.
// Respects configuration for submodules, depth, filters, and backup mode.
func (g GitClient) Clone(repo scm.Repo) error {
	args := []string{"clone", repo.CloneURL, repo.HostPath}

	if includeSubmodules() {
		args = insertArg(args, 1, "--recursive")
	}

	if depth := getCloneDepth(); depth != "" {
		args = insertArg(args, 1, fmt.Sprintf("--depth=%s", depth))
	}

	if filter := getGitFilter(); filter != "" {
		args = insertArg(args, 1, fmt.Sprintf("--filter=%s", filter))
	}

	if isBackupMode() {
		args = append(args, "--mirror")
	}

	cmd := exec.Command("git", args...)
	return runGitCommand(cmd, repo)
}

// SetOriginWithCredentials sets the origin remote URL using the clone URL (which may include credentials).
func (g GitClient) SetOriginWithCredentials(repo scm.Repo) error {
	cmd := exec.Command("git", "remote", "set-url", "origin", repo.CloneURL)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// SetOrigin sets the origin remote URL to the repository's base URL.
func (g GitClient) SetOrigin(repo scm.Repo) error {
	cmd := exec.Command("git", "remote", "set-url", "origin", repo.URL)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// Checkout checks out the specified branch in the repository.
func (g GitClient) Checkout(repo scm.Repo) error {
	cmd := exec.Command("git", "checkout", repo.CloneBranch)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// Clean removes untracked files and directories from the working tree.
func (g GitClient) Clean(repo scm.Repo) error {
	cmd := exec.Command("git", "clean", "-f", "-d")
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// UpdateRemote fetches updates from all remotes.
func (g GitClient) UpdateRemote(repo scm.Repo) error {
	cmd := exec.Command("git", "remote", "update")
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// Pull pulls the latest changes from the origin for the specified branch.
// Respects configuration for submodules and depth.
func (g GitClient) Pull(repo scm.Repo) error {
	args := []string{"pull", "origin", repo.CloneBranch}

	if includeSubmodules() {
		args = insertArg(args, 1, "--recurse-submodules")
	}

	if depth := getCloneDepth(); depth != "" {
		args = insertArg(args, 1, fmt.Sprintf("--depth=%s", depth))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// Reset performs a hard reset to the origin branch.
func (g GitClient) Reset(repo scm.Repo) error {
	cmd := exec.Command("git", "reset", "--hard", "origin/"+repo.CloneBranch)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// FetchAll fetches from all remotes.
// Respects configuration for depth.
func (g GitClient) FetchAll(repo scm.Repo) error {
	args := []string{"fetch", "--all"}

	if depth := getCloneDepth(); depth != "" {
		args = insertArg(args, 1, fmt.Sprintf("--depth=%s", depth))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// Branch returns the list of branches in the repository.
func (g GitClient) Branch(repo scm.Repo) (string, error) {
	cmd := exec.Command("git", "branch")
	cmd.Dir = repo.HostPath
	return runGitCommandWithOutput(cmd, repo)
}

// RevListCompare returns the list of commits in the local branch that are not in the remote branch.
func (g GitClient) RevListCompare(repo scm.Repo, localBranch string, remoteBranch string) (string, error) {
	cmd := exec.Command("git", "-C", repo.HostPath, "rev-list", localBranch, "^"+remoteBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// FetchCloneBranch fetches the specified branch from origin.
// Respects configuration for depth.
func (g GitClient) FetchCloneBranch(repo scm.Repo) error {
	args := []string{"fetch", "origin", repo.CloneBranch}

	if depth := getCloneDepth(); depth != "" {
		args = insertArg(args, 1, fmt.Sprintf("--depth=%s", depth))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// ShortStatus returns the short status of the repository.
func (g GitClient) ShortStatus(repo scm.Repo) (string, error) {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = repo.HostPath
	return runGitCommandWithOutput(cmd, repo)
}

// RepoCommitCount returns the number of commits in the specified branch.
func (g GitClient) RepoCommitCount(repo scm.Repo) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", repo.CloneBranch, "--")
	cmd.Dir = repo.HostPath

	output, err := runGitCommandWithOutput(cmd, repo)
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetRemoteDefaultBranch returns the default branch name from the remote (e.g., "main" or "master").
func (g GitClient) GetRemoteDefaultBranch(repo scm.Repo) (string, error) {
	// Try symbolic-ref first (fast, doesn't require network)
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repo.HostPath

	output, err := runGitCommandWithOutput(cmd, repo)
	if err == nil {
		// Output will be like "refs/remotes/origin/main", extract just "main"
		parts := strings.Split(strings.TrimSpace(output), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback to git ls-remote (works with local and remote repos)
	cmd = exec.Command("git", "ls-remote", "--symref", "origin", "HEAD")
	cmd.Dir = repo.HostPath

	output, err = runGitCommandWithOutput(cmd, repo)
	if err != nil {
		return "", err
	}

	// Parse output for "ref: refs/heads/<branch-name> HEAD"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ref:") && strings.Contains(trimmed, "refs/heads/") {
			// Extract branch name between "refs/heads/" and whitespace
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				refPath := parts[1]
				if strings.HasPrefix(refPath, "refs/heads/") {
					return strings.TrimPrefix(refPath, "refs/heads/"), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine default branch from ls-remote output")
}

// GetRemoteURL returns the URL for the given remote name (e.g., "origin").
func (g GitClient) GetRemoteURL(repo scm.Repo, remote string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", remote)
	cmd.Dir = repo.HostPath
	return runGitCommandWithOutput(cmd, repo)
}

// HasLocalChanges returns true if there are uncommitted changes in the working tree.
func (g GitClient) HasLocalChanges(repo scm.Repo) (bool, error) {
	status, err := g.ShortStatus(repo)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// HasUnpushedCommits returns true if there are commits present locally that are not pushed to upstream.
func (g GitClient) HasUnpushedCommits(repo scm.Repo) (bool, error) {
	cmd := exec.Command("git", "rev-list", "--count", "@{u}..HEAD")
	cmd.Dir = repo.HostPath

	output, err := runGitCommandWithOutput(cmd, repo)
	if err != nil {
		return false, err
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse unpushed commit count: %w", err)
	}
	return count > 0, nil
}

// GetCurrentBranch returns the currently checked-out branch name.
func (g GitClient) GetCurrentBranch(repo scm.Repo) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo.HostPath
	return runGitCommandWithOutput(cmd, repo)
}

// GetRefHash returns the commit hash for the given ref.
func (g GitClient) GetRefHash(repo scm.Repo, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repo.HostPath
	return runGitCommandWithOutput(cmd, repo)
}

// HasCommitsNotOnDefaultBranch returns true if currentBranch contains commits not present on the default branch.
func (g GitClient) HasCommitsNotOnDefaultBranch(repo scm.Repo, currentBranch string) (bool, error) {
	cmd := exec.Command("git", "rev-list", "--count", currentBranch, "^refs/heads/"+repo.CloneBranch)
	cmd.Dir = repo.HostPath

	output, err := runGitCommandWithOutput(cmd, repo)
	if err != nil {
		return false, err
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count > 0, nil
}

// IsDefaultBranchBehindHead returns true if the default branch is an ancestor of the current branch (i.e., can be fast-forwarded).
func (g GitClient) IsDefaultBranchBehindHead(repo scm.Repo, currentBranch string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", "refs/heads/"+repo.CloneBranch, currentBranch)
	cmd.Dir = repo.HostPath

	var err error
	if isDebugMode() {
		err = printDebugCmd(cmd, repo)
	} else {
		err = cmd.Run()
	}

	if err == nil {
		return true, nil
	}
	return handleMergeBaseError(err)
}

// handleMergeBaseError handles the exit codes from git merge-base --is-ancestor.
// Exit code 1 means "not an ancestor" which is a valid result, not an error.
func handleMergeBaseError(err error) (bool, error) {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// MergeIntoDefaultBranch attempts a fast-forward merge of currentBranch into the default branch locally.
func (g GitClient) MergeIntoDefaultBranch(repo scm.Repo, currentBranch string) error {
	// Checkout default branch
	checkoutCmd := exec.Command("git", "checkout", repo.CloneBranch)
	checkoutCmd.Dir = repo.HostPath
	if err := runGitCommand(checkoutCmd, repo); err != nil {
		return fmt.Errorf("failed to checkout default branch: %w", err)
	}

	// Merge with --ff-only
	mergeCmd := exec.Command("git", "merge", "--ff-only", currentBranch)
	mergeCmd.Dir = repo.HostPath
	return runGitCommand(mergeCmd, repo)
}

// MergeFastForward merges the remote branch into the current branch using fast-forward only.
// This is used during sync to update the local branch with remote changes.
func (g GitClient) MergeFastForward(repo scm.Repo) error {
	remoteBranch := fmt.Sprintf("origin/%s", repo.CloneBranch)
	cmd := exec.Command("git", "merge", "--ff-only", remoteBranch)
	cmd.Dir = repo.HostPath
	return runGitCommand(cmd, repo)
}

// UpdateRef updates a local ref to point to the given remote ref (by resolving the remote ref SHA first).
func (g GitClient) UpdateRef(repo scm.Repo, refName string, commitRef string) error {
	// Resolve commitRef to SHA
	revCmd := exec.Command("git", "rev-parse", commitRef)
	revCmd.Dir = repo.HostPath

	sha, err := runGitCommandWithOutput(revCmd, repo)
	if err != nil {
		return fmt.Errorf("failed to resolve ref %s: %w", commitRef, err)
	}

	// Update the ref
	updCmd := exec.Command("git", "update-ref", refName, sha)
	updCmd.Dir = repo.HostPath
	return runGitCommand(updCmd, repo)
}
