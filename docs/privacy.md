# Privacy and current limits

`lintpal lint` reads committed Git objects at the requested base and head. It
does not inspect uncommitted working-tree source for lint context. It sends
bounded committed source context and rule questions to the **selected remote
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

This MVP covers committed comparisons, changed-line diagnostics, built-in or
declarative rules, and `human`/`json` reports. It does not analyze uncommitted
changes, apply fixes, emit SARIF, or integrate directly with a PR host. The
offline [evaluation corpus](evaluation.md) checks policy plumbing with fake
provider probabilities; its counts are not live model precision, latency, or
cost guarantees. Live evaluation is an optional operator action.
