# lintpal CLI

`lintpal lint` reads a committed Git comparison from the current repository.
It never reads uncommitted source for lint context. The command writes one
complete report to stdout; operational messages use stderr.

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

The default provider is `jev` (TypeSafe), format is `human`, model alias is
`jev-latest`, and gate is `high`. TypeSafe's [System One API](https://api.typesafe.ai/docs)
documents model discovery through `/v1/models`; set `--model` to an alias your
selected provider accepts. The CLI does not query the model catalog during lint.

| Flag | Environment setting | Purpose |
| --- | --- | --- |
| `--base` | `LINTPAL_BASE` | Required base revision. |
| `--head` | `LINTPAL_HEAD` | Required head revision. |
| `--provider` | `LINTPAL_PROVIDER` | `jev`, `openrouter`, or `custom`. |
| `--model` | `LINTPAL_MODEL` | System One model name or alias. |
| `--rules` | `LINTPAL_RULES` | Markdown rule directory or installed `@NAME` pack; built-ins otherwise. |
| `--include` | — | Include changed source paths matching a glob; repeatable. |
| `--exclude` | — | Exclude changed source paths matching a glob; repeatable. |
| `--rule-threshold` | `LINTPAL_RULE_THRESHOLD` | Override every rule's true-probability threshold, 0–1; default 0.95. |
| `--rule-severity` | `LINTPAL_RULE_SEVERITY` | Override every rule's severity; default medium. |
| `--format` | `LINTPAL_FORMAT` | `human` or `json` on stdout. |
| `--out` | `LINTPAL_OUT` | Additional atomic JSON artifact path. |
| `--fail-on` | `LINTPAL_FAIL_ON` | `low`, `medium`, `high`, `critical`, or `none`. |
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

| Exit code | Meaning |
| ---: | --- |
| 0 | Completed without a gate failure. |
| 2 | Invalid CLI/configuration/revision input or nonretryable provider setup. |
| 3 | Retryable provider failure or timeout. |
| 4 | Report or artifact export failure. |
| 5 | Internal, protocol, or unclassified local failure. |
| 10 | Complete report written; finding met the gate. |
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

[Rule packs](rule-packs.md) documents local and GitHub import, the project
lockfile, offline verification, and example rules.
