package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestVersionCommand tests that the version command works
func TestVersionCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v, output: %s", err, output)
	}

	// Version output should contain a version number pattern
	if len(output) == 0 {
		t.Error("version command produced no output")
	}
}

// TestHelpCommand tests that the help command works
func TestHelpCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	// --help may exit with status 0 or 1 depending on implementation
	// We just want to make sure it produces help output
	_ = err

	outputStr := string(output)
	if !strings.Contains(outputStr, "clone") {
		t.Errorf("help output should contain 'clone' command, got: %s", outputStr)
	}
}

// TestUnknownCommand tests that unknown commands return an error
func TestUnknownCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "unknowncommand123")
	cmd.Dir = "."

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for unknown command, got nil")
	}
}

// TestCloneHelpCommand tests that clone --help works
func TestCloneHelpCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "clone", "--help")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	_ = err // --help may return non-zero exit code

	outputStr := string(output)
	// Should contain clone-related flags
	if !strings.Contains(outputStr, "token") && !strings.Contains(outputStr, "protocol") {
		t.Errorf("clone help should contain flag information, got: %s", outputStr)
	}
}

// TestLsHelpCommand tests that ls --help works
func TestLsHelpCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "ls", "--help")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	_ = err // --help may return non-zero exit code

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("ls help command produced no output")
	}
}

// TestRecloneHelpCommand tests that reclone --help works
func TestRecloneHelpCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "reclone", "--help")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	_ = err // --help may return non-zero exit code

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("reclone help command produced no output")
	}
}

// TestExamplesCommand tests that the examples command works
func TestExamplesCommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "examples")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		// examples might require input, so just check it doesn't crash immediately
		outputStr := string(output)
		if strings.Contains(outputStr, "panic") {
			t.Fatalf("examples command panicked: %s", outputStr)
		}
	}
}

// TestEnvironmentVariables tests that environment variables are read
func TestEnvironmentVariables(t *testing.T) {
	// Set a test environment variable
	os.Setenv("GHORG_SCM_TYPE", "gitlab")
	defer os.Unsetenv("GHORG_SCM_TYPE")

	cmd := exec.Command("go", "run", ".", "clone", "--help")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GHORG_SCM_TYPE=gitlab")

	output, err := cmd.CombinedOutput()
	_ = err // --help returns non-zero

	// Should not panic or error out
	outputStr := string(output)
	if strings.Contains(outputStr, "panic") {
		t.Errorf("command panicked with env var set: %s", outputStr)
	}
}
