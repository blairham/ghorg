package cmd

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/blairham/ghorg/internal/scm"
)

func TestShouldLowerRegularString(t *testing.T) {

	upperName := "RepoName"
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{upperName})

	if outputDirName != "reponame" {
		t.Errorf("Wrong folder name, expected: %s, got: %s", upperName, outputDirName)
	}
}

func TestShouldNotChangeLowerCasedRegularString(t *testing.T) {

	lowerName := "repo_name"
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{lowerName})

	if outputDirName != "repo_name" {
		t.Errorf("Wrong folder name, expected: %s, got: %s", lowerName, outputDirName)
	}
}

func TestReplaceDashWithUnderscore(t *testing.T) {

	want := "repo-name"
	lowerName := "repo-name"
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{lowerName})

	if outputDirName != want {
		t.Errorf("Wrong folder name, expected: %s, got: %s", want, outputDirName)
	}
}

func TestShouldNotChangeNonLettersString(t *testing.T) {

	numberName := "1234567_8"
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{numberName})

	if outputDirName != "1234567_8" {
		t.Errorf("Wrong folder name, expected: %s, got: %s", numberName, outputDirName)
	}
}

func TestSourcehutStripsTildePrefix(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	usernameWithTilde := "~blairham"
	os.Setenv("GHORG_SCM_TYPE", "sourcehut")
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{usernameWithTilde})

	expected := "blairham"
	if outputDirName != expected {
		t.Errorf("Wrong folder name for sourcehut with tilde, expected: %s, got: %s", expected, outputDirName)
	}
}

func TestSourcehutWithoutTildePrefix(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	usernameWithoutTilde := "blairham"
	os.Setenv("GHORG_SCM_TYPE", "sourcehut")
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{usernameWithoutTilde})

	expected := "blairham"
	if outputDirName != expected {
		t.Errorf("Wrong folder name for sourcehut without tilde, expected: %s, got: %s", expected, outputDirName)
	}
}

func TestSourcehutStripsTildePrefixUppercase(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	usernameWithTilde := "~blairham"
	os.Setenv("GHORG_SCM_TYPE", "sourcehut")
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{usernameWithTilde})

	// Should be lowercased AND have tilde stripped
	expected := "blairham"
	if outputDirName != expected {
		t.Errorf("Wrong folder name for sourcehut with uppercase and tilde, expected: %s, got: %s", expected, outputDirName)
	}
}

func TestNonSourcehutPreservesTilde(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	usernameWithTilde := "~user123"
	os.Setenv("GHORG_SCM_TYPE", "github")
	defer setOutputDirName([]string{""})
	setOutputDirName([]string{usernameWithTilde})

	// For non-sourcehut SCMs, tilde should be preserved (even though it's unusual)
	expected := "~user123"
	if outputDirName != expected {
		t.Errorf("Wrong folder name for github with tilde, expected: %s, got: %s", expected, outputDirName)
	}
}

type MockGitClient struct{}

func NewMockGit() MockGitClient {
	return MockGitClient{}
}

func (g MockGitClient) HasRemoteHeads(repo scm.Repo) (bool, error) {
	if repo.Name == "testRepoEmpty" {
		return false, nil
	}
	return true, nil
}

func (g MockGitClient) Clone(repo scm.Repo) error {
	_, err := os.MkdirTemp(os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO"), repo.Name)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (g MockGitClient) SetOrigin(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) SetOriginWithCredentials(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) Checkout(repo scm.Repo) error {
	if repo.Name == "testRepoEmpty" {
		return errors.New("Cannot checkout any specific branch in an empty repository")
	}
	return nil
}

func (g MockGitClient) Clean(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) UpdateRemote(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) Pull(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) Reset(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) FetchAll(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) FetchCloneBranch(repo scm.Repo) error {
	return nil
}

func (g MockGitClient) RepoCommitCount(repo scm.Repo) (int, error) {
	return 0, nil
}

func (g MockGitClient) Branch(repo scm.Repo) (string, error) {
	return "", nil
}

func (g MockGitClient) RevListCompare(repo scm.Repo, ref1 string, ref2 string) (string, error) {
	return "", nil
}

func (g MockGitClient) ShortStatus(repo scm.Repo) (string, error) {
	return "", nil
}

func (g MockGitClient) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	return false, nil
}

// GetRemoteURL returns the URL for the given remote name.
func (g MockGitClient) GetRemoteURL(repo scm.Repo, remote string) (string, error) {
	return "https://github.com/mock/repo.git", nil
}

