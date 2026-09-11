#!/bin/sh
# Runs every `make verify` prerequisite except those owned by sibling CI jobs.
# The skip set is read from .github/gate-contract.json so a second source of
# truth cannot drift from the contract check.
set -eu
makefile="${1:-Makefile}"
contract="${2:-.github/gate-contract.json}"

command -v jq >/dev/null || { echo "run-verify-ci: jq is required" >&2; exit 1; }

skips=$(jq -r '.ci_jobs[] | select(.owns != null) | .owns[]' "$contract" | LC_ALL=C sort -u)
[ -n "$skips" ] || { echo "run-verify-ci: read no CI-owned prerequisites out of $contract" >&2; exit 1; }

prereqs=$(awk '/^verify:/ { for (i = 2; i <= NF; i++) print $i }' "$makefile")
[ -n "$prereqs" ] || { echo "run-verify-ci: read no verify prerequisites out of $makefile" >&2; exit 1; }

for prereq in $prereqs; do
  if printf '%s\n' "$skips" | grep -qx "$prereq"; then
    continue
  fi
  make --no-print-directory "$prereq"
done
