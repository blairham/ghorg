package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blairham/ghorg/internal/scm"
)

// setupTestRepo creates a temporary git repository for testing.
// Returns the path to the repo and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ghorg-go-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user and disable autocrlf for Windows compatibility
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "core.autocrlf", "false")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	cmd.Run()

	// Rename default branch to main
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = tmpDir
	cmd.Run()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// setupBareRepoWithClone creates a bare repo and a clone, simulating remote/local setup.
func setupBareRepoWithClone(t *testing.T) (bareRepo, cloneRepo string, cleanup func()) {
	t.Helper()

	// Create bare repo (simulates remote)
	bareDir, err := os.MkdirTemp("", "ghorg-bare-*")
	if err != nil {
		t.Fatalf("failed to create bare dir: %v", err)
	}

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(bareDir)
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create a temporary working repo to push initial content
	workDir, err := os.MkdirTemp("", "ghorg-work-*")
	if err != nil {
		os.RemoveAll(bareDir)
		t.Fatalf("failed to create work dir: %v", err)
	}

	cmd = exec.Command("git", "clone", bareDir, workDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(bareDir)
		os.RemoveAll(workDir)
		t.Fatalf("failed to clone bare repo: %v", err)
	}

	// Configure git user and disable autocrlf to prevent line ending issues on Windows
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = workDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = workDir
	cmd.Run()

	cmd = exec.Command("git", "config", "core.autocrlf", "false")
	cmd.Dir = workDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(bareDir)
		os.RemoveAll(workDir)
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = workDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = workDir
	cmd.Run()

	// Rename to main and push
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = workDir
	cmd.Run()

	cmd = exec.Command("git", "push", "-u", "origin", "main")
	cmd.Dir = workDir
	cmd.Run()

	// Create clone directory
	cloneDir, err := os.MkdirTemp("", "ghorg-clone-*")
	if err != nil {
		os.RemoveAll(bareDir)
		os.RemoveAll(workDir)
		t.Fatalf("failed to create clone dir: %v", err)
	}
	os.RemoveAll(cloneDir) // Remove so clone can create it

	cmd = exec.Command("git", "clone", bareDir, cloneDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(bareDir)
		os.RemoveAll(workDir)
		t.Fatalf("failed to clone: %v", err)
	}

	// Configure git user and disable autocrlf in clone
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = cloneDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = cloneDir
	cmd.Run()

	cmd = exec.Command("git", "config", "core.autocrlf", "false")
	cmd.Dir = cloneDir
	cmd.Run()

	os.RemoveAll(workDir) // Clean up work dir

	cleanup = func() {
		os.RemoveAll(bareDir)
		os.RemoveAll(cloneDir)
	}

	return bareDir, cloneDir, cleanup
}

// TestNewGitBackendSelection tests that NewGit() returns the correct backend based on env var.
func TestNewGitBackendSelection(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		expectType string
	}{
		{
			name:       "Default backend (empty env) is golang",
			envValue:   "",
			expectType: "goGitClient",
		},
		{
			name:       "Explicit exec backend",
			envValue:   "exec",
			expectType: "GitClient",
		},
		{
			name:       "Explicit golang backend",
			envValue:   "golang",
			expectType: "goGitClient",
		},
		{
			name:       "Unknown backend falls back to golang",
			envValue:   "unknown",
			expectType: "goGitClient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			oldValue := os.Getenv("GHORG_GIT_BACKEND")
			defer os.Setenv("GHORG_GIT_BACKEND", oldValue)

			if tt.envValue == "" {
				os.Unsetenv("GHORG_GIT_BACKEND")
			} else {
				os.Setenv("GHORG_GIT_BACKEND", tt.envValue)
			}

			git := NewGit()

			// Check type
			switch git.(type) {
			case GitClient:
				if tt.expectType != "GitClient" {
					t.Errorf("expected %s, got GitClient", tt.expectType)
				}
			case goGitClient:
				if tt.expectType != "goGitClient" {
					t.Errorf("expected %s, got goGitClient", tt.expectType)
				}
			default:
				t.Errorf("unexpected type: %T", git)
			}
		})
	}
}

