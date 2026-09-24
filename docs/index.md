# lintpal documentation

lintpal reviews changes between two committed Git revisions. It asks a selected
System One provider questions defined by built-in or declarative rules, then
reports findings tied to changed lines. Start with the [README](../README.md)
for the short overview.

## Start using lintpal

- [Getting started](getting-started.md): build from source and review a first
  committed comparison.
- [Configuration](configuration.md): providers, `.env`, rule selection, output,
  and common setup failures.
- [CLI reference](cli.md): flags, environment settings, and exit codes.
- [Report reference](report.md): JSON and human output contract.
- [Privacy and limitations](privacy.md): source transfer and current scope.

## Write and use rules

- [Rule authoring](rule-authoring.md): schema, `noul`, `choice`, and `score`
  decisions with complete examples.
- [Rule packs](rule-packs.md): import from a directory or pinned GitHub ref,
  understand the lockfile, and verify offline.
- [Go review example](../examples/rules/go-review/rules.yaml): a complete pack
  with `noul`, `choice`, and `score` rules.
- [Documentation review example](../examples/rules/docs-review/rules.yaml): a
  focused `noul` rule for changed Markdown commands.

## Maintain the project

- [Development tasks](development.md): Taskfile build, tests, evaluation,
  format, and verification targets.
- [Self-review](self-review.md): review committed lintpal changes and collect
  a JSON artifact.
- [Evaluation](evaluation.md): frozen offline corpus and optional live checks.
- [Architecture](architecture.md): runtime boundaries and data flow.
- [Resource limits](resource-limits.md): input and output bounds.
- [Distribution](distribution.md) and [v0.3.0 release runbook](release-v0.3.0.md):
  packaging and publication checks.
