# yomihon Roadmap — the blueprint after the judge face

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
  pack (five targets, rescanner timing, benchmarks, the `fuzz` job). The four
  cron consumers already run on `yomihon check` (switched 2026-07-05, with
  rollback backups).
- **Done** (2026-07-07): the differential fuzz campaign (judge-plan §13) ran to
  its completion bar with zero unexplained divergence; kura was declared retired
  (D43) and the conformance scaffolding was deleted, the goldens kept.
- **Done** (2026-07-08): the sidebar wayfinding rebuild (PR #25) and the
  experience batch (PR #27 — view transitions, the smoothness inventory, seal
  feedback, disclosure persistence, the search shell, the rail redesign), which
  also fixed the concept-sheet-over-TOC and search-layout bugs. The coordinated
  rename to yomihon then completed and was accepted in PR #28 (D44).
- **The two retirement gates** (evidence-based, D11; refined by D38/D40),
  stated precisely:
  - **kura gate — met; declared retired 2026-07-07 (D43)** = spec §5 acceptance
    (all merged, consumers switched) plus the differential campaign completion
    bar (judge-plan §13), which the declaration cited.
  - **yomihon-dev gate — closed (D40)**: the engineering item is merged; the
    parity and two-week observation items are waived. Retirement is effective
    on Koopa's declaration alone. **Export stays excluded (D38)** — nothing
    consumes yomihon-dev's SSG output.

## 1. Sequence

The PR-granular execution view of this table — roles, briefing protocol, and
acceptance per unit — is `program.md`.

| # | Work item | Why this position | Decision(s) |
|---|---|---|---|
| 1 | **A: judge Stage 4** (merged) | Produced the kura-gate evidence; the four external pipelines now run on it | judge-plan, D30 |
| 2 | **I: wall-lock sweep** | Retirement declarations require the four walls as test locks; also locks D30's three sandbox mechanisms | D30 |
| 3 | **Quality rails** (parallel with I) | CI + linters + fuzz pack, per D36 "right after the A face, alongside the I sweep"; the switchover benefits from CI existing | D36 |
| 4 | **kura retirement (done, D43)** | The switchover executed 2026-07-05 (ahead of the originally planned campaign-first order — a disclosed deviation, safe because the switched surfaces were triple-verified and the wrapper backups keep a rollback path). The differential campaign then ran to its completion bar with zero unexplained divergence, kura was declared retired 2026-07-07, and the §13 cleanup checklist was executed. judge-plan §13 remains the authoritative record | D36, D40, D43 |
| 4b | **Reading-surface UX repairs** (parallel with anything) | Daily use moved to yomihon outright (D40), so reading-surface pain is paid every day; the mechanical repairs need no ruling, the taste batch waits for Koopa's accumulated pain list — both in §5b | D40 |
| 5 | **B: lexical search UI + agent hybrid CLI** | The shipped ⌘K and `/search` remain lexical-only, and `serve` has no semantic dependency. The privacy capability unit required by D50.10 precedes the CLI/agent hybrid implementation. A cold/incompatible corpus is built explicitly with `yomihon search-index build`; later compatible vault drift is reconciled by the explicit `search --semantic` action before it queries. D50.11/search-plan H12.5 records the optional BYOK boundary. Compilation and offline work require no key; Koopa supplies his paid-project key only at the live network-client step, while downstream users supply their own provider account/key and accept its terms. Needs a **B plan doc** before the hybrid half — its required contents are listed in §5a | D31, D32, D50.10, D50.11 |
| 6 | **H: agent toolbox** | Graph verbs + whole-graph export + frontmatter query; cheap, unlocks agents and dreaming. Needs an **H plan doc** — output contracts are the point (§5a) | D33 |
| 7 | **D: Home = the adjudication cockpit** | See §3 — the face that scales Koopa's throughput when agents write faster than he reads. Needs a **D plan doc**; must reconcile with spec §2's four home blocks (see §3) | D26, D35, D37 |
| 8 | **G: export** | Absorbs yomihon-dev's SSG mode on its own schedule; excluded from the yomihon-dev gate (D38) | spec §6, D38 |
| 9 | **Dreaming pipeline + adjudication inbox** | Agent-side consumer of 5/6; the inbox UI lands in the cockpit. **Honesty note**: the accept→apply loop is closed only by the seal-applies-patch ruling (§3) — until Koopa makes it, proposals are read-only reports and acting on an acceptance is out-of-band | D35 |

## 2. Capability ↔ UI mapping (yomihon has a UI; nothing ships CLI-only unless it is agent-only)

The current command inventory is `serve`, `check`, `coverage`, and `exists`.
Every other CLI spelling below remains planned until its owning face lands.

| Capability | Agent surface (CLI) | Human surface (UI) |
|---|---|---|
| Hybrid search | `yomihon search-index build` for an explicit cold/full build; `yomihon search --semantic` for self-refreshing compatible hybrid retrieval (see degraded-mode rules, §4a) | None in ordinary search; a future Related/Find-related surface requires a separate ruling |
| Relation queries | `yomihon graph backlinks/neighbors/path/related`, `graph export` — the agent `related` verb may merge structural and semantic channels under H's frozen contract | Backlinks / structural-neighbors panel in the reading page's right column; a human semantic or mixed Related surface requires a separate ruling; graph view later (H → D) |
| Diagnostics | `yomihon check` (JSONL/human/md) | Note-page diagnostics column (exists); diagnostics index page (parked in D26, lands with the cockpit) |
| Coverage | `yomihon coverage` | Cockpit tile (domains / pending / orphans) |
| Status flow | — (the write is human-only, wall 1) | Lifecycle queues + per-note seal (D26/D27); cockpit queue flow (§3) |
| Dreaming proposals | Agent writes report files | Reports face today; adjudication inbox in the cockpit (D35, §3) |
| Reading preferences | — (presentation is human-only, D48) | Aa popover or existing header toggles may suffice; a Settings page is allowed only when persistent preferences need grouping and explanation — pain-driven, unscheduled |

**Standing contract rule (D37)**: every agent-facing CLI output is a frozen
public interface (D14's principle, generalized): its JSON field set, ordering,
and exit codes are specified in the owning face's plan doc *before* build and
pinned by golden tests, exactly as the judge face's were. The system's own
history is the warning — a cron greps `"severity":"warn"` out of check's JSONL;
the next consumer will grep whatever B and H emit.

## 3. The adjudication cockpit (D face, designed 2026-07-05)

The scenario that defines it (Koopa): agents write many documents; reviewing,
reading, learning, and adjudicating them must not mean opening `.md` files one
by one and hand-editing status. Everything flows through yomihon.

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
  content hash, not a git commit hash, so: yomihon's own status-only commits do
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
  yomihon write). The D plan doc pins the convention.
- **The accept dead-end, named**: today, accepting a proposal has no mechanical
  path — yomihon cannot persist the acceptance (wall 1) and the agent cannot
  read Koopa's mind. Until the ruling below, "accept" means Koopa acts on it
  out-of-band (tells the agent, or edits himself). This is a known v1 limit,
  not an oversight.
- **A named future adjudication (not smuggled in)**: the shape that closes the
  loop while preserving the human terminal is **seal-applies-patch**: the agent
  precomputes the exact patch, Koopa seals it, yomihon applies it mechanically
  in one commit. That amends **wall 1** (today: one field, status only) **and
  wall 4** (today: yomihon never modifies note content — a mechanical
  application of a human-sealed patch is still yomihon writing content). Both
  walls, one ruling, parked for Koopa — the pressure will come; it must arrive
  as a ruling, not as an implementation detail.
- **Agent write discipline (operational prerequisite)**: the status flip's
  preflight 409s on a dirty file; a cron fleet that writes without committing
  stalls every adjudication behind it. Rule: vault-writing agents commit their
  writes (their own identity); yomihon's 409 stands as the safety net, not the
  norm. Recorded here because the cockpit's throughput assumes it.

## 4. Escalation ladders (upgrade by trigger, not by taste — D31)

- **Semantic storage/retrieval**: the current design is a local SQLite
  active/previous/staging generation store plus a per-command immutable
  in-memory exact-search index. Rung 1 requires fewer than 100,000 chunks and
  at most 1 GiB raw vector payload (`chunks × dimension × 4`); crossing either
  bound, or p95 exact top-k above ~100 ms, **opens a measured candidate
  evaluation; it does not preselect the
  winner**. Compare the current design with then-current embedded-vector
  candidates and PostgreSQL exact search on the same corpus, filters, crash
  cases, and SLO. PostgreSQL is adopted only when it owns a real server
  capability (shared remote access, independently operating writers, or
  database backup/replication) or when the embedded design misses its SLO and
  PostgreSQL passes correctness, privacy, resource, cold/warm latency, and
  operational-cost gates. Neon is the preferred managed PostgreSQL candidate
  if that gate opens, but any real-vault corpus- or query-derived transfer to a
  remote service—vectors, query vectors, filters, or identifiers, persisted or
  transient—needs an explicit egress ruling first. pgvector ANN is a
  **separate** promotion:
  PostgreSQL exact search must first miss the SLO, then ANN must meet the
  held-out recall floor, filter completeness, deterministic projection, and
  failure-mode requirements. Fusion and the agent CLI remain unchanged;
  ordinary UI search stays lexical-only and outside this ladder.
  The promotion benchmark is executable, not a prose comparison:
  - freeze one synthetic corpus, its exact filters, and recorded query vectors;
    run the embedded design and PostgreSQL exact scan against the same rows,
    with brute-force exact top-50 as the result oracle;
  - report build, bounded-drift, process-cold, and warm-query p50/p95/p99,
    peak RSS, steady/peak disk, crash recovery, and operational cost. PostgreSQL
    must preserve path/filter completeness and deterministic ordering and meet
    the predeclared ~100 ms p95 on the target scale before it can replace the
    embedded design;
  - evaluate Neon direct and pooled connections separately, including warm and
    scale-to-zero latency, chosen region, outage behavior, and monthly cost.
    Until real-vault remote corpus/query egress is explicitly authorized, this lane
    uses synthetic data only;
  - benchmark pgvector ANN only after PostgreSQL exact misses the SLO. Freeze
    ANN parameters before the held-out run; require top-50 recall ≥0.98 overall
    and ≥0.96 for every query, zero forbidden/filter leakage, and no regression
    below H9's recall@5 floor. A tuned result that saw the held-out answers is
    invalid evidence.
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

### 4a. Hybrid degraded modes (CLI/agent surface)

The semantic channel depends on a network API for *both* indexing and query
embedding; the lexical channel never does. spec §0.1's availability invariant
("search is as available as reading") therefore binds the **lexical** channel
only, and the design keeps semantic strictly an enhancement layer:

- **⌘K / `/search` / live results**: lexical-only in every state. They never
  construct a query embedder, read an API key, consult a vector-cache epoch, or
  show provider/cache diagnostics. Their availability contract is exactly the
  shipped lexical contract.
- **`yomihon search` CLI**: lexical by default; `--semantic` adds the hybrid
  channel and **fails loudly** (nonzero exit, stderr says why) when the cache
  is cold or the API is unreachable — an agent must never get silently
  different result sets with exit 0. There is no best-effort mode (ruled
  2026-07-12, D50): strictness lives in the exit code, and a partial or
  degraded answer never wears exit 0.
- **Cache freshness belongs only to explicit CLI actions.** `serve` never
  opens the vector store, reads the key, or constructs an embedder. A cold or
  incompatible identity requires `yomihon search-index build`. With a
  compatible active generation, `search --semantic` compares a content-hash
  corpus manifest, reuses unchanged vectors, and may reconcile a small drift
  before its one query send. The interactive ceiling is 128 missing chunks
  and 100,000 submitted proxy tokens; above either it exits 3 with
  `rebuild-required` before document or query egress. Interactive document
  calls are single-attempt/fail-fast. Only the explicit build may automatically
  retry 429. Each pending chunk receives at most five durably reserved send
  slots from its storage generation; a reservation can be consumed before HTTP,
  so slots upper-bound sends. Exhaustion is `attempt-budget-exhausted`, not
  `rate-limited`, and ordinary build cannot renew it.
  `search-index build --renew-attempt-budget` is the sole renewal path: under
  the writer lease it admits only an exact exhausted staging target, atomically
  copies completed vectors into one replacement stage, and then continues the
  same build action. Missing, incompatible, corrupt, or not-exhausted staging
  returns `attempt-budget-not-renewable`, exit 3, with zero domain mutation/send
  (SQLite recovery/WAL bookkeeping may occur on an existing store) and no
  ordinary-build fallback.
- **Platform scope is explicit.** Version 1 supports the semantic generation
  store only on Darwin and Linux. Windows has no store because synthetic
  mode bits cannot establish the owner-only privacy
  boundary. A text-bearing `search --semantic` preserves lexical results and
  exits 3 with `unsupported-platform`; `search-index build` emits the exit-3
  error envelope. All lexical/UI/serve/judge surfaces remain available. Store
  entry points fail before filesystem, key, or provider access; a future
  Windows DACL design is a separate ruling and runtime gate.
  Other targets have no v1 compile/runtime support promise; a Unix-like target
  name alone does not establish driver, permission, or lease support.
- **Exact-index capacity is deterministic.** Before hydration/build, require
  `chunks < 100,000` and raw vector payload `<= 1 GiB`; otherwise return
  `capacity` and open the §4 comparison. At 3,072 dimensions this means 87,381
  chunks pass and 87,382 do not. This is not an arbitrary-OOM recovery claim.
- **Every publication is a complete immutable generation.** One mutable
  staging generation may retain successful work across interruption, but only
  while its full identity, target manifest, policy sources, expected count,
  and retry state still match. The final corpus manifest and policy-source
  bytes are revalidated, then one SQLite transaction flips
  `previous=active; active=staging; staging=NULL` and removes unreferenced
  generations. Failure or interruption leaves any valid prior active generation
  unchanged; an ordinary explicit build that resets corrupt derived state may
  instead truthfully leave active absent. A concurrent
  writer is `index-refreshing`; after losing the lease, a search re-reads the
  active generation once so a just-completed writer is not reported as a
  spurious failure. No stale vector and no partial hybrid ranking is served.
- **One process owns one complete generation identity.** An incompatible
  explicit build may keep the old active generation transactionally intact
  while it stages a replacement, but the new-identity process does not keep a
  retired embedder registry and cannot query that old generation. Semantic
  search exits 3 with `cache-mismatch` until activation. `previous` is retained
  as publication-retention state, never selected as an automatic ranking
  fallback.
- **Numerical-identity changes are incompatible builds, not store-schema
  migrations.** A model, dimension, prompt/protocol, response-handling, or
  vector-format change builds a replacement generation; a query never scores
  vectors across incompatible identities. The SQLite container schema has its
  own `PRAGMA user_version`: unknown or incompatible schemas are rebuilt only
  by the explicit build command, while a future known-compatible schema bump
  must ship a version-specific copy-forward into a new file, validate it, and
  atomically replace the old file. There is no in-place migration ladder.
  Chunk inputs remain bounded to the model input limit as specified in the B
  plan.

## 5. Quality rails (D36, work items not advice)

- **CI (GitHub Actions), jobs by code area**: verify (format, vet, linters,
  security analysis, tests, race, retained-driver check, and build) ·
  govulncheck · windows-semantic-contract · darwin-semantic-contract ·
  assets-drift (templ + CSS regeneration must diff clean) · lint-frontend
  (Biome for the runtime and browser-probe JavaScript, Stylelint for the
  hand-written stylesheet sources) · e2e-http · fuzz (10,000 executions/target) ·
  e2e-behavior (the live-browser locks) · e2e-mutations (each probe's
  self-tests, exit 1 plus the caught marker).
  **Posture note**: the Node-based frontend tools run in CI only — the
  *product build and runtime* stay Node-free.
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
  pinned eval set (30–50 CJK-heavy queries with expected-note judgments)
  run as a test — not to gate CI on a score, but so a model swap or
  chunking change shows its relevance diff instead of relying on Koopa's
  gut-feel that "search got worse". Fixture policy (D50.8): the committed
  set is synthetic; real-vault queries and their paired diffs stay local,
  and only content-free aggregates enter the repo.

## 5a. Plan-doc obligations (the judge-plan pattern, generalized)

Before building items 5/6/7, a plan doc per face, carrying at minimum:
- **B**: chunking rules (heading-based, token bound, fences/frontmatter
  handling — state the chunks-per-note assumption; measurement says ~10–15/note
  on today's vault, so ~5–7×10³ chunks now, ~2–3×10⁵ at 18k notes); the durable
  cache's identity/publication/concurrency contract plus a pre-code measured
  selection gate that pins the chosen byte/schema format before storage
  implementation; RRF specifics (k, per-channel depths,
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
and a new `status: published` for release. yomihon needs zero code for the new
enum values: domain and status flow from the contract file, and the write
face offers a transition the moment its lifecycle entry exists there (until
then it correctly offers nothing). Per D51, `published` records Koopa's
selection for the public collection; it is input to a future publisher, not a
claim that an external deployment succeeded or remains live. What the new
pillar actually asks of the faces, by ownership:
- Structured queries (domain × status × topics) are **already shipped** in
  the lexical search grammar; the one delta is `source_kind` — an index
  field + filter key added when H builds (ruled 2026-07-12, D50: not a
  hybrid dependency — B leaves the grammar untouched; amending the index's
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

D40 moved daily reading to yomihon outright, so reading-surface pain is paid
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
2. `.y-prose table` has no horizontal-overflow guard, while code blocks and
   mermaid diagrams have one; a wide GFM table breaks the article column.
   Align tables with the existing overflow pattern.
3. `.y-title` and the metadata row carry no `overflow-wrap` guard against long
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

- The four walls (product.md) stand; wall 2's text carries its three
  authorized egress exceptions explicitly — instance, non-private note
  content to the embedding API (D32, never any contract-declared private
  path (D18), never non-instance artifacts (D47)), the query text of an
  explicitly requested semantic search (D50.1, at most once per explicit
  action, never logged or stored), and the fixed synthetic certification
  probes and eval fixtures of an explicit developer certification action
  (D57, test-only, never arbitrary input or vault bytes) — so the wall text
  and its authoritative reading are the same text. Any widening beyond these
  three is a new ruling.
- Every new capability gets both surfaces where it makes sense (§2). Hybrid
  retrieval is deliberately agent-only; the ordinary UI is not a degraded
  copy of it but a separately frozen lexical surface.
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
