package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/jessevdk/go-flags"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/blairham/ghorg/internal/configs"
)

// ConfigCmdCommand implements the `ghorg config` command, modeled after `git config`.
type ConfigCmdCommand struct {
	UI cli.Ui
}

type configFlags struct {
	Get     bool `long:"get" description:"Get the value of a config key"`
	Unset   bool `long:"unset" description:"Remove a config key"`
	List    bool `long:"list" short:"l" description:"List all config settings"`
	Global  bool `long:"global" description:"Use global config file (~/.config/ghorg/conf.yaml)"`
	Local   bool `long:"local" description:"Use local config file (.ghorg/config.yaml in current directory)"`
	Migrate bool `long:"migrate" description:"Convert legacy GHORG_* config file to new nested format"`
}

func (c *ConfigCmdCommand) Help() string {
	return `Usage: ghorg config [options] [<key> [<value>]]

Read or write ghorg configuration values. Works like git config.

Get a value:
  ghorg config scm.type
  ghorg config --get scm.type

Set a value:
  ghorg config scm.type github
  ghorg config clone.protocol ssh

Remove a value:
  ghorg config --unset scm.type

List all values:
  ghorg config --list
  ghorg config --list --global
  ghorg config --list --local

Migrate legacy config:
  ghorg config --migrate
  ghorg config --migrate --local

Options:
  --get          Get the value of a config key
  --unset        Remove a config key
  -l, --list     List all config settings
  --global       Use global config (~/.config/ghorg/conf.yaml)
  --local        Use local config (.ghorg/config.yaml in current directory)
  --migrate      Convert legacy GHORG_* config to new nested format

Configuration files:
  Global: ~/.config/ghorg/conf.yaml (default)
  Local:  .ghorg/config.yaml (in current directory)

  Local config values override global when both are present.

Available sections:
  core, scm, clone, auth, git, filter, prune, fetch,
  exit-code, stats, ssh, github, gitlab, bitbucket,
  gitea, sourcehut, reclone
`
}

func (c *ConfigCmdCommand) Synopsis() string {
	return "Get and set ghorg configuration values"
}

func (c *ConfigCmdCommand) Run(args []string) int {
	var opts configFlags
	parser := flags.NewParser(&opts, flags.Default&^flags.PrintErrors)
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

	// --migrate: convert legacy config to new format
	if opts.Migrate {
		return c.runMigrate(opts)
	}

	// --list: show all config
	if opts.List {
		return c.runList(opts)
	}

	// --unset <key>
	if opts.Unset {
		if len(remaining) < 1 {
			colorlog.PrintError("Usage: ghorg config --unset <key>")
			return 1
		}
		return c.runUnset(remaining[0], opts)
	}

	// No positional args and no action flags
	if len(remaining) == 0 {
		fmt.Println(c.Help())
		return 0
	}

	key := remaining[0]

	// Validate key exists in registry
	if configs.LookupByDot(key) == nil {
		// Check if it looks like a legacy env var
		if strings.HasPrefix(key, "GHORG_") {
			dot := configs.EnvVarToDot(key)
			if dot != "" {
				colorlog.PrintInfo(fmt.Sprintf("Hint: use %q instead of %q", dot, key))
				key = dot
			} else {
				colorlog.PrintError(fmt.Sprintf("Unknown config key: %s", key))
				return 1
			}
		} else {
			colorlog.PrintError(fmt.Sprintf("Unknown config key: %s", key))
			c.suggestKeys(key)
			return 1
		}
	}

	// ghorg config <key> <value> — set
	if len(remaining) >= 2 {
		value := remaining[1]
		return c.runSet(key, value, opts)
	}

	// ghorg config <key> or ghorg config --get <key> — get
	return c.runGet(key, opts)
}

