# Rule directories and packs

`--rules PATH` loads a local directory of Markdown files. `--rules @NAME`
loads an installed copy. With no `--rules`, LintPal uses its built-in Markdown
rules. See [rule authoring](rule-authoring.md) and the
[Go examples](../examples/rules/go-review/unchecked-error.md).

## Local import

```bash
lintpal pack import go-review ./examples/rules/go-review
lintpal pack verify go-review
lintpal lint --base origin/main --head HEAD --rules @go-review
```

Import validates the directory recursively, copies its `.md` files, and
records a content hash in `.lintpal/packs.lock.json`. The installed directory
is `.lintpal/packs/NAME/SHA256/`; relative Markdown paths are preserved.
Commit the installed copy and lockfile together. `pack verify` checks every
installed pack offline; `pack verify NAME` checks one. Lint verifies a selected
pack before contacting a provider. A changed installed copy fails verification.
Use `pack update NAME` to refresh the source, or `pack update NAME PATH` to
switch sources, then review the copy and lockfile. Direct paths into
`.lintpal/packs/` are rejected; use `@NAME`.

## GitHub import

Use `github:OWNER/REPO//DIRECTORY@REF`. `DIRECTORY` is optional for the
repository root; `REF` is required and can be a tag, branch, or commit.

```bash
lintpal pack import team-go github:acme/lint-rules//go@v1.2.0
lintpal pack verify team-go
```

LintPal resolves the ref to a commit and fetches Markdown files below the
selected directory. The lockfile records both requested ref and resolved
commit. Normal `lint`, `doctor`, and `pack verify` do not fetch from GitHub.
Move to another ref with `pack update`, then review the installed copy and
lockfile before committing them.

Imports reject malformed Markdown or frontmatter, unsafe paths, symlinks,
oversized content, and malformed locks. A failed update leaves the active
entry in place. Old inactive copies may remain after an update. The v1 YAML
pack lockfile is not loaded; re-import a Markdown directory to create a v2
lockfile. See [privacy](privacy.md) for provider transfer.
