# Reading-surface UX plan (navigation, motion, and the sidebar)

Status: **ruled 2026-07-06; consolidated 2026-07-07** after a three-source
adversarial round (two internal lenses plus an external reviewer, 32 findings
triaged) — the amendments that had accumulated as sediment are folded into one
coherent design. §9 keeps the ruling record; §14 records the round. This is
the buildable design; further changes are amendments.

Scope: the global chrome, the landing page, the left sidebar, the note page's
rails, motion/loading, the hover layer, and the smoothness inventory. Out of
scope: the adjudication cockpit's queue mechanics (the D plan doc owns those,
including per-note resume state — §14.6) and the ⌘K panel's retrieval content
(the B plan doc).

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
  place. Admission criteria are in D41; mermaid is the standing precedent.
- **Motion is meaning.** Transitions exist to preserve context, signal
  success, or mask real latency — never decoration. Decorative motion dies
  under `prefers-reduced-motion`; **essential progress feedback survives it**
  (the seal's hold-fill, the reading-position hairline) — that distinction is
  part of this plan, not a loophole.

## 2. Global chrome and navigation transitions

- Topbar stays: wordmark, search field (⌘K), furigana toggle, theme toggle.
  No new global elements except the backlog chip that arrives when Lifecycle
  leaves the sidebar (§13.6).
- **One shell everywhere.** Every page — search results included — renders
  inside the same shell with the sidebar present. A lifecycle chip or a
  search never strands the reader in a chromeless page. (Plumbing note: the
  search handler carries no nav provider today; wire it the way the report
  handler already does.)
- **View transitions, the precise recipe** (cross-document,
  `@view-transition { navigation: auto }`, CSS only). Naming a region makes
  it an independent transition group — it does not make it "stable"; a named
  group whose content differs cross-fades into double-exposure, and unnamed
  content rides the root cross-fade. So:
  - The chrome — topbar, sidebar, seal bar — each get a
    `view-transition-name` **and their old/new image animations set to
    `none`**: they hard-cut. A hard cut of near-identical pixels reads as
    perfect stillness; a hard cut of the sidebar's per-note differences
    (Here, auto-opened branches) reads as a crisp update, never a ghost.
  - The article scroll container gets a name and **keeps** the default group
    animation: when the rail column appears or disappears between pages
    (§6), its width morphs smoothly instead of cross-fading two reflowed
    text blocks.
  - The note title keeps its name and travels (list → note reads as the
    title moving).
  - Reduced motion: the `@view-transition` rule flips to `navigation: none`
    inside the media query — no transition at all.
- **The write path is the accepted exception.** `navigation: auto` does not
  fire for POST form submissions, so the seal's POST → redirect → GET (D27)
  does not cross-fade — and that stays. Do not "fix" it by intercepting the
  form with `fetch`; that is exactly the write-path scripting D27 forbids.
  The seal's feedback is §7's pulse and toast.
- Keyboard: `⌘K` focuses search; `/` focuses the sidebar filter; `[` toggles
  the drawer at narrow widths. Single-key bindings stay live when focus sits
  on links, summaries, or buttons (the keyboard-navigation convention
  everywhere else); they are suppressed only inside text entry and dialogs.

## 3. Landing (`/`)

Today `/` redirects to `/notes/README.md`. The vault README is genuinely good
(verified 2026-07-06); the failure was making it the *only* thing on landing.

**Design (Koopa's ruling): `/` renders Home v0.5 — the dashboard blocks
first, then the vault README rendered in full below them.** No redirect, no
vault edit. The blocks are assembled at request time from the in-memory
snapshot (no new state *store*; the reading-tracker stays cockpit territory):

1. **Recently changed** — the newest N (≈7) knowledge notes by file mtime,
   with status chips. Honest label: "what changed on disk", not "what you
   read last" — the latter needs cockpit state. **Plumbing note**: the
   snapshot types expose no timestamp; thread an mtime through the snapshot
   build onto the nav model — captured at scan time, never a per-request
   stat walk (freshness stays centralized in the scanner, D25).
2. **Lifecycle strip** — the status counts as one row of chips, each linking
   to its filtered list. The board's trailhead, not the board.
3. **Study paths** — one card per path: title, sealed/total count, link.
4. **Search** — the same field as the topbar, autofocused.

Below the blocks, the README body renders through the same pipeline as any
note; `/notes/README.md` and every direct link keep working.

**Boundary against spec §2**: spec §2's four home blocks (domain MOC entries,
cross-domain boards, the mechanical-gate list, doc pointers) are **not**
discharged by v0.5 — they remain the cockpit-content obligation (roadmap §3);
the D plan doc reconciles both when it absorbs this page.

## 4. The left sidebar (structure, wayfinding, disclosure)

Shipped (2026-07-07, PR #25): context-first order — filter box, **Here**
(same-directory siblings, current marked, plain-text directory label — not a
link; no folder route exists), **Paths** (every course a closed `<details>`,
the current note's course/module chain auto-opened server-side from the
reverse index, all containing paths when a note appears in several, status
chips kept), **Lifecycle** (collapsed, backlog count — retires per §13.6),
**Reports**, **Folders** (ancestor chain auto-expanded, note marked). §13
grows Paths into Paths & Maps and adds Journal.

Width fixed at 264px; the ≤900px drawer stays; no resize handle (a single
user restyles a CSS token; revisit only if daily use disproves it).

`<details>` disclosure animates via `interpolate-size` + `::details-content`
(CSS only); reduced-motion disables it.

**Disclosure state has exactly one JS owner.** Three forces act on
`<details open>`: the server's wayfinding chain, the reader's manual
toggles, and the filter's temporary open-to-show-matches. The rules:

1. The **current note's chain is never persisted and always forced open** —
   a manual close of the active branch is not recorded (recording it would
   either hide the reader's own location on the next page or make
   persistence look broken; neither is acceptable).
2. Manual toggles on **discretionary** sections persist per section key in
   `sessionStorage` (session-scoped; yesterday's posture should not
   fossilize) and are reapplied by a **pre-paint inline script** — applied
   before first paint there is no flash and no spurious open-animation.
3. The **filter owns a transient layer**: while a query is active it may
   open anything to reveal matches; clearing the query restores layer 1 ∪
   layer 2 — the filter's existing bookkeeping merges into the same state
   module, not a second competing map.

No JS → server defaults, exactly as shipped.

## 5. Sidebar filter

A text box pinned at the sidebar top, hidden until JS runs. Typing filters
the visible text of **every sidebar row — links and non-link rows alike**
(an unresolved lesson is a span, and it matters for wayfinding); sections
with no hit collapse; matches keep ancestor context; `Enter` follows the
first *link* match; `Esc` clears and returns focus to the page; `/` focuses
it. The no-match state is designed (§8.8), not a silently empty column.
Budget ≤ ~60 lines including its share of the state module (§4).

## 6. The note page's right side (the full matrix)

The rail exists to hold **reading aids**. The write face is never hidden —
including its fail-closed notice. The matrix, exhaustively:

| Viewport | TOC or diagnostics? | Rail | Status face |
|---|---|---|---|
| >1280px | yes | rail renders the aids **and the status panel** (as today) | in the rail |
| >1280px | no | **no rail column** — the article takes the width | the fixed-bottom seal bar |
| ≤1280px | yes | rail hidden (as today) — but the aids stay reachable: the TOC renders as a collapsed native `<details>` ("On this page") at the top of the article, diagnostics inline beneath it | the fixed-bottom seal bar |
| ≤1280px | no | nothing to reach | the fixed-bottom seal bar |

Corrections this consolidation makes explicit:

- "Under the note header" (an earlier phrasing) is retracted: the compact
  affordance **is the existing fixed-bottom seal bar**, at every width where
  the rail is absent. One muscle memory, not two.
- **The seal bar's guard changes**: today it renders nothing when the write
  face is closed; it must instead mirror **every** status-face state the
  rail panel would show — the fail-closed notice and the no-frontmatter
  notice alike (ruled at acceptance, 2026-07-08: the "nowhere to appear"
  argument covers all of them, not just the closed contract). A dedicated
  fixture pins WriteClosed × no TOC × no diagnostics.
- The ≤1280 inline TOC is new (the shipped media query silently discarded
  reading aids on laptop widths); it is a native disclosure, closed by
  default, zero JS.
- The TOC gets no show/hide toggle beyond that; the rail is already sticky
  and independently scrollable (shipped) — keep, don't re-add.

## 7. Motion, loading, feedback

- **Navigation**: §2's view transitions — the single highest-leverage polish
  item.
- **Seal feedback**: POST-redirect-GET stays (D27). The redirect carries
  `?sealed=1`; the updated status chip plays one ~400ms pulse and a
  server-rendered toast auto-fades — **both are CSS and play with or
  without JS** (the earlier "minus the pulse" phrasing was wrong). JS's
  only role is cosmetic: stripping the param via `history.replaceState`.
  With JS off, a reload or bookmark of the sealed URL replays the pulse —
  accepted for a single-user local tool, recorded here so nobody "fixes"
  it into a nonce system.
- **Press-and-hold on `ready`** stays; its hold-fill is essential progress
  feedback and survives reduced-motion (as shipped).
- **Loading**: no global spinner; buttons get a busy state on submit; the
  mermaid container shows a CSS shimmer until the client render replaces it.
- **TOC scroll-spy** (IntersectionObserver, enhancement-only), coordinated
  with smooth scrolling: on a TOC click the clicked target is locked as the
  active entry until `scrollend` (short timeout fallback), so the spy does
  not flicker through every intermediate heading; the `:target` arrival
  echo (§12.5) keys off the same settle moment.

## 8. Acceptance criteria (for the PRs implementing this plan)

1. Keyboard-only pass: every sidebar entry, filter, TOC link, and seal form
   reachable and operable; focus visible throughout.
2. JS disabled: every page renders, navigates, and seals correctly; the
   filter box is absent; disclosure falls back to server defaults; no
   layout depends on script.
3. `prefers-reduced-motion: reduce` kills every **decorative** transition
   and animation (view transitions, disclosure, pulses, echoes, shimmer) —
   asserted by computed style; **essential progress feedback survives**
   (the seal hold-fill, the reading-position hairline), also asserted; the
   hairline's survival requires its explicit exemption from the blanket
   kill (§12.9). Until the screenshot e2e exists, the CSS-borne halves of
   these guarantees (the exemption selector, the reduced-motion
   view-transition rule) are locked by stylesheet-text assertions — crude,
   but a lock that can go red beats a manual check that cannot.
4. Wayfinding invariants on the three representative fixtures (concept,
   lesson, no-frontmatter Sources note), plus the multi-section fixture.
5. The write face is reachable in every layout state — including the
   WriteClosed × no-aids fixture (§6) — template-level tests plus one
   screenshot per matrix row.
6. Screenshot set at 1600 and 1280 widths (1280 proves the inline TOC) for
   home, a concept note, a lesson, and a no-heading note.
7. No new dependency without a D41 admission recorded in `decisions.md`.
8. Empty states are designed, not accidental: search no-results, filter
   no-match, empty sections — each says what happened and what to do next,
   in the interface's quiet register.
9. `standards.md` §5 protocol, as for every PR.

## 9. Resolutions (ruled 2026-07-06; item 1 by Koopa, 2–15 delegated to the
guide; §14 records the 2026-07-07 consolidation round)

1. **Landing** — Koopa: Home v0.5 with the README rendered below (§3).
2. Home blocks and labels — as §3.
3. Sidebar order — as §4.
4. Lifecycle demoted to a collapsed section — confirmed (retires per §13.6).
5. Paths closed by default, active path(s) auto-opened — confirmed.
6. "Here" siblings block — built.
7. No sidebar resize — confirmed.
8. Sidebar filter box — built.
9. View transitions, title as shared element, write path excluded — per §2's
   consolidated recipe.
10. Seal feedback = chip pulse + server-rendered toast — per §7's corrected
    wording.
11. TOC toggle — stays deferred; the ≤1280 inline TOC (§6) is a layout
    necessity, not the deferred preference toggle.
12. TOC scroll-spy — build, with §7's settle coordination.
13. Mermaid shimmer skeleton — build.
14. D41 admission criteria — stand as written.
15. The ambient number on the collapsed Lifecycle summary — **the
    adjudication backlog**: a note counts when its current status appears as
    a concrete (non-wildcard) `from` predecessor of at least one lifecycle
    stage whose owner includes the vault's owner, and is not the seal status
    itself. Derived from the loaded contract at render time — never a
    hardcoded status list (wall 3); the seal-status constant is the one
    sanctioned special case. The wildcard archive escape names no pending
    work; agent-advanced statuses are the agents' queue; promotion beyond
    the seal is opt-in. Against the 2026-07-07 contract this yields
    imported + draft (an illustration, not a list). Rendered with an
    accessible label ("N to decide"), not a bare number with a tooltip.

## 10. Platform-feature audit (2026-07-07; re-run when a new UI surface lands)

- **In use**: `<details>`/`<summary>`, `<dialog>` (modal — background
  inertness comes free from `showModal`; the `inert` attribute needs no
  separate adoption), `aria-current`, `interpolate-size` +
  `::details-content`, `content-visibility`, `color-mix`, `text-wrap`,
  `overflow-wrap`, `:focus-visible`, `requestSubmit`.
- **Planned, specced here**: cross-document view transitions (§2), the
  scroll-spy IntersectionObserver (§7), the §12 inventory, the hover layer
  (§11).
- **Rejected with reasons, do not re-adopt without a new ruling**:
  `<details name>` exclusive accordions — exclusivity fights the wayfinding
  invariant (multi-section membership holds several branches open at once).
- **Watch**: `ruby-overhang` — the lesson pages are ruby-dense (furigana);
  act only when a rendering complaint exists.
- **Not applicable**: lazy iframes (the report iframe is the page's sole,
  immediately-visible content), declarative shadow DOM, media/GPU/telemetry
  surfaces — no consumer in a local server-rendered reader.

## 11. The hover layer (ruled 2026-07-07; its own PR after the motion batch)

Reading a wikilink-dense vault wants the glance-without-leaving move. Two
surfaces, one mechanism — the popover attribute for the top layer and light
dismiss, CSS anchor positioning (`anchor-name`, `position-area`,
`@position-try` flip fallbacks, `position-visibility`) for placement; no
positioning JS, ever.

1. **Wikilink hover preview.** Hovering or focusing a resolved wikilink
   opens an anchored card with the target note's beginning (title, status
   chip, first rendered blocks). Content comes from a small read-only
   fragment endpoint reusing the existing rendering pipeline — same
   sanitization, same dialect, truncated at the server; journal pages render
   like any other (local reading is not egress). Enhancement JS owns only
   intent and fetch: ~250ms hover delay, focus triggers for keyboard, Esc
   dismisses, one in-flight fetch. **Freshness rides HTTP, not a JS cache**:
   the endpoint sets an ETag from the content hash and the browser's
   conditional GET does the rest — a preview can never outlive the note's
   next edit by more than a request. No JS → links navigate as today.
