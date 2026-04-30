package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/git"
	"github.com/blairham/ghorg/internal/scm"
)

// Helper function to apply clone delay if configured
func applyCloneDelay(repoURL string) {
	delaySeconds, hasDelay := getCloneDelaySeconds()
	if !hasDelay {
		return
	}

	if os.Getenv("GHORG_DEBUG") != "" {
		colorlog.PrintInfo(fmt.Sprintf("Applying %d second delay before processing %s", delaySeconds, repoURL))
	}
	time.Sleep(time.Duration(delaySeconds) * time.Second)
}

// RepositoryProcessor handles the processing of individual repositories
type RepositoryProcessor struct {
	git            git.Gitter
	stats          *CloneStats
	state          *StateManifest
	mutex          *sync.RWMutex
	untouchedRepos []string
	protectedRepos []string
}

// CloneStats tracks statistics during clone operations
type CloneStats struct {
	CloneCount           int
	PulledCount          int
	SkippedCount         int
	ProtectedCount       int
	UpdateRemoteCount    int
	NewCommits           int
	UntouchedPrunes      int
	SyncedCount          int
	TotalDurationSeconds int
	CloneInfos           []string
	CloneErrors          []string
	CloneSkipped         []string
}

// NewRepositoryProcessor creates a new repository processor
func NewRepositoryProcessor(git git.Gitter) *RepositoryProcessor {
	return &RepositoryProcessor{
		git:   git,
		stats: &CloneStats{},
		mutex: &sync.RWMutex{},
	}
}

// SetState attaches a state manifest. Per-repo outcomes will be recorded as
// repositories are processed. Pass nil to disable state tracking (the default).
func (rp *RepositoryProcessor) SetState(state *StateManifest) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()
	rp.state = state
}

// State returns the attached state manifest, or nil if none.
func (rp *RepositoryProcessor) State() *StateManifest {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()
	return rp.state
}

// findLastMessageFor returns the most recent error or info message that
// references repo.URL, or an empty string. Caller must NOT hold rp.mutex.
func (rp *RepositoryProcessor) findLastMessageFor(repoURL string) string {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()
	for i := len(rp.stats.CloneErrors) - 1; i >= 0; i-- {
		if strings.Contains(rp.stats.CloneErrors[i], repoURL) {
			return rp.stats.CloneErrors[i]
		}
	}
	for i := len(rp.stats.CloneInfos) - 1; i >= 0; i-- {
		if strings.Contains(rp.stats.CloneInfos[i], repoURL) {
			return rp.stats.CloneInfos[i]
		}
	}
	return ""
}

// recordOutcome records the per-repo outcome to the state manifest if one is
// attached. Best-effort; any HEAD-SHA read errors are ignored.
func (rp *RepositoryProcessor) recordOutcome(repo *scm.Repo, status string) {
	rp.mutex.RLock()
	state := rp.state
	rp.mutex.RUnlock()
	if state == nil {
		return
	}
	var sha, errStr string
	switch status {
	case StateStatusOK:
		sha, _ = rp.git.HeadSHA(*repo)
	case StateStatusError:
		errStr = rp.findLastMessageFor(repo.URL)
	}
	state.Record(*repo, status, sha, errStr)
}

