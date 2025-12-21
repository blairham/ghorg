// Package cmd encapsulates the logic for all cli commands
package cmd

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/configs"
	"github.com/blairham/ghorg/internal/git"
	"github.com/blairham/ghorg/internal/scm"
	"github.com/blairham/ghorg/internal/utils"
	"github.com/jessevdk/go-flags"
	"github.com/korovkin/limiter"
	"github.com/mitchellh/cli"
)

// Helper function to safely parse integer environment variables
func parseIntEnv(envVar string) (int, error) {
	value := os.Getenv(envVar)
	if value == "" {
		return 0, fmt.Errorf("environment variable %s not set", envVar)
	}
	return strconv.Atoi(value)
}

// Helper function to get clone delay configuration
func getCloneDelaySeconds() (int, bool) {
	delaySeconds, err := parseIntEnv("GHORG_CLONE_DELAY_SECONDS")
	if err != nil || delaySeconds <= 0 {
		return 0, false
	}
	return delaySeconds, true
}

// Helper function to check if concurrency should be auto-adjusted for delay
func shouldAutoAdjustConcurrency() (int, bool, bool) {
	delaySeconds, hasDelay := getCloneDelaySeconds()
	if !hasDelay {
		return 0, false, false
	}

	concurrency, err := parseIntEnv("GHORG_CONCURRENCY")
	if err != nil || concurrency <= 1 {
		return delaySeconds, false, false
	}

	return delaySeconds, true, true
}

type CloneCommand struct {
	UI cli.Ui
}

type CloneFlags struct {
	// Global flags (these were on rootCmd in the old cobra version)
	Config string `long:"config" description:"GHORG_CONFIG - Manually set the path to your config file"`
	Color  string `long:"color" description:"GHORG_COLOR - Toggles colorful output, enabled/disabled (default: disabled)"`

	// Path and protocol flags
	Path     string `short:"p" long:"path" description:"GHORG_ABSOLUTE_PATH_TO_CLONE_TO - Absolute path to the home for ghorg clones. Must start with / (default $HOME/ghorg)"`
	Protocol string `long:"protocol" description:"GHORG_CLONE_PROTOCOL - Protocol to clone with, ssh or https, (default https)"`

	// Branch and sync flags
	Branch            string `short:"b" long:"branch" description:"GHORG_BRANCH - Branch left checked out for each repo cloned (default master)"`
	SyncDefaultBranch bool   `long:"sync-default-branch" description:"GHORG_SYNC_DEFAULT_BRANCH - Automatically keep the default branch in sync with the remote by performing a fetch and fast-forward merge before cloning"`

	// Token and auth flags
	Token             string `short:"t" long:"token" description:"GHORG_GITHUB_TOKEN/GHORG_GITLAB_TOKEN/GHORG_GITEA_TOKEN/GHORG_BITBUCKET_APP_PASSWORD/GHORG_BITBUCKET_OAUTH_TOKEN/GHORG_SOURCEHUT_TOKEN - scm token to clone with"`
	BitbucketUsername string `long:"bitbucket-username" description:"GHORG_BITBUCKET_USERNAME - Bitbucket only: username associated with the app password"`
	NoToken           bool   `long:"no-token" description:"GHORG_NO_TOKEN - Allows you to run ghorg with no token (GHORG_<SCM>_TOKEN), SCM server needs to specify no auth required for api calls"`

	// SCM and clone type flags
	SCMType   string `short:"s" long:"scm" description:"GHORG_SCM_TYPE - Type of scm used, github, gitlab, gitea, bitbucket or sourcehut (default github)"`
	CloneType string `short:"c" long:"clone-type" description:"GHORG_CLONE_TYPE - Clone target type, user or org (default org)"`
	BaseURL   string `long:"base-url" description:"GHORG_SCM_BASE_URL - Change SCM base url, for on self hosted instances (currently gitlab, gitea and github (use format of https://git.mydomain.com/api/v3))"`

	// Filter flags
	SkipArchived                 bool   `long:"skip-archived" description:"GHORG_SKIP_ARCHIVED - Skips archived repos, github/gitlab/gitea only"`
	SkipForks                    bool   `long:"skip-forks" description:"GHORG_SKIP_FORKS - Skips repo if its a fork, github/gitlab/gitea only"`
	Topics                       string `long:"topics" description:"GHORG_TOPICS - Comma separated list of github/gitea topics to filter for"`
	MatchPrefix                  string `long:"match-prefix" description:"GHORG_MATCH_PREFIX - Only clone repos with matching prefix, can be a comma separated list"`
	ExcludeMatchPrefix           string `long:"exclude-match-prefix" description:"GHORG_EXCLUDE_MATCH_PREFIX - Exclude cloning repos with matching prefix, can be a comma separated list"`
	MatchRegex                   string `long:"match-regex" description:"GHORG_MATCH_REGEX - Only clone repos that match name to regex provided"`
	ExcludeMatchRegex            string `long:"exclude-match-regex" description:"GHORG_EXCLUDE_MATCH_REGEX - Exclude cloning repos that match name to regex provided"`
	GitlabGroupExcludeMatchRegex string `long:"gitlab-group-exclude-match-regex" description:"GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX - Exclude cloning gitlab groups that match name to regex provided"`
	GhorgIgnorePath              string `long:"ghorgignore-path" description:"GHORG_IGNORE_PATH - If you want to set a path other than $HOME/.config/ghorg/ghorgignore for your ghorgignore"`
	GhorgOnlyPath                string `long:"ghorgonly-path" description:"GHORG_ONLY_PATH - If you want to set a path other than $HOME/.config/ghorg/ghorgonly for your ghorgonly"`
	TargetReposPath              string `long:"target-repos-path" description:"GHORG_TARGET_REPOS_PATH - Path to file with list of repo names to clone, file should contain one repo name per line"`

	// Clone behavior flags
	NoClean                 bool `long:"no-clean" description:"GHORG_NO_CLEAN - Only clones new repos and does not perform a git clean on existing repos"`
	Prune                   bool `long:"prune" description:"GHORG_PRUNE - Deletes all files/directories found in your local clone directory that are not found on the remote (e.g., after remote deletion). With GHORG_SKIP_ARCHIVED set, archived repositories will also be pruned from your local clone. Will prompt before deleting any files unless used in combination with --prune-no-confirm"`
	PruneNoConfirm          bool `long:"prune-no-confirm" description:"GHORG_PRUNE_NO_CONFIRM - Don't prompt on every prune candidate, just delete"`
	PruneUntouched          bool `long:"prune-untouched" description:"GHORG_PRUNE_UNTOUCHED - Prune repositories that don't have any local changes, see sample-conf.yaml for more details"`
	PruneUntouchedNoConfirm bool `long:"prune-untouched-no-confirm" description:"GHORG_PRUNE_UNTOUCHED_NO_CONFIRM - Automatically delete repos without showing an interactive confirmation prompt"`
	FetchAll                bool `long:"fetch-all" description:"GHORG_FETCH_ALL - Fetches all remote branches for each repo by running a git fetch --all"`
	DryRun                  bool `long:"dry-run" description:"GHORG_DRY_RUN - Perform a dry run of the clone; fetches repos but does not clone them"`
	Backup                  bool `long:"backup" description:"GHORG_BACKUP - Backup mode, clone as mirror, no working copy (ignores branch parameter)"`
	IncludeSubmodules       bool `long:"include-submodules" description:"GHORG_INCLUDE_SUBMODULES - Include submodules in all clone and pull operations"`

	// Additional content flags
	CloneWiki     bool `long:"clone-wiki" description:"GHORG_CLONE_WIKI - Additionally clone the wiki page for repo"`
	CloneSnippets bool `long:"clone-snippets" description:"GHORG_CLONE_SNIPPETS - Additionally clone all snippets, gitlab only"`

	// Insecure client flags
	InsecureGitlabClient    bool `long:"insecure-gitlab-client" description:"GHORG_INSECURE_GITLAB_CLIENT - Skip TLS certificate verification for hosted gitlab instances"`
	InsecureGiteaClient     bool `long:"insecure-gitea-client" description:"GHORG_INSECURE_GITEA_CLIENT - Must be set to clone from a Gitea instance using http"`
	InsecureBitbucketClient bool `long:"insecure-bitbucket-client" description:"GHORG_INSECURE_BITBUCKET_CLIENT - Must be set to clone from a Bitbucket Server instance using http"`
	InsecureSourcehutClient bool `long:"insecure-sourcehut-client" description:"GHORG_INSECURE_SOURCEHUT_CLIENT - Must be set to clone from a Sourcehut instance using http"`

	// Directory and output flags
	PreserveDir         bool   `long:"preserve-dir" description:"GHORG_PRESERVE_DIRECTORY_STRUCTURE - Clones repos in a directory structure that matches gitlab namespaces eg company/unit/subunit/app would clone into ghorg/unit/subunit/app, gitlab only"`
	OutputDir           string `long:"output-dir" description:"GHORG_OUTPUT_DIR - Name of directory repos will be cloned into (default name of org/repo being cloned"`
	NoDirSize           bool   `long:"no-dir-size" description:"GHORG_NO_DIR_SIZE - Skips the calculation of the output directory size at the end of a clone operation. This can save time, especially when cloning a large number of repositories"`
	PreserveSCMHostname bool   `long:"preserve-scm-hostname" description:"GHORG_PRESERVE_SCM_HOSTNAME - Appends the scm hostname to the GHORG_ABSOLUTE_PATH_TO_CLONE_TO which will organize your clones into specific folders by the scm provider. e.g. /github.com/kubernetes"`

	// Performance and control flags
	Concurrency       string `long:"concurrency" description:"GHORG_CONCURRENCY - Max goroutines to spin up while cloning (default 25)"`
	CloneDelaySeconds string `long:"clone-delay-seconds" description:"GHORG_CLONE_DELAY_SECONDS - Delay in seconds between cloning repos. Useful for rate limiting. Automatically sets concurrency to 1 when > 0 (default 0)"`
	CloneDepth        string `long:"clone-depth" description:"GHORG_CLONE_DEPTH - Create a shallow clone with a history truncated to the specified number of commits"`
	GitFilter         string `long:"git-filter" description:"GHORG_GIT_FILTER - Allows you to pass arguments to git's filter flag. Useful for filtering out binary objects from repos with --git-filter=blob:none, this requires git version 2.19 or greater"`
	GitBackend        string `long:"git-backend" description:"GHORG_GIT_BACKEND - Git backend to use: 'golang' (default, pure Go implementation) or 'exec' (uses system git)"`

	// Exit code flags
	ExitCodeOnCloneInfos  string `long:"exit-code-on-clone-infos" description:"GHORG_EXIT_CODE_ON_CLONE_INFOS - Allows you to control the exit code when ghorg runs into a problem (info level message) cloning a repo from the remote. Info messages will appear after a clone is complete, similar to success messages. (default 0)"`
	ExitCodeOnCloneIssues string `long:"exit-code-on-clone-issues" description:"GHORG_EXIT_CODE_ON_CLONE_ISSUES - Allows you to control the exit code when ghorg runs into a problem (issue level message) cloning a repo from the remote. Issue messages will appear after a clone is complete, similar to success messages (default 1)"`

	// Logging and stats flags
	Quiet        bool `long:"quiet" description:"GHORG_QUIET - Emit critical output only"`
	StatsEnabled bool `long:"stats-enabled" description:"GHORG_STATS_ENABLED - Creates a CSV in the GHORG_ABSOLUTE_PATH_TO_CLONE_TO called _ghorg_stats.csv with info about each clone. This allows you to track clone data over time such as number of commits and size in megabytes of the clone directory"`

	// GitHub specific flags
	GitHubTokenFromGitHubApp string `long:"github-token-from-github-app" description:"GHORG_GITHUB_TOKEN_FROM_GITHUB_APP - Indicate that the Github token should be treated as an app token. Needed if you already obtained a github app token outside the context of ghorg"`
	GitHubAppPemPath         string `long:"github-app-pem-path" description:"GHORG_GITHUB_APP_PEM_PATH - Path to your GitHub App PEM file, for authenticating with GitHub App"`
	GitHubAppInstallationID  string `long:"github-app-installation-id" description:"GHORG_GITHUB_APP_INSTALLATION_ID - GitHub App Installation ID, for authenticating with GitHub App"`
	GitHubAppID              string `long:"github-app-id" description:"GHORG_GITHUB_APP_ID - GitHub App ID, for authenticating with GitHub App"`
	GitHubFilterLanguage     string `long:"github-filter-language" description:"GHORG_GITHUB_FILTER_LANGUAGE - Filter repos by a language. Can be a comma separated value with no spaces"`
	GitHubUserOption         string `long:"github-user-option" description:"GHORG_GITHUB_USER_OPTION - Only available when also using GHORG_CLONE_TYPE: user e.g. --clone-type=user can be one of: all, owner, member (default: owner)"`
}

