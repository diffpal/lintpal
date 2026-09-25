# v0.2.0 release runbook

This runbook describes the first lintpal release. Its external commands require
separate approval of the reviewed source revision and artifacts. Local staging
does not complete the release.

The existing `jevlint` v0.1.0 tag and six scoped npm packages are immutable
history. npm rejected the unscoped `jevlint` name as too similar to `jev-lint`;
there is no v0.1.0 GitHub release. Do not retag or unpublish those artifacts.

## Fixed release contract

- GitHub repository, tag, and release: `diffpal/lintpal`, `v0.2.0`.
- License: MIT, Copyright (c) 2026 Alexey Samoylov.
- npm meta packages: `lintpal@0.2.0` and `@diffpal/lintpal@0.2.0`.
- Shared npm platform packages: `@diffpal/lintpal-darwin-x64`,
  `@diffpal/lintpal-darwin-arm64`, `@diffpal/lintpal-linux-x64`,
  `@diffpal/lintpal-linux-arm64`, and `@diffpal/lintpal-win32-x64`, all at
  `0.2.0`.
- GitHub assets: `lintpal-darwin-amd64`, `lintpal-darwin-arm64`,
  `lintpal-linux-amd64`, `lintpal-linux-arm64`,
  `lintpal-windows-amd64.exe`, and `checksums.txt`.

## Local preflight

Run from a clean, reviewed source revision:

```bash
export OMNIDIST_VERSION=0.2.0
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
must also pass the hosted [test matrix](../.github/workflows/ci.yml) before any
registry publication. The [stage workflow](../.github/workflows/stage.yml)
can store additional review artifacts; it has no publication step.

## External actions, after exact approval

Push the reviewed rebrand commit to the existing `diffpal/jevlint` repository
and wait for hosted CI. Rebuild from that clean commit and compare the final
checksums. Stop if the repository or hosted CI is unavailable.

Only after a second approval naming the final commit and artifact hashes may
the release operator publish. Because npm's name-similarity check runs only at
upload, publish the staged unscoped `lintpal@0.2.0` package first under the
`next` dist-tag. If npm rejects the name, stop without publishing new scoped
packages or renaming the repository. Once accepted, rename the repository to
`diffpal/lintpal`, push `v0.2.0`, publish the five staged platform packages and
the scoped meta package, and move `lintpal@0.2.0` to `latest`. npm requires
interactive browser authorization for this account, so publish each staged
package with an interactive `npm publish --access public --auth-type=web` call.
Use `gh release create --verify-tag` with all six assets. Registry uploads,
repository rename, and pushed tags are durable; record partial success and
stop for a repair plan if any step fails.

## Public verification

After publication, inspect registry metadata for all seven names and compare
both meta-package optional dependency maps, versions, and `bin` entries. In
two isolated directories with a fresh npm cache, install each public meta
package and run `lintpal version` and `lintpal --help`; compare outputs and the
installed native platform payload. Download the six GitHub assets, run
`sha256sum -c checksums.txt`, compare the checksum file to the approved one,
and confirm the release tag points to the approved source SHA. Record public
links and these results in Story `.11`. Close the Story and Epic only after
both npm identities and the GitHub release pass.