func (c *ConfigCmdCommand) runGet(key string, opts configFlags) int {
	// Determine which files to check based on scope
	if opts.Local {
		path := localConfigPath()
		val, found, err := configs.ReadConfigValue(path, key)
		if err != nil {
			if os.IsNotExist(err) {
				colorlog.PrintError("No local config file found. Create one with: ghorg config --local <key> <value>")
				return 1
			}
			colorlog.PrintError(fmt.Sprintf("Error reading config: %v", err))
			return 1
		}
		if !found {
			return 1
		}
		fmt.Println(val)
		return 0
	}

	if opts.Global {
		path := configs.DefaultConfFile()
		val, found, err := configs.ReadConfigValue(path, key)
		if err != nil {
			if os.IsNotExist(err) {
				colorlog.PrintError("No global config file found")
				return 1
			}
			colorlog.PrintError(fmt.Sprintf("Error reading config: %v", err))
			return 1
		}
		if !found {
			return 1
		}
		fmt.Println(val)
		return 0
	}

	// Default: check local config, then primary config, then default
	// (matches git config behavior — reads from files, not env vars)
	localPath := localConfigPath()
	if val, found, err := configs.ReadConfigValue(localPath, key); err == nil && found {
		fmt.Println(val)
		return 0
	}

	primaryPath := c.targetConfigPath(configFlags{})
	if val, found, err := configs.ReadConfigValue(primaryPath, key); err == nil && found {
		fmt.Println(val)
		return 0
	}

	// Fall back to default
	ck := configs.LookupByDot(key)
	if ck != nil && ck.DefaultValue != "" {
		fmt.Println(ck.DefaultValue)
		return 0
	}

	return 1
}

func (c *ConfigCmdCommand) runSet(key, value string, opts configFlags) int {
	// Validate boolean keys
	ck := configs.LookupByDot(key)
	if ck != nil && ck.IsBool {
		if value != "true" && value != "false" {
			colorlog.PrintError(fmt.Sprintf("Key %q expects a boolean value (true/false), got %q", key, value))
			return 1
		}
	}

	path := c.targetConfigPath(opts)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		colorlog.PrintError(fmt.Sprintf("Error creating config directory: %v", err))
		return 1
	}

	if err := configs.WriteConfigValue(path, key, value); err != nil {
		colorlog.PrintError(fmt.Sprintf("Error writing config: %v", err))
		return 1
	}

	return 0
}

func (c *ConfigCmdCommand) runUnset(key string, opts configFlags) int {
	// Validate key
	if configs.LookupByDot(key) == nil {
		if strings.HasPrefix(key, "GHORG_") {
			dot := configs.EnvVarToDot(key)
			if dot != "" {
				key = dot
			}
		}
		if configs.LookupByDot(key) == nil {
			colorlog.PrintError(fmt.Sprintf("Unknown config key: %s", key))
			return 1
		}
	}

	path := c.targetConfigPath(opts)

	if err := configs.UnsetConfigValue(path, key); err != nil {
		if os.IsNotExist(err) {
			return 0 // nothing to unset
		}
		colorlog.PrintError(fmt.Sprintf("Error unsetting config: %v", err))
		return 1
	}

	return 0
}

func (c *ConfigCmdCommand) runList(opts configFlags) int {
	merged := make(map[string]string)

	if opts.Local {
		path := localConfigPath()
		values, err := configs.ListConfigValues(path)
		if err != nil {
			if os.IsNotExist(err) {
				colorlog.PrintError("No local config file found")
				return 1
			}
			colorlog.PrintError(fmt.Sprintf("Error reading config: %v", err))
			return 1
		}
		merged = values
	} else if opts.Global {
		path := configs.DefaultConfFile()
		values, err := configs.ListConfigValues(path)
		if err != nil {
			if os.IsNotExist(err) {
				colorlog.PrintError("No global config file found")
				return 1
			}
			colorlog.PrintError(fmt.Sprintf("Error reading config: %v", err))
			return 1
		}
		merged = values
	} else {
		// Merge: primary config first, then local overrides
		primaryPath := c.targetConfigPath(configFlags{})
		if primaryValues, err := configs.ListConfigValues(primaryPath); err == nil {
			for k, v := range primaryValues {
				merged[k] = v
			}
		}

		localPath := localConfigPath()
		if localValues, err := configs.ListConfigValues(localPath); err == nil {
			for k, v := range localValues {
				merged[k] = v
			}
		}
	}

	if len(merged) == 0 {
		colorlog.PrintInfo("No configuration values set")
		return 0
	}

	fmt.Print(configs.FormatConfigList(merged, false))
	return 0
}