// ProcessRepository handles the cloning or updating of a single repository
func (rp *RepositoryProcessor) ProcessRepository(repo *scm.Repo, repoNameWithCollisions map[string]bool, hasCollisions bool, repoSlug string, index int) {
	// Update repo slug for collisions if needed
	finalRepoSlug := rp.handleNameCollisions(*repo, repoNameWithCollisions, hasCollisions, repoSlug, index)

	// Set the final host path
	repo.HostPath = rp.buildHostPath(*repo, finalRepoSlug)

	// Handle prune untouched logic
	if rp.shouldPruneUntouched(repo) {
		return
	}

	// Skip if prune untouched is active (only prune, don't clone)
	if os.Getenv("GHORG_PRUNE_UNTOUCHED") == "true" {
		return
	}

	// Apply clone delay if configured (before any repository operations)
	applyCloneDelay(repo.URL)

	// Determine if this repo exists locally
	repoWillBePulled := repoExistsLocally(*repo)
	var action string

	// Protect local: skip repos with uncommitted changes or unpushed commits
	if repoWillBePulled && os.Getenv("GHORG_PROTECT_LOCAL") == "true" {
		if rp.hasLocalChangesForProtect(*repo) {
			colorlog.PrintWarning(fmt.Sprintf("Protected %s (has local changes or unpushed commits)", repo.URL))
			rp.addProtected(fmt.Sprintf("%s: has local changes or unpushed commits", repo.URL))
			return
		}
	} else if repoWillBePulled {
		// Legacy behavior: skip repos with local modifications
		status, statusErr := rp.git.ShortStatus(*repo)
		if statusErr == nil && status != "" {
			colorlog.PrintWarning(fmt.Sprintf("Skipped %s (has local changes)", repo.URL))
			rp.addSkipped(fmt.Sprintf("%s: has uncommitted local changes", repo.URL))
			return
		}
	}

	// Save current branch for restore if protect-local is enabled
	var originalBranch string
	if repoWillBePulled && os.Getenv("GHORG_PROTECT_LOCAL") == "true" {
		branch, err := rp.git.GetCurrentBranch(*repo)
		if err == nil {
			originalBranch = branch
		}
	}

	// Process the repository (clone or update)
	if repoWillBePulled {
		success := rp.handleExistingRepository(repo, &action)
		if !success {
			rp.recordOutcome(repo, StateStatusError)
			return
		}
		// Restore original branch if protect-local and we were on a different branch
		if originalBranch != "" && originalBranch != repo.CloneBranch {
			if err := rp.git.CheckoutBranch(*repo, originalBranch); err != nil {
				rp.addInfo(fmt.Sprintf("Could not restore original branch %s for %s: %v", originalBranch, repo.URL, err))
			}
		}
	} else {
		success := rp.handleNewRepository(repo, &action)
		if !success {
			rp.recordOutcome(repo, StateStatusError)
			return
		}
	}

	rp.recordOutcome(repo, StateStatusOK)

	// Print unified success message (matching original behavior)
	if repo.SyncedDefaultBranch {
		if repo.Commits.CountDiff > 0 {
			colorlog.PrintSuccess(fmt.Sprintf("Success pull %s, branch: %s, new commits: %d", repo.URL, repo.CloneBranch, repo.Commits.CountDiff))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("Success pull %s, branch: %s", repo.URL, repo.CloneBranch))
		}
	} else if repoWillBePulled && repo.Commits.CountDiff > 0 {
		colorlog.PrintSuccess(fmt.Sprintf("Success %s %s, branch: %s, new commits: %d", action, repo.URL, repo.CloneBranch, repo.Commits.CountDiff))
	} else {
		colorlog.PrintSuccess(fmt.Sprintf("Success %s %s, branch: %s", action, repo.URL, repo.CloneBranch))
	}
}

// handleNameCollisions manages repository name collisions
func (rp *RepositoryProcessor) handleNameCollisions(repo scm.Repo, repoNameWithCollisions map[string]bool, hasCollisions bool, repoSlug string, index int) string {
	if !hasCollisions {
		return rp.addSuffixesIfNeeded(repo, repoSlug)
	}

	rp.mutex.Lock()
	var inHash bool
	if repo.IsGitLabSnippet && !repo.IsGitLabRootLevelSnippet {
		inHash = repoNameWithCollisions[repo.GitLabSnippetInfo.NameOfRepo]
	} else {
		inHash = repoNameWithCollisions[repo.Name]
	}
	rp.mutex.Unlock()

	if inHash {
		// Replace both forward slashes and backslashes with underscores for cross-platform compatibility
		pathWithUnderscores := strings.ReplaceAll(repo.Path, "/", "_")
		pathWithUnderscores = strings.ReplaceAll(pathWithUnderscores, "\\", "_")
		repoSlug = trimCollisionFilename(pathWithUnderscores)
		repoSlug = rp.addSuffixesIfNeeded(repo, repoSlug)

		rp.mutex.Lock()
		slugCollision := repoNameWithCollisions[repoSlug]
		rp.mutex.Unlock()

		if slugCollision {
			repoSlug = fmt.Sprintf("_%v_%v", strconv.Itoa(index), repoSlug)
		} else {
			rp.mutex.Lock()
			repoNameWithCollisions[repoSlug] = true
			rp.mutex.Unlock()
		}
	}

	return rp.addSuffixesIfNeeded(repo, repoSlug)
}

