#!/bin/sh
# Owns the mapping between three surfaces that must stay aligned:
#   1. every prerequisite reached by `make verify`;
#   2. the CI jobs the workflow defines;
#   3. the status contexts listed in the committed .github/rulesets/main.json
#      artifact — never the live GitHub ruleset.
#
# The contract lists all verify prerequisites explicitly. Dropping one from the
# Makefile without updating the contract fails here, and listing one in the
# contract that verify no longer reaches fails here too.
set -eu
lock_test=0
case "${1:-}" in
  --lock-test)
    lock_test=1
    shift
    ;;
esac
makefile="${1:-Makefile}"
workflow="${2:-.github/workflows/ci.yml}"
ruleset="${3:-.github/rulesets/main.json}"
contract="${4:-.github/gate-contract.json}"
status=0

fail() {
  echo "check-gate-contract: $1" >&2
  status=1
}

sorted_lines() {
  LC_ALL=C sort -u
}

same_sets() {
  left=$(printf '%s\n' "$1" | sorted_lines)
  right=$(printf '%s\n' "$2" | sorted_lines)
  [ "$left" = "$right" ]
}

verify_prereqs() {
  awk '/^verify:/ { for (i = 2; i <= NF; i++) print $i }' "$makefile" | sorted_lines
}

ci_jobs() {
  awk '
    /^jobs:/ { in_jobs = 1; next }
    in_jobs && /^  [a-zA-Z0-9_-]+:$/ {
      line = $0
      sub(/^  /, "", line)
      sub(/:$/, "", line)
      print line
      next
    }
    in_jobs && /^[^ ]/ { in_jobs = 0 }
  ' "$workflow" | sorted_lines
}

ruleset_contexts() {
  jq -r '.rules[] | select(.type == "required_status_checks") | .parameters.required_status_checks[].context' "$ruleset" | sorted_lines
}

contract_verify_prereqs() {
  jq -r '.verify_prerequisites[]' "$contract" | sorted_lines
}

ci_owned_prereqs() {
  jq -r '.ci_jobs[] | select(.owns != null) | .owns[]' "$contract" | sorted_lines
}

verify_invocation() {
  jq -r '.ci_jobs[] | select(.name == "verify") | .invocation // empty' "$contract"
}

# Comment lines are not steps. A recipe line may carry a trailing comment, but
# a line whose first non-blank character is "#" runs nothing.
uncommented() {
  sed 's/^[[:space:]]*#.*$//' "$1"
}

vet_tags() {
  uncommented "$1" | awk -v guarded="$2" '
    function flush(   i) {
      for (i = 1; i <= n; i++) {
        if (has_if && guarded == "report") { print "guarded:" pending[i] } else { print pending[i] }
      }
      n = 0
      has_if = 0
    }
    /^[[:space:]]*-[[:space:]]*name:/ { flush() }
    /^[[:space:]]*if:/ { has_if = 1 }
    /go vet/ {
      tag = "(none)"
      if (match($0, /-tags [A-Za-z0-9_,]+/)) {
        tag = substr($0, RSTART + 6, RLENGTH - 6)
      }
      pending[++n] = tag
    }
    END { flush() }
  ' | sorted_lines
}

lint_globs() {
  uncommented "$1" | awk '/biome lint/ { for (i = 1; i <= NF; i++) if ($i ~ /\*/) print $i }' | sorted_lines
}

run_lock_test() {
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/gate-contract-lock.XXXXXX")
  trap 'rm -rf "$tmp"' EXIT INT HUP
  cp "$makefile" "$tmp/Makefile"
  cp "$workflow" "$tmp/ci.yml"
  cp "$ruleset" "$tmp/main.json"
  cp "$contract" "$tmp/gate-contract.json"
  sed '/^verify:/ s/ test / /' "$tmp/Makefile" >"$tmp/Makefile.new"
  mv "$tmp/Makefile.new" "$tmp/Makefile"
  if sh "$0" "$tmp/Makefile" "$tmp/ci.yml" "$tmp/main.json" "$tmp/gate-contract.json"; then
    echo "check-gate-contract: lock test survived dropping test from verify; the contract must fail closed" >&2
    exit 1
  fi
  echo "check-gate-contract: lock test caught a dropped verify prerequisite"
}

[ -f "$contract" ] || fail "missing contract file $contract"
[ -f "$ruleset" ] || fail "missing ruleset file $ruleset"
command -v jq >/dev/null || fail "jq is required to read $contract and $ruleset"

if [ "$lock_test" -eq 1 ]; then
  run_lock_test
  exit 0
fi

reached=$(verify_prereqs)
[ -n "$reached" ] || fail "read no verify prerequisites out of $makefile"

contract_prereqs=$(contract_verify_prereqs)
[ -n "$contract_prereqs" ] || fail "read no verify prerequisites out of $contract"