// targetConfigPath returns the file path to read/write based on --global/--local flags.
// Without flags, uses GHORG_CONFIG if set, otherwise the global default.
func (c *ConfigCmdCommand) targetConfigPath(opts configFlags) string {
	if opts.Local {
		return localConfigPath()
	}
	if opts.Global {
		return configs.DefaultConfFile()
	}
	// Respect GHORG_CONFIG env var if set (and not "none")
	if cfg := os.Getenv("GHORG_CONFIG"); cfg != "" && cfg != "none" {
		return cfg
	}
	return configs.DefaultConfFile()
}

// localConfigPath returns the path to the local config file in the current directory.
func localConfigPath() string {
	curDir, _ := os.Getwd()
	return filepath.Join(curDir, ".ghorg", "config.yaml")
}

func (c *ConfigCmdCommand) runMigrate(opts configFlags) int {
	path := c.targetConfigPath(opts)

	// Read current file
	values, err := configs.ReadConfigFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			colorlog.PrintError(fmt.Sprintf("Config file not found: %s", path))
			return 1
		}
		colorlog.PrintError(fmt.Sprintf("Error reading config: %v", err))
		return 1
	}

	// Check if any keys are legacy GHORG_* format
	var legacyKeys []string
	for k := range values {
		if strings.HasPrefix(k, "GHORG_") {
			legacyKeys = append(legacyKeys, k)
		}
	}

	if len(legacyKeys) == 0 {
		colorlog.PrintSuccess("Config file is already in the new format, nothing to migrate")
		return 0
	}

	// Back up original
	backupPath := path + ".bak"
	data, err := os.ReadFile(path)
	if err != nil {
		colorlog.PrintError(fmt.Sprintf("Error reading config for backup: %v", err))
		return 1
	}
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		colorlog.PrintError(fmt.Sprintf("Error creating backup: %v", err))
		return 1
	}
	colorlog.PrintInfo(fmt.Sprintf("Backed up %s to %s", path, backupPath))

	// Build new config by translating legacy keys to dot notation
	migrated := make(map[string]string)
	var skipped []string
	for k, v := range values {
		if strings.HasPrefix(k, "GHORG_") {
			dot := configs.EnvVarToDot(k)
			if dot != "" {
				migrated[dot] = v
			} else {
				skipped = append(skipped, k)
			}
		} else {
			// Already a dot-notation or non-legacy key, keep as-is
			migrated[k] = v
		}
	}

	// Write new file by setting each key individually (builds nested YAML)
	// Start with empty file
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		colorlog.PrintError(fmt.Sprintf("Error writing config: %v", err))
		return 1
	}

	for dotKey, v := range migrated {
		if v == "" {
			continue // skip empty values
		}
		if err := configs.WriteConfigValue(path, dotKey, v); err != nil {
			colorlog.PrintError(fmt.Sprintf("Error writing key %s: %v", dotKey, err))
			return 1
		}
	}

	colorlog.PrintSuccess(fmt.Sprintf("Migrated %d keys to new format", len(legacyKeys)))

	if len(skipped) > 0 {
		colorlog.PrintInfo("Skipped unrecognized keys:")
		for _, k := range skipped {
			colorlog.PrintInfo(fmt.Sprintf("  %s", k))
		}
	}

	return 0
}

// suggestKeys prints similar keys when a typo is detected.
func (c *ConfigCmdCommand) suggestKeys(key string) {
	parts := strings.SplitN(key, ".", 2)
	section := parts[0]

	var suggestions []string
	for _, ck := range configs.AllKeys {
		if ck.Section() == section {
			suggestions = append(suggestions, ck.DotNotation)
		}
	}

	if len(suggestions) > 0 {
		colorlog.PrintInfo(fmt.Sprintf("Available keys in section %q:", section))
		for _, s := range suggestions {
			colorlog.PrintInfo(fmt.Sprintf("  %s", s))
		}
	}
}
