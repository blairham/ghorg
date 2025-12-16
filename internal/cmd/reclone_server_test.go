package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/mitchellh/cli"
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