// TestGoGitClientGetCurrentBranch tests GetCurrentBranch for go-git backend.
func TestGoGitClientGetCurrentBranch(t *testing.T) {
	// Use setupBareRepoWithClone to get a proper repo with remote tracking
	_, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	branch, err := goGit.GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}

	if branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", branch)
	}
}

// TestGoGitClientShortStatus tests ShortStatus for go-git backend.
func TestGoGitClientShortStatus(t *testing.T) {
	// Use setupBareRepoWithClone to get a proper clean repo
	_, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// Clean working directory after clone
	status, err := goGit.ShortStatus(repo)
	if err != nil {
		t.Fatalf("ShortStatus failed: %v", err)
	}

	if status != "" {
		t.Errorf("expected empty status for clean repo, got '%s'", status)
	}

	// Create untracked file
	testFile := filepath.Join(cloneRepo, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("untracked\n"), 0644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	status, err = goGit.ShortStatus(repo)
	if err != nil {
		t.Fatalf("ShortStatus failed: %v", err)
	}

	if status == "" {
		t.Error("expected non-empty status with untracked file")
	}
}

// TestGoGitClientHasLocalChanges tests HasLocalChanges for go-git backend.
func TestGoGitClientHasLocalChanges(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// Clean working directory
	hasChanges, err := goGit.HasLocalChanges(repo)
	if err != nil {
		t.Fatalf("HasLocalChanges failed: %v", err)
	}

	if hasChanges {
		t.Error("expected no local changes for clean repo")
	}

	// Modify existing file
	testFile := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	hasChanges, err = goGit.HasLocalChanges(repo)
	if err != nil {
		t.Fatalf("HasLocalChanges failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected local changes after modifying file")
	}
}

// TestGoGitClientBranch tests Branch for go-git backend.
func TestGoGitClientBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	branches, err := goGit.Branch(repo)
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if branches == "" {
		t.Error("expected at least one branch")
	}

	// Should contain main branch marker
	if !contains(branches, "main") {
		t.Errorf("expected branches to contain 'main', got '%s'", branches)
	}
}

// TestGoGitClientGetRemoteURL tests GetRemoteURL for go-git backend.
func TestGoGitClientGetRemoteURL(t *testing.T) {
	_, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	url, err := goGit.GetRemoteURL(repo, "origin")
	if err != nil {
		t.Fatalf("GetRemoteURL failed: %v", err)
	}

	if url == "" {
		t.Error("expected non-empty remote URL")
	}
}

