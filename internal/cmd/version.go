package cmd

import (
	"fmt"

	"github.com/mitchellh/cli"
)

const ghorgVersion = "v1.11.8"

type VersionCommand struct {
	UI cli.Ui
}

func (c *VersionCommand) Help() string {
	return "Print the version number of Ghorg"
}

func (c *VersionCommand) Synopsis() string {
	return "Print the version number of Ghorg"
}

func (c *VersionCommand) Run(args []string) int {
	fmt.Println(ghorgVersion)
	return 0
}

func PrintVersion() {
	fmt.Println(ghorgVersion)
}

func GetVersion() string {
	return ghorgVersion
}
