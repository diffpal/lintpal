# v0.1.0 release runbook

This runbook describes the first public release. Its external commands require
separate approval of the reviewed source revision and artifacts. Local staging
does not complete the release.

## Fixed release contract

- GitHub repository, tag, and release: `diffpal/jevlint`, `v0.1.0`.
- License: MIT, Copyright (c) 2026 Alexey Samoylov.
- npm meta packages: `jevlint@0.1.0` and `@diffpal/jevlint@0.1.0`.
- Shared npm platform packages: `@diffpal/jevlint-darwin-x64`,
  `@diffpal/jevlint-darwin-arm64`, `@diffpal/jevlint-linux-x64`,
  `@diffpal/jevlint-linux-arm64`, and `@diffpal/jevlint-win32-x64`, all at
  `0.1.0`.
- GitHub assets: `jevlint-darwin-amd64`, `jevlint-darwin-arm64`,
  `jevlint-linux-amd64`, `jevlint-linux-arm64`,
  `jevlint-windows-amd64.exe`, and `checksums.txt`.

## Local preflight

Run from a clean, reviewed source revision:

```bash
export OMNIDIST_VERSION=0.1.0
npx -y @omnidist/omnidist@latest build
npx -y @omnidist/omnidist@latest stage
npx -y @omnidist/omnidist@latest verify
bash scripts/verify-staging.sh
bash scripts/prepare-release-assets.sh
npx -y @omnidist/omnidist@latest publish --dry-run
```

The last command is a dry run; it does not publish packages. Review its
platform-first package list and tarball contents. The asset script compares
each staged platform payload with the corresponding build binary, then writes
and verifies `.omnidist/default/release-assets/checksums.txt`. Go currently
embeds VCS metadata in binaries. **Rebuild after the release source commit**
and use those final hashes for approval and publication; pre-commit candidate
hashes are not final release evidence. Record the source SHA, Omnidist version,
the seven package manifests, final six assets, checksums, and native `version`
output together.

The required local gates are `go build ./...`, `go test ./...`, `go vet ./...`,
`go test -race ./...`, format checks, and `bd lint`. The pushed source revision
must also pass the hosted [CI matrix](../.github/workflows/ci.yml) before any
registry publication. The [stage workflow](../.github/workflows/stage.yml)
can store additional review artifacts; it has no publication step.

## External actions, after exact approval

The source step needs explicit authorization to create `diffpal/jevlint` and
push the reviewed source commit. Rebuild from that clean commit and compare the
new final checksums. Stop if the GitHub repository or hosted CI is unavailable.

Only after a second approval naming the final commit and artifact hashes may
the release operator push `v0.1.0`, run Omnidist npm publish, and create the
GitHub release. Omnidist publishes the five platform packages first, then the
primary and alias meta packages. `gh release create` must use `--verify-tag`
and attach all six assets so it cannot silently tag a different commit.
Registry uploads and pushed tags are durable; record partial success and stop
for a repair plan if any step fails.

## Public verification

After publication, inspect registry metadata for all seven names and compare
both meta-package optional dependency maps, versions, and `bin` entries. In
two isolated directories with a fresh npm cache, install each public meta
package and run `jevlint version` and `jevlint --help`; compare outputs and the
installed native platform payload. Download the six GitHub assets, run
`sha256sum -c checksums.txt`, compare the checksum file to the approved one,
and confirm the release tag points to the approved source SHA. Record public
links and these results in Story `.11`. Close the Story and Epic only after
both npm identities and the GitHub release pass.
