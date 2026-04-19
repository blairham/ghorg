# CLAUDE.md — ghorg

## Project Overview

ghorg is a CLI tool that bulk-clones all repositories from a GitHub, GitLab, Gitea, Bitbucket, or Sourcehut organization/user into a single local directory. Use cases include code searching, team onboarding, backups, and audits.

- **Language:** Go 1.24.11
- **CLI framework:** `hashicorp/cli`
- **Flag parsing:** `jessevdk/go-flags`
- **Config parsing:** `knadh/koanf` (YAML)
- **Git backends:** `go-git/go-git` (pure Go, default) or system `git` binary (`exec` mode)
- **Entry point:** `cmd/ghorg/main.go`

## Quick Reference — Build Commands

Run `make help` for a full list.

| Command | Purpose |
|---|---|
| `make build-local` | Fast local dev build → `dist/ghorg` |
| `make build` | Multi-platform build via GoReleaser snapshot |
| `make build-docker` | Build Docker images locally (no push) |
| `make install` | Build + copy sample config to `~/.config/ghorg/` |
| `make test` | Run all tests (`go test ./... -v`) |
| `make test-git` | Run git package tests only |
| `make test-sync` | Run sync-related tests only |
| `make test-helpers` | Run git helper function tests only |
| `make test-all` | fmt + lint + test (full quality gate) |
| `make test-coverage` | Tests with HTML coverage report → `coverage.html` |
| `make fmt` | Format all Go files (`gofmt -s -w`) |
| `make lint` | Run golangci-lint (47 linters enabled) |
| `make clean` | Remove build artifacts |
| `make deps-install` | Install goreleaser, golangci-lint |
| `make deps-verify` | Check required tools are available |
| `make release` | Full GoReleaser release (requires `GITHUB_TOKEN`) |
| `make release-dry` | Dry-run release (no publish) |

## Project Structure

```
cmd/ghorg/
  main.go                       # Entry point — creates CLI, registers commands, runs

internal/
  cmd/
    cli.go                      # Command registration (clone, reclone, ls, version, etc.)
    clone.go                    # Core clone command — flag parsing, config setup, orchestration
    config.go                   # Config loading (koanf YAML → env vars → CLI flags)
    repository_processor.go     # Per-repo clone/pull logic, stats tracking, collision handling
    repository_filter.go        # Filtering pipeline (regex, prefix, topics, ghorgignore/only)
    reclone.go                  # Batch reclone from reclone.yaml
    reclone-server.go           # HTTP server for triggering reclones
    reclone-cron.go             # Cron-scheduled recloning
    ls.go                       # List cloned repos
    version.go                  # Version command

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
    git.go                      # Gitter interface (26 methods) + exec-based GitClient
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
       → applyStringFlags() — loops over string flag→env var mappings
       → applyBoolFlags() — loops over bool flag→env var mappings
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
        8. Report stats, prune untouched, write CSV
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

**Git Operations** (`internal/git/git.go`):
```go
type Gitter interface {
    Clone(repo scm.Repo) error
    Pull(repo scm.Repo) error
    Reset(repo scm.Repo) error
    Checkout(repo scm.Repo) error
    SetOrigin(repo scm.Repo) error
    SetOriginWithCredentials(repo scm.Repo) error
    // ... 20 more methods for fetch, branch, sync, status, etc.
}
```
Backend selected by `GHORG_GIT_BACKEND`: `"exec"` → system git, default → pure Go (go-git).

### Configuration Precedence

```
CLI flags (highest) → Environment variables → Config file (YAML) → Defaults (lowest)
```

Config file locations checked in order:
1. `GHORG_CONFIG` env var / `--config` flag
2. `./ghorg.yaml` (current working directory)
3. `$HOME/.config/ghorg/conf.yaml` (or `$XDG_CONFIG_HOME/ghorg/conf.yaml`)

Config keys use env var names as YAML keys (e.g., `GHORG_SCM_TYPE: github`).

### Credential Handling Pattern

1. Clone with credentials embedded in URL (`repo.CloneURL`)
2. Immediately strip credentials from origin (`repo.URL`)
3. Temporarily restore credentials only when needed (e.g., fetch-all)
4. Always strip again after operation completes
5. On macOS, automatic Keychain lookup as fallback for GitHub/GitLab/Bitbucket tokens

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
- Test files are excluded from linting (`.golangci.yml: tests: false`)

### CI Test Matrix

Tests run on **macOS, Ubuntu, and Windows** via GitHub Actions. CI configures git before tests:
```
git config --global user.name "Test User"
git config --global user.email "test@example.com"
git config --global init.defaultBranch main
```

## Authentication by Provider

| Provider | Token Env Var | Alt Auth | Keychain Fallback |
|---|---|---|---|
| GitHub | `GHORG_GITHUB_TOKEN` | GitHub App (PEM + Installation ID + App ID) | Yes (macOS) |
| GitLab | `GHORG_GITLAB_TOKEN` | — | Yes (macOS) |
| Gitea | `GHORG_GITEA_TOKEN` | — | No |
| Bitbucket | `GHORG_BITBUCKET_OAUTH_TOKEN` OR `GHORG_BITBUCKET_APP_PASSWORD` + `GHORG_BITBUCKET_USERNAME` | — | Yes (macOS) |
| Sourcehut | `GHORG_SOURCEHUT_TOKEN` | — | No |

All tokens accept either a literal value or a path to a file containing the token. File contents are cleaned (BOM stripped, whitespace trimmed). Use `GHORG_NO_TOKEN=true` to skip authentication entirely.

## Release & Distribution

- **GoReleaser v2** config in `.goreleaser.yml`
- **Platforms:** Linux (amd64/arm64/arm), macOS (amd64/arm64), Windows (amd64)
- **Docker:** Published to `ghcr.io/blairham/ghorg` (amd64/arm64/arm/v7)
- **Triggered by:** Git tag push (`v*`) or manual `workflow_dispatch`
- **Static binaries:** `CGO_ENABLED=0` for full portability
- **Dependencies:** Fetched via Go module proxy (no vendor directory)

## Linting

golangci-lint v2.7.2 with 47 linters enabled. Notable settings:
- `tests: false` — test files excluded
- No naked returns allowed (`nakedret.max-func-lines: 0`)
- `nolint` directives must specify which linter
- No global slog loggers (`sloglint.no-global: all`)
- Exhaustive switch and map checks enabled
- Import grouping enforced: stdlib, then third-party, then `github.com/blairham/ghorg` (via `goimports` with `local-prefixes`)

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/google/go-github/v72` | GitHub API |
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
