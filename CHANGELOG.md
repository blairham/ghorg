# Changelog

## [Unreleased]

### CI/CD
- Bump Go to 1.26.6, resolved in CI via check-latest
- Consolidate open dependabot bumps
- Warm the cross-compile build cache between releases
- Bump go-pre-commit action to v4.6.6

## [v0.1.5] - 2026-08-13

### Build
- Publish a Homebrew formula (instead of a cask) to `blairham/homebrew-tap`

### CI/CD
- Run formatters through golangci-lint, add toolchain pin check
- Bump go-pre-commit action to v4.6.3

## [v0.1.4] - 2026-08-06

### Features
- Sign and notarize macOS binaries on release

### Build
- Harden macOS Gatekeeper handling in the Homebrew cask

### CI/CD
- Run pre-commit hooks via go-pre-commit action
- Upgrade node20-deprecated actions; use codeql-action v4
- Dependency updates

## [v0.1.3] - 2026-05-14

### Features
- Add resumable state manifest (`_ghorg_state.json`), `--retry-failed`, and `--sparse-checkout`
- Publish releases to `blairham/homebrew-tap`

### Bug Fixes
- Read version from Go build info so `go install` builds report the right version

### Other
- Move GoReleaser to the `go.mod` tool block
- Add `.editorconfig`, align pre-commit config, document resumability and the git-backend feature matrix

## [v0.1.2] - 2026-04-20

### Other
- Move entry point from `cmd/ghorg/` to the project root; update build configs and docs

## [v0.1.1] - 2026-04-19

### Features
- Add interactive `ghorg init` setup wizard
- Add `gh` CLI token fallback for GitHub authentication

### Other
- Replace legacy changelog with fresh project history

## [v0.1.0] - 2026-04-19

### Features
- Add git-config-style `ghorg config` command with get/set/list/edit/migrate subcommands
- Use Go `tool` directive in `go.mod` for `golangci-lint` and `gofumpt`

### Bug Fixes
- Resolve data race in config registry lazy init

### CI/CD
- Update pre-commit hooks: add `go-fumpt`, `go-mod-tidy`, `detect-secrets`, `gitleaks`
- Remove homebrew tap from release pipeline

### Other
- Apply `gofumpt` formatting across codebase

## [v0.0.0] - 2026-04-13

Initial fork from gabrie30/ghorg with restructured codebase.
