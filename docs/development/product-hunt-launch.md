# Product Hunt launch draft for LintPal

This is a reviewable draft for the launch owner. It does not enable GitHub
Discussions or create a Product Hunt post. The current published release is
[v0.5.4](https://github.com/diffpal/lintpal/releases/tag/v0.5.4). The
[README](../../README.md) and [privacy guide](../architecture/privacy.md)
describe the product and data flow. LintPal is part of the
[DiffPal family](https://github.com/diffpal/diffpal); the two tools have
separate CLIs and share the findings v5 schema.

## Product Hunt fields

| Field | Draft |
| --- | --- |
| Name | LintPal |
| Product URL | [LintPal website](https://lintpal.metalagman.dev) |
| Tagline | Turn plain-English engineering rules into pull-request checks |
| Description | LintPal checks committed pull-request changes against Markdown rules owned by your repo. Get findings on changed lines, then gate CI by severity. Use your selected provider for rule evaluation. |

The description is under Product Hunt's 260-character limit. The launch owner
should select the most relevant available topics in the Product Hunt editor;
topic names can change. [Product Hunt's posting guide](https://help.producthunt.com/en/articles/479557-how-to-post-a-product)
describes the current fields and media sizes.

## Launch decision and evidence

Snapshot from 2026-09-30. **The public Product Hunt launch is NO-GO.**
The published release is still **0.5.4**; **0.6.0 is an unpublished candidate**
for the changes now on `main`. Community, gallery and private draft readiness remain unverified. The owner confirmed their personal
Product Hunt account is ready on 2026-09-30. Screenshot work and browser use remain excluded
by operator direction.

| Gate | Current evidence and status | Owner / exit condition |
| --- | --- | --- |
| Source | `main` at `88eacdf8cb8a04165b89c998caf1e1b16c1bad0c` has passed CI, lint and security. Provider support, runtime fixes, global CLI quickstart and the mascot sticker landed after 0.5.4. The release-refresh branch has passed local package preparation checks. **Current main passed; candidate merge pending.** | Review and merge the candidate, then verify its exact CI commit. |
| Package | GitHub latest release and all seven npm latest identities remain 0.5.4. The 0.5.4 release record below is historical; it does not verify newer CLI flags or README assets. Registry and local-tag checks found 0.6.0 unused. **Local 0.6.0 staging passed; public release pending.** | Review the candidate and authorize the tag push; verify workflow, seven public identities and fresh installs. Never republish an existing version. |
| Community | API reports `has_discussions=false`, no categories and no threads. Three forms are on the default branch; Issues is live. **Blocked.** | Enable Discussions, confirm `q-a`, `ideas`, `rule-feedback` categories and usable forms, then publish the welcome thread after explicit authorization. |
| Page | The [mascot sticker](../assets/lintpal-thumbnail.png) is prepared. Two gallery images are absent; The owner confirmed personal account readiness; private draft/page and gallery readiness remain unverified. **Blocked.** | Owner supplies the required gallery/page evidence within permitted scope. Screenshot/browser restrictions remain in force; the mascot does not satisfy the gallery gate. |

The [candidate changelog](../../CHANGELOG.md) distinguishes the new behavior
from the historical 0.5.4 release. OpenRouter's corrected route, custom API
paths and the OpenAI preset are source changes, not additions to the already
published 0.5.4 package. OpenAI support explicitly assumes a compatible
Decisions API; live availability is unverified. Launch copy must describe the
final published version, and must not imply a verified OpenAI integration.

### Local 0.6.0 candidate verification

An isolated temporary source snapshot with a local-only `v0.6.0` tag passed
`task release-stage` and `task release-dry-run`. All seven staged identities
use 0.6.0 and the current product description. Both npm meta-package READMEs
contain the current supported-provider section and custom API path examples.
Fresh offline installs of both `lintpal` and `@diffpal/lintpal` reported
`lintpal 0.6.0` and working help. All five release asset checksums passed;
Go metadata for each target reports `CGO_ENABLED=0`. Only Linux x64 binaries
were executed locally; the other targets were verified as cross-built files.

The configured Git-tag version source does not accept an environment version
override as a replacement for the tag. The isolated tag is not in the public
repository and has not been pushed. No registry publication occurred: the
public package version is still 0.5.4. After merging the reviewed candidate,
repeat the exact-tag preparation on the real release commit before a tag push.

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
> We'd value that feedback in GitHub Discussions once the forum is enabled.

The owner should review the first-person wording before posting. Invite
feedback and discussion; do not ask for votes. See [Product Hunt's sharing
guidance](https://help.producthunt.com/en/articles/2690626-how-do-i-share-my-post).

## Brand asset and media gate

The [LintPal thumbnail](../assets/lintpal-thumbnail.png) uses the existing
[DiffPal mark](https://github.com/diffpal/diffpal.github.io/blob/main/public/logo-mark.png).
The thumbnail adds a white sticker outline and transparent background for
visibility on dark themes, without a text label. The original source mascot
remains in [diffpal-mark.png](../assets/diffpal-mark.png).
This identifies LintPal as part of the DiffPal family; it does not show product
output. No gallery images are supplied. The operator set all screenshot and
browser-use work to NO-GO, so the page gate remains blocked. The thumbnail does
not substitute for gallery media or private account/draft confirmation.

## GitHub Discussions setup

GitHub reported Discussions disabled for `diffpal/lintpal` on 2026-09-30,
with no categories or threads. The
three forms are on the default branch, but the owner must enable Discussions and
create or confirm categories with these exact slugs before they become usable:

| Category | Format | Form |
| --- | --- | --- |
| Q&A (`q-a`) | Question and answer | [q-a.yml](../../.github/DISCUSSION_TEMPLATE/q-a.yml) |
| Ideas (`ideas`) | Open discussion | [ideas.yml](../../.github/DISCUSSION_TEMPLATE/ideas.yml) |
| Rule feedback (`rule-feedback`) | Open discussion | [rule-feedback.yml](../../.github/DISCUSSION_TEMPLATE/rule-feedback.yml) |

Announcements and Show and tell are optional additional categories. GitHub
requires each [form filename to match the category slug](https://docs.github.com/en/discussions/managing-discussions-for-your-community/creating-discussion-category-forms).
If GitHub assigns another slug, rename the corresponding form in a reviewed
change. The forms already live on the default branch.

### Welcome discussion draft

Title: **Welcome to LintPal — what engineering rule should your repo enforce?**

> LintPal checks committed changes against the Markdown rules your repository
> owns. Start with one rule your team already repeats in reviews. Was it easy
> to express, and did the resulting finding match your intent?
>
> Use Q&A for installation and provider questions, Ideas for workflow
> suggestions, and Rule feedback for false positives, false negatives, or rules
> that were hard to write. Please share only sanitized examples; do not post
> credentials or proprietary source.

## Remaining owner gates

1. Review the 0.6.0 release candidate and its staging evidence, then explicitly
   authorize the release tag push. Verify the existing Omnidist workflow and
   all seven public package identities before presenting new features as
   installable. The candidate does not itself authorize publication.
2. The owner enables Discussions, confirms the `q-a`, `ideas`, and
   `rule-feedback` category slugs and live forms, and posts the welcome
   discussion. Until then, keep the working [Issues route](https://github.com/diffpal/lintpal/issues/new)
   in the README.
3. The owner confirmed personal account readiness on 2026-09-30; Product Hunt
   gallery and private draft/page readiness remain unverified.
   Screenshot and browser work are excluded by the operator's 2026-09-29
   direction. Do not infer readiness from the logo thumbnail or this draft.
4. If the operator later reopens these gates, repeat the public source,
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
