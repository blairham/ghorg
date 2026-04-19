package configs

// ConfigKey represents a single configuration key with all its representations.
type ConfigKey struct {
	// DotNotation is the canonical key name (e.g., "scm.type").
	DotNotation string

	// EnvVar is the legacy environment variable name (e.g., "GHORG_SCM_TYPE").
	EnvVar string

	// DefaultValue is the hard-coded default, empty string means no default.
	DefaultValue string

	// IsBool indicates this key stores a boolean value ("true"/"false").
	IsBool bool

	// IsSecret indicates this key stores sensitive data (tokens, passwords).
	// Secret values are redacted in `ghorg config --list` output.
	IsSecret bool

	// Description is a short human-readable description of the key.
	Description string
}

// Section returns the top-level section name (everything before the first dot).
func (c ConfigKey) Section() string {
	for i, ch := range c.DotNotation {
		if ch == '.' {
			return c.DotNotation[:i]
		}
	}
	return c.DotNotation
}

// YAMLPath returns the nested YAML key path (e.g., ["scm", "type"]).
func (c ConfigKey) YAMLPath() []string {
	parts := make([]string, 0, 2)
	start := 0
	for i, ch := range c.DotNotation {
		if ch == '.' {
			parts = append(parts, c.DotNotation[start:i])
			start = i + 1
		}
	}
	parts = append(parts, c.DotNotation[start:])
	return parts
}

