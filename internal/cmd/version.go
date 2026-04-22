package cmd

import (
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/hashicorp/cli"
)

// version is set via ldflags at build time by goreleaser or make build-local.
// When not set (e.g., go install), it falls back to Go build info then git SHA.
var version string //nolint:gochecknoglobals // set via ldflags

func init() { //nolint:gochecknoinits // populate version from build info when ldflags are absent
	if version != "" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

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
	fmt.Println(GetVersion())
	return 0
}

func PrintVersion() {
	fmt.Println(GetVersion())
}

func GetVersion() string {
	if version != "" {
		return version
	}
	return gitSHA()
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}
