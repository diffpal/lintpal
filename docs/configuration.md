# Configuration

lintpal selects settings in this order: explicit CLI flag, process environment,
worktree-root `.env`, then the CLI default. For a selected provider's key,
process environment takes precedence over `.env`. Both `lint` and `doctor`
read the optional root `.env` automatically. Use `--env-file PATH` to choose a
file or `--no-env-file` to disable file loading. A missing default file is
fine; a missing explicit file is an error. See [`.env.example`](../.env.example)
and the full [CLI reference](cli.md).

| Provider | Credential | Additional setting |
| --- | --- | --- |
| `jev` (default) | `TYPESAFE_API_KEY` | Preset endpoint; optional model alias |
| `openrouter` | `OPENROUTER_API_KEY` | Preset endpoint; optional model alias |
| `custom` | `LINTPAL_TOKEN` by default | Required `--base-url` or `LINTPAL_BASE_URL` |

The model defaults to `jev-latest`; use `--model` or `LINTPAL_MODEL` for a
model your provider accepts. A custom loopback endpoint may use HTTP; remote
custom endpoints require HTTPS. `doctor` checks local Git and credential
presence without contacting a provider. `lint` makes provider requests.

## Rules and reports

Without `--rules`, lintpal loads Markdown from the Git worktree-root
`.lintpal/rules/` directory. There are no built-in mandates. A missing, empty,
or invalid directory fails before contacting a provider. Use `--rules PATH`
to select another local Markdown directory for one lint run. Use
[`rule import`](rule-import.md) to copy local or GitHub rules into the default
directory, then review and commit the files.
Optional frontmatter sets per-rule severity, threshold, and title. An explicit
`--rule-severity` or `--rule-threshold` overrides frontmatter for all rules;
`--include` and `--exclude` filter changed source paths for the whole run.

The default Markdown feedback goes to stdout. `--format json` selects JSON stdout;
`--out PATH` also writes a complete JSON artifact. Create the destination
directory first. `--fail-on high` is the default gate; `--fail-on none` reports
findings without turning them into exit code `10`. `--block-on` records the
threshold for a later `feedback markdown --gate` or `feedback github --gate` command. Other nonzero codes signal
configuration, provider, or output errors. See [report fields](report.md) and
[exit codes](cli.md).

## Common failures

- Missing base or head: provide two locally available committed Git revisions
  with `--base` and `--head`.
- Missing credential: select a provider and set only its key in process env or
  `.env`; run `lintpal doctor` to check presence.
- Invalid rules: run `lintpal rule validate`, inspect the named Markdown files,
  and correct the input before running lint again.
- Report path failure: create the parent directory and ensure it is writable.

Credentials should stay out of Git, shell traces, rule files, and report
paths. The selected provider receives bounded committed source and questions;
see [privacy](privacy.md).
