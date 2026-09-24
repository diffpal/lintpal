# Write a rule pack

A rule pack is one declarative UTF-8 `rules.yaml` file. It asks a provider
questions about selected changed lines and supplies fixed diagnostic text.
It cannot execute code, choose an endpoint, or read credentials. A lint run
uses one pack: the built-in rules, one unmanaged file, or one installed pack.

Start with the [Go review pack](../examples/rules/go-review/rules.yaml) to see
all three rule kinds together. The
[documentation review pack](../examples/rules/docs-review/rules.yaml) is a
smaller example that selects changed Markdown lines:

```yaml
schema: lintpal.rules.v1
rules:
  - id: docs.command-drift
    type: noul
    instructions: Does a changed documentation command name a lintpal flag or subcommand that the current CLI does not support?
    criteria:
      "true": The command names a flag or subcommand that does not exist in the current CLI.
      "false": The command uses supported CLI syntax or contains no lintpal command.
    threshold: 0.95
    severity: medium
    title: Possible outdated CLI example
    message: Check this command against lintpal help output.
    paths: ['*.md']
    sides: [RIGHT]
```

The top-level `schema` must be `lintpal.rules.v1`, with a nonempty `rules`
list. Each rule needs a unique `id`, `type`, nonempty `instructions`, a
`threshold` from 0 to 1, `severity` (`low`, `medium`, `high`, or `critical`),
`title`, and `message`. `paths` narrows eligible files with glob patterns;
`sides` can select `LEFT` or `RIGHT`. In this example, `RIGHT` means added or
changed lines in the head revision. Check the source pack for the exact
validated form; unknown YAML fields and duplicate keys are rejected.

| Type | Criteria | Decision |
| --- | --- | --- |
| `noul` | Optional `"true"` and `"false"` descriptions | Fires when the provider's true probability reaches `threshold`. |
| `choice` | Two or more named choices and nonempty `trigger_choices` | Fires when the selected choice is a trigger and its probability reaches `threshold`. |
| `score` | Two to ten ordered descriptions, low to high | Fires when the provider score, normalized to 0–1, reaches `threshold`. |

Thresholds are decision settings, not measured accuracy. Write criteria that
separate a real issue from a harmless change, then calibrate on examples from
your own repository before enabling a blocking gate. Rule instructions and
committed source context go to the selected provider during lint.

## Try, install, and update

From this repository root after [building lintpal](getting-started.md):

```bash
./lintpal pack import docs-review ./examples/rules/docs-review
./lintpal pack verify docs-review
./lintpal lint --base HEAD~1 --head HEAD --rules @docs-review
```

The import makes a local content-addressed copy and records its exact hash in
`.lintpal/packs.lock.json`. Commit the active copy and lockfile together for
offline reproducibility. Import from a public GitHub repository with an
explicit ref such as
`./lintpal pack import team-docs github:acme/lint-rules//docs@v1.2.0`.
lintpal resolves that ref to a commit and records both the requested ref and
resolved commit. Later changes need an explicit `pack update`; ordinary lint
does not fetch GitHub. See [rule pack management](rule-packs.md) for source
syntax, verification, lock drift, and updates.
