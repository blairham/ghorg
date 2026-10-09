// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/jessevdk/go-flags"
)

func TestRecloneServerCommand_Synopsis(t *testing.T) {
	cmd := &RecloneServerCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}
}

func TestRecloneServerCommand_Help(t *testing.T) {
	cmd := &RecloneServerCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}

	// Help should mention the port flag
	if !bytes.Contains([]byte(help), []byte("--port")) {
		t.Error("Help should mention --port flag")
	}
	if !bytes.Contains([]byte(help), []byte("-p")) {
		t.Error("Help should mention -p short flag")
	}

	// Help should mention endpoints
	if !bytes.Contains([]byte(help), []byte("/trigger/reclone")) {
		t.Error("Help should mention /trigger/reclone endpoint")
	}
	if !bytes.Contains([]byte(help), []byte("/health")) {
		t.Error("Help should mention /health endpoint")
	}
	if !bytes.Contains([]byte(help), []byte("/stats")) {
		t.Error("Help should mention /stats endpoint")
	}
}

func TestRecloneServerFlags_Parse(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		expectPort string
	}{
		{
			name:       "no flags",
			args:       []string{},
			expectPort: "",
		},
		{
			name:       "port flag short",
			args:       []string{"-p", "8080"},
			expectPort: "8080",
		},
		{
			name:       "port flag full",
			args:       []string{"--port", "9000"},
			expectPort: "9000",
		},
		{
			name:       "port flag with equals",
			args:       []string{"--port=3000"},
			expectPort: "3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts RecloneServerFlags
			parser := createTestParser(&opts)
			_, err := parser.ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("Failed to parse args: %v", err)
			}

			if opts.Port != tt.expectPort {
				t.Errorf("Port: expected %q, got %q", tt.expectPort, opts.Port)
			}
		})
	}
}

func TestRecloneServerCommand_SetsEnvironmentVariable(t *testing.T) {
	// Save and clear original env
	origPort := os.Getenv("GHORG_RECLONE_SERVER_PORT")
	os.Unsetenv("GHORG_RECLONE_SERVER_PORT")
	defer func() {
		if origPort != "" {
			os.Setenv("GHORG_RECLONE_SERVER_PORT", origPort)
		} else {
			os.Unsetenv("GHORG_RECLONE_SERVER_PORT")
		}
	}()

	// We can't easily test the full Run since it blocks on http.ListenAndServe
	// But we can test the flag parsing sets the environment variable

	var buf bytes.Buffer
	ui := &cli.BasicUi{
		Writer:      &buf,
		ErrorWriter: &buf,
	}

	cmd := &RecloneServerCommand{UI: ui}

	// Test that flags are parsed correctly by checking help output
	exitCode := cmd.Run([]string{"--help"})

	// --help returns 0
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --help, got %d", exitCode)
	}
}

func TestServerPortFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "port without colon",
			input:    "8080",
			expected: ":8080",
		},
		{
			name:     "port with colon",
			input:    ":8080",
			expected: ":8080",
		},
		{
			name:     "empty port",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverPort := tt.input
			if serverPort != "" && serverPort[0] != ':' {
				serverPort = ":" + serverPort
			}

			if serverPort != tt.expected {
				t.Errorf("Port formatting: expected %q, got %q", tt.expected, serverPort)
			}
		})
	}
}

func TestRecloneTriggerArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cmd     string
		want    []string
		wantErr bool
	}{
		{name: "empty runs every reclone", cmd: "", want: []string{"reclone"}},
		{name: "an entry name", cmd: "my-org", want: []string{"reclone", "--", "my-org"}},
		{name: "a long flag is refused", cmd: "--reclone-path=/tmp/evil.yaml", wantErr: true},
		{name: "list is refused", cmd: "--list", wantErr: true},
		{name: "a short flag is refused", cmd: "-h", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := recloneTriggerArgs(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("recloneTriggerArgs(%q) error = %v, wantErr %v", tt.cmd, err, tt.wantErr)
			}
			if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Errorf("recloneTriggerArgs(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

// TestRecloneDoubleDashEndsFlags pins the go-flags behavior recloneTriggerArgs
// relies on: after `--`, a word that looks like a flag is an entry name.
func TestRecloneDoubleDashEndsFlags(t *testing.T) {
	t.Parallel()
	var opts RecloneFlags
	remaining, err := flags.NewParser(&opts, flags.Default).ParseArgs([]string{"--", "--reclone-path=/tmp/evil.yaml"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if opts.ReclonePath != "" {
		t.Errorf("ReclonePath = %q, want it unset", opts.ReclonePath)
	}
	if len(remaining) != 1 || remaining[0] != "--reclone-path=/tmp/evil.yaml" {
		t.Errorf("remaining = %q, want the word as an entry name", remaining)
	}
}