// addSuffixesIfNeeded adds appropriate suffixes for wikis and snippets
func (rp *RepositoryProcessor) addSuffixesIfNeeded(repo scm.Repo, repoSlug string) string {
	if repo.IsWiki && !strings.HasSuffix(repoSlug, ".wiki") {
		repoSlug = repoSlug + ".wiki"
	}

	if repo.IsGitLabSnippet && !repo.IsGitLabRootLevelSnippet && !strings.HasSuffix(repoSlug, ".snippets") {
		repoSlug = repoSlug + ".snippets"
	}

	return repoSlug
}

// buildHostPath constructs the final host path for the repository
func (rp *RepositoryProcessor) buildHostPath(repo scm.Repo, repoSlug string) string {
	if repo.IsGitLabRootLevelSnippet {
		return filepath.Join(outputDirAbsolutePath, "_ghorg_root_level_snippets", repo.GitLabSnippetInfo.Title+"-"+repo.GitLabSnippetInfo.ID)
	}

	if repo.IsGitLabSnippet {
		return filepath.Join(outputDirAbsolutePath, repoSlug, repo.GitLabSnippetInfo.Title+"-"+repo.GitLabSnippetInfo.ID)
	}

	return filepath.Join(outputDirAbsolutePath, repoSlug)
}

// shouldPruneUntouched determines if a repository should be pruned as untouched
func (rp *RepositoryProcessor) shouldPruneUntouched(repo *scm.Repo) bool {
	if os.Getenv("GHORG_PRUNE_UNTOUCHED") != "true" || !repoExistsLocally(*repo) {
		return false
	}

	// Fetch and check branches
	_ = rp.git.FetchCloneBranch(*repo)

	branches, err := rp.git.Branch(*repo)
	if err != nil {
		colorlog.PrintError(fmt.Sprintf("Failed to list local branches for repository %s: %v", repo.Name, err))
		return false
	}

	// Delete if it has no branches
	if branches == "" {
		rp.mutex.Lock()
		rp.untouchedRepos = append(rp.untouchedRepos, repo.HostPath)
		rp.mutex.Unlock()
		return true
	}

	// Skip if multiple branches
	if len(strings.Split(strings.TrimSpace(branches), "\n")) > 1 {
		return false
	}

	// Check for modified changes
	status, err := rp.git.ShortStatus(*repo)
	if err != nil {
		colorlog.PrintError(fmt.Sprintf("Failed to get short status for repository %s: %v", repo.Name, err))
		return false
	}

	if status != "" {
		return false
	}

	// Check for new commits on the branch that exist locally but not on the remote
	commits, err := rp.git.RevListCompare(*repo, "HEAD", "@{u}")
	if err != nil {
		colorlog.PrintError(fmt.Sprintf("Failed to get commit differences for repository %s. The repository may be empty or does not have a .git directory. Error: %v", repo.Name, err))
		return false
	}

	if commits != "" {
		return false
	}

	rp.mutex.Lock()
	rp.untouchedRepos = append(rp.untouchedRepos, repo.HostPath)
	rp.mutex.Unlock()
	return true
}

// handleExistingRepository processes repositories that already exist locally
func (rp *RepositoryProcessor) handleExistingRepository(repo *scm.Repo, action *string) bool {
	*action = "pulling"

	// Set origin with credentials
	err := rp.git.SetOriginWithCredentials(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem setting remote with credentials on: %s Error: %v", repo.Name, err))
		return false
	}

	var success bool
	if os.Getenv("GHORG_BACKUP") == "true" {
		*action = "updating remote"
		success = rp.handleBackupMode(repo)
	} else if os.Getenv("GHORG_NO_CLEAN") == "true" {
		*action = "fetching"
		success = rp.handleNoCleanMode(repo)
	} else {
		// Standard pull mode
		success = rp.handleStandardPull(repo)
	}

	// Always reset origin to remove credentials, even if processing failed
	err = rp.git.SetOrigin(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem resetting remote: %s Error: %v", repo.Name, err))
		return false
	}

	// Return success after ensuring tokens are stripped
	if !success {
		return false
	}

	rp.mutex.Lock()
	rp.stats.PulledCount++
	rp.mutex.Unlock()

	return true
}

