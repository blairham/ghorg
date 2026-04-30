package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blairham/ghorg/internal/scm"
)

// StateFileName is the name of the per-repo state manifest written alongside
// _ghorg_stats.csv in the clone target directory.
const StateFileName = "_ghorg_state.json"

const stateSchemaVersion = 1

// Repo status values recorded in the manifest.
const (
	StateStatusOK      = "ok"
	StateStatusError   = "error"
	StateStatusSkipped = "skipped"
)

// RepoState is the per-repository entry in the state manifest.
type RepoState struct {
	Name       string    `json:"name"`
	HostPath   string    `json:"host_path"`
	LastSHA    string    `json:"last_sha,omitempty"`
	LastBranch string    `json:"last_branch,omitempty"`
	LastStatus string    `json:"last_status"`
	LastError  string    `json:"last_error,omitempty"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// StateManifest is a JSON file recording the last-known state of every repo
// processed by ghorg in a given clone target directory. It enables features
// like --retry-failed and differential reclone.
type StateManifest struct {
	mu sync.Mutex

	Version   int                  `json:"version"`
	SCM       string               `json:"scm"`
	Target    string               `json:"target"`
	UpdatedAt time.Time            `json:"updated_at"`
	Repos     map[string]RepoState `json:"repos"`
}

// NewStateManifest returns an empty manifest tagged with the given scm/target.
func NewStateManifest(scmType, target string) *StateManifest {
	return &StateManifest{
		Version: stateSchemaVersion,
		SCM:     scmType,
		Target:  target,
		Repos:   make(map[string]RepoState),
	}
}

// LoadState reads a manifest from disk. If the file does not exist, returns
// an empty manifest and no error — first-run is a normal case.
func LoadState(path, scmType, target string) (*StateManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewStateManifest(scmType, target), nil
		}
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}

	m := &StateManifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}
	if m.Version != stateSchemaVersion {
		return nil, fmt.Errorf("unsupported state schema version %d in %s (expected %d)", m.Version, path, stateSchemaVersion)
	}
	if m.Repos == nil {
		m.Repos = make(map[string]RepoState)
	}
	// SCM/target may be re-tagged on save; do not error if they differ here,
	// the caller decides whether the manifest is applicable.
	return m, nil
}

// SaveState writes the manifest to disk atomically (write tmp, then rename).
func SaveState(path string, m *StateManifest) error {
	m.mu.Lock()
	m.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(m, "", "  ")
	m.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ghorg-state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state file into place: %w", err)
	}
	return nil
}

// Record updates the manifest entry for the given repo. Safe for concurrent
// callers. errStr may be empty for success cases.
func (m *StateManifest) Record(repo scm.Repo, status, sha, errStr string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Repos == nil {
		m.Repos = make(map[string]RepoState)
	}
	prev := m.Repos[repo.URL]
	entry := RepoState{
		Name:       repo.Name,
		HostPath:   repo.HostPath,
		LastSHA:    sha,
		LastBranch: repo.CloneBranch,
		LastStatus: status,
		LastError:  errStr,
		LastSeenAt: time.Now().UTC(),
	}
	// On error, preserve the last successful SHA if the new write doesn't have one.
	if status == StateStatusError && entry.LastSHA == "" {
		entry.LastSHA = prev.LastSHA
	}
	m.Repos[repo.URL] = entry
}

// FailedRepos returns the URLs of repos whose last recorded status was error.
func (m *StateManifest) FailedRepos() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, 0)
	for url, r := range m.Repos {
		if r.LastStatus == StateStatusError {
			out = append(out, url)
		}
	}
	return out
}
