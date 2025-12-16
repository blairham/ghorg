package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/blairham/ghorg/colorlog"
	"github.com/blairham/ghorg/configs"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	// Global variables still needed for cloning operations
	outputDirName         string
	outputDirAbsolutePath string
	targetCloneSource     string
	cloneErrors           []string
	cloneInfos            []string
	cachedDirSizeMB       float64
	isDirSizeCached       bool
	commandStartTime      time.Time

	// Global koanf instance for configuration
	k = koanf.New(".")
)

func init() {
	// Initialize config on startup
	InitConfig()
}

func getHostname() string {
	var hostname string
	baseURL := os.Getenv("GHORG_SCM_BASE_URL")
	if baseURL != "" {
		// Parse the URL to extract the hostname
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			colorlog.PrintError(fmt.Sprintf("Error parsing GHORG_SCM_BASE_URL clone may be affected, error: %v", err))
		}
		// Append the hostname to the absolute path
		hostname = parsedURL.Hostname()
	} else {
		// Use the predefined hostname based on the SCM type
		hostname = configs.GetCloudScmTypeHostnames()
	}

	return hostname
}

// updateAbsolutePathToCloneToWithHostname modifies the absolute path by appending the hostname if the user has enabled it,
// supporting the GHORG_PRESERVE_SCM_HOSTNAME feature. It checks the GHORG_PRESERVE_SCM_HOSTNAME environment variable, and if set to "true",
// it uses the hostname from GHORG_SCM_BASE_URL if available, otherwise, it defaults to a predefined hostname based on the SCM type.
func updateAbsolutePathToCloneToWithHostname() {
	// Verify if GHORG_PRESERVE_SCM_HOSTNAME is set to "true"
	if os.Getenv("GHORG_PRESERVE_SCM_HOSTNAME") == "true" {
		// Retrieve the hostname from the environment variable
		hostname := getHostname()
		absolutePath := os.Getenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
		os.Setenv("GHORG_ORIGINAL_ABSOLUTE_PATH_TO_CLONE_TO", absolutePath)
		absolutePath = filepath.Join(absolutePath, hostname)
		os.Setenv("GHORG_ABSOLUTE_PATH_TO_CLONE_TO", configs.EnsureTrailingSlashOnFilePath(absolutePath))
	}
}

// getDefaultValue returns the default value for a given environment variable
func getDefaultValue(envVar string) string {
	defaults := map[string]string{
		"GHORG_SCM_TYPE":                     "github",
		"GHORG_CLONE_PROTOCOL":               "https",
		"GHORG_CLONE_TYPE":                   "org",
		"GHORG_GITHUB_USER_OPTION":           "owner",
		"GHORG_SYNC_DEFAULT_BRANCH":          "false",
		"GHORG_SKIP_ARCHIVED":                "false",
		"GHORG_SKIP_FORKS":                   "false",
		"GHORG_NO_CLEAN":                     "false",
		"GHORG_NO_TOKEN":                     "false",
		"GHORG_NO_DIR_SIZE":                  "false",
		"GHORG_FETCH_ALL":                    "false",
		"GHORG_PRUNE":                        "false",
		"GHORG_PRUNE_NO_CONFIRM":             "false",
		"GHORG_PRUNE_UNTOUCHED":              "false",
		"GHORG_PRUNE_UNTOUCHED_NO_CONFIRM":   "false",
		"GHORG_DRY_RUN":                      "false",
		"GHORG_CLONE_WIKI":                   "false",
		"GHORG_CLONE_SNIPPETS":               "false",
		"GHORG_INSECURE_GITLAB_CLIENT":       "false",
		"GHORG_INSECURE_GITEA_CLIENT":        "false",
		"GHORG_INSECURE_BITBUCKET_CLIENT":    "false",
		"GHORG_INSECURE_SOURCEHUT_CLIENT":    "false",
		"GHORG_BACKUP":                       "false",
		"GHORG_RECLONE_ENV_CONFIG_ONLY":      "false",
		"GHORG_RECLONE_QUIET":                "false",
		"GHORG_INCLUDE_SUBMODULES":           "false",
		"GHORG_EXIT_CODE_ON_CLONE_INFOS":     "0",
		"GHORG_EXIT_CODE_ON_CLONE_ISSUES":    "1",
		"GHORG_STATS_ENABLED":                "false",
		"GHORG_PRESERVE_DIRECTORY_STRUCTURE": "false",
		"GHORG_PRESERVE_SCM_HOSTNAME":        "false",
		"GHORG_QUIET":                        "false",
		"GHORG_CONCURRENCY":                  "25",
		"GHORG_CLONE_DELAY_SECONDS":          "0",
		"GHORG_CRON_TIMER_MINUTES":           "60",
		"GHORG_RECLONE_SERVER_PORT":          ":8080",
		"GHORG_COLOR":                        "disabled",
		"GHORG_GITHUB_TOKEN_FROM_GITHUB_APP": "false",
	}
	return defaults[envVar]
}

