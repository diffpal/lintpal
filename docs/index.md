# lintpal documentation

lintpal reviews changes between two committed Git revisions. It asks a selected
System One provider questions defined by repository Markdown rules, then
reports findings tied to changed lines. Start with the [README](../README.md)
for the short overview.

## User Guides

Practical guides for setting up and running lintpal in local and CI environments.

- [Getting started](guides/getting-started.md): install through npm and lint a first
  committed comparison.
- [Configuration](guides/configuration.md): providers, `.env`, rule selection, output,
  and common setup failures.
- [Demo pull request](https://github.com/diffpal/lintpal-demo/pull/3): review
  live GitHub Actions feedback and inline rule findings.

## Rules System

Specifications, guidelines, and tools for writing and importing repository review rules.

- [Rule authoring](rules/authoring.md): Markdown mandates and frontmatter.
- [Rule import](rules/import.md): copy a local or pinned GitHub directory into
  `.lintpal/rules/` and validate the catalog offline.
- [Go review example](../examples/rules/go-review/unchecked-error.md): a
  Markdown mandate for changed Go code.
- [Documentation review example](../examples/rules/docs-review/command-drift.md):
  a Markdown mandate for documentation commands.

## Technical Reference

Formal specifications for command-line interfaces and reporting schemas.

- [CLI reference](reference/cli.md): flags, environment settings, and exit codes.
- [Report reference](reference/report.md): shared JSON findings and Markdown feedback.

## Architecture & Security

Design principles, trust boundaries, and operational constraints.

- [Architecture overview](architecture/overview.md): runtime boundaries and data flow.
- [Privacy and limitations](architecture/privacy.md): source transfer and current scope.
- [Resource limits](architecture/resource-limits.md): input and output bounds.

## Contributor & Maintainer Guide

Workflows and tooling for developing and verifying lintpal.

- [Contributing](../CONTRIBUTING.md): source setup and contribution checks.
- [Development tasks](development/tasks.md): Taskfile build, tests, evaluation,
  format, and verification targets.
- [Self-review](development/self-review.md): review committed lintpal changes and collect
  a JSON artifact.
- [Evaluation](development/evaluation.md): frozen offline corpus and optional live checks.
- [Distribution](development/distribution.md): packaging and publication checks.

## Releases

Version-specific runbooks and release notes.

- [v0.3.0 release runbook](releases/release-v0.3.0.md): packaging and publication runbook for v0.3.0.
- [v0.2.0 release runbook](releases/release-v0.2.0.md): release verification and history for v0.2.0.