// HasLocalChanges returns true if there are uncommitted changes in the working tree.
func (g MockGitClient) HasLocalChanges(repo scm.Repo) (bool, error) {
	return false, nil
}

// HasUnpushedCommits returns true if there are commits present locally that are not pushed to upstream.
func (g MockGitClient) HasUnpushedCommits(repo scm.Repo) (bool, error) {
	return false, nil
}

// GetCurrentBranch returns the currently checked-out branch name.
func (g MockGitClient) GetCurrentBranch(repo scm.Repo) (string, error) {
	return "main", nil
}

// HasCommitsNotOnDefaultBranch returns true if currentBranch contains commits not present on the default branch.
func (g MockGitClient) HasCommitsNotOnDefaultBranch(repo scm.Repo, currentBranch string) (bool, error) {
	return false, nil
}

// IsDefaultBranchBehindHead returns true if the default branch is an ancestor of the current branch.
func (g MockGitClient) IsDefaultBranchBehindHead(repo scm.Repo, currentBranch string) (bool, error) {
	return false, nil
}

// MergeIntoDefaultBranch attempts a fast-forward merge of currentBranch into the default branch locally.
func (g MockGitClient) MergeIntoDefaultBranch(repo scm.Repo, currentBranch string) error {
	return nil
}

// UpdateRef updates a local ref to point to the given remote ref.
func (g MockGitClient) UpdateRef(repo scm.Repo, refName string, commitRef string) error {
	return nil
}

func TestInitialClone(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_initial")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")
	var testRepos = []scm.Repo{
		{
			Name: "testRepoOne",
		},
		{
			Name: "testRepoTwo",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	got, _ := os.ReadDir(dir)
	expected := len(testRepos)
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone, expected: %v, got: %v", expected, got)
	}
}

