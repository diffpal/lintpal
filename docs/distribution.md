# Distribution

Omnidist builds the same `./cmd/lintpal` executable for Linux amd64/arm64,
macOS amd64/arm64, and Windows amd64. The npm meta packages `lintpal` and
`@diffpal/lintpal` both point to one set of five `@diffpal/lintpal-*`
platform packages. The configured version comes from `OMNIDIST_VERSION` and
is embedded in `lintpal version`. The first public lintpal release was
[v0.2.0](https://github.com/diffpal/lintpal/releases/tag/v0.2.0); both npm meta
packages and all five platform packages were published. See
[GitHub Releases](https://github.com/diffpal/lintpal/releases) for the latest version.

Install the current release with `npm install -g lintpal`. To stage the next
release locally from the repository root, with Go, Node, and npm available:

```bash
OMNIDIST_VERSION=0.3.0 task release-stage
OMNIDIST_VERSION=0.3.0 task release-dry-run
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

The [release workflow](../.github/workflows/omnidist-release.yml) is generated
by `omnidist ci` and customized for lintpal's source checks and version source.
Pushing a `v*` tag runs those checks, builds and verifies all npm packages,
publishes with npm trusted publishing, then creates a GitHub Release with the
five binaries and checksums. The [manual stage workflow](../.github/workflows/stage.yml)
stores build and npm artifacts without publishing.

Staging and the dry run do not publish. The [v0.3.0 release runbook](release-v0.3.0.md)
records the exact package and asset set, preflight commands, and public
verification gates. The [v0.2.0 runbook](release-v0.2.0.md) is kept as release
history.
