# lintpal CLI Reference

The `lintpal` command-line interface provides commands to lint Git changes, publish feedback, inspect and import rules, check local prerequisites, and generate shell completions.

## Global Flags

The following flag is available across all commands and subcommands:

| Flag | Description |
| --- | --- |
| `-h`, `--help` | Display help and usage information for `lintpal` or the active subcommand. |

---

## `lintpal lint`

Lint committed Git changes between two revisions, or uncommitted changes in the local working tree. By default, it prints Markdown feedback to stdout; `--format json` outputs a versioned findings v5 JSON report. Operational status and metrics use stderr.

### Usage

```bash
lintpal lint [flags]
```

### Examples

```bash
# Review committed revisions with the default provider (jev)
export TYPESAFE_API_KEY='...'
lintpal lint --base origin/main --head HEAD

# Review uncommitted working-tree changes before committing
lintpal lint --uncommitted

# Use OpenRouter with JSON output
export OPENROUTER_API_KEY='...'
lintpal lint --base origin/main --head HEAD --provider openrouter --format json

# Use the assumed compatible OpenAI Decisions endpoint with an explicit model
export OPENAI_API_KEY='...'
lintpal lint --base origin/main --head HEAD --provider openai --model '<model-id>'

# Use a custom endpoint and write an atomic JSON artifact
export LINTPAL_TOKEN='...'
mkdir -p .artifacts/lintpal
lintpal lint --base origin/main --head HEAD --provider custom \
  --base-url https://decisions.example.test/api --api-path /v1/decisions --model '<model-id>' \
  --format json --out .artifacts/lintpal/findings.json
```

### Flags

| Flag | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `--uncommitted` | — | `false` | Review uncommitted working tree changes (staged, unstaged, and untracked regular files) against `HEAD`. Mutually exclusive with `--base` and `--head`. |
| `--base <rev>` | `LINTPAL_BASE` | — | Base revision used to find the unique merge base with head; the diff starts at that merge base. Required unless `--uncommitted` is specified. |
| `--head <rev>` | `LINTPAL_HEAD` | — | Head commit or revision. Required unless `--uncommitted` is specified. |
| `--provider <name>` | `LINTPAL_PROVIDER` | `jev` | Decision provider: `jev` (TypeSafe), `openrouter`, `openai`, or `custom`. |
| `--model <name>` | `LINTPAL_MODEL` | `typesafe/jev-1.13` for OpenRouter; required for OpenAI; `jev-latest` for jev/custom | Model identifier accepted by the selected provider. Explicit values are sent unchanged. |
| `--rules <path>` | `LINTPAL_RULES` | `.lintpal/rules/` | Override Markdown rules directory for this single run. |
| `--include <glob>` | — | — | Include changed source paths matching glob (repeatable). |
| `--exclude <glob>` | — | — | Exclude changed source paths matching glob (repeatable). |
| `--rule-threshold <val>` | `LINTPAL_RULE_THRESHOLD` | `0.95` | Override every rule's violation probability threshold (0 to 1). |
| `--rule-severity <val>` | `LINTPAL_RULE_SEVERITY` | `medium` | Override every rule's finding severity: `low`, `medium`, `high`, or `critical`. |
| `--format <val>` | `LINTPAL_FORMAT` | `markdown` | Stdout output format: `markdown` or `json`. |
| `--out <path>` | `LINTPAL_OUT` | — | Additional path to write complete findings v5 JSON report atomically. |
| `--fail-on <val>` | `LINTPAL_FAIL_ON` | `high` | Severity gate threshold: `low`, `medium`, `high`, `critical`, or `none`. Returns exit code `10` if any finding meets the gate. |
| `--block-on <val>` | — | — | Mark findings `blocking` at this threshold without failing the lint run (exit code 0). Mutually exclusive with `--fail-on`. |
| `--timeout <duration>` | `LINTPAL_TIMEOUT` | `2m` | Whole-run execution deadline (maximum `10m`). |
| `--max-concurrency <int>` | `LINTPAL_MAX_CONCURRENCY` | `4` | Maximum concurrent provider requests (maximum `16`). |
| `--base-url <url>` | `LINTPAL_BASE_URL` | — | Base URL for `custom` provider. Required for `custom`; rejected for presets. |
| `--api-path <path>` | `LINTPAL_API_PATH` | `/v1/systemone` for custom | API path appended to the custom base prefix; rejected for presets. Explicit empty input is invalid. |
| `--auth-token-env <name>` | `LINTPAL_AUTH_TOKEN_ENV` | `LINTPAL_TOKEN` | Environment variable name holding bearer token for `custom` provider. |
| `--metrics` | — | `false` | Print local run metrics (stage counts and durations) to stderr. |
| `--env-file <path>` | — | — | Load settings and credentials from a specified `.env` file. |
| `--no-env-file` | — | `false` | Disable automatic `.env` file loading from the worktree root. |

