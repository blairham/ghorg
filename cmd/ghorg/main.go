package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/cli"
	"github.com/jessevdk/go-flags"

	"github.com/blairham/ghorg/internal/cmd"
)

// GlobalFlags holds flags that apply to all commands
type GlobalFlags struct {
	Color bool `long:"color" description:"Enable colorful output"`
}

func main() {
	// Parse global flags first, passing remaining args to hashicorp/cli
	var globalOpts GlobalFlags
	parser := flags.NewParser(&globalOpts, flags.IgnoreUnknown)
	remaining, _ := parser.ParseArgs(os.Args[1:])

	if globalOpts.Color {
		os.Setenv("GHORG_COLOR", "enabled")
	}

	c := cli.NewCLI("ghorg", cmd.GetVersion())
	c.Args = remaining
	c.Commands = cmd.CommandFactory()

	exitStatus, err := c.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	os.Exit(exitStatus)
}