func (c *CloneCommand) Help() string {
	return `Usage: ghorg clone [options] [org/user]

Clone user or org repos from GitHub, GitLab, Gitea or Bitbucket.
See $HOME/.config/ghorg/conf.yaml for defaults.

For complete examples of how to clone repos from each SCM provider, run:
  ghorg examples github
  ghorg examples gitlab
  ghorg examples bitbucket
  ghorg examples gitea

Or see examples directory at https://github.com/blairham/ghorg/tree/master/examples

Options:
  -p, --path                           Absolute path to clone repos to
  --protocol                           Protocol to clone with (ssh or https)
  -b, --branch                         Branch to checkout for each repo
  -t, --token                          SCM token for authentication
  -s, --scm                            SCM type (github, gitlab, gitea, bitbucket, sourcehut)
  -c, --clone-type                     Clone target type (user or org)
  --base-url                           SCM base URL for self-hosted instances
  --skip-archived                      Skip archived repos
  --skip-forks                         Skip forked repos
  --no-clean                           Only clone new repos, don't clean existing
  --prune                              Delete local repos not found on remote
  --fetch-all                          Fetch all remote branches
  --dry-run                            Perform a dry run
  --backup                             Backup mode (clone as mirror)
  --include-submodules                 Include submodules
  --clone-wiki                         Clone wiki pages
  --quiet                              Emit critical output only
  --stats-enabled                      Create stats CSV file

Examples:
  ghorg clone kubernetes
  ghorg clone --scm gitlab my-org
  ghorg clone --clone-type user my-user
  ghorg clone --branch develop my-org
`
}

func (c *CloneCommand) Synopsis() string {
	return "Clone user or org repos from GitHub, GitLab, Gitea or Bitbucket"
}

