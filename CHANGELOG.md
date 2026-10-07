# Changelog

The [GitHub Releases](https://github.com/diffpal/lintpal/releases) page shows
which versions have been published.

## [0.8.0] - 2026-10-07

### Added

- Native support for OpenAI's official Decisions API request and response
  schema, including predicate, choice, score, refusal, and usage handling.
- Provider-aware request sizing so local batching limits use the selected wire
  format exactly.

### Changed

- OpenAI examples use `gpt-6-luna` and document the API's preview access.
- Review cost for `gpt-6-luna` uses reported Standard short-context input,
  cached-input, cache-write, and output token categories. Unknown OpenAI models
  retain an unknown cost unless the provider reports one.

### Fixed

- The `openai` preset now sends the official ordered question array instead of
  the shared TypeSafe/OpenRouter question-map shape.
- Malformed, refused, duplicate, unknown, missing, unnamed, or mismatched
  OpenAI answers fail the complete review without partial findings.

### Known limitations

- OpenAI Decisions API is in limited preview and requires an explicit model.
- LintPal currently sends text input only and supports string-valued choices.

## [0.7.0] - 2026-10-04

### Added

- Successful findings artifacts include provider request count, review
  duration, and optional USD cost in `stats.review`.
- Markdown and GitHub feedback render the stored review summary without another
  provider request.

### Changed

- LintPal owns the findings v5 schema while retaining DiffPal v5 as a design
  and compatibility baseline.

## [0.6.1] - 2026-09-30

This patch updates documentation and npm package copy. CLI runtime behavior
is unchanged from 0.6.0.

### Changed

- README and npm descriptions use "Turn your engineering rules into
  pull-request checks." without implying an English-language requirement.
- The README explains provider evaluation, optional GitHub feedback,
  comment deduplication, local setup and the rule-to-finding path.
- README and documentation link to the live Q&A and Ideas Discussions categories.

### Fixed

- README rule imports use explicit `general` and `go` prefixes, so the
  documented `rule view general/authorization.md` command resolves correctly.
- Provider setup examples identify the published 0.6.0 capability baseline.

### Removed

- Custom Discussion forms. Discussions uses GitHub's standard composer;
  bugs and incorrect findings use GitHub Issues.

## [0.6.0] - 2026-09-30

### Added

- An OpenAI provider preset for an assumed compatible Decisions API, requiring
  an explicit model. Live API availability has not been verified.
- Configurable custom provider paths through `--api-path` and `LINTPAL_API_PATH`,
  retaining `/v1/systemone` when no path is configured.

### Changed

- The README quickstart installs the CLI globally and runs lint and GitHub
  feedback commands directly; all four provider options have setup examples.
- LintPal uses the DiffPal family mascot with a white sticker outline for
  visibility on dark backgrounds, without a text label.
- Release tooling uses Omnidist 0.1.40 and its corrected checksum command.
- npm descriptions use the product tagline.

### Fixed

- OpenRouter now uses `/api/alpha/decisions` and the provider-specific default
  model `typesafe/jev-1.13`.
- CLI argument validation and Git resource limits match their documented
  contracts; rule imports and Git comparison documentation describe actual
  behavior.

### Known limitations

- OpenAI support assumes a compatible `/v1/decisions` service; neither its live
  endpoint nor a model identifier has been verified.
- Remote providers receive bounded source context and rule text. Findings are
  not a model-accuracy guarantee; see [privacy](docs/architecture/privacy.md).
- Product Hunt community, gallery and draft readiness remain separate launch
  gates; account readiness has been confirmed by the owner.

## [0.5.4] - 2026-09-29

### Added

- GitHub Discussion forms for Q&A, ideas, and rule feedback, once Discussions
  and the matching categories are enabled.
- A fake-provider process test for a committed violation, blocking gate, fix,
  and clean rerun.

### Changed

- Corrected the README Action example to pin 0.5.4 and
  clarified provider data flow, rule-to-finding behavior, and feedback.

### Fixed

- No CLI runtime fixes are included in this release.

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
