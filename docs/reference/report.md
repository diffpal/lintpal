# LintPal findings v5

`lintpal lint --format json` writes one UTF-8 JSON object followed by a
newline. `--out PATH` writes the complete JSON object atomically regardless of
stdout format. LintPal owns and evolves the v5 contract documented by its
[JSON schema](../schema/findings-v5.schema.json). The format originated from
the DiffPal v5 schema, which remains a compatibility baseline and design
reference rather than an authority for LintPal changes. The
[JSON](../../cmd/lintpal/testdata/golden/report.json) and
[Markdown](../../cmd/lintpal/testdata/golden/report.md) goldens show LintPal output.

LintPal uses `version: "v5"`, `review_id`, `base_sha`, `head_sha`, and
`findings`. It also emits `merge_base_sha`, `skips`, and `stats`.
`findings` is always an array, including on a clean review. The IDs are
deterministic for the review inputs and finding location. Consumers should
parse by field name; JSON key order is not a contract.

Each finding has `id`, `review_id`, `category`, `severity`, `path`,
`start_line`, `end_line`, `changed_span`, `title`, `message`, `evidence`,
`blocking`, and `provider`. `changed_span` repeats the path and one-based
inclusive range and adds `side`: `LEFT` for deleted old-revision lines or
`RIGHT` for added new-revision lines. Both sides are reviewed. A finding is
anchored to a changed-line work item, not an arbitrary model-proposed line.

For LintPal, `category` is `rule_violation` and evidence references the
Markdown rule, for example:

```json
{
  "evidence": {"kind": "rule", "rule_id": "go/unchecked-error.md"},
  "decision": {"kind": "noul_probability", "value": 0.97}
}
```

The number is the provider's true probability used for the threshold
decision. It is not a calibrated probability that the code is wrong. Rule
findings omit `confidence` and `impact`. Their message is fixed and includes
the rule ID. Optional `model` and `work_item_id` identify the run and question.
Severity comes from frontmatter or an explicit override. `blocking` indicates
whether the finding meets the run's selected `--fail-on` or `--block-on`
threshold. With `--block-on`, lint records the flag but does not gate.

The LintPal v5 schema retains support for DiffPal-style code findings with
`evidence.kind: "code"`, structured `impact`, and numeric `confidence`. See the
[LintPal schema](../schema/findings-v5.schema.json) for exact v5 types and
optional metadata.

Each LintPal skip has `reason` and at least one of `old_path` or `new_path`.
Reasons are `binary`, `non_regular`, and `no_changed_lines`. `stats` records
work items, skips, groups, batches, questions, findings (under the historical
counter name `diagnostics`), and provider-reported token totals. Its optional
`review` object records provider request attempts, evaluate-stage wall-clock
milliseconds, and optional provider-specific USD cost. The direct TypeSafe
adapter calculates Jev cost from reported input tokens at the
[published $0.042 per million rate](https://typesafe.ai/blog/introducing-system-one-models-and-jev);
output tokens are free. The OpenAI adapter calculates `gpt-6-luna` Standard
short-context cost from reported uncached input, cached input, cache writes,
and output tokens using the
[published OpenAI rates](https://developers.openai.com/api/docs/pricing).
Provider-reported cost takes precedence when available. Other models and
adapters omit `cost_usd` when no supported price or provider cost is available.
A zero token count does not prove the provider used no tokens.

The gate runs after the complete report is written. The default
`--fail-on high` returns exit code `10` for a high or critical finding;
`--fail-on none` disables that exit code. `lint` writes Markdown to stdout by
default; both feedback commands consume stored v5 findings without changing
their `blocking` values. `feedback markdown --in PATH` renders local Markdown.
`feedback github --in PATH` computes a gate-status/count block and inline
finding bodies from the same fields, without a provider call or semantic
summary. Their optional `--gate` returns exit 10 only after complete output or
successful publication when any finding is blocking. See [CLI exit codes](cli.md).

## Migration

The previous LintPal `lintpal.report.v1` object used `schema_version` and
`diagnostics[]`. In v5, use `version` and `findings[]`. Move `rule_id` into
`evidence.rule_id`, side and range into `changed_span`, and the numeric answer
from `evidence` into `decision`. Compatibility with DiffPal is intentional
where useful but is not guaranteed in lockstep. Consumers should branch on
`version` and `evidence.kind` when handling stored findings.
