# Quality evaluation

The required evaluation is offline. Run `go test ./internal/apps/lintpal/eval -count=1` to check the frozen corpus in `internal/apps/lintpal/eval/testdata/corpus.json` against `summary.json` and `reports.json` in the same directory. These eight labeled examples use **fake Noul probabilities** with the real built-in rule selection, decision, and report code. The baseline has 3 true positives, 3 true negatives, 1 false positive, and 1 false negative. Those counts test the evaluator and threshold behavior; they are not measured model precision.

The normal `go test ./...` run also checks the CLI JSON and Markdown goldens, parser fuzz seeds, temporary Git repositories, fake System One transport, redaction, cancellation, and resource limits. The required CI workflow has no live provider secret or endpoint. To update an offline baseline after reviewing changed labels, source examples, and diagnostic diffs, run `UPDATE_EVAL_BASELINE=1 go test ./internal/apps/lintpal/eval -count=1` and review both resulting JSON files. This update is a source change that needs normal code review.

## Optional live run

The Linux helper `scripts/eval-live.sh` requires Bash, Python 3, `sha256sum`, `jq`, Git, a built lintpal binary, a selected provider credential, and an exact model ID. It creates a temporary committed Git repository for each frozen case and runs `lintpal lint --fail-on none` with the explicitly selected provider and model. It pins built-in rules, an empty artifact path, a two-minute timeout, and four workers so inherited `LINTPAL_*` settings cannot change those choices. The provider receives that case's committed source and built-in questions. Both changed sides can be evaluated; classification uses only RIGHT-side findings while elapsed time and token counts cover the complete run. Durations use a monotonic clock.

Example with a trusted custom System One endpoint:

```bash
go build -o /tmp/lintpal-eval ./cmd/lintpal
export LINTPAL_TOKEN='...'
export LINTPAL_EVAL_BIN=/tmp/lintpal-eval
export LINTPAL_EVAL_PROVIDER=custom
export LINTPAL_EVAL_BASE_URL=https://jev.example.internal
export LINTPAL_EVAL_MODEL='<exact-model-id>'
export LINTPAL_EVAL_OUTPUT=/tmp/lintpal-live-before.json
scripts/eval-live.sh
```

For the fixed presets, set `LINTPAL_EVAL_PROVIDER=jev` with `TYPESAFE_API_KEY`, or `openrouter` with `OPENROUTER_API_KEY`; omit the custom base URL. A custom token variable can be selected with `LINTPAL_EVAL_AUTH_TOKEN_ENV`. The helper refuses a missing credential and the default `jev-latest` alias. No live job is part of pull-request CI, and the helper makes no request until an operator runs it.

The local manifest pins the provider, model, corpus SHA-256, binary SHA-256, corpus schema, and case IDs. The binary hash identifies the built-in rule implementation used for the run. The manifest records per-case decisions, elapsed milliseconds, input/output tokens, and aggregate false-positive/false-negative counts. It does not contain source snippets, questions, paths, raw provider bodies, or credentials. The helper writes temporary reports privately and checks the final manifest against the selected credential before an atomic local write. If pricing is known, set both `LINTPAL_EVAL_INPUT_USD_PER_MILLION` and `LINTPAL_EVAL_OUTPUT_USD_PER_MILLION` to nonnegative USD rates. Estimated cost is `(input_tokens × input_rate + output_tokens × output_rate) / 1,000,000`; with no rates, `cost_usd` is `null` and cost is unknown. Prices and latency are observations/inputs, not guarantees.

## Compare an upgrade

Run the optional helper before and after a model or rule-pack change using the **same corpus SHA-256** and an exact model ID for each run. Then run:

```bash
scripts/compare-eval.sh /tmp/lintpal-live-before.json /tmp/lintpal-live-after.json
```

The comparison reports changed case decisions and before/after quality counts, latency, token usage, and cost estimate. It rejects different case IDs, labels, or corpus hashes. The same command accepts two offline summaries for a policy-baseline comparison. Investigate changed safe and buggy decisions before adopting an upgrade; update the frozen corpus or expected outputs only through an explicit reviewed change. The live manifest stays local unless an operator separately decides to share its sanitized contents.
