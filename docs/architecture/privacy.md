# Privacy and current limits

`lintpal lint` reads committed Git objects at the requested base and head,
or uncommitted working-tree changes when `--uncommitted` is specified. It sends
bounded source context and rule questions to the **selected remote
provider** when using a remote endpoint. Decide whether that transfer is
permitted for the repository before configuring a provider. A `custom`
loopback endpoint can keep the provider exchange on the local machine.

Provider presets read only their selected credential (`TYPESAFE_API_KEY` for
`jev`, `OPENROUTER_API_KEY` for `openrouter`); `custom` reads the variable named
by `--auth-token-env` (`LINTPAL_TOKEN` by default). Tokens are not accepted as
CLI flag values. Before exporting a report, lintpal checks its complete JSON
and rendered output against the **selected credential value** and fails if it
appears. This guard does not scan for unrelated process secrets, and it does
not prevent source transfer to the chosen provider. Avoid putting secrets in
source, rule files, paths, or CI command lines.

`lint` and `doctor` can read a worktree-root `.env` file, or one selected with
`--env-file`. Process environment values take priority, and `--no-env-file`
disables local file loading. lintpal does not export these values into the
process environment. Keep the file untracked; the repository `.gitignore`
excludes `.env` and `.env.*`. The selected credential from either source is
used by both the provider and the report-output guard.

`--metrics` prints fixed stage counts and durations to stderr. The in-process
ADK/OpenTelemetry recorder has no exporter or global registration by default,
so it makes no telemetry request. Metrics are excluded from reports. The
selected credential is also checked before a metrics snapshot is printed;
matching output is suppressed. Local artifacts and stdout remain under the
operator's control.

Git patch, source, context, request, response, concurrency, run duration, and
report sizes have finite limits documented in [resource limits](resource-limits.md).
Oversized or canceled runs fail without a partial report. The CLI reports a
fixed error category instead of raw provider or source text.

`lintpal feedback github` reads an already stored findings v5 artifact and sends
only its deterministic result plus finding text and locations to the selected
GitHub pull request. It reads the GitHub token from an environment variable,
never a value flag, and does not send source context to Jev or any other model.
Fork pull requests are skipped before token use. GitHub event files, API
responses, and pagination links are bounded and treated as untrusted; pagination
must remain on the configured API origin. API errors do not include response
bodies or tokens.

This version covers committed comparisons, uncommitted working-tree linting,
changed-line diagnostics, repository Markdown rules, Markdown/JSON reports, and
GitHub findings publication. It does not apply fixes or emit SARIF. The
offline [evaluation corpus](../development/evaluation.md) checks policy plumbing with fake
provider probabilities; its counts are not live model precision, latency, or
cost guarantees. Live evaluation is an optional operator action.
