# Working in lintpal

lintpal is a Go CLI that reviews two committed Git revisions. Keep the
committed-only input model, declarative rule boundary, report schema, and
selected-provider credential handling intact when changing behavior.

## Project commands

- Use the Go version in `go.mod` (currently 1.26.6).
- Run `task --list` to discover tooling. `task check` runs the routine offline
  build, test, vet, format, evaluation, rule, and docs checks.
- Run `task race` for race-sensitive code and `task self-review-smoke` for the
  fake-provider process path. These tests use local HTTP listeners.
- Run `task docs-verify` after changing Markdown links or rule examples.
- Run `task lint-go` for the separate static lint gate; `go.mod` pins the
  `go tool golangci-lint` version.
- Run `task security` for the vulnerability scan; it contacts the Go
  vulnerability database and uses the module-pinned `go tool govulncheck`.
- Run `go mod tidy` when dependencies change, inspect the module diff, and
  verify with `go mod verify`.

## Repository conventions

- Keep `.lintpal/rules/` as declarative Markdown data. Validate examples
  through the existing rules parser; review imported rule files in Git.
- Keep documentation links relative and route new user topics through
  `docs/index.md`. Explain observable CLI behavior rather than promising model
  accuracy or unpublished distribution channels.
- Keep `.env` and generated reports out of Git. `.env.example` documents keys
  without containing credentials. Do not print or commit provider tokens.
- CI checks use fake providers and local fixtures. `task self-review` and live
  evaluation contact a selected provider only when explicitly invoked.
- Track Prism lifecycle work in the repository Beads workspace. Read the
  current issue state before updating it; keep requirements, design, tasks,
  approvals, and verification in Beads.

See [development tasks](docs/development.md), [architecture](docs/architecture.md),
and [privacy](docs/privacy.md) for details.
