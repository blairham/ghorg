package main

import (
	"fmt"
	"os"

	"github.com/blairham/ghorg/cmd"
	"github.com/mitchellh/cli"
)

func main() {
	c := cli.NewCLI("ghorg", cmd.GetVersion())
	c.Args = os.Args[1:]
	c.Commands = cmd.CommandFactory()

	exitStatus, err := c.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	os.Exit(exitStatus)
}
