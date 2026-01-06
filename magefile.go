//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

// Deps namespace groups dependency management targets
type Deps mg.Namespace

// Install installs mage and other development dependencies
func (Deps) Install() error {
	fmt.Println("Installing development dependencies...")
	if err := sh.RunV("go", "install", "github.com/magefile/mage@latest"); err != nil {
		return err
	}
	if err := sh.RunV("go", "install", "github.com/goreleaser/goreleaser@latest"); err != nil {
		return err
	}
	if err := sh.RunV("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"); err != nil {
		return err
	}
	fmt.Println("All development dependencies installed!")
	return nil
}

// Verify checks that all dependencies are available
func (Deps) Verify() error {
	fmt.Println("Verifying dependencies...")
	deps := []string{"go", "git", "goreleaser", "golangci-lint"}
	missing := []string{}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			missing = append(missing, dep)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing dependencies: %s", strings.Join(missing, ", "))
	}
	fmt.Println("All dependencies available!")
	return nil
}

// Build builds ghorg for all platforms using goreleaser snapshot (no publish)
func Build() error {
	mg.Deps(Deps.Verify)
	fmt.Println("Building ghorg with goreleaser (snapshot)...")
	return sh.RunV("goreleaser", "build", "--snapshot", "--clean")
}

// BuildDocker builds Docker images using goreleaser snapshot (no push)
func BuildDocker() error {
	mg.Deps(Deps.Verify)
	fmt.Println("Building Docker images with goreleaser (snapshot)...")
	return sh.RunV("goreleaser", "release", "--snapshot", "--clean", "--skip=publish")
}

// BuildLocal builds a single ghorg binary for local development (fast)
func BuildLocal() error {
	mg.Deps(Fmt)
	fmt.Println("Building ghorg locally...")
	return sh.RunV("go", "build", "-o", "ghorg", "./cmd/ghorg")
}

// Install installs ghorg and sets up the config directory
func Install() error {
	mg.Deps(BuildLocal)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "ghorg")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "conf.yaml")
	fmt.Printf("Copying sample-conf.yaml to %s\n", configFile)
	return sh.Copy(configFile, "sample-conf.yaml")
}

// Homebrew sets up config for homebrew installation
func Homebrew() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "ghorg")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "conf.yaml")
	fmt.Printf("Copying sample-conf.yaml to %s\n", configFile)
	return sh.Copy(configFile, "sample-conf.yaml")
}

// Fmt formats Go source files
func Fmt() error {
	fmt.Println("Formatting Go files...")
	gofiles, err := getGoFiles()
	if err != nil {
		return err
	}

	args := append([]string{"-s", "-w"}, gofiles...)
	return sh.RunV("gofmt", args...)
}

// Test runs all tests
func Test() error {
	fmt.Println("Running tests...")
	return sh.RunV("go", "test", "./...", "-v")
}

// TestGit runs git-specific tests
func TestGit() error {
	fmt.Println("Running git tests...")
	return sh.RunV("go", "test", "./internal/git", "-v")
}

// TestCoverage runs tests with coverage and generates HTML report
func TestCoverage() error {
	fmt.Println("Running tests with coverage...")
	if err := sh.RunV("go", "test", "./...", "-coverprofile=coverage.out", "-covermode=atomic"); err != nil {
		return err
	}
	if err := sh.RunV("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		return err
	}
	fmt.Println("Coverage report generated: coverage.html")
	return nil
}

// TestCoverageFunc runs coverage and displays function-level coverage
func TestCoverageFunc() error {
	fmt.Println("Running coverage analysis...")

	// Run tests with coverage in internal/git
	cmd := exec.Command("go", "test", "-coverprofile=coverage.out", "-covermode=atomic")
	cmd.Dir = "internal/git"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Move coverage file to root
	if err := os.Rename("internal/git/coverage.out", "coverage.out"); err != nil {
		// Try copy if rename fails (cross-device)
		if err := sh.Copy("coverage.out", "internal/git/coverage.out"); err != nil {
			return err
		}
		os.Remove("internal/git/coverage.out")
	}

	if err := sh.RunV("go", "tool", "cover", "-func=coverage.out"); err != nil {
		return err
	}

	fmt.Println("")
	fmt.Println("=== New Git Helper Functions Coverage ===")

	// Run coverage func and filter for specific functions
	output, err := sh.Output("go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}

	patterns := []string{
		"GetRemoteURL", "HasLocalChanges", "HasUnpushedCommits",
		"GetCurrentBranch", "HasCommitsNotOnDefaultBranch",
		"IsDefaultBranchBehindHead", "MergeIntoDefaultBranch", "UpdateRef",
	}

	for _, line := range strings.Split(output, "\n") {
		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				fmt.Println(line)
				break
			}
		}
	}

	return nil
}

// TestAll runs all tests
func TestAll() error {
	mg.Deps(Fmt, Lint)
	if err := Test(); err != nil {
		return err
	}
	fmt.Println("")
	fmt.Println("=== All Tests Complete ===")
	return nil
}

// TestSync runs sync-specific tests
func TestSync() error {
	fmt.Println("Running sync tests...")
	return sh.RunV("go", "test", "./internal/git", "-v", "-run", "TestSync")
}

// TestHelpers runs helper function tests
func TestHelpers() error {
	fmt.Println("Running helper tests...")
	return sh.RunV("go", "test", "./internal/git", "-v", "-run",
		"^Test(GetRemoteURL|HasLocalChanges|HasUnpushedCommits|GetCurrentBranch|HasCommitsNotOnDefaultBranch|IsDefaultBranchBehindHead|MergeIntoDefaultBranch|UpdateRef)")
}

// Release runs goreleaser to create a release (requires GITHUB_TOKEN)
func Release() error {
	mg.Deps(Deps.Verify, Test)
	fmt.Println("Running goreleaser release...")
	return sh.RunV("goreleaser", "release", "--clean")
}

// ReleaseDry runs goreleaser in dry-run mode (no publish)
func ReleaseDry() error {
	mg.Deps(Deps.Verify)
	fmt.Println("Running goreleaser release (dry-run)...")
	return sh.RunV("goreleaser", "release", "--snapshot", "--clean")
}

// ReleaseCheck validates the goreleaser configuration
func ReleaseCheck() error {
	fmt.Println("Checking goreleaser configuration...")
	return sh.RunV("goreleaser", "check")
}

// Examples copies example files
func Examples() error {
	fmt.Println("Copying examples...")
	return sh.Run("cp", "-rf", "examples/", "internal/cmd/examples-copy/")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning build artifacts...")
	files := []string{"ghorg", "coverage.out", "coverage.html"}
	dirs := []string{"dist"}
	for _, f := range files {
		os.Remove(f)
	}
	for _, d := range dirs {
		os.RemoveAll(d)
	}
	return nil
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")
	return sh.RunV("golangci-lint", "run", "./...")
}

// Vendor updates vendor directory
func Vendor() error {
	fmt.Println("Updating vendor directory...")
	if err := sh.RunV("go", "mod", "tidy"); err != nil {
		return err
	}
	return sh.RunV("go", "mod", "vendor")
}

// getGoFiles returns all Go files excluding vendor directory
func getGoFiles() ([]string, error) {
	var gofiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, "vendor/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "bindata.go") {
			gofiles = append(gofiles, path)
		}
		return nil
	})
	return gofiles, err
}
