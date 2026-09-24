# Getting started

lintpal needs Go 1.26.6 or newer to build from this source tree. From the
repository root:

```bash
go build -o ./lintpal ./cmd/lintpal
./lintpal version
./lintpal --help
```

The local build reports version `dev`. The planned npm release is described in
[distribution staging](distribution.md); this source build works today.

## Choose a committed comparison

lintpal reads Git commits, not unsaved or uncommitted working-tree changes.
Make sure both revisions exist locally. For this repository, `HEAD~1` and
`HEAD` are convenient when it has at least two commits:

```bash
git rev-parse HEAD~1 HEAD
```

If that fails in a shallow checkout, fetch the history or choose two commit
IDs that are present. When reviewing another repository, run the built lintpal
binary from that repository's directory and use refs from that repository.

## Configure a provider and review

The default `jev` provider reads `TYPESAFE_API_KEY`. Set it in your process
environment or in a worktree-root `.env` copied from
[`.env.example`](../.env.example). Do not commit the filled file. Check local
configuration without contacting the provider:

```bash
./lintpal doctor --provider jev
```

Then lint a committed comparison in this repository:

```bash
mkdir -p .artifacts/lintpal
./lintpal lint --base HEAD~1 --head HEAD --provider jev \
  --out .artifacts/lintpal/report.json
```

Markdown feedback is printed to stdout and the findings v5 report is written
as JSON to the artifact path.
Exit code `10` means a finding reached the severity gate after a complete
report was written. Other nonzero codes indicate setup, provider, or export
failure; see the [CLI reference](cli.md). A remote provider receives bounded
committed source context and rule questions; read [privacy](privacy.md) before
running a live review.

Next, [configure providers and rules](configuration.md),
[import a rule pack](rule-packs.md), or use the
[Taskfile self-review workflow](self-review.md).
