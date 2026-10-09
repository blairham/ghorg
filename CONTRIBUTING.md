# Contributing

Thank you for considering a contribution. Please read the two rules below
before opening a pull request -- both are non-negotiable.

## 1. Tokens never leak

ghorg holds provider tokens for every repository it clones. A change that
writes a token to a log line, the state manifest, the stats CSV or a clone's
`.git/config` will not be merged. Clones are made with the credential in the
URL and stripped from `origin` straight away; keep that true. See
[`SECURITY.md`](SECURITY.md) for the trust model.

## 2. The Contributor License Agreement

Contributions require a signed CLA; the text is in [`CLA.md`](CLA.md).

**Why.** The project may need to offer different licensing terms in future.
That is only possible if one party can license the whole work, and copyright
in a contribution stays with its author unless licensed onward.

The CLA does **not** take your copyright. You keep it; you grant a license
broad enough to include sublicensing, and you affirm the work is your own --
including that no employer holds rights to it.

## Practical

- Open an issue before a large change, so the design can be agreed first.
- Work on a branch and open a pull request against `main`.
- Commit messages explain the **why**, not a restatement of the diff.
  Conventional-commit prefixes (`feat:`, `fix:`, `docs:`, ...).
- Commits must be signed.
- Every `.go` file carries the SPDX header; the pre-commit hook fails without
  it. Files carried over from gabrie30/ghorg keep their upstream
  `SPDX-FileCopyrightText` line.
- Every user-visible change gets a line under `[Unreleased]` in
  [`CHANGELOG.md`](CHANGELOG.md); a release's notes are that section.
- `pre-commit install` once per checkout. The hooks format, lint and scan for
  secrets on every commit; never bypass them with `--no-verify`.
- `go test -race ./...` must pass, and needs no provider account. **New functionality
  comes with tests in the same pull request**, and a pull request that adds
  behavior without them will not be merged. A bug fix comes with the test
  that would have caught it. Provider calls are tested against
  `httptest.NewServer`, git operations against real temporary repositories.

Please also read the [Code of Conduct](CODE_OF_CONDUCT.md). Security issues go
through [SECURITY.md](SECURITY.md), never a public issue.
