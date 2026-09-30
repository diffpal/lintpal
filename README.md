# LintPal

<img src="docs/assets/lintpal-thumbnail.png" alt="DiffPal family mascot" width="120">

[![ci](https://github.com/diffpal/lintpal/actions/workflows/ci.yml/badge.svg)](https://github.com/diffpal/lintpal/actions/workflows/ci.yml)
[![lintpal-dev review](https://github.com/diffpal/lintpal/actions/workflows/lintpal-dev-review.yml/badge.svg)](https://github.com/diffpal/lintpal/actions/workflows/lintpal-dev-review.yml)
[![npm](https://img.shields.io/npm/v/lintpal?label=npm)](https://www.npmjs.com/package/lintpal)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Turn plain-English engineering rules into pull-request checks.**

LintPal checks committed changes against Markdown rules owned by your
repository. It produces findings on changed lines, applies a deterministic
severity gate, and can publish the result directly to GitHub. Use it for
specific requirements your team wants enforced on every change.

LintPal is part of the [DiffPal family](https://github.com/diffpal/diffpal).
It checks explicit repository rules; [DiffPal](https://diffpal.github.io/)
provides broader AI pull-request review across GitHub, GitLab, and Azure DevOps.

[Quickstart](docs/guides/getting-started.md) ·
[Documentation](docs/index.md) ·
[Rule packs](https://github.com/diffpal/lintpal-rules) ·
[Demo PR (v0.5.0)](https://github.com/diffpal/lintpal-demo/pull/3)

For example, a [Go error-handling rule](examples/rules/go-review/unchecked-error.md)
can flag a violating changed line in a pull request; the configured severity
gate can then fail the check.

## Features

- **Provider choice:** use TypeSafe/Jev, OpenRouter, OpenAI, or your own
  compatible Decisions service with the same repository rules and CI workflow.
- **Freeform rules:** write one clear requirement per Markdown file, with
  optional severity and decision-threshold policy in frontmatter.
- **Rule packs:** import a local directory or a versioned GitHub catalog, then
  review and commit the rules with your code.
- **Gating:** choose which finding severities block CI while retaining the
  complete findings artifact after each successful evaluation.
- **Platform feedback:** publish a deterministic GitHub review summary and
  inline comments, or consume the same findings as JSON or Markdown in CI.

## Supported Providers

Choose the provider that fits your account and deployment. Your Markdown rules,
findings, and severity gate work the same way across providers.

| Provider | Choose it for | CLI option | Credential |
| --- | --- | --- | --- |
| [**TypeSafe / Jev**](docs/guides/configuration.md#typesafe--jev) | Direct access to Jev; the default setup | `--provider jev` | `TYPESAFE_API_KEY` |
| [**OpenRouter**](docs/guides/configuration.md#openrouter) | Decisions through your OpenRouter account | `--provider openrouter` | `OPENROUTER_API_KEY` |
| [**OpenAI**](docs/guides/configuration.md#openai) | A preset for compatible OpenAI Decisions | `--provider openai --model <model-id>` | `OPENAI_API_KEY` |
| [**Custom**](docs/guides/configuration.md#custom-compatible-service) | Your own compatible service, including a local endpoint | `--provider custom --base-url <url> --api-path <path>` | `LINTPAL_TOKEN` or your chosen token variable |

The OpenAI preset assumes a compatible `/v1/decisions` API; live availability
has not been verified. These options describe this source revision; check
[releases](https://github.com/diffpal/lintpal/releases) for published package support.
See [provider setup and commands](docs/guides/configuration.md#supported-providers)
for all four options. The quickstart below uses TypeSafe/Jev.

## How It Works

| Stage | What LintPal does |
| --- | --- |
| Rules | Loads the repository's `.lintpal/rules/**/*.md` requirements |
| Diff | Reads changed lines from the unique merge base through head, or from `HEAD` to the working tree with `--uncommitted` |
| Decisions | Evaluates each applicable rule through the configured provider |
| Findings | Writes line-anchored findings in LintPal's findings v5 format |
| Feedback | Publishes inline GitHub comments and applies the configured gate |

LintPal is a focused policy checker. It does not generate a narrative code
review or invent new review criteria during a run.

## Minimal GitHub Quickstart

Install the CLI globally and add a versioned rule pack in your repository:

```bash
npm install -g lintpal
lintpal rule import github:diffpal/lintpal-rules//general@v1.1.0
lintpal rule validate
```

Review and commit `.lintpal/rules/` so the workflow can load the rules.
The selected remote provider receives bounded source context and rule text.
Check whether that transfer is permitted for your repository before adding
`TYPESAFE_API_KEY` as an Actions secret; see [privacy](docs/architecture/privacy.md).
Then create `.github/workflows/lintpal.yml`:

```yaml
name: lintpal

on:
  pull_request:
    types: [opened, synchronize, reopened, ready_for_review]

jobs:
  lint:
    if: ${{ !github.event.pull_request.draft && github.event.pull_request.head.repo.full_name == github.repository }}
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - uses: actions/setup-node@v6
        with:
          node-version: 24
      - run: npm install -g lintpal
      - name: Lint pull request
        run: |
          mkdir -p .artifacts/lintpal
          lintpal lint --no-env-file --provider jev \
            --base ${{ github.event.pull_request.base.sha }} \
            --head ${{ github.event.pull_request.head.sha }} \
            --block-on high --format json --out .artifacts/lintpal/findings.json
        env:
          TYPESAFE_API_KEY: ${{ secrets.TYPESAFE_API_KEY }}
      - name: Publish review and apply gate
        run: lintpal feedback github --in .artifacts/lintpal/findings.json --gate
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: lintpal-findings
          path: .artifacts/lintpal/
          if-no-files-found: warn
```

Open a same-repository pull request. LintPal publishes a review with
`No blocking findings`, `1 blocking finding`, or `N blocking findings`, plus
one inline comment for each finding GitHub can attach to the diff. The gate
runs after publication; the workflow keeps `.artifacts/lintpal/findings.json`
even when a blocking finding fails the job. Draft and fork pull requests are
skipped. GitHub supplies the pull-request context to the feedback command.

See the [CLI reference](docs/reference/cli.md) for provider selection,
feedback commands, and exit codes. The [LintPal Action](https://github.com/diffpal/lintpal-action)
is also available for workflows that prefer an Action wrapper.

## Write Rules in Markdown

The file path beneath `.lintpal/rules/` is the rule ID. The body is the
requirement. Optional frontmatter controls its policy:

```markdown
---
severity: high
threshold: 0.97
title: Unchecked error
---

Handle errors returned by calls when failure changes the result or behavior.
```

Rules are ordinary project files: review them, version them, and change them
through the same pull-request process as code. LintPal ships without hidden or
built-in mandates.

```bash
lintpal rule list
lintpal rule view general/authorization.md
lintpal rule validate
lintpal rule import github:diffpal/lintpal-rules//go@v1.1.0
```

Read [rule authoring](docs/rules/authoring.md) for the complete format and
[rule import](docs/rules/import.md) for local and pinned GitHub sources.

## Run Locally

Check uncommitted changes in your working tree before committing, or compare two committed revisions:

```bash
export TYPESAFE_API_KEY='your-provider-key'
lintpal doctor

# Lint uncommitted working-tree changes (staged, unstaged, and untracked regular files)
lintpal lint --uncommitted

# Or lint committed revisions
lintpal lint --base origin/main --head HEAD
```

Committed comparisons start at the unique merge base of base and head; changes
found only on the base branch are excluded. See the [CLI reference](docs/reference/cli.md)
for report revision fields and uncommitted behavior.

Markdown findings go to stdout. Use `--out` to retain the complete JSON
artifact, or `--format json` for JSON on stdout. The default gate returns exit
code `10` when a high or critical finding blocks the run.

Choose TypeSafe/Jev, OpenRouter, OpenAI, or a custom compatible endpoint
from [Supported Providers](#supported-providers). Provider credentials stay in environment variables; the
selected provider receives bounded source context and rule text.
See [configuration](docs/guides/configuration.md) and [privacy](docs/architecture/privacy.md).

## Documentation by Goal

| Goal | Start here |
| --- | --- |
| Run the first check | [Getting started](docs/guides/getting-started.md) |
| Configure providers and policy | [Configuration](docs/guides/configuration.md) |
| Write project rules | [Rule authoring](docs/rules/authoring.md) |
| Import reusable rule packs | [Rule import](docs/rules/import.md) |
| Understand findings and gates | [Report reference](docs/reference/report.md) |
| Automate the CLI | [CLI reference](docs/reference/cli.md) |
| Review security and data flow | [Privacy](docs/architecture/privacy.md) and [architecture](docs/architecture/overview.md) |
| Report a bug or share rule feedback | [GitHub Issues](https://github.com/diffpal/lintpal/issues/new) |
| Contribute to LintPal | [Contributing](CONTRIBUTING.md) |

## License

LintPal is released under the [MIT License](LICENSE).
