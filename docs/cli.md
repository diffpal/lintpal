# lintpal CLI

`lintpal lint` reads a committed Git comparison from the current repository.
It never reads uncommitted source for lint context. By default it writes
Markdown feedback to stdout; `--format json` writes the findings v5 report.
Operational messages use stderr.

```bash
export TYPESAFE_API_KEY='...'
lintpal lint --base origin/main --head HEAD --provider jev

export OPENROUTER_API_KEY='...'
lintpal lint --base origin/main --head HEAD --provider openrouter --format json

export LINTPAL_TOKEN='...'
mkdir -p .artifacts/lintpal
lintpal lint --base origin/main --head HEAD --provider custom \
  --base-url https://jev.example.internal --model jev-latest \
  --format json --out .artifacts/lintpal/report.json
```

The default provider is `jev` (TypeSafe), format is `markdown`, model alias is
`jev-latest`, and gate is `high`. TypeSafe's [System One API](https://api.typesafe.ai/docs)
documents model discovery through `/v1/models`; set `--model` to an alias your
selected provider accepts. The CLI does not query the model catalog during lint.

| Flag | Environment setting | Purpose |
| --- | --- | --- |
| `--base` | `LINTPAL_BASE` | Required base revision. |
| `--head` | `LINTPAL_HEAD` | Required head revision. |
| `--provider` | `LINTPAL_PROVIDER` | `jev`, `openrouter`, or `custom`. |
| `--model` | `LINTPAL_MODEL` | System One model name or alias. |
| `--rules` | `LINTPAL_RULES` | Override the default `.lintpal/rules/` directory for this lint run. |
| `--include` | — | Include changed source paths matching a glob; repeatable. |
| `--exclude` | — | Exclude changed source paths matching a glob; repeatable. |
| `--rule-threshold` | `LINTPAL_RULE_THRESHOLD` | Override every rule's true-probability threshold, 0–1; default 0.95. |
| `--rule-severity` | `LINTPAL_RULE_SEVERITY` | Override every rule's severity; default medium. |
| `--format` | `LINTPAL_FORMAT` | `markdown` or `json` on stdout; default `markdown`. |
| `--out` | `LINTPAL_OUT` | Additional atomic JSON artifact path. |
| `--fail-on` | `LINTPAL_FAIL_ON` | `low`, `medium`, `high`, `critical`, or `none`. |
| `--block-on` | — | Mark findings `blocking` at this threshold without failing `lint`; same values as `--fail-on`. Cannot be combined with explicit `--fail-on`. |
| `--timeout` | `LINTPAL_TIMEOUT` | Whole-run deadline; default `2m`, maximum `10m`. |
| `--max-concurrency` | `LINTPAL_MAX_CONCURRENCY` | Provider workers; default `4`, maximum `16`. |
| `--base-url` | `LINTPAL_BASE_URL` | Required for `custom`; rejected for presets. |
| `--auth-token-env` | `LINTPAL_AUTH_TOKEN_ENV` | Custom token variable name; defaults to `LINTPAL_TOKEN`. |
| `--metrics` | — | Print fixed local stage counts and durations to stderr. |
| `--env-file` | — | Read settings and selected credential from an explicit `.env` file. |
| `--no-env-file` | — | Disable automatic `.env` loading. |

`lint` and `doctor` automatically read `.env` at the Git worktree root when it
exists, including when run from a subdirectory. Both accept `--env-file PATH` to
select another file or `--no-env-file` to disable loading; these options cannot
be combined. A missing default file is harmless; a missing explicit file or
invalid file fails before contacting the provider. The file accepts UTF-8
`KEY=VALUE` lines, blank lines, full-line `#` comments, and single or double
quoted values. It does not evaluate shell syntax or expand variables. Files
larger than 64 KiB, duplicate or malformed keys, and symlinks are rejected.
Start from [`.env.example`](../.env.example) and fill in your own credential.

