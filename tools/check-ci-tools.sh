#!/bin/sh
# The Makefile declares the gate's tool versions. Each must reach CI unchanged
# and have a workflow reference.
set -eu
makefile="${1:-Makefile}"
workflow="${2:-.github/workflows/ci.yml}"
status=0
for input in "$makefile" "$workflow"; do
  [ -r "$input" ] || { echo "check-ci-tools: cannot read $input" >&2; exit 1; }
done
# Refuse visible declarations the literal-pin comparison cannot interpret.
if awk '
function pin_name(line, name) {
  if (!match(line, /^[^[:space:]:=!?+]*_VERSION([[:space:]:=!?+]|$)/)) return ""
  name = substr(line, RSTART, RLENGTH)
  sub(/[[:space:]:=!?+].*$/, "", name)
  return name
}
function literal(value, quote) {
  quote = substr(value, 1, 1)
  if (quote == "\047" || quote == "\"") {
    if (length(value) < 3 || substr(value, length(value), 1) != quote) return 0
    value = substr(value, 2, length(value) - 2)
  }
  return value != "" && value !~ /[$\\\047"#]/
}
function refuse(name, reason) {
  print "check-ci-tools: " name ": " reason " tool pin declaration in " FILENAME
  failed = 1
}
{
  if (substr($0, 1, 1) == "\t") next
  line = $0
  sub(/^[[:space:]]*/, "", line)
  if (line == "" || line ~ /^#/) next
  sub(/[[:space:]]+#.*/, "", line)
  sub(/[[:space:]]*$/, "", line)
  count = split(line, fields, /[[:space:]]+/)
  inside = definitions > 0
  if (fields[1] == "define") definitions++
  else if (fields[1] == "endef") {
    if (definitions > 0) definitions--
    next
  }
  candidate = line
  canonical = 1
  while (candidate ~ /^(export|override|private|define|undefine)[[:space:]]+/) {
    sub(/^[^[:space:]]+[[:space:]]+/, "", candidate)
    canonical = 0
  }
  name = pin_name(candidate)
  if (name == "" && index(candidate, ":") > 0) {
    candidate = substr(candidate, index(candidate, ":") + 1)
    sub(/^[[:space:]]*/, "", candidate)
    name = pin_name(candidate)
    canonical = 0
  }
  if (name == "") next
  if (seen[name]++) refuse(name, "duplicate")
  if (inside || !canonical || name !~ /^[A-Z][A-Z0-9_]*_VERSION$/ || count != 3 || fields[1] != name || fields[2] != ":=" || !literal(fields[3])) {
    refuse(name, "unsupported")
  }
}
END { exit failed }
' "$makefile" >&2; then
  :
else
  status=1
fi
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
  checked="$checked $var=$want"
done
[ "$status" -eq 0 ] && echo "check-ci-tools: every declared tool pin agrees with CI:$checked"
exit "$status"