func (c *CloneCommand) Run(args []string) int {
	// Record start time for the entire command duration including SCM API calls
	commandStartTime = time.Now()

	var opts CloneFlags
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

	// Handle config flag first - if set, reinitialize configuration
	if opts.Config != "" {
		os.Setenv("GHORG_CONFIG", opts.Config)
		InitConfig()
	}

	// Handle color flag
	if opts.Color != "" {
		os.Setenv("GHORG_COLOR", opts.Color)
	}

	// Set environment variables from flags
	if opts.Path != "" {
		absolutePath := configs.EnsureTrailingSlashOnFilePath(opts.Path)
		os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", absolutePath)
	}

	if opts.Protocol != "" {
		os.Setenv("GHORG_CLONE_PROTOCOL", opts.Protocol)
	}

	if opts.Branch != "" {
		os.Setenv("GHORG_BRANCH", opts.Branch)
	}

	if opts.GitHubTokenFromGitHubApp != "" {
		os.Setenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP", opts.GitHubTokenFromGitHubApp)
	}

	if opts.GitHubAppPemPath != "" {
		os.Setenv("GHORG_GITHUB_APP_PEM_PATH", opts.GitHubAppPemPath)
	}

	if opts.GitHubAppInstallationID != "" {
		os.Setenv("GHORG_GITHUB_APP_INSTALLATION_ID", opts.GitHubAppInstallationID)
	}

	if opts.GitHubFilterLanguage != "" {
		os.Setenv("GHORG_GITHUB_FILTER_LANGUAGE", opts.GitHubFilterLanguage)
	}

	if opts.GitHubAppID != "" {
		os.Setenv("GHORG_GITHUB_APP_ID", opts.GitHubAppID)
	}

	if opts.BitbucketUsername != "" {
		os.Setenv("GHORG_BITBUCKET_USERNAME", opts.BitbucketUsername)
	}

	if opts.CloneType != "" {
		cloneType := strings.ToLower(opts.CloneType)
		os.Setenv("GHORG_CLONE_TYPE", cloneType)
	}

	if opts.SCMType != "" {
		scmType := strings.ToLower(opts.SCMType)
		os.Setenv("GHORG_SCM_TYPE", scmType)
	}

	if opts.GitHubUserOption != "" {
		os.Setenv("GHORG_GITHUB_USER_OPTION", opts.GitHubUserOption)
	}

	if opts.BaseURL != "" {
		os.Setenv("GHORG_SCM_BASE_URL", opts.BaseURL)
	}

	if opts.Concurrency != "" {
		os.Setenv("GHORG_CONCURRENCY", opts.Concurrency)
	}

	if opts.CloneDelaySeconds != "" {
		os.Setenv("GHORG_CLONE_DELAY_SECONDS", opts.CloneDelaySeconds)
	}

	if opts.CloneDepth != "" {
		os.Setenv("GHORG_CLONE_DEPTH", opts.CloneDepth)
	}

	if opts.ExitCodeOnCloneInfos != "" {
		os.Setenv("GHORG_EXIT_CODE_ON_CLONE_INFOS", opts.ExitCodeOnCloneInfos)
	}

	if opts.ExitCodeOnCloneIssues != "" {
		os.Setenv("GHORG_EXIT_CODE_ON_CLONE_ISSUES", opts.ExitCodeOnCloneIssues)
	}

	if opts.Topics != "" {
		os.Setenv("GHORG_TOPICS", opts.Topics)
	}

	if opts.MatchPrefix != "" {
		os.Setenv("GHORG_MATCH_PREFIX", opts.MatchPrefix)
	}

	if opts.ExcludeMatchPrefix != "" {
		os.Setenv("GHORG_EXCLUDE_MATCH_PREFIX", opts.ExcludeMatchPrefix)
	}

	if opts.GitlabGroupExcludeMatchRegex != "" {
		os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", opts.GitlabGroupExcludeMatchRegex)
	}

	if opts.MatchRegex != "" {
		os.Setenv("GHORG_MATCH_REGEX", opts.MatchRegex)
	}

	if opts.ExcludeMatchRegex != "" {
		os.Setenv("GHORG_EXCLUDE_MATCH_REGEX", opts.ExcludeMatchRegex)
	}

	if opts.GhorgIgnorePath != "" {
		os.Setenv("GHORG_IGNORE_PATH", opts.GhorgIgnorePath)
	}

	if opts.GhorgOnlyPath != "" {
		os.Setenv("GHORG_ONLY_PATH", opts.GhorgOnlyPath)
	}

	if opts.TargetReposPath != "" {
		os.Setenv("GHORG_TARGET_REPOS_PATH", opts.TargetReposPath)
	}

	if opts.GitFilter != "" {
		os.Setenv("GHORG_GIT_FILTER", opts.GitFilter)
	}

	if opts.GitBackend != "" {
		os.Setenv("GHORG_GIT_BACKEND", opts.GitBackend)
	}

	if opts.PreserveSCMHostname {
		os.Setenv("GHORG_PRESERVE_SCM_HOSTNAME", "true")
	}

	if opts.SkipArchived {
		os.Setenv("GHORG_SKIP_ARCHIVED", "true")
	}

	if opts.StatsEnabled {
		os.Setenv("GHORG_STATS_ENABLED", "true")
	}

	if opts.NoClean {
		os.Setenv("GHORG_NO_CLEAN", "true")
	}

	if opts.Prune {
		os.Setenv("GHORG_PRUNE", "true")
	}

	if opts.PruneNoConfirm {
		os.Setenv("GHORG_PRUNE_NO_CONFIRM", "true")
	}

	if opts.PruneUntouched {
		os.Setenv("GHORG_PRUNE_UNTOUCHED", "true")
	}

	if opts.PruneUntouchedNoConfirm {
		os.Setenv("GHORG_PRUNE_UNTOUCHED_NO_CONFIRM", "true")
	}

	if opts.FetchAll {
		os.Setenv("GHORG_FETCH_ALL", "true")
	}

	if opts.IncludeSubmodules {
		os.Setenv("GHORG_INCLUDE_SUBMODULES", "true")
	}

	if opts.DryRun {
		os.Setenv("GHORG_DRY_RUN", "true")
	}

	if opts.CloneWiki {
		os.Setenv("GHORG_CLONE_WIKI", "true")
	}

	if opts.CloneSnippets {
		os.Setenv("GHORG_CLONE_SNIPPETS", "true")
	}

	if opts.InsecureGitlabClient {
		os.Setenv("GHORG_INSECURE_GITLAB_CLIENT", "true")
	}

	if opts.InsecureGiteaClient {
		os.Setenv("GHORG_INSECURE_GITEA_CLIENT", "true")
	}

	if opts.InsecureBitbucketClient {
		os.Setenv("GHORG_INSECURE_BITBUCKET_CLIENT", "true")
	}

	if opts.InsecureSourcehutClient {
		os.Setenv("GHORG_INSECURE_SOURCEHUT_CLIENT", "true")
	}

	if opts.SkipForks {
		os.Setenv("GHORG_SKIP_FORKS", "true")
	}

	if opts.Quiet {
		os.Setenv("GHORG_QUIET", "true")
	}

	if opts.NoToken {
		os.Setenv("GHORG_NO_TOKEN", "true")
	}

	if opts.NoDirSize {
		os.Setenv("GHORG_NO_DIR_SIZE", "true")
	}

	if opts.PreserveDir {
		os.Setenv("GHORG_PRESERVE_DIRECTORY_STRUCTURE", "true")
	}

	if opts.Backup {
		os.Setenv("GHORG_BACKUP", "true")
	}

	if opts.SyncDefaultBranch {
		os.Setenv("GHORG_SYNC_DEFAULT_BRANCH", "true")
	}

	if opts.OutputDir != "" {
		os.Setenv("GHORG_OUTPUT_DIR", opts.OutputDir)
	}

	if len(remaining) < 1 {
		if os.Getenv("GHORG_SCM_TYPE") == "github" && os.Getenv("GHORG_CLONE_TYPE") == "user" {
			remaining = append(remaining, "")
		} else {
			colorlog.PrintError("You must provide an org or user to clone")
			return 1
		}
	}

	configs.GetOrSetToken()

	if opts.Token != "" {
		token := opts.Token
		if configs.IsFilePath(token) {
			token = configs.GetTokenFromFile(token)
		}
		if os.Getenv("GHORG_SCM_TYPE") == "github" {
			os.Setenv("GHORG_GITHUB_TOKEN", token)
		} else if os.Getenv("GHORG_SCM_TYPE") == "gitlab" {
			os.Setenv("GHORG_GITLAB_TOKEN", token)
		} else if os.Getenv("GHORG_SCM_TYPE") == "bitbucket" {
			if opts.BitbucketUsername != "" {
				os.Setenv("GHORG_BITBUCKET_APP_PASSWORD", token)
			} else {
				os.Setenv("GHORG_BITBUCKET_OAUTH_TOKEN", token)
			}
		} else if os.Getenv("GHORG_SCM_TYPE") == "gitea" {
			os.Setenv("GHORG_GITEA_TOKEN", token)
		} else if os.Getenv("GHORG_SCM_TYPE") == "sourcehut" {
			os.Setenv("GHORG_SOURCEHUT_TOKEN", token)
		}
	}
	err = configs.VerifyTokenSet()
	if err != nil {
		colorlog.PrintError(err)
		return 1
	}

	err = configs.VerifyConfigsSetCorrectly()
	if err != nil {
		colorlog.PrintError(err)
		return 1
	}

	if os.Getenv("GHORG_PRESERVE_SCM_HOSTNAME") == "true" {
		updateAbsolutePathToCloneToWithHostname()
	}

	setOutputDirName(remaining)
	setOuputDirAbsolutePath()
	targetCloneSource = remaining[0]

	// Auto-adjust concurrency for clone delay before setup (silently)
	if _, _, shouldAdjust := shouldAutoAdjustConcurrency(); shouldAdjust {
		os.Setenv("GHORG_CONCURRENCY", "1")
		os.Setenv("GHORG_CONCURRENCY_AUTO_ADJUSTED", "true")
	}

	setupRepoClone()
	return 0
}
func setupRepoClone() {
	// Clear global slices and cached values at the start of each clone operation
	// to prevent memory leaks in long-running processes like reclone-server
	cloneErrors = nil
	cloneInfos = nil
	cachedDirSizeMB = 0
	isDirSizeCached = false

	var cloneTargets []scm.Repo
	var err error

	if os.Getenv("GHORG_CLONE_TYPE") == "org" {
		cloneTargets, err = getAllOrgCloneUrls()
	} else if os.Getenv("GHORG_CLONE_TYPE") == "user" {
		cloneTargets, err = getAllUserCloneUrls()
	} else {
		colorlog.PrintError("GHORG_CLONE_TYPE not set or unsupported")
		os.Exit(1)
	}

	if err != nil {
		colorlog.PrintError("Encountered an error, aborting")
		fmt.Println(err)
		os.Exit(1)
	}

	if len(cloneTargets) == 0 {
		colorlog.PrintInfo("No repos found for " + os.Getenv("GHORG_SCM_TYPE") + " " + os.Getenv("GHORG_CLONE_TYPE") + ": " + targetCloneSource + ", please verify you have sufficient permissions to clone target repos, double check spelling and try again.")
		os.Exit(0)
	}
	git := git.NewGit()
	CloneAllRepos(git, cloneTargets)
}

