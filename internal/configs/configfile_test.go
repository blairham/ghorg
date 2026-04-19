package configs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadConfigValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write a value
	if err := WriteConfigValue(path, "scm.type", "gitlab"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}

	// Read it back
	val, found, err := ReadConfigValue(path, "scm.type")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if !found {
		t.Fatal("expected to find scm.type")
	}
	if val != "gitlab" {
		t.Errorf("expected 'gitlab', got %q", val)
	}
}

func TestWriteMultipleValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := WriteConfigValue(path, "scm.type", "github"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}
	if err := WriteConfigValue(path, "clone.protocol", "ssh"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}
	if err := WriteConfigValue(path, "github.token", "abc123"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}

	values, err := ReadConfigFile(path)
	if err != nil {
		t.Fatalf("ReadConfigFile failed: %v", err)
	}

	tests := map[string]string{
		"scm.type":       "github",
		"clone.protocol": "ssh",
		"github.token":   "abc123",
	}
	for k, expected := range tests {
		if values[k] != expected {
			t.Errorf("key %q: expected %q, got %q", k, expected, values[k])
		}
	}
}

func TestWriteOverwritesExistingValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := WriteConfigValue(path, "scm.type", "github"); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := WriteConfigValue(path, "scm.type", "gitlab"); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	val, found, err := ReadConfigValue(path, "scm.type")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if !found || val != "gitlab" {
		t.Errorf("expected 'gitlab', got %q (found=%v)", val, found)
	}
}

func TestUnsetConfigValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := WriteConfigValue(path, "scm.type", "github"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}
	if err := WriteConfigValue(path, "scm.base-url", "https://example.com/"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}

	if err := UnsetConfigValue(path, "scm.type"); err != nil {
		t.Fatalf("UnsetConfigValue failed: %v", err)
	}

	val, found, err := ReadConfigValue(path, "scm.type")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if found {
		t.Errorf("expected scm.type to be unset, got %q", val)
	}

	// Other key in same section should still exist
	val, found, err = ReadConfigValue(path, "scm.base-url")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if !found || val != "https://example.com/" {
		t.Errorf("expected scm.base-url to still exist, got %q (found=%v)", val, found)
	}
}

func TestUnsetLastKeyInSectionCleansUpSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := WriteConfigValue(path, "stats.enabled", "true"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}
	if err := UnsetConfigValue(path, "stats.enabled"); err != nil {
		t.Fatalf("UnsetConfigValue failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// File should be essentially empty (just "{}\n" for empty YAML map)
	if string(data) != "{}\n" {
		t.Errorf("expected empty YAML, got %q", string(data))
	}
}

func TestReadLegacyFlatFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "conf.yaml")

	content := []byte("GHORG_SCM_TYPE: github\nGHORG_CLONE_PROTOCOL: ssh\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// ReadConfigValue should find legacy keys via env var lookup
	val, found, err := ReadConfigValue(path, "scm.type")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if !found {
		t.Fatal("expected to find scm.type via legacy key")
	}
	if val != "github" {
		t.Errorf("expected 'github', got %q", val)
	}
}

func TestListConfigValuesTranslatesLegacyKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "conf.yaml")

	content := []byte("GHORG_SCM_TYPE: github\nGHORG_CLONE_PROTOCOL: ssh\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	values, err := ListConfigValues(path)
	if err != nil {
		t.Fatalf("ListConfigValues failed: %v", err)
	}

	if values["scm.type"] != "github" {
		t.Errorf("expected scm.type=github, got %q", values["scm.type"])
	}
	if values["clone.protocol"] != "ssh" {
		t.Errorf("expected clone.protocol=ssh, got %q", values["clone.protocol"])
	}
}

func TestFormatConfigListRedactsSecrets(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"scm.type":     "github",
		"github.token": "secret123",
	}

	output := FormatConfigList(values, false)
	if !contains(output, "github.token=********") {
		t.Errorf("expected token to be redacted, got:\n%s", output)
	}
	if !contains(output, "scm.type=github") {
		t.Errorf("expected scm.type=github in output, got:\n%s", output)
	}

	// With showSecrets=true
	output = FormatConfigList(values, true)
	if !contains(output, "github.token=secret123") {
		t.Errorf("expected token to be shown, got:\n%s", output)
	}
}

func TestReadConfigFileNotExist(t *testing.T) {
	t.Parallel()
	_, err := ReadConfigFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestWriteCreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	// Create the parent dir (WriteConfigValue doesn't create parent dirs)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if err := WriteConfigValue(path, "scm.type", "gitea"); err != nil {
		t.Fatalf("WriteConfigValue failed: %v", err)
	}

	val, found, err := ReadConfigValue(path, "scm.type")
	if err != nil {
		t.Fatalf("ReadConfigValue failed: %v", err)
	}
	if !found || val != "gitea" {
		t.Errorf("expected 'gitea', got %q (found=%v)", val, found)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