// reads in configuration file and updates anything not set to default
func getOrSetDefaults(envVar string) {
	if envVar == "GHORG_COLOR" {
		color := os.Getenv("GHORG_COLOR")
		if color == "enabled" {
			os.Setenv("GHORG_COLOR", "enabled")
			return
		}

		if color == "disabled" {
			os.Setenv("GHORG_COLOR", "disabled")
			return
		}

		if k.String(envVar) == "enabled" {
			os.Setenv("GHORG_COLOR", "enabled")
			return
		}
	}

	// When a user does not set value in $HOME/.config/ghorg/conf.yaml set the default values, else set env to what they have added to the file.
	if os.Getenv(envVar) == "" {
		koanfResult := k.String(envVar)
		if koanfResult != "" {
			// Handle path-related env vars that need special formatting
			if envVar == "GHORG_SCM_BASE_URL" {
				os.Setenv(envVar, configs.EnsureTrailingSlashOnURL(koanfResult))
			} else if envVar == "GHORG_ABSOLUTE_PATH_TO_CLONE_TO" {
				os.Setenv(envVar, configs.EnsureTrailingSlashOnFilePath(koanfResult))
			} else {
				os.Setenv(envVar, koanfResult)
			}
		} else {
			// If not in config file, use hard-coded default or computed default
			switch envVar {
			case "GHORG_ABSOLUTE_PATH_TO_CLONE_TO":
				os.Setenv(envVar, configs.GetAbsolutePathToCloneTo())
			case "GHORG_IGNORE_PATH":
				os.Setenv(envVar, configs.GhorgIgnoreLocation())
			case "GHORG_RECLONE_PATH":
				os.Setenv(envVar, configs.GhorgReCloneLocation())
			default:
				defaultValue := getDefaultValue(envVar)
				if defaultValue != "" {
					os.Setenv(envVar, defaultValue)
				}
			}
		}
	}
}