// AllKeys is the complete registry of configuration keys.
//
//nolint:gochecknoglobals // Registry is intentionally global as a single source of truth.
var AllKeys = []ConfigKey{
	// ── Core ──────────────────────────────────────────────────────────────
	{
		DotNotation:  "core.path",
		EnvVar:       "GHORG_ABSOLUTE_PATH_TO_CLONE_TO",
		DefaultValue: "", // computed at runtime via GetAbsolutePathToCloneTo()
		Description:  "Absolute path to clone repositories into",
	},
	{
		DotNotation:  "core.output-dir",
		EnvVar:       "GHORG_OUTPUT_DIR",
		DefaultValue: "",
		Description:  "Override the output directory name (defaults to org/user name)",
	},
	{
		DotNotation:  "core.concurrency",
		EnvVar:       "GHORG_CONCURRENCY",
		DefaultValue: "25",
		Description:  "Maximum number of concurrent clone operations",
	},
	{
		DotNotation:  "core.color",
		EnvVar:       "GHORG_COLOR",
		DefaultValue: "disabled",
		Description:  "Enable colored output (enabled/disabled)",
	},
	{
		DotNotation:  "core.quiet",
		EnvVar:       "GHORG_QUIET",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Emit only critical output",
	},
	{
		DotNotation:  "core.dry-run",
		EnvVar:       "GHORG_DRY_RUN",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Show what would be cloned without actually cloning",
	},
	{
		DotNotation:  "core.no-dir-size",
		EnvVar:       "GHORG_NO_DIR_SIZE",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip directory size calculation after cloning",
	},

	// ── SCM ──────────────────────────────────────────────────────────────
	{
		DotNotation:  "scm.type",
		EnvVar:       "GHORG_SCM_TYPE",
		DefaultValue: "github",
		Description:  "Source code management provider (github, gitlab, gitea, bitbucket, sourcehut)",
	},
	{
		DotNotation:  "scm.base-url",
		EnvVar:       "GHORG_SCM_BASE_URL",
		DefaultValue: "",
		Description:  "Base URL for self-hosted SCM instances",
	},

	// ── Clone ────────────────────────────────────────────────────────────
	{
		DotNotation:  "clone.protocol",
		EnvVar:       "GHORG_CLONE_PROTOCOL",
		DefaultValue: "https",
		Description:  "Clone protocol (https or ssh)",
	},
	{
		DotNotation:  "clone.type",
		EnvVar:       "GHORG_CLONE_TYPE",
		DefaultValue: "org",
		Description:  "Clone target type (org or user)",
	},
	{
		DotNotation:  "clone.branch",
		EnvVar:       "GHORG_BRANCH",
		DefaultValue: "",
		Description:  "Branch to checkout after cloning",
	},
	{
		DotNotation:  "clone.depth",
		EnvVar:       "GHORG_CLONE_DEPTH",
		DefaultValue: "",
		Description:  "Shallow clone depth (empty for full clone)",
	},
	{
		DotNotation:  "clone.delay-seconds",
		EnvVar:       "GHORG_CLONE_DELAY_SECONDS",
		DefaultValue: "0",
		Description:  "Delay in seconds between clone operations (forces concurrency to 1)",
	},
	{
		DotNotation:  "clone.wiki",
		EnvVar:       "GHORG_CLONE_WIKI",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Clone wiki repositories",
	},
	{
		DotNotation:  "clone.snippets",
		EnvVar:       "GHORG_CLONE_SNIPPETS",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Clone GitLab snippets",
	},
	{
		DotNotation:  "clone.backup",
		EnvVar:       "GHORG_BACKUP",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Mirror clone for backup purposes",
	},
	{
		DotNotation:  "clone.no-clean",
		EnvVar:       "GHORG_NO_CLEAN",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip git clean on existing repositories",
	},
	{
		DotNotation:  "clone.include-submodules",
		EnvVar:       "GHORG_INCLUDE_SUBMODULES",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Initialize and update submodules after cloning",
	},
	{
		DotNotation:  "clone.sync-default-branch",
		EnvVar:       "GHORG_SYNC_DEFAULT_BRANCH",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Sync local default branch with remote on existing repos",
	},
	{
		DotNotation:  "clone.preserve-dir",
		EnvVar:       "GHORG_PRESERVE_DIRECTORY_STRUCTURE",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Preserve GitLab namespace directory structure",
	},
	{
		DotNotation:  "clone.preserve-scm-hostname",
		EnvVar:       "GHORG_PRESERVE_SCM_HOSTNAME",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Append SCM hostname to clone path",
	},
	{
		DotNotation:  "clone.protect-local",
		EnvVar:       "GHORG_PROTECT_LOCAL",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip repos with uncommitted changes or unpushed commits",
	},

	// ── Auth ─────────────────────────────────────────────────────────────
	{
		DotNotation:  "auth.no-token",
		EnvVar:       "GHORG_NO_TOKEN",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Clone without authentication",
	},

	// ── Git ──────────────────────────────────────────────────────────────
	{
		DotNotation:  "git.filter",
		EnvVar:       "GHORG_GIT_FILTER",
		DefaultValue: "",
		Description:  "Git filter options (e.g., blob:none for partial clones)",
	},
	{
		DotNotation:  "git.backend",
		EnvVar:       "GHORG_GIT_BACKEND",
		DefaultValue: "golang",
		Description:  "Git backend: golang (pure Go) or exec (system git)",
	},

	// ── Filter ───────────────────────────────────────────────────────────
	{
		DotNotation:  "filter.skip-archived",
		EnvVar:       "GHORG_SKIP_ARCHIVED",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip archived repositories",
	},
	{
		DotNotation:  "filter.skip-forks",
		EnvVar:       "GHORG_SKIP_FORKS",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip forked repositories",
	},
	{
		DotNotation:  "filter.topics",
		EnvVar:       "GHORG_TOPICS",
		DefaultValue: "",
		Description:  "Comma-separated topic filter",
	},
	{
		DotNotation:  "filter.match-prefix",
		EnvVar:       "GHORG_MATCH_PREFIX",
		DefaultValue: "",
		Description:  "Include repos matching prefix (comma-separated, case-insensitive)",
	},
	{
		DotNotation:  "filter.exclude-match-prefix",
		EnvVar:       "GHORG_EXCLUDE_MATCH_PREFIX",
		DefaultValue: "",
		Description:  "Exclude repos matching prefix (comma-separated, case-insensitive)",
	},
	{
		DotNotation:  "filter.match-regex",
		EnvVar:       "GHORG_MATCH_REGEX",
		DefaultValue: "",
		Description:  "Include repos matching regex pattern",
	},
	{
		DotNotation:  "filter.exclude-match-regex",
		EnvVar:       "GHORG_EXCLUDE_MATCH_REGEX",
		DefaultValue: "",
		Description:  "Exclude repos matching regex pattern",
	},
	{
		DotNotation:  "filter.ignore-path",
		EnvVar:       "GHORG_IGNORE_PATH",
		DefaultValue: "", // computed at runtime via GhorgIgnoreLocation()
		Description:  "Path to ghorgignore file",
	},
	{
		DotNotation:  "filter.only-path",
		EnvVar:       "GHORG_ONLY_PATH",
		DefaultValue: "",
		Description:  "Path to ghorgonly file",
	},
	{
		DotNotation:  "filter.target-repos-path",
		EnvVar:       "GHORG_TARGET_REPOS_PATH",
		DefaultValue: "",
		Description:  "Path to file listing specific repos to clone",
	},

	// ── Prune ────────────────────────────────────────────────────────────
	{
		DotNotation:  "prune.enabled",
		EnvVar:       "GHORG_PRUNE",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Delete local repos not found on remote",
	},
	{
		DotNotation:  "prune.no-confirm",
		EnvVar:       "GHORG_PRUNE_NO_CONFIRM",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip confirmation when pruning",
	},
	{
		DotNotation:  "prune.untouched",
		EnvVar:       "GHORG_PRUNE_UNTOUCHED",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Prune repos that were not cloned or updated",
	},
	{
		DotNotation:  "prune.untouched-no-confirm",
		EnvVar:       "GHORG_PRUNE_UNTOUCHED_NO_CONFIRM",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip confirmation when pruning untouched repos",
	},

	// ── Fetch ────────────────────────────────────────────────────────────
	{
		DotNotation:  "fetch.all",
		EnvVar:       "GHORG_FETCH_ALL",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Fetch all remote branches",
	},
	{
		DotNotation:  "fetch.prune",
		EnvVar:       "GHORG_FETCH_PRUNE",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Append --prune to fetch (requires fetch.all)",
	},

	// ── Exit Codes ───────────────────────────────────────────────────────
	{
		DotNotation:  "exit-code.clone-infos",
		EnvVar:       "GHORG_EXIT_CODE_ON_CLONE_INFOS",
		DefaultValue: "0",
		Description:  "Exit code when clone info messages are present",
	},
	{
		DotNotation:  "exit-code.clone-issues",
		EnvVar:       "GHORG_EXIT_CODE_ON_CLONE_ISSUES",
		DefaultValue: "1",
		Description:  "Exit code when clone errors occur",
	},

	// ── Stats ────────────────────────────────────────────────────────────
	{
		DotNotation:  "stats.enabled",
		EnvVar:       "GHORG_STATS_ENABLED",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Generate a stats CSV file after cloning",
	},

	// ── SSH ──────────────────────────────────────────────────────────────
	{
		DotNotation:  "ssh.hostname",
		EnvVar:       "GHORG_SSH_HOSTNAME",
		DefaultValue: "",
		Description:  "Custom SSH hostname alias for clone URLs",
	},

	// ── GitHub ───────────────────────────────────────────────────────────
	{
		DotNotation:  "github.token",
		EnvVar:       "GHORG_GITHUB_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "GitHub personal access token",
	},
	{
		DotNotation:  "github.token-from-app",
		EnvVar:       "GHORG_GITHUB_TOKEN_FROM_GITHUB_APP",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Treat token as a GitHub App token",
	},
	{
		DotNotation:  "github.app-pem-path",
		EnvVar:       "GHORG_GITHUB_APP_PEM_PATH",
		DefaultValue: "",
		Description:  "Path to GitHub App PEM private key file",
	},
	{
		DotNotation:  "github.app-installation-id",
		EnvVar:       "GHORG_GITHUB_APP_INSTALLATION_ID",
		DefaultValue: "",
		Description:  "GitHub App installation ID",
	},
	{
		DotNotation:  "github.app-id",
		EnvVar:       "GHORG_GITHUB_APP_ID",
		DefaultValue: "",
		Description:  "GitHub App application ID",
	},
	{
		DotNotation:  "github.user-option",
		EnvVar:       "GHORG_GITHUB_USER_OPTION",
		DefaultValue: "owner",
		Description:  "GitHub user repo filter: owner, member, or all",
	},
	{
		DotNotation:  "github.filter-language",
		EnvVar:       "GHORG_GITHUB_FILTER_LANGUAGE",
		DefaultValue: "",
		Description:  "Filter GitHub repos by language (comma-separated)",
	},
	{
		DotNotation:  "github.user-gists",
		EnvVar:       "GHORG_GITHUB_USER_GISTS",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Clone GitHub user gists",
	},

	// ── GitLab ───────────────────────────────────────────────────────────
	{
		DotNotation:  "gitlab.token",
		EnvVar:       "GHORG_GITLAB_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "GitLab personal access token",
	},
	{
		DotNotation:  "gitlab.insecure",
		EnvVar:       "GHORG_INSECURE_GITLAB_CLIENT",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Skip TLS verification for GitLab",
	},
	{
		DotNotation:  "gitlab.group-exclude-match-regex",
		EnvVar:       "GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX",
		DefaultValue: "",
		Description:  "Exclude GitLab groups matching regex",
	},
	{
		DotNotation:  "gitlab.group-match-regex",
		EnvVar:       "GHORG_GITLAB_GROUP_MATCH_REGEX",
		DefaultValue: "",
		Description:  "Include only GitLab groups matching regex",
	},

	// ── Bitbucket ────────────────────────────────────────────────────────
	{
		DotNotation:  "bitbucket.app-password",
		EnvVar:       "GHORG_BITBUCKET_APP_PASSWORD",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "Bitbucket app password",
	},
	{
		DotNotation:  "bitbucket.username",
		EnvVar:       "GHORG_BITBUCKET_USERNAME",
		DefaultValue: "",
		Description:  "Bitbucket username for app password auth",
	},
	{
		DotNotation:  "bitbucket.oauth-token",
		EnvVar:       "GHORG_BITBUCKET_OAUTH_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "Bitbucket OAuth token",
	},
	{
		DotNotation:  "bitbucket.api-token",
		EnvVar:       "GHORG_BITBUCKET_API_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "Bitbucket Cloud API token",
	},
	{
		DotNotation:  "bitbucket.api-email",
		EnvVar:       "GHORG_BITBUCKET_API_EMAIL",
		DefaultValue: "",
		Description:  "Email associated with Bitbucket API token",
	},
	{
		DotNotation:  "bitbucket.insecure",
		EnvVar:       "GHORG_INSECURE_BITBUCKET_CLIENT",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Allow HTTP for Bitbucket Server",
	},

	// ── Gitea ────────────────────────────────────────────────────────────
	{
		DotNotation:  "gitea.token",
		EnvVar:       "GHORG_GITEA_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "Gitea personal access token",
	},
	{
		DotNotation:  "gitea.insecure",
		EnvVar:       "GHORG_INSECURE_GITEA_CLIENT",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Allow HTTP for Gitea",
	},

	// ── Sourcehut ────────────────────────────────────────────────────────
	{
		DotNotation:  "sourcehut.token",
		EnvVar:       "GHORG_SOURCEHUT_TOKEN",
		DefaultValue: "",
		IsSecret:     true,
		Description:  "Sourcehut personal access token",
	},
	{
		DotNotation:  "sourcehut.insecure",
		EnvVar:       "GHORG_INSECURE_SOURCEHUT_CLIENT",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Allow HTTP for Sourcehut",
	},

	// ── Reclone ──────────────────────────────────────────────────────────
	{
		DotNotation:  "reclone.path",
		EnvVar:       "GHORG_RECLONE_PATH",
		DefaultValue: "", // computed at runtime via GhorgReCloneLocation()
		Description:  "Path to reclone.yaml configuration file",
	},
	{
		DotNotation:  "reclone.quiet",
		EnvVar:       "GHORG_RECLONE_QUIET",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Suppress reclone output",
	},
	{
		DotNotation:  "reclone.env-config-only",
		EnvVar:       "GHORG_RECLONE_ENV_CONFIG_ONLY",
		DefaultValue: "false",
		IsBool:       true,
		Description:  "Use only environment variables for reclone config",
	},
	{
		DotNotation:  "reclone.server-port",
		EnvVar:       "GHORG_RECLONE_SERVER_PORT",
		DefaultValue: ":8080",
		Description:  "Port for the reclone HTTP server",
	},
	{
		DotNotation:  "reclone.cron-timer-minutes",
		EnvVar:       "GHORG_CRON_TIMER_MINUTES",
		DefaultValue: "60",
		Description:  "Interval in minutes for cron-based recloning",
	},
}

