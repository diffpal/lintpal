# Product Hunt launch draft for LintPal

This is a reviewable draft for the launch owner. It does not enable GitHub
Discussions or create a Product Hunt post. The current published release is
[v0.6.1](https://github.com/diffpal/lintpal/releases/tag/v0.6.1). The
[README](../../README.md) and [privacy guide](../architecture/privacy.md)
describe the product and data flow. LintPal is part of the
[DiffPal family](https://github.com/diffpal/diffpal); the two tools have
separate CLIs and share the findings v5 schema.

## Product Hunt fields

| Field | Draft |
| --- | --- |
| Name | LintPal |
| Product URL | [LintPal website](https://lintpal.metalagman.dev) |
| Tagline | Turn your engineering rules into pull-request checks |
| Description | LintPal checks committed pull-request changes against Markdown rules owned by your repo. Get findings on changed lines, then gate CI by severity. Use your selected provider for rule evaluation. |

The description is under Product Hunt's 260-character limit. The launch owner
should select the most relevant available topics in the Product Hunt editor;
topic names can change. [Product Hunt's posting guide](https://help.producthunt.com/en/articles/479557-how-to-post-a-product)
describes the current fields and media sizes.

## Launch decision and evidence

Snapshot from 2026-09-30. **The public Product Hunt launch is NO-GO.**
The published release is **0.6.1** on GitHub and all seven npm identities.
Both npm meta-packages passed fresh installation checks.
Gallery and private draft readiness remain unverified. The owner confirmed their personal
Product Hunt account is ready on 2026-09-30. Screenshot work and browser use remain excluded
by operator direction.

| Gate | Current evidence and status | Owner / exit condition |
| --- | --- | --- |
| Source | `v0.6.1` points to `3b349c08c5c52bef2f5d1b084a8ae4be60875d3f`, which passed CI, lint and security. The release workflow also passed its source checks before publication. **Passed.** | Keep launch claims tied to this published revision. |
| Package | [Release workflow](https://github.com/diffpal/lintpal/actions/runs/36711277622) passed and created GitHub Release 0.6.1. Five downloaded asset checksums, CGO-disabled metadata, version and release commit passed. Both fresh npm installs, corrected public package copy and all seven registry identities passed as 0.6.1. **Passed.** | Keep the launch tied to these verified packages. Never republish an existing version. |
| Community | Discussions is enabled with GitHub's standard categories and composer. The [welcome thread](https://github.com/diffpal/lintpal/discussions/18) is public; README links to Q&A and Ideas. Bugs and incorrect findings use Issues. **Passed.** | Keep the forum and feedback links accessible. |
| Page | The [mascot sticker](../assets/lintpal-thumbnail.png) is prepared. Two gallery images are absent. The owner confirmed personal account readiness; private draft/page and gallery readiness remain unverified. **Blocked.** | Owner supplies the required gallery/page evidence within permitted scope. Screenshot/browser restrictions remain in force; the mascot does not satisfy the gallery gate. |

The [changelog](../../CHANGELOG.md) distinguishes the new behavior
from the historical 0.5.4 release. OpenRouter's corrected route, custom API
paths and the OpenAI preset are source changes, not additions to the already
published 0.5.4 package. OpenAI support explicitly assumes a compatible
Decisions API; live availability is unverified. Launch copy must describe the
final published version, and must not imply a verified OpenAI integration.

### Published 0.6.1 verification

The owner authorized the documentation and package-copy patch release on
2026-09-30. Preparation on the real `v0.6.1` tag passed staging, offline installs
of both meta-packages and five asset checksums. All five targets reported
`CGO_ENABLED=0`, version 0.6.1 and the exact release commit. The publication
workflow passed and created the GitHub Release on 2026-09-30. All five downloaded
public assets passed checksum and build-metadata verification; the Linux x64
binary ran successfully.

Fresh registry installs of both `lintpal` and `@diffpal/lintpal` reported
`lintpal 0.6.1` and working provider help, including `--api-path`. Both public
package READMEs contain the corrected product description, supported providers,
website link and rule-import prefixes. Their manifests share the same five
exact-version platform dependencies. All seven registry identities report
version and `latest` as 0.6.1. Only Linux x64 binaries were executed locally;
the other targets were inspected as cross-built files. CLI runtime behavior
is unchanged from 0.6.0.

### Historical 0.6.0 verification

Before publication, an isolated source snapshot passed `task release-stage`
and `task release-dry-run`; preparation on the real `v0.6.0` tag also passed.
The owner then authorized the tag push. The release workflow completed npm
publication and created the GitHub Release on 2026-09-30.

Fresh registry installs with separate caches of both `lintpal` and
`@diffpal/lintpal` reported `lintpal 0.6.0` and working help, including
`--api-path`. Both public package READMEs contain the current supported-provider
section and website link; their manifests share the same five exact-version
platform dependencies and product description. All seven registry identities
were checked after npm finished processing their publication; each reports
version 0.6.0 and `latest=0.6.0`.

All five downloaded GitHub assets passed their published checksums. Go metadata
for each reports `CGO_ENABLED=0`, version 0.6.0 and the exact release commit.
Only Linux x64 binaries were executed locally; the other targets were inspected
as cross-built files. The configured Git-tag version source requires the tag;
an environment version override does not replace it.

## Maker's first comment draft

> Hi Product Hunt! We built LintPal for engineering rules that teams repeat in
> pull-request reviews. Write each requirement as a Markdown file in the repo,
> review rule changes like code, and check committed changes against those rules.
>
> LintPal evaluates the explicit rules you supply through a selected provider.
> It returns findings on changed lines and applies a severity-based CI gate. It
> does not invent a new review policy for each pull request. With a remote
> provider, bounded source context and rule text go to that provider.
>
> Try one rule your team already repeats. Was it easy to express? Did the
> finding match your intent? Where did it produce too much or too little signal?
> We'd value that feedback in [GitHub Discussions](https://github.com/diffpal/lintpal/discussions).
> Use [Issues](https://github.com/diffpal/lintpal/issues/new) for bugs and incorrect findings.

The owner should review the first-person wording before posting. Invite
feedback and discussion; do not ask for votes. See [Product Hunt's sharing
guidance](https://help.producthunt.com/en/articles/2690626-how-do-i-share-my-post).

## Brand asset and media gate

The [LintPal thumbnail](../assets/lintpal-thumbnail.png) uses the existing
[DiffPal mark](https://github.com/diffpal/diffpal.github.io/blob/main/public/logo-mark.png).
The thumbnail uses grayscale shading, a white sticker outline and a transparent
background for visibility on dark themes, without a text label. The original
source mascot remains in [diffpal-mark.png](../assets/diffpal-mark.png).
This identifies LintPal as part of the DiffPal family; it does not show product
output. No gallery images are supplied. The operator set all screenshot and
browser-use work to NO-GO, so the page gate remains blocked. The thumbnail does
not substitute for gallery media or private account/draft confirmation.

## GitHub Discussions

Discussions was enabled with the owner's authorization on 2026-09-30.
The API confirms `q-a` (question and answer) and `ideas` (open discussion).
Use GitHub's standard discussion composer. Ask setup questions in
[Q&A](https://github.com/diffpal/lintpal/discussions/categories/q-a) and suggest
workflow improvements in [Ideas](https://github.com/diffpal/lintpal/discussions/categories/ideas).
Bugs and incorrect findings go to [Issues](https://github.com/diffpal/lintpal/issues/new).

### Published welcome discussion

The owner authorized the [welcome thread](https://github.com/diffpal/lintpal/discussions/18),
published in Announcements on 2026-09-30. The text uses Issues for bugs and
incorrect findings:

Title: **Welcome to LintPal — what engineering rule should your repo enforce?**

> LintPal checks committed changes against the Markdown rules your repository
> owns. Start with one rule your team already repeats in reviews. Was it easy
> to express, and did the resulting finding match your intent?
>
> Get started at [lintpal.metalagman.dev](https://lintpal.metalagman.dev), or
> install the CLI with `npm install -g lintpal`.
>
> Use [Q&A](https://github.com/diffpal/lintpal/discussions/categories/q-a) for
> installation and provider questions, and
> [Ideas](https://github.com/diffpal/lintpal/discussions/categories/ideas) for
> workflow suggestions. For false positives, false negatives, or rules that were
> hard to write, use [GitHub Issues](https://github.com/diffpal/lintpal/issues/new).
>
> Please share only sanitized examples; do not post credentials or proprietary source.

## Remaining owner gates

1. The owner confirmed personal account readiness on 2026-09-30; Product Hunt
   gallery and private draft/page readiness remain unverified.
   Screenshot and browser work are excluded by the operator's 2026-09-29
   direction. Do not infer readiness from the logo thumbnail or this draft.
2. If the operator later reopens these gates, repeat the public source,
   package, feedback, and page audit before changing the Story from NO-GO to GO.
   Posting to Product Hunt remains an owner action.

## v0.5.4 release record

The annotated `v0.5.4` tag points to `7fa6025d87bd6172862f2916e50f0c3d8fcb08ae`.
The [release workflow](https://github.com/diffpal/lintpal/actions/runs/36464775409)
completed successfully and published a GitHub Release plus the two npm meta
packages and five platform packages. The published `lintpal` and
`@diffpal/lintpal` READMEs both pin Action version `0.5.4`. Fresh registry
installs of each meta package on Linux x64 reported `lintpal 0.5.4` and working
help. Go build metadata for all five locally staged targets, and for the
downloaded public Linux x64 asset, reported `CGO_ENABLED=0`.

Subsequent work includes CLI changes as well as branding and documentation.
Do not retag or republish `v0.5.4`; use a new verified release for the current
product launch.
