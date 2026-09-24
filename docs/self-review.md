# Review lintpal with lintpal

`task self-review` runs lintpal against two committed revisions of this
repository. It builds a local binary, invokes `lintpal lint`, and writes the
complete JSON report to `.artifacts/lintpal/self-review.json` and Markdown
feedback to stdout. This directory
is ignored by Git. After checking `BASE` and `HEAD`, the target removes an
older report before building and running lintpal.

Install [Task](development.md), then make both revisions available locally.
For example, fetch your base branch before comparing it with `HEAD`:

```bash
git fetch origin main
export TYPESAFE_API_KEY='your-token'
BASE=origin/main HEAD=HEAD task self-review
```

`BASE` and `HEAD` are required. Use commit IDs or revision names that Git can
resolve in this worktree; a shallow checkout may need a deeper fetch. The
default provider is `jev`, with the repository's `.lintpal/rules/` and a
`high` severity gate.
Use `PROVIDER=openrouter` with `OPENROUTER_API_KEY`, or put the selected
provider settings and credential in a project-root `.env` as described in the
[CLI guide](cli.md). The Taskfile never prints the credential.

To use more rules, [import them](rule-import.md) into `.lintpal/rules/`, review
the files, and run self-review normally:

```bash
BASE=origin/main HEAD=HEAD task self-review
```

`RULES` optionally selects another local Markdown directory for this run.
Set `FAIL_ON=none` to keep
findings in the report without failing the severity gate. `PROVIDER`, `RULES`,
and `FAIL_ON` are optional. For a custom provider, set `PROVIDER=custom` and
configure `LINTPAL_BASE_URL` and `LINTPAL_TOKEN` through process environment
or `.env`.

A clean run exits successfully. When a finding reaches the gate, lintpal
writes the complete JSON report and exits with code `10`; Task reports that
as a failed target. Other nonzero results mean setup, provider, or export
failure; see [CLI exit codes](cli.md). Inspect the report only when the current
run produced it. `task self-review-smoke` uses a local fake provider and needs
no account or real credential. CI runs this smoke target and does not run a
live self-review.

A live self-review sends bounded committed source context and rule questions
to the selected provider. It can incur provider usage charges. Read
[privacy and limitations](privacy.md) before choosing an endpoint or sharing
the report.
