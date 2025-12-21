// Package git provides Git repository synchronization functionality for ghorg.
//
// For comprehensive documentation on sync functionality, safety philosophy,
// configuration options, and troubleshooting, see README.md in this directory.
package git

import (
	"fmt"
	"os"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/scm"
)

// SyncDefaultBranch synchronizes the local default branch with the remote
// It checks for local changes and unpushed commits before performing the sync
// Returns (wasUpdated, error) where wasUpdated indicates if the branch was actually changed
func (g GitClient) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	// Check if sync is disabled via configuration
	// GHORG_SYNC_DEFAULT_BRANCH defaults to false (sync disabled by default)
	syncEnabled := os.Getenv("GHORG_SYNC_DEFAULT_BRANCH")
	if syncEnabled != "true" {
		return false, nil
	}

	// First check if the remote exists and is accessible
	_, err := g.GetRemoteURL(repo, "origin")
	if err != nil {
		// Remote doesn't exist or isn't accessible, skip sync
		return false, nil
	}

	// Get the actual default branch from the remote
	// This ensures we use the correct branch even if repo.CloneBranch is wrong
	defaultBranch, err := g.GetRemoteDefaultBranch(repo)
	if err != nil {
		// If we can't get the remote default branch, fall back to repo.CloneBranch
		defaultBranch = repo.CloneBranch
		if defaultBranch == "" {
			m := fmt.Sprintf("Failed to determine default branch for %s: %v", repo.Name, err)
			colorlog.PrintError(m)
			return false, fmt.Errorf("failed to determine default branch: %w", err)
		}
	}

	// Check what branch we're currently on first
	currentBranch, err := g.GetCurrentBranch(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to get current branch for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if the working directory has any uncommitted changes
	hasWorkingDirChanges, err := g.HasLocalChanges(repo)
	if err != nil {
		m := fmt.Sprintf("Failed to check working directory status for %s: %v", repo.Name, err)
		colorlog.PrintError(m)
		return false, fmt.Errorf("failed to check working directory status: %w", err)
	}

	// Skip sync if working directory has uncommitted changes
	if hasWorkingDirChanges {
		return false, nil
	}

	// Only check for unpushed commits if we're on the default branch
	// (feature branches might not have a remote tracking branch)
	if currentBranch == defaultBranch {
		hasUnpushedCommits, err := g.HasUnpushedCommits(repo)
		if err != nil {
			// If we can't check for unpushed commits (e.g., no tracking branch set up),
			// skip the sync to be safe - we don't want to potentially lose commits
			return false, nil
		}

		// Skip sync if there are unpushed commits
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
	// Temporarily update repo.CloneBranch for the fetch and merge
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
		return true, nil
	}

	wasUpdated := beforeHash != afterHash
	return wasUpdated, nil
}
