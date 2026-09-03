#!/bin/sh
# Every tool a Makefile gate requires must reach CI at the same pinned version:
# the workflow's env must carry the pin and an install step must use it. A gate
# that gained a tool locally otherwise fails only on the runner.
set -eu
makefile="${1:-Makefile}"
workflow="${2:-.github/workflows/ci.yml}"
status=0
names=$(grep -o 'require-go-tool,[^,]*,[^,]*,[$]([A-Z_]*)' "$makefile" | awk -F, '{print $2 ":" $4}' | sed 's/[$](//; s/)//' | sort -u)
[ -n "$names" ] || { echo "check-ci-tools: no require-go-tool call found in $makefile" >&2; exit 1; }
for pair in $names; do
  tool=${pair%%:*}; var=${pair#*:}
  pin=$(awk -v v="$var" '$1 == v && $2 == ":=" { print $3 }' "$makefile")
  [ -n "$pin" ] || { echo "check-ci-tools: $var is not pinned in $makefile" >&2; status=1; continue; }
  ci=$(awk -v v="$var:" '$1 == v { print $2 }' "$workflow")
  if [ -z "$ci" ]; then
    echo "check-ci-tools: $tool: $workflow does not pin $var (Makefile has $pin)" >&2; status=1; continue
  fi
  if [ "$ci" != "$pin" ]; then
    echo "check-ci-tools: $tool: $workflow pins $var at $ci, $makefile at $pin" >&2; status=1; continue
  fi
  grep -q "\${$var}" "$workflow" || { echo "check-ci-tools: $tool: no install step in $workflow uses \${$var}" >&2; status=1; }
done
[ "$status" -eq 0 ] && echo "check-ci-tools: every required tool is pinned and installed by CI"
exit "$status"
