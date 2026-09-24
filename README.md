# lintpal

[![Test](https://github.com/diffpal/lintpal/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/diffpal/lintpal/actions/workflows/test.yml)
[![Lint](https://github.com/diffpal/lintpal/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/diffpal/lintpal/actions/workflows/lint.yml)
[![Security](https://github.com/diffpal/lintpal/actions/workflows/security.yml/badge.svg?branch=main)](https://github.com/diffpal/lintpal/actions/workflows/security.yml)
[![Latest release](https://img.shields.io/github/v/release/diffpal/lintpal)](https://github.com/diffpal/lintpal/releases/latest)
[![npm version](https://img.shields.io/npm/v/lintpal)](https://www.npmjs.com/package/lintpal)
[![License: MIT](https://img.shields.io/github/license/diffpal/lintpal)](LICENSE)

**Review committed code changes with questions you control.** lintpal compares
two Git revisions, asks a selected System One provider focused questions, and
reports findings on changed lines. Rules are Markdown requirements, so a team can
keep review policy beside its code, pin imported rule packs, and revisit the
same committed inputs.

lintpal is a CLI for review, not an auto-fixer. It reads committed Git objects;
unsaved and uncommitted working-tree changes are outside the comparison.

## Start in a few commands

Install the current public release through npm:

```bash
npm install -g lintpal
lintpal version
```

To use features from this source revision before they reach npm, build it with
Go 1.26.6 or newer:

```bash
go build -o ./lintpal ./cmd/lintpal
./lintpal version
```

Choose a provider credential. For the default `jev` provider, set
`TYPESAFE_API_KEY` in your environment or copy [`.env.example`](.env.example)
to `.env` and fill in the key. The local `doctor` check does not contact the
provider:

```bash
./lintpal doctor --provider jev
mkdir -p .artifacts/lintpal
./lintpal lint --base HEAD~1 --head HEAD --provider jev \
  --out .artifacts/lintpal/report.json
```

Both revisions must be available commits in the repository where you run
lintpal. Use an absolute path to the built binary when reviewing another
repository. Markdown feedback is printed to stdout and JSON findings are written to the artifact path;
exit code `10` means a finding met the gate **after** the complete report was
written. See [getting started](docs/getting-started.md) for shallow clones,
other providers, and error handling. See [distribution](docs/distribution.md)
for the published package layout and release status.

## Make the review yours

A rule is a Markdown file whose relative path is its ID. Its body is a
requirement for changed code. Optional frontmatter sets severity, threshold,
and title. For example, `go/unchecked-error.md` can contain:

```markdown
---
severity: high
title: Possible unchecked error
---

Handle errors returned by calls when failure changes the result or behavior.
```

The checked-in [Go review directory](examples/rules/go-review/unchecked-error.md)
and [documentation review rule](examples/rules/docs-review/command-drift.md)
show authoring examples. These are not claims of measured model accuracy.
See [rule authoring](docs/rule-authoring.md) for defaults, overrides, and
source path filters.

Import a pack explicitly from a local directory, verify its lockfile, and
select it for a review:

```bash
./lintpal pack import go-review ./examples/rules/go-review
./lintpal pack verify go-review
./lintpal lint --base HEAD~1 --head HEAD --rules @go-review
```

GitHub imports require an explicit ref. lintpal resolves it to a commit,
stores a local copy, and records the exact content hash in
`.lintpal/packs.lock.json`. Normal lint runs use the local copy offline; see
[rule packs](docs/rule-packs.md) for source syntax and updates.

## Use it in development

[Task](https://taskfile.dev/docs/installation) runs routine checks from
`Taskfile.yml`:

```bash
task --list
task check
task self-review-smoke
```

`task check` builds, tests, vets, checks formatting, evaluates the frozen
offline corpus, and verifies docs and installed packs without a provider key.
The [test workflow](.github/workflows/test.yml) keeps these checks credential-free
on Linux, macOS, and Windows as appropriate. To review lintpal itself with a
real provider, use explicit committed refs:

```bash
BASE=origin/main HEAD=HEAD task self-review
```

This opt-in command writes `.artifacts/lintpal/self-review.json`. Read the
[self-review guide](docs/self-review.md) for provider setup, rule selection,
and gate behavior.

## Documentation

Start at the [documentation index](docs/index.md), or jump to:

| Use lintpal | Extend and maintain it |
| --- | --- |
| [Getting started](docs/getting-started.md) · [Configuration](docs/configuration.md) · [CLI](docs/cli.md) | [Rule authoring](docs/rule-authoring.md) · [Rule packs](docs/rule-packs.md) |
| [Reports](docs/report.md) · [Privacy and limits](docs/privacy.md) | [Development tasks](docs/development.md) · [Self-review](docs/self-review.md) · [Evaluation](docs/evaluation.md) |

When a remote provider is selected, lintpal sends bounded committed source
context and rule questions to it. Review the [privacy guide](docs/privacy.md)
before running a live review. The current CLI does not inspect uncommitted
changes, apply fixes, or report measured model precision. lintpal is available
under the [MIT license](LICENSE).