// TestGoGitClientRepoCommitCount tests RepoCommitCount for go-git backend.
func TestGoGitClientRepoCommitCount(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	count, err := goGit.RepoCommitCount(repo)
	if err != nil {
		t.Fatalf("RepoCommitCount failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 commit, got %d", count)
	}

	// Add another commit
	testFile := filepath.Join(repoPath, "file2.txt")
	if err := os.WriteFile(testFile, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Second commit")
	cmd.Dir = repoPath
	cmd.Run()

	count, err = goGit.RepoCommitCount(repo)
	if err != nil {
		t.Fatalf("RepoCommitCount failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 commits, got %d", count)
	}
}

// TestBackendParity tests that both backends produce the same results.
func TestBackendParity(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	execGit := NewExecGit()
	goGit := GoGitClient()

	t.Run("GetCurrentBranch parity", func(t *testing.T) {
		execBranch, execErr := execGit.GetCurrentBranch(repo)
		goGitBranch, goGitErr := goGit.GetCurrentBranch(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execBranch != goGitBranch {
			t.Errorf("branch mismatch: exec='%s', go-git='%s'", execBranch, goGitBranch)
		}
	})

	t.Run("HasLocalChanges parity - clean", func(t *testing.T) {
		execHas, execErr := execGit.HasLocalChanges(repo)
		goGitHas, goGitErr := goGit.HasLocalChanges(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execHas != goGitHas {
			t.Errorf("HasLocalChanges mismatch: exec=%v, go-git=%v", execHas, goGitHas)
		}
	})

	t.Run("HasLocalChanges parity - dirty", func(t *testing.T) {
		// Create untracked file
		testFile := filepath.Join(repoPath, "parity-test.txt")
		if err := os.WriteFile(testFile, []byte("test\n"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		defer os.Remove(testFile)

		execHas, execErr := execGit.HasLocalChanges(repo)
		goGitHas, goGitErr := goGit.HasLocalChanges(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execHas != goGitHas {
			t.Errorf("HasLocalChanges mismatch: exec=%v, go-git=%v", execHas, goGitHas)
		}
	})

	t.Run("RepoCommitCount parity", func(t *testing.T) {
		execCount, execErr := execGit.RepoCommitCount(repo)
		goGitCount, goGitErr := goGit.RepoCommitCount(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execCount != goGitCount {
			t.Errorf("commit count mismatch: exec=%d, go-git=%d", execCount, goGitCount)
		}
	})
}

// TestBackendParityWithRemote tests backend parity for operations requiring a remote.
func TestBackendParityWithRemote(t *testing.T) {
	bareRepo, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneURL:    bareRepo,
		CloneBranch: "main",
	}

	execGit := NewExecGit()
	goGit := GoGitClient()

	t.Run("GetRemoteURL parity", func(t *testing.T) {
		execURL, execErr := execGit.GetRemoteURL(repo, "origin")
		goGitURL, goGitErr := goGit.GetRemoteURL(repo, "origin")

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execURL != goGitURL {
			t.Errorf("remote URL mismatch: exec='%s', go-git='%s'", execURL, goGitURL)
		}
	})

	t.Run("HasUnpushedCommits parity - no unpushed", func(t *testing.T) {
		execHas, execErr := execGit.HasUnpushedCommits(repo)
		goGitHas, goGitErr := goGit.HasUnpushedCommits(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execHas != goGitHas {
			t.Errorf("HasUnpushedCommits mismatch: exec=%v, go-git=%v", execHas, goGitHas)
		}
	})

	t.Run("HasUnpushedCommits parity - with unpushed", func(t *testing.T) {
		// Create local commit
		testFile := filepath.Join(cloneRepo, "unpushed.txt")
		if err := os.WriteFile(testFile, []byte("unpushed\n"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		cmd := exec.Command("git", "add", ".")
		cmd.Dir = cloneRepo
		cmd.Run()

		cmd = exec.Command("git", "commit", "-m", "Unpushed commit")
		cmd.Dir = cloneRepo
		cmd.Run()

		execHas, execErr := execGit.HasUnpushedCommits(repo)
		goGitHas, goGitErr := goGit.HasUnpushedCommits(repo)

		if (execErr != nil) != (goGitErr != nil) {
			t.Errorf("error mismatch: exec=%v, go-git=%v", execErr, goGitErr)
		}

		if execHas != goGitHas {
			t.Errorf("HasUnpushedCommits mismatch: exec=%v, go-git=%v", execHas, goGitHas)
		}
	})
}

// TestGoGitClientClean tests Clean for go-git backend.
func TestGoGitClientClean(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	// Create untracked file and directory
	testFile := filepath.Join(repoPath, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("untracked\n"), 0644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	testDir := filepath.Join(repoPath, "untracked-dir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create untracked dir: %v", err)
	}

	goGit := GoGitClient()

	err := goGit.Clean(repo)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify untracked file is removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("expected untracked file to be removed")
	}

	// Verify untracked directory is removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("expected untracked directory to be removed")
	}
}

// TestGoGitClientCheckout tests Checkout for go-git backend.
func TestGoGitClientCheckout(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a new branch using exec
	cmd := exec.Command("git", "branch", "feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "feature",
	}

	goGit := GoGitClient()

	err := goGit.Checkout(repo)
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Verify we're on the feature branch
	branch, _ := goGit.GetCurrentBranch(repo)
	if branch != "feature" {
		t.Errorf("expected branch 'feature', got '%s'", branch)
	}
}

// TestGoGitClientSetOrigin tests SetOrigin and SetOriginWithCredentials for go-git backend.
func TestGoGitClientSetOrigin(t *testing.T) {
	_, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		URL:         "https://github.com/test/repo.git",
		CloneURL:    "https://user:token@github.com/test/repo.git",
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// Test SetOrigin
	err := goGit.SetOrigin(repo)
	if err != nil {
		t.Fatalf("SetOrigin failed: %v", err)
	}

	url, _ := goGit.GetRemoteURL(repo, "origin")
	if url != repo.URL {
		t.Errorf("expected URL '%s', got '%s'", repo.URL, url)
	}

	// Test SetOriginWithCredentials
	err = goGit.SetOriginWithCredentials(repo)
	if err != nil {
		t.Fatalf("SetOriginWithCredentials failed: %v", err)
	}

	url, _ = goGit.GetRemoteURL(repo, "origin")
	if url != repo.CloneURL {
		t.Errorf("expected URL '%s', got '%s'", repo.CloneURL, url)
	}
}

// TestGoGitClientReset tests Reset for go-git backend.
func TestGoGitClientReset(t *testing.T) {
	bareRepo, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneURL:    bareRepo,
		CloneBranch: "main",
	}

	// Modify a file locally
	testFile := filepath.Join(cloneRepo, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	goGit := GoGitClient()

	// Verify we have changes
	hasChanges, _ := goGit.HasLocalChanges(repo)
	if !hasChanges {
		t.Error("expected local changes before reset")
	}

	// Reset
	err := goGit.Reset(repo)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Verify changes are gone
	hasChanges, _ = goGit.HasLocalChanges(repo)
	if hasChanges {
		t.Error("expected no local changes after reset")
	}
}

// TestGoGitClientFetchAll tests FetchAll for go-git backend.
func TestGoGitClientFetchAll(t *testing.T) {
	bareRepo, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneURL:    bareRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	err := goGit.FetchAll(repo)
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}
}

// TestGoGitClientFetchCloneBranch tests FetchCloneBranch for go-git backend.
func TestGoGitClientFetchCloneBranch(t *testing.T) {
	bareRepo, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneURL:    bareRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	err := goGit.FetchCloneBranch(repo)
	if err != nil {
		t.Fatalf("FetchCloneBranch failed: %v", err)
	}
}

// TestGoGitClientUpdateRemote tests UpdateRemote for go-git backend.
func TestGoGitClientUpdateRemote(t *testing.T) {
	bareRepo, cloneRepo, cleanup := setupBareRepoWithClone(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    cloneRepo,
		CloneURL:    bareRepo,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	err := goGit.UpdateRemote(repo)
	if err != nil {
		t.Fatalf("UpdateRemote failed: %v", err)
	}
}

// TestGoGitClientHasCommitsNotOnDefaultBranch tests HasCommitsNotOnDefaultBranch for go-git backend.
func TestGoGitClientHasCommitsNotOnDefaultBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// On main branch, should have no extra commits
	hasCommits, err := goGit.HasCommitsNotOnDefaultBranch(repo, "main")
	if err != nil {
		t.Fatalf("HasCommitsNotOnDefaultBranch failed: %v", err)
	}

	if hasCommits {
		t.Error("expected no commits not on default branch when on main")
	}

	// Create feature branch with extra commit
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoPath
	cmd.Run()

	testFile := filepath.Join(repoPath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Feature commit")
	cmd.Dir = repoPath
	cmd.Run()

	hasCommits, err = goGit.HasCommitsNotOnDefaultBranch(repo, "feature")
	if err != nil {
		t.Fatalf("HasCommitsNotOnDefaultBranch failed: %v", err)
	}

	if !hasCommits {
		t.Error("expected commits not on default branch for feature branch")
	}
}

// TestGoGitClientIsDefaultBranchBehindHead tests IsDefaultBranchBehindHead for go-git backend.
func TestGoGitClientIsDefaultBranchBehindHead(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// Create feature branch with extra commit
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoPath
	cmd.Run()

	testFile := filepath.Join(repoPath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Feature commit")
	cmd.Dir = repoPath
	cmd.Run()

	// Default branch should be behind feature
	isBehind, err := goGit.IsDefaultBranchBehindHead(repo, "feature")
	if err != nil {
		t.Fatalf("IsDefaultBranchBehindHead failed: %v", err)
	}

	if !isBehind {
		t.Error("expected default branch to be behind feature branch")
	}
}

// TestGoGitClientUpdateRef tests UpdateRef for go-git backend.
func TestGoGitClientUpdateRef(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	repo := scm.Repo{
		Name:        "test-repo",
		HostPath:    repoPath,
		CloneBranch: "main",
	}

	goGit := GoGitClient()

	// Update a test ref to point to HEAD
	err := goGit.UpdateRef(repo, "refs/heads/test-ref", "HEAD")
	if err != nil {
		t.Fatalf("UpdateRef failed: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