// handleNewRepository processes repositories that don't exist locally
func (rp *RepositoryProcessor) handleNewRepository(repo *scm.Repo, action *string) bool {
	*action = "cloning"

	err := rp.git.Clone(*repo)

	// Handle wiki clone attempts that might fail
	if err != nil && repo.IsWiki {
		rp.addInfo(fmt.Sprintf("Wiki may be enabled but there was no content to clone: %s Error: %v", repo.URL, err))
		return false
	}

	if err != nil {
		rp.addError(fmt.Sprintf("Problem trying to clone: %s Error: %v", repo.URL, err))
		return false
	}

	// Checkout specific branch if specified
	if os.Getenv("GHORG_BRANCH") != "" {
		checkoutErr := rp.git.Checkout(*repo)
		if checkoutErr != nil {
			rp.addInfo(fmt.Sprintf("Could not checkout out %s, branch may not exist or may not have any contents/commits, no changes to: %s Error: %v", repo.CloneBranch, repo.URL, checkoutErr))
			return false
		}
	}

	rp.mutex.Lock()
	rp.stats.CloneCount++
	rp.mutex.Unlock()

	// Set origin to remove credentials from URL
	err = rp.git.SetOrigin(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem trying to set remote: %s Error: %v", repo.URL, err))
		return false
	}

	// Fetch all if enabled
	if os.Getenv("GHORG_FETCH_ALL") == "true" {
		// Temporarily restore credentials for fetch-all to work with private repos
		err = rp.git.SetOriginWithCredentials(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to set remote with credentials: %s Error: %v", repo.URL, err))
			return false
		}

		err = rp.git.FetchAll(*repo)
		fetchErr := err // Store fetch error for later reporting

		// Always strip credentials again for security, even if fetch failed
		err = rp.git.SetOrigin(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to reset remote after fetch: %s Error: %v", repo.URL, err))
			return false
		}

		// Report fetch error if it occurred
		if fetchErr != nil {
			rp.addError(fmt.Sprintf("Could not fetch remotes: %s Error: %v", repo.URL, fetchErr))
			return false
		}
	}

	return true
}

// handleBackupMode processes repositories in backup mode
func (rp *RepositoryProcessor) handleBackupMode(repo *scm.Repo) bool {
	err := rp.git.UpdateRemote(*repo)

	if err != nil && repo.IsWiki {
		rp.addInfo(fmt.Sprintf("Wiki may be enabled but there was no content to clone on: %s Error: %v", repo.URL, err))
		return false
	}

	if err != nil {
		rp.addError(fmt.Sprintf("Could not update remotes: %s Error: %v", repo.URL, err))
		return false
	}

	rp.mutex.Lock()
	rp.stats.UpdateRemoteCount++
	rp.mutex.Unlock()

	return true
}

