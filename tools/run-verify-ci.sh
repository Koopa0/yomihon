#!/bin/sh
# Runs every `make verify` prerequisite except those a sibling job in
# .github/workflows/ci.yml already runs, so CI runs each stage once. A stage
# that moves into a job of its own joins this list in the same change.
set -eu
makefile="${1:-Makefile}"
owned_by_sibling_jobs="vuln frontend-check e2e-http-check fuzz-smoke performance-smoke browser-check"

prereqs=$(awk '/^verify:/ { for (i = 2; i <= NF; i++) print $i }' "$makefile")
[ -n "$prereqs" ] || { echo "run-verify-ci: read no verify prerequisites out of $makefile" >&2; exit 1; }

for prereq in $prereqs; do
  case " $owned_by_sibling_jobs " in
  *" $prereq "*) continue ;;
  esac
  make --no-print-directory "$prereq"
done
