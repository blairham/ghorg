package cmd

import (
	"bytes"
	"testing"

	"github.com/hashicorp/cli"
)

func TestVersionCommand_Run(t *testing.T) {
	var buf bytes.Buffer
	ui := &cli.BasicUi{
		Writer: &buf,
	}

	cmd := &VersionCommand{UI: ui}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestVersionCommand_Synopsis(t *testing.T) {
	cmd := &VersionCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}
}

func TestVersionCommand_Help(t *testing.T) {
	cmd := &VersionCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}
}

func TestGetVersion(t *testing.T) {
	v := GetVersion()

	if v == "" {
		t.Error("GetVersion should return a non-empty string")
	}
}

func TestGetVersion_WithLdflags(t *testing.T) {
	// When version is set via ldflags, GetVersion returns it directly
	old := version
	version = "v2.0.0"
	defer func() { version = old }()

	if GetVersion() != "v2.0.0" {
		t.Errorf("Expected 'v2.0.0', got %q", GetVersion())
	}
}

func TestGetVersion_FallbackToGitSHA(t *testing.T) {
	// When version is empty, GetVersion falls back to git SHA
	old := version
	version = ""
	defer func() { version = old }()

	v := GetVersion()
	if v == "" {
		t.Error("GetVersion should fall back to git SHA or 'dev'")
	}
}
