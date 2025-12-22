package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/configs"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v2"
)

type RecloneCommand struct {
	UI cli.Ui
}

type RecloneFlags struct {
	ReclonePath   string `long:"reclone-path" description:"GHORG_RECLONE_PATH - If you want to set a path other than $HOME/.config/ghorg/reclone.yaml for your reclone configuration"`
	Quiet         bool   `long:"quiet" description:"GHORG_RECLONE_QUIET - Quiet logging output"`
	List          bool   `long:"list" description:"Prints reclone commands and optional descriptions to stdout then will exit 0. Does not obsfucate tokens, and is only available as a commandline argument"`
	EnvConfigOnly bool   `long:"env-config-only" description:"GHORG_RECLONE_ENV_CONFIG_ONLY - Only use environment variables to set the configuration for all reclones"`
}

type ReClone struct {
	Cmd            string `yaml:"cmd"`
	Description    string `yaml:"description"`
	PostExecScript string `yaml:"post_exec_script"` // optional
}

func (c *RecloneCommand) Help() string {
	return `Usage: ghorg reclone [options] [reclone-keys...]

Reruns one, multiple, or all preconfigured clones from configuration set in $HOME/.config/ghorg/reclone.yaml.
Allows you to set preconfigured clone commands for handling multiple users/orgs at once.

Options:
  --reclone-path          Path to reclone.yaml configuration file
  --quiet                 Quiet logging output
  --list                  List available reclone commands
  --env-config-only       Only use environment variables for configuration

Examples:
  ghorg reclone                    # Run all configured reclones
  ghorg reclone my-org            # Run specific reclone
  ghorg reclone --list            # List all configured reclones

See https://github.com/blairham/ghorg#reclone-command for setup and additional information.
`
}

func (c *RecloneCommand) Synopsis() string {
	return "Reruns one, multiple, or all preconfigured clones"
}

func (c *RecloneCommand) Run(args []string) int {
	var opts RecloneFlags
	parser := flags.NewParser(&opts, flags.Default)
	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			fmt.Println(c.Help())
			return 0
		}
		colorlog.PrintError(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}

	if opts.ReclonePath != "" {
		os.Setenv("GHORG_RECLONE_PATH", opts.ReclonePath)
	}

	if opts.Quiet {
		os.Setenv("GHORG_RECLONE_QUIET", "true")
	}

	if opts.EnvConfigOnly {
		os.Setenv("GHORG_RECLONE_ENV_CONFIG_ONLY", "true")
	}

	path := configs.GhorgReCloneLocation()
	yamlBytes, err := os.ReadFile(path)
	if err != nil {
		colorlog.PrintErrorAndExit(fmt.Sprintf("ERROR: parsing reclone.yaml, error: %v", err))
	}

	mapOfReClones := make(map[string]ReClone)

	err = yaml.Unmarshal(yamlBytes, &mapOfReClones)
	if err != nil {
		colorlog.PrintErrorAndExit(fmt.Sprintf("ERROR: unmarshaling reclone.yaml, error:%v", err))
	}

	if opts.List {
		colorlog.PrintInfo("**************************************************************")
		colorlog.PrintInfo("**** Available reclone commands and optional descriptions ****")
		colorlog.PrintInfo("**************************************************************")
		fmt.Println("")
		for key, value := range mapOfReClones {
			colorlog.PrintInfo(fmt.Sprintf("- %s", key))
			if value.Description != "" {
				colorlog.PrintSubtleInfo(fmt.Sprintf("    description: %s", value.Description))
			}
			colorlog.PrintSubtleInfo(fmt.Sprintf("    cmd: %s", value.Cmd))
			fmt.Println("")
		}
		return 0
	}

	if len(remaining) == 0 {
		for rcIdentifier, reclone := range mapOfReClones {
			runReClone(reclone, rcIdentifier)
		}
	} else {
		for _, rcIdentifier := range remaining {
			if _, ok := mapOfReClones[rcIdentifier]; !ok {
				colorlog.PrintErrorAndExit(fmt.Sprintf("ERROR: The key %v was not found in reclone.yaml", rcIdentifier))
			} else {
				runReClone(mapOfReClones[rcIdentifier], rcIdentifier)
			}
		}
	}

	printFinalOutput(remaining, mapOfReClones)
	return 0
}

func isQuietReClone() bool {
	return os.Getenv("GHORG_RECLONE_QUIET") == "true"
}

func printFinalOutput(argz []string, reCloneMap map[string]ReClone) {
	fmt.Println("")
	colorlog.PrintSuccess("Completed! The following reclones were ran successfully...")
	if len(argz) == 0 {
		for key := range reCloneMap {
			colorlog.PrintSuccess(fmt.Sprintf("  * %v", key))
		}
	} else {
		for _, arg := range argz {
			colorlog.PrintSuccess(fmt.Sprintf("  * %v", arg))
		}
	}
}

