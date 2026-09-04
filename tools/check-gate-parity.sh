#!/bin/sh
# Two gates run over this repository — the Make target a person types and the
# workflow the runner drives — and where they disagree the weaker one is the
# one somebody trusts. Each check below compares a set, not a line, so a step
# reordered or reworded is not a failure and a step that scans less is.
#
# What it reads is what runs, as far as a text comparison can tell:
#   - a commented-out step is not a step, so comment lines are dropped;
#   - a workflow step carrying an `if:` may not run on any of the platforms in
#     the matrix, so it is reported rather than counted — a gate hidden behind
#     a condition is exactly the divergence this exists to catch;
#   - a Make target nobody depends on does not run either, so the targets the
#     comparison rests on are checked against `verify:`'s own prerequisites.
#
# What it still cannot tell: whether a step that does run reaches the files it
# names, and whether the workflow's own job matrix covers the platforms a
# reader assumes. The Go package list is deliberately not compared — the Make
# target narrows to the packages this repository owns and the workflow says
# ./..., a superset, so the runner is never the weaker of the two there.
set -eu
makefile="${1:-Makefile}"
workflow="${2:-.github/workflows/ci.yml}"
status=0

# Comment lines are not steps. A recipe line may carry a trailing comment, but
# a line whose first non-blank character is "#" runs nothing.
uncommented() {
  sed 's/^[[:space:]]*#.*$//' "$1"
}

# The tags `go vet` runs under, with "(none)" standing for an empty tag set so
# the comparison has a member to hold. A workflow step guarded by `if:` is
# reported instead of counted.
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
  ' | sort -u
}

# The arguments the frontend linter is pointed at.
lint_globs() {
  uncommented "$1" | awk '/biome lint/ { for (i = 1; i <= NF; i++) if ($i ~ /\*/) print $i }' | sort -u
}

# The Make targets the comparison above rests on have to be reachable from the
# gate a person types, or the local side is compared against something nobody
# runs.
verify_reaches() {
  awk '/^verify:/ { for (i = 2; i <= NF; i++) print $i }' "$makefile" | sort -u
}

compare() {
  what="$1"; left="$2"; right="$3"
  if [ -z "$left" ]; then
    echo "check-gate-parity: read no $what out of $makefile; the pattern has drifted from how it is written" >&2
    return 1
  fi
  if [ -z "$right" ]; then
    echo "check-gate-parity: read no $what out of $workflow; the pattern has drifted from how it is written" >&2
    return 1
  fi
  if [ "$left" != "$right" ]; then
    echo "check-gate-parity: the $what differ between $makefile and $workflow" >&2
    printf '%s\n' "$left" | sed 's/^/  make: /' >&2
    printf '%s\n' "$right" | sed 's/^/  ci:   /' >&2
    return 1
  fi
  return 0
}

compare "vet tag sets" "$(vet_tags "$makefile" count)" "$(vet_tags "$workflow" report)" || status=1
compare "frontend lint targets" "$(lint_globs "$makefile")" "$(lint_globs "$workflow")" || status=1

reached=$(verify_reaches)
for target in vet frontend-check; do
  if ! printf '%s\n' "$reached" | grep -qx "$target"; then
    echo "check-gate-parity: $makefile's verify target does not reach $target, so comparing it against $workflow compares something nobody runs" >&2
    status=1
  fi
done

[ "$status" -eq 0 ] && echo "check-gate-parity: the Make gate and the workflow scan the same ground"
exit "$status"
