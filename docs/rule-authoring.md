# Write Markdown rules

A rule is one UTF-8 Markdown file containing a requirement for changed code.
The path relative to the selected rule directory is its ID. For example,
`go/unchecked-error.md` is the ID of a file at that path. Directories organize
IDs; they do not select source files. Every rule applies to eligible changed
lines on both `LEFT` and `RIGHT` sides.

The Markdown body is the mandate. Write one clear, testable requirement per
file. The selected provider answers whether changed code violates it; LintPal
reports a finding when the true probability reaches the rule threshold. This
threshold is a decision setting, not measured model accuracy.

Optional YAML frontmatter sets policy:

```markdown
---
severity: high
threshold: 0.97
title: Unchecked error
---

Handle errors returned by calls when failure changes the result or behavior.
```

The only frontmatter fields are `severity` (`low`, `medium`, `high`, or
`critical`), `threshold` (a number from 0 to 1), and a nonempty `title`.
Unknown or duplicate fields are rejected. Defaults are `medium`, `0.95`, and
`Possible rule violation`. The fixed finding message is
`Changed code may violate RULE_ID.` Its evidence references the rule ID.

An explicit `--rule-severity` or `LINTPAL_RULE_SEVERITY` overrides frontmatter
severity for every rule in the run. `--rule-threshold` and
`LINTPAL_RULE_THRESHOLD` likewise override threshold. A flag wins over the
environment setting. Without either override, each rule uses its frontmatter
or the default. Repeatable `--include GLOB` and `--exclude GLOB` filter changed
source paths for the whole run; they do not change rule IDs.

Try the [Go review directory](../examples/rules/go-review/unchecked-error.md):

```bash
lintpal lint --base HEAD~1 --head HEAD --rules ./examples/rules/go-review \
  --include '*.go' --rule-threshold 0.95
```

You can also [import and pin a directory](rule-packs.md). Rule text and bounded
committed source context are sent to the selected provider; see
[privacy](privacy.md).

## Migrating YAML rule packs

Replace each old YAML rule with one `.md` file. Use its relative path as the
ID, put the requirement in the body, and move only `severity`, `threshold`,
and `title` into optional frontmatter. Old `choice`, `score`, `criteria`,
`message`, `paths`, and `sides` fields have no Markdown equivalent. Rewrite
choice or score questions as yes/no requirements, use `--include`/`--exclude`
for source selection, and review both changed sides. The CLI no longer loads
`rules.yaml` or a v1 pack lockfile; re-import the Markdown directory to create
a v2 lockfile. See [report migration](report.md) for output changes.