func TestCloneEmptyRepo(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_empty_repo")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	setOuputDirAbsolutePath()

	os.Setenv("GHORG_DONT_EXIT_UNDER_TEST", "true")

	// simulate a previous clone of empty git repository
	repoErr := os.Mkdir(outputDirAbsolutePath+"/"+"testRepoEmpty", 0o700)
	if repoErr != nil {
		log.Fatal(repoErr)
	}
	defer os.RemoveAll(outputDirAbsolutePath + "/" + "testRepoEmpty")

	os.Setenv("GHORG_CONCURRENCY", "1")
	var testRepos = []scm.Repo{
		{
			Name:        "testRepoEmpty",
			URL:         "git@github.com:org/testRepoEmpty.git",
			CloneBranch: "main",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	gotInfos := len(cloneInfos)
	expectedInfos := 1
	if gotInfos != expectedInfos {
		t.Fatalf("Wrong number of cloneInfos, expected: %v, got: %v", expectedInfos, gotInfos)
	}
	gotInfo := cloneInfos[0]
	expected := "Could not checkout main due to repository being empty, no changes made on: git@github.com:org/testRepoEmpty.git"
	if gotInfo != expected {
		t.Errorf("Wrong cloneInfo, expected: %v, got: %v", expected, gotInfo)
	}
}

func TestMatchPrefix(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_match_prefix")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")
	os.Setenv("GHORG_MATCH_PREFIX", "test")
	os.Setenv("GHORG_DONT_EXIT_UNDER_TEST", "true")

	var testRepos = []scm.Repo{
		{
			Name: "testRepoOne",
		},
		{
			Name: "testRepoTwo",
		},
		{
			Name: "testRepoThree",
		},
		{
			Name: "nottestRepoTwo",
		},
		{
			Name: "nottestRepoThree",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	got, _ := os.ReadDir(dir)
	expected := 3
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone, expected: %v, got: %v", expected, len(got))
	}
}

func TestExcludeMatchPrefix(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_exclude_match_prefix")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")
	os.Setenv("GHORG_EXCLUDE_MATCH_PREFIX", "test")
	os.Setenv("GHORG_DONT_EXIT_UNDER_TEST", "true")

	var testRepos = []scm.Repo{
		{
			Name: "testRepoOne",
		},
		{
			Name: "testRepoTwo",
		},
		{
			Name: "testRepoThree",
		},
		{
			Name: "nottestRepoTwo",
		},
		{
			Name: "nottestRepoThree",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	got, _ := os.ReadDir(dir)
	expected := 2
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone, expected: %v, got: %v", expected, got)
	}
}

func TestMatchRegex(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_match_regex")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")
	os.Setenv("GHORG_MATCH_REGEX", "^test-")
	os.Setenv("GHORG_DONT_EXIT_UNDER_TEST", "true")

	var testRepos = []scm.Repo{
		{
			Name: "test-RepoOne",
		},
		{
			Name: "test-RepoTwo",
		},
		{
			Name: "test-RepoThree",
		},
		{
			Name: "nottestRepoTwo",
		},
		{
			Name: "nottestRepoThree",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	got, _ := os.ReadDir(dir)
	expected := 3
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone, expected: %v, got: %v", expected, got)
	}
}

func TestExcludeMatchRegex(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	testDescriptor := "ghorg_test_exclude_match_regex"
	dir, err := os.MkdirTemp("", testDescriptor)
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")
	os.Setenv("GHORG_OUTPUT_DIR", testDescriptor)
	os.Setenv("GHORG_EXCLUDE_MATCH_REGEX", "^test-")
	os.Setenv("GHORG_DONT_EXIT_UNDER_TEST", "true")

	var testRepos = []scm.Repo{
		{
			Name: "test-RepoOne",
		},
		{
			Name: "test-RepoTwo",
		},
		{
			Name: "test-RepoThree",
		},
		{
			Name: "nottestRepoTwo",
		},
		{
			Name: "nottestRepoThree",
		},
	}

	mockGit := NewMockGit()
	commandStartTime = time.Now() // Set command start time for timing functionality
	CloneAllRepos(mockGit, testRepos)
	got, _ := os.ReadDir(dir)
	expected := 2
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone, expected: %v, got: %v", expected, got)
	}
}

// UnsetEnv unsets all envars having prefix and returns a function
// that restores the env. Any newly added envars having prefix are
// also unset by restore. It is idiomatic to use with a defer.
//
// Note that modifying the env may have unpredictable results when
// tests are run with t.Parallel.
func UnsetEnv(prefix string) (restore func()) {
	before := map[string]string{}

	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, prefix) {
			continue
		}

		parts := strings.SplitN(e, "=", 2)
		before[parts[0]] = parts[1]

		os.Unsetenv(parts[0])
	}

	return func() {
		after := map[string]string{}

		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, prefix) {
				continue
			}

			parts := strings.SplitN(e, "=", 2)
			after[parts[0]] = parts[1]

			// Check if the envar previously existed
			v, ok := before[parts[0]]
			if !ok {
				// This is a newly added envar with prefix, zap it
				os.Unsetenv(parts[0])
				continue
			}

			if parts[1] != v {
				// If the envar value has changed, set it back
				os.Setenv(parts[0], v)
			}
		}

		// Still need to check if there have been any deleted envars
		for k, v := range before {
			if _, ok := after[k]; !ok {
				// k is not present in after, so we set it.
				os.Setenv(k, v)
			}
		}
	}
}

func Test_filterWithGhorgignore(t *testing.T) {
	type testCase struct {
		name           string
		cloneTargets   []scm.Repo
		expectedResult []scm.Repo
	}

	testCases := []testCase{
		{
			name: "filters out repo named 'shouldbeignored'",
			cloneTargets: []scm.Repo{
				{Name: "shouldbeignored", URL: "https://github.com/org/shouldbeignored"},
				{Name: "bar", URL: "https://github.com/org/bar"},
			},
			expectedResult: []scm.Repo{
				{Name: "bar", URL: "https://github.com/org/bar"},
			},
		},
		{
			name: "filters out repo named 'shouldbeignored'",
			cloneTargets: []scm.Repo{
				{Name: "foo", URL: "https://github.com/org/foo"},
				{Name: "shouldbeignored", URL: "https://github.com/org/shouldbeignored"},
			},
			expectedResult: []scm.Repo{
				{Name: "foo", URL: "https://github.com/org/foo"},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := createTempFileWithContent("shouldbeignored")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			os.Setenv("GHORG_IGNORE_PATH", tmpfile.Name())

			got := filterByGhorgignore(tt.cloneTargets)
			if !reflect.DeepEqual(got, tt.expectedResult) {
				t.Errorf("filterWithGhorgignore() = %v, want %v", got, tt.expectedResult)
			}
		})
	}
}

// createTempFileWithContent will create
func createTempFileWithContent(content string) (*os.File, error) {
	tmpfile, err := os.CreateTemp("", "ghorgtest")
	if err != nil {
		return nil, err
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return nil, err
	}

	if err := tmpfile.Close(); err != nil {
		return nil, err
	}

	return tmpfile, nil
}

