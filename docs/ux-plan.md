# Reading-surface UX plan (navigation, motion, and the sidebar)

Status: **ruled 2026-07-06** — item 1 by Koopa (the landing combines the
dashboard blocks with the vault README rendered in place), items 2–15
delegated to the guide and resolved as recorded in §9. This document is now
the buildable design; further changes are amendments, not review rounds. Scope: the global chrome, the landing page, the left
sidebar, the note page's rails, and motion/loading. Out of scope: the
adjudication cockpit's queue mechanics (the D plan doc owns those; §4 here
only builds the skeleton it will inherit) and the ⌘K panel's content (the B
plan doc owns retrieval).

## 1. Principles

- **One user, reading daily.** Every choice optimizes the owner's daily read,
  not a visitor's first impression. Friction he pays once is fine; friction
  he pays per note is a defect.
- **Server-rendered MPA, enhanced in place.** State lives in the URL and the
  vault; the server renders it; the page works with JavaScript disabled —
  the write path especially (D27). Enhancement never becomes a requirement.
- **The platform ladder (D41).** For any interactive need, in order: semantic
  HTML → CSS → a Chromium-native Web API (this app runs in the owner's
  Chromium; Baseline-wide support is not a constraint) → a small vanilla-JS
  enhancement → a mature, vendorable library when it genuinely earns its
  place. Do not reinvent what a mature library does well; do not import what
  a `<details>` element already does. Admission criteria for a library are in
  D41; mermaid is the standing precedent.
- **Motion is meaning.** Transitions exist to preserve context (where did
  that panel go?), signal success, or mask real latency — never decoration
  for its own sake. Every animation respects `prefers-reduced-motion`.

## 2. Global chrome

- Topbar stays: wordmark → breadcrumb territory, search field (⌘K), furigana
  toggle, theme toggle. No new global elements.
- **Cross-document view transitions** for link navigations: CSS-only
  (`@view-transition { navigation: auto }`), default cross-fade ≈180ms; the
  note title carries a `view-transition-name` so moving between list and note
  reads as the title traveling, not a repaint. Reduced-motion: transitions
  off entirely.
- **The write path is the accepted exception.** `navigation: auto` does not
  fire for POST form submissions, so the seal's POST → redirect → GET (D27)
  will not cross-fade — and that stays as it is. Do not "fix" it by
  intercepting the form with `fetch` and driving a same-document transition:
  that is exactly the write-path scripting D27 forbids. The seal gets its
  feedback from §7's chip pulse and toast instead.
- Keyboard: `⌘K` (exists) focuses search; `/` focuses the sidebar filter
  (§5); `[` toggles the sidebar drawer at narrow widths. No other global
  bindings until asked for.

## 3. Landing (`/`)

Today `/` redirects to `/notes/README.md`. The vault README is genuinely good
(verified 2026-07-06: lifecycle, structure, where-to-start, norms — a proper
door); the failure was making it the *only* thing on landing, when the daily
reader needs "what moved, what needs me" at zero scroll.

**Design (Koopa's ruling): `/` renders Home v0.5 — the dashboard blocks
first, then the vault README rendered in full below them** as the standing
introduction. No redirect, no vault edit: the README note stays the intro it
already is, one scroll away instead of in the way. The blocks are assembled
at request time from the in-memory snapshot (no new state *store*; the
reading-tracker stays cockpit territory, deferred to the D plan doc, which
absorbs this page):

1. **Recently changed** — the newest N (≈7) knowledge notes by file mtime,
   with status chips. Honest label: this is "what changed on disk", not
   "what you read last" — the latter needs cockpit state. **Plumbing note
   (not free today)**: the snapshot types expose no timestamp; the scanner
   already stats every file, so the work is threading an mtime through the
   snapshot build onto the nav model — one field, captured at scan time. Do
   not bolt on a per-request stat walk; freshness stays centralized in the
   scanner (D25).
2. **Lifecycle strip** — the status counts as one row of chips, each linking
   to its filtered list. This is the board's trailhead, not the board.
3. **Study paths** — one card per syllabus: title, sealed/total count, link
   to the syllabus page.
4. **Search** — the same field as the topbar, autofocused.

Below the four blocks, the README body renders through the same pipeline as
any note; its own page (`/notes/README.md`) and every direct link keep
working.

**Boundary against spec §2**: spec §2's four home blocks (domain MOC entries,
cross-domain boards, the mechanical-gate list, doc pointers) are **not**
discharged by v0.5 — they remain the cockpit-content obligation (roadmap §3),
and the D plan doc reconciles both when it absorbs this page. v0.5 is the
pre-cockpit skeleton, nothing more.

