# lintpal

`lintpal` reviews a **committed Git comparison** with declarative rules and a
selected System One provider. It prints a deterministic report of diagnostics
anchored to changed lines. The current MVP reads committed blobs only; save and
commit the revision you want to inspect before running it.

The first lintpal release is planned as `v0.2.0` under the [MIT license](LICENSE).
Once publication is verified, users will be able to install either npm identity:

```bash
npm install -g lintpal@0.2.0
# or
npm install -g @diffpal/lintpal@0.2.0
lintpal version
```

These registry commands become available only after the release Story finishes;
the local build below works now.

The earlier `jevlint` v0.1.0 tag and six scoped npm packages remain available
for existing installs. npm rejected the unscoped `jevlint` name, so that release
did not receive a GitHub release.

## Build and try locally

Go 1.26.6 or newer is required by `go.mod`. From the repository root:

```bash
go build -o ./lintpal ./cmd/lintpal
./lintpal version
./lintpal --help
go test ./cmd/lintpal -run TestProcessExitAndStreams -count=1
```

The process test creates temporary committed Git repositories and a local fake
System One server, so the last command needs no provider secret or network
account. A plain local build reports version `dev`. Omnidist staging can embed
an explicit version; see [distribution staging](docs/distribution.md).

To lint a real comparison, run these commands **inside the repository being
reviewed** after both revisions are committed:

```bash
export TYPESAFE_API_KEY='your-token'
./lintpal doctor --provider jev
mkdir -p .artifacts/lintpal
./lintpal lint --base origin/main --head HEAD --provider jev \
  --format json --fail-on high --out .artifacts/lintpal/report.json
```

Use an absolute path to the built executable when linting another repository.
The default provider is `jev`; `openrouter` reads `OPENROUTER_API_KEY`, and
`custom` reads `LINTPAL_TOKEN` by default and requires `--base-url`. `doctor`
checks the local Git repository and presence of the selected credential; it
does not contact the provider. A real `lint` sends bounded committed source
context and rule questions to the selected provider. See the [CLI guide](docs/cli.md)
for flags, environment settings, output formats, and exit codes. Exit code `10`
means the complete report was written and a finding met `--fail-on`; it can be
used as a CI gate.

## CI and reports

The checked-in [CI workflow](.github/workflows/ci.yml) builds and tests on
Linux, macOS, and Windows without provider credentials. For a repository using
`lintpal` as a quality gate, install a locally built or separately approved
binary in the job, fetch the base revision, provide the selected credential as
a CI secret, then run the `lint` command above with `--base` and `--head` set to
commits available in that checkout. Handle exit code `10` as a lint failure;
other nonzero codes indicate setup, provider, or export failure. Never print
the credential or enable shell tracing around it.

The [report reference](docs/report.md) defines the versioned JSON and human
formats. [Privacy and limitations](docs/privacy.md) explains source transfer,
local metrics, bounded inputs, and the current quality evidence. The
[evaluation guide](docs/evaluation.md) describes the offline corpus and an
optional, explicit live procedure.
