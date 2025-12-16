package cmd

import (
	"os"

	"github.com/mitchellh/cli"
)

// CommandFactory creates CLI commands
func CommandFactory() map[string]cli.CommandFactory {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	return map[string]cli.CommandFactory{
		"version": func() (cli.Command, error) {
			return &VersionCommand{
				UI: ui,
			}, nil
		},
		"clone": func() (cli.Command, error) {
			return &CloneCommand{
				UI: ui,
			}, nil
		},
		"reclone": func() (cli.Command, error) {
			return &RecloneCommand{
				UI: ui,
			}, nil
		},
		"reclone-cron": func() (cli.Command, error) {
			return &RecloneCronCommand{
				UI: ui,
			}, nil
		},
		"reclone-server": func() (cli.Command, error) {
			return &RecloneServerCommand{
				UI: ui,
			}, nil
		},
		"ls": func() (cli.Command, error) {
			return &LsCommand{
				UI: ui,
			}, nil
		},
		"examples": func() (cli.Command, error) {
			return &ExamplesCommand{
				UI: ui,
			}, nil
		},
	}
}
