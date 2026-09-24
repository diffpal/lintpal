# lintpal architecture

The executable lives in `cmd/lintpal`. All application packages live under
`internal/apps/lintpal`.

```text
cmd/lintpal
  -> internal/apps/lintpal/di (Fx composition)
  -> internal/apps/lintpal/cli (Cobra commands)
  -> internal/apps/lintpal/app (one-shot Linter)
  -> internal/apps/lintpal/jev (typed decision port)
```

The arrows describe allowed dependency flow. Plain constructors in `app` and
the typed Jev port can be used without Cobra, Fx, ADK, credentials, or a live
provider. The DI package creates a run-scoped Fx graph after Cobra resolves
trusted lint options.
`internal/apps/lintpal/runtime/adk` owns local ADK telemetry providers; the
Fx module registers their shutdown hook. It does not install global providers,
configure an exporter, run an ADK agent, or convert typed Jev decisions into
ADK model requests. Its local observer records only fixed stage/status names,
counts, and durations. Trace attributes are fixed status and count values;
source text, paths, questions, URLs, and errors do not enter telemetry.

This foundation requires Go 1.26.6 because the pinned ADK v2.4.0 module
declares that minimum. Cobra v1.10.2 and Fx v1.24.0 are pinned in `go.mod`.
Build and verify locally with `go build ./...`, `go test ./...`, and
`go vet ./...`. `go run ./cmd/lintpal --help` displays the command tree;
`docs/cli.md` records its flags and exit contract.

Git scope, bounded context, rules, the System One transport, lint orchestration,
and reports now live under `internal/apps/lintpal`. The CLI Story binds their
constructors in `di` and owns process options and exit behavior. The ADK runtime
does not own Jev answers.

## Committed Git input

`internal/apps/lintpal/git.NewRepository(dir, limits).Compare(ctx, base, head)`
resolves both revisions to commits, requires one merge base, and reads a
bounded raw diff and patch. It returns ordered LEFT/RIGHT `WorkItem` values
with stable IDs and changed-line spans. `Result.Source(item)` returns a copy of
the corresponding merge-base or head blob; it never reads the working tree.
Binary, non-regular, and line-free changes appear in `Result.Skips`. Invalid
revisions, ambiguous merge bases, malformed diffs, limit breaches, and canceled
commands return an error with no partial result. Default limits are 8 MiB per
diff output, 2 MiB per blob, 16 MiB total source, and 10,000 items; `Limits`
allows lower values. `contextplan.Assemble` slices these object-backed sources
into bounded hunk context. The CLI Story owns user-facing flags and exit codes.

## Bounded context and batching

`internal/apps/lintpal/contextplan.Assemble` reads only `git.Result.Source` for
the committed work items. It sorts items by path, side, hunk, and line, renders
numbered source lines with a two-line surrounding window, and retains each
work-item ID and changed-line span in exactly one `Group`. Compatible hunks on
one file side may share a group; a size boundary splits between hunks. An
indivisible oversized hunk, malformed source, or cancellation returns no groups.

`contextplan.Plan` accepts typed question bindings from `rules.Questions`.
It batches questions with byte-identical state, splitting only question sets
when needed. Each `Batch` contains a native `jev.Request`, the group IDs, and
an answer-ID-to-work-item-ID mapping for later anchored diagnostics. It rejects
unknown or duplicate IDs, invalid questions, oversized requests, and
cancellation without returning a partial plan. `app.Linter` owns invocation
order, run deadlines, and concurrency; this package performs no provider calls.

Default local caps are 20,000 bytes per state, 24,000 serialized JSON bytes per
request, a 24,000-byte portable context budget, 16 MiB total assembled state,
16 MiB total planned requests, 10,000 items/groups, 100,000 questions, and 128
questions per batch. Callers can lower limits but cannot exceed finite hard
caps. One serialized UTF-8 byte counts as one token in the conservative local
estimate; the actual provider tokenizer may differ. The request cap remains
well below the System One transport's 1 MiB guard, and the byte budget reserves
headroom beneath the portable 32K token target. Limit errors contain no source
or question text. Source disclosure to a remote provider occurs when
`app.Linter` sends a batch.

## Declarative rules and policy

`internal/apps/lintpal/rules` accepts one UTF-8 YAML document with
`schema: lintpal.rules.v1` and a nonempty `rules` list. Each rule declares
`id`, `type` (`noul`, `choice`, or `score`), `instructions`, `threshold` in
`[0,1]`, `severity` (`low`, `medium`, `high`, `critical`), fixed `title` and
`message`, and optional `paths` and `sides`. Choice requires option
`criteria` and `trigger_choices`; Score requires an ordered criteria list;
Noul may provide quoted `"true"` and `"false"` criteria. Unknown fields,
duplicate keys/IDs, aliases, merge keys, custom tags, malformed selectors,
and executable fields are rejected. The loader caps a file at 256 KiB and a
pack at 256 rules. `LoadContext` supports cancellation; `Load` is the
background-context convenience call.

For applicability, a pattern without `/` matches the path basename, so
`*.go` covers Go files at any depth. A pattern with `/` matches the full
repo-relative path using Go `path.Match` semantics. Empty `paths` or `sides`
means all paths or both LEFT and RIGHT. `Select` orders work items and rules
deterministically; `Questions` generates native Jev questions with IDs of the
form `<work-item-id>/<rule-id>` for `contextplan.Plan`. The total selected
rule text is capped at 16 MiB before question construction.

