package cmd

import (
	"bytes"
	"testing"

	"github.com/mitchellh/cli"
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
	version := GetVersion()

	if version == "" {
		t.Error("GetVersion should return a non-empty string")
	}

	// Version should start with 'v'
	if version[0] != 'v' {
		t.Errorf("Version should start with 'v', got %q", version)
	}
}

func TestVersionConstant(t *testing.T) {
	// Test that the version constant is set
	if ghorgVersion == "" {
		t.Error("ghorgVersion constant should not be empty")
	}
}