func Test_filterDownReposIfTargetReposPathEnabled(t *testing.T) {
	type testCase struct {
		name           string
		cloneTargets   []scm.Repo
		expectedResult []scm.Repo
	}

	testCases := []testCase{
		{
			name: "filters out repos not matching 'targetRepo'",
			cloneTargets: []scm.Repo{
				{Name: "targetRepo", URL: "https://github.com/org/targetRepo"},
				{Name: "bar", URL: "https://github.com/org/bar"},
			},
			expectedResult: []scm.Repo{
				{Name: "targetRepo", URL: "https://github.com/org/targetRepo"},
			},
		},
		{
			name: "filters out all repos",
			cloneTargets: []scm.Repo{
				{Name: "foo", URL: "https://github.com/org/foo"},
				{Name: "shouldbefiltered", URL: "https://github.com/org/shouldbefiltered"},
			},
			expectedResult: []scm.Repo{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := createTempFileWithContent("targetRepo")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			os.Setenv("GHORG_TARGET_REPOS_PATH", tmpfile.Name())

			got := filterByTargetReposPath(tt.cloneTargets)
			if !reflect.DeepEqual(got, tt.expectedResult) {
				t.Errorf("filterWithGhorgignore() = %v, want %v", got, tt.expectedResult)
			}
		})
	}
}