`Decide` first validates the complete typed Jev response against its batch.
Noul triggers when its yes probability is at least the rule threshold. Choice
triggers only for a declared trigger option whose selected probability reaches
the threshold. Score triggers when `score / (number of levels - 1)` reaches
the threshold. Equality triggers. A decision gets its title, message, severity
and rule ID from the rule, and path, side and changed lines from the Git work
item. Numeric evidence is labeled as a Noul probability, selected Choice
probability, or normalized Score. The model cannot supply free-form diagnostic
text or an anchor.

`BuiltIn()` contains two conservative Go Noul examples for possible shell
injection and ignored errors, each at a 0.95 threshold. Hermetic examples
verify policy behavior; they do not measure model precision. Story .9 must
calibrate the pack on a frozen corpus. `app.Linter` takes an already selected
pack, executes batches, and constructs anchored reports. Repository rules
cannot choose a provider destination or credential source.

`internal/apps/lintpal/packs` acquires one declarative `rules.yaml` per named
pack through explicit `pack import`/`pack update` commands. It validates source
bytes with the same rule loader, stores content-addressed copies under
`.lintpal/packs/`, and atomically switches `.lintpal/packs.lock.json` after the
copy is ready. A GitHub source is resolved to a commit before fetching the YAML.
`pack verify` and `--rules @NAME` check the lock hash and rule validity offline;
direct paths into managed storage are rejected. The legacy `--rules PATH` path
and built-in default remain available. See [rule packs](rule-packs.md).

## System One provider

`internal/apps/lintpal/jev.Provider` remains the native typed decision port.
Its Noul, Choice, and Score questions carry the criteria documented by the
System One API; Choice and Score answers preserve distributions and confidence
for later rule policy. `jev.ValidateRequest` and `jev.ValidateResponse` reject
invalid or partial decisions without echoing state or question text.

`internal/apps/lintpal/provider/systemone` implements the port with one
`POST /v1/systemone` transport. `TypeSafe()` fixes the native destination and
`TYPESAFE_API_KEY`; `OpenRouter()` fixes its destination and
`OPENROUTER_API_KEY`. `TrustedCustom(baseURL, tokenEnv)` must be called only
from trusted process settings, never repository rules/config. It does not
inherit preset tokens and rejects their environment variable names. Custom
HTTP is limited to loopback; remote endpoints require HTTPS. Redirects are not
followed with credentials. The final CLI Story owns trusted option precedence
and should avoid a raw token argument.

Each call has a 1 MiB request and 2 MiB response limit, at most three attempts,
and a 15-second per-attempt deadline. Transient network failures, 429, 529,
and 5xx may retry with capped backoff; ordinary 4xx and malformed answers do
not. Errors contain safe categories or HTTP status only. The provider writes
no payload to telemetry or logs. Remote provider use sends the bounded state
and question instructions to the selected service; release documentation must
disclose that transfer. `app.Linter` connects the provider to lint orchestration;
the CLI resolves trusted process configuration before constructing it.

## Lint application and report handoff

`app.NewLinter(comparer, provider, pack)` composes a Git comparer, a typed Jev
provider, and a validated rule pack without Cobra, Fx, or ADK LLM imports.
`Linter.Lint(ctx, app.Request{Base, Head, Model, ProviderName, Limits})` compares
commits, assembles context, selects rules, plans batches, validates every typed
answer and rule decision, and returns one `report.Report`. Errors return a zero
report. A comparison with no applicable rules completes without provider calls.
The provider's returned model is used in diagnostics; inconsistent models
across batches fail the run.

`app.Limits` defaults to four provider workers, a two-minute whole-run
deadline, and a 16 MiB rendered JSON report limit. The hard ceilings are 16
workers, ten minutes, and 64 MiB. Context limits are passed through to
`contextplan`. Provider results occupy stable batch slots before final sorting;
the first failure cancels siblings, and no partial findings are exposed.
`report.New` checks decisions against the original Git work items and sorts
diagnostics and skips. The versioned `lintpal.report.v1` artifact includes
resolved revisions, evidence, skips, and count/usage stats without source state,
question text, or credentials.

`report.WriteJSON` and `report.WriteHuman` validate and write the same artifact.
`report.WriteAndGate(writer, artifact, format, threshold)` writes the complete
output first, then returns `report.ErrGate` for a finding at or above the
inclusive severity threshold. `none` disables the gate. A writer failure
returns `report.ErrExport` before gate evaluation. The CLI maps these errors
to process exit codes, writes the optional JSON artifact before stdout, and
binds the Linter through Fx. The executable uses `cli.NewRoot` and
`di.ExecuteLint`; the placeholder service and container have been removed.

The DI layer checks the selected credential against every report string field
and the rendered stdout and JSON artifact bytes before writing either sink.
An unsafe report fails with the export category and no partial artifact. The
local `--metrics` snapshot is printed to stderr only when requested and is
suppressed if its bytes contain that credential. These checks protect known
selected credentials at the output boundary; they do not scan unrelated
environment variables. The selected provider still receives committed source
context and rule instructions to perform linting.

The executable exposes `lint`, `doctor`, `pack`, `version`, and `completion`.
Its core packages import neither Cobra, Fx, nor ADK LLM APIs. There is no
PR-host publisher, working-tree analysis, SARIF writer, autofix, RAG, general
chat path, or embedded Laya sidecar. [Resource limits](resource-limits.md)
lists boundary owners and adversarial tests.