func getAllOrgCloneUrls() ([]scm.Repo, error) {
	return getCloneUrls(true)
}

func getAllUserCloneUrls() ([]scm.Repo, error) {
	return getCloneUrls(false)
}

func getCloneUrls(isOrg bool) ([]scm.Repo, error) {
	asciiTime()
	PrintConfigs()
	scmType := strings.ToLower(os.Getenv("GHORG_SCM_TYPE"))
	if len(scmType) == 0 {
		colorlog.PrintError("GHORG_SCM_TYPE not set")
		os.Exit(1)
	}
	client, err := scm.GetClient(scmType)
	if err != nil {
		colorlog.PrintError(err)
		os.Exit(1)
	}

	if isOrg {
		return client.GetOrgRepos(targetCloneSource)
	}

	return client.GetUserRepos(targetCloneSource)
}

func createDirIfNotExist() {
	if _, err := os.Stat(outputDirAbsolutePath); os.IsNotExist(err) {
		err = os.MkdirAll(os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO"), 0o700)
		if err != nil {
			panic(err)
		}
	}
}

func repoExistsLocally(repo scm.Repo) bool {
	if _, err := os.Stat(repo.HostPath); os.IsNotExist(err) {
		return false
	}

	return true
}

func getAppNameFromURL(url string) string {
	withGit := strings.Split(url, "/")
	appName := withGit[len(withGit)-1]
	split := strings.Split(appName, ".")
	return strings.Join(split[0:len(split)-1], ".")
}

func printRemainingMessages() {
	if len(cloneInfos) > 0 {
		colorlog.PrintInfo("\n============ Info ============\n")
		for _, i := range cloneInfos {
			colorlog.PrintInfo(i)
		}
	}

	if len(cloneErrors) > 0 {
		colorlog.PrintError("\n============ Issues ============\n")
		for _, e := range cloneErrors {
			colorlog.PrintError(e)
		}
	}
}

func readTargetReposFile() ([]string, error) {
	file, err := os.Open(os.Getenv("GHORG_TARGET_REPOS_PATH"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() != "" {
			lines = append(lines, scanner.Text())
		}
	}
	return lines, scanner.Err()
}

func readGhorgIgnore() ([]string, error) {
	file, err := os.Open(configs.GhorgIgnoreLocation())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() != "" {
			lines = append(lines, scanner.Text())
		}
	}
	return lines, scanner.Err()
}

func readGhorgOnly() ([]string, error) {
	file, err := os.Open(configs.GhorgOnlyLocation())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() != "" {
			lines = append(lines, scanner.Text())
		}
	}
	return lines, scanner.Err()
}

func hasRepoNameCollisions(repos []scm.Repo) (map[string]bool, bool) {

	repoNameWithCollisions := make(map[string]bool)

	if os.Getenv("GHORG_GITLAB_TOKEN") == "" {
		return repoNameWithCollisions, false
	}

	if os.Getenv("GHORG_PRESERVE_DIRECTORY_STRUCTURE") == "true" {
		return repoNameWithCollisions, false
	}

	hasCollisions := false

	for _, repo := range repos {

		// Snippets should never have collions because we append the snippet id to the directory name
		if repo.IsGitLabSnippet {
			continue
		}

		if repo.IsWiki {
			continue
		}

		if _, ok := repoNameWithCollisions[repo.Name]; ok {
			repoNameWithCollisions[repo.Name] = true
			hasCollisions = true
		} else {
			repoNameWithCollisions[repo.Name] = false
		}
	}

	return repoNameWithCollisions, hasCollisions
}

func printDryRun(repos []scm.Repo) {
	for _, repo := range repos {
		colorlog.PrintSubtleInfo(repo.URL + "\n")
	}
	count := len(repos)
	colorlog.PrintSuccess(fmt.Sprintf("%v repos to be cloned into: %s", count, outputDirAbsolutePath))

	if os.Getenv("GHORG_PRUNE") == "true" {

		if stat, err := os.Stat(outputDirAbsolutePath); err == nil && stat.IsDir() {
			// We check that the clone path exists, otherwise there would definitely be no pruning
			// to do.
			colorlog.PrintInfo("\nScanning for local clones that have been removed on remote...")

			repositories, err := getRelativePathRepositories(outputDirAbsolutePath)
			if err != nil {
				log.Fatal(err)
			}

			eligibleForPrune := 0
			for _, repository := range repositories {
				// for each item in the org's clone directory, let's make sure we found a
				// corresponding repo on the remote.
				if !sliceContainsNamedRepo(repos, repository) {
					eligibleForPrune++
					colorlog.PrintSubtleInfo(fmt.Sprintf("%s not found in remote.", repository))
				}
			}
			colorlog.PrintSuccess(fmt.Sprintf("Local clones eligible for pruning: %d", eligibleForPrune))
		}
	}
}

func trimCollisionFilename(filename string) string {
	maxLen := 248
	if len(filename) > maxLen {
		return filename[:strings.LastIndex(filename[:maxLen], "_")]
	}

	return filename
}

func getCloneableInventory(allRepos []scm.Repo) (int, int, int, int) {
	var wikis, snippets, repos, total int
	for _, repo := range allRepos {
		if repo.IsGitLabSnippet {
			snippets++
		} else if repo.IsWiki {
			wikis++
		} else {
			repos++
		}
	}
	total = repos + snippets + wikis
	return total, repos, snippets, wikis
}

func isGitRepository(path string) bool {
	stat, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && stat.IsDir()
}

func getRelativePathRepositories(root string) ([]string, error) {
	var relativePaths []string
	err := filepath.WalkDir(root, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != outputDirAbsolutePath && file.IsDir() && isGitRepository(path) {
			rel, err := filepath.Rel(outputDirAbsolutePath, path)
			if err != nil {
				return err
			}
			relativePaths = append(relativePaths, rel)
		}
		return nil
	})
	return relativePaths, err
}

