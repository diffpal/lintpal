# v0.3.0 release runbook

This release includes `.env` loading, pinned local rule packs and lockfiles,
self-review, Taskfile tooling, rule examples, and the reorganized documentation.
The existing v0.2.0 GitHub release and seven npm packages remain available.

## Contract

- Source repository and GitHub release: `diffpal/lintpal`, tag `v0.3.0`.
- npm meta packages: `lintpal@0.3.0` and `@diffpal/lintpal@0.3.0`.
- Platform packages: `@diffpal/lintpal-darwin-x64`,
  `@diffpal/lintpal-darwin-arm64`, `@diffpal/lintpal-linux-x64`,
  `@diffpal/lintpal-linux-arm64`, and `@diffpal/lintpal-win32-x64`, each at
  `0.3.0`.
- GitHub assets: five native binaries and `checksums.txt`, as prepared by
  `scripts/prepare-release-assets.sh`.

## Preflight

From the reviewed source revision, run:

```bash
task check
task race
task lint-go
task security
go mod verify
bd lint
```

When the source commit is clean and the hosted Test, Lint, and Security
workflows pass, create a local tag and stage the exact release:

```bash
git tag -a v0.3.0 -m 'lintpal v0.3.0'
task release-stage
task release-dry-run
```

Review the seven staged npm manifests and tarball contents in
`.omnidist/default/npm`, the five binaries and checksum file in
`.omnidist/default/release-assets`, the embedded `lintpal version`, and the
Omnidist publication plan. The staged Linux amd64 package must install and
run under both npm meta-package names. The other four targets are checked as
build artifacts and package payloads.

Omnidist requires that `HEAD` be exactly at that tag. Record the commit SHA,
Omnidist version, final asset hashes, and
package manifests together. The manual stage workflow can preserve review
artifacts after the tag is pushed.

## Publication

The [Omnidist release workflow](../.github/workflows/omnidist-release.yml)
publishes on a pushed `v*` tag. Before its first run, configure npm trusted
publishing for all seven packages, with repository `diffpal/lintpal` and
workflow file `omnidist-release.yml`. Print the exact setup commands with:

```bash
npx -y @omnidist/omnidist@latest npm trust
```

After reviewing them, the package owner can run `npm trust --apply` through
Omnidist. The workflow requests `id-token: write` only for npm publication;
it does not need an `NPM_PUBLISH_TOKEN` secret. It checks out the complete
tagged history; Omnidist reads `0.3.0` directly from `v0.3.0`. The workflow
runs the source checks, builds, stages, verifies, and dry-runs publication
before Omnidist uploads the five platform packages
and both meta packages. GitHub Release creation waits for npm publication and
uploads the five binaries and checksums. See the
[upstream release runbook](https://github.com/metalagman/omnidist/blob/master/docs/releases.md)
for partial-publication recovery.

Pushing the `v0.3.0` tag changes external registries, so approve the exact
source commit and staged artifacts first. Push the reviewed local tag once
the trusted publishers are configured. If a package fails, stop
and inspect which versions reached npm before retrying.

After publication, inspect versions and dist-tags for all seven npm names.
Install each public meta package in a fresh directory and run `lintpal
version` and `lintpal --help`. Download GitHub assets, verify
`sha256sum -c checksums.txt`, and check the tag resolves to the approved
commit. Record public links and verification results.
