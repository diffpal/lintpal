# Import rules

`lintpal rule import SOURCE` copies Markdown rules into the current Git
worktree's `.lintpal/rules/` directory. Relative paths in the source become
rule IDs. Imported files are ordinary project files: review and commit their
diff. The next `lint`, `rule list`, or `rule view` uses them automatically.

## Local directory

```bash
npx lintpal rule import ./examples/rules/go-review --prefix team
npx lintpal rule validate
npx lintpal rule list
```

A local `SOURCE` path is relative to the current directory. `--prefix team`
places `go/errors.md` at `.lintpal/rules/team/go/errors.md`. Omit it to preserve
source IDs directly.

## GitHub directory

Use `github:OWNER/REPO//DIRECTORY@REF`. `DIRECTORY` is optional for the
repository root; `REF` is required and can be a branch, tag, or commit.

```bash
npx lintpal rule import github:acme/lint-rules//go@v1.2.0 --prefix team
```

LintPal resolves the ref to a commit and fetches files from that commit. The
command prints the resolved SHA. Normal lint and catalog commands use only
local files and make no rule-source network request. Reimport with a new ref
when you want an update, then review the resulting Git diff.

## Collisions and failures

An existing rule ID causes import to fail without changing the catalog. Pass
`--force` to replace only IDs from this import; other files stay intact. Import
validates the complete resulting catalog before installing it and rejects
malformed Markdown, unsafe paths, symlinks, and content over the limits. Failed
validation leaves the previous rule set in place. `rule validate` checks the
catalog locally without a provider key.

For mandate syntax and policy frontmatter, see [rule authoring](authoring.md).
