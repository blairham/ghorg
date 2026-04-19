package configs

import (
	"strings"
	"testing"
)

func TestAllKeysHaveRequiredFields(t *testing.T) {
	t.Parallel()
	for _, key := range AllKeys {
		t.Run(key.DotNotation, func(t *testing.T) {
			t.Parallel()
			if key.DotNotation == "" {
				t.Error("DotNotation must not be empty")
			}
			if key.EnvVar == "" {
				t.Errorf("EnvVar must not be empty for key %s", key.DotNotation)
			}
			if key.Description == "" {
				t.Errorf("Description must not be empty for key %s", key.DotNotation)
			}
			if !strings.HasPrefix(key.EnvVar, "GHORG_") {
				t.Errorf("EnvVar %q should start with GHORG_", key.EnvVar)
			}
			if !strings.Contains(key.DotNotation, ".") {
				t.Errorf("DotNotation %q should contain a dot separator", key.DotNotation)
			}
		})
	}
}

func TestNoDuplicateDotNotation(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, key := range AllKeys {
		if seen[key.DotNotation] {
			t.Errorf("duplicate DotNotation: %s", key.DotNotation)
		}
		seen[key.DotNotation] = true
	}
}

func TestNoDuplicateEnvVar(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, key := range AllKeys {
		if seen[key.EnvVar] {
			t.Errorf("duplicate EnvVar: %s", key.EnvVar)
		}
		seen[key.EnvVar] = true
	}
}

func TestLookupByDot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dot    string
		envVar string
	}{
		{"scm.type", "GHORG_SCM_TYPE"},
		{"clone.protocol", "GHORG_CLONE_PROTOCOL"},
		{"github.token", "GHORG_GITHUB_TOKEN"},
		{"prune.enabled", "GHORG_PRUNE"},
	}
	for _, tt := range tests {
		t.Run(tt.dot, func(t *testing.T) {
			t.Parallel()
			key := LookupByDot(tt.dot)
			if key == nil {
				t.Fatalf("LookupByDot(%q) returned nil", tt.dot)
			}
			if key.EnvVar != tt.envVar {
				t.Errorf("expected EnvVar %q, got %q", tt.envVar, key.EnvVar)
			}
		})
	}
}

func TestLookupByDotNotFound(t *testing.T) {
	t.Parallel()
	if LookupByDot("nonexistent.key") != nil {
		t.Error("expected nil for unknown key")
	}
}

func TestLookupByEnvVar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		envVar string
		dot    string
	}{
		{"GHORG_SCM_TYPE", "scm.type"},
		{"GHORG_CLONE_PROTOCOL", "clone.protocol"},
		{"GHORG_GITHUB_TOKEN", "github.token"},
	}
	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			t.Parallel()
			key := LookupByEnvVar(tt.envVar)
			if key == nil {
				t.Fatalf("LookupByEnvVar(%q) returned nil", tt.envVar)
			}
			if key.DotNotation != tt.dot {
				t.Errorf("expected DotNotation %q, got %q", tt.dot, key.DotNotation)
			}
		})
	}
}

func TestDotToEnvVar(t *testing.T) {
	t.Parallel()
	if got := DotToEnvVar("scm.type"); got != "GHORG_SCM_TYPE" {
		t.Errorf("expected GHORG_SCM_TYPE, got %q", got)
	}
	if got := DotToEnvVar("nonexistent"); got != "" {
		t.Errorf("expected empty string for unknown key, got %q", got)
	}
}

func TestEnvVarToDot(t *testing.T) {
	t.Parallel()
	if got := EnvVarToDot("GHORG_SCM_TYPE"); got != "scm.type" {
		t.Errorf("expected scm.type, got %q", got)
	}
	if got := EnvVarToDot("NONEXISTENT"); got != "" {
		t.Errorf("expected empty string for unknown env var, got %q", got)
	}
}

func TestConfigKeySection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dot     string
		section string
	}{
		{"scm.type", "scm"},
		{"clone.protocol", "clone"},
		{"github.app-pem-path", "github"},
		{"exit-code.clone-infos", "exit-code"},
	}
	for _, tt := range tests {
		t.Run(tt.dot, func(t *testing.T) {
			t.Parallel()
			key := LookupByDot(tt.dot)
			if key == nil {
				t.Fatalf("key %q not found", tt.dot)
			}
			if got := key.Section(); got != tt.section {
				t.Errorf("expected section %q, got %q", tt.section, got)
			}
		})
	}
}

func TestConfigKeyYAMLPath(t *testing.T) {
	t.Parallel()
	key := LookupByDot("github.app-pem-path")
	if key == nil {
		t.Fatal("key not found")
	}
	path := key.YAMLPath()
	if len(path) != 2 || path[0] != "github" || path[1] != "app-pem-path" {
		t.Errorf("expected [github app-pem-path], got %v", path)
	}
}

func TestSections(t *testing.T) {
	t.Parallel()
	sections := Sections()
	if len(sections) == 0 {
		t.Fatal("expected at least one section")
	}
	// Verify no duplicates
	seen := make(map[string]bool)
	for _, s := range sections {
		if seen[s] {
			t.Errorf("duplicate section: %s", s)
		}
		seen[s] = true
	}
	// Verify expected sections exist
	expected := []string{"core", "scm", "clone", "git", "filter", "github", "gitlab", "bitbucket", "gitea", "sourcehut"}
	for _, e := range expected {
		if !seen[e] {
			t.Errorf("missing expected section: %s", e)
		}
	}
}

func TestSecretKeysAreMarked(t *testing.T) {
	t.Parallel()
	secretEnvVars := []string{
		"GHORG_GITHUB_TOKEN",
		"GHORG_GITLAB_TOKEN",
		"GHORG_GITEA_TOKEN",
		"GHORG_SOURCEHUT_TOKEN",
		"GHORG_BITBUCKET_APP_PASSWORD",
		"GHORG_BITBUCKET_OAUTH_TOKEN",
		"GHORG_BITBUCKET_API_TOKEN",
	}
	for _, env := range secretEnvVars {
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			key := LookupByEnvVar(env)
			if key == nil {
				t.Fatalf("key not found for %s", env)
			}
			if !key.IsSecret {
				t.Errorf("expected %s to be marked as secret", env)
			}
		})
	}
}

func TestBoolKeysHaveCorrectDefaults(t *testing.T) {
	t.Parallel()
	for _, key := range AllKeys {
		if key.IsBool && key.DefaultValue != "" {
			t.Run(key.DotNotation, func(t *testing.T) {
				t.Parallel()
				if key.DefaultValue != "true" && key.DefaultValue != "false" {
					t.Errorf("bool key %s has non-boolean default: %q", key.DotNotation, key.DefaultValue)
				}
			})
		}
	}
}
