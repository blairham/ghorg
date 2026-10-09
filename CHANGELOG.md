# Changelog

## [Unreleased]

### Security
- golang.org/x/net 0.60.0 (five HTTP/2 advisories) and klauspost/compress
  1.18.7 (GO-2026-5841); `osv-scanner.toml` records the two advisories with
  no fix, which reach only GoReleaser's tool graph

### Documentation
- New README screenshots of `ghorg clone` and `ghorg reclone`, recorded from
  VHS tapes in `docs/tapes/`; a note on the clash with homebrew-core's `ghorg`

## [0.1.6] - 2026-10-09

### Security
- `reclone-server`: the unauthenticated `/trigger/reclone?cmd=` value is now
  only ever a reclone.yaml entry name. A value starting with `-` was parsed as
  a `ghorg reclone` flag, so `--reclone-path=<file>` could run the
  `ghorg clone` entries of any YAML file the server could read, with its
  tokens (GHSA-77wq-45v4-4r3j, #78)
- `reclone` masks every `-t`, `--token` and `--bitbucket-api-token` value in
  the command it logs. Before, a token could be printed in full when another
  flag's value contained `-t=`, a quoted token was masked only up to its
  first space, and `--bitbucket-api-token` was not masked (#76)
- Sign `checksums.txt` and the `ghcr.io/blairham/ghorg` image with keyless
  cosign, and attest SLSA build provenance for the archives and the image;
  `SECURITY.md` shows how to verify them
- Pin the image's base by digest and move it to Alpine 3.23
- Add `SECURITY.md` with the trust model and private vulnerability reporting
- The stats CSV is created `0600`
- Build with Go 1.26.9

### Build
- go-git 5.19.3; consolidate the open Dependabot security bumps (sigstore,
  grpc, in-toto, go-pkcs12, slack, MCP registry — GoReleaser's tool graph)

### CI/CD
- Release and CI through blairham/.github's shared workflows, with the shared
  configuration baseline, CodeQL `security-extended` and OpenSSF Scorecard
- Pin every action by commit SHA
- Fuzz targets for the reclone command parser and token masking
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