### Operational Details

Supported providers are TypeSafe/Jev (`jev`), OpenRouter (`openrouter`),
OpenAI (`openai`), and `custom`. See [provider setup](../guides/configuration.md#supported-providers)
for complete commands, endpoint and credential choices.

`jev` uses `https://api.typesafe.ai/v1/systemone`; OpenRouter uses
`https://openrouter.ai/api/alpha/decisions`. OpenAI fixes
`https://api.openai.com/v1/decisions` under the assumed compatible API contract;
live availability is not verified. OpenAI requires `--model` or `LINTPAL_MODEL`.
Without an override, OpenRouter uses `typesafe/jev-1.13`; jev/custom use `jev-latest`.

Custom appends `--api-path` (default `/v1/systemone`) to the trusted base URL
prefix after trimming trailing base slashes. Only unescaped absolute paths are
accepted: no query, fragment, whitespace, backslashes, repeated slashes or
`.`/`..` segments. Presets reject custom endpoint/path/token options; custom
rejects all three preset key variable names. Invalid configuration fails before HTTP.

- **Uncommitted vs. Committed**: With `--uncommitted`, LintPal reviews working-tree modifications, staged edits, and untracked regular files against `HEAD`. Combining `--uncommitted` with `--base` or `--head` is rejected. In this mode, `head_sha` in reports is set to `UNCOMMITTED` and `base_sha` is the current `HEAD` commit SHA (or the Git empty tree if no commits exist). Untracked empty and binary files are skipped.
- **Precedence Order**: Explicit CLI flags override environment variables (`LINTPAL_*`), which override `.env` values, which override built-in defaults. Provider credentials in process environment take precedence over `.env`.
- **Credential Protection**: Before writing any report or artifact, LintPal checks output bytes against the active credential. If found, the run fails with exit code `4` without writing output.
- **Metrics**: `--metrics` prints fixed stage lines to stderr (e.g. `metric stage=compare status=ok count=1 duration_ms=18`). No source code, prompt, endpoint, or token data ever enters metrics.

Committed comparisons review the unique merge base of `--base` and `--head`
through `--head`, equivalent to the range selected by `git diff BASE...HEAD`.
Changes found only on the base branch are excluded. Reports retain the requested
base commit in `base_sha` and the actual old-side commit in `merge_base_sha`;
LEFT anchors refer to that merge base. Missing or multiple merge bases fail
with exit code `2`.

---

## `lintpal feedback`

Render or publish stored findings v5 JSON reports without rerunning analysis or calling AI providers.

### `lintpal feedback markdown`

Renders stored findings v5 JSON as Markdown to stdout or an atomic output file.

#### Usage

```bash
lintpal feedback markdown [flags]
```

#### Examples

```bash
# Render report to stdout
lintpal feedback markdown --in .artifacts/lintpal/findings.json

# Write Markdown to file and evaluate gate
lintpal feedback markdown --in .artifacts/lintpal/findings.json \
  --out .artifacts/lintpal/feedback.md --gate
```

#### Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--in <path>` | string | — | **Required.** Path to findings v5 JSON report to read. |
| `--out <path>` | string | — | Write Markdown atomically to this path instead of stdout. |
| `--gate` | bool | `false` | Exit with code `10` after output if any stored finding is blocking. |

---

### `lintpal feedback github`

Publishes stored findings to a GitHub pull request as a summary review comment with inline comments on changed lines.

#### Usage

```bash
lintpal feedback github [flags]
```

#### Examples

```bash
# Publish review in GitHub Actions
export GITHUB_TOKEN='...'
lintpal feedback github --in .artifacts/lintpal/findings.json \
  --repo ${{ github.repository }} --pr-number ${{ github.event.pull_request.number }} \
  --base ${{ github.event.pull_request.base.sha }} --head ${{ github.event.pull_request.head.sha }} \
  --gate

# Dry-run locally without API calls or GitHub token
lintpal feedback github --in .artifacts/lintpal/findings.json \
  --repo owner/repo --pr-number 42 \
  --base "$BASE_SHA" --head "$HEAD_SHA" \
  --dry-run
```

#### Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--in <path>` | string | — | **Required.** Path to findings v5 JSON report to read. |
| `--repo <owner/name>` | string | — | GitHub repository as `owner/repo`. Auto-detected from `GITHUB_REPOSITORY` if omitted. |
| `--pr-number <int>` | int | `0` | Pull request number. Auto-detected from `GITHUB_EVENT_PATH` if omitted. |
| `--base <sha>` | string | — | Expected PR base commit SHA. Auto-detected from `GITHUB_EVENT_PATH` if omitted. |
| `--head <sha>` | string | — | Expected PR head commit SHA. Auto-detected from `GITHUB_EVENT_PATH` if omitted. |
| `--review-channel <name>` | string | `lintpal` | Publication channel tag to isolate reviews and deduplicate unchanged findings. |
| `--auth-token-env <name>` | string | `GITHUB_TOKEN` | Name of the environment variable containing the GitHub token. |
| `--dry-run` | bool | `false` | Render intended review and inline bodies to stdout without GitHub API calls. |
| `--gate` | bool | `false` | Exit with code `10` after output or publication if any stored finding is blocking. |

#### GitHub Permissions & Behavior

- Requires `pull-requests: write` permission to publish reviews; otherwise only `contents: read` is needed.
- Fork pull requests are skipped cleanly before reading tokens or making API requests.
- An identical marked summary and unchanged inline finding digests are skipped. Changed feedback creates a new review and new inline comments for changed findings; existing comments are not edited. New reviews may generate GitHub notifications.

---

## `lintpal doctor`

Verifies local Git repository state and confirms that provider credentials and configuration options are set up correctly without making remote provider API calls.

### Usage

```bash
lintpal doctor [flags]
```

### Examples

```bash
# Check default jev configuration
lintpal doctor

# Check OpenRouter credentials
export OPENROUTER_API_KEY='...'
lintpal doctor --provider openrouter

# Check OpenAI credential presence without contacting the assumed endpoint
export OPENAI_API_KEY='...'
lintpal doctor --provider openai

# Check custom provider settings
lintpal doctor --provider custom --base-url https://decisions.example.test/api \
  --api-path /v1/decisions --auth-token-env CUSTOM_TOKEN
```

### Flags

| Flag | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `--provider <name>` | `LINTPAL_PROVIDER` | `jev` | Provider to verify: `jev`, `openrouter`, `openai`, or `custom`. |
| `--base-url <url>` | `LINTPAL_BASE_URL` | — | Custom provider base URL (required when `--provider custom`). |
| `--api-path <path>` | `LINTPAL_API_PATH` | `/v1/systemone` for custom | API path appended to the custom base prefix; rejected for presets. Explicit empty input is invalid. |
| `--auth-token-env <name>` | `LINTPAL_AUTH_TOKEN_ENV` | `LINTPAL_TOKEN` | Token environment variable name to check for `custom` provider. |
| `--env-file <path>` | — | — | Load settings from this `.env` file before checking. |
| `--no-env-file` | — | `false` | Do not load `.env` from the Git worktree root. |

---

## `lintpal rule`

Inspect, validate, and import repository Markdown rules in `.lintpal/rules/`. `list`, `view`, `validate`, and imports from local directories run without network access. Imports from GitHub use its API and raw content endpoints. Rule commands do not use provider credentials.

### Subcommands

```bash
lintpal rule list
lintpal rule view <id>
lintpal rule validate
lintpal rule import <source> [flags]
```

### `lintpal rule list`
Prints all discovered rule IDs in the worktree root `.lintpal/rules/` directory, one per line. Takes no flags.

### `lintpal rule view <id>`
Displays the mandate body and effective policy (title, severity, threshold) for a specific rule ID (e.g. `go/errors.md`). Takes no flags.

### `lintpal rule validate`
Validates all Markdown files in `.lintpal/rules/` for valid frontmatter, character limits, path constraints, and syntax. Prints the number of validated rules. Takes no flags.

### `lintpal rule import <source>`
Copies Markdown rules from a local directory or versioned GitHub repository into `.lintpal/rules/`.

#### Examples

```bash
# Import from a local directory with a prefix
lintpal rule import ./examples/rules/go-review --prefix team

# Import from a pinned GitHub tag
lintpal rule import github:diffpal/lintpal-rules//go@v1.1.0

# Overwrite colliding rules
lintpal rule import github:acme/rules//security@v2.0.0 --force
```

#### Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--prefix <string>` | string | — | Prefix prepended to all imported rule IDs (e.g. `--prefix team`). |
| `--force` | bool | `false` | Overwrite existing rule files if IDs collide; otherwise fails without modifying catalog. |

---

## `lintpal completion`

Generate shell autocompletion scripts for supported shells.

### Usage

```bash
lintpal completion <bash|zsh|fish|powershell>
```

### Examples

```bash
# Bash
source <(lintpal completion bash)

# Zsh
lintpal completion zsh > "${fpath[1]}/_lintpal"

# Fish
lintpal completion fish | source
```

---

## `lintpal version`

Print the current lintpal build version. Takes no flags.

```bash
lintpal version
# Output: lintpal 0.5.1
```

---

## Exit Codes

All `lintpal` commands adhere to the following exit codes:

| Exit code | Meaning |
| ---: | --- |
| **0** | Success. Command completed without errors or gate failures. |
| **2** | Invalid CLI options, configuration flags, revision inputs, or non-retryable provider settings. |
| **3** | Retryable provider failure, network transport error, or request timeout. |
| **4** | Report export failure, file write error, or credential leak detected in output. |
| **5** | Internal error, protocol violation, or unclassified local execution failure. |
| **10** | Gate failure. Report or feedback written/published, but one or more findings met or exceeded the severity gate threshold. |
| **130** | Process interrupted or canceled (e.g. via `SIGINT` / `Ctrl+C`). |

---

## Environment Variables Reference

| Variable | Associated Flag | Description |
| --- | --- | --- |
| `TYPESAFE_API_KEY` | — | API key for the default `jev` (TypeSafe) provider. |
| `OPENROUTER_API_KEY` | — | API key for the `openrouter` provider. |
| `OPENAI_API_KEY` | — | API key for the `openai` preset (assumed compatible Decisions API). |
| `LINTPAL_TOKEN` | `--auth-token-env` | Default environment variable checked for bearer token when `--provider custom` is used. |
| `GITHUB_TOKEN` | `--auth-token-env` | Default token variable for `lintpal feedback github`. |
| `LINTPAL_PROVIDER` | `--provider` | Default provider (`jev`, `openrouter`, `openai`, or `custom`). |
| `LINTPAL_MODEL` | `--model` | Model name or alias. |
| `LINTPAL_BASE` | `--base` | Base commit revision. |
| `LINTPAL_HEAD` | `--head` | Head commit revision. |
| `LINTPAL_RULES` | `--rules` | Path to rules directory. |
| `LINTPAL_RULE_THRESHOLD` | `--rule-threshold` | Global rule threshold override. |
| `LINTPAL_RULE_SEVERITY` | `--rule-severity` | Global rule severity override. |
| `LINTPAL_FORMAT` | `--format` | Stdout output format (`markdown` or `json`). |
| `LINTPAL_OUT` | `--out` | Output path for findings v5 JSON artifact. |
| `LINTPAL_FAIL_ON` | `--fail-on` | Gate severity threshold. |
| `LINTPAL_TIMEOUT` | `--timeout` | Whole-run execution timeout. |
| `LINTPAL_MAX_CONCURRENCY`| `--max-concurrency`| Maximum concurrent provider requests. |
| `LINTPAL_BASE_URL` | `--base-url` | Custom provider base URL. |
| `LINTPAL_API_PATH` | `--api-path` | Custom API path; defaults to `/v1/systemone` when absent. |
| `LINTPAL_AUTH_TOKEN_ENV` | `--auth-token-env` | Environment variable name for custom provider bearer token. |
| `GITHUB_REPOSITORY` | `--repo` | In GitHub Actions, auto-detects `owner/repo`. |
| `GITHUB_EVENT_PATH` | `--pr-number`, `--base`, `--head` | In GitHub Actions, auto-detects PR number, base SHA, and head SHA. |

See the [Report reference](report.md) for findings v5 format specifications, [Privacy and limitations](../architecture/privacy.md) for data transfer boundaries, and [Resource limits](../architecture/resource-limits.md) for size caps.
