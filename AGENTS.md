# AGENTS.md — ghorg

Working agreements and project reference for AI coding agents. This is the cross-tool source of truth; `CLAUDE.md` imports it. The tree-wide agreements in `~/Developer/github.com/blairham/AGENTS.md` apply here too.

## Project Overview

ghorg is a CLI tool that bulk-clones all repositories from a GitHub, GitLab, Gitea, Bitbucket, or Sourcehut organization/user into a single local directory. Use cases include code searching, team onboarding, backups, and audits.

- **Language:** Go (pinned in `go.mod`, currently 1.26.x — `go.mod` is authoritative)
- **CLI framework:** `hashicorp/cli`
- **Flag parsing:** `jessevdk/go-flags`
- **Config parsing:** `knadh/koanf` (YAML)
- **Git backends:** `go-git/go-git` (pure Go, default) or system `git` binary (`exec` mode)
- **Entry point:** `main.go` (root)

ghorg started as a fork of `gabrie30/ghorg` but is a standalone repo now. Upstream issues/PRs are referenced as `gabrie30/ghorg#N`.

## Quick Reference — Make Targets

Run `make help` for the full, current list.

| Command | Purpose |
|---|---|
| `make build-local` | Fast local dev build → `dist/ghorg` |
| `make build` | Multi-platform build via GoReleaser snapshot |
| `make build-docker` | Build Docker images locally (no push) |
| `make install` | Build + copy sample config to `~/.config/ghorg/` |
| `make test` | Run all tests (`go test ./... -v`) |
| `make test-race` | Run all tests with race detector |
| `make test-git` / `test-sync` / `test-helpers` | Focused test subsets |
| `make test-all` | fmt + lint + test (full quality gate) |
| `make test-coverage` | Tests with HTML coverage report → `coverage.html` |
| `make fmt` | Format all Go files (`go tool gofumpt -w`) |
| `make lint` | Run golangci-lint |
| `make clean` | Remove build artifacts |
| `make release` | Full GoReleaser release (requires `GITHUB_TOKEN`) |
| `make release-dry` / `release-check` | Dry-run release / validate GoReleaser config |

gofumpt, golangci-lint, and GoReleaser are all pinned in `go.mod`'s `tool` block — run them via `go tool`, never a separately installed binary.

## Project Structure

```
main.go                         # Entry point — creates CLI, registers commands, runs

internal/
  cmd/
    cli.go                      # Command registration (clone, reclone, ls, config, init, version, ...)
    clone.go                    # Core clone command — flag parsing, config setup, orchestration
    config.go                   # Config loading (koanf YAML → env vars → CLI flags)
    config_command.go           # git-config-style `ghorg config` command (get/set/unset/list/migrate)
    repository_processor.go     # Per-repo clone/pull logic, stats tracking, collision handling
    repository_filter.go        # Filtering pipeline (regex, prefix, topics, ghorgignore/only)
    state.go                    # Resumable state manifest (_ghorg_state.json) + --retry-failed
    reclone.go                  # Batch reclone from reclone.yaml
    reclone-server.go           # HTTP server for triggering reclones
    reclone-cron.go             # Cron-scheduled recloning
    init.go                     # Interactive setup wizard
    ls.go                       # List cloned repos
    version.go                  # Version command (falls back to Go build info for go install)

  scm/
    client.go                   # SCM Client interface + global registry
    structs.go                  # Repo, RepoCommits, GitLabSnippet structs
    filter.go                   # Topic matching logic
    github.go                   # GitHub provider (go-github)
    gitlab.go                   # GitLab provider (gitlab-org/api/client-go)
    gitea.go                    # Gitea provider (gitea SDK)
    bitbucket.go                # Bitbucket provider (go-bitbucket)
    sourcehut.go                # Sourcehut provider (GraphQL)
    *_parallel.go               # Parallel fetching per provider

  git/
    git.go                      # Gitter interface + exec-based GitClient
    go-git.go                   # Pure Go git implementation (goGitClient)
    sync.go                     # Default branch sync with safety checks

  configs/
    configs.go                  # Config loading, token handling, validation, keychain lookup

  colorlog/
    colorlog.go                 # Singleton logger (charmbracelet/log), color via GHORG_COLOR

  utils/
    utils.go                    # IsStringInSlice, CalculateDirSizeInMb
```

## Architecture

### Data Flow (clone command)