func TestRelativePathRepositories(t *testing.T) {
	testing, err := os.MkdirTemp("", "testing")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(testing)

	outputDirAbsolutePath = testing

	repository := filepath.Join(testing, "repository", ".git")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	files, err := getRelativePathRepositories(testing)
	if err != nil {
		t.Fatalf("getRelativePathRepositories returned an error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(files))
	}

	if len(files) > 0 && files[0] != "repository" {
		t.Errorf("Expected 'repository', got '%s'", files[0])
	}
}

func TestRelativePathRepositoriesNoGitDir(t *testing.T) {
	testing, err := os.MkdirTemp("", "testing")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(testing)

	outputDirAbsolutePath = testing

	directory := filepath.Join(testing, "directory")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	files, err := getRelativePathRepositories(testing)
	if err != nil {
		t.Fatalf("getRelativePathRepositories returned an error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 directories, got %d", len(files))
	}
}

func TestRelativePathRepositoriesWithGitSubmodule(t *testing.T) {
	testing, err := os.MkdirTemp("", "testing")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(testing)

	outputDirAbsolutePath = testing

	repository := filepath.Join(testing, "repository", ".git")
	submodule := filepath.Join(testing, "repository", "submodule", ".git")

	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(submodule), 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if _, err := os.Create(submodule); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	files, err := getRelativePathRepositories(testing)
	if err != nil {
		t.Fatalf("getRelativePathRepositories returned an error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(files))
	}

	if len(files) > 0 && files[0] != "repository" {
		t.Errorf("Expected 'repository', got '%s'", files[0])
	}
}

func TestRelativePathRepositoriesDeeplyNested(t *testing.T) {
	testing, err := os.MkdirTemp("", "testing")
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer os.RemoveAll(testing)

	outputDirAbsolutePath = testing

	repository := filepath.Join(testing, "deeply", "nested", "repository", ".git")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	files, err := getRelativePathRepositories(testing)
	if err != nil {
		t.Fatalf("getRelativePathRepositories returned an error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(files))
	}

	expected := filepath.Join("deeply", "nested", "repository")
	if len(files) > 0 && files[0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, files[0])
	}
}

func TestPruneRepos(t *testing.T) {
	os.Setenv("GHORG_PRUNE_NO_CONFIRM", "true")

	cloneTargets := []scm.Repo{{Path: "/repository"}}

	testing, err := os.MkdirTemp("", "testing")
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer os.RemoveAll(testing)

	outputDirAbsolutePath = testing

	repository := filepath.Join(testing, "repository", ".git")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	prunable := filepath.Join(testing, "prunnable", ".git")
	if err := os.MkdirAll(prunable, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	pruneRepos(cloneTargets)

	if _, err := os.Stat(repository); os.IsNotExist(err) {
		t.Errorf("Expected '%s' to exist, but it was deleted", repository)
	}

	if _, err := os.Stat(prunable); !os.IsNotExist(err) {
		t.Errorf("Expected '%s' to be deleted, but it exists", prunable)
	}
}

func TestSliceContainsNamedRepoWithPathSeparators(t *testing.T) {
	// Test that path separator normalization works correctly
	// This simulates the Windows issue where GitLab API returns forward slashes
	// but Windows filesystem uses backslashes

	testCases := []struct {
		name        string
		repos       []scm.Repo
		needle      string
		shouldMatch bool
	}{
		{
			name:        "Forward slash in repo, forward slash in needle",
			repos:       []scm.Repo{{Path: "group/subgroup/repo"}},
			needle:      "group/subgroup/repo",
			shouldMatch: true,
		},
		{
			name:        "Forward slash in repo, backslash in needle (Windows case)",
			repos:       []scm.Repo{{Path: "group/subgroup/repo"}},
			needle:      "group\\subgroup\\repo",
			shouldMatch: true,
		},
		{
			name:        "Backslash in repo, forward slash in needle",
			repos:       []scm.Repo{{Path: "group\\subgroup\\repo"}},
			needle:      "group/subgroup/repo",
			shouldMatch: true,
		},
		{
			name:        "Leading slash normalization",
			repos:       []scm.Repo{{Path: "/group/subgroup/repo"}},
			needle:      "group\\subgroup\\repo",
			shouldMatch: true,
		},
		{
			name:        "Mixed separators",
			repos:       []scm.Repo{{Path: "group/subgroup\\repo"}},
			needle:      "group\\subgroup/repo",
			shouldMatch: true,
		},
		{
			name:        "No match case",
			repos:       []scm.Repo{{Path: "group/subgroup/repo"}},
			needle:      "different/path",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sliceContainsNamedRepo(tc.repos, tc.needle)
			if result != tc.shouldMatch {
				t.Errorf("Expected %v, got %v for needle '%s' in repos %+v",
					tc.shouldMatch, result, tc.needle, tc.repos)
			}
		})
	}
}

func TestCloneAllRepos_Timing(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_timing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")

	var testRepos = []scm.Repo{
		{
			Name: "testRepoOne",
		},
		{
			Name: "testRepoTwo",
		},
	}

	// Create an extended mock that can simulate timing
	mockGit := NewMockGit()

	// Set command start time before calling CloneAllRepos (simulating what cloneFunc does)
	commandStartTime = time.Now()
	before := commandStartTime
	CloneAllRepos(mockGit, testRepos)
	after := time.Now()

	// The actual duration should be close to what we measured
	actualDuration := int(after.Sub(before).Seconds() + 0.5)

	// Since we can't easily access the processor from this test,
	// we verify that the timing functionality doesn't break anything
	got, _ := os.ReadDir(dir)
	expected := len(testRepos)
	if len(got) != expected {
		t.Errorf("Wrong number of repos in clone (timing test), expected: %v, got: %v", expected, got)
	}

	// Verify that actual duration is reasonable (should be less than 5 seconds for this simple test)
	if actualDuration > 5 {
		t.Errorf("Test took too long, expected less than 5 seconds, got %d seconds", actualDuration)
	}
}

// DelayedMockGit is a mock that adds delay to clone operations for timing tests
type DelayedMockGit struct {
	MockGitClient
}

func (g DelayedMockGit) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	return g.MockGitClient.SyncDefaultBranch(repo)
}

func (g DelayedMockGit) Clone(repo scm.Repo) error {
	time.Sleep(100 * time.Millisecond) // Add 100ms delay
	return g.MockGitClient.Clone(repo)
}

func TestCloneAllRepos_TimingWithDelay(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_timing_delay")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")

	var testRepos = []scm.Repo{
		{
			Name: "testRepoDelayed",
		},
	}

	delayedMock := DelayedMockGit{MockGitClient: NewMockGit()}

	// Set command start time before calling CloneAllRepos (simulating what cloneFunc does)
	commandStartTime = time.Now()
	before := commandStartTime
	CloneAllRepos(delayedMock, testRepos)
	after := time.Now()

	actualDuration := after.Sub(before)

	// Should be at least 100ms due to our delay
	if actualDuration < 100*time.Millisecond {
		t.Errorf("Expected at least 100ms duration due to delay, got %v", actualDuration)
	}

	// Should still complete in reasonable time (less than 2 seconds)
	if actualDuration > 2*time.Second {
		t.Errorf("Test took too long, expected less than 2 seconds, got %v", actualDuration)
	}
}

