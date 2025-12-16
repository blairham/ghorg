package cmd

import (
	"embed"
	"fmt"

	gtm "github.com/MichaelMure/go-term-markdown"
	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/mitchellh/cli"
)

var (
	//go:embed examples-copy/*
	examples embed.FS
)

type ExamplesCommand struct {
	UI cli.Ui
}

func (c *ExamplesCommand) Help() string {
	return `Usage: ghorg examples [github|gitlab|bitbucket|gitea|sourcehut]

Get documentation and examples for each SCM provider in the terminal.

Examples:
  ghorg examples github
  ghorg examples gitlab
  ghorg examples bitbucket
`
}

func (c *ExamplesCommand) Synopsis() string {
	return "Documentation and examples for each SCM provider"
}

func (c *ExamplesCommand) Run(args []string) int {
	if len(args) == 0 {
		colorlog.PrintErrorAndExit("Please additionally provide a SCM provider: github, gitlab, bitbucket, gitea, or sourcehut")
	}

	// TODO: fix the examples-copy directory mess
	filePath := fmt.Sprintf("examples-copy/%s.md", args[0])
	input := getFileContents(filePath)
	result := gtm.Render(string(input), 80, 6)
	fmt.Println(string(result))

	return 0
}

func getFileContents(filepath string) []byte {

	contents, err := examples.ReadFile(filepath)
	if err != nil {
		colorlog.PrintErrorAndExit("Only supported SCM providers are available for examples, please use one of the following: github, gitlab, bitbucket, gitea, or sourcehut")
	}

	return contents

}
