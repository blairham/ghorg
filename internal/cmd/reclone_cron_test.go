package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/mitchellh/cli"
)

func TestRecloneCronCommand_Synopsis(t *testing.T) {
	cmd := &RecloneCronCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}
}

func TestRecloneCronCommand_Help(t *testing.T) {
	cmd := &RecloneCronCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}

	// Help should mention the minutes flag
	if !bytes.Contains([]byte(help), []byte("--minutes")) {
		t.Error("Help should mention --minutes flag")
	}
	if !bytes.Contains([]byte(help), []byte("-m")) {
		t.Error("Help should mention -m short flag")
	}
}

func TestRecloneCronFlags_Parse(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectMinutes string
	}{
		{
			name:          "no flags",
			args:          []string{},
			expectMinutes: "",
		},
		{
			name:          "minutes flag short",
			args:          []string{"-m", "30"},
			expectMinutes: "30",
		},
		{
			name:          "minutes flag full",
			args:          []string{"--minutes", "60"},
			expectMinutes: "60",
		},
		{
			name:          "minutes flag with equals",
			args:          []string{"--minutes=45"},
			expectMinutes: "45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts RecloneCronFlags
			parser := createTestParser(&opts)
			_, err := parser.ParseArgs(tt.args)

			if err != nil {
				t.Fatalf("Failed to parse args: %v", err)
			}

			if opts.Minutes != tt.expectMinutes {
				t.Errorf("Minutes: expected %q, got %q", tt.expectMinutes, opts.Minutes)
			}
		})
	}
}

func TestRecloneCronCommand_SetsEnvironmentVariable(t *testing.T) {
	// Save and clear original env
	origMinutes := os.Getenv("GHORG_CRON_TIMER_MINUTES")
	os.Unsetenv("GHORG_CRON_TIMER_MINUTES")
	defer func() {
		if origMinutes != "" {
			os.Setenv("GHORG_CRON_TIMER_MINUTES", origMinutes)
		} else {
			os.Unsetenv("GHORG_CRON_TIMER_MINUTES")
		}
	}()

	var buf bytes.Buffer
	ui := &cli.BasicUi{
		Writer:      &buf,
		ErrorWriter: &buf,
	}

	cmd := &RecloneCronCommand{UI: ui}

	// Run in a goroutine since it blocks, but we're testing flag parsing
	// The command will return immediately if GHORG_CRON_TIMER_MINUTES is not set
	// after flag parsing, since we're not setting --minutes
	exitCode := cmd.Run([]string{})

	// Without minutes flag, the cron won't start but should return 0
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestStartReCloneCron_NoTimerSet(t *testing.T) {
	// Save and clear original env
	origMinutes := os.Getenv("GHORG_CRON_TIMER_MINUTES")
	os.Unsetenv("GHORG_CRON_TIMER_MINUTES")
	defer func() {
		if origMinutes != "" {
			os.Setenv("GHORG_CRON_TIMER_MINUTES", origMinutes)
		} else {
			os.Unsetenv("GHORG_CRON_TIMER_MINUTES")
		}
	}()

	// This should return immediately without starting a cron
	// since GHORG_CRON_TIMER_MINUTES is not set
	startReCloneCron()

	// If we reach here, the function returned properly
	// (it would block forever if it started the cron)
}

func TestStartReCloneCron_InvalidTimer(t *testing.T) {
	// Save and clear original env
	origMinutes := os.Getenv("GHORG_CRON_TIMER_MINUTES")
	os.Setenv("GHORG_CRON_TIMER_MINUTES", "invalid")
	defer func() {
		if origMinutes != "" {
			os.Setenv("GHORG_CRON_TIMER_MINUTES", origMinutes)
		} else {
			os.Unsetenv("GHORG_CRON_TIMER_MINUTES")
		}
	}()

	// This should return immediately with an error message
	// since the timer value is invalid
	startReCloneCron()

	// If we reach here, the function handled the invalid value properly
}
