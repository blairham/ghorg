package scm

import (
	"sort"
	"strings"
	"testing"
)

func TestGetClient_InvalidType(t *testing.T) {
	t.Parallel()
	_, err := GetClient("nonexistent")
	if err == nil {
		t.Fatal("expected error for unsupported client type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error message, got: %v", err)
	}
}

func TestGetClient_ValidType(t *testing.T) {
	t.Parallel()
	// GetClient("github") will call NewClient() which needs a token,
	// so it will return an error — but NOT an "unsupported" error.
	_, err := GetClient("github")
	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Errorf("github client should be registered, got unsupported error: %v", err)
	}
}

func TestSupportedClients(t *testing.T) {
	t.Parallel()
	supported := SupportedClients()

	expected := []string{"github", "gitlab", "gitea", "bitbucket", "sourcehut"}
	sort.Strings(expected)

	got := make([]string, len(supported))
	copy(got, supported)
	sort.Strings(got)

	if len(got) != len(expected) {
		t.Fatalf("expected %d supported clients, got %d: %v", len(expected), len(got), got)
	}

	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("expected client %q at position %d, got %q", exp, i, got[i])
		}
	}
}
