# `lintpal.report.v1` report reference

`lintpal lint --format json` writes one UTF-8 JSON object followed by a newline.
`--format human` writes a line-oriented rendering of the same report. The
checked-in [JSON](../cmd/lintpal/testdata/golden/report.json) and
[human](../cmd/lintpal/testdata/golden/report.txt) goldens show a two-diagnostic
comparison. `--out PATH` writes an additional JSON artifact atomically, even
when stdout uses the human format.

## Top-level JSON fields

| Field | Meaning |
| --- | --- |
| `schema_version` | Exact string `lintpal.report.v1`. |
| `base_sha`, `head_sha` | Full Git object IDs of the requested committed revisions (40 or 64 hex digits). |
| `merge_base_sha` | Full Git object ID of their merge base; it may equal `base_sha`. |
| `diagnostics` | Ordered array of accepted findings; empty array when none. |
| `skips` | Ordered array of files excluded from changed-line work; empty array when none. |
| `stats` | Nonnegative run counts and provider-reported token totals. |

Each diagnostic contains:

| Field | Meaning |
| --- | --- |
| `rule_id` | Declarative rule identifier. |
| `work_item_id` | Stable identifier of the changed-line work item that produced the question. |
| `severity` | `low`, `medium`, `high`, or `critical`; used by the gate. |
| `path` | Git path of the anchored changed side. |
| `side` | `LEFT` or `RIGHT` side of the comparison. |
| `start_line`, `end_line` | Inclusive, one-based changed-line range on that side. |
| `title`, `message` | Rule finding text; consumers should treat it as data, not executable instructions. |
| `provider`, `model` | Selected provider and requested model identity for this run. |
| `evidence` | Normalized decision evidence described below. |

The report constructor checks each finding against the original Git work
item's path, side, and line range and rejects duplicate `work_item_id` plus
`rule_id` pairs. A diagnostic points to a changed-line work item, not an
arbitrary line proposed by a model. A model answer alone is not proof of a bug.

`evidence.kind` is `noul_probability`, `selected_probability`, or
`normalized_score`. `evidence.value` is a finite number from 0 through 1.
The optional `evidence.confidence` is also a finite number in that range;
it appears only when supplied by the selected answer type. These values
explain the rule decision and are not calibrated probabilities of correctness.

Each skip has `reason` and at least one of `old_path` or `new_path`. Missing
paths are omitted in JSON. Reasons are `binary`, `non_regular` (for example a
symlink), and `no_changed_lines`. A skip is not a diagnostic and does not meet
the severity gate.

| `stats` field | Meaning |
| --- | --- |
| `work_items` | Number of changed-line work items assembled from Git. |
| `skipped` | Number of skip records; equals `skips.length`. |
| `groups` | Context groups assembled for rule selection. |
| `batches` | Provider request batches planned for the run. |
| `questions` | Rule questions bound to work items. |
| `diagnostics` | Number of emitted diagnostics; equals `diagnostics.length`. |
| `input_tokens`, `output_tokens` | Sum of usage values reported by provider responses; zero does not imply zero actual usage. |

Diagnostics are sorted by `path`, `side`, `start_line`, `end_line`, `rule_id`,
then `work_item_id`, using ascending bytewise/string and numeric order as
appropriate. Skips are sorted by `old_path`, `new_path`, then `reason`. JSON
field order follows the current renderer, but consumers should parse by field
name. Both arrays are always present, including when empty.

## Human output and gate

The first human line names the schema and three commit IDs. Diagnostic lines
show severity, side, quoted path, inclusive range, quoted rule/title/message,
evidence, and quoted provider/model. `confidence` appears after evidence when
present. Skip lines show reason and quoted old/new paths. The final `stats`
line lists the counters above. Strings use Go-style quoting, so embedded
newlines do not create extra output lines. Human output is intended for reading;
use JSON for integrations.

The gate runs **after** a complete report is written. `--fail-on high`, for
example, returns exit code `10` for a `high` or `critical` diagnostic; lower
severities remain in the report. `--fail-on none` disables that exit code.
The default threshold is `high`. Other exit codes and export behavior are in
the [CLI guide](cli.md).

## Compatibility

Consumers should check `schema_version` before interpreting fields. The v1
contract covers these names, types, meanings, allowed enum values, and ordering.
A change to those semantics requires a new schema version and updated goldens.
Future additive fields should be ignored by tolerant v1 readers. Exact human
wording and JSON object key order are presentation details, not parsing APIs.
