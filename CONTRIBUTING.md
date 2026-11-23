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
- **The pull request title is a [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/)**,
  and it is the changelog entry and the version bump: the repository
  squash-merges with the title as the commit subject, and release-please
  reads the subjects. The required **PR title** check fails one that is not.

  | Title | Bump | [`CHANGELOG.md`](CHANGELOG.md) |
  |---|---|---|
  | `feat!:` / `fix!:` (any type with `!`) | minor before 1.0, then major | ⚠ Breaking Changes |
  | `feat:` | minor | Added |
  | `fix:` (a compatible security fix too) | patch | Fixed |
  | `perf:` / `revert:` | patch | Performance / Reverted |
  | `docs:` `ci:` `build:` `refactor:` `test:` `chore:` `style:` | none | hidden |

  The text after the colon is the entry, so write it for someone upgrading.
  A `BREAKING CHANGE: …` footer in the PR body also marks it breaking.
  Before 1.0 a breaking change bumps the minor version, as the changelog's
  header promises; it never makes 1.0.0 by itself.
- **Releases are release-please's.** Every merge updates one standing pull
  request, `chore(main): release X.Y.Z`, with the next version and its
  `CHANGELOG.md` section. Merging it is the release: it tags `vX.Y.Z`, and
  `release-please.yml` calls `release.yml` to publish it. Never tag by
  hand. `Release-As: X.Y.Z` in a PR body forces a version. A title that went
  wrong after the merge is fixed on the merged pull request: add the messages
  it should have had between `BEGIN_COMMIT_OVERRIDE` and `END_COMMIT_OVERRIDE`
  in its body, **separated by blank lines** (one message per line reads as a
  single message with the rest as its body). release-please starts at the
  first `BEGIN_COMMIT_OVERRIDE` anywhere in the body, so never write that
  word elsewhere in a PR body, even in backticks, and the block replaces the
  subject even of a conventional commit. The PR title check on the release
  pull request reads each commit as release-please will and fails on any
  message that is not a Conventional Commit.
- `pre-commit install` once per checkout. The hooks format, lint and scan for
  secrets on every commit; never bypass them with `--no-verify`.
- `go test -race ./...` must pass, and needs no provider account. **New functionality
  comes with tests in the same pull request**, and a pull request that adds
  behavior without them will not be merged. A bug fix comes with the test
  that would have caught it. Provider calls are tested against
  `httptest.NewServer`, git operations against real temporary repositories.

Please also read the [Code of Conduct](CODE_OF_CONDUCT.md). Security issues go
through [SECURITY.md](SECURITY.md), never a public issue.