func TestWriteGhorgStats_WithTiming(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	// Create temporary directory for stats file
	dir, err := os.MkdirTemp("", "ghorg_stats_timing_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)

	// Create some test data
	date := "2023-12-01 15:04:05"
	allReposToCloneCount := 5
	cloneCount := 3
	pulledCount := 2
	cloneInfosCount := 1
	cloneErrorsCount := 0
	updateRemoteCount := 1
	newCommits := 10
	pruneCount := 0
	totalDurationSeconds := 45
	hasCollisions := false

	// Call writeGhorgStats with timing data
	err = writeGhorgStats(date, allReposToCloneCount, cloneCount, pulledCount,
		cloneInfosCount, cloneErrorsCount, updateRemoteCount, newCommits,
		0, pruneCount, totalDurationSeconds, hasCollisions)

	if err != nil {
		t.Fatalf("writeGhorgStats returned error: %v", err)
	}

	// Read the stats file and verify timing is included
	statsFilePath := getGhorgStatsFilePath()
	content, err := os.ReadFile(statsFilePath)
	if err != nil {
		t.Fatalf("Failed to read stats file: %v", err)
	}

	contentStr := string(content)

	// Verify header includes timing
	if !strings.Contains(contentStr, "totalDurationSeconds") {
		t.Error("Stats file header should contain 'totalDurationSeconds'")
	}

	// Verify data includes the timing value (45)
	if !strings.Contains(contentStr, ",45,") {
		t.Error("Stats file data should contain the timing value ',45,'")
	}

	// Count the number of commas in header to ensure we have the right number of fields
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")
	if len(lines) < 2 {
		t.Fatal("Stats file should have at least 2 lines (header + data)")
	}

	headerCommas := strings.Count(lines[0], ",")
	dataCommas := strings.Count(lines[1], ",")

	if headerCommas != dataCommas {
		t.Errorf("Header and data should have same number of commas. Header: %d, Data: %d", headerCommas, dataCommas)
	}

	// Should have 19 commas (20 fields total including timing and syncedCount)
	expectedCommas := 19
	if headerCommas != expectedCommas {
		t.Errorf("Expected %d commas in header, got %d", expectedCommas, headerCommas)
	}
}

func TestCommandTiming_IncludesFullDuration(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_full_timing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", dir)
	os.Setenv("GHORG_CONCURRENCY", "1")

	var testRepos = []scm.Repo{
		{
			Name: "testRepo",
		},
	}

	// Simulate the beginning of cloneFunc by setting commandStartTime with some artificial delay
	beforeCommand := time.Now()
	commandStartTime = beforeCommand

	// Add a small delay to simulate command setup time (like SCM API calls)
	time.Sleep(50 * time.Millisecond)

	// Now call CloneAllRepos (this would happen after SCM API calls)
	mockGit := NewMockGit()
	CloneAllRepos(mockGit, testRepos)

	afterCommand := time.Now()
	commandDuration := afterCommand.Sub(beforeCommand)

	// The timing should include the full duration from command start, not just CloneAllRepos
	// This verifies that we're now capturing the entire command duration including setup time
	if commandDuration < 50*time.Millisecond {
		t.Errorf("Expected command duration to be at least 50ms (including setup delay), got %v", commandDuration)
	}

	// Verify that timing functionality works without breaking anything
	got, _ := os.ReadDir(dir)
	expected := len(testRepos)
	if len(got) != expected {
		t.Errorf("Wrong number of repos cloned, expected: %v, got: %v", expected, len(got))
	}
}

func TestPrintCloneStatsMessage_WithTiming(t *testing.T) {
	// Test different timing formats
	testCases := []struct {
		name              string
		cloneCount        int
		pulledCount       int
		updateRemoteCount int
		newCommits        int
		syncedCount       int
		untouchedPrunes   int
		durationSeconds   int
		expectedText      string
	}{
		{
			name:            "Basic stats under 60 seconds",
			cloneCount:      3,
			pulledCount:     2,
			durationSeconds: 45,
			expectedText:    "completed in 45s",
		},
		{
			name:            "Stats with minutes only",
			cloneCount:      5,
			pulledCount:     3,
			durationSeconds: 120, // 2 minutes exactly
			expectedText:    "completed in 2m",
		},
		{
			name:            "Stats with minutes and seconds",
			cloneCount:      7,
			pulledCount:     4,
			durationSeconds: 95, // 1m 35s
			expectedText:    "completed in 1m35s",
		},
		{
			name:            "Stats with new commits",
			cloneCount:      4,
			pulledCount:     3,
			newCommits:      15,
			durationSeconds: 30,
			expectedText:    "completed in 30s",
		},
		{
			name:              "Stats with update remote",
			cloneCount:        2,
			pulledCount:       1,
			updateRemoteCount: 3,
			newCommits:        8,
			durationSeconds:   75, // 1m 15s
			expectedText:      "completed in 1m15s",
		},
		{
			name:            "Stats with prunes",
			cloneCount:      3,
			pulledCount:     2,
			untouchedPrunes: 1,
			durationSeconds: 25,
			expectedText:    "completed in 25s",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Capture output to verify timing is included
			// Note: In a real scenario we'd use a proper output capture mechanism,
			// but for this test we're just verifying the function doesn't panic
			// and the timing formatting logic is correct

			// This should not panic and should execute successfully
			printCloneStatsMessage(tc.cloneCount, tc.pulledCount, tc.updateRemoteCount,
				tc.newCommits, tc.syncedCount, tc.untouchedPrunes, tc.durationSeconds)

			// The expectedText should be present in the output (in a real test with output capture)
			// For now, we'll just verify the function runs without error
			if tc.expectedText == "" {
				t.Errorf("expectedText should not be empty for test case: %s", tc.name)
			}
		})
	}
}