// CloneAllRepos clones all repos
func CloneAllRepos(git git.Gitter, cloneTargets []scm.Repo) {
	// Initialize filter and apply all filtering
	filter := NewRepositoryFilter()
	cloneTargets = filter.ApplyAllFilters(cloneTargets)

	totalResourcesToClone, reposToCloneCount, snippetToCloneCount, wikisToCloneCount := getCloneableInventory(cloneTargets)

	if os.Getenv("GHORG_CLONE_WIKI") == "true" && os.Getenv("GHORG_CLONE_SNIPPETS") == "true" {
		m := fmt.Sprintf("%v resources to clone found in %v, %v repos, %v snippets, and %v wikis\n", totalResourcesToClone, targetCloneSource, snippetToCloneCount, reposToCloneCount, wikisToCloneCount)
		colorlog.PrintInfo(m)
	} else if os.Getenv("GHORG_CLONE_WIKI") == "true" {
		m := fmt.Sprintf("%v resources to clone found in %v, %v repos and %v wikis\n", totalResourcesToClone, targetCloneSource, reposToCloneCount, wikisToCloneCount)
		colorlog.PrintInfo(m)
	} else if os.Getenv("GHORG_CLONE_SNIPPETS") == "true" {
		m := fmt.Sprintf("%v resources to clone found in %v, %v repos and %v snippets\n", totalResourcesToClone, targetCloneSource, reposToCloneCount, snippetToCloneCount)
		colorlog.PrintInfo(m)
	} else {
		colorlog.PrintInfo(strconv.Itoa(reposToCloneCount) + " repos found in " + targetCloneSource + "\n")
	}

	// Show concurrency adjustment message if it was auto-adjusted
	if os.Getenv("GHORG_CONCURRENCY_AUTO_ADJUSTED") == "true" {
		if delaySeconds, hasDelay := getCloneDelaySeconds(); hasDelay {
			colorlog.PrintInfo(fmt.Sprintf("GHORG_CLONE_DELAY_SECONDS is set to %d seconds. Automatically setting GHORG_CONCURRENCY to 1 for predictable rate limiting.", delaySeconds))
		}
		// Clear the tracking variable
		os.Unsetenv("GHORG_CONCURRENCY_AUTO_ADJUSTED")
	}

	if os.Getenv("GHORG_DRY_RUN") == "true" {
		printDryRun(cloneTargets)
		return
	}

	createDirIfNotExist()

	// check for duplicate names will cause issues for some clone types on gitlab
	repoNameWithCollisions, hasCollisions := hasRepoNameCollisions(cloneTargets)

	l, err := strconv.Atoi(os.Getenv("GHORG_CONCURRENCY"))
	if err != nil {
		log.Fatal("Could not determine GHORG_CONCURRENCY")
	}

	limit := limiter.NewConcurrencyLimiter(l)

	// Initialize repository processor
	processor := NewRepositoryProcessor(git)

	for i := range cloneTargets {
		repo := cloneTargets[i]

		// We use this because we dont want spaces in the final directory, using the web address makes it more file friendly
		// In the case of root level snippets we use the title which will have spaces in it, the url uses an ID so its not possible to use name from url
		// With snippets that originate on repos, we use that repo name
		var repoSlug string
		if os.Getenv("GHORG_SCM_TYPE") == "sourcehut" {
			// The URL handling in getAppNameFromURL makes strong presumptions that the URL will end in an
			// extension like '.git', but this is not the case for sourcehut (and possibly other forges).
			repoSlug = repo.Name
		} else if repo.IsGitLabSnippet && !repo.IsGitLabRootLevelSnippet {
			repoSlug = getAppNameFromURL(repo.GitLabSnippetInfo.URLOfRepo)
		} else if repo.IsGitLabRootLevelSnippet {
			repoSlug = repo.Name
		} else {
			repoSlug = getAppNameFromURL(repo.URL)
		}

		if !isPathSegmentSafe(repoSlug) {
			log.Fatal("Unsafe path segment found in SCM output")
		}

		limit.Execute(func() {
			if repo.Path != "" && os.Getenv("GHORG_PRESERVE_DIRECTORY_STRUCTURE") == "true" {
				repoSlug = repo.Path
			}

			processor.ProcessRepository(&repo, repoNameWithCollisions, hasCollisions, repoSlug, i)
		})

	}

	limit.WaitAndClose()

	// Calculate total duration from command start (including SCM API calls) and set it on the processor
	totalDuration := time.Since(commandStartTime)
	totalDurationSeconds := int(totalDuration.Seconds() + 0.5) // Round to nearest second
	processor.SetTotalDuration(totalDurationSeconds)

	// Get statistics and untouched repos from processor
	stats := processor.GetStats()
	untouchedReposToPrune := processor.GetUntouchedRepos()
	var untouchedPrunes int

	if os.Getenv("GHORG_PRUNE_UNTOUCHED") == "true" && len(untouchedReposToPrune) > 0 {
		if os.Getenv("GHORG_PRUNE_UNTOUCHED_NO_CONFIRM") != "true" {
			colorlog.PrintSuccess(fmt.Sprintf("PLEASE CONFIRM: The following %d untouched repositories will be deleted. Press enter to confirm: ", len(untouchedReposToPrune)))
			for _, repoPath := range untouchedReposToPrune {
				colorlog.PrintInfo(fmt.Sprintf("- %s", repoPath))
			}
			fmt.Scanln()
		}

		for _, repoPath := range untouchedReposToPrune {
			err := os.RemoveAll(repoPath)
			if err != nil {
				colorlog.PrintError(fmt.Sprintf("Failed to prune repository at %s: %v", repoPath, err))
			} else {
				untouchedPrunes++
				colorlog.PrintSuccess(fmt.Sprintf("Successfully deleted %s", repoPath))
			}
		}
	}

	// Update global error/info arrays for backward compatibility
	cloneInfos = stats.CloneInfos
	cloneErrors = stats.CloneErrors

	printRemainingMessages()
	printCloneStatsMessage(stats.CloneCount, stats.PulledCount, stats.UpdateRemoteCount, stats.NewCommits, stats.SyncedCount, untouchedPrunes, stats.TotalDurationSeconds)

	if hasCollisions {
		fmt.Println("")
		colorlog.PrintInfo("ATTENTION: ghorg detected collisions in repo names from the groups that were cloned. This occurs when one or more groups share common repo names trying to be cloned to the same directory. The repos that would have collisions were renamed with the group/subgroup appended.")
		if os.Getenv("GHORG_DEBUG") != "" {
			fmt.Println("")
			colorlog.PrintInfo("Collisions Occured in the following repos...")
			for repoName, collision := range repoNameWithCollisions {
				if collision {
					colorlog.PrintInfo("- " + repoName)
				}
			}
		}
	}

	var pruneCount int
	cloneInfosCount := len(stats.CloneInfos)
	cloneErrorsCount := len(stats.CloneErrors)
	allReposToCloneCount := len(cloneTargets)
	// Now, clean up local repos that don't exist in remote, if prune flag is set
	if os.Getenv("GHORG_PRUNE") == "true" {
		pruneCount = pruneRepos(cloneTargets)
	}

	if os.Getenv("GHORG_QUIET") != "true" {
		if os.Getenv("GHORG_NO_DIR_SIZE") == "false" {
			printFinishedWithDirSize()
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("\nFinished! %s", outputDirAbsolutePath))
		}
	}

	// This needs to be called after printFinishedWithDirSize()
	if os.Getenv("GHORG_STATS_ENABLED") == "true" {
		date := time.Now().Format("2006-01-02 15:04:05")
		writeGhorgStats(date, allReposToCloneCount, stats.CloneCount, stats.PulledCount, cloneInfosCount, cloneErrorsCount, stats.UpdateRemoteCount, stats.NewCommits, stats.SyncedCount, pruneCount, stats.TotalDurationSeconds, hasCollisions)
	}

	if os.Getenv("GHORG_DONT_EXIT_UNDER_TEST") != "true" {
		if os.Getenv("GHORG_EXIT_CODE_ON_CLONE_INFOS") != "0" && cloneInfosCount > 0 {
			exitCode, err := strconv.Atoi(os.Getenv("GHORG_EXIT_CODE_ON_CLONE_INFOS"))
			if err != nil {
				colorlog.PrintError("Could not convert GHORG_EXIT_CODE_ON_CLONE_INFOS from string to integer")
				os.Exit(1)
			}

			os.Exit(exitCode)
		}
	}

	if os.Getenv("GHORG_DONT_EXIT_UNDER_TEST") != "true" {
		if cloneErrorsCount > 0 {
			exitCode, err := strconv.Atoi(os.Getenv("GHORG_EXIT_CODE_ON_CLONE_ISSUES"))
			if err != nil {
				colorlog.PrintError("Could not convert GHORG_EXIT_CODE_ON_CLONE_ISSUES from string to integer")
				os.Exit(1)
			}
			os.Exit(exitCode)
		}
	} else {
		cloneErrorsCount = 0
	}

}

