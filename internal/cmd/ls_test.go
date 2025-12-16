package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

func TestLsCommand_Synopsis(t *testing.T) {
	cmd := &LsCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}
}

func TestLsCommand_Help(t *testing.T) {
	cmd := &LsCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}

	// Help should mention the flags
	if !bytes.Contains([]byte(help), []byte("--long")) {
		t.Error("Help should mention --long flag")
	}
	if !bytes.Contains([]byte(help), []byte("--total")) {
		t.Error("Help should mention --total flag")
	}
}

func TestLsFlags_Parse(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectLong  bool
		expectTotal bool
	}{
		{
			name:        "no flags",
			args:        []string{},
			expectLong:  false,
			expectTotal: false,
		},
		{
			name:        "long flag short",
			args:        []string{"-l"},
			expectLong:  true,
			expectTotal: false,
		},
		{
			name:        "long flag full",
			args:        []string{"--long"},
			expectLong:  true,
			expectTotal: false,
		},
		{
			name:        "total flag short",
			args:        []string{"-t"},
			expectLong:  false,
			expectTotal: true,
		},
		{
			name:        "total flag full",
			args:        []string{"--total"},
			expectLong:  false,
			expectTotal: true,
		},
		{
			name:        "both flags",
			args:        []string{"-l", "-t"},
			expectLong:  true,
			expectTotal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts LsFlags
			parser := createTestParser(&opts)
			_, err := parser.ParseArgs(tt.args)

			if err != nil {
				t.Fatalf("Failed to parse args: %v", err)
			}

			if opts.Long != tt.expectLong {
				t.Errorf("Long flag: expected %v, got %v", tt.expectLong, opts.Long)
			}
			if opts.Total != tt.expectTotal {
				t.Errorf("Total flag: expected %v, got %v", tt.expectTotal, opts.Total)
			}
		})
	}
}

func TestLsCommand_RunWithTempDir(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	orgDir := filepath.Join(tmpDir, "test-org")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create some fake repo directories
	repos := []string{"repo1", "repo2", "repo3"}
	for _, repo := range repos {
		repoPath := filepath.Join(orgDir, repo)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}
	}

	// Set environment variable
	origPath := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", tmpDir)
	defer func() {
		if origPath != "" {
			os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", origPath)
		} else {
			os.Unsetenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
		}
	}()

	var buf bytes.Buffer
	ui := &cli.BasicUi{
		Writer:      &buf,
		ErrorWriter: &buf,
	}

	cmd := &LsCommand{UI: ui}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestLsCommand_RunWithSpecificDir(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	orgDir := filepath.Join(tmpDir, "test-org")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create some fake repo directories
	repos := []string{"repo1", "repo2"}
	for _, repo := range repos {
		repoPath := filepath.Join(orgDir, repo)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}
	}

	// Set environment variable
	origPath := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
	os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", tmpDir)
	defer func() {
		if origPath != "" {
			os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", origPath)
		} else {
			os.Unsetenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
		}
	}()

	var buf bytes.Buffer
	ui := &cli.BasicUi{
		Writer:      &buf,
		ErrorWriter: &buf,
	}

	cmd := &LsCommand{UI: ui}
	exitCode := cmd.Run([]string{"test-org"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

// Helper function to create a test parser
func createTestParser(opts interface{}) *flags.Parser {
	return flags.NewParser(opts, flags.Default&^flags.PrintErrors)
}
