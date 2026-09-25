# Getting started

Run LintPal from the Git repository you want to check. The npm package
provides a native CLI; use `npx lintpal` after installing it in the project:

```bash
npm install --save-dev lintpal
npx lintpal version
```

The commands below describe this source revision. Check
[releases](https://github.com/diffpal/lintpal/releases) for the features in a
published npm version. Source builds are covered in [CONTRIBUTING](../CONTRIBUTING.md).

## Add a mandate

Write one requirement per Markdown file. Its path relative to `.lintpal/rules/`
is its rule ID:

```bash
mkdir -p .lintpal/rules/go
cat > .lintpal/rules/go/errors.md <<'RULE'
Handle errors returned by calls when failure can change the result or behavior.
RULE
npx lintpal rule validate
```

LintPal finds that directory at the Git worktree root even when you run from a
subdirectory. It does not supply built-in rules. See [rule authoring](rule-authoring.md)
for optional frontmatter and [rule import](rule-import.md) for copying a team
catalog into the same directory.

## Lint committed revisions

The default `jev` provider reads `TYPESAFE_API_KEY` from the environment or
an uncommitted worktree-root `.env`. `doctor` checks local setup without a
provider request:

```bash
export TYPESAFE_API_KEY='your-provider-key'
npx lintpal doctor
npx lintpal lint --base origin/main --head HEAD
```

Both revisions must be available commits. If a shallow checkout lacks the base,
fetch that history or choose two commit IDs that exist locally. Working-tree
edits are not lint inputs. Markdown findings go to stdout. Exit code `10`
means a finding met the severity gate after the complete report was written;
see [CLI options and exit codes](cli.md). A remote provider receives bounded
committed source context and rule text; see [privacy](privacy.md). Inspect the
[demo pull request](https://github.com/diffpal/lintpal-demo/pull/3) to see live
GitHub Actions review feedback and inline findings on committed changes.
