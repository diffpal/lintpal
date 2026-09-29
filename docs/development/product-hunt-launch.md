# Product Hunt launch draft for LintPal

This is a reviewable draft for the launch owner. It does not enable GitHub
Discussions or create a Product Hunt post. The current published release is
[v0.5.4](https://github.com/diffpal/lintpal/releases/tag/v0.5.4). The
[README](../../README.md) and [privacy guide](../architecture/privacy.md)
describe the product and data flow. LintPal is part of the
[DiffPal family](https://github.com/diffpal/diffpal); the two tools have
separate CLIs and share the findings v5 report format.

## Product Hunt fields

| Field | Draft |
| --- | --- |
| Name | LintPal |
| Product URL | `https://github.com/diffpal/lintpal` |
| Tagline | Turn plain-English engineering rules into pull-request checks |
| Description | LintPal checks committed pull-request changes against Markdown rules owned by your repo. Get findings on changed lines, then gate CI by severity. Use your selected provider for rule evaluation. |

The description is under Product Hunt's 260-character limit. The launch owner
should select the most relevant available topics in the Product Hunt editor;
topic names can change. [Product Hunt's posting guide](https://help.producthunt.com/en/articles/479557-how-to-post-a-product)
describes the current fields and media sizes.

## Launch decision and evidence

Snapshot from 2026-09-29. **The public Product Hunt launch is NO-GO.** The
release gate passed; community and page readiness have not passed. The operator
explicitly excluded screenshot work and browser use from the current scope.

| Gate | Current evidence and status | Owner / exit condition |
| --- | --- | --- |
| Source | [PR #5](https://github.com/diffpal/lintpal/pull/5) merged as `7fa6025`; CI, lint, and security checks passed on that commit. Later family-branding documentation does not change the tagged CLI. **Release source passed.** | Keep later documentation changes under normal review and CI. |
| Package | The [v0.5.4 workflow](https://github.com/diffpal/lintpal/actions/runs/36464775409) passed. All seven npm identities, both published README pins, and fresh Linux x64 installs of both meta packages reported `0.5.4`. All five staged targets and the public Linux asset showed `CGO_ENABLED=0`. **Passed.** | Recheck public versions before posting; do not republish an existing version. |
| Community | The three forms are on the default branch, but the repository reported Discussions disabled. README keeps the working Issues route. **Blocked.** | Owner enables Discussions, confirms category slugs and forms, and posts the welcome thread. |
| Page | The [family-logo thumbnail](../assets/lintpal-thumbnail.png) is prepared. Gallery media and private Product Hunt account/draft evidence are absent; screenshot and browser work are out of scope by operator direction. **Blocked.** | Keep NO-GO until the operator changes those constraints and the missing gates can be verified. |

The [v0.5.4 changelog](../../CHANGELOG.md) covers the corrected Action pin,
prepared Discussion forms, and a fake-provider committed-revision process test.
There is no new CLI behavior or claimed model-accuracy improvement. Later
branding and cross-links are documentation changes; they do not rewrite the
published package or tag.

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

The [LintPal thumbnail](../assets/lintpal-thumbnail.png) is a 240×240 local
composition of the LintPal name and the existing
[DiffPal mark](https://github.com/diffpal/diffpal.github.io/blob/main/public/logo-mark.png).
The source mark is copied unchanged as [diffpal-mark.png](../assets/diffpal-mark.png).
This identifies LintPal as part of the DiffPal family; it does not show product
output. No gallery images are supplied. The operator set all screenshot and
browser-use work to NO-GO, so the page gate remains blocked. The thumbnail does
not substitute for gallery media or private account/draft confirmation.

## GitHub Discussions setup

GitHub reported Discussions disabled for `diffpal/lintpal` on 2026-09-29. The
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

1. The owner enables Discussions, confirms the `q-a`, `ideas`, and
   `rule-feedback` category slugs and live forms, and posts the welcome
   discussion. Until then, keep the working [Issues route](https://github.com/diffpal/lintpal/issues/new)
   in the README.
2. Product Hunt gallery and private account/draft readiness remain unverified.
   Screenshot and browser work are excluded by the operator's 2026-09-29
   direction. Do not infer readiness from the logo thumbnail or this draft.
3. If the operator later reopens these gates, repeat the public source,
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

Branding and cross-link edits after this tag are documentation changes. Do not
retag or republish `v0.5.4` to carry them.
