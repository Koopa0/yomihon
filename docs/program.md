# Delivery program (start here)

This is the entry document for any session — any model — that picks up this
project. It exists so that a change of implementer never changes the target.
It is an execution view, not a second canon: if anything here conflicts with
the sources below, they win, in this order.

| Concern | Canon |
|---|---|
| Why the product exists, its modes, its aesthetic | `product.md` (positioning, the constitutional queue, the taste charter) |
| What the finished system is | `spec.md` (goals §0, per-face specs and acceptance) |
| Why it is the way it is | `decisions.md` (D01–D47; a ruling that is not there did not happen) |
| What order and why | `roadmap.md` (dependency and leverage; no milestone fences, D15) |
| How well work must be done | `standards.md` (testing, CI, taste, verification protocol) |
| Per-face contracts | that face's plan doc (`judge-plan.md`, `search-plan.md`, `ux-plan.md`, and the B/H/D docs to come) |
| The hard boundaries | the four walls (`README`, D02) — violating one means stop and ask Koopa, not route around |

## 1. Roles (the division of labor, fixed)

- **Builder** — an implementing session. Writes code on a branch from a
  written briefing, self-verifies per `standards.md` §5, commits locally,
  reports with verified/assumed marked. Never pushes, never merges, never
  rules on walls, schema, taste, or divergence registers.
- **Guide** — a separate session. Writes briefings from the plan docs,
  arbitrates reversible conflicts, and performs acceptance as an independent
  re-verification: re-run the gates, re-read the diff, re-run kill-tests,
  mutation-test every new lock. Acceptance that only reads the builder's
  report is not acceptance. One guide at a time; intermediate reviews are
  input to the guide, not rulings.
- **Koopa** — rules on everything irreversible: the walls, the schema, taste,
  new dependencies (D41 admissions), divergence adjudications, and every
  push / merge / retirement declaration. These are his decisions always;
  executing a specific one is delegable on his explicit instruction for that
  instance, never on the guide's initiative.

Work arrives one PR at a time: briefing → build → independent acceptance →
push/PR → bot-review triage (findings fixed or refuted line by line) → CI
green → Koopa merges. Plan docs get one adversarial review round before any
code is built from them (D37).

## 2. The remaining program, in PR-sized units

Order within each track is dependency-driven; tracks marked ∥ can run in
parallel because their files do not overlap. Sizes are chosen for
reviewability — one PR is one reviewable idea. These are not fences (D15):
a unit ships when it is ready, and pain may reorder the tracks (roadmap §5b).

