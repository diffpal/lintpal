# Product Hunt launch draft for LintPal

This is a reviewable draft for the launch owner. It does not publish a release,
enable GitHub Discussions, or create a Product Hunt post. At the pre-merge
check, the published binary release was
[v0.5.3](https://github.com/diffpal/lintpal/releases/tag/v0.5.3);
its [release notes](../../CHANGELOG.md) record a package-description change,
not new CLI behavior. The [README](../../README.md) and
[privacy guide](../architecture/privacy.md) describe the product and data flow.
The [candidate PR #5](https://github.com/diffpal/lintpal/pull/5) prepares
`v0.5.4`. At the pre-merge check on 2026-09-29, the local and remote tag,
GitHub Release, and all seven npm identities were absent for that version.
Recheck immediately before release.

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

Pre-merge snapshot from 2026-09-29. **The preparation handoff is ready for review;
the public Product Hunt launch is NO-GO.** A passing local check does not put
the current worktree changes into the already published `v0.5.3` tag. Merge the
scoped changes and confirm the owner gates below before posting.

| Gate | Current evidence and status | Owner / exit condition |
| --- | --- | --- |
| Source | PR #5 prepares a `v0.5.4` README pin and changelog entry along with the new docs, test, and forms. Local checks passed on 2026-09-28; PR checks passed on 2026-09-29 across Linux, macOS, and Windows. **PR pass; merge and default-branch CI pending.** | Maintainer reviews and merges the scoped diff; default-branch CI passes. |
| Package | Local Omnidist staging and dry run passed; isolated Linux x64 installs of both npm meta packages returned `lintpal 0.5.3` and working help. All seven packages are published at `0.5.3`. However, both published npm meta-package READMEs still show the old Action pin `0.4.1`. **Binary pass; published-copy blocker.** Other platforms were inspected as artifacts, not executed locally. | Maintainer publishes an unused new version through the release workflow after merging the corrected README, then checks all seven identities, the published README, and a fresh install. |
| Community | GitHub reported Discussions disabled. Three repository forms are prepared, while README uses the working Issues route. **Blocked.** | Owner enables Discussions, confirms category slugs, merges the forms, verifies that they render, and posts the welcome thread. |
| Page | Copy and capture instructions are drafted; genuine thumbnail and gallery images are absent. Product Hunt account and page state are unverified. **Blocked.** | Owner captures and checks the media, reviews the draft in an eligible account, and posts only after a fresh product check. |

The released `v0.5.3` package is immutable. **Do not retag or republish
`v0.5.3`** to include these worktree changes. A merge without binary changes
can keep using the existing binary, but it cannot correct the already
published npm README. The corrected package content needs an unused new SemVer
version through the normal release workflow before the version-consistency
gate can pass.

The [candidate changelog](../../CHANGELOG.md) describes `v0.5.4`: the README
Action pin and setup/provider wording are corrected, repository Discussion
forms are prepared, and a committed-revision violation/fix/green rerun is
covered by a fake-provider process test. There is no new CLI behavior or
claimed model-accuracy improvement. Confirm the merged change list and date
before creating the release tag.

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

## Real asset capture plan

No screenshots or GIFs have been generated for this draft. The
[demo pull request](https://github.com/diffpal/lintpal-demo/pull/3) shows a
v0.5.0 flow and can guide the sequence; capture fresh output from the final
published version for the launch. A local run can use committed revisions and
a fake provider. Check all text at final image size; do not imply measured
model accuracy or an unsupported language matrix.

| Asset | Capture | Suggested caption |
| --- | --- | --- |
| Square thumbnail | LintPal name or genuine project mark, no tiny terminal text | LintPal |
| Gallery 1 | Repository Markdown rule beside its line-level PR finding | One Markdown rule. One PR finding. |
| Gallery 2 | The same finding's severity and failing CI gate | Gate CI by severity. |
| Gallery 3 | Changed-line finding followed by a fixed committed revision and green rerun | Review, fix, rerun. |

Product Hunt currently recommends a 240×240 thumbnail and 1270×760 gallery
images; a gallery needs at least two images to be visible. The owner must
capture and inspect the final images before launch. A video is optional and
must be available on YouTube if included.

## GitHub Discussions setup

GitHub reported Discussions disabled for `diffpal/lintpal` on 2026-09-29. The
owner must enable it and create or confirm categories with these exact slugs
before the matching forms become usable:

| Category | Format | Form |
| --- | --- | --- |
| Q&A (`q-a`) | Question and answer | [q-a.yml](../../.github/DISCUSSION_TEMPLATE/q-a.yml) |
| Ideas (`ideas`) | Open discussion | [ideas.yml](../../.github/DISCUSSION_TEMPLATE/ideas.yml) |
| Rule feedback (`rule-feedback`) | Open discussion | [rule-feedback.yml](../../.github/DISCUSSION_TEMPLATE/rule-feedback.yml) |

Announcements and Show and tell are optional additional categories. GitHub
requires each [form filename to match the category slug](https://docs.github.com/en/discussions/managing-discussions-for-your-community/creating-discussion-category-forms).
If GitHub assigns another slug, rename the corresponding form before merging.
The forms need to be on the default branch to appear to visitors.

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

## Owner actions before posting

1. Review the candidate diff and checks, then prepare a pull request with the
   documentation, test, and template changes. These commands show the initial
   branch and PR path; inspect Git and GitHub first and skip steps already done:

   ```bash
   git switch -c docs/product-hunt-launch
   git add README.md CHANGELOG.md cmd/lintpal/main_test.go docs/index.md \
     docs/development/product-hunt-launch.md .github/DISCUSSION_TEMPLATE/
   git diff --cached --check
   git commit -m "docs: prepare LintPal launch"
   git push -u origin docs/product-hunt-launch
   gh pr create --base main --head docs/product-hunt-launch \
     --title "Prepare LintPal launch" \
     --body "Review launch docs, feedback forms, and CLI smoke coverage."
   ```

   Review the PR after its CI passes. **Hold the merge until the owner is ready
   to push the `v0.5.4` release tag promptly afterward.** Before that release,
   the new README pin points to a package that is not available. The branch
   name and PR command assume they are not already in use; adjust them if
   necessary.
2. Enable Discussions, create the categories, confirm the three slugs, and
   verify the forms render on the default branch. In repository Settings,
   select Discussions under Features; then manage categories from the
   Discussions tab. See GitHub's [enablement](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/enabling-or-disabling-github-discussions-for-a-repository)
   and [category](https://docs.github.com/en/discussions/managing-discussions-for-your-community/managing-categories-for-discussions)
   instructions. Post the Welcome discussion.
3. Capture a genuine square thumbnail and at least two readable gallery
   images, and verify their content against the shipped product.
4. Review the draft fields and maker comment in a personal Product Hunt
   account. Use the repository as the destination unless a current product
   page offers a better direct route.
5. Recheck the published package, CI, feedback link, and assets immediately
   before scheduling or posting. The commands below inspect the current
   `0.5.3` baseline; inspect `0.5.4` too after its release. Do not
   re-publish `0.5.3`.

   ```bash
   gh api repos/diffpal/lintpal --jq .has_discussions
   gh run list --repo diffpal/lintpal --branch main --limit 5
   npm view lintpal@0.5.3 version
   npm view @diffpal/lintpal@0.5.3 version
   npm install -g lintpal@0.5.3
   lintpal version
   lintpal --help
   ```

   Check all five platform-package versions against the same release before
   claiming registry parity. Verify the published npm README Action pin, the
   merged repository README, and rendered Discussion forms in a browser.
   Record a fresh GO decision only when source, package, community, and page
   gates all pass.

### New package release path

The published npm README mismatch means the version-consistency gate needs a
new package release even if the binary code does not change. PR #5 prepares
the `0.5.4` README pin and changelog; recheck that the version is
unused, finalize the release date, merge the changes, and confirm the release
commit is checked out and clean. Otherwise the new npm package could embed an
old Action pin and repeat the mismatch. For the reviewed `0.5.4` candidate:

```bash
task check
task race
task lint-go
task security
go mod verify
git tag -a v0.5.4 -m "Release v0.5.4"
task release-stage
rg -n 'lintpal-version: "0.5.4"' README.md \
  .omnidist/default/npm/lintpal/README.md \
  .omnidist/default/npm/@diffpal/lintpal/README.md
task release-dry-run
git push origin v0.5.4
```

**The tag push starts `.github/workflows/omnidist-release.yml`.** That workflow
publishes the seven npm packages and creates the GitHub Release after its
checks pass. Do not also run `npm publish` or `gh release create` manually.
Confirm the workflow, release assets, all seven registry versions, and the
corrected README on both new npm meta-package pages before posting to Product
Hunt. The two meta packages share the same version and platform-package set.