func getGhorgStatsFilePath() string {
	var statsFilePath string
	absolutePath := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
	if os.Getenv("GHORG_PRESERVE_SCM_HOSTNAME") == "true" {
		originalAbsolutePath := os.Getenv("GHORG_ORIGINAL_ABSOLUTE_PATH_TO_CLONE_TO")
		statsFilePath = filepath.Join(originalAbsolutePath, "_ghorg_stats.csv")
	} else {
		statsFilePath = filepath.Join(absolutePath, "_ghorg_stats.csv")
	}

	return statsFilePath
}

func writeGhorgStats(date string, allReposToCloneCount, cloneCount, pulledCount, cloneInfosCount, cloneErrorsCount, updateRemoteCount, newCommits, syncedCount, pruneCount, totalDurationSeconds int, hasCollisions bool) error {

	statsFilePath := getGhorgStatsFilePath()
	fileExists := true

	if _, err := os.Stat(statsFilePath); os.IsNotExist(err) {
		fileExists = false
	}

	header := "datetime,clonePath,scm,cloneType,cloneTarget,totalCount,newClonesCount,existingResourcesPulledCount,dirSizeInMB,newCommits,syncedCount,cloneInfosCount,cloneErrorsCount,updateRemoteCount,pruneCount,hasCollisions,ghorgignore,ghorgonly,totalDurationSeconds,ghorgVersion\n"

	var file *os.File
	var err error

	if fileExists {
		// Read the existing header
		existingHeader, err := readFirstLine(statsFilePath)
		if err != nil {
			colorlog.PrintError(fmt.Sprintf("Error reading header from stats file: %v", err))
			return err
		}

		// Check if the existing header is different from the new header, need to add a newline
		if existingHeader+"\n" != header {
			hashedHeader := fmt.Sprintf("%x", sha256.Sum256([]byte(header)))
			newHeaderFilePath := filepath.Join(os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO"), fmt.Sprintf("ghorg_stats_new_header_%s.csv", hashedHeader))
			// Create a new file with the new header
			file, err = os.OpenFile(newHeaderFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				colorlog.PrintError(fmt.Sprintf("Error creating new header stats file: %v", err))
				return err
			}
			if _, err := file.WriteString(header); err != nil {
				colorlog.PrintError(fmt.Sprintf("Error writing new header to GHORG_STATS file: %v", err))
				return err
			}
		} else {
			// Open the existing file in append mode
			file, err = os.OpenFile(statsFilePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				colorlog.PrintError(fmt.Sprintf("Error opening stats file for appending: %v", err))
				return err
			}
		}
	} else {
		// Create the file and write the header
		file, err = os.OpenFile(statsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			colorlog.PrintError(fmt.Sprintf("Error creating stats file: %v", err))
			return err
		}
		if _, err := file.WriteString(header); err != nil {
			colorlog.PrintError(fmt.Sprintf("Error writing header to GHORG_STATS file: %v", err))
			return err
		}
	}
	defer file.Close()

	data := fmt.Sprintf("%v,%v,%v,%v,%v,%v,%v,%v,%.2f,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
		date,
		outputDirAbsolutePath,
		os.Getenv("GHORG_SCM_TYPE"),
		os.Getenv("GHORG_CLONE_TYPE"),
		targetCloneSource,
		allReposToCloneCount,
		cloneCount,
		pulledCount,
		cachedDirSizeMB,
		newCommits,
		syncedCount,
		cloneInfosCount,
		cloneErrorsCount,
		updateRemoteCount,
		pruneCount,
		hasCollisions,
		configs.GhorgIgnoreDetected(),
		configs.GhorgOnlyDetected(),
		totalDurationSeconds,
		GetVersion())
	if _, err := file.WriteString(data); err != nil {
		colorlog.PrintError(fmt.Sprintf("Error writing data to GHORG_STATS file: %v", err))
		return err
	}

	return nil
}

func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}

func printFinishedWithDirSize() {
	dirSizeMB, err := getCachedOrCalculatedOutputDirSizeInMb()
	if err != nil {
		if os.Getenv("GHORG_DEBUG") == "true" {
			colorlog.PrintError(fmt.Sprintf("Error calculating directory size: %v", err))
		}
		colorlog.PrintSuccess(fmt.Sprintf("\nFinished! %s", outputDirAbsolutePath))
		return
	}

	if dirSizeMB > 1000 {
		dirSizeGB := dirSizeMB / 1000
		colorlog.PrintSuccess(fmt.Sprintf("\nFinished! %s (Size: %.2f GB)", outputDirAbsolutePath, dirSizeGB))
	} else {
		colorlog.PrintSuccess(fmt.Sprintf("\nFinished! %s (Size: %.2f MB)", outputDirAbsolutePath, dirSizeMB))
	}
}

func getCachedOrCalculatedOutputDirSizeInMb() (float64, error) {
	if !isDirSizeCached {
		dirSizeMB, err := utils.CalculateDirSizeInMb(outputDirAbsolutePath)
		if err != nil {
			return 0, err
		}
		cachedDirSizeMB = dirSizeMB
		isDirSizeCached = true
	}
	return cachedDirSizeMB, nil
}

func filterByTargetReposPath(cloneTargets []scm.Repo) []scm.Repo {

	_, err := os.Stat(os.Getenv("GHORG_TARGET_REPOS_PATH"))

	if err != nil {
		colorlog.PrintErrorAndExit(fmt.Sprintf("Error finding your GHORG_TARGET_REPOS_PATH file, error: %v", err))
	}

	if !os.IsNotExist(err) {
		// Open the file parse each line and remove cloneTargets containing
		toTarget, err := readTargetReposFile()
		if err != nil {
			colorlog.PrintErrorAndExit(fmt.Sprintf("Error parsing your GHORG_TARGET_REPOS_PATH file, error: %v", err))
		}

		colorlog.PrintInfo("Using GHORG_TARGET_REPOS_PATH, filtering repos down...")

		filteredCloneTargets := []scm.Repo{}
		var flag bool

		targetRepoSeenOnOrg := make(map[string]bool)

		for _, cloneTarget := range cloneTargets {
			flag = false
			for _, targetRepo := range toTarget {
				if _, ok := targetRepoSeenOnOrg[targetRepo]; !ok {
					targetRepoSeenOnOrg[targetRepo] = false
				}
				clonedRepoName := strings.TrimSuffix(filepath.Base(cloneTarget.URL), ".git")
				if strings.EqualFold(clonedRepoName, targetRepo) {
					flag = true
					targetRepoSeenOnOrg[targetRepo] = true
				}

				if os.Getenv("GHORG_CLONE_WIKI") == "true" {
					targetRepoWiki := targetRepo + ".wiki"
					if strings.EqualFold(targetRepoWiki, clonedRepoName) {
						flag = true
						targetRepoSeenOnOrg[targetRepo] = true
					}
				}

				if os.Getenv("GHORG_CLONE_SNIPPETS") == "true" {
					if cloneTarget.IsGitLabSnippet {
						targetSnippetOriginalRepo := strings.TrimSuffix(filepath.Base(cloneTarget.GitLabSnippetInfo.URLOfRepo), ".git")
						if strings.EqualFold(targetSnippetOriginalRepo, targetRepo) {
							flag = true
							targetRepoSeenOnOrg[targetRepo] = true
						}
					}
				}
			}

			if flag {
				filteredCloneTargets = append(filteredCloneTargets, cloneTarget)
			}
		}

		// Print all the repos in the file that were not in the org so users know the entry is not being cloned
		for targetRepo, seen := range targetRepoSeenOnOrg {
			if !seen {
				cloneInfos = append(cloneInfos, fmt.Sprintf("Target in GHORG_TARGET_REPOS_PATH was not found in the org, repo: %v", targetRepo))
			}
		}

		cloneTargets = filteredCloneTargets

	}

	return cloneTargets
}

