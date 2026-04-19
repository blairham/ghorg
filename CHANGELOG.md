# Changelog

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