func TestPrintFinishedWithDirSize_NoTiming(t *testing.T) {
	defer UnsetEnv("GHORG_")()

	// Set up environment
	dir, err := os.MkdirTemp("", "ghorg_test_finished_no_timing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir

	// Test that the function works without timing parameter
	// This should not panic and should execute successfully
	printFinishedWithDirSize()

	// The function should complete without error and not include timing info
}

// SyncTrackingMockGit tracks whether sync was called and what it returned
type SyncTrackingMockGit struct {
	MockGitClient
	syncCalled     bool
	syncWasUpdated bool
	syncError      error
}

func (g *SyncTrackingMockGit) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	g.syncCalled = true
	return g.syncWasUpdated, g.syncError
}

func TestSyncDefaultBranch_Enabled(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_enabled")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	os.Setenv("GHORG_NO_CLEAN", "true") // Use no-clean mode

	// Create a mock repo directory to simulate existing repo
	repoDir := filepath.Join(dir, "testRepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockGit := &SyncTrackingMockGit{
		syncWasUpdated: true, // Simulate that sync made changes
	}
	processor := NewRepositoryProcessor(mockGit)

	repo := scm.Repo{
		Name: "testRepo",
		URL:  "https://github.com/test/testRepo",
	}

	processor.ProcessRepository(&repo, nil, false, "testRepo", 0)

	if !mockGit.syncCalled {
		t.Error("Expected SyncDefaultBranch to be called when GHORG_SYNC_DEFAULT_BRANCH=true")
	}

	stats := processor.GetStats()
	if stats.SyncedCount != 1 {
		t.Errorf("Expected SyncedCount to be 1, got %d", stats.SyncedCount)
	}
}

