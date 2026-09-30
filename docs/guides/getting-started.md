# Getting started

Run LintPal from the Git repository you want to check. The npm package
provides a native CLI for repositories in any language; install it globally:

```bash
npm install -g lintpal
lintpal version
```

The commands below are supported by
[v0.6.0](https://github.com/diffpal/lintpal/releases/tag/v0.6.0).
Source builds are covered in [CONTRIBUTING](../../CONTRIBUTING.md).

## Add a mandate

Write one requirement per Markdown file. Its path relative to `.lintpal/rules/`
is its rule ID:

```bash
mkdir -p .lintpal/rules/go
cat > .lintpal/rules/go/errors.md <<'RULE'
Handle errors returned by calls when failure can change the result or behavior.
RULE
lintpal rule validate
```

LintPal finds that directory at the Git worktree root even when you run from a
subdirectory. It does not supply built-in rules. See [rule authoring](../rules/authoring.md)
for optional frontmatter and [rule import](../rules/import.md) for copying a team
catalog into the same directory.

## Choose a provider

LintPal supports TypeSafe/Jev (`jev`, the default), OpenRouter (`openrouter`),
OpenAI (`openai`, assuming a compatible Decisions API), and your own compatible
endpoint (`custom`). Follow [provider setup](configuration.md#supported-providers)
for credentials and commands for each. OpenAI live availability is unverified
and its preset requires an explicit model. The example below uses TypeSafe/Jev.

## Lint uncommitted or committed changes

The default `jev` provider reads `TYPESAFE_API_KEY` from the environment or
an uncommitted worktree-root `.env`. `doctor` checks local setup without a
provider request:

```bash
export TYPESAFE_API_KEY='your-provider-key'
lintpal doctor

# Review uncommitted working-tree changes before committing
lintpal lint --uncommitted

# Or review committed revisions
lintpal lint --base origin/main --head HEAD
```

Run `lint --uncommitted` to review local working-tree changes (including unstaged
edits, staged changes, and untracked regular files) against `HEAD` before
committing. Or run `lint --base <base> --head <head>` to compare two committed
revisions: LintPal reviews their unique merge base through head, so changes
found only on the base branch are excluded. If a shallow checkout lacks the base, fetch that history or choose
two commit IDs that exist locally. Markdown findings go to stdout. Exit code `10`
means a finding met the severity gate after the complete report was written;
see [CLI options and exit codes](../reference/cli.md). A remote provider receives bounded
source context and rule text; see [privacy](../architecture/privacy.md). Inspect the
[demo pull request](https://github.com/diffpal/lintpal-demo/pull/3) to see live
GitHub Actions review feedback and inline findings on committed changes.
