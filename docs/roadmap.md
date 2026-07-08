# kurodo Roadmap — the blueprint after the judge face

> What this document is: the sequencing and design view of everything that remains,
> so any future session can pick up a work item with its goal, shape, UI surface,
> and acceptance already thought through. Scope ruling (D37): every remaining face
> ships; ordering below is by dependency and leverage, and it is a suggestion, not
> a milestone fence. Decisions D30–D37 carry the reasoning. Acceptance lives in
> spec.md where a face already has a section; a face without one gets a plan
> document (the judge-plan pattern) before build, and that plan carries its
> acceptance. This document was adversarially reviewed on 2026-07-05 (four
> independent lenses); the corrections are folded in below.

## 0. Where we are

- **Done and merged** (as of 2026-07-06): reading face + redesign, search face
  (lexical engine + `/search` page + the ⌘K shell — the panel's *content* is
  item 5 below), C syllabus, F five lesson interactions, E reports (sandboxed,
  D30), the judge face in full (all four stages: rules, formats, CLI dispatch,
  coverage/exists, gating, exit codes — byte-compat pinned by golden fixtures +
  real-vault sandwiches), the wall-lock sweep (D39 included), the quality rails
  (pinned toolchain, assets-drift, frontend lint, loopback e2e), and the fuzz
  pack (five targets, rescanner timing, benchmarks, fuzz-smoke job). The four
  cron consumers already run on `kurodo check` (switched 2026-07-05, with
  rollback backups).
- **Done** (2026-07-07): the differential fuzz campaign (judge-plan §13) ran to
  its completion bar with zero unexplained divergence; kura was declared retired
  (D43) and the conformance scaffolding was deleted, the goldens kept.
