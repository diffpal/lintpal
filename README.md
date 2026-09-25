# LintPal

[![ci](https://github.com/diffpal/lintpal/actions/workflows/ci.yml/badge.svg)](https://github.com/diffpal/lintpal/actions/workflows/ci.yml)
[![lintpal-dev review](https://github.com/diffpal/lintpal/actions/workflows/lintpal-dev-review.yml/badge.svg)](https://github.com/diffpal/lintpal/actions/workflows/lintpal-dev-review.yml)
[![npm](https://img.shields.io/npm/v/lintpal?label=npm)](https://www.npmjs.com/package/lintpal)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Turn plain-English engineering rules into pull-request checks.**

LintPal checks committed changes against Markdown rules owned by your
repository. It produces findings on changed lines, applies a deterministic
severity gate, and can publish the result directly to GitHub. Use it for
specific requirements your team wants enforced on every change.

[Quickstart](docs/getting-started.md) ·
[Documentation](docs/index.md) ·
[Rule packs](https://github.com/diffpal/lintpal-rules) ·
[Demo](https://github.com/diffpal/lintpal-demo/pull/3)

## Features

- **Freeform rules:** write one clear requirement per Markdown file, with
  optional severity and decision-threshold policy in frontmatter.
- **Rule packs:** import a local directory or a versioned GitHub catalog, then
  review and commit the rules with your code.
- **Gating:** choose which finding severities block CI while retaining the
  complete findings artifact after each successful evaluation.
- **Platform feedback:** publish a deterministic GitHub review summary and
  inline comments, or consume the same findings as JSON or Markdown in CI.

## How It Works

| Stage | What LintPal does |
| --- | --- |
| Rules | Loads the repository's `.lintpal/rules/**/*.md` requirements |
| Diff | Reads changed lines between two committed Git revisions |
| Decisions | Evaluates each applicable rule through the configured provider |
| Findings | Writes line-anchored findings in the shared findings v5 format |
| Feedback | Publishes inline GitHub comments and applies the configured gate |

LintPal is a focused policy checker. It does not generate a narrative code
review or invent new review criteria during a run.

## Minimal GitHub Quickstart

Install LintPal and add a versioned rule pack:

```bash
npm install --save-dev lintpal
npx lintpal rule import github:diffpal/lintpal-rules//general@v1.1.0
npx lintpal rule validate
```

Add `TYPESAFE_API_KEY` as a repository Actions secret, then create
`.github/workflows/lintpal.yml`:

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
      - uses: diffpal/lintpal-action@v1
        with:
          lintpal-version: "0.4.1"
          base: ${{ github.event.pull_request.base.sha }}
          head: ${{ github.event.pull_request.head.sha }}
          block-on: high
          gate: true
        env:
          TYPESAFE_API_KEY: ${{ secrets.TYPESAFE_API_KEY }}
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
one inline comment for each finding GitHub can attach to the diff. The workflow
also keeps `.artifacts/lintpal/findings.json` for later steps.

See the [LintPal Action](https://github.com/diffpal/lintpal-action) for every
input, artifact upload, provider selection, and fork pull-request guidance.

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
npx lintpal rule list
npx lintpal rule view general/authorization.md
npx lintpal rule validate
npx lintpal rule import github:diffpal/lintpal-rules//go@v1.1.0
```

Read [rule authoring](docs/rule-authoring.md) for the complete format and
[rule import](docs/rule-import.md) for local and pinned GitHub sources.

## Run Locally

Both revisions must be committed and available in the local Git repository:

```bash
export TYPESAFE_API_KEY='your-provider-key'
npx lintpal doctor
npx lintpal lint --base origin/main --head HEAD
```

Markdown findings go to stdout. Use `--out` to retain the complete JSON
artifact, or `--format json` for JSON on stdout. The default gate returns exit
code `10` when a high or critical finding blocks the run.

The default provider is Jev. LintPal also supports OpenRouter and a trusted
custom endpoint. Provider credentials stay in environment variables; the
selected provider receives bounded committed source context and rule text.
See [configuration](docs/configuration.md) and [privacy](docs/privacy.md).

## Documentation by Goal

| Goal | Start here |
| --- | --- |
| Run the first check | [Getting started](docs/getting-started.md) |
| Configure providers and policy | [Configuration](docs/configuration.md) |
| Write project rules | [Rule authoring](docs/rule-authoring.md) |
| Import reusable rule packs | [Rule import](docs/rule-import.md) |
| Understand findings and gates | [Report reference](docs/report.md) |
| Automate the CLI | [CLI reference](docs/cli.md) |
| Review security and data flow | [Privacy](docs/privacy.md) and [architecture](docs/architecture.md) |
| Contribute to LintPal | [Contributing](CONTRIBUTING.md) |

## License

LintPal is released under the [MIT License](LICENSE).