2. **In-place diagnostic cards.** The renderer already knows, per link, why
   it is broken or ambiguous; it embeds that detail (for collisions, the
   candidate list) as data attributes at render time. Hover/focus anchors a
   card showing the detail where the problem sits — no fetch. The right-rail
   diagnostics list stays; this connects each entry to its location.

**Overlay discipline** (this layer joins existing floating surfaces): one
floating surface at a time, globally — opening a preview closes any other;
Esc dismisses the topmost surface first (popover, then dialog);
**concept-sheet triggers are exempt from generic link previews** — they
already have their own richer surface, and stacking a preview under a sheet
is noise. Any open popover is dismissed on `pagehide`, before a view
transition snapshots the page.

**New-route obligations** (the fragment endpoint is a new production GET):
it joins the hand-maintained route lists — the read-faces-never-write
wall-lock test and the e2e smoke's face assertions — in the same PR that
adds it. A route that dodges the sweeps does not exist.

Bounds: previews never nest; the card is display furniture — never a write
surface (the seal stays on the page proper). Deferred until felt: footnote
previews; converting the concept sheet from modal to anchored popover
(reopens a built surface — Koopa's taste call, offered not assumed).

## 12. Reading-smoothness inventory (rides with the motion batch)

A reader's product is typography and scroll feel. **The scroll container in
this shell is `.k-main` (the rails scroll independently); every scroll
property below lands there, not on the document — on `html` it is a no-op.**

1. **Smooth anchor travel** — `scroll-behavior: smooth` on `.k-main`
   (headings already carry `scroll-margin-top`). Known platform edge,
   accepted: a second fragment click while a smooth scroll is still in
   flight is swallowed by Chromium; intercepting navigation to fix it would
   rebuild the platform, so it stays.
2. **Scroll containment** — `overscroll-behavior: contain` on `.k-main` and
   both rails: reaching an end stops chaining into the parent.
3. **Prose wrapping** — `text-wrap: pretty` on article paragraphs (titles
   keep `balance`).
4. **CJK punctuation trim** — `text-spacing-trim` on the article; judged
   visually against real lesson and zh-Hant concept pages at acceptance.
5. **Arrival echo** — a brief `:target` highlight on the heading jumped to,
   keyed to the settle moment (§7), so the echo plays when the eye arrives,
   not while the viewport is still traveling.
6. **Entry transitions** — `@starting-style` for the seal toast, dialogs,
   and the hover layer's popovers.
7. **Stable gutter** — `scrollbar-gutter: stable` on `.k-main`.
8. **Phrase-aware Japanese wrapping** — `word-break: auto-phrase` where
   `lang="ja"` exists; the renderer does not language-tag prose runs today,
   and widening that waits for real pain.
9. **Reading-position hairline** — ruled in: the header's own bottom edge
   doubles as the indicator (its border-color carries the progress fill —
   no second line fighting the existing 1px seam), driven by
   `animation-timeline: scroll()` reading `.k-main`, `aria-hidden` (it is
   scroll position, not completion — the syllabus bars own "done").
   **Reduced-motion note**: it survives (essential state display), which
   requires adding it to the blanket kill's exemption alongside
   `.k-sealfill` — and the blanket's duration-crush would not disable a
   scroll-driven animation anyway (`animation-timeline`, not time, drives
   it); the exemption must be explicit either way.

## 13. The sidebar grows from the content (ruled 2026-07-07)

The vault keeps sprouting pillars; hand-coded sidebar categories go stale
the day a new one lands. **The vault's own maps are the navigation.**

1. **Generalize the tree-builder.** The path tree (headings → sections,
   resolved wikilinks → entries) is how any map note works. Extend the nav
   model to build the same tree for every map-typed note (`moc`,
   `topic-map`, `source-map`, alongside `study-path`), from the types the
   contract already defines. **The rename cascade is part of the work**:
   the nav model's path-flavored names (`Syllabus`, `Section`, `Lesson`,
   `typeStudyPath`, `Placement.SyllabusRelPath`, `openSyllabi`, the "Open
   study path" label) generalize honestly per this repo's naming rules — a
   `moc` served by a type named `Syllabus` is the kind of lie the naming
   rules exist to prevent. The `/syllabus/` route keeps its name (it renders
   study-paths; maps are notes and already have a page).
2. **Only what exists appears.** A tree entry is a resolved wikilink; the
   reading map's hundreds of unwritten rows stay on the map's own page. The
   sidebar shows the written vault and grows as fast as the writing does.
3. **Wayfinding generalizes.** The reverse index extends to every map:
   reading a humanities 心得 auto-opens the reading map at its theme
   section, current entry marked — the same arrival a lesson gets.
4. **Grouping**: **Paths** (study-paths, as built), then **Maps** (one
   `<details>` per map, collapsed, ordered by domain then title). A new map
   in the vault is a new section with zero yomihon changes.
5. **Journal is content too**: a **Journal** section — most recent entries
   under `Diary/`, newest first, small fixed count, collapsed — below Maps.
   Local reading is legitimate and always was (D39/D42 guard machine
   outputs, not the owner's eyes); it just never had a door better than the
   Folders tree. Renders nothing when the directory is empty.
6. **Lifecycle leaves the sidebar** once Home v0.5 lands: the strip on Home
   is its home; the ambient backlog number moves to a quiet topbar chip
   (accessible label, §9.15) linking there. End state: the sidebar is pure
   wayfinding, Home/cockpit is status, search is query — three questions,
   three surfaces, no overlap.

Lands as `PR-ux-b3`; the search page's shell adoption ships **with the
experience batch** (§2 — the earlier B-lexical assignment that appeared here
was a contradiction, resolved in favor of §2).

## 14. Consolidation record (2026-07-07)

Three-source adversarial round before the experience batch: two internal
lenses (buildability against the shipped code; interaction coherence across
the ruled mechanisms) and an external reviewer; 32 findings, all triaged.
The consequential corrections, so their reasons survive:

1. The view-transition "stable chrome" mechanism was wrong as first worded —
   naming alone cross-fades, it does not stabilize; §2 now carries the
   precise recipe (named chrome hard-cuts, the article morphs, the title
   travels).
2. Scroll properties aimed at the document were no-ops — `.k-main` is the
   scroller; §12 now says so once, for all nine items.
3. The rail collapse could orphan the write face's fail-closed notice, and
   the ≤1280 media query was silently discarding reading aids on laptop
   widths; §6 now carries the exhaustive matrix and the seal-bar guard
   change.
4. Disclosure state had three uncoordinated owners waiting to fight (server
   chain, persistence, filter); §4 now defines one state module and makes
   the active chain non-persistable.
5. The hover layer gained overlay discipline (concept-sheet exemption,
   one-surface rule, pagehide dismissal), HTTP-conditional freshness, and
   new-route sweep obligations.
6. "Resume where I left off" is a real product gap this batch does not
   close: recorded as a D-plan-doc obligation (per-note resume anchor keyed
   by content hash) in roadmap §5a.
7. External-review rejects, with reasons: single-key shortcuts stay live on
   focused links/buttons (keyboard-navigation convention; suppressed only in
   text entry and dialogs); the "missing product.md" finding was an artifact
   of reviewing a branch cut before that document landed on main.