```
main.go → cli.go (command factory)
  → clone.go: CloneCommand.Run()
    1. parseAndApplyFlags() — parse CLI flags, apply via data-driven mapping
       → applyStringFlags() / applyBoolFlags() — flag→env var mappings
       → setTokenForSCM() — routes --token to correct SCM-specific env var
    2. validateConfig() — token verification, SCM type check
    3. Setup output directory
    → setupRepoClone()
      4. Fetch repos from SCM provider (scm.Client interface)
      → CloneAllRepos()
        5. Filter repos (RepositoryFilter pipeline)
        6. Concurrent processing via korovkin/limiter (default 25)
        → RepositoryProcessor.ProcessRepository() per repo
          7a. New repo: git.Clone() → checkout branch → strip credentials
          7b. Existing repo: set origin → pull/reset → strip credentials
          7c. Optional: SyncDefaultBranch()
        8. Report stats, prune untouched, write state manifest + CSV
```

### Key Interfaces

**SCM Client** (`internal/scm/client.go`):
```go
type Client interface {
    NewClient() (Client, error)
    GetUserRepos(targetUsername string) ([]Repo, error)
    GetOrgRepos(targetOrg string) ([]Repo, error)
    GetType() string
}
```
Providers self-register via `registerClient()`. Retrieved via `GetClient(scmType)`.

**Git Operations** (`internal/git/git.go`): the `Gitter` interface (Clone, Pull, Reset, Checkout, SetOrigin, fetch/branch/sync/status methods). Backend selected by `GHORG_GIT_BACKEND`: `"exec"` → system git, default → pure Go (go-git). `--git-filter` and `--sparse-checkout` are exec-only; go-git ignores them with a warning.

### Configuration

Precedence: CLI flags → environment variables → config file (YAML) → defaults.

The config file uses **nested lowercase keys** (e.g. `scm.type`, `core.concurrency`, `github.token`) — see `sample-conf.yaml`. The legacy format using `GHORG_*` env var names as YAML keys is converted with `ghorg config --migrate`. Internally, all config is normalized onto `GHORG_*` environment variables at startup (`internal/cmd/config.go`), so `os.Getenv("GHORG_...")` is the runtime representation throughout the codebase.

Config file locations checked in order:
1. `GHORG_CONFIG` env var / `--config` flag
2. `./ghorg.yaml` (current working directory, legacy local config)
3. `$XDG_CONFIG_HOME/ghorg/conf.yaml`, falling back to `$HOME/.config/ghorg/conf.yaml`

A `./.ghorg/config.yaml` in the current directory, if present, is overlaid on top (`ghorg config --local` writes here).

### Credential Handling Pattern

1. Clone with credentials embedded in URL (`repo.CloneURL`)
2. Immediately strip credentials from origin (`repo.URL`)
3. Temporarily restore credentials only when needed (e.g., fetch-all)
4. Always strip again after operation completes
5. On macOS, automatic Keychain lookup as fallback for GitHub/GitLab/Bitbucket tokens
6. GitHub token fallback chain: explicit token/env var → `gh auth token` (gh CLI) → macOS Keychain

### Concurrency

- `korovkin/limiter` controls goroutine count (default 25, via `GHORG_CONCURRENCY`)
- All stats tracked via `RepositoryProcessor` with `sync.RWMutex` protection
- `GHORG_CLONE_DELAY_SECONDS > 0` silently forces concurrency to 1
- `GHORG_DEBUG=true` also forces concurrency to 1 (override with `GHORG_CONCURRENCY_DEBUG`)

### Repository Filtering Pipeline

Applied in order:
1. `GHORG_MATCH_REGEX` — include repos matching regex (substring match)
2. `GHORG_EXCLUDE_MATCH_REGEX` — exclude repos matching regex
3. `GHORG_MATCH_PREFIX` — case-insensitive prefix include (comma-separated)
4. `GHORG_EXCLUDE_MATCH_PREFIX` — case-insensitive prefix exclude
5. `GHORG_TARGET_REPOS_PATH` — include only repos listed in file
6. `ghorgonly` file — inclusion filter
7. `ghorgignore` file — exclusion filter
8. `--retry-failed` — only repos whose last recorded status in `_ghorg_state.json` was `error`

### Error Model

- **Fatal:** Missing token, invalid config, unsafe path → immediate exit
- **Recoverable:** Individual repo clone/pull failures → collected in `CloneStats.CloneErrors`, reported at end
- **Info:** Wiki failures, missing target repos, branch issues → collected in `CloneStats.CloneInfos`
- Exit codes configurable via `GHORG_EXIT_CODE_ON_CLONE_ISSUES` and `GHORG_EXIT_CODE_ON_CLONE_INFOS`

## Testing Conventions

### Patterns

