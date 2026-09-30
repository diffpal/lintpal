# Configuration

lintpal selects settings in this order: explicit CLI flag, process environment,
worktree-root `.env`, then the CLI default. For a selected provider's key,
process environment takes precedence over `.env`. Both `lint` and `doctor`
read the optional root `.env` automatically. Use `--env-file PATH` to choose a
file or `--no-env-file` to disable file loading. A missing default file is
fine; a missing explicit file is an error. See [`.env.example`](../../.env.example)
and the full [CLI reference](../reference/cli.md).

## Supported providers

Choose TypeSafe/Jev for direct native access, OpenRouter to use your OpenRouter
account, OpenAI for its compatible Decisions preset, or custom to connect your
own compatible service. All four use the same rules, findings, and gate.

| Provider | Credential | Additional setting |
| --- | --- | --- |
| `jev` (default) | `TYPESAFE_API_KEY` | Fixed native endpoint; optional model alias |
| `openrouter` | `OPENROUTER_API_KEY` | Fixed Decisions endpoint; optional model identifier |
| `openai` | `OPENAI_API_KEY` | Fixed assumed Decisions endpoint; required `--model` or `LINTPAL_MODEL` |
| `custom` | `LINTPAL_TOKEN` by default | Required `--base-url`; optional `--api-path` and `--auth-token-env` |

These commands assume a globally installed CLI (`npm install -g lintpal`),
a Git worktree and [repository rules](../rules/authoring.md). All four provider
options are included in [v0.6.0](https://github.com/diffpal/lintpal/releases/tag/v0.6.0).

### TypeSafe / Jev

```bash
export TYPESAFE_API_KEY='your-typesafe-key'
lintpal doctor --provider jev
lintpal lint --uncommitted --provider jev
```

The native endpoint is `https://api.typesafe.ai/v1/systemone`, with model
`jev-latest` by default.

### OpenRouter

```bash
export OPENROUTER_API_KEY='your-openrouter-key'
lintpal doctor --provider openrouter
lintpal lint --uncommitted --provider openrouter
```

OpenRouter uses `https://openrouter.ai/api/alpha/decisions`, with model
`typesafe/jev-1.13` by default.

### OpenAI

The preset assumes OpenAI exposes `https://api.openai.com/v1/decisions` with
the compatible Decisions shape. Its live availability and model identifiers
have not been verified. Replace the model placeholder with one accepted by
that service; LintPal requires an explicit model and supplies no OpenAI default.

```bash
export OPENAI_API_KEY='your-openai-key'
lintpal doctor --provider openai
lintpal lint --uncommitted --provider openai --model '<model-id>'
```

`doctor` checks local configuration and credential presence, not API or model
availability.

### Custom compatible service

```bash
export MY_DECISIONS_API_KEY='your-service-key'
lintpal doctor --provider custom --base-url https://decisions.example.test/api \
  --api-path /v1/decisions --auth-token-env MY_DECISIONS_API_KEY
lintpal lint --uncommitted --provider custom \
  --base-url https://decisions.example.test/api --api-path /v1/decisions \
  --auth-token-env MY_DECISIONS_API_KEY --model '<model-id>'
```

The target is `https://decisions.example.test/api/v1/decisions`: the API path
is appended to the base prefix, preserving `/api`. Without `--api-path` or
`LINTPAL_API_PATH`, custom retains `/v1/systemone`; an explicitly empty path
is invalid. Paths must start with one `/` and contain no query, fragment,
percent escapes, whitespace, backslashes, repeated slashes or `.`/`..` segments.
A base URL must not already end in the selected API path. Loopback custom
endpoints may use HTTP; remote endpoints require HTTPS. Custom services must
accept the same typed `model`, `state`, `questions` request and
`model`, `answers`, `usage` response; this is not a general chat API adapter.

Custom defaults to `LINTPAL_TOKEN` and model `jev-latest`. Select your own
model with `--model` or `LINTPAL_MODEL`, and token variable with
`--auth-token-env` or `LINTPAL_AUTH_TOKEN_ENV`. Custom cannot use the preset
key names `TYPESAFE_API_KEY`, `OPENROUTER_API_KEY`, or `OPENAI_API_KEY`.
Presets reject custom base URL, API path and token-source overrides.

For any provider, explicit model values are sent unchanged. Review existing
model and custom-setting overrides when switching providers. `lint` makes
provider requests; `doctor` never does.

## Rules and reports

Without `--rules`, lintpal loads Markdown from the Git worktree-root
`.lintpal/rules/` directory. There are no built-in mandates. A missing, empty,
or invalid directory fails before contacting a provider. Use `--rules PATH`
to select another local Markdown directory for one lint run. Use
[`rule import`](../rules/import.md) to copy local or GitHub rules into the default
directory, then review and commit the files.
Optional frontmatter sets per-rule severity, threshold, and title. An explicit
`--rule-severity` or `--rule-threshold` overrides frontmatter for all rules;
`--include` and `--exclude` filter changed source paths for the whole run.

The default Markdown feedback goes to stdout. `--format json` selects JSON stdout;
`--out PATH` also writes a complete JSON artifact. Create the destination
directory first. `--fail-on high` is the default gate; `--fail-on none` reports
findings without turning them into exit code `10`. `--block-on` records the
threshold for a later `feedback markdown --gate` or `feedback github --gate` command. Other nonzero codes signal
configuration, provider, or output errors. See [report fields](../reference/report.md) and
[exit codes](../reference/cli.md).

## Common failures

- Missing base or head: provide two locally available committed Git revisions
  with `--base` and `--head`, or select `--uncommitted` for local changes.
- Missing credential: select a provider and set only its key in process env or
  `.env`; run `lintpal doctor` to check presence.
- Invalid rules: run `lintpal rule validate`, inspect the named Markdown files,
  and correct the input before running lint again.
- Report path failure: create the parent directory and ensure it is writable.

Credentials should stay out of Git, shell traces, rule files, and report
paths. The selected provider receives bounded source from the selected committed or
uncommitted comparison and rule questions;
see [privacy](../architecture/privacy.md).
