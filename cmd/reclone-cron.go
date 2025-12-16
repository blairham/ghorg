package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/blairham/ghorg/colorlog"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

var (
	recloneRunning bool
	recloneMutex   sync.Mutex
)

type RecloneCronCommand struct {
	UI cli.Ui
}

type RecloneCronFlags struct {
	Minutes string `short:"m" long:"minutes" description:"GHORG_CRON_TIMER_MINUTES - Number of minutes to run the reclone command on a cron"`
}

func (c *RecloneCronCommand) Help() string {
	return `Usage: ghorg reclone-cron [options]

Simple cron that will trigger your reclone command at specified minute intervals indefinitely.

Options:
  -m, --minutes    Number of minutes between reclone runs

Examples:
  ghorg reclone-cron --minutes 60
  ghorg reclone-cron -m 30

Read the documentation and examples in the Readme under Reclone Server heading.
`
}

func (c *RecloneCronCommand) Synopsis() string {
	return "Simple cron that triggers reclone at specified intervals"
}

func (c *RecloneCronCommand) Run(args []string) int {
	var opts RecloneCronFlags
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			fmt.Println(c.Help())
			return 0
		}
		colorlog.PrintError(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}

	if opts.Minutes != "" {
		os.Setenv("GHORG_CRON_TIMER_MINUTES", opts.Minutes)
	}

	startReCloneCron()
	return 0
}

func startReCloneCron() {
	cronTimer := os.Getenv("GHORG_CRON_TIMER_MINUTES")
	if cronTimer == "" {
		colorlog.PrintInfo("GHORG_CRON_TIMER_MINUTES is not set. Cron job will not start.")
		return
	}

	colorlog.PrintInfo("Cron activated and will first run after " + cronTimer + " minutes ")

	minutes, err := strconv.Atoi(cronTimer)
	if err != nil {
		colorlog.PrintError("Invalid GHORG_CRON_TIMER_MINUTES: " + cronTimer)
		return
	}

	ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		recloneMutex.Lock()
		if recloneRunning {
			recloneMutex.Unlock()
			continue
		}
		recloneRunning = true
		recloneMutex.Unlock()

		colorlog.PrintInfo("Starting reclone cron, time: " + time.Now().Format(time.RFC1123))
		cmd := exec.Command("ghorg", "reclone")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			colorlog.PrintError("Failed to start ghorg reclone: " + err.Error())
			recloneMutex.Lock()
			recloneRunning = false
			recloneMutex.Unlock()
			continue
		}

		go func() {
			if err := cmd.Wait(); err != nil {
				colorlog.PrintError("ghorg reclone command failed: " + err.Error())
			}
			recloneMutex.Lock()
			recloneRunning = false
			recloneMutex.Unlock()
		}()
	}
}