// handleNoCleanMode processes repositories in no-clean mode
func (rp *RepositoryProcessor) handleNoCleanMode(repo *scm.Repo) bool {
	// Fetch all if enabled
	if os.Getenv("GHORG_FETCH_ALL") == "true" {
		// Temporarily restore credentials for fetch-all to work with private repos
		err := rp.git.SetOriginWithCredentials(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to set remote with credentials: %s Error: %v", repo.URL, err))
			return false
		}

		err = rp.git.FetchAll(*repo)
		fetchErr := err // Store fetch error for later reporting

		// Always strip credentials again for security, even if fetch failed
		err = rp.git.SetOrigin(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to reset remote after fetch: %s Error: %v", repo.URL, err))
			return false
		}

		if fetchErr != nil && repo.IsWiki {
			rp.addInfo(fmt.Sprintf("Wiki may be enabled but there was no content to clone on: %s Error: %v", repo.URL, fetchErr))
			return false
		}

		if fetchErr != nil {
			rp.addError(fmt.Sprintf("Could not fetch remotes: %s Error: %v", repo.URL, fetchErr))
			return false
		}
	}

	// If enabled, attempt to synchronize default branch to HEAD
	if os.Getenv("GHORG_SYNC_DEFAULT_BRANCH") == "true" {
		wasUpdated, err := rp.git.SyncDefaultBranch(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Could not sync default branch for %s: %v", repo.URL, err))
		} else if wasUpdated {
			// Only increment if sync actually made changes
			repo.SyncedDefaultBranch = true
			rp.mutex.Lock()
			rp.stats.SyncedCount++
			rp.mutex.Unlock()
		}
	}

	return true
}

// handleStandardPull processes repositories in standard pull mode
func (rp *RepositoryProcessor) handleStandardPull(repo *scm.Repo) bool {
	// Fetch all if enabled
	if os.Getenv("GHORG_FETCH_ALL") == "true" {
		// Temporarily restore credentials for fetch-all to work with private repos
		err := rp.git.SetOriginWithCredentials(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to set remote with credentials: %s Error: %v", repo.URL, err))
			return false
		}

		err = rp.git.FetchAll(*repo)
		fetchErr := err // Store fetch error for later reporting

		// Always strip credentials again for security, even if fetch failed
		err = rp.git.SetOrigin(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Problem trying to reset remote after fetch: %s Error: %v", repo.URL, err))
			return false
		}

		// Report fetch error if it occurred
		if fetchErr != nil {
			rp.addError(fmt.Sprintf("Could not fetch remotes: %s Error: %v", repo.URL, fetchErr))
			return false
		}
	}

	// Checkout branch
	err := rp.git.Checkout(*repo)
	if err != nil {
		_ = rp.git.FetchCloneBranch(*repo)

		// Retry checkout
		errRetry := rp.git.Checkout(*repo)
		if errRetry != nil {
			hasRemoteHeads, errHasRemoteHeads := rp.git.HasRemoteHeads(*repo)
			if errHasRemoteHeads != nil {
				rp.addError(fmt.Sprintf("Could not checkout %s, branch may not exist or may not have any contents/commits, no changes made on: %s Errors: %v %v", repo.CloneBranch, repo.URL, errRetry, errHasRemoteHeads))
				return false
			}
			if hasRemoteHeads {
				rp.addError(fmt.Sprintf("Could not checkout %s, branch may not exist or may not have any contents/commits, no changes made on: %s Error: %v", repo.CloneBranch, repo.URL, errRetry))
				return false
			} else {
				rp.addInfo(fmt.Sprintf("Could not checkout %s due to repository being empty, no changes made on: %s", repo.CloneBranch, repo.URL))
				return false
			}
		}
	}

	// Get pre-pull commit count
	count, err := rp.git.RepoCommitCount(*repo)
	if err != nil {
		rp.addInfo(fmt.Sprintf("Problem trying to get pre pull commit count for on repo: %s", repo.URL))
	}
	repo.Commits.CountPrePull = count

	// Clean
	err = rp.git.Clean(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem running git clean: %s Error: %v", repo.URL, err))
		return false
	}

	// Reset
	err = rp.git.Reset(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem resetting branch: %s for: %s Error: %v", repo.CloneBranch, repo.URL, err))
		return false
	}

	// Pull
	err = rp.git.Pull(*repo)
	if err != nil {
		rp.addError(fmt.Sprintf("Problem trying to pull branch: %v for: %s Error: %v", repo.CloneBranch, repo.URL, err))
		return false
	}

	// Get post-pull commit count
	count, err = rp.git.RepoCommitCount(*repo)
	if err != nil {
		rp.addInfo(fmt.Sprintf("Problem trying to get post pull commit count for on repo: %s", repo.URL))
	}

	repo.Commits.CountPostPull = count
	repo.Commits.CountDiff = (repo.Commits.CountPostPull - repo.Commits.CountPrePull)

	rp.mutex.Lock()
	rp.stats.NewCommits += repo.Commits.CountDiff
	rp.mutex.Unlock()

	// If enabled, attempt to synchronize default branch to HEAD
	if os.Getenv("GHORG_SYNC_DEFAULT_BRANCH") == "true" {
		wasUpdated, err := rp.git.SyncDefaultBranch(*repo)
		if err != nil {
			rp.addError(fmt.Sprintf("Could not sync default branch for %s: %v", repo.URL, err))
		} else if wasUpdated {
			// Only increment if sync actually made changes
			repo.SyncedDefaultBranch = true
			rp.mutex.Lock()
			rp.stats.SyncedCount++
			rp.mutex.Unlock()
		}
	}

	return true
}

