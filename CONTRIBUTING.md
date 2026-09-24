# Contributing to LintPal

Use the Go version pinned by `go.mod` and install [Task](https://taskfile.dev/docs/installation).
From a source checkout:

```bash
go build -o ./lintpal ./cmd/lintpal
task --list
task check
```

`task check` runs the offline build, tests, vet, format, evaluation, rule, and
documentation checks. Run `task race` for concurrent code and
`task self-review-smoke` for the fake-provider process path. Neither needs a
provider credential. `task lint-go` is a separate static lint gate; `task
security` contacts the Go vulnerability database.

Keep rule examples as Markdown data. Review imported files in
`.lintpal/rules/` as policy changes. Update documentation when CLI behavior
changes and run `task docs-verify` after changing Markdown links or examples.
The [development guide](docs/development.md), [architecture](docs/architecture.md),
and [distribution guide](docs/distribution.md) cover the rest of the repository.

Live self-review and evaluation contact a selected provider only when
explicitly invoked. Keep `.env` and generated reports out of Git; see
[privacy](docs/privacy.md).
