#!/usr/bin/env bash
# Drives every live-browser probe against a server that is already running (see
# serve.sh). The table below pairs each probe with the page it needs; every
# probe is expected to pass.
#
# A probe may also carry self-test modes. Those run by hand against the same
# server, one at a time: `PAGE_PATH=<page> MUTATE=<mode> node --import
# ./support/browser-language.mjs <probe>`, with `MUTATE=list` naming them. The
# import is what pins the language the browser asks in; without it a probe is
# run in whichever language this machine's browser asks for.
set -euo pipefail

[ "$#" -eq 0 ] || { echo "usage: probes.sh" >&2; exit 2; }

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${YOMIHON_BASE:?probes.sh needs a running server; start it with serve.sh}"

# probe file | the page path it must be driven against
probes=(
  "page-change.mjs|/"
  "brand-contract.mjs|/"
  "search-live.mjs|/"
  "search-focus-restore.mjs|/notes/Notes/alpha.md"
  "filter-inline-reveal.mjs|/notes/Notes/cutover.md"
  "drawer-contract.mjs|/notes/Notes/cutover.md"
  "rail-foot.mjs|/notes/Notes/alpha.md"
  "mermaid-fallback.mjs|/notes/Notes/alpha.md"
  "mermaid-name.mjs|/notes/Notes/alpha.md"
  "browser-boundary.mjs|/notes/Notes/browser-boundary.md"
  "asset-identity.mjs|/notes/Notes/alpha.md"
  "image-size.mjs|/notes/Notes/image-sizes.md"
  "figure-boundary.mjs|/notes/Notes/figures.md"
  "figure-mat.mjs|/notes/Notes/figure-mat.md"
  "prose-overflow.mjs|/notes/Notes/browser-boundary.md"
  "article-language-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "language-scroll-restore.mjs|/notes/Notes/Glass%20Tide.md"
  "sidebar-contents.mjs|/notes/Writing/lessons/japanese/L01.md"
  "left-edge.mjs|/"
  "skip-link-contract.mjs|/notes/Notes/alpha.md"
  "contrast-contract.mjs|/notes/Notes/reading-fidelity.md"
  "print-librarian-chrome.mjs|/notes/Notes/Reading%20flow.md"
  "print-fold-open.mjs|/notes/Notes/reading-fidelity.md"
  "callout-fold-chevron.mjs|/notes/Notes/reading-fidelity.md"
  "heading-fragment.mjs|/notes/Notes/reading-fidelity.md"
  "preview-card.mjs|/notes/Notes/reading-fidelity.md"
  "preview-live.mjs|/notes/Notes/alpha.md"
  "task-list-marker.mjs|/notes/Notes/reading-fidelity.md"
  "code-ligatures.mjs|/notes/Notes/reading-fidelity.md"
  "code-copy.mjs|/notes/Notes/code-copy.md"
  "wide-blocks.mjs|/notes/Notes/wide-code.md"
  "tables-plates.mjs|/notes/Notes/tables-plates.md"
  "prose-type.mjs|/notes/Notes/prose-type.md"
  "note-end.mjs|/notes/Notes/prose-type.md"
  "sidebar-content.mjs|/notes/Notes/alpha.md"
  "rail-disclosure-state.mjs|/notes/Course/C02.md"
  "rail-filter-state.mjs|/notes/Course/C02.md"
  "study-path-branches.mjs|/notes/Notes/alpha.md"
  "instance-contract.mjs|/notes/Notes/alpha.md"
  "note-end-facts.mjs|/notes/Writing/lessons/japanese/L01.md"
  "shortcut-contract.mjs|/notes/Writing/lessons/long-rail/Rail%20lesson%2001.md"
  "rail-collapse.mjs|/notes/Writing/lessons/long-rail/Rail%20lesson%2002.md"
  "rail-initial-target.mjs|/notes/Writing/lessons/long-rail/Rail%20lesson%2030.md"
  "keyboard-scroll.mjs|/notes/Writing/lessons/japanese/L01.md"
  "slot-announce-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "read-aloud-run.mjs|/notes/Writing/lessons/japanese/L02.md"
  "read-aloud-status.mjs|/notes/Writing/lessons/japanese/Practice%20only.md"
  "read-aloud-languages.mjs|/notes/Writing/lessons/languages/Read%20aloud.md"
  "read-aloud-local-voices.mjs|/notes/Writing/lessons/languages/Read%20aloud.md"
  "speak-cells.mjs|/notes/Writing/lessons/languages/Read%20aloud.md"
  "listen-course.mjs|/listen/Maps/listen.md"
  "lesson-title-wrap.mjs|/courses/Maps/study.md"
  "branch-title-wrap.mjs|/courses/Maps/branches.md"
  "course-line.mjs|/notes/Course/C01.md"
  "book-rail-head.mjs|/notes/Course/C02.md"
  "dialog-exit.mjs|/notes/Writing/lessons/japanese/L01.md"
  "motion-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "overlay-commands.mjs|/notes/Writing/lessons/japanese/L01.md"
  "appearance-choice.mjs|/preferences"
  "header-row.mjs|/notes/Notes/alpha.md"
  "running-head.mjs|/notes/Notes/Back%20restore.md"
  "japanese-reading-face.mjs|/notes/Writing/lessons/japanese/L01.md"
  "preference-restore.mjs|/notes/Writing/lessons/japanese/L01.md"
  "preference-immediate.mjs|/preferences?from=%2Fnotes%2FNotes%2Falpha.md"
  "language-follows-browser.mjs|/"
  "preference-persistence.mjs|/preferences?from=%2Fnotes%2FNotes%2Falpha.md"
  "prefetch-stale-preference.mjs|/notes/Writing/lessons/japanese/L01.md"
  "freshness-visibility.mjs|/notes/Writing/lessons/japanese/L01.md"
  "health-table.mjs|/health"
  "source-limit.mjs|/health"
  "shell-columns.mjs|/health"
  "reports-shelf.mjs|/reports"
  # A month named outright rather than whichever one it is today, so what this
  # probe measures is the same measurement next month.
  "journal-month.mjs|/journal?month=2026-07"
  "reply-contract.mjs|/notes/Notes/reading-fidelity.md"
  "fragment-split-visibility.mjs|/notes/Writing/lessons/japanese/L01.md"
  "result-landing-visibility.mjs|/search?q=%22alpha%20beta%20gamma%22"
  "result-landing-cjk.mjs|/search?q=%E7%8D%A8%E8%A7%92%E7%8D%B8"
  "midword-landing.mjs|/search?q=lybdenum"
  "directive-edges.mjs|/search?q=nthanu"
  "range-end-landing.mjs|/search?q=%22alpha%20beta%20gamm%22"
  "result-landing-suffix.mjs|/search?q=%E3%82%8C%E3%80%81"
  "nothing-notice-width.mjs|/search?q=qqzzxxwwvvuuttssrrppoonnmmllkkjjiihhggffeeddccbbaa0011223344556677889900aabbccddeeffgghhiijjkkll"
  "search-overflow.mjs|/search?q=stoppedAt"
  "largest-text.mjs|/search?q=alpha"
  "shelf-order.mjs|/marks"
  "schema-notice-visibility.mjs|/notes/Notes/schema-notice-probe.md"
  "compare-columns.mjs|/compare/Notes/cutover.md?with=Notes%2Fcutover-zh-tw.md"
  "note-outline-position.mjs|/notes/Notes/Glass%20Tide.md"
  "source-location-round-trip.mjs|/notes/Notes/source-locations-claim.md"
  "source-preview.mjs|/notes/Notes/source-locations-claim.md"
  # A query broad enough that its answer runs past one page, which is what
  # gives this one a way on to follow. It drives the findings table itself.
  "question-marks.mjs|/notes/Writing/lessons/japanese/L01.md"
  "concept-sheet.mjs|/notes/Writing/lessons/japanese/L01.md"
  "reading-without-interaction.mjs|/notes/Notes/Reading%20flow.md"
  "section-marks.mjs|/notes/Notes/Reading%20flow.md"
  "return-from-a-detour.mjs|/notes/Notes/Reading%20flow.md"
  "back-scroll-restore.mjs|/notes/Notes/Back%20restore.md"
  "position-return.mjs|/notes/Notes/Back%20restore.md"
  "position-first-frame.mjs|/notes/Notes/Back%20restore.md"
  "position-fragment.mjs|/notes/Notes/Back%20restore.md"
  "position-gone.mjs|/notes/Notes/Back%20restore.md"
  "position-back.mjs|/notes/Notes/Back%20restore.md"
  "position-leave.mjs|/notes/Notes/Back%20restore.md"
  "position-off.mjs|/preferences"
  "position-cap.mjs|/notes/Notes/Back%20restore.md"
  "copy-path.mjs|/notes/Notes/Back%20restore.md"
  "current-section.mjs|/notes/Notes/Back%20restore.md"
  "reading-face.mjs|/notes/Notes/reading-fidelity.md"
  "reading-scale.mjs|/notes/Notes/reading-scale.md"
  "ruby-selection.mjs|/notes/Notes/ruby-selection.md"
  "a11y-audit.mjs|/notes/Notes/reading-fidelity.md"
  # Last, and they have to stay last: these keep a reading place, and from then
  # on every desk the run draws carries a row offering it back, and the course
  # holding the marked lesson offers to go back to it. A probe that reads
  # either would meet a page the fixture alone does not explain.
  "course-cover.mjs|/courses/Maps/branches.md"
  "reader-mark.mjs|/notes/Notes/reading-fidelity.md"
  "bookmark-size.mjs|/"
  "bookmark-first-frame.mjs|/"
  "bookmark-after-back.mjs|/"
  "bookmark-return.mjs|/"
  "reading-furniture.mjs|/notes/Notes/Reading%20flow.md"
)

