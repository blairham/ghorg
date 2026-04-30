package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/blairham/ghorg/internal/scm"
)

func TestLoadStateMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m, err := LoadState(filepath.Join(dir, StateFileName), "github", "blairham")
	if err != nil {
		t.Fatalf("LoadState returned unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("Expected non-nil manifest")
	}
	if m.Version != stateSchemaVersion {
		t.Errorf("Version = %d, want %d", m.Version, stateSchemaVersion)
	}
	if m.SCM != "github" || m.Target != "blairham" {
		t.Errorf("SCM/Target = %q/%q, want github/blairham", m.SCM, m.Target)
	}
	if len(m.Repos) != 0 {
		t.Errorf("Expected empty Repos, got %d entries", len(m.Repos))
	}
}

func TestStateRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)

	m := NewStateManifest("github", "blairham")
	m.Record(scm.Repo{
		Name: "ghorg", URL: "https://github.com/blairham/ghorg",
		HostPath: "/tmp/ghorg", CloneBranch: "main",
	}, StateStatusOK, "abc123def456abc123def456abc123def456abcd", "")
	m.Record(scm.Repo{
		Name: "broken", URL: "https://github.com/blairham/broken",
		HostPath: "/tmp/broken", CloneBranch: "main",
	}, StateStatusError, "", "clone failed: timeout")

	if err := SaveState(path, m); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(path, "github", "blairham")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(loaded.Repos) != 2 {
		t.Fatalf("Expected 2 repos after reload, got %d", len(loaded.Repos))
	}

	ok := loaded.Repos["https://github.com/blairham/ghorg"]
	if ok.LastStatus != StateStatusOK || ok.LastSHA == "" {
		t.Errorf("ok entry malformed: %+v", ok)
	}

	bad := loaded.Repos["https://github.com/blairham/broken"]
	if bad.LastStatus != StateStatusError || bad.LastError == "" {
		t.Errorf("error entry malformed: %+v", bad)
	}

	failed := loaded.FailedRepos()
	if len(failed) != 1 || failed[0] != "https://github.com/blairham/broken" {
		t.Errorf("FailedRepos = %v, want [.../broken]", failed)
	}
}

func TestSaveStateAtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)

	// Pre-existing file with garbage; SaveState must replace it cleanly.
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewStateManifest("github", "blairham")
	m.Record(scm.Repo{Name: "x", URL: "u", HostPath: "/p", CloneBranch: "main"}, StateStatusOK, "", "")
	if err := SaveState(path, m); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Verify no leftover tmp files remain in the dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || (len(e.Name()) > 0 && e.Name()[0] == '.' && e.Name() != filepath.Base(path)) {
			t.Errorf("Leftover tmp file in dir: %s", e.Name())
		}
	}

	loaded, err := LoadState(path, "github", "blairham")
	if err != nil {
		t.Fatalf("LoadState after overwrite: %v", err)
	}
	if len(loaded.Repos) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(loaded.Repos))
	}
}

func TestRecordConcurrent(t *testing.T) {
	t.Parallel()
	m := NewStateManifest("github", "blairham")
	var wg sync.WaitGroup
	const n = 200

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Record(scm.Repo{
				Name: "r", URL: "https://example/repo",
				HostPath: "/p", CloneBranch: "main",
			}, StateStatusOK, "sha", "")
		}(i)
	}
	wg.Wait()

	if len(m.Repos) != 1 {
		t.Errorf("Expected 1 entry after concurrent writes to same URL, got %d", len(m.Repos))
	}
}

func TestRecordPreservesPreviousSHAOnError(t *testing.T) {
	t.Parallel()
	m := NewStateManifest("github", "blairham")
	repo := scm.Repo{Name: "r", URL: "u", HostPath: "/p", CloneBranch: "main"}

	m.Record(repo, StateStatusOK, "sha1", "")
	m.Record(repo, StateStatusError, "", "boom")

	got := m.Repos["u"]
	if got.LastStatus != StateStatusError {
		t.Errorf("status = %q, want %q", got.LastStatus, StateStatusError)
	}
	if got.LastSHA != "sha1" {
		t.Errorf("LastSHA = %q, want sha1 (preserved from prior success)", got.LastSHA)
	}
	if got.LastError != "boom" {
		t.Errorf("LastError = %q, want boom", got.LastError)
	}
}

func TestLoadStateUnsupportedVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)
	if err := os.WriteFile(path, []byte(`{"version": 99, "repos": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path, "github", "blairham")
	if err == nil {
		t.Fatal("Expected error for unsupported schema version")
	}
}

func TestLoadStateCorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)
	if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path, "github", "blairham")
	if err == nil {
		t.Fatal("Expected error for corrupt JSON")
	}
	// Should not be ErrNotExist.
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("Did not expect ErrNotExist for corrupt JSON")
	}
}
