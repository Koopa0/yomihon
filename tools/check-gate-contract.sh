#!/bin/sh
# Owns the mapping between three surfaces that must stay aligned:
#   1. the prerequisites reached by `make verify`;
#   2. the CI jobs that add evidence beyond that canonical gate;
#   3. the status contexts the protected `main` ruleset requires.
#
# A same-platform job that only reruns a verify prerequisite is a duplicate
# and fails here. A required context with no matching job, or a job with no
# matching required context, fails here too.
set -eu
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

[ -f "$contract" ] || fail "missing contract file $contract"
[ -f "$ruleset" ] || fail "missing ruleset file $ruleset"
command -v jq >/dev/null || fail "jq is required to read $contract and $ruleset"

reached=$(verify_prereqs)
[ -n "$reached" ] || fail "read no verify prerequisites out of $makefile"

jobs=$(ci_jobs)
[ -n "$jobs" ] || fail "read no jobs out of $workflow"

while IFS= read -r target; do
  [ -n "$target" ] || continue
  if ! printf '%s\n' "$reached" | grep -qx "$target"; then
    fail "$makefile verify does not reach $target, which a removed duplicate job used to own"
  fi
done <<EOF
$(jq -r '.required_verify_targets[]' "$contract")
EOF

while IFS= read -r removed; do
  [ -n "$removed" ] || continue
  if printf '%s\n' "$jobs" | grep -qx "$removed"; then
    fail "$workflow still defines duplicate job $removed; its behavior belongs under make verify"
  fi
done <<EOF
$(jq -r '.removed_duplicate_jobs[]' "$contract")
EOF

expected_jobs=$(jq -r '.canonical_job, .ci_only_jobs[].name' "$contract" | sorted_lines)
if ! same_sets "$expected_jobs" "$jobs"; then
  fail "CI jobs differ from the contract"
  printf '%s\n' "$expected_jobs" | sed 's/^/  expected: /' >&2
  printf '%s\n' "$jobs" | sed 's/^/  workflow: /' >&2
fi

contexts=$(ruleset_contexts)
[ -n "$contexts" ] || fail "read no required contexts out of $ruleset"

expected_contexts=$(
  canonical=$(jq -r '.canonical_job' "$contract")
  printf '%s\n' "$canonical"
  jq -r '.ci_only_jobs[] | if (.matrix | length) > 0 then .name as $job | .matrix[] | "\($job) (\(.))" else .name end' "$contract"
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
  fail "vet tag sets differ between $makefile and $workflow (portable-core must scan the same ground as verify)"
  printf '%s\n' "$make_vet" | sed 's/^/  make: /' >&2
  printf '%s\n' "$workflow_vet" | sed 's/^/  ci:   /' >&2
fi

[ "$status" -eq 0 ] && echo "check-gate-contract: verify prerequisites, CI-only jobs, and ruleset contexts align"
exit "$status"
