#!/usr/bin/env bash
# Drives every live-browser probe against a server that is already running (see
# serve.sh). One table below pairs each probe with the page it needs, so a probe
# added to the plain run cannot be forgotten by the mutation run.
#
#   probes.sh            # the locks themselves, all expected to pass
#   probes.sh --mutate   # each probe's self-tests, all expected to be caught
#   probes.sh --mutate --shard N   # one of four balanced parts of that run
#
# The second form enforces the mutation contract of every probe outside the
# behavior_only classification. `MUTATE=list` names a
# probe's modes; running one of them injects the regression that probe exists to
# catch, and the run must then exit 1 and print "MUTATE-RESULT: caught <mode>".
# Exit 0 means the injected regression walked past the probe. Exit 2 means the
# mutation's needle matched nothing — which is how a self-test that quietly died
# against a rewritten source turns red here, rather than on the day a human next
# runs it by hand.
set -euo pipefail


usage() {
  echo "usage: probes.sh [--mutate [--shard 1|2|3|4]]" >&2
  exit 2
}

action=""
shard=""
case "$#" in
0) ;;
1) [ "$1" = --mutate ] || usage; action=--mutate ;;
3)
  [ "$1" = --mutate ] && [ "$2" = --shard ] || usage
  case "$3" in 1|2|3|4) shard="$3" ;; *) usage ;; esac
  action=--mutate
  ;;
*) usage ;;
esac

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${YOMIHON_BASE:?probes.sh needs a running server; start it with serve.sh}"