## 4. The left sidebar (structure, wayfinding, disclosure)

Today the sidebar is global and identical on every page: Lifecycle (nine
statuses, always expanded, top), Syllabus (every course fully expanded),
Reports, Folders (collapsed). Reading a Sources note, it shows a Go syllabus
and no trace of where you are. The redesign makes it **context-first**:

Order, top to bottom:

1. **Filter box** (§5).
2. **Here** — only on note/syllabus pages: the current note's siblings
   (same-directory notes, sorted, current one marked `aria-current="page"`),
   under a plain-text heading naming the parent directory. The heading is a
   label, not a link — no folder-index route exists and this plan does not
   invent one; "everything in this folder" is what the block itself shows.
   One glance answers "where am I, what is next to me".
3. **Syllabus** — every course a closed `<details>`; the course — and inside
   it the module — containing the current note is auto-opened server-side.
   **Plumbing note (not free today)**: nothing maps a note back to the
   sections that reference it; build a reverse index (rel-path → syllabus /
   section chain) at snapshot build time, next to the existing nav model. A
   note referenced from several sections opens **all** of its containing
   paths, `aria-current` on each occurrence — no tie-break rule to invent.
   Lesson rows keep the status chip; the current lesson is highlighted.
4. **Lifecycle** — demoted to one collapsed `<details>`, summary carrying
   one ambient number (which number is checklist item 15); expanding shows
   the nine counts linking to filtered lists. The full board lives on Home /
   the future cockpit — the sidebar only keeps its doorway. This deliberately
   revisits the Lifecycle-first ordering (D26): that ruling optimized for
   adjudication; daily reading optimizes for wayfinding. Both survive —
   adjudication keeps the seal panel and the Home strip.
5. **Reports** — unchanged, collapsed.
6. **Folders** — the whole vault tree, collapsed, but with the current
   note's ancestor chain auto-expanded and the note marked, so "reveal in
   tree" is the default state, not a feature.

Width stays fixed (264px) at desktop; the drawer behavior ≤900px stays. No
resize handle: a resizable rail is state to persist and a wall of edge cases
(min/max, double-click reset) for a single user who can restyle a CSS token
instead. If real use disproves this, it returns as a UX-B item with a
persisted preference like the theme cookie.

`<details>` open/close animates via `interpolate-size` +
`::details-content` transition (Chromium-native, CSS-only); reduced-motion
disables it.

## 5. Sidebar filter (the one new vanilla-JS enhancement)

A text box pinned at the sidebar top, hidden until JS runs (it is pure
enhancement). Typing filters the *visible text* of all sidebar entries —
sections with no hit collapse their contents to nothing; matches keep their
ancestor context; `Enter` follows the first match; `Esc` clears and returns
focus to the page. It filters what the sidebar already shows — it is not
search (⌘K and `/search` own content retrieval; B owns making them better).
Budget: ≤ ~60 lines in `kurodo.js`, zero network, zero state.

## 6. The note page's right rail

- UX-A's repair holds (roadmap §5b): the rail collapses only when all three
  blocks (TOC, status panel, diagnostics) render nothing; the status panel —
  the write face — is never hidden by any layout state.
- The TOC gets no show/hide toggle for now: the original complaint was the
  blank column, which the repair removes. If a toggle is still wanted after
  living with the repair, it lands as a `<details>` around the TOC with a
  cookie-persisted default — one evening of work, ruled then, not built on
  speculation.
- Long-TOC behavior — **already shipped, keep it**: the rail is independently
  scrollable and sticky today (`position: sticky` + `overflow-y: auto` in the
  stylesheet). No work here; listed only so nobody re-adds it.

## 7. Motion, loading, feedback (the appropriate-animation inventory)

- **Navigation**: cross-document view transitions (§2, with the write-path
  exception stated there). This is the single highest-leverage polish item —
  every link click stops flashing white.