if ! same_sets "$reached" "$contract_prereqs"; then
  fail "verify prerequisites differ between $makefile and $contract"
  tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-gate-contract.XXXXXX")
  trap 'rm -rf "$tmpdir"' 0 HUP INT TERM
  printf '%s\n' "$reached" > "$tmpdir/reached"
  printf '%s\n' "$contract_prereqs" > "$tmpdir/contract"
  comm -23 "$tmpdir/reached" "$tmpdir/contract" | sed 's/^/  only in make: /' >&2
  comm -13 "$tmpdir/reached" "$tmpdir/contract" | sed 's/^/  only in contract: /' >&2
fi

jobs=$(ci_jobs)
[ -n "$jobs" ] || fail "read no jobs out of $workflow"

expected_jobs=$(jq -r '.ci_jobs[].name' "$contract" | sorted_lines)
if ! same_sets "$expected_jobs" "$jobs"; then
  fail "CI jobs differ from the contract"
  printf '%s\n' "$expected_jobs" | sed 's/^/  expected: /' >&2
  printf '%s\n' "$jobs" | sed 's/^/  workflow: /' >&2
fi

invocation=$(verify_invocation)
[ -n "$invocation" ] || fail "contract must name the verify job invocation target"
case "$invocation" in
  verify-ci) ;;
  *)
    fail "verify job invocation must be verify-ci, not $invocation"
    ;;
esac
if ! awk '
  /^  verify:/ { in_job = 1; next }
  in_job && /^  [a-zA-Z0-9_-]+:$/ { in_job = 0 }
  in_job && /make verify-ci/ { found = 1 }
  END { exit !found }
' "$workflow"; then
  fail "verify job must run make $invocation"
fi

owned=$(ci_owned_prereqs)
[ -n "$owned" ] || fail "read no CI-owned prerequisites out of $contract"

owned_count=$(printf '%s\n' "$owned" | wc -l | tr -d ' ')
contract_count=$(printf '%s\n' "$contract_prereqs" | wc -l | tr -d ' ')
if [ "$owned_count" -ne "$(printf '%s\n' "$owned" | sorted_lines | wc -l | tr -d ' ')" ]; then
  fail "CI-owned prerequisites must be unique in $contract"
fi

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-gate-contract-owned.XXXXXX")
trap 'rm -rf "$tmpdir"' 0 HUP INT TERM
printf '%s\n' "$owned" > "$tmpdir/owned"
printf '%s\n' "$contract_prereqs" > "$tmpdir/all"
if [ -n "$(comm -23 "$tmpdir/owned" "$tmpdir/all")" ]; then
  fail "CI-owned prerequisites must be verify prerequisites"
  comm -23 "$tmpdir/owned" "$tmpdir/all" | sed 's/^/  owned but not verify: /' >&2
fi
verify_ci=$(comm -13 "$tmpdir/owned" "$tmpdir/all" | sorted_lines)
if [ -z "$verify_ci" ]; then
  fail "verify-ci would run no prerequisites; every verify prerequisite must be owned by a sibling job or verify-ci"
fi
if [ "$contract_count" -ne "$((owned_count + $(printf '%s\n' "$verify_ci" | wc -l | tr -d ' ')))" ]; then
  fail "CI-owned prerequisites and verify-ci must partition verify prerequisites exactly once"
fi

contexts=$(ruleset_contexts)
[ -n "$contexts" ] || fail "read no required contexts out of $ruleset"

expected_contexts=$(
  jq -r '.ci_jobs[] | if (.matrix | length) > 0 then .name as $job | .matrix[] | "\($job) (\(.))" else .name end' "$contract"
)
if ! same_sets "$expected_contexts" "$contexts"; then
  fail "ruleset contexts differ from the contract"
  printf '%s\n' "$expected_contexts" | sorted_lines | sed 's/^/  expected: /' >&2
  printf '%s\n' "$contexts" | sed 's/^/  ruleset: /' >&2
fi

make_vet=$(vet_tags "$makefile" count)
workflow_vet=$(vet_tags "$workflow" report)
if [ -z "$make_vet" ]; then
  fail "read no vet tag sets out of $makefile"
elif [ -z "$workflow_vet" ]; then
  fail "read no vet tag sets out of $workflow"
elif [ "$make_vet" != "$workflow_vet" ]; then
  fail "vet tag sets differ between $makefile and $workflow"
  printf '%s\n' "$make_vet" | sed 's/^/  make: /' >&2
  printf '%s\n' "$workflow_vet" | sed 's/^/  ci:   /' >&2
fi

make_lint=$(lint_globs "$makefile")
workflow_lint=$(lint_globs "$workflow")
if [ -z "$make_lint" ]; then
  fail "read no frontend lint targets out of $makefile"
elif [ -z "$workflow_lint" ]; then
  fail "read no frontend lint targets out of $workflow"
elif [ "$make_lint" != "$workflow_lint" ]; then
  fail "frontend lint targets differ between $makefile and $workflow"
  printf '%s\n' "$make_lint" | sed 's/^/  make: /' >&2
  printf '%s\n' "$workflow_lint" | sed 's/^/  ci:   /' >&2
fi

[ "$status" -eq 0 ] && echo "check-gate-contract: verify prerequisites, CI jobs, and contexts in the committed .github/rulesets/main.json align"
exit "$status"