# The probes above that leave a kept reading place behind, in the order the
# table has to end with. Each of them sets its own place rather than assuming
# the file is empty, so they may follow each other; nothing else may follow
# them.
leaves_a_place=(
  "course-cover.mjs"
  "reader-mark.mjs"
  "bookmark-size.mjs"
  "bookmark-first-frame.mjs"
  "bookmark-after-back.mjs"
  "bookmark-return.mjs"
  "reading-furniture.mjs"
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

# The comment beside them says these have to be last; this is what holds them
# there. From the moment one runs, a reading place is kept, and every desk the
# rest of the run would draw carries a row offering that place back — a page
# the fixture alone does not account for. Moving one up the table would make
# some other probe fail on state it left behind, which is a long way from where
# the mistake was made.
tail_start=$((${#listed[@]} - ${#leaves_a_place[@]}))
[ "$tail_start" -ge 0 ] ||
  fail "the table lists fewer probes than the ${#leaves_a_place[@]} that leave a kept reading place"
for i in "${!leaves_a_place[@]}"; do
  at="${listed[$((tail_start + i))]}"
  [ "$at" = "${leaves_a_place[$i]}" ] ||
    fail "${leaves_a_place[$i]} leaves a kept reading place behind and has to sit among the last ${#leaves_a_place[@]} probes, but position $((tail_start + i + 1)) holds ${at}"
done

# Bash 3.2 with nounset refuses expansion of an empty array. Keep the count
# separately and expand the ordered failure records only when it is positive.
failures=()
failure_count=0

record_failure() {
  failures+=("$*")
  failure_count=$((failure_count + 1))
}

finish_run() {
  if [ "$failure_count" -gt 0 ]; then
    printf 'FAIL probes.sh: %s failure(s):\n' "$failure_count" >&2
    printf '  %s\n' "${failures[@]}" >&2
    return 1
  fi
  echo "probes.sh: $1"
}

run_locks() {
  local entry status
  for entry in "${probes[@]}"; do
    if MUTATE='' PAGE_PATH="${entry#*|}" node --import "${here}/support/browser-language.mjs" "${here}/${entry%%|*}"; then
      status=0
    else
      status=$?
    fi
    if [ "$status" -ne 0 ]; then
      record_failure "${entry%%|*} plain exited ${status}, want 0"
    fi
  done
  finish_run "every lock passed"
}

run_locks
