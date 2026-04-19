package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/jessevdk/go-flags"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/configs"
)

type InitCommand struct {
	UI cli.Ui
}

type initFlags struct {
	DryRun bool `long:"dry-run" description:"Preview the config without writing to disk"`
	Force  bool `long:"force" description:"Overwrite existing config file"`
}

func (c *InitCommand) Help() string {
	return `Usage: ghorg init [options]

Interactive setup wizard for ghorg. Prompts you to select your SCM provider,
enter your token, and configure your clone path.

Options:
  --dry-run    Preview the config without writing to disk
  --force      Overwrite existing config file`
}

func (c *InitCommand) Synopsis() string {
	return "Interactive setup wizard for ghorg configuration"
}

func (c *InitCommand) Run(args []string) int {
	var opts initFlags
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			fmt.Println(c.Help())
			return 0
		}
		colorlog.PrintError(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}

	confDir := configs.GhorgConfDir()
	confFile := configs.DefaultConfFile()

	if !opts.Force {
		if _, err := os.Stat(confFile); err == nil {
			colorlog.PrintError(fmt.Sprintf("Config file already exists: %s", confFile))
			colorlog.PrintInfo("Use --force to overwrite, or use 'ghorg config' to edit values")
			return 1
		}
	}

	reader := bufio.NewReader(os.Stdin)

	scmType := promptSelect(reader, "SCM provider", []string{"github", "gitlab", "gitea", "bitbucket", "sourcehut"}, "github")

	var token string
	if scmType == "github" {
		if ghTokenAvailable() {
			colorlog.PrintSuccess("GitHub token will be detected automatically via gh CLI at runtime")
		} else {
			colorlog.PrintInfo("Tip: install the gh CLI (https://cli.github.com) and run 'gh auth login' to skip token management")
			token = promptString(reader, "GitHub personal access token (leave empty to skip)", "")
		}
	} else {
		token = promptString(reader, fmt.Sprintf("Personal access token for %s (leave empty to skip)", scmType), "")
	}

	protocol := promptSelect(reader, "Clone protocol", []string{"ssh", "https"}, "ssh")

	defaultPath := filepath.Join(configs.HomeDir(), "ghorg")
	clonePath := promptString(reader, "Absolute path for cloned repos", defaultPath)

	// Build the config values
	values := []struct {
		key, value string
	}{
		{"scm.type", scmType},
		{"clone.protocol", protocol},
		{"core.path", clonePath},
	}

	tokenKey := scmTokenKey(scmType)
	if token != "" && tokenKey != "" {
		values = append(values, struct{ key, value string }{tokenKey, token})
	}

	if opts.DryRun {
		fmt.Println()
		colorlog.PrintInfo("Dry run — config would be written to: " + confFile)
		fmt.Println()
		for _, v := range values {
			display := v.value
			if strings.Contains(v.key, "token") || strings.Contains(v.key, "password") {
				display = "********"
			}
			fmt.Printf("  %s = %s\n", v.key, display)
		}
		return 0
	}

	if err := os.MkdirAll(confDir, 0o755); err != nil {
		colorlog.PrintError(fmt.Sprintf("Failed to create config directory: %v", err))
		return 1
	}

	for _, v := range values {
		if err := configs.WriteConfigValue(confFile, v.key, v.value); err != nil {
			colorlog.PrintError(fmt.Sprintf("Failed to write config: %v", err))
			return 1
		}
	}

	fmt.Println()
	colorlog.PrintSuccess("Config written to: " + confFile)
	colorlog.PrintInfo("Run 'ghorg clone <org>' to get started")
	return 0
}

func promptSelect(reader *bufio.Reader, label string, options []string, defaultVal string) string {
	fmt.Printf("%s (%s) [%s]: ", label, strings.Join(options, ", "), defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	for _, o := range options {
		if strings.EqualFold(input, o) {
			return o
		}
	}
	fmt.Printf("  Invalid choice, using default: %s\n", defaultVal)
	return defaultVal
}

func promptString(reader *bufio.Reader, label string, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func ghTokenAvailable() bool {
	out, err := exec.Command("gh", "auth", "token").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func scmTokenKey(scmType string) string {
	switch scmType {
	case "github":
		return "auth.github.token"
	case "gitlab":
		return "auth.gitlab.token"
	case "gitea":
		return "auth.gitea.token"
	case "bitbucket":
		return "auth.bitbucket.app-password"
	case "sourcehut":
		return "auth.sourcehut.token"
	default:
		return ""
	}
}