- **Done** (2026-07-08): the sidebar wayfinding rebuild (PR #25) and the
  experience batch (PR #27 — view transitions, the smoothness inventory, seal
  feedback, disclosure persistence, the search shell, the rail redesign), which
  also fixed the concept-sheet-over-TOC and search-layout bugs. **Next is the
  rename to yomihon (D44, `rename-plan.md`)** — a solo sweep before the next
  feature branch.
- **The two retirement gates** (evidence-based, D11; refined by D38/D40),
  stated precisely:
  - **kura gate — met; declared retired 2026-07-07 (D43)** = spec §5 acceptance
    (all merged, consumers switched) plus the differential campaign completion
    bar (judge-plan §13), which the declaration cited.
  - **yomihon gate — closed (D40)**: the engineering item is merged; the
    parity and two-week observation items are waived. Retirement is effective
    on Koopa's declaration alone. **Export stays excluded (D38)** — nothing
    consumes yomihon's SSG output.

## 1. Sequence

The PR-granular execution view of this table — roles, briefing protocol, and
acceptance per unit — is `program.md`.

| # | Work item | Why this position | Decision(s) |
|---|---|---|---|
| 1 | **A: judge Stage 4** (merged) | Produced the kura-gate evidence; the four external pipelines now run on it | judge-plan, D30 |
| 2 | **I: wall-lock sweep** | Retirement declarations require the four walls as test locks; also locks D30's three sandbox mechanisms | D30 |
| 3 | **Quality rails** (parallel with I) | CI + linters + fuzz pack, per D36 "right after the A face, alongside the I sweep"; the switchover benefits from CI existing | D36 |
| 4 | **kura retirement (done, D43)** | The switchover executed 2026-07-05 (ahead of the originally planned campaign-first order — a disclosed deviation, safe because the switched surfaces were triple-verified and the wrapper backups keep a rollback path). The differential campaign then ran to its completion bar with zero unexplained divergence, kura was declared retired 2026-07-07, and the §13 cleanup checklist was executed. judge-plan §13 remains the authoritative record | D36, D40, D43 |
| 4b | **Reading-surface UX repairs** (parallel with anything) | Daily use moved to kurodo outright (D40), so reading-surface pain is paid every day; the mechanical repairs need no ruling, the taste batch waits for Koopa's accumulated pain list — both in §5b | D40 |
| 5 | **B: search panel, lexical then hybrid** | ⌘K content over the existing shell first; then Gemini embeddings + RRF fusion (key arrives at build start). Needs a **B plan doc** before the hybrid half — its required contents are listed in §5a | D31, D32 |
| 6 | **H: agent toolbox** | Graph verbs + whole-graph export + frontmatter query; cheap, unlocks agents and dreaming. Needs an **H plan doc** — output contracts are the point (§5a) | D33 |
| 7 | **D: Home = the adjudication cockpit** | See §3 — the face that scales Koopa's throughput when agents write faster than he reads. Needs a **D plan doc**; must reconcile with spec §2's four home blocks (see §3) | D26, D35, D37 |
| 8 | **G: export** | Absorbs yomihon's SSG mode on its own schedule; excluded from the yomihon gate (D38) | spec §6, D38 |
| 9 | **Dreaming pipeline + adjudication inbox** | Agent-side consumer of 5/6; the inbox UI lands in the cockpit. **Honesty note**: the accept→apply loop is closed only by the seal-applies-patch ruling (§3) — until Koopa makes it, proposals are read-only reports and acting on an acceptance is out-of-band | D35 |

## 2. Capability ↔ UI mapping (kurodo has a UI; nothing ships CLI-only unless it is agent-only)

| Capability | Agent surface (CLI) | Human surface (UI) |
|---|---|---|
| Hybrid search | `kurodo search` (see degraded-mode rules, §4a) | ⌘K panel + `/search` page (B) |
| Relation queries | `kurodo graph backlinks/neighbors/path/related`, `graph export` — `related` merges the structural and semantic channels in **one call** (it reads the embedding cache; without it, structural-only plus a warning) | Backlinks / related panel in the reading page's right column; graph view later (H → D) |
| Diagnostics | `kurodo check` (JSONL/human/md) | Note-page diagnostics column (exists); diagnostics index page (parked in D26, lands with the cockpit) |
| Coverage | `kurodo coverage` | Cockpit tile (domains / pending / orphans) |
| Status flow | — (the write is human-only, wall 1) | Lifecycle queues + per-note seal (D26/D27); cockpit queue flow (§3) |
| Dreaming proposals | Agent writes report files | Reports face today; adjudication inbox in the cockpit (D35, §3) |

**Standing contract rule (D37)**: every agent-facing CLI output is a frozen
public interface (D14's principle, generalized): its JSON field set, ordering,
and exit codes are specified in the owning face's plan doc *before* build and
pinned by golden tests, exactly as the judge face's were. The system's own
history is the warning — a cron greps `"severity":"warn"` out of check's JSONL;
the next consumer will grep whatever B and H emit.

## 3. The adjudication cockpit (D face, designed 2026-07-05)

The scenario that defines it (Koopa): agents write many documents; reviewing,
reading, learning, and adjudicating them must not mean opening `.md` files one
by one and hand-editing status. Everything flows through kurodo.

**Relation to spec §2**: the cockpit becomes the landing surface, leading with
the status queues (D26's axis promoted from sidebar to landing page); spec §2's
four stable home blocks (domain MOC entries, cross-domain boards, reports,
folders) are not deleted — they become cockpit content below/beside the queues.
The exact layout is the D plan doc's job; what is fixed here is: queues lead,
the four blocks survive, nothing hardcodes what the toml can express (wall 3).

- **Queues, not folders**: each reviewable status is a queue with live counts;
  entering a queue gives a next/previous progression through its notes — read,
  decide, move on, without returning to a list. The D plan doc must pin: the
  ordering *within* a queue (default: oldest-unread first; overridable), the
  fate of skipped notes (stay in place, surfaced again next session — skip is
  not a state write), and whether queue position survives a session (it does,
  via the reading-state map).
- **Decide in place**: every legal transition is available on the note page as
  today (quiet one-press secondary forms; `ready` keeps the press-and-hold seal
  ritual, D27). Same single form POST, same state machine, same
  one-transition-one-commit (wall 1). **Spec §4 touch (small, explicit)**: the
  PRG redirect target today is the note page itself; queue flow needs an
  optional queue-aware redirect (decide → 303 to the next queue item). Same PRG
  semantics, parameterized target — a spec §4 amendment to record when D builds.
- **Reading state is primary local state, not derived data**: the
  read/last-visited map cannot be rebuilt from the vault or git — it derives
  from Koopa's behavior. Deleting it loses it (D06 does *not* apply; §6 below
  is explicit). v1 keeps it as a small local file, backed up by whatever backs
  up the machine; losing it resets read-tracking, which is annoying but not
  data loss in the vault sense — recorded as an accepted cost. It is keyed by
  note path and stores a **content hash of the note body at last read** — a
  content hash, not a git commit hash, so: kurodo's own status-only commits do
  not false-flag a re-read (frontmatter is excluded from the hash), uncommitted
  agent edits *are* detected (the scanner sees content, not commits), and a
  renamed note loses its read state (accepted, noted).
- **Diff-aware re-review**: a note whose body hash changed since last read
  surfaces "what changed" (rendered diff against the stored last-read state —
  which requires storing enough to diff: v1 stores the prior body text
  compressed, bounded by a size cap; the D plan doc sizes this).
- **Summary-first triage**: an agent-written abstract, when present, renders
  above the fold so a keep/park decision is possible without entering the full
  text.
- **Inbox for proposals**: proposals live under `System/` (outside the note
  scan boundary, no status frontmatter), so the inbox queue is **not** a status
  queue — its source is the reports listing (nav already indexes
  `System/reports/`), plus a pending/processed convention the dreaming pipeline
  owns (e.g. processed proposals move to a subfolder — agent-side, not a
  kurodo write). The D plan doc pins the convention.
- **The accept dead-end, named**: today, accepting a proposal has no mechanical
  path — kurodo cannot persist the acceptance (wall 1) and the agent cannot
  read Koopa's mind. Until the ruling below, "accept" means Koopa acts on it
  out-of-band (tells the agent, or edits himself). This is a known v1 limit,
  not an oversight.
- **A named future adjudication (not smuggled in)**: the shape that closes the
  loop while preserving the human terminal is **seal-applies-patch**: the agent
  precomputes the exact patch, Koopa seals it, kurodo applies it mechanically
  in one commit. That amends **wall 1** (today: one field, status only) **and
  wall 4** (today: kurodo never modifies note content — a mechanical
  application of a human-sealed patch is still kurodo writing content). Both
  walls, one ruling, parked for Koopa — the pressure will come; it must arrive
  as a ruling, not as an implementation detail.
- **Agent write discipline (operational prerequisite)**: the status flip's
  preflight 409s on a dirty file; a cron fleet that writes without committing
  stalls every adjudication behind it. Rule: vault-writing agents commit their
  writes (their own identity); kurodo's 409 stands as the safety net, not the
  norm. Recorded here because the cockpit's throughput assumes it.

## 4. Escalation ladders (upgrade by trigger, not by taste — D31)

- **Vector store**: rung 1 = in-process matrix + content-hash cache file.
  **Rung 1→2 trigger** (this was missing and matters more than rung 3): ~10⁵
  chunks *or* p95 exact-scan latency > ~100 ms — at 3072d float32, 10⁵ chunks
  is already ~1.2 GB resident and ~10⁸ multiply-adds per query; brute force
  quietly expires well before the old headline trigger. Rung 2 = sqlite-vec
  (single file, serverless). **Rung 2→3 (pgvector)**: corpus beyond the vault
  (dreaming ingesting repos/clippings/logs), or ANN + metadata filtering
  combined, or multi-process writers. The store sits behind a narrow interface
  (put / get / top-k) from day one; fusion, CLI, and UI do not change across
  rungs.
- **Graph**: in-process index + whole-graph export → embedded store → graph
  server. Triggers: ~10⁵ notes; transactional multi-writer mutation; at-scale
  graph algorithms; cross-corpus relation queries (D33). **Export size,
  honestly**: today the full graph in path-keyed JSON is low hundreds of KB
  (not "tens") — fine for one call and even for an LLM context; at ~10⁴ notes
  it is MBs and stops fitting in a context window, at which point the *verbs*
  (bounded queries) are the agent interface and export remains a tooling
  convenience. The verbs are therefore the contract-bearing surface, not the
  export.
- **Search persistence**: in-memory rebuild → SQLite FTS, only if a
  frequently-invoked search CLI makes per-invocation rescans painful (D24).
- **Snapshot rescan**: the ~2s full mtime walk has its own trigger (~10⁴
  files, where the ≤3s freshness bound breaks) — and under any sustained
  agent-writing rate it fires *before* the vector rungs. It is listed here so
  the incremental-scan work is claimed by a trigger, not forgotten.
- **Not on any ladder**: Rust (no capability Go lacks here — the judge face
  proved byte-level parity with serde/yaml-rust/pulldown-cmark from Go); MCP
  (D34, CLI-first with a recorded reversal condition); a second web framework
  or client framework (native-web-first stands).

### 4a. Hybrid degraded modes (ruled now, because every surface needs them)

The semantic channel depends on a network API for *both* indexing and query
embedding; the lexical channel never does. spec §0.1's availability invariant
("search is as available as reading") therefore binds the **lexical** channel
only, and the design keeps semantic strictly an enhancement layer:

- **⌘K / `/search`**: embedder unreachable → lexical results with a visible
  "semantic offline" indicator. Never blank, never blocking.
- **`kurodo search` CLI**: lexical by default; `--semantic` adds the hybrid
  channel and **fails loudly** (nonzero exit, stderr says why) when the cache
  is cold or the API is unreachable — an agent must never get silently
  different result sets with exit 0. A `--semantic=best-effort` mode may exist
  for interactive use; the default for automation is deterministic.
- **Cache freshness has one owner**: the serve-process scanner embeds
  incrementally (content-hash diff → API call → cache write). The snapshot
  swap is **not** blocked on embedding: the swap carries fresh
  lexical/graph/nav immediately, and vectors for changed notes update
  asynchronously (a changed-note's stale vector is masked from semantic
  results until refreshed — stale-masking, not stale-serving). The CLI never
  writes the cache; cold cache + no serve process = semantic unavailable
  (loud, per above). Cron pipelines use lexical/judge surfaces and are
  unaffected.
- **Model/dimension swap is an epoch cutover, not an in-place migration**:
  vectors from different models are incomparable, so (model, dim) versioning
  *detects* mismatch; the swap mechanism is embed-new-epoch-in-background,
  flip when complete, delete old. Cost reality: the whole vault today is
  ~2–4M tokens ≈ under a dollar to re-embed; even at 10⁵ chunks the cost is
  dollars — the real constraint is wall-clock under rate limits, which the
  background epoch absorbs. Chunk inputs are bounded to the model's input
  token limit (long sections split, recorded in the B plan doc).

## 5. Quality rails (D36, work items not advice)

- **CI (GitHub Actions), jobs by code area**: lint-go · test-go(-race) ·
  fuzz-smoke (30s/target) · assets-drift (templ + css regeneration must diff
  clean) · lint-frontend (Biome for the single JS file, Stylelint for
  input.css) · build · e2e-smoke (boot server on a fixture vault, assert key
  pages; startup scan is synchronous, so no race with the rescanner).
  **Posture note**: Biome/Stylelint/axe-core run in CI only — the *build*
  stays Node-free (the stack fact is about the product, not the CI runner).
  Reference-binary conformance tests skip in CI by design; golden fixtures
  carry byte-compat there.
- **Fuzz pack** (targets chosen by where the judge build actually bled):
  splitFrontmatter, parseNote/scalar resolution, wikilink + pathref extraction,
  WriteJSONL invariants, stripTarget.
- **synctest** for the D25 rescanner timing; **benchmarks** (scan/rebuild,
  search, full check) under benchstat discipline, smoke-only in CI.
- **Differential fuzz campaign** (one-off, before the scaffolding is deleted —
  sequencing pinned in §1 item 4): the honest cost is the **generator**, not
  the runner — random bytes only exercise the bad-YAML path; reaching the
  schema/graph rules needs near-valid frontmatter against vault-schema.toml
  plus cross-note link topology (collisions, NFC variants, ambiguous targets).
  Budget the generator as the bulk of the campaign.
- **Retrieval quality is measured, not assumed**: the B plan doc includes a
  small pinned eval set (~30–50 real queries, CJK-heavy, with expected-note
  judgments) run as a test — not to gate CI on a score, but so a model swap or
  chunking change shows its relevance diff instead of relying on Koopa's
  gut-feel that "search got worse".

## 5a. Plan-doc obligations (the judge-plan pattern, generalized)

Before building items 5/6/7, a plan doc per face, carrying at minimum:
- **B**: chunking rules (heading-based, token bound, fences/frontmatter
  handling — state the chunks-per-note assumption; measurement says ~10–15/note
  on today's vault, so ~5–7×10³ chunks now, ~2–3×10⁵ at 18k notes); cache file
  format versioned by (model, dim); RRF specifics (k, per-channel depths,
  chunk→note aggregation); the degraded-mode matrix of §4a as acceptance
  cases; the eval set; egress guard test (Diary/ never reaches the embedder —
  an I-style lock).
- **H**: the JSON contract per verb (fields, ordering, exit codes) + goldens;
  `related`'s fusion semantics; export format + its documented size envelope;
  a frontmatter-query verb with multi-key filtering (domain / status / topics /
  source_kind — reuse the shipped search filter grammar, no new language); a
  backlinks-aggregation verb (`--group-by topic` ranking by in-degree), which
  must define its edge sets explicitly: wikilink backlinks live in the graph
  index today, `based_on` provenance references do not — the verb states
  whether and how it resolves them.
- **D**: queue ordering/skip/read-vs-decided semantics; the spec §4 redirect
  amendment; reading-state file format + diff storage bound; inbox
  pending/processed convention; reconciliation with spec §2's four blocks;
  **per-note resume state** (scroll/anchor position keyed by content hash,
  so a long read survives an interruption — the gap the 2026-07-07 UX round
  named that no stateless surface can close).

**Humanities capability inputs (2026-07-07).** The vault grew a humanities
pillar: per-book and per-topic concepts under `domain: humanities`, a library
map (`type: moc`) tracking books with the reading state kept in a table
column — deliberately not frontmatter, so the write face never touches it —
and a new `status: published` for release. kurodo needs zero code for the new
enum values: domain and status flow from the contract file, and the write
face offers a transition the moment its lifecycle entry exists there (until
then it correctly offers nothing). What the new pillar actually asks of the
faces, by ownership:
- Structured queries (domain × status × topics) are **already shipped** in
  the lexical search grammar; the one delta is `source_kind` — an index
  field + filter key added when B or H builds (amending the index's
  only-what-search-reads list is part of that PR, not a side effect).
- Topic-depth ranking is the H backlinks-aggregation verb above.
- **Deferred, deliberately**: a "read but no review written" gap report
  would require parsing the library map's table cells (a state column plus
  per-row wikilink presence) — presentation-coupled and brittle. Not built
  until the manual answer fails: the map is small enough to read directly.
  Reopen trigger: the question recurs weekly or the map outgrows one
  screen. When reopened it lands as an H query-verb composition; a judge
  coverage rule stays the wrong home even after the judge face unfreezes,
  because the fragility is in the map format, not the rule engine.

## 5b. Reading-surface UX queue (from daily use, 2026-07-06)

D40 moved daily reading to kurodo outright, so reading-surface pain is paid
every day; this queue holds it. The diagnosis below was reproduced against the
real vault with headless screenshots before anything was queued.

**UX-A — mechanical repairs (no ruling needed; parallel with anything):**

1. The right rail occupies its grid column unconditionally, and the TOC is
   only its largest block — the rail also carries the status panel (**the
   write face**) and the diagnostics list, each with its own render condition.
   On a note with no headings the column reads as blank because a small card
   sits atop a tall empty track. The repair must never hide the status panel:
   collapse the column only when *all three* blocks render nothing (the
   template can compute that), zero JS. The sparser-but-present case (a lone
   status card on a tall empty column) is a layout-taste question and belongs
   to UX-B, not here.
2. `.k-prose table` has no horizontal-overflow guard, while code blocks and
   mermaid diagrams have one; a wide GFM table breaks the article column.
   Align tables with the existing overflow pattern.
3. `.k-title` and the metadata row carry no `overflow-wrap` guard against long
   unbroken titles and paths.
4. The status handler's success redirect concatenates the raw note path, while
   every rendered note link percent-escapes per segment; a note whose name
   carries `?` or `#` seals correctly but redirects to the wrong URL. Reuse
   the same per-segment escaping in the redirect (and its `?sealed=1` suffix),
   with tests covering `?`, `#`, CJK, and spaces.

**UX-B — designed in `ux-plan.md` (2026-07-06), awaiting Koopa's item-by-item
review (its §9 checklist), then one or two PRs per the approved plan.** The
questions this batch used to hold — TOC toggle, sidebar collapse/resize,
sidebar content order and wayfinding, landing page, motion — all have
positions taken there. Batch the implementation deliberately: piecemeal
layout changes are how the HTML-first discipline erodes.

**Cockpit priority signal:** whole-queue status management is the cockpit
(§3), designed and unbuilt. If that turns out to be the dominant daily pain,
pull item 7 ahead of item 5 — this sequence is pain-driven by design.

**Screenshot e2e / Playwright:** deferred until the reading surface
stabilizes — a baseline captured mid-repair is waste. The CI-only-Node
posture (D36) already answers the tooling question. No probe artifact is
checked in; the diagnosis above came from an ad-hoc headless-Chrome run whose
shape the future job re-creates: build, serve a fixture vault, screenshot
each face at two widths, compare against a committed baseline.

## 6. Standing rules that survive this roadmap

- The four walls (CLAUDE.md) stand; wall 2's text now carries D32's bounded
  egress explicitly (note content to the embedding API, never `Diary/`, D18) —
  the wall text and its authoritative reading are the same text again. Any
  future widening of egress is a new ruling.
- Every new capability gets both surfaces where it makes sense (§2) — the UI
  is not an afterthought to the CLI, nor vice versa.
- Derived data stays disposable (D06): embedding caches can be deleted and
  rebuilt. **Exception, named**: the cockpit's reading-state map is primary
  local state (it derives from Koopa's behavior, not the vault) — small,
  backed up with the machine, loseable at the cost of resetting read-tracking.
- Zero text coupling to the retired reference implementations in code and
  tests; docs that record the gates are the recorded exception.
- Scale outlook, honestly bounded: the vector/graph/rescan triggers above are
  sized against a hypothetical sustained agent-writing rate (~50 notes/day →
  ~18k notes/yr). That rate is only reachable if adjudication keeps up or a
  backlog is accepted — Koopa's reading bandwidth is the scarcest resource in
  the system, which is why the cockpit (§3) is a scaling work item, not a
  convenience. If the write rate stays human-scale, the triggers simply don't
  fire; nothing above is wasted either way because rung 1 is the cheapest
  implementation regardless.