# probe file | the page path it must be driven against
probes=(
  "arrival-readiness.mjs|/"
  "brand-contract.mjs|/"
  "palette.mjs|/"
  "search-behavior.mjs|/"
  "search-keys.mjs|/"
  "search-focus-restore.mjs|/notes/Notes/alpha.md"
  "filter-inline-reveal.mjs|/notes/Notes/alpha.md"
  "drawer-contract.mjs|/notes/Notes/alpha.md"
  "rail-foot.mjs|/notes/Notes/alpha.md"
  "mermaid-fallback.mjs|/notes/Notes/alpha.md"
  "mermaid-name.mjs|/notes/Notes/alpha.md"
  "browser-boundary.mjs|/notes/Notes/browser-boundary.md"
  "asset-identity.mjs|/notes/Notes/alpha.md"
  "image-size.mjs|/notes/Notes/image-sizes.md"
  "prose-overflow.mjs|/notes/Notes/browser-boundary.md"
  "report-frame-contract.mjs|/reports/browser-boundary.html"
  "article-language-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "language-scroll-restore.mjs|/notes/Notes/Glass%20Tide.md"
  "right-rail-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "narrow-aids.mjs|/notes/Writing/lessons/japanese/L01.md"
  "skip-link-contract.mjs|/notes/Notes/alpha.md"
  "contrast-contract.mjs|/notes/Notes/reading-fidelity.md"
  "print-librarian-chrome.mjs|/notes/Notes/reading-fidelity.md"
  "print-fold-open.mjs|/notes/Notes/reading-fidelity.md"
  "heading-fragment.mjs|/notes/Notes/reading-fidelity.md"
  "preview-card.mjs|/notes/Notes/reading-fidelity.md"
  "preview-live.mjs|/notes/Notes/alpha.md"
  "task-list-marker.mjs|/notes/Notes/reading-fidelity.md"
  "code-ligatures.mjs|/notes/Notes/reading-fidelity.md"
  "code-copy.mjs|/notes/Notes/code-copy.md"
  "sidebar-content.mjs|/notes/Notes/alpha.md"
  "rail-disclosure-state.mjs|/notes/Course/C02.md"
  "rail-filter-state.mjs|/notes/Course/C02.md"
  "vault-sidebar.mjs|/search"
  "study-path-branches.mjs|/notes/Notes/alpha.md"
  "instance-contract.mjs|/notes/Notes/alpha.md"
  "note-head-facts.mjs|/notes/Writing/lessons/japanese/L01.md"
  "status-recovery-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "shortcut-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "rail-collapse.mjs|/notes/Notes/alpha.md"
  "rail-initial-target.mjs|/notes/Writing/lessons/long-rail/Rail%20lesson%2030.md"
  "keyboard-scroll.mjs|/notes/Writing/lessons/japanese/L01.md"
  "slot-announce-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "read-aloud-run.mjs|/notes/Writing/lessons/japanese/L02.md"
  "read-aloud-status.mjs|/notes/Writing/lessons/japanese/Practice%20only.md"
  "read-aloud-languages.mjs|/notes/Writing/lessons/languages/Read%20aloud.md"
  "read-aloud-local-voices.mjs|/notes/Writing/lessons/languages/Read%20aloud.md"
  "listen-course.mjs|/listen/Maps/listen.md"
  "lesson-title-wrap.mjs|/syllabus/Maps/study.md"
  "branch-title-wrap.mjs|/syllabus/Maps/branches.md"
  "course-line.mjs|/notes/Course/C01.md"
  "book-rail-head.mjs|/notes/Course/C02.md"
  "dialog-exit.mjs|/notes/Writing/lessons/japanese/L01.md"
  "motion-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "overlay-commands.mjs|/notes/Writing/lessons/japanese/L01.md"
  "theme-toggle-pressed.mjs|/notes/Notes/alpha.md"
  "header-fold.mjs|/notes/Notes/alpha.md"
  "reading-switch-label.mjs|/notes/Writing/lessons/japanese/L01.md"
  "japanese-reading-face.mjs|/notes/Writing/lessons/japanese/L01.md"
  "preference-restore.mjs|/notes/Notes/alpha.md"
  "preference-immediate.mjs|/preferences?from=%2Fnotes%2FNotes%2Falpha.md"
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
  "flip-receipt-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "reply-contract.mjs|/notes/Notes/reading-fidelity.md"
  "sealbar-flow-contract.mjs|/notes/Writing/lessons/japanese/L01.md"
  "status-and-fragment-visibility.mjs|/search?q=status%3Adraft"
  "result-landing-visibility.mjs|/search?q=%22alpha%20beta%20gamma%22"
  "result-landing-cjk.mjs|/search?q=%E7%8D%A8%E8%A7%92%E7%8D%B8"
  "midword-landing.mjs|/search?q=lybdenum"
  "directive-edges.mjs|/search?q=nthanu"
  "range-end-landing.mjs|/search?q=%22alpha%20beta%20gamm%22"
  "result-landing-suffix.mjs|/search?q=%E3%82%8C%E3%80%81"
  "nothing-notice-width.mjs|/search?q=qqzzxxwwvvuuttssrrppoonnmmllkkjjiihhggffeeddccbbaa0011223344556677889900aabbccddeeffgghhiijjkkll"
  "search-facets.mjs|/search?q=a"
  "search-overflow.mjs|/search?q=stoppedAt"
  "schema-notice-visibility.mjs|/notes/Notes/schema-notice-probe.md"
  "compare-columns.mjs|/compare/Notes/cutover.md?with=Notes%2Fcutover-zh-tw.md"
  "note-outline-position.mjs|/notes/Notes/Glass%20Tide.md"
  "source-location-round-trip.mjs|/notes/Notes/source-locations-claim.md"
  "source-preview.mjs|/notes/Notes/source-locations-claim.md"
  # A query broad enough that its answer runs past one page, which is what
  # gives this one a way on to follow. It drives the findings table itself.
  "pager-contract.mjs|/search?q=e"
  "uncertainty-marks.mjs|/notes/Writing/lessons/japanese/L01.md"
  "concept-sheet.mjs|/notes/Writing/lessons/japanese/L01.md"
  "reading-face.mjs|/notes/Notes/reading-fidelity.md"
  "reading-scale.mjs|/notes/Notes/reading-scale.md"
  "a11y-audit.mjs|/notes/Notes/reading-fidelity.md"
  # Last, and they have to stay last: these keep a reading place, and from then
  # on every desk the run draws carries a row offering it back, and the course
  # holding the marked lesson offers to go back to it. A probe that reads
  # either would meet a page the fixture alone does not explain.
  "course-cover.mjs|/syllabus/Maps/branches.md"
  "reader-mark.mjs|/notes/Notes/reading-fidelity.md"
)

# The audit's plain run includes its canary; mutation jobs retain the targeted
# probes and do not rediscover this broader development-only audit.
behavior_only=(
  "a11y-audit.mjs"
)

