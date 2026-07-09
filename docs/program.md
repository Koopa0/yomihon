# Delivery program (start here)

This is the entry document for any session — any model — that picks up this
project. It exists so that a change of implementer never changes the target.
It is an execution view, not a second canon: if anything here conflicts with
the sources below, they win, in this order.

| Concern | Canon |
|---|---|
| Why the product exists, its modes, its aesthetic | `product.md` (positioning, the constitutional queue, the taste charter) |
| What the finished system is | `spec.md` (goals §0, per-face specs and acceptance) |
| Why it is the way it is | `decisions.md` (D01–D41; a ruling that is not there did not happen) |
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

**Track 2 ∥ — CI hygiene (one chore PR, already specified):**
5. `PR-ci-hygiene` — pay the debts recorded in `standards.md` §3: rename the
   umbrella `ci` job to `verify` (mirror the local gate), split the
   vulnerability scan into its own job, pin `govulncheck`, pin and verify the
   golangci-lint installer, add a concurrency group and per-job timeouts.
   Koopa updates the branch-protection required checks in the same motion.
   The local gate is part of the same debt: the Makefile's `./...` targets
   pick up Go packages shipped inside the ignored `node_modules/` tree when
   it exists (the frontend linters leave one), so test/vet/fmt scope to a
   package list filtered of `/node_modules/`, with a guard that keeps the
   filter honest.

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
   independently and merged. Its follow-ups live in ux-plan §16: a small
   second fix batch (filter pre-paint reveal, prose-link underline, comment
   tokens) and the customizable-select PR, plus one scope fork awaiting
   Koopa (the sidebar/route `.md` asymmetry).
9b. `PR-ux-b2h` — Home v0.5 (ux-plan §3): the mtime plumbing and the
    landing page; smaller and separable from the experience batch.
9c. `PR-ux-b3` — the content-driven sidebar (ux-plan §13): every map note
    renders as a navigable tree, wayfinding generalizes to all pillars, the
    journal gets its door, lifecycle retires to Home's strip and a topbar
    chip.
9d. `PR-ux-c` — the hover layer (ux-plan §11): wikilink hover previews over
    a read-only fragment endpoint, and in-place diagnostic cards — popover +
    CSS anchor positioning, zero positioning JS.
10a. `PR-e2e-reading-behavior` — promote the scratchpad live-browser probes
     for the §15/§16 reading-surface regressions into CI before they evaporate:
     a committed `.github/e2e/*.mjs` script using `playwright-core` with
     `channel: "chrome"` against the runner's Chrome, installed in the job
     with `npm install --no-save --no-package-lock` in the same runner-only
     style as `lint-frontend`. No `package.json`, no bundled browser, no pixel
     baseline. Locks: palette centered and opaque; the filter present before
     first paint with the enhancement normal or its deferred script blocked,
     and staying hidden with JavaScript disabled; and a held `R` inside a
     focused select never touching the seal path.
10b. `PR-e2e-screenshots` — the deferred screenshot baseline job, once the
     surface is stable enough that a baseline is worth committing. Its
     dependency shape is a fresh D41-style decision: use `@playwright/test`
     only if the screenshot runner, traces, and diff reports earn a checked-in
     dev dependency and package manifest.

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