- **Table-driven tests** with `t.Run()` subtests are the primary pattern
- **Interface mocks** defined per test file (e.g., `MockGitClient`, `DelayedMockGit`, `SyncTrackingMockGit`)
- **Real git integration tests** in `internal/git/` — create actual repos with `createTestGitRepo()` / `setupTestRepo()`
- **HTTP mock servers** for SCM providers via `httptest.NewServer()` + `http.ServeMux`
- **Environment cleanup** via `UnsetEnv("GHORG_")` helper with deferred restore
- **Temp directories** with `os.MkdirTemp()` + `defer os.RemoveAll()`
- **Benchmarks** exist for filter and pagination performance

### Conventions for New Tests

- Use table-driven tests with descriptive `name` fields
- Use `t.Parallel()` for independent tests and subtests
- Use `t.Fatalf()` for setup errors, `t.Errorf()` for assertion failures
- Clean up environment variables with `defer UnsetEnv("GHORG_")()`
- For git tests: create real temp repos rather than mocking filesystem
- For SCM tests: use `httptest.NewServer()` with JSON responses
- Tests must never touch real user state (home directory, keychains) — redirect via temp dirs and env vars
- Test files are excluded from linting (`.golangci.yml: tests: false`)

### CI Test Matrix

Tests run on **macOS, Ubuntu, and Windows** via GitHub Actions (`.github/workflows/go-test.yml`). CI configures git before tests:
```
git config --global user.name "Test User"
git config --global user.email "test@example.com"
git config --global init.defaultBranch main
```
Pre-commit hooks run in CI via the go-pre-commit action; CodeQL analysis also runs.

## Authentication by Provider

| Provider | Token Env Var | Alt Auth | Auto-detect Fallback |
|---|---|---|---|
| GitHub | `GHORG_GITHUB_TOKEN` | GitHub App (PEM + Installation ID + App ID) | `gh auth token` → macOS Keychain |
| GitLab | `GHORG_GITLAB_TOKEN` | — | Yes (macOS) |
| Gitea | `GHORG_GITEA_TOKEN` | — | No |
| Bitbucket | `GHORG_BITBUCKET_OAUTH_TOKEN`, OR `GHORG_BITBUCKET_APP_PASSWORD` + `GHORG_BITBUCKET_USERNAME`, OR `GHORG_BITBUCKET_API_TOKEN` + `GHORG_BITBUCKET_API_EMAIL` | — | Yes (macOS) |
| Sourcehut | `GHORG_SOURCEHUT_TOKEN` | — | No |

All tokens accept either a literal value or a path to a file containing the token. File contents are cleaned (BOM stripped, whitespace trimmed). Use `GHORG_NO_TOKEN=true` to skip authentication entirely.

## Release & Distribution

- **GoReleaser v2**, pinned in `go.mod`'s `tool` block, config in `.goreleaser.yml`
- **Platforms:** Linux (amd64/arm64/arm), macOS (amd64/arm64), Windows (amd64)
- **Homebrew:** a **formula** (not a cask) published to `blairham/homebrew-tap` under `Formula/` via the `brews:` GoReleaser section
- **macOS binaries** are signed and notarized when signing env vars are present
- **Docker:** Published to `ghcr.io/blairham/ghorg` (amd64/arm64)
- **Triggered by:** Git tag push (`v*`) or manual `workflow_dispatch`
- **Static binaries:** `CGO_ENABLED=0` for full portability
- Update `CHANGELOG.md` when tagging a release

## Linting

golangci-lint v2, pinned in `go.mod`'s `tool` block, config in `.golangci.yml` (~49 linters enabled). Runs in CI, not pre-commit. Notable settings:
- `tests: false` — test files excluded
- No naked returns allowed (`nakedret.max-func-lines: 0`)
- `nolint` directives must specify which linter
- No global slog loggers (`sloglint.no-global: all`)
- Exhaustive switch and map checks enabled
- Import grouping enforced: stdlib, then third-party, then `github.com/blairham/ghorg` (via `goimports` with `local-prefixes`)

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/google/go-github` | GitHub API |
| `gitlab.com/gitlab-org/api/client-go` | GitLab API |
| `code.gitea.io/sdk/gitea` | Gitea API |
| `github.com/ktrysmt/go-bitbucket` | Bitbucket API |
| `github.com/go-git/go-git/v5` | Pure Go git |
| `github.com/bradleyfalzon/ghinstallation/v2` | GitHub App auth |
| `github.com/knadh/koanf/v2` | Config management |
| `github.com/jessevdk/go-flags` | CLI flag parsing |
| `github.com/hashicorp/cli` | CLI framework |
| `github.com/korovkin/limiter` | Concurrency control |
| `github.com/charmbracelet/lipgloss` + `log` | Terminal styling/logging |

Check `go.mod` for current major versions rather than trusting docs.