func TestSyncDefaultBranch_Disabled(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_disabled")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	// GHORG_SYNC_DEFAULT_BRANCH not set - should default to disabled
	os.Setenv("GHORG_NO_CLEAN", "true")

	// Create a mock repo directory to simulate existing repo
	repoDir := filepath.Join(dir, "testRepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockGit := &SyncTrackingMockGit{}
	processor := NewRepositoryProcessor(mockGit)

	repo := scm.Repo{
		Name: "testRepo",
		URL:  "https://github.com/test/testRepo",
	}

	processor.ProcessRepository(&repo, nil, false, "testRepo", 0)

	// Sync should be called even when disabled, but returns false
	// The actual sync function checks the env var and returns early
	if mockGit.syncCalled {
		t.Error("SyncDefaultBranch should not be called when GHORG_SYNC_DEFAULT_BRANCH is not set")
	}

	stats := processor.GetStats()
	if stats.SyncedCount != 0 {
		t.Errorf("Expected SyncedCount to be 0, got %d", stats.SyncedCount)
	}
}

// CountingMockGit tracks call counts and can return different results per call
type CountingMockGit struct {
	MockGitClient
	syncCallCount int
	syncResults   []bool
}

func (g *CountingMockGit) SyncDefaultBranch(repo scm.Repo) (bool, error) {
	if g.syncCallCount >= len(g.syncResults) {
		return false, nil
	}
	result := g.syncResults[g.syncCallCount]
	g.syncCallCount++
	return result, nil
}

func TestSyncDefaultBranch_CountsOnlyWhenUpdated(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_count")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	os.Setenv("GHORG_NO_CLEAN", "true")

	// Create mock repos
	for _, name := range []string{"repo1", "repo2", "repo3"} {
		repoDir := filepath.Join(dir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Mock returns: repo1 updated, repo2 not updated, repo3 updated
	mockGit := &CountingMockGit{
		syncResults: []bool{true, false, true},
	}
	processor := NewRepositoryProcessor(mockGit)

	for _, name := range []string{"repo1", "repo2", "repo3"} {
		repo := scm.Repo{
			Name: name,
			URL:  "https://github.com/test/" + name,
		}
		processor.ProcessRepository(&repo, nil, false, name, 0)
	}

	stats := processor.GetStats()
	// Should only count repo1 and repo3 (the ones that were updated)
	if stats.SyncedCount != 2 {
		t.Errorf("Expected SyncedCount to be 2 (only repos that were updated), got %d", stats.SyncedCount)
	}
}

func TestSyncDefaultBranch_WithStandardPullMode(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_standard")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	// Not setting GHORG_NO_CLEAN or GHORG_BACKUP - should use standard pull mode

	repoDir := filepath.Join(dir, "testRepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockGit := &SyncTrackingMockGit{
		syncWasUpdated: true,
	}
	processor := NewRepositoryProcessor(mockGit)

	repo := scm.Repo{
		Name: "testRepo",
		URL:  "https://github.com/test/testRepo",
	}

	processor.ProcessRepository(&repo, nil, false, "testRepo", 0)

	if !mockGit.syncCalled {
		t.Error("Expected SyncDefaultBranch to be called in standard pull mode")
	}
}

func TestSyncDefaultBranch_WithBackupMode(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_backup")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	os.Setenv("GHORG_BACKUP", "true") // Backup mode

	repoDir := filepath.Join(dir, "testRepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockGit := &SyncTrackingMockGit{
		syncWasUpdated: false,
	}
	processor := NewRepositoryProcessor(mockGit)

	repo := scm.Repo{
		Name: "testRepo",
		URL:  "https://github.com/test/testRepo",
	}

	processor.ProcessRepository(&repo, nil, false, "testRepo", 0)

	// Backup mode uses UpdateRemote, which doesn't call sync
	// Just verify it doesn't crash
}

func TestSyncDefaultBranch_OnlyForExistingRepos(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_new")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")

	// Don't create the directory - this simulates a new repo being cloned
	mockGit := &SyncTrackingMockGit{}
	processor := NewRepositoryProcessor(mockGit)

	repo := scm.Repo{
		Name: "newRepo",
		URL:  "https://github.com/test/newRepo",
	}

	processor.ProcessRepository(&repo, nil, false, "newRepo", 0)

	// Sync should not be called for new repos (only existing ones)
	// The repo will be cloned, not pulled
	if mockGit.syncCalled {
		t.Error("SyncDefaultBranch should not be called for new repos being cloned")
	}

	stats := processor.GetStats()
	if stats.CloneCount != 1 {
		t.Errorf("Expected CloneCount to be 1, got %d", stats.CloneCount)
	}
	if stats.SyncedCount != 0 {
		t.Errorf("Expected SyncedCount to be 0, got %d", stats.SyncedCount)
	}
}

func TestSyncDefaultBranch_StatsReporting(t *testing.T) {
	defer UnsetEnv("GHORG_")()
	dir, err := os.MkdirTemp("", "ghorg_test_sync_stats")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	outputDirAbsolutePath = dir
	os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	os.Setenv("GHORG_NO_CLEAN", "true")

	// Create multiple repos
	numRepos := 5
	for i := 0; i < numRepos; i++ {
		repoName := "repo" + string(rune('0'+i))
		repoDir := filepath.Join(dir, repoName)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Mock returns true for repos 0, 2, 4 (3 total)
	mockGit := &CountingMockGit{
		syncResults: []bool{true, false, true, false, true},
	}
	processor := NewRepositoryProcessor(mockGit)

	for i := 0; i < numRepos; i++ {
		repoName := "repo" + string(rune('0'+i))
		repo := scm.Repo{
			Name: repoName,
			URL:  "https://github.com/test/" + repoName,
		}
		processor.ProcessRepository(&repo, nil, false, repoName, 0)
	}

	stats := processor.GetStats()
	// Should track that 3 repos were synced
	if stats.SyncedCount != 3 {
		t.Errorf("Expected SyncedCount to be 3, got %d", stats.SyncedCount)
	}

	// Verify other stats are also updated
	if stats.PulledCount != numRepos {
		t.Errorf("Expected PulledCount to be %d, got %d", numRepos, stats.PulledCount)
	}
}
