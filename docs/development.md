# Development tasks

Install Go 1.26.6 or newer as required by `go.mod`, then install
[Task](https://taskfile.dev/docs/installation) v3.44.0. The
[test workflow](../.github/workflows/test.yml) installs that version explicitly.
Run `task --list` from the repository root to see the available targets.

| Target | Purpose | Direct command |
| --- | --- | --- |
| `task build` | Build every Go package | `go build ./...` |
| `task test` | Run the test suite | `go test ./...` |
| `task vet` | Run Go static analysis | `go vet ./...` |
| `task format` | Check Go formatting without editing files | `go run ./scripts/verify-format` |
| `task race` | Run tests with the race detector | `go test -race ./...` |
| `task eval` | Check the frozen offline evaluation baseline | `go test ./internal/apps/lintpal/eval -run TestFrozenCorpusBaseline -count=1` |
| `task pack-verify` | Check installed rule packs against their lockfile | `go run ./scripts/verify-packs` |
| `task docs-verify` | Check local Markdown links and rule examples | `go run ./scripts/verify-docs` |
| `task self-review` | Review committed changes in this repository | `go run ./scripts/self-review` |
| `task self-review-smoke` | Exercise self-review with a fake provider | `go test ./scripts/self-review -run TestSelfReviewSmoke -count=1` |
| `task lint` | Run vet and format checks | Run both direct commands above |
| `task lint-go` | Run the module-pinned golangci-lint gate | `go tool golangci-lint run ./...` |
| `task security` | Scan reachable code for known vulnerabilities | `go tool govulncheck ./...` |
| `task check` | Run build, test, lint, eval, pack and docs checks | Run the direct commands above |

`task pack-verify` reports `verified 0 pack(s)` when the project has no pack
lockfile. If the lockfile exists, a malformed lock or changed installed copy
fails the target. `task docs-verify` checks relative file links in `README.md`
and `docs/*.md` and loads every Markdown example rule directory through the same
rule parser used by lintpal. Both targets use local files only.

The test matrix runs build, test, and vet through Task on Linux, macOS, and
Windows. Linux also runs format, eval, pack and docs verification, and race.
These checks do not need a provider key or send source to a model. The race
tests start local HTTP servers, so the local environment must allow loopback
sockets. The live evaluation and release scripts remain separate commands;
see the [evaluation guide](evaluation.md) and
[distribution guide](distribution.md).

The separate [static lint workflow](../.github/workflows/lint.yml) runs
`go tool golangci-lint` with the version pinned by the `tool` directive in
`go.mod`.
`task lint-go` runs it with `go tool`; no separate binary installation is
needed. The ordinary `task check` does not compile the linter.

The [security workflow](../.github/workflows/security.yml) runs the
module-pinned `govulncheck` on pushes, pull requests, a weekly schedule, and
manual dispatch. It queries the Go vulnerability database and does not use a
provider credential.

For the live command, required revisions, report path, and provider setup,
see [self-review](self-review.md).
