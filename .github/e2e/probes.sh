#!/usr/bin/env bash
# Drives every live-browser probe against a server that is already running (see
# serve.sh). One table below pairs each probe with the page it needs, so a probe
# added to the plain run cannot be forgotten by the mutation run.
#
#   probes.sh            # the locks themselves, all expected to pass
#   probes.sh --mutate   # each probe's self-tests, all expected to be caught
#
# The second form enforces the contract every probe owes. `MUTATE=list` names a
# probe's modes; running one of them injects the regression that probe exists to
# catch, and the run must then exit 1 and print "MUTATE-RESULT: caught <mode>".
# Exit 0 means the injected regression walked past the probe. Exit 2 means the
# mutation's needle matched nothing — which is how a self-test that quietly died
# against a rewritten source turns red here, rather than on the day a human next
# runs it by hand.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${YOMIHON_BASE:?probes.sh needs a running server; start it with serve.sh}"

# probe file | the page path it must be driven against
probes=(
  "brand-contract.mjs|/"
  "palette.mjs|/"
  "search-behavior.mjs|/"
  "filter-inline-reveal.mjs|/notes/Notes/alpha.md"
  "drawer-contract.mjs|/notes/Notes/alpha.md"
  "mermaid-fallback.mjs|/notes/Notes/alpha.md"
  "browser-boundary.mjs|/notes/Notes/browser-boundary.md"
  "article-language-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "right-rail-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "skip-link-contract.mjs|/notes/Notes/alpha.md"
  "contrast-contract.mjs|/notes/Notes/alpha.md"
  "sidebar-content.mjs|/notes/Notes/alpha.md"
  "study-path-branches.mjs|/notes/Notes/alpha.md"
  "instance-contract.mjs|/notes/Notes/alpha.md"
  "status-recovery-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "seal-select-guard.mjs|/notes/Writing/lessons/japanese/L01.md"
  "shortcut-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "keyboard-scroll.mjs|/notes/Writing/lessons/japanese/L01.md"
  "slot-announce-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
)

fail() {
  echo "FAIL probes.sh: $*" >&2
  exit 1
}

# The table has to name every probe file beside it, and no others. A probe wired
# to nothing would never run and nothing would say so; a table emptied by a bad
# edit would let both runs below announce success over no work at all, which is
# the silence this whole file exists to break.
[ "${#probes[@]}" -gt 0 ] || fail "the probe table is empty, so a run of it proves nothing"
listed=()
for entry in "${probes[@]}"; do
  # Without the separator both halves of the entry read as the whole of it, and
  # the run would drive a probe named after a page path.
  case "$entry" in
  *"|"*) ;;
  *) fail "the table entry ${entry} has no | between the probe and the page it needs" ;;
  esac
  listed+=("${entry%%|*}")
done
present=()
for file in "$here"/*.mjs; do
  [ -f "$file" ] || fail "no probe files sit beside this script"
  present+=("$(basename "$file")")
done
undriven="$(comm -23 <(printf '%s\n' "${present[@]}" | sort) <(printf '%s\n' "${listed[@]}" | sort) | tr '\n' ' ')"
absent="$(comm -13 <(printf '%s\n' "${present[@]}" | sort) <(printf '%s\n' "${listed[@]}" | sort) | tr '\n' ' ')"
[ -z "${undriven// /}" ] || fail "these probe files are driven by nothing: ${undriven}"
[ -z "${absent// /}" ] || fail "the table names probes that are not here: ${absent}"

run_locks() {
  local entry
  for entry in "${probes[@]}"; do
    PAGE_PATH="${entry#*|}" node "${here}/${entry%%|*}"
  done
  echo "probes.sh: every lock passed"
}

run_mutations() {
  local entry probe page modes mode out status
  for entry in "${probes[@]}"; do
    probe="${entry%%|*}"
    page="${entry#*|}"
    # A probe that cannot even name its modes — a refused table, a syntax error —
    # would otherwise stop this script with the interpreter's message and no clue
    # which probe it came from.
    if ! modes="$(MUTATE=list node "${here}/${probe}")"; then
      fail "${probe} could not name its mutation modes"
    fi
    [ -n "$modes" ] || fail "${probe} names no mutation modes, so nothing shows it can fail"
    while IFS= read -r mode; do
      [ -n "$mode" ] || continue
      echo "--- ${probe} MUTATE=${mode}"
      if out="$(PAGE_PATH="$page" MUTATE="$mode" node "${here}/${probe}")"; then status=0; else status=$?; fi
      printf '%s\n' "$out"
      [ "$status" -eq 1 ] || fail "${probe} MUTATE=${mode} exited ${status}, want 1 (0: the regression walked past the probe; 2: the mutation matched nothing)"
      # Whole-line, so the marker names this mode and no other: one mode's name
      # can be a prefix of another's, and a substring match would let the marker
      # for palette-fill-partial answer for palette-fill.
      printf '%s\n' "$out" | grep -qxF "MUTATE-RESULT: caught ${mode}" ||
        fail "${probe} MUTATE=${mode} exited 1 without a line reading exactly 'MUTATE-RESULT: caught ${mode}', so its exit code proves nothing"
    done <<<"$modes"
  done
  echo "probes.sh: every mutation was caught"
}

case "${1:-}" in
"") run_locks ;;
--mutate) run_mutations ;;
*)
  echo "usage: probes.sh [--mutate]" >&2
  exit 2
  ;;
esac
