# lintpal

**Review committed code changes with questions you control.** lintpal compares
two Git revisions, asks a selected System One provider focused questions, and
reports findings on changed lines. Rules are declarative YAML, so a team can
keep review policy beside its code, pin imported rule packs, and revisit the
same committed inputs.

lintpal is a CLI for review, not an auto-fixer. It reads committed Git objects;
unsaved and uncommitted working-tree changes are outside the comparison.

## Start in a few commands

Install the current public release through npm:

```bash
npm install -g lintpal
lintpal version
```

To use features from this source revision before they reach npm, build it with
Go 1.26.6 or newer:

```bash
go build -o ./lintpal ./cmd/lintpal
./lintpal version
```

Choose a provider credential. For the default `jev` provider, set
`TYPESAFE_API_KEY` in your environment or copy [`.env.example`](.env.example)
to `.env` and fill in the key. The local `doctor` check does not contact the
provider:

```bash
./lintpal doctor --provider jev
mkdir -p .artifacts/lintpal
./lintpal lint --base HEAD~1 --head HEAD --provider jev \
  --format json --fail-on high --out .artifacts/lintpal/report.json
```

Both revisions must be available commits in the repository where you run
lintpal. Use an absolute path to the built binary when reviewing another
repository. The report is printed to stdout and written to the artifact path;
exit code `10` means a finding met the gate **after** the complete report was
written. See [getting started](docs/getting-started.md) for shallow clones,
other providers, and error handling. See [distribution](docs/distribution.md)
for the published package layout and release status.

## Make the review yours

A rule pairs a question with a decision threshold and fixed diagnostic text.
For example, a `noul` rule can ask whether changed Go code drops a meaningful
error:

```yaml
schema: lintpal.rules.v1
rules:
  - id: go.unchecked-error
    type: noul
    instructions: Does the changed Go code discard an error whose failure should be handled?
    threshold: 0.95
    severity: high
    title: Possible unchecked error
    message: Review whether this error needs handling.
    paths: ['*.go']
    sides: [RIGHT]
```

The checked-in [Go review pack](examples/rules/go-review/rules.yaml) also shows
`choice` and `score` rules. The
[documentation review pack](examples/rules/docs-review/rules.yaml) shows a
focused Markdown rule. These are authoring examples, not claims of measured
model accuracy. See [rule authoring](docs/rule-authoring.md) to write and
calibrate rules.

Import a pack explicitly from a local directory, verify its lockfile, and
select it for a review:

```bash
./lintpal pack import go-review ./examples/rules/go-review
./lintpal pack verify go-review
./lintpal lint --base HEAD~1 --head HEAD --rules @go-review
```

GitHub imports require an explicit ref. lintpal resolves it to a commit,
stores a local copy, and records the exact content hash in
`.lintpal/packs.lock.json`. Normal lint runs use the local copy offline; see
[rule packs](docs/rule-packs.md) for source syntax and updates.

## Use it in development

[Task](https://taskfile.dev/docs/installation) runs routine checks from
`Taskfile.yml`:

```bash
task --list
task check
task self-review-smoke
```

`task check` builds, tests, vets, checks formatting, evaluates the frozen
offline corpus, and verifies docs and installed packs without a provider key.
The [CI workflow](.github/workflows/ci.yml) keeps these checks credential-free
on Linux, macOS, and Windows as appropriate. To review lintpal itself with a
real provider, use explicit committed refs:

```bash
BASE=origin/main HEAD=HEAD task self-review
```

This opt-in command writes `.artifacts/lintpal/self-review.json`. Read the
[self-review guide](docs/self-review.md) for provider setup, rule selection,
and gate behavior.

## Documentation

Start at the [documentation index](docs/index.md), or jump to:

| Use lintpal | Extend and maintain it |
| --- | --- |
| [Getting started](docs/getting-started.md) · [Configuration](docs/configuration.md) · [CLI](docs/cli.md) | [Rule authoring](docs/rule-authoring.md) · [Rule packs](docs/rule-packs.md) |
| [Reports](docs/report.md) · [Privacy and limits](docs/privacy.md) | [Development tasks](docs/development.md) · [Self-review](docs/self-review.md) · [Evaluation](docs/evaluation.md) |

When a remote provider is selected, lintpal sends bounded committed source
context and rule questions to it. Review the [privacy guide](docs/privacy.md)
before running a live review. The current CLI does not inspect uncommitted
changes, apply fixes, or report measured model precision. lintpal is available
under the [MIT license](LICENSE).
