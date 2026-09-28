# Changelog

The [GitHub Releases](https://github.com/diffpal/lintpal/releases) page contains
the published release history. Version 0.5.4 below is a candidate; it has not
been tagged or published.

## [0.5.4] - Unreleased

### Added

- GitHub Discussion forms for Q&A, ideas, and rule feedback, once Discussions
  and the matching categories are enabled.
- A fake-provider process test for a committed violation, blocking gate, fix,
  and clean rerun.

### Changed

- Corrected the README Action example to pin the planned 0.5.4 release and
  clarified provider data flow, rule-to-finding behavior, and feedback.

### Fixed

- No CLI runtime fixes are included in this candidate.

### Known limitations

- The selected remote provider receives bounded source context and rule text;
  findings do not guarantee model accuracy. See [privacy](docs/architecture/privacy.md).
- The CLI does not apply fixes or emit SARIF. See the [report reference](docs/reference/report.md).

## [0.5.3] - 2026-09-28

### Added

- No CLI features were added in this patch release.

### Changed

- The generated npm package description was updated. The change from
  [v0.5.2 to v0.5.3](https://github.com/diffpal/lintpal/compare/v0.5.2...v0.5.3)
  affects `.omnidist/omnidist.yaml` only.

### Fixed

- No runtime fixes were included in this patch release.

### Known limitations

- Rule evaluation uses the selected provider; findings are not a guarantee of
  model accuracy. A selected remote provider receives bounded source context
  and rule text. See [privacy](docs/architecture/privacy.md).
- The CLI does not apply fixes or emit SARIF. See the [report reference](docs/reference/report.md).
