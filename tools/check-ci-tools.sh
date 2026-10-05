#!/bin/sh
# The Makefile declares the gate's tool versions. Each must reach CI unchanged
# and have a workflow reference; a bootstrap that copies a pin must agree too.
set -eu
makefile="${1:-Makefile}"
workflow="${2:-.github/workflows/ci.yml}"
bootstrap="${3:-.cursor/install.sh}"
status=0
for input in "$makefile" "$workflow" "$bootstrap"; do
  [ -r "$input" ] || { echo "check-ci-tools: cannot read $input" >&2; exit 1; }
done
names=$(awk '$1 ~ /^[A-Z][A-Z0-9_]*_VERSION$/ && $2 == ":=" { print $1 }' "$makefile" | sort -u)
[ -n "$names" ] || { echo "check-ci-tools: no *_VERSION declaration found in $makefile" >&2; exit 1; }

normalize() {
  printf '%s\n' "$1" | sed "s/^['\"]//; s/['\"]$//; s/^v//"
}

checked=""
for var in $names; do
  tool=$(printf '%s' "${var%_VERSION}" | tr '[:upper:]_' '[:lower:]-')
  pin=$(awk -v v="$var" '$1 == v && $2 == ":=" { print $3 }' "$makefile")
  [ -n "$pin" ] || { echo "check-ci-tools: $var is not pinned in $makefile" >&2; status=1; continue; }
  want=$(normalize "$pin")
  ci=$(awk -v v="$var:" '$1 == v { print $2 }' "$workflow")
  if [ -z "$ci" ]; then
    echo "check-ci-tools: $tool: $workflow does not pin $var (Makefile has $pin)" >&2; status=1
  elif [ "$(normalize "$ci")" != "$want" ]; then
    echo "check-ci-tools: $tool: $workflow pins $var at $ci, $makefile at $pin" >&2; status=1
  fi
  grep -q "\${$var}" "$workflow" || { echo "check-ci-tools: $tool: no reference in $workflow uses \${$var}" >&2; status=1; }
  copies=$(awk -v v="$var" '{
    line = $0
    sub(/^[[:space:]]*/, "", line)
    sub(/^export[[:space:]]+/, "", line)
    if (index(line, v "=") == 1) {
      sub(/^[^=]*=/, "", line)
      sub(/[[:space:]]+#.*/, "", line)
      sub(/[[:space:]]*$/, "", line)
      print v "=" line
    }
  }' "$bootstrap")
  for assignment in $copies; do
    copy=${assignment#*=}
    if [ "$(normalize "$copy")" != "$want" ]; then
      echo "check-ci-tools: $tool: $bootstrap pins $var at $copy, $makefile at $pin" >&2; status=1
    fi
  done
  checked="$checked $var=$want"
done
[ "$status" -eq 0 ] && echo "check-ci-tools: every declared tool pin agrees with CI and bootstrap:$checked"
exit "$status"
