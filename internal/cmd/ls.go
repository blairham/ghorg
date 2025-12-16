package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/utils"
	"github.com/briandowns/spinner"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

type LsCommand struct {
	UI cli.Ui
}

type LsFlags struct {
	Long  bool `short:"l" long:"long" description:"Display detailed information about each clone directory, including size and number of repositories. Note: This may take longer depending on the number and size of the cloned organizations."`
	Total bool `short:"t" long:"total" description:"Display total amounts of all repos cloned. Note: This may take longer depending on the number and size of the cloned organizations."`
}

var spinningSpinner *spinner.Spinner

func init() {
	spinningSpinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
}

func (c *LsCommand) Help() string {
	return `Usage: ghorg ls [options] [dir]

List contents of your ghorg home or ghorg directories.
If no dir is specified it will list contents of GHORG_ABSOLUTE_PATH_TO_CLONE_TO.

Options:
  -l, --long   Display detailed information about each clone directory
  -t, --total  Display total amounts of all repos cloned

Examples:
  ghorg ls
  ghorg ls -l
  ghorg ls -t my-org
`
}

func (c *LsCommand) Synopsis() string {
	return "List contents of your ghorg home or ghorg directories"
}

func (c *LsCommand) Run(args []string) int {
	var opts LsFlags
	parser := flags.NewParser(&opts, flags.Default)
	remaining, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			fmt.Println(c.Help())
			return 0
		}
		colorlog.PrintError(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}

	if len(remaining) == 0 {
		listGhorgHome(opts.Long, opts.Total)
	} else {
		for _, arg := range remaining {
			listGhorgDir(arg, opts.Long, opts.Total)
		}
	}

	return 0
}

func listGhorgHome(longFormat, totalFormat bool) {
	path := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
	files, err := os.ReadDir(path)
	if err != nil {
		colorlog.PrintError("No clones found. Please clone some and try again.")
		return
	}

	if !longFormat && !totalFormat {
		for _, f := range files {
			if f.IsDir() {
				colorlog.PrintInfo(path + f.Name())
			}
		}
		return
	}

	spinningSpinner.Start()

	var totalDirs int
	var totalSizeMB float64
	var totalRepos int

	type result struct {
		dirPath     string
		dirSizeMB   float64
		subDirCount int
		err         error
	}

	results := make(chan result, len(files))
	var wg sync.WaitGroup

	for _, f := range files {
		if f.IsDir() {
			totalDirs++
			wg.Add(1)
			go func(f os.DirEntry) {
				defer wg.Done()
				dirPath := filepath.Join(path, f.Name())
				dirSizeMB, err := utils.CalculateDirSizeInMb(dirPath)
				if err != nil {
					results <- result{dirPath: dirPath, err: err}
					return
				}

				subDirCount := 0
				subFiles, err := os.ReadDir(dirPath)
				if err != nil {
					results <- result{dirPath: dirPath, err: err}
					return
				}
				for _, subFile := range subFiles {
					if subFile.IsDir() {
						subDirCount++
					}
				}
				results <- result{dirPath: dirPath, dirSizeMB: dirSizeMB, subDirCount: subDirCount}
			}(f)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			colorlog.PrintError(fmt.Sprintf("Error processing directory %s: %v", res.dirPath, res.err))
			continue
		}
		totalSizeMB += res.dirSizeMB
		totalRepos += res.subDirCount
		if !totalFormat || longFormat {
			spinningSpinner.Stop()
			if longFormat {
				if res.dirSizeMB > 1000 {
					dirSizeGB := res.dirSizeMB / 1000
					colorlog.PrintInfo(fmt.Sprintf("%-90s %10.2f GB %10d repos", res.dirPath, dirSizeGB, res.subDirCount))
				} else {
					colorlog.PrintInfo(fmt.Sprintf("%-90s %10.2f MB %10d repos", res.dirPath, res.dirSizeMB, res.subDirCount))
				}
			} else {
				colorlog.PrintInfo(res.dirPath)
			}
		}
	}

	spinningSpinner.Stop()
	if totalFormat {
		if totalSizeMB > 1000 {
			totalSizeGB := totalSizeMB / 1000
			colorlog.PrintSuccess(fmt.Sprintf("Total: %d directories, %.2f GB, %d repos", totalDirs, totalSizeGB, totalRepos))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("Total: %d directories, %.2f MB, %d repos", totalDirs, totalSizeMB, totalRepos))
		}
	}
}

func listGhorgDir(arg string, longFormat, totalFormat bool) {

	path := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO") + arg

	_, err := os.ReadDir(path)
	if err != nil {
		// ghorg natively uses underscores in folder names, but a user can specify an output dir with underscores
		// so first try what the user types if not then try replace
		arg = strings.ReplaceAll(arg, "-", "_")
		path = os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO") + arg
	}

	files, err := os.ReadDir(path)
	if err != nil {
		colorlog.PrintError("No clones found. Please clone some and try again.")
		return
	}

	if !longFormat && !totalFormat {
		for _, f := range files {
			if f.IsDir() {
				str := filepath.Join(path, f.Name())
				colorlog.PrintInfo(str)
			}
		}
		return
	}

	spinningSpinner.Start()

	var totalDirs int
	var totalSizeMB float64

	type result struct {
		dirPath     string
		dirSizeMB   float64
		subDirCount int
		err         error
	}

	results := make(chan result)
	var wg sync.WaitGroup

	for _, f := range files {
		if f.IsDir() {
			wg.Add(1)
			go func(f os.DirEntry) {
				defer wg.Done()
				dirPath := filepath.Join(path, f.Name())
				dirSizeMB, err := utils.CalculateDirSizeInMb(dirPath)
				if err != nil {
					results <- result{dirPath: dirPath, err: err}
					return
				}

				// Count the number of directories with a depth of 1 inside
				subDirCount := 0
				subFiles, err := os.ReadDir(dirPath)
				if err != nil {
					results <- result{dirPath: dirPath, err: err}
					return
				}
				for _, subFile := range subFiles {
					if subFile.IsDir() {
						subDirCount++
					}
				}
				results <- result{dirPath: dirPath, dirSizeMB: dirSizeMB, subDirCount: subDirCount}
			}(f)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			colorlog.PrintError(fmt.Sprintf("Error processing directory %s: %v", res.dirPath, res.err))
			continue
		}
		totalSizeMB += res.dirSizeMB
		totalDirs++
		if !totalFormat || longFormat {
			spinningSpinner.Stop()
			if longFormat {
				if res.dirSizeMB > 1000 {
					dirSizeGB := res.dirSizeMB / 1000
					colorlog.PrintInfo(fmt.Sprintf("%-90s %10.2f GB ", res.dirPath, dirSizeGB))
				} else {
					colorlog.PrintInfo(fmt.Sprintf("%-90s %10.2f MB", res.dirPath, res.dirSizeMB))
				}
			} else {
				colorlog.PrintInfo(res.dirPath)
			}
		}
	}

	spinningSpinner.Stop()
	if totalFormat {
		if totalSizeMB > 1000 {
			totalSizeGB := totalSizeMB / 1000
			colorlog.PrintSuccess(fmt.Sprintf("Total: %d repos, %.2f GB", totalDirs, totalSizeGB))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("Total: %d repos, %.2f MB", totalDirs, totalSizeMB))
		}
	}
}