- **Seal / status flip**: the POST-redirect-get stays (D27). After the
  redirect, the updated status chip plays one ~400ms pulse (CSS animation
  keyed off a `?sealed=1` query param the redirect carries), and a small
  "sealed" toast renders server-side with a CSS auto-fade. Zero JS; the
  no-JS path shows the same page minus the pulse.
- **Press-and-hold on `ready`** (exists) stays — it is the correct weight
  for an irreversible-feeling action.
- **Loading**: local MPA renders are near-instant; no global spinner.
  Buttons get a busy state (disabled + reduced opacity) on submit. The
  mermaid container shows a CSS shimmer skeleton until the client render
  replaces it — the one real async wait a reader sees today.
- **TOC scroll-spy**: the TOC highlights the section in view
  (IntersectionObserver, ~20 lines, enhancement-only). Earns its place
  because long concept notes are the norm in this vault.
- Everything above: disabled or reduced under `prefers-reduced-motion`
  (the reading page already carries this discipline; extend it, don't fork
  it).

## 8. Acceptance criteria (for the PRs that implement this plan)

1. Keyboard-only pass: every sidebar entry, filter, TOC link, and seal form
   reachable and operable; focus visible throughout.
2. JS disabled: every page renders, navigates, and seals correctly; the
   filter box is simply absent; no layout depends on script.
3. `prefers-reduced-motion: reduce` kills every transition and animation
   (assert by computed style in the e2e).
4. The wayfinding invariants: on any note page, the sidebar shows the note's
   siblings, its syllabus path(s) auto-opened (when it has any), and its
   folder-ancestors expanded — assert on three representative fixtures
   (concept, lesson, no-frontmatter Sources note).
5. The rail never hides the status panel while frontmatter is valid — a
   template-level test, plus one screenshot.
6. Screenshot set at 1600 and 1320 widths for home, a concept note, a
   lesson, and a no-heading note — committed as the baseline that seeds
   `PR-e2e-screenshots`.
7. No new dependency without a D41 admission recorded in `decisions.md`.
8. `standards.md` §5 protocol, as for every PR.

## 9. Resolutions (ruled 2026-07-06; item 1 by Koopa, 2–15 delegated to the
guide and resolved as follows)

1. **Landing** — Koopa: the README hypothesis was checked and the note is
   good; keep it in the landing experience. Resolution: Home v0.5 renders the
   dashboard blocks first and the vault README in full below (§3). No
   redirect, no vault edit.
2. Home blocks and labels — as specced in §3.
3. Sidebar order — as specced in §4.
4. Lifecycle demoted to a collapsed section — confirmed.
5. Syllabus closed by default, active path(s) auto-opened — confirmed.
6. "Here" siblings block — build it.
7. No sidebar resize — confirmed (returns only if daily use disproves it).
8. Sidebar filter box — build it as specced in §5.
9. View transitions, title as shared element, write path excluded — build.
10. Seal feedback = chip pulse + server-rendered toast — build.
11. TOC toggle — stays deferred.
12. TOC scroll-spy — build.
13. Mermaid shimmer skeleton — build.
14. D41 admission criteria — stand as written in `decisions.md`.
15. The ambient number on the collapsed Lifecycle summary — **the
    adjudication backlog**, precisely (refined 2026-07-07 when the archive
    stage's wildcard `from` made the first phrasing degenerate to "all
    notes"): a note counts when its current status appears as a concrete
    (non-wildcard) `from` predecessor of at least one lifecycle stage whose
    owner includes the vault's owner, and is not the seal status itself.
    Derived from the loaded contract at render time — never a hardcoded
    status list (wall 3); the one sanctioned constant is the existing seal
    status. Why each clause: the wildcard archive escape names no real
    pending work; an agent-advanced status is the agents' queue, not the
    owner's — the owner field is the machine-readable answer to "whose hand
    moves this", so a future ownership change re-scopes the number with no
    code change; and the seal is the pipeline's resting point — promotion
    beyond it is opt-in, not pending adjudication. Against the 2026-07-07
    contract this yields imported + draft (an illustration, not a list to
    copy). The predicate's test constructs its own contract exercising all
    three exclusions.