An explicitly supplied flag wins over its process environment setting; that
setting wins over `.env`, which wins over the default. The selected provider's
key follows process environment over `.env`. Put `.env` in `.gitignore` and keep
it out of committed source. A rule file is untrusted policy input and cannot
select provider, endpoint, or token source. Presets always read
`TYPESAFE_API_KEY` or `OPENROUTER_API_KEY` respectively. Custom HTTP endpoints
are restricted to loopback; remote custom endpoints require HTTPS. Raw token
flags are not supported. A remote provider receives the bounded committed
source context and question instructions required for linting. Reports and
error messages exclude source state, questions, credentials, and raw responses.
Before writing a report or artifact, the executable checks the complete
report and rendered bytes for the selected credential value. A match fails
with exit code 4 and no report output. It does not scan for other process
secrets or prevent source transfer to the selected provider.

`--metrics` prints one line per completed stage, for example
`metric stage=compare status=ok count=1 duration_ms=18`. Stage and status names
are fixed; the values are nonnegative counts and elapsed milliseconds. No
source, path, question, endpoint, token, raw response, or error text enters
the local recorder or its trace attributes. The recorder uses an in-process
ADK/OpenTelemetry provider without an exporter or global registration, so it
does not make telemetry requests. Metrics stay out of the v5 findings bundle.
If the selected credential happens to match a metrics line, that snapshot is
suppressed. Durations vary by run and are not a performance guarantee.

`--out` writes JSON to a temporary file in the destination directory and
renames it only after the complete artifact is written. With both sinks,
lintpal writes the artifact, then stdout, then evaluates the severity gate.
`--fail-on none` keeps findings in the report and disables exit code 10.
`--block-on` sets the same `blocking` field but returns success after lint,
allowing a later feedback command to apply the gate. An explicit `--block-on`
overrides `LINTPAL_FAIL_ON`; combining it with an explicit `--fail-on` is an
input error.

For a single command with Markdown stdout and the default high gate:

```bash
mkdir -p .artifacts/lintpal
lintpal lint --base origin/main --head HEAD --out .artifacts/lintpal/findings.json
```

For a two-command CI flow, first write findings, then render and gate them:

```bash
mkdir -p .artifacts/lintpal
lintpal lint --base origin/main --head HEAD --block-on high \
  --format json --out .artifacts/lintpal/findings.json
lintpal feedback markdown --in .artifacts/lintpal/findings.json \
  --out .artifacts/lintpal/feedback.md --gate
```

`feedback markdown` reads only the stored findings v5 file. It makes no provider
request and needs no provider credential. Without `--out`, it writes Markdown to
stdout. Without `--gate`, blocking findings do not change its exit status.
With `--gate`, it returns exit code 10 after the Markdown is written if any
stored finding has `blocking: true`. An invalid report fails before output.

| Exit code | Meaning |
| ---: | --- |
| 0 | Completed without a gate failure. |
| 2 | Invalid CLI/configuration/revision input or nonretryable provider setup. |
| 3 | Retryable provider failure or timeout. |
| 4 | Report or artifact export failure. |
| 5 | Internal, protocol, or unclassified local failure. |
| 10 | Complete lint report or feedback written; a finding met the gate. |
| 130 | Interrupted or canceled. |

`lintpal doctor` checks local Git/repository availability and whether the
selected provider's credential is present. It makes no provider request and
prints no credential value. `lintpal version` prints the build version (`dev`
in an unreleased build). `lintpal completion bash|zsh|fish|powershell`
generates a shell completion script. [Resource limits](resource-limits.md)
records finite caps and their tests. [Privacy and limitations](privacy.md)
explains provider transfer and the output guard.

[Quality evaluation](evaluation.md) describes the offline baseline, optional
local live procedure, and model-upgrade comparison. [Report reference](report.md)
defines the versioned output contract.

`lintpal rule list` prints the IDs in the worktree-root `.lintpal/rules/`
directory. `lintpal rule view ID` shows one mandate with its effective title,
severity, and threshold. `lintpal rule validate` checks the whole directory.
These commands are local and do not need provider credentials. `lintpal rule
import SOURCE` copies a local or pinned GitHub Markdown directory into the
same rule root; see [rule import](rule-import.md). Rules are required for lint;
missing or invalid defaults fail before a provider request. `--rules PATH`
selects another local directory for a single lint run.