# The probes above that leave a kept reading place behind, in the order the
# table has to end with. Each of them sets its own place rather than assuming
# the file is empty, so they may follow each other; nothing else may follow
# them.
leaves_a_place=(
  "course-cover.mjs"
  "reader-mark.mjs"
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

# Behavior-only probes still run as locks, but cannot declare mutation work.
# Validate the whole classification before either kind of child is invoked.
for probe in ${behavior_only[@]+"${behavior_only[@]}"}; do
  registered=0
  for listed_probe in "${listed[@]}"; do
    [ "$probe" != "$listed_probe" ] || registered=1
  done
  [ "$registered" -eq 1 ] || fail "behavior_only names an unregistered probe: ${probe}"
done

is_behavior_only() {
  local probe
  for probe in ${behavior_only[@]+"${behavior_only[@]}"}; do
    if [ "$1" = "$probe" ]; then return 0; fi
  done
  return 1
}

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
    if MUTATE='' PAGE_PATH="${entry#*|}" node "${here}/${entry%%|*}"; then
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

# A child only proves its mutation when its real status is one and its whole
# stdout line names this exact mode. Failures accumulate instead of aborting.
run_mutation() {
  local probe="$1" page="$2" mode="$3" out status reason
  echo "--- ${probe} MUTATE=${mode}"
  if out="$(PAGE_PATH="$page" MUTATE="$mode" node "${here}/${probe}")"; then status=0; else status=$?; fi
  printf '%s\n' "$out"
  reason=""
  if [ "$status" -ne 1 ]; then
    reason="exited ${status}, want 1"
  fi
  # Whole-line, so the marker names this mode and no other: one mode's name
  # can be a prefix of another's, and a substring match would let the marker
  # for palette-fill-partial answer for palette-fill.
  if ! printf '%s\n' "$out" | grep -xF "MUTATE-RESULT: caught ${mode}" >/dev/null; then
    if [ -n "$reason" ]; then reason="${reason}; "; fi
    reason="${reason}missing exact stdout line 'MUTATE-RESULT: caught ${mode}'"
  fi
  if [ -n "$reason" ]; then
    record_failure "${probe} MUTATE=${mode}: ${reason}"
  fi
}

run_mutations() {
  local entry probe page modes mode status mode_count
  for entry in "${probes[@]}"; do
    probe="${entry%%|*}"
    page="${entry#*|}"
    if is_behavior_only "$probe"; then continue; fi
    # Discovery failure leaves this probe's modes unknowable, but the next
    # probe can still name and exercise its own modes.
    if modes="$(MUTATE=list node "${here}/${probe}")"; then
      status=0
    else
      status=$?
    fi
    if [ "$status" -ne 0 ]; then
      record_failure "${probe} MUTATE=list exited ${status}, cannot discover mutation modes"
      continue
    fi
    mode_count=0
    while IFS= read -r mode; do
      [ -n "$mode" ] || continue
      mode_count=$((mode_count + 1))
      run_mutation "$probe" "$page" "$mode"
    done <<<"$modes"
    if [ "$mode_count" -eq 0 ]; then
      record_failure "${probe} MUTATE=list names no runnable mutation modes"
    fi
  done
  finish_run "every mutation was caught"
}


# Discovery is complete before selection. Failed inventories still allow every
# later list and valid selected item to run, but prevent a successful verdict.
discover_shard_work() {
  local entry probe page modes status mode seen_count duplicate j
  work_probes=(); work_pages=(); work_modes=(); declarations=()
  work_count=0
  for entry in "${probes[@]}"; do
    probe="${entry%%|*}"; page="${entry#*|}"
    if is_behavior_only "$probe"; then continue; fi
    if modes="$(MUTATE=list node "${here}/${probe}")"; then status=0; else status=$?; fi
    if [ "$status" -ne 0 ]; then
      record_failure "${probe} MUTATE=list exited ${status}, cannot discover mutation modes"
      continue
    fi
    seen=(); seen_count=0
    while IFS= read -r mode; do
      [ -n "$mode" ] || continue
      duplicate=0
      for ((j=0; j<seen_count; j++)); do
        [ "${seen[$j]}" != "$mode" ] || duplicate=1
      done
      if [ "$duplicate" -eq 1 ]; then
        record_failure "${probe} MUTATE=list repeats mutation mode '${mode}'"
        continue
      fi
      seen+=("$mode"); seen_count=$((seen_count + 1))
      declarations+=("${probe}|${mode}")
      work_probes+=("$probe"); work_pages+=("$page"); work_modes+=("$mode")
      work_count=$((work_count + 1))
    done <<<"$modes"
    if [ "$seen_count" -eq 0 ]; then
      record_failure "${probe} MUTATE=list names no runnable mutation modes"
    fi
  done
}

# Tail modes prefill the fourth load; ordinary modes go to the least-loaded
# shard, with a lower shard number winning a tie. Ownership never reorders work.
assign_shard_work() {
  local i j probe tail_count=0 owner
  owners=(); loads=(0 0 0 0)
  for ((i=0; i<work_count; i++)); do
    for probe in "${leaves_a_place[@]}"; do
      if [ "${work_probes[$i]}" = "$probe" ]; then tail_count=$((tail_count + 1)); fi
    done
  done
  loads[3]="$tail_count"
  for ((i=0; i<work_count; i++)); do
    owner=0
    for probe in "${leaves_a_place[@]}"; do
      if [ "${work_probes[$i]}" = "$probe" ]; then owner=4; fi
    done
    if [ "$owner" -eq 0 ]; then
      owner=1
      for ((j=1; j<4; j++)); do
        if [ "${loads[$j]}" -lt "${loads[$((owner - 1))]}" ]; then owner=$((j + 1)); fi
      done
      loads[owner - 1]=$((loads[owner - 1] + 1))
    fi
    owners+=("$owner")
  done
}

# Each admitted identity/page stays at its discovery position, with exactly one
# canonical owner. Counts alone would overlook a dropped-and-duplicated item.
validate_shard_work() {
  local i probe identity entry page owner
  [ "$work_count" -gt 0 ] || { record_failure "mutation discovery produced no work"; return; }
  [ "${#declarations[@]}" -eq "$work_count" ] &&
    [ "${#work_probes[@]}" -eq "$work_count" ] &&
    [ "${#work_pages[@]}" -eq "$work_count" ] &&
    [ "${#work_modes[@]}" -eq "$work_count" ] &&
    [ "${#owners[@]}" -eq "$work_count" ] || fail "mutation partition lost or duplicated declared work"
  for ((i=0; i<work_count; i++)); do
    identity="${work_probes[$i]}|${work_modes[$i]}"
    [ "$identity" = "${declarations[$i]}" ] || fail "mutation partition changed declared identity ${declarations[$i]}"
    owner="${owners[$i]}"
    case "$owner" in 1|2|3|4) ;; *) fail "mutation partition has invalid owner ${owner} for ${identity}" ;; esac
    page=""
    for entry in "${probes[@]}"; do
      if [ "${entry%%|*}" = "${work_probes[$i]}" ]; then page="${entry#*|}"; break; fi
    done
    [ -n "$page" ] && [ "$page" = "${work_pages[$i]}" ] || fail "mutation partition changed page for ${identity}"
    for probe in "${leaves_a_place[@]}"; do
      if [ "${work_probes[$i]}" = "$probe" ] && [ "$owner" -ne 4 ]; then fail "kept-place mutation ${identity} belongs to shard 4"; fi
    done
  done
}

run_shard_mutations() {
  local i probe page mode selected_count=0
  discover_shard_work
  assign_shard_work
  validate_shard_work
  for ((i=0; i<work_count; i++)); do
    [ "${owners[$i]}" = "$shard" ] || continue
    probe="${work_probes[$i]}"; page="${work_pages[$i]}"; mode="${work_modes[$i]}"
    selected_count=$((selected_count + 1))
    run_mutation "$probe" "$page" "$mode"
  done
  if [ "$selected_count" -eq 0 ]; then record_failure "shard ${shard} selected no mutation work"; fi
  finish_run "every mutation was caught (shard ${shard}: ${selected_count} mode(s))"
}

case "$action" in
"") run_locks ;;
--mutate)
  if [ -n "$shard" ]; then run_shard_mutations; else run_mutations; fi
  ;;
esac