// addError adds an error to the stats in a thread-safe manner
func (rp *RepositoryProcessor) addError(msg string) {
	rp.mutex.Lock()
	rp.stats.CloneErrors = append(rp.stats.CloneErrors, msg)
	rp.mutex.Unlock()
}

// addInfo adds an info message to the stats in a thread-safe manner
func (rp *RepositoryProcessor) addInfo(msg string) {
	rp.mutex.Lock()
	rp.stats.CloneInfos = append(rp.stats.CloneInfos, msg)
	rp.mutex.Unlock()
}

// hasLocalChangesForProtect checks if a repo has uncommitted changes or unpushed commits.
func (rp *RepositoryProcessor) hasLocalChangesForProtect(repo scm.Repo) bool {
	// Check for uncommitted changes
	status, err := rp.git.ShortStatus(repo)
	if err != nil {
		return false // If we can't check, allow the update
	}
	if status != "" {
		return true
	}

	// Check for unpushed commits (skip this check in backup mode)
	if os.Getenv("GHORG_BACKUP") == "true" {
		return false
	}

	hasUnpushed, err := rp.git.HasUnpushedCommits(repo)
	if err != nil {
		return false // If we can't check, allow the update
	}
	return hasUnpushed
}

// addProtected adds a protected message to the stats in a thread-safe manner
func (rp *RepositoryProcessor) addProtected(msg string) {
	rp.mutex.Lock()
	rp.stats.ProtectedCount++
	rp.protectedRepos = append(rp.protectedRepos, msg)
	rp.mutex.Unlock()
}

// addSkipped adds a skipped message to the stats in a thread-safe manner
func (rp *RepositoryProcessor) addSkipped(msg string) {
	rp.mutex.Lock()
	rp.stats.SkippedCount++
	rp.stats.CloneSkipped = append(rp.stats.CloneSkipped, msg)
	rp.mutex.Unlock()
}

// GetStats returns a copy of the current statistics
func (rp *RepositoryProcessor) GetStats() CloneStats {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()

	return CloneStats{
		CloneCount:           rp.stats.CloneCount,
		PulledCount:          rp.stats.PulledCount,
		SkippedCount:         rp.stats.SkippedCount,
		ProtectedCount:       rp.stats.ProtectedCount,
		UpdateRemoteCount:    rp.stats.UpdateRemoteCount,
		NewCommits:           rp.stats.NewCommits,
		UntouchedPrunes:      rp.stats.UntouchedPrunes,
		SyncedCount:          rp.stats.SyncedCount,
		TotalDurationSeconds: rp.stats.TotalDurationSeconds,
		CloneInfos:           append([]string(nil), rp.stats.CloneInfos...),
		CloneErrors:          append([]string(nil), rp.stats.CloneErrors...),
		CloneSkipped:         append([]string(nil), rp.stats.CloneSkipped...),
	}
}

// GetUntouchedRepos returns the list of untouched repositories
func (rp *RepositoryProcessor) GetUntouchedRepos() []string {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()
	// Return a copy to prevent external modifications
	return append([]string(nil), rp.untouchedRepos...)
}

// SetTotalDuration sets the total duration in seconds for the clone operation
func (rp *RepositoryProcessor) SetTotalDuration(durationSeconds int) {
	rp.mutex.Lock()
	rp.stats.TotalDurationSeconds = durationSeconds
	rp.mutex.Unlock()
}
