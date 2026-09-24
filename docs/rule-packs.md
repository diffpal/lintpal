# Rule packs

A rule pack is one declarative `lintpal.rules.v1` YAML file. It contains rule
questions and fixed diagnostic text; it cannot run commands or choose a provider,
endpoint, or credential. A lint run uses the built-in pack by default, one
unmanaged file with `--rules PATH`, or one installed pack with `--rules @NAME`.
See [rule authoring](rule-authoring.md) for schema fields and complete examples.

## Try a local pack

From this repository's root, after building `lintpal`:

```bash
./lintpal pack import go-review ./examples/rules/go-review
./lintpal pack verify go-review
./lintpal lint --base origin/main --head HEAD --rules @go-review
```

The [Go review example](../examples/rules/go-review/rules.yaml) shows `noul`,
`choice`, and `score` rules. `paths: ['*.go']` selects Go files at any depth;
`sides: [RIGHT]` considers added or changed lines on the head side. Thresholds
are decision thresholds, not measured model accuracy. Calibrate rules on your
own examples before making them a blocking CI gate.

Each source directory must contain `rules.yaml`. Names use lowercase letters,
digits, `.`, `_`, and `-`, start with a letter or digit, and are at most 63
characters. Duplicate names require an explicit update. `lintpal pack update
go-review` refreshes the recorded source; `lintpal pack update go-review
./another-directory` switches to another source.

## Import from GitHub

Use the explicit `github:OWNER/REPO//DIRECTORY@REF` form, where `DIRECTORY`
contains `rules.yaml`. Omit `//DIRECTORY` when the file is at the repository
root. `REF` can be a tag, branch, or commit and is required:

```bash
lintpal pack import team-go github:acme/lint-rules//go@v1.2.0
lintpal pack verify team-go
lintpal lint --base origin/main --head HEAD --rules @team-go
```

The GitHub import is a network operation for public repositories. lintpal
resolves the requested ref to a commit, fetches only `rules.yaml` at that
commit, validates it, and stores an exact local copy. Normal `lint`, `doctor`,
and `pack verify` runs do not fetch from GitHub. To move to a new ref, run
`lintpal pack update team-go github:acme/lint-rules//go@v1.3.0` and review the
copy and lockfile changes before committing them.

## Lockfile and verification

The project stores installed copies at `.lintpal/packs/NAME/SHA256.yaml` and
records the active copy in `.lintpal/packs.lock.json`. Each lock entry contains
the source, requested ref and resolved commit for GitHub sources, path, and
SHA-256 of the exact YAML bytes. Commit both the active copy and lockfile for
reproducible offline runs. `pack verify` checks every entry; `pack verify NAME`
checks one. `--rules @NAME` verifies the hash and parses the copy before any
provider request. If a managed copy changes, re-import or update explicitly.
Direct paths into `.lintpal/packs/` are rejected; use `@NAME`.

Imports reject invalid YAML, unsafe paths and symlinks, oversized files, and
malformed lockfiles. An update writes a new content-addressed copy before
switching the lockfile; failed updates retain the previous active entry. Old
inactive copies can remain after an update and may be removed manually after
checking the lockfile. Imported rule instructions and committed source context
are sent to the selected provider during lint; see [privacy](privacy.md).
