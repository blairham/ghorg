package cmd

import (
	"testing"
)

func TestCommandFactory(t *testing.T) {
	commands := CommandFactory()

	expectedCommands := []string{
		"version",
		"clone",
		"reclone",
		"reclone-cron",
		"reclone-server",
		"ls",
	}

	for _, cmdName := range expectedCommands {
		t.Run(cmdName, func(t *testing.T) {
			factory, exists := commands[cmdName]
			if !exists {
				t.Errorf("Command %q not found in CommandFactory", cmdName)
				return
			}

			cmd, err := factory()
			if err != nil {
				t.Errorf("CommandFactory for %q returned error: %v", cmdName, err)
				return
			}

			if cmd == nil {
				t.Errorf("CommandFactory for %q returned nil command", cmdName)
				return
			}

			// Test that Synopsis returns non-empty string
			synopsis := cmd.Synopsis()
			if synopsis == "" {
				t.Errorf("Command %q has empty Synopsis", cmdName)
			}

			// Test that Help returns non-empty string
			help := cmd.Help()
			if help == "" {
				t.Errorf("Command %q has empty Help", cmdName)
			}
		})
	}
}

func TestCommandFactoryCount(t *testing.T) {
	commands := CommandFactory()

	expectedCount := 6
	if len(commands) != expectedCount {
		t.Errorf("Expected %d commands, got %d", expectedCount, len(commands))
	}
}

func TestCommandFactoryReturnsNewInstances(t *testing.T) {
	commands := CommandFactory()

	// Test that factory creates new instances each time
	factory := commands["version"]

	cmd1, err1 := factory()
	cmd2, err2 := factory()

	if err1 != nil || err2 != nil {
		t.Fatalf("Factory returned errors: %v, %v", err1, err2)
	}

	// Commands should be different instances
	if cmd1 == cmd2 {
		t.Error("Factory should return new instances each time")
	}
}