func pruneRepos(cloneTargets []scm.Repo) int {
	count := 0
	colorlog.PrintInfo("\nScanning for local clones that have been removed on remote...")

	repositories, err := getRelativePathRepositories(outputDirAbsolutePath)
	if err != nil {
		log.Fatal(err)
	}

	// The first time around, we set userAgreesToDelete to true, otherwise we'd immediately
	// break out of the loop.
	userAgreesToDelete := true
	pruneNoConfirm := os.Getenv("GHORG_PRUNE_NO_CONFIRM") == "true"
	for _, repository := range repositories {
		absolutePathToDelete := filepath.Join(outputDirAbsolutePath, repository)

		// Safeguard: Ensure the path is within the expected base directory
		if !strings.HasPrefix(absolutePathToDelete, outputDirAbsolutePath) {
			colorlog.PrintErrorAndExit(fmt.Sprintf("DANGEROUS ACTION DETECTED! Preventing deletion of %s as it is outside the base directory this deletion is not expected, exiting.", absolutePathToDelete))
		}

		// For each item in the org's clone directory, let's make sure we found a corresponding
		// repo on the remote.  We check userAgreesToDelete here too, so that if the user says
		// "No" at any time, we stop trying to prune things altogether.
		if userAgreesToDelete && !sliceContainsNamedRepo(cloneTargets, repository) {
			// If the user specified --prune-no-confirm, we needn't prompt interactively.
			userAgreesToDelete = pruneNoConfirm || interactiveYesNoPrompt(
				fmt.Sprintf("%s was not found in remote.  Do you want to prune it? %s", repository, absolutePathToDelete))
			if userAgreesToDelete {
				colorlog.PrintSubtleInfo(
					fmt.Sprintf("Deleting %s", absolutePathToDelete))
				err = os.RemoveAll(absolutePathToDelete)
				count++
				if err != nil {
					log.Fatal(err)
				}
			} else {
				colorlog.PrintError("Pruning cancelled by user.  No more prunes will be considered.")
			}
		}
	}

	return count
}

// formatDurationText formats duration in seconds to a human-readable string
func formatDurationText(durationSeconds int) string {
	if durationSeconds >= 60 {
		minutes := durationSeconds / 60
		seconds := durationSeconds % 60
		if seconds > 0 {
			return fmt.Sprintf(" (completed in %dm%ds)", minutes, seconds)
		} else {
			return fmt.Sprintf(" (completed in %dm)", minutes)
		}
	} else {
		return fmt.Sprintf(" (completed in %ds)", durationSeconds)
	}
}

func printCloneStatsMessage(cloneCount, pulledCount, updateRemoteCount, newCommits, syncedCount, untouchedPrunes, durationSeconds int) {
	durationText := formatDurationText(durationSeconds)

	if updateRemoteCount > 0 {
		if syncedCount > 0 {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total new commits: %v, remotes updated: %v, default branches synced: %v%s", cloneCount, pulledCount, newCommits, updateRemoteCount, syncedCount, durationText))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total new commits: %v, remotes updated: %v%s", cloneCount, pulledCount, newCommits, updateRemoteCount, durationText))
		}
		return
	}

	if newCommits > 0 {
		if syncedCount > 0 {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total new commits: %v, default branches synced: %v%s", cloneCount, pulledCount, newCommits, syncedCount, durationText))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total new commits: %v%s", cloneCount, pulledCount, newCommits, durationText))
		}
		return
	}

	if untouchedPrunes > 0 {
		if syncedCount > 0 {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total prunes: %v, default branches synced: %v%s", cloneCount, pulledCount, untouchedPrunes, syncedCount, durationText))
		} else {
			colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, total prunes: %v%s", cloneCount, pulledCount, untouchedPrunes, durationText))
		}
		return
	}

	if syncedCount > 0 {
		colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v, default branches synced: %v%s", cloneCount, pulledCount, syncedCount, durationText))
	} else {
		colorlog.PrintSuccess(fmt.Sprintf("New clones: %v, existing resources pulled: %v%s", cloneCount, pulledCount, durationText))
	}
}

func interactiveYesNoPrompt(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(strings.TrimSpace(prompt) + " (y/N) ")
	s, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if s == "y" || s == "yes" {
		return true
	}
	return false
}

// There's probably a nicer way of finding whether any scm.Repo in the slice matches a given name.
func sliceContainsNamedRepo(haystack []scm.Repo, needle string) bool {

	// GitLab Cloud vs GitLab on Prem seem to have different needle/repo.Paths when it comes to this,
	// so normalize to handle both
	// I'm not really sure whats going on here though, could be a bug with how this is set
	needle = strings.TrimPrefix(needle, "/")

	// Normalize path separators for cross-platform compatibility (Windows vs Unix)
	// Convert both needle and repo paths to use forward slashes for comparison
	// We need to handle both forward and back slashes regardless of OS
	needle = strings.ReplaceAll(needle, "\\", "/")
	needle = filepath.ToSlash(needle)

	for _, repo := range haystack {
		normalizedPath := strings.TrimPrefix(repo.Path, "/")
		// Convert repo path to forward slashes for comparison
		// We need to handle both forward and back slashes regardless of OS
		normalizedPath = strings.ReplaceAll(normalizedPath, "\\", "/")
		normalizedPath = filepath.ToSlash(normalizedPath)

		if normalizedPath == needle {
			if os.Getenv("GHORG_DEBUG") != "" {
				fmt.Printf("Debug: Match found for repo path: %s\n", repo.Path)
			}
			return true
		}
	}

	return false
}

func asciiTime() {
	colorlog.PrintInfo(
		`
 +-+-+-+-+ +-+-+ +-+-+-+-+-+
 |T|I|M|E| |T|O| |G|H|O|R|G|
 +-+-+-+-+ +-+-+ +-+-+-+-+-+
`)
}

