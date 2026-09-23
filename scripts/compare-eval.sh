#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'usage: compare-eval.sh BEFORE.json AFTER.json' >&2
  exit 2
fi

jq -e -n --slurpfile before "$1" --slurpfile after "$2" '
  ($before[0]) as $a | ($after[0]) as $b |
  ([ $a.cases[] | {id, rule_id, truth} ] | sort_by(.id)) as $old_cases |
  ([ $b.cases[] | {id, rule_id, truth} ] | sort_by(.id)) as $new_cases |
  if $a.schema_version != $b.schema_version or
     $a.corpus_version != $b.corpus_version or $old_cases != $new_cases or
     ($old_cases|length) == 0 or
     ($a.cases|all(.[]; (.predicted|type) == "boolean")|not) or
     ($b.cases|all(.[]; (.predicted|type) == "boolean")|not) or
     ($a.true_positive + $a.true_negative + $a.false_positive + $a.false_negative != ($a.cases|length)) or
     ($b.true_positive + $b.true_negative + $b.false_positive + $b.false_negative != ($b.cases|length)) or
     (($a.corpus_sha256 == null) != ($b.corpus_sha256 == null)) or
     ($a.corpus_sha256 != null and $b.corpus_sha256 != null and
      $a.corpus_sha256 != $b.corpus_sha256) then
    error("evaluation manifests use different corpora or labels")
  else
    {corpus_version:$a.corpus_version,
     changed_cases:[ $a.cases[] as $old |
       $b.cases[] | select(.id == $old.id and .predicted != $old.predicted) |
       {id, rule_id, before:$old.predicted, after:.predicted} ] | sort_by(.id),
     before:{true_positive:$a.true_positive, true_negative:$a.true_negative,
             false_positive:$a.false_positive, false_negative:$a.false_negative,
             elapsed_ms:$a.elapsed_ms, input_tokens:$a.input_tokens,
             output_tokens:$a.output_tokens, cost_usd:$a.cost_usd},
     after:{true_positive:$b.true_positive, true_negative:$b.true_negative,
            false_positive:$b.false_positive, false_negative:$b.false_negative,
            elapsed_ms:$b.elapsed_ms, input_tokens:$b.input_tokens,
            output_tokens:$b.output_tokens, cost_usd:$b.cost_usd}}
  end
'