**Track 1 — finish the kura retirement (the only externally-blocked thread):**
1. `PR-diff-fuzz` — the differential harness (merged, PR #19). Acceptance
   held: judge-plan §13's harness shape, rule-reach self-check, kill-tested
   runner.
1b. `PR-diary-influence` — **ruled (D42, delegated): the journal influences
   no egress verdict.** Tests first: fixtures pin that a journal-only mount
   edge leaves a public concept orphaned and a journal-only planned name
   leaves a public broken link at warn; then exclude journal sources from
   coverage's mount edges and the planned-name set; register entry 10 and
   the differential harness's manifest/normalizer grow with it. Lands
   *before* the campaign, so its evidence is collected against the final
   behavior. The real-vault sandwich is run and its outcome recorded.
2. Campaign runs — **done**: runs to the §13 completion bar across three
   independent seed bases with zero unexplained divergence; nothing needed
   adjudication.
3. Koopa's retirement declaration, citing §13 — **done (D43, 2026-07-07)**.
4. `PR-descaffold` — **done (PR #26)**: conformance, sandwich, and differential
   tests deleted; goldens and pinning fixtures kept; the docs' gate passages
   rewritten to past tense. Vault-side: shell export removed, wrapper backups
   cleaned, reference binaries deleted (operations alongside the PR).

**The rename (kurodo → yomihon, D44) — done (PR #28, merged 2026-07-08;
acceptance held the same day).** The coordinated migration ran as
`rename-plan.md` records — module path, command directory, client script and
CSS scope, client-state keys, env whitelist lock, report tool identity,
wordmark, and living docs in one solo sweep; the directory, memory-slug, and
GitHub moves landed around it. Independent acceptance re-ran the gates,
kill-tested the env lock on a different channel, and probed the real vault
under the new name. Operations remainder (Koopa's hand): the four-cron
cutover to `~/go/bin/yomihon`, then deleting the old binary and the emptied
old directory.

**Track 2 ∥ — CI hygiene:**
5. `PR-ci-hygiene` — **done (PR #21, merged 2026-07-07)**: the umbrella job
   renamed to `verify`, the vulnerability scan split into its own pinned
   `govulncheck` job, the golangci-lint installer checksum-verified, a
   concurrency group and per-job timeouts added, and the Makefile's Go
   targets scoped past a stray `node_modules/` tree. (The unit stayed listed
   as open here for two days after it merged — the staleness Koopa caught on
   2026-07-09; `standards.md` §3's debt paragraph retired in the same
   motion.)

**Track 3 ∥ — the reading surface (quality of daily life):**
6. `PR-ux-a` — the four mechanical repairs (roadmap §5b; no ruling needed).
7. `ux-plan.md` review — Koopa walks the design calls item by item
   (the checklist at its end) and rules; the doc is amended to match.
8. `PR-ux-b1` — sidebar restructure: wayfinding, syllabus disclosure, the
   filter box, lifecycle demotion (per the approved plan).
9. `PR-ux-b2` — **the experience batch (merged, PR #27)**: view transitions
   with stable chrome regions, the reading-smoothness inventory (§12, hairline
   included), seal feedback, mermaid shimmer, TOC scroll-spy, disclosure
   persistence, the search page's shell adoption, and the right-rail redesign
   (§6). It also carried two fixes that resolved standing bugs — the concept
   sheet no longer paints an opaque panel over the TOC on lesson pages, and
   the search page renders inside the shell (both were diagnosed as already
   fixed here, 2026-07-08). The cosmetic remainder shipped in `PR-ux-fixes` —
   **done (PR #29, 2026-07-09)**: the frontend fix batch from ux-plan §15's
   three-source review (palette centering and surface, the seal-shortcut
   guard on focused selects, light dismiss, focus indicators, ARIA state,
   the ruby-TOC repair, and the smaller platform fixes), accepted
   independently and merged. Its follow-ups from ux-plan §16 are done: the
   second fix batch — **done (PR #30)** — carried the filter pre-paint reveal,
   the prose-link underline, and the comment tokens; the class-prefix sweep
   `k-` → `y-` followed — **done (PR #31)**; and the customizable select's open
   picker was branded to match its closed face — **done (PR #34)**. The scope
   fork Koopa held (the sidebar/route `.md` asymmetry) became D45 and shipped —
   **done (PR #32)** — with the embed refusal pinned to the bytes it must put on
   the wire — **done (PR #33)**. One Safari glance at a slot lesson is still
   owed, to turn PR #34's button-in-select assumption into a fact.
9b. `PR-ux-b2h` — **done (PR #40)**: Home v0.5, including scanner-owned mtime
    plumbing, four snapshot-backed dashboard blocks, and the rendered vault
    README below them.
9c. `PR-ux-b3` — **done (PR #42)**: the content-driven sidebar; every map note
    renders as a navigable tree, wayfinding generalizes to all pillars, the
    journal gets its door, and lifecycle moves to Home's strip and a topbar
    aggregate.
9c-i. `PR-instance-contract` — **in review (PR #43)**: schema-declared Paths and
    Maps roles, the non-instance artifact boundary, differentiated map
    resolution, metadata capability degradation, and fail-closed writes.
9c-ii. `PR-home-search-behavior` — Home's start-at-top/no-autofocus correction
    lands with #43 while its form remains plain GET. The remaining B lexical
    work adds input-driven results to ⌘K and `/search` as progressive enhancement
    while preserving Enter, the submit button, and the no-JS GET path.
9c-iii. `PR-published-receipt-boundary` — urgent: hide and service-reject the
    generic `ready → published` transition until a publisher path can present a
    verifiable external-success receipt. UI hiding alone is insufficient.
9c-iv. `PR-advanceable-chip-truth` — stop presenting lifecycle advanceability as
    a proven pending-decision queue; rename or hide the aggregate until an
    independent pending signal exists, then let the D plan own real queue
    semantics.
9c-v. `PR-canon-reconciliation` — reconcile the remaining preexisting drift in
    `program.md`, `roadmap.md`, `judge-plan.md`, `vault-model.md`, and historical
    design inputs; install any vault-side governance rule before repo canon says
    it is active. This unit also records explicit supersessions for the retired
    Lifecycle-first sidebar, old file-surface and harness assumptions, and the
    current judge authority pointers.
9d. `PR-ux-c` — the hover layer (ux-plan §11): wikilink hover previews over
    a read-only fragment endpoint, and in-place diagnostic cards — popover +
    CSS anchor positioning, zero positioning JS.
10a. `PR-e2e-reading-behavior` — **done (PR #35)**: the scratchpad live-browser
     probes for the §15/§16 reading-surface regressions are committed as
     `.github/e2e/*.mjs`, driven by `playwright-core` with `channel: "chrome"`
     against the runner's Chrome, installed with
     `npm install --no-save --no-package-lock` in the same runner-only style as
     `lint-frontend`. No `package.json`, no bundled browser, no pixel baseline.
     Locks: the palette centered and opaque; the filter revealed by the
     document's own inline script and hidden with JavaScript off; a held `R`
     inside a focused select never touching the seal path.
10a-i. `PR-wall-locks` — **done (PR #36)**: the two wall guards made able to
     fail, and the file faces swept.
10a-ii. `PR-instrument-hardening` — **done (PR #37)**: every lock provably able
     to fail, and CI proving it. The probe contract of `standards.md` §2 lands
     here — site-bound markers, `not-applied` exiting 2, the `e2e-mutations`
     job asserting exit 1 plus the exact marker — together with the job family
     renamed (`e2e-http`, `e2e-behavior`, `e2e-mutations`, `fuzz`), the
     loopback socket assertion's first self-test, and the environment guard
     widened to every package the binary links. The probe named for a paint it
     never sampled became `filter-inline-reveal.mjs`.
10b. `PR-e2e-screenshots` — the deferred screenshot baseline job, once the
     surface is stable enough that a baseline is worth committing. Its
     dependency shape is a fresh D41-style decision: use `@playwright/test`
     only if the screenshot runner, traces, and diff reports earn a checked-in
     dev dependency and package manifest.
10c. `PR-typing-guard-coverage` — the seal shortcut's typing guard is probed on
     the select's two faces only. Cover the rest: `INPUT` (the sidebar filter),
     `TEXTAREA`, `contentEditable`, and the clause that stands down while the
     search dialog is open. Fold in a needle-uniqueness assertion for
     `rewriteGuard`: the needle must occur exactly once in the fetched script,
     and a second occurrence reports `not-applied` rather than rewriting a
     clause the mode was never aimed at. Low priority — after the Home unit.

**Track 4 — the remaining faces (each: plan doc → adversarial round → build):**
11. `B` — search panel, two halves in order (roadmap §1 row 5): the lexical
    ⌘K panel PR first — no plan doc needed, `search-plan.md` already pins
    lexical semantics and the shell exists (the search page's shell
    adoption ships earlier, with the experience batch); then the **B plan doc** (chunking,
    cache format, RRF, the degraded-mode matrix, the eval set — roadmap §5a;
    Gemini key arrives at build start) before any hybrid work.
12. `H` — agent toolbox: plan doc (JSON contract per verb + goldens — these
    outputs are frozen contracts from day one, D37), then the build PR.
13. `D` — the adjudication cockpit: plan doc (queue semantics, reading-state
    file, inbox convention — roadmap §3; absorbs the v0.5 landing page),
    then one or two build PRs.
14. `G` — export: SSG static output with the five interactions, `Diary/`
    absent from `dist/` (spec §6).

Roughly fifteen PRs to completion. Do not batch tracks into mega-PRs to "save
review"; review capacity is the reason the units are this size.

## 3. What "done" means (coverage and grade)

The system is finished when spec §0's four end-states hold, at this grade:

- **Reading**: every `.md` in the vault opens (zero 500s, zero blanks); the
  surface is pleasant enough that Koopa reads there by preference — the UX
  plan's acceptance criteria, met, are the measurable form of "pleasant".
- **Adjudication**: seal-on-finish with zero JS required (D27), wall-1 locks
  green forever.
- **Judge / agent toolbox**: output bytes frozen and consumed (done for
  check/coverage/exists; H extends the same discipline — goldens before
  logic).
- **Search**: lexical always available; hybrid semantic with its relevance
  measured against the pinned eval set, degraded modes per the matrix
  (roadmap §4a) — never a hard dependency for reading.
- **Cockpit**: Koopa's decision throughput scales past agent writing speed —
  the queue, not the prettiness, is the point.
- **Export**: lessons work as static files; nothing else claims that
  surface.
- **Quality floor everywhere**: `standards.md` in full — mutation-proof
  locks, frozen goldens, pinned toolchains, minimal permissions, the
  verification protocol before every push, hygiene greps at zero.

## 4. How to start a work unit (any session, any model)

1. Read this file, then `standards.md`, then the canon row for your unit
   (spec section, plan doc, roadmap section).
2. If the unit has no plan doc and needs one (every remaining face does),
   write the plan doc first and run its adversarial round — do not write
   code.
3. Confirm the unit's briefing exists (the guide writes it; it carries scope,
   verified starting state, deliverables, out-of-scope, hygiene, and the
   report contract). No briefing, no build.
4. Build on a branch; self-verify per `standards.md` §5; report with
   verified/assumed marked; stop at anything that smells like a wall.
5. Expect independent acceptance to re-run everything you claim. Write the
   report so that it can.

## 5. Taking the guide seat (for a fresh guide session)

The guide inherits from documents, not from conversation. Read, in order:
this file, `standards.md`, `product.md`, then the canon row for the active
unit (its spec section or plan doc). A ruling that is not in the documents
did not happen — do not trust conversational summaries, including your
predecessor's. Current position: `git log` on main plus §2's unit list tell
you what is merged; anything in flight is on a branch.

The guide's obligations, none delegable to a builder:

1. **Briefings come from canon.** Every dispatch carries: scope, a verified
   starting state (re-verify the SHAs and file:line claims yourself before
   issuing — cheaply, but yourself), deliverables, out-of-scope, hygiene,
   self-verification steps, and a report contract. No briefing issues from a
   plan section that has not had an adversarial round (D37) — "designed" and
   "hardened" are different states, and building from the first one produces
   rework.
2. **Rulings land in documents before code proceeds.** Reversible UI and
   semantic conflicts are the guide's to rule; walls, schema, taste, and
   dependency admissions (D41) are Koopa's. Either way, the ruling is
   committed to the owning document first — the builder builds from the doc,
   never from a chat message.
3. **Acceptance is independent re-verification.** Re-run the gates
   (`make verify`, and the frontend lint pair when CSS/JS changed —
   `make verify` does not cover it); replay at least one kill-test on a
   different channel than the builder chose; hand-verify any user-visible
   number against the real vault; probe UI work with headless-Chrome
   screenshots against `~/obsidian`; run the hygiene greps (`standards.md`
   §5). Intermediate verifiers' reports — including well-written ones — are
   input, never the verdict.
3a. **Acceptance is cold first.** Form your findings from the raw diff before
   you read the builder's report; a report is a warm reading and it hands you
   its own frame. Review the merge preview against main, not the branch alone
   — a textually clean merge can produce a state neither side ever ran, and it
   has: a kill-test whose needle matched the branch's script died against
   main's. Treat the verification instruments as review targets in their own
   right, and ask each one to show you it failing — the reviewer's findings on
   this repository land on the instruments as often as on the code. Scale the
   number of fresh-context lenses to the blast radius, not to the diff size.
   And re-run the mutation transcripts rather than re-reading them: a
   transcript is a claim, and re-running it is the only thing that turns the
   claim into a fact.
4. **Review-bot triage is line-by-line**: each finding is either fixed or
   refuted against the real code in the PR conversation. Never wave a batch
   through in either direction.
5. **The shared-tree rule.** Builders may own the main working tree. While
   any builder is active, the guide commits only through a private
   `git worktree` — a commit in a shared tree sweeps the builder's staged
   index into your commit. Never `git add -A` anywhere; prefer
   `git commit -- <paths>` even in a private tree.
6. **Koopa presses the last key.** Dispatching builders, pushing, merging,
   and declarations are his; a specific instance may be delegated by his
   explicit word, never assumed from precedent.
