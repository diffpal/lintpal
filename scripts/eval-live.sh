#!/usr/bin/env bash
set -euo pipefail
umask 077

# Optional, local-only evaluation. Requires Python 3, jq, git, and a built lintpal.
: "${LINTPAL_EVAL_BIN:?set LINTPAL_EVAL_BIN to a built lintpal executable}"
: "${LINTPAL_EVAL_PROVIDER:?set LINTPAL_EVAL_PROVIDER}"
: "${LINTPAL_EVAL_MODEL:?set LINTPAL_EVAL_MODEL to a pinned model}"
: "${LINTPAL_EVAL_OUTPUT:?set LINTPAL_EVAL_OUTPUT to a local JSON path}"
if [[ "$LINTPAL_EVAL_MODEL" == "jev-latest" ]]; then
  echo 'a pinned model is required' >&2
  exit 2
fi
if [[ "$LINTPAL_EVAL_BIN" != /* ]]; then
  echo 'LINTPAL_EVAL_BIN must be an absolute path' >&2
  exit 2
fi
case "$LINTPAL_EVAL_PROVIDER" in
  jev) token_env=TYPESAFE_API_KEY ;;
  openrouter) token_env=OPENROUTER_API_KEY ;;
  custom)
    : "${LINTPAL_EVAL_BASE_URL:?custom evaluation requires LINTPAL_EVAL_BASE_URL}"
    token_env="${LINTPAL_EVAL_AUTH_TOKEN_ENV:-LINTPAL_TOKEN}"
    ;;
  *) echo 'invalid evaluation provider' >&2; exit 2 ;;
esac
if [[ ! "$token_env" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo 'invalid credential variable name' >&2
  exit 2
fi
token="${!token_env:-}"
if [[ -z "$token" ]]; then
  echo 'selected credential is missing' >&2
  exit 2
fi
if [[ ( -n "${LINTPAL_EVAL_INPUT_USD_PER_MILLION:-}" && -z "${LINTPAL_EVAL_OUTPUT_USD_PER_MILLION:-}" ) ||
      ( -z "${LINTPAL_EVAL_INPUT_USD_PER_MILLION:-}" && -n "${LINTPAL_EVAL_OUTPUT_USD_PER_MILLION:-}" ) ]]; then
  echo 'provide both token prices or neither' >&2
  exit 2
fi
input_price="${LINTPAL_EVAL_INPUT_USD_PER_MILLION:-null}"
output_price="${LINTPAL_EVAL_OUTPUT_USD_PER_MILLION:-null}"
if ! jq -e -n --argjson input "$input_price" --argjson output "$output_price" \
  'if $input == null then $output == null else
     ($input|type) == "number" and ($output|type) == "number" and $input >= 0 and $output >= 0 end' >/dev/null; then
  echo 'invalid token price' >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
corpus="$repo_root/internal/apps/lintpal/eval/testdata/corpus.json"
if ! jq -e '.schema_version == "lintpal.eval.corpus.v1" and (.cases | type == "array" and length > 0 and length <= 128)' "$corpus" >/dev/null; then
  echo 'invalid evaluation corpus' >&2
  exit 2
fi
corpus_hash="$(sha256sum "$corpus" | cut -d ' ' -f 1)"
binary_hash="$(sha256sum "$LINTPAL_EVAL_BIN" | cut -d ' ' -f 1)"
workdir="$(mktemp -d)"
trap 'rm -rf -- "$workdir"' EXIT
rows="$workdir/rows.jsonl"
for variable in ${!GIT_@}; do
  unset "$variable"
done
mkdir -p "$workdir/empty-template"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_TEMPLATE_DIR="$workdir/empty-template"
export GIT_OPTIONAL_LOCKS=0
export GIT_AUTHOR_DATE='2000-01-01T00:00:00Z'
export GIT_COMMITTER_DATE='2000-01-01T00:00:00Z'

while IFS= read -r case_json; do
  case_id="$(jq -r '.id' <<<"$case_json")"
  case_path="$(jq -r '.path' <<<"$case_json")"
  if [[ ! "$case_id" =~ ^[a-z0-9][a-z0-9_-]{0,63}$ ||
        ! "$case_path" =~ ^[A-Za-z0-9_./-]+$ || "$case_path" == /* || "$case_path" == *..* ]]; then
    echo 'invalid corpus case path or ID' >&2
    exit 2
  fi
  case_dir="$workdir/$case_id"
  mkdir -p "$case_dir/$(dirname "$case_path")"
  git -C "$case_dir" init -q
  git -C "$case_dir" config user.name 'lintpal eval'
  git -C "$case_dir" config user.email 'eval@example.invalid'
  printf 'package fixture\n' >"$case_dir/$case_path"
  git -C "$case_dir" add -- "$case_path"
  git -C "$case_dir" commit -qm base
  base="$(git -C "$case_dir" rev-parse HEAD)"
  jq -j '.source' <<<"$case_json" >"$case_dir/$case_path"
  git -C "$case_dir" add -- "$case_path"
  git -C "$case_dir" commit -qm head
  head="$(git -C "$case_dir" rev-parse HEAD)"

  args=(lint --base "$base" --head "$head" --provider "$LINTPAL_EVAL_PROVIDER"
    --model "$LINTPAL_EVAL_MODEL" --rules "$repo_root/internal/apps/lintpal/eval/testdata/rules" --out "" --format json --fail-on none
    --timeout 2m --max-concurrency 4)
  if [[ "$LINTPAL_EVAL_PROVIDER" == custom ]]; then
    args+=(--base-url "$LINTPAL_EVAL_BASE_URL" --auth-token-env "$token_env")
  fi
  started="$(python3 -c 'import time; print(time.monotonic_ns() // 1000000)')"
  if ! (cd "$case_dir" && "$LINTPAL_EVAL_BIN" "${args[@]}") >"$case_dir/report.json" 2>"$case_dir/error.txt"; then
    echo "evaluation failed for case $case_id" >&2
    exit 1
  fi
  elapsed=$(( $(python3 -c 'import time; print(time.monotonic_ns() // 1000000)') - started ))
  jq -n --arg id "$case_id" --arg rule "$(jq -r '.rule_id' <<<"$case_json")" \
    --arg truth "$(jq -r '.truth' <<<"$case_json")" --argjson elapsed "$elapsed" \
    --argjson input_price "$input_price" --argjson output_price "$output_price" \
    --slurpfile report "$case_dir/report.json" '
      ($report[0]) as $r |
      {id:$id, rule_id:$rule, truth:$truth,
       predicted:any($r.findings[]; .evidence.kind == "rule" and .evidence.rule_id == $rule and .changed_span.side == "RIGHT"), elapsed_ms:$elapsed,
       input_tokens:$r.stats.input_tokens, output_tokens:$r.stats.output_tokens,
       cost_usd:(if $input_price == null then null else
         (($r.stats.input_tokens * $input_price + $r.stats.output_tokens * $output_price) / 1000000) end)}
    ' >>"$rows"
done < <(jq -c '.cases[]' "$corpus")

manifest="$(jq -s --arg provider "$LINTPAL_EVAL_PROVIDER" --arg model "$LINTPAL_EVAL_MODEL" \
  --arg corpus_hash "$corpus_hash" --arg binary_hash "$binary_hash" '
  sort_by(.id) as $cases |
  {schema_version:"lintpal.eval.live.v1", corpus_version:"lintpal.eval.corpus.v1",
   corpus_sha256:$corpus_hash, binary_sha256:$binary_hash,
   provider:$provider, model:$model, cases:$cases,
   true_positive:($cases|map(select(.truth == "buggy" and .predicted))|length),
   true_negative:($cases|map(select(.truth == "safe" and (.predicted|not)))|length),
   false_positive:($cases|map(select(.truth == "safe" and .predicted))|length),
   false_negative:($cases|map(select(.truth == "buggy" and (.predicted|not)))|length),
   elapsed_ms:($cases|map(.elapsed_ms)|add),
   input_tokens:($cases|map(.input_tokens)|add),
   output_tokens:($cases|map(.output_tokens)|add),
   cost_usd:(if ($cases|all(.[]; .cost_usd != null)) then ($cases|map(.cost_usd)|add) else null end)}
' "$rows")"
if [[ "$manifest" == *"$token"* ]]; then
  echo 'evaluation manifest is unsafe' >&2
  exit 4
fi
output_dir="$(dirname "$LINTPAL_EVAL_OUTPUT")"
mkdir -p "$output_dir"
temp_output="$(mktemp "$output_dir/.lintpal-eval.XXXXXX")"
printf '%s\n' "$manifest" >"$temp_output"
mv -f -- "$temp_output" "$LINTPAL_EVAL_OUTPUT"