// PrintConfigs shows the user what is set before cloning
func PrintConfigs() {
	if os.Getenv("GHORG_QUIET") == "true" {
		return
	}

	colorlog.PrintInfo("*************************************")
	colorlog.PrintInfo("* SCM           : " + os.Getenv("GHORG_SCM_TYPE"))
	colorlog.PrintInfo("* Type          : " + os.Getenv("GHORG_CLONE_TYPE"))
	colorlog.PrintInfo("* Protocol      : " + os.Getenv("GHORG_CLONE_PROTOCOL"))
	colorlog.PrintInfo("* Location      : " + os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO"))
	colorlog.PrintInfo("* Concurrency   : " + os.Getenv("GHORG_CONCURRENCY"))
	if delaySeconds, hasDelay := getCloneDelaySeconds(); hasDelay {
		colorlog.PrintInfo("* Clone Delay   : " + strconv.Itoa(delaySeconds) + " seconds")
	}

	if os.Getenv("GHORG_BRANCH") != "" {
		colorlog.PrintInfo("* Branch        : " + getGhorgBranch())
	}
	if os.Getenv("GHORG_SCM_BASE_URL") != "" {
		colorlog.PrintInfo("* Base URL      : " + os.Getenv("GHORG_SCM_BASE_URL"))
	}
	if os.Getenv("GHORG_SKIP_ARCHIVED") == "true" {
		colorlog.PrintInfo("* Skip Archived : " + os.Getenv("GHORG_SKIP_ARCHIVED"))
	}
	if os.Getenv("GHORG_SKIP_FORKS") == "true" {
		colorlog.PrintInfo("* Skip Forks    : " + os.Getenv("GHORG_SKIP_FORKS"))
	}
	if os.Getenv("GHORG_BACKUP") == "true" {
		colorlog.PrintInfo("* Backup        : " + os.Getenv("GHORG_BACKUP"))
	}
	if os.Getenv("GHORG_CLONE_WIKI") == "true" {
		colorlog.PrintInfo("* Wikis         : " + os.Getenv("GHORG_CLONE_WIKI"))
	}
	if os.Getenv("GHORG_CLONE_SNIPPETS") == "true" {
		colorlog.PrintInfo("* Snippets      : " + os.Getenv("GHORG_CLONE_SNIPPETS"))
	}
	if configs.GhorgIgnoreDetected() {
		colorlog.PrintInfo("* Ghorgignore   : " + configs.GhorgIgnoreLocation())
	}
	if configs.GhorgOnlyDetected() {
		colorlog.PrintInfo("* Ghorgonly     : " + configs.GhorgOnlyLocation())
	}
	if os.Getenv("GHORG_TARGET_REPOS_PATH") != "" {
		colorlog.PrintInfo("* Target Repos  : " + os.Getenv("GHORG_TARGET_REPOS_PATH"))
	}
	if os.Getenv("GHORG_MATCH_REGEX") != "" {
		colorlog.PrintInfo("* Regex Match   : " + os.Getenv("GHORG_MATCH_REGEX"))
	}
	if os.Getenv("GHORG_EXCLUDE_MATCH_REGEX") != "" {
		colorlog.PrintInfo("* Exclude Regex : " + os.Getenv("GHORG_EXCLUDE_MATCH_REGEX"))
	}
	if os.Getenv("GHORG_MATCH_PREFIX") != "" {
		colorlog.PrintInfo("* Prefix Match  : " + os.Getenv("GHORG_MATCH_PREFIX"))
	}
	if os.Getenv("GHORG_EXCLUDE_MATCH_PREFIX") != "" {
		colorlog.PrintInfo("* Exclude Prefix: " + os.Getenv("GHORG_EXCLUDE_MATCH_PREFIX"))
	}
	if os.Getenv("GHORG_INCLUDE_SUBMODULES") == "true" {
		colorlog.PrintInfo("* Submodules    : " + os.Getenv("GHORG_INCLUDE_SUBMODULES"))
	}
	if os.Getenv("GHORG_GIT_FILTER") != "" {
		colorlog.PrintInfo("* Git --filter= : " + os.Getenv("GHORG_GIT_FILTER"))
	}
	if os.Getenv("GHORG_OUTPUT_DIR") != "" {
		colorlog.PrintInfo("* Output Dir    : " + outputDirName)
	}
	if os.Getenv("GHORG_NO_CLEAN") == "true" {
		colorlog.PrintInfo("* No Clean      : " + "true")
	}
	if os.Getenv("GHORG_PRUNE") == "true" {
		noConfirmText := ""
		if os.Getenv("GHORG_PRUNE_NO_CONFIRM") == "true" {
			noConfirmText = " (skipping confirmation)"
		}
		colorlog.PrintInfo("* Prune         : " + "true" + noConfirmText)
	}
	if os.Getenv("GHORG_FETCH_ALL") == "true" {
		colorlog.PrintInfo("* Fetch All     : " + "true")
	}
	if os.Getenv("GHORG_DRY_RUN") == "true" {
		colorlog.PrintInfo("* Dry Run       : " + "true")
	}

	if os.Getenv("GHORG_RECLONE_PATH") != "" && os.Getenv("GHORG_RECLONE_RUNNING") == "true" {
		colorlog.PrintInfo("* Reclone Conf  : " + os.Getenv("GHORG_RECLONE_PATH"))
	}

	if os.Getenv("GHORG_PRESERVE_DIRECTORY_STRUCTURE") == "true" {
		colorlog.PrintInfo("* Preserve Dir  : " + "true")
	}

	if os.Getenv("GHORG_GITHUB_APP_PEM_PATH") != "" {
		colorlog.PrintInfo("* GH App Auth   : " + "true")
	}

	if os.Getenv("GHORG_CLONE_DEPTH") != "" {
		colorlog.PrintInfo("* Clone Depth   : " + os.Getenv("GHORG_CLONE_DEPTH"))
	}

	colorlog.PrintInfo("* Config Used   : " + os.Getenv("GHORG_CONFIG"))
	if os.Getenv("GHORG_STATS_ENABLED") == "true" {
		colorlog.PrintInfo("* Stats Enabled : " + os.Getenv("GHORG_STATS_ENABLED"))
	}
	colorlog.PrintInfo("* Ghorg version : " + GetVersion())

	colorlog.PrintInfo("*************************************")
}

func getGhorgBranch() string {
	if os.Getenv("GHORG_BRANCH") == "" {
		return "default branch"
	}

	return os.Getenv("GHORG_BRANCH")
}

func setOuputDirAbsolutePath() {
	outputDirAbsolutePath = filepath.Join(os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO"), outputDirName)
}

func setOutputDirName(argz []string) {
	if os.Getenv("GHORG_OUTPUT_DIR") != "" {
		outputDirName = os.Getenv("GHORG_OUTPUT_DIR")
		return
	}

	outputDirName = strings.ToLower(argz[0])

	// Strip ~ prefix for sourcehut usernames to avoid shell expansion issues
	if os.Getenv("GHORG_SCM_TYPE") == "sourcehut" {
		outputDirName = strings.TrimPrefix(outputDirName, "~")
	}

	if os.Getenv("GHORG_PRESERVE_SCM_HOSTNAME") != "true" {
		// If all-group is used set the parent folder to the name of the baseurl
		if argz[0] == "all-groups" && os.Getenv("GHORG_SCM_BASE_URL") != "" {
			u, err := url.Parse(os.Getenv("GHORG_SCM_BASE_URL"))
			if err != nil {
				colorlog.PrintError(fmt.Sprintf("Error parsing GHORG_SCM_BASE_URL, clone may be affected, error: %v", err))
			}
			outputDirName = u.Hostname()
		}

		if argz[0] == "all-users" && os.Getenv("GHORG_SCM_BASE_URL") != "" {
			u, err := url.Parse(os.Getenv("GHORG_SCM_BASE_URL"))
			if err != nil {
				colorlog.PrintError(fmt.Sprintf("Error parsing GHORG_SCM_BASE_URL, clone may be affected, error: %v", err))
			}
			outputDirName = u.Hostname()
		}
	}

	if os.Getenv("GHORG_BACKUP") == "true" {
		outputDirName = outputDirName + "_backup"
	}
}

// filter repos down based on ghorgignore if one exists
func filterByGhorgignore(cloneTargets []scm.Repo) []scm.Repo {

	_, err := os.Stat(configs.GhorgIgnoreLocation())
	if !os.IsNotExist(err) {
		// Open the file parse each line and remove cloneTargets containing
		toIgnore, err := readGhorgIgnore()
		if err != nil {
			colorlog.PrintErrorAndExit(fmt.Sprintf("Error parsing your ghorgignore, error: %v", err))
		}

		colorlog.PrintInfo("Using ghorgignore, filtering repos down...")

		filteredCloneTargets := []scm.Repo{}
		var flag bool
		for _, cloned := range cloneTargets {
			flag = false
			for _, ignore := range toIgnore {
				if strings.Contains(cloned.URL, ignore) {
					flag = true
				}
			}

			if !flag {
				filteredCloneTargets = append(filteredCloneTargets, cloned)
			}
		}

		cloneTargets = filteredCloneTargets

	}

	return cloneTargets
}

func isPathSegmentSafe(seg string) bool {
	return strings.IndexByte(seg, '/') < 0 && !strings.ContainsRune(seg, filepath.Separator)
}
