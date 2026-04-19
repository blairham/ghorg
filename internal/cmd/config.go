package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/configs"
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

// getDefaultValue returns the default value for a given environment variable.
// It first checks the config registry, then falls back to computed defaults.
func getDefaultValue(envVar string) string {
	ck := configs.LookupByEnvVar(envVar)
	if ck != nil && ck.DefaultValue != "" {
		return ck.DefaultValue
	}
	return ""
}

// resolveKoanfValue looks up a config value in koanf, checking both the new
// dot-notation key and the legacy GHORG_* flat key.
func resolveKoanfValue(envVar string) string {
	// Try new dot-notation key first (e.g., "scm.type")
	ck := configs.LookupByEnvVar(envVar)
	if ck != nil {
		if val := k.String(ck.DotNotation); val != "" {
			return val
		}
	}
	// Fall back to legacy flat key (e.g., "GHORG_SCM_TYPE")
	return k.String(envVar)
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

		if resolveKoanfValue(envVar) == "enabled" {
			os.Setenv("GHORG_COLOR", "enabled")
			return
		}
	}

	// When a user does not set value in config file, set the default values,
	// else set env to what they have added to the file.
	if os.Getenv(envVar) == "" {
		koanfResult := resolveKoanfValue(envVar)
		if koanfResult != "" {
			// Handle path-related env vars that need special formatting
			switch envVar {
			case "GHORG_SCM_BASE_URL":
				os.Setenv(envVar, configs.EnsureTrailingSlashOnURL(koanfResult))
			case "GHORG_ABSOLUTE_PATH_TO_CLONE_TO":
				os.Setenv(envVar, configs.EnsureTrailingSlashOnFilePath(koanfResult))
			default:
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
	legacyLocalConfig := filepath.Join(curDir, "ghorg.yaml")
	newLocalConfig := filepath.Join(curDir, ".ghorg", "config.yaml")

	var configFile string
	config := os.Getenv("GHORG_CONFIG")
	if config != "" {
		configFile = config
		os.Setenv("GHORG_CONFIG", config)
	} else if os.Getenv("GHORG_CONFIG") != "" {
		configFile = os.Getenv("GHORG_CONFIG")
	} else if _, err := os.Stat(legacyLocalConfig); !errors.Is(err, os.ErrNotExist) {
		configFile = legacyLocalConfig
		os.Setenv("GHORG_CONFIG", legacyLocalConfig)
	} else {
		configFile = configs.DefaultConfFile()
		os.Setenv("GHORG_CONFIG", configs.DefaultConfFile())
	}

	// Load the primary config file using Koanf
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		// Check if file doesn't exist
		if _, statErr := os.Stat(configFile); os.IsNotExist(statErr) {
			os.Setenv("GHORG_CONFIG", "none")
		} else {
			colorlog.PrintError(fmt.Sprintf("Something unexpected happened reading configuration file: %s, err: %s", os.Getenv("GHORG_CONFIG"), err))
			os.Exit(1)
		}
	}

	// Overlay local .ghorg/config.yaml if it exists and we didn't already load it
	if configFile != newLocalConfig {
		if _, err := os.Stat(newLocalConfig); err == nil {
			if loadErr := k.Load(file.Provider(newLocalConfig), yaml.Parser()); loadErr != nil {
				colorlog.PrintError(fmt.Sprintf("Error reading local config %s: %s", newLocalConfig, loadErr))
			}
		}
	}

	if os.Getenv("GHORG_DEBUG") != "" {
		fmt.Println("-------- Setting Default ENV values ---------")
		if os.Getenv("GHORG_CONCURRENCY_DEBUG") == "" {
			fmt.Println("Setting concurrency to 1, this can be overwritten by setting GHORG_CONCURRENCY_DEBUG; however when using concurrency with GHORG_DEBUG, not all debugging output will be printed in serial order.")
			os.Setenv("GHORG_CONCURRENCY", "1")
		}
	}

	// Apply all config keys from the registry
	for _, ck := range configs.AllKeys {
		getOrSetDefaults(ck.EnvVar)
	}

	if os.Getenv("GHORG_DEBUG") != "" {
		fmt.Println("Koanf config file used:", os.Getenv("GHORG_CONFIG"))
		fmt.Printf("GHORG_CONFIG SET TO: %s\n", os.Getenv("GHORG_CONFIG"))
		// Print all loaded config keys for debugging
		fmt.Println("Loaded config keys:", k.Keys())
	}

	updateAbsolutePathToCloneToWithHostname()
}
