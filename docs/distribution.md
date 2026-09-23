# Local distribution staging

Omnidist builds the same `./cmd/lintpal` executable for Linux amd64/arm64,
macOS amd64/arm64, and Windows amd64. The npm meta packages `lintpal` and
`@diffpal/lintpal` both point to one set of five `@diffpal/lintpal-*`
platform packages. The configured version comes from `OMNIDIST_VERSION` and
is embedded in `lintpal version`. The first lintpal release is planned as
`v0.2.0` under MIT with Alexey Samoylov as copyright holder. Registry package
publication has not yet been verified; the example below stages locally. The
earlier `jevlint` v0.1.0 tag and six scoped npm packages are historical and
will not be overwritten.

From the repository root, with Go, Node, and npm available:

```bash
export OMNIDIST_VERSION=0.2.0-dev.1
npx -y @omnidist/omnidist@latest build
npx -y @omnidist/omnidist@latest stage
npx -y @omnidist/omnidist@latest verify
bash scripts/verify-staging.sh
```

The script compares the two meta-package manifests, checks all five platform
package names and versions, prints SHA-256 hashes for the five target binaries,
and compares the Linux amd64 staged payload with its build artifact. On a Linux
amd64 host it packs the two meta packages and the local platform package,
installs each identity into a separate temporary directory with npm offline,
then compares `version` and `--help` output. It uses a temporary npm cache and
does not contact a registry. The local install smoke requires Linux amd64;
cross-built macOS, Windows, and Linux arm64 binaries are staged and verified
as files, but cannot be executed on that host.

The manual [stage workflow](../.github/workflows/stage.yml) accepts an explicit
SemVer input, pins Omnidist 0.1.37, and stores build and npm artifacts for
review. It has read-only repository permission and no publication step. The
regular [CI workflow](../.github/workflows/ci.yml) remains the offline
build/test gate. Generated Omnidist release workflows include publication jobs
and are not used here.

Staging does not establish availability of `lintpal` or `@diffpal/lintpal` in
npm. The release Story will verify registry rights, publish both names and a
matching GitHub release after separate authorization, then test fresh public
installs. Until those checks pass, the Epic remains open.

The [v0.2.0 release runbook](release-v0.2.0.md) records the exact package and
asset set, preflight commands, and public verification gates.
