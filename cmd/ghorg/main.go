package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/cli"

	"github.com/blairham/ghorg/internal/cmd"
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
