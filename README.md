# LintPal

[![Test](https://github.com/diffpal/lintpal/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/diffpal/lintpal/actions/workflows/test.yml)
[![npm version](https://img.shields.io/npm/v/lintpal)](https://www.npmjs.com/package/lintpal)
[![License: MIT](https://img.shields.io/github/license/diffpal/lintpal)](LICENSE)

**Lint committed changes against the rules your team writes.** Put requirements
in Markdown files beside your code. LintPal checks changed lines in two Git
revisions, reports findings with file and line anchors, and can fail CI when a
finding reaches your severity gate.

## Start

Install LintPal in your project, or use `npx lintpal` directly:

```bash
npm install --save-dev lintpal
mkdir -p .lintpal/rules/go
cat > .lintpal/rules/go/errors.md <<'RULE'
Handle errors returned by calls when failure can change the result or behavior.
RULE
export TYPESAFE_API_KEY='your-provider-key'
npx lintpal lint --base origin/main --head HEAD
```

Both revisions must be committed and available locally. LintPal reads rules
from the Git worktree root `.lintpal/rules/`, even when run from a subdirectory.
There are no built-in mandates. A missing or invalid rule directory stops the
run before any provider request. The selected provider receives bounded
committed source context and rule text; see [privacy](docs/privacy.md).

The command prints Markdown findings. The default gate fails with exit code
`10` for a finding of **high** or **critical** severity, after writing the
complete output. Use `--out .artifacts/lintpal/findings.json` for a JSON
artifact, or `--format json` for JSON on stdout. See the [CLI reference](docs/cli.md)
for provider options, filters, and two-command CI feedback.

Stored findings can also be published to a GitHub pull request without another
model call. `lintpal feedback github` posts a deterministic blocking-status
result and inline rule findings; it does not generate a semantic code review or
change summary. See the [CLI reference](docs/cli.md#lintpal-cli) for flags and
the required pull-request permission.

## Work with rules

Each `.md` file is one mandate; its path beneath `.lintpal/rules/` is its ID.
Optional frontmatter sets `title`, `severity`, and `threshold`. Directory names
organize IDs; every rule applies to eligible changed lines on both sides of the
diff.

```bash
npx lintpal rule list
npx lintpal rule view go/errors.md
npx lintpal rule validate
npx lintpal rule import ./team-rules --prefix team
```

`rule import` copies validated Markdown into `.lintpal/rules/`, so the next lint
uses it automatically. Commit and review those files with your project. GitHub
imports can select a ref and subdirectory; [rule import](docs/rule-import.md)
explains the source syntax and collision handling. See [rule authoring](docs/rule-authoring.md)
for examples and per-run overrides.

## Learn more

- [Getting started](docs/getting-started.md) and [configuration](docs/configuration.md)
- [Findings and gates](docs/report.md)
- [Contributing](CONTRIBUTING.md) and [development tasks](docs/development.md)

The commands above describe this source revision. Check
[releases](https://github.com/diffpal/lintpal/releases) for the features in a
published npm version. LintPal is available under the [MIT license](LICENSE).