func InitConfig() {
	// Reset koanf instance for testing
	k = koanf.New(".")
	
	curDir, _ := os.Getwd()
	localConfig := filepath.Join(curDir, "ghorg.yaml")

	var configFile string
	config := os.Getenv("GHORG_CONFIG")
	if config != "" {
		configFile = config
		os.Setenv("GHORG_CONFIG", config)
	} else if os.Getenv("GHORG_CONFIG") != "" {
		configFile = os.Getenv("GHORG_CONFIG")
	} else if _, err := os.Stat(localConfig); !errors.Is(err, os.ErrNotExist) {
		configFile = localConfig
		os.Setenv("GHORG_CONFIG", localConfig)
	} else {
		configFile = configs.DefaultConfFile()
		os.Setenv("GHORG_CONFIG", configs.DefaultConfFile())
	}

	// Load the config file using Koanf
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		// Check if file doesn't exist
		if _, statErr := os.Stat(configFile); os.IsNotExist(statErr) {
			os.Setenv("GHORG_CONFIG", "none")
		} else {
			colorlog.PrintError(fmt.Sprintf("Something unexpected happened reading configuration file: %s, err: %s", os.Getenv("GHORG_CONFIG"), err))
			os.Exit(1)
		}
	}

	if os.Getenv("GHORG_DEBUG") != "" {
		fmt.Println("-------- Setting Default ENV values ---------")
		if os.Getenv("GHORG_CONCURRENCY_DEBUG") == "" {
			fmt.Println("Setting concurrency to 1, this can be overwritten by setting GHORG_CONCURRENCY_DEBUG; however when using concurrency with GHORG_DEBUG, not all debugging output will be printed in serial order.")
			os.Setenv("GHORG_CONCURRENCY", "1")
		}
	}

	getOrSetDefaults("GHORG_ABSOLUTE_PATH_TO_CLONE_TO")
	getOrSetDefaults("GHORG_BRANCH")
	getOrSetDefaults("GHORG_CLONE_PROTOCOL")
	getOrSetDefaults("GHORG_CLONE_TYPE")
	getOrSetDefaults("GHORG_SCM_TYPE")
	getOrSetDefaults("GHORG_PRESERVE_SCM_HOSTNAME")
	getOrSetDefaults("GHORG_SKIP_ARCHIVED")
	getOrSetDefaults("GHORG_SKIP_FORKS")
	getOrSetDefaults("GHORG_NO_CLEAN")
	getOrSetDefaults("GHORG_NO_TOKEN")
	getOrSetDefaults("GHORG_NO_DIR_SIZE")
	getOrSetDefaults("GHORG_FETCH_ALL")
	getOrSetDefaults("GHORG_PRUNE")
	getOrSetDefaults("GHORG_PRUNE_NO_CONFIRM")
	getOrSetDefaults("GHORG_PRUNE_UNTOUCHED")
	getOrSetDefaults("GHORG_PRUNE_UNTOUCHED_NO_CONFIRM")
	getOrSetDefaults("GHORG_DRY_RUN")
	getOrSetDefaults("GHORG_GITHUB_USER_OPTION")
	getOrSetDefaults("GHORG_CLONE_WIKI")
	getOrSetDefaults("GHORG_CLONE_SNIPPETS")
	getOrSetDefaults("GHORG_INSECURE_GITLAB_CLIENT")
	getOrSetDefaults("GHORG_INSECURE_GITEA_CLIENT")
	getOrSetDefaults("GHORG_INSECURE_BITBUCKET_CLIENT")
	getOrSetDefaults("GHORG_INSECURE_SOURCEHUT_CLIENT")
	getOrSetDefaults("GHORG_BACKUP")
	getOrSetDefaults("GHORG_SYNC_DEFAULT_BRANCH")
	getOrSetDefaults("GHORG_RECLONE_ENV_CONFIG_ONLY")
	getOrSetDefaults("GHORG_RECLONE_QUIET")
	getOrSetDefaults("GHORG_CONCURRENCY")
	getOrSetDefaults("GHORG_CLONE_DELAY_SECONDS")
	getOrSetDefaults("GHORG_INCLUDE_SUBMODULES")
	getOrSetDefaults("GHORG_EXIT_CODE_ON_CLONE_INFOS")
	getOrSetDefaults("GHORG_EXIT_CODE_ON_CLONE_ISSUES")
	getOrSetDefaults("GHORG_STATS_ENABLED")
	getOrSetDefaults("GHORG_CRON_TIMER_MINUTES")
	getOrSetDefaults("GHORG_RECLONE_SERVER_PORT")
	// Optionally set
	getOrSetDefaults("GHORG_TARGET_REPOS_PATH")
	getOrSetDefaults("GHORG_CLONE_DEPTH")
	getOrSetDefaults("GHORG_GITHUB_TOKEN")
	getOrSetDefaults("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP")
	getOrSetDefaults("GHORG_GITHUB_FILTER_LANGUAGE")
	getOrSetDefaults("GHORG_COLOR")
	getOrSetDefaults("GHORG_TOPICS")
	getOrSetDefaults("GHORG_GITLAB_TOKEN")
	getOrSetDefaults("GHORG_BITBUCKET_USERNAME")
	getOrSetDefaults("GHORG_BITBUCKET_APP_PASSWORD")
	getOrSetDefaults("GHORG_BITBUCKET_OAUTH_TOKEN")
	getOrSetDefaults("GHORG_SCM_BASE_URL")
	getOrSetDefaults("GHORG_PRESERVE_DIRECTORY_STRUCTURE")
	getOrSetDefaults("GHORG_OUTPUT_DIR")
	getOrSetDefaults("GHORG_MATCH_REGEX")
	getOrSetDefaults("GHORG_EXCLUDE_MATCH_REGEX")
	getOrSetDefaults("GHORG_MATCH_PREFIX")
	getOrSetDefaults("GHORG_EXCLUDE_MATCH_PREFIX")
	getOrSetDefaults("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX")
	getOrSetDefaults("GHORG_IGNORE_PATH")
	getOrSetDefaults("GHORG_RECLONE_PATH")
	getOrSetDefaults("GHORG_QUIET")
	getOrSetDefaults("GHORG_GIT_FILTER")
	getOrSetDefaults("GHORG_GITEA_TOKEN")
	getOrSetDefaults("GHORG_SOURCEHUT_TOKEN")
	getOrSetDefaults("GHORG_INSECURE_GITEA_CLIENT")
	getOrSetDefaults("GHORG_GITHUB_APP_PEM_PATH")
	getOrSetDefaults("GHORG_GITHUB_APP_INSTALLATION_ID")
	getOrSetDefaults("GHORG_GITHUB_APP_ID")

	if os.Getenv("GHORG_DEBUG") != "" {
		fmt.Println("Koanf config file used:", os.Getenv("GHORG_CONFIG"))
		fmt.Printf("GHORG_CONFIG SET TO: %s\n", os.Getenv("GHORG_CONFIG"))
		// Print all loaded config keys for debugging
		fmt.Println("Loaded config keys:", k.Keys())
	}

	updateAbsolutePathToCloneToWithHostname()
}
