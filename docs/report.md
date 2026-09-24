# Shared findings v5

`lintpal lint --format json` writes one UTF-8 JSON object followed by a
newline. `--out PATH` writes the complete JSON object atomically regardless of
stdout format. DiffPal also
writes a v5 findings bundle. The canonical contract is in DiffPal at
`schemas/findings/v5.schema.json`; LintPal keeps a pinned
[offline copy](schema/findings-v5.schema.json). The
[JSON](../cmd/lintpal/testdata/golden/report.json) and
[Markdown](../cmd/lintpal/testdata/golden/report.md) goldens show LintPal output.

Both tools use `version: "v5"`, `review_id`, `base_sha`, `head_sha`, and
`findings`. LintPal also emits `merge_base_sha`, `skips`, and `stats`.
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

DiffPal code findings use `evidence.kind: "code"` with `anchor`,
`reasoning_basis`, and `source`; they include structured `impact` and numeric
`confidence`. These are the same fields previously used by DiffPal, now with
an explicit evidence kind. See the [shared schema](schema/findings-v5.schema.json)
for exact types and optional metadata.

Each LintPal skip has `reason` and at least one of `old_path` or `new_path`.
Reasons are `binary`, `non_regular`, and `no_changed_lines`. `stats` records
work items, skips, groups, batches, questions, findings (under the historical
counter name `diagnostics`), and provider-reported token totals. A zero token
count does not prove the provider used no tokens.

The gate runs after the complete report is written. The default
`--fail-on high` returns exit code `10` for a high or critical finding;
`--fail-on none` disables that exit code. `lint` writes Markdown to stdout by
default; `feedback markdown --in PATH` renders stored v5 findings without
changing their `blocking` values. Its optional `--gate` returns exit 10 after
complete feedback output when any finding is blocking. See [CLI exit codes](cli.md).

## Migration

The previous LintPal `lintpal.report.v1` object used `schema_version` and
`diagnostics[]`. In v5, use `version` and `findings[]`. Move `rule_id` into
`evidence.rule_id`, side and range into `changed_span`, and the numeric answer
from `evidence` into `decision`. DiffPal v4 readers remain available in
DiffPal, while its new writes use v5. Consumers of either tool should branch
on `version` and adopt the [shared schema](schema/findings-v5.schema.json).