func sanitizeCmd(cmd string) string {
	if strings.Contains(cmd, "-t=") {
		splitCmd := strings.Split(cmd, "-t=")
		firstHalf := splitCmd[0]
		secondHalf := strings.Split(splitCmd[1], " ")[1:]
		return firstHalf + "-t=XXXXXXX " + strings.Join(secondHalf, " ")
	}
	if strings.Contains(cmd, "-t ") {
		splitCmd := strings.Split(cmd, "-t ")
		firstHalf := splitCmd[0]
		secondHalf := strings.Split(splitCmd[1], " ")[1:]
		return firstHalf + "-t XXXXXXX " + strings.Join(secondHalf, " ")
	}
	if strings.Contains(cmd, "--token=") {
		splitCmd := strings.Split(cmd, "--token=")
		firstHalf := splitCmd[0]
		secondHalf := strings.Split(splitCmd[1], " ")[1:]
		return firstHalf + "--token=XXXXXXX " + strings.Join(secondHalf, " ")
	}
	if strings.Contains(cmd, "--token ") {
		splitCmd := strings.Split(cmd, "--token ")
		firstHalf := splitCmd[0]
		secondHalf := strings.Split(splitCmd[1], " ")[1:]
		return firstHalf + "--token XXXXXXX " + strings.Join(secondHalf, " ")
	}
	return cmd
}

func runReClone(rc ReClone, rcIdentifier string) {
	// make sure command starts with ghorg clone
	splitCommand := strings.Split(rc.Cmd, " ")
	ghorg, clone, remainingCommand := splitCommand[0], splitCommand[1], splitCommand[1:]

	if ghorg != "ghorg" || clone != "clone" {
		colorlog.PrintErrorAndExit("ERROR: Only ghorg clone commands are permitted in your reclone.yaml")
	}

	safeToLogCmd := sanitizeCmd(strings.Clone(rc.Cmd))

	if !isQuietReClone() {
		fmt.Println("")
		colorlog.PrintInfo(fmt.Sprintf("Running reclone: %v", rcIdentifier))
		if rc.Description != "" {
			colorlog.PrintInfo(fmt.Sprintf("Description: %v", rc.Description))
			fmt.Println("")
		}
		colorlog.PrintInfo(fmt.Sprintf("> %v", safeToLogCmd))
	}

	ghorgClone := exec.Command("ghorg", remainingCommand...)

	if os.Getenv("GHORG_CONFIG") == "none" {
		os.Setenv("GHORG_CONFIG", "")
	}

	os.Setenv("GHORG_RECLONE_RUNNING", "true")
	defer os.Setenv("GHORG_RECLONE_RUNNING", "false")

	if os.Getenv("GHORG_RECLONE_ENV_CONFIG_ONLY") == "false" {
		// have to unset all ghorg envs because root command will set them on initialization of ghorg cmd
		for _, e := range os.Environ() {
			keyValue := strings.SplitN(e, "=", 2)
			env := keyValue[0]
			ghorgEnv := strings.HasPrefix(env, "GHORG_")

			// skip global flags and reclone flags which are set in the conf.yaml
			if env == "GHORG_COLOR" || env == "GHORG_CONFIG" || env == "GHORG_RECLONE_QUIET" || env == "GHORG_RECLONE_PATH" || env == "GHORG_RECLONE_RUNNING" {
				continue
			}
			if ghorgEnv {
				os.Unsetenv(env)
			}
		}
	}

	// Connect ghorgClone's stdout and stderr to the current process's stdout and stderr
	if !isQuietReClone() {
		ghorgClone.Stdout = os.Stdout
		ghorgClone.Stderr = os.Stderr
	} else {
		spinningSpinner.Start()
		defer spinningSpinner.Stop()
		ghorgClone.Stdout = nil
		ghorgClone.Stderr = nil
	}

	err := ghorgClone.Start()
	if err != nil {
		spinningSpinner.Stop()
		colorlog.PrintErrorAndExit(fmt.Sprintf("ERROR: Starting ghorg clone cmd: %v, err: %v", safeToLogCmd, err))
	}

	err = ghorgClone.Wait()
	status := "success"
	if err != nil {
		status = "fail"
	}

	if rc.PostExecScript != "" {
		postCmd := exec.Command(rc.PostExecScript, status, rcIdentifier)
		postCmd.Stdout = os.Stdout
		postCmd.Stderr = os.Stderr
		errPost := postCmd.Run()
		if errPost != nil {
			colorlog.PrintError(fmt.Sprintf("ERROR: Running post_exec_script %s: %v", rc.PostExecScript, errPost))
		}
	}

	if err != nil {
		spinningSpinner.Stop()
		colorlog.PrintErrorAndExit(fmt.Sprintf("ERROR: Running ghorg clone cmd: %v, err: %v", safeToLogCmd, err))
	}
}