// indexes built on first access
var (
	byDot    map[string]*ConfigKey
	byEnvVar map[string]*ConfigKey
)

func buildIndexes() {
	if byDot != nil {
		return
	}
	byDot = make(map[string]*ConfigKey, len(AllKeys))
	byEnvVar = make(map[string]*ConfigKey, len(AllKeys))
	for i := range AllKeys {
		byDot[AllKeys[i].DotNotation] = &AllKeys[i]
		byEnvVar[AllKeys[i].EnvVar] = &AllKeys[i]
	}
}

// LookupByDot returns the ConfigKey for a dot-notation key, or nil if not found.
func LookupByDot(dot string) *ConfigKey {
	buildIndexes()
	return byDot[dot]
}

// LookupByEnvVar returns the ConfigKey for an environment variable name, or nil if not found.
func LookupByEnvVar(env string) *ConfigKey {
	buildIndexes()
	return byEnvVar[env]
}

// DotToEnvVar converts a dot-notation key to its environment variable name.
// Returns empty string if the key is not in the registry.
func DotToEnvVar(dot string) string {
	k := LookupByDot(dot)
	if k == nil {
		return ""
	}
	return k.EnvVar
}

// EnvVarToDot converts an environment variable name to its dot-notation key.
// Returns empty string if the env var is not in the registry.
func EnvVarToDot(env string) string {
	k := LookupByEnvVar(env)
	if k == nil {
		return ""
	}
	return k.DotNotation
}

// Sections returns a deduplicated, ordered list of all section names.
func Sections() []string {
	seen := make(map[string]bool)
	var sections []string
	for i := range AllKeys {
		s := AllKeys[i].Section()
		if !seen[s] {
			seen[s] = true
			sections = append(sections, s)
		}
	}
	return sections
}
