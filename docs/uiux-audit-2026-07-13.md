# UI/UX adversarial audit — 2026-07-13

Status: **review record, not product canon**. This document records observed
failures, source evidence, candidate dispositions, and unresolved design calls.
It does not amend `product.md`, `decisions.md`, `roadmap.md`, or `ux-plan.md`.
A row marked `NEEDS-RULING` must not be implemented until its owner records the
ruling in the owning document.

## 1. Scope and review method

This review treats accessibility conformance as a floor, not the product-quality
ceiling. It combines five lenses rather than trusting one checklist or one
reviewer:

1. product positioning, information architecture, and semantic honesty;
2. brand identity, visual hierarchy, typography, and interaction taste;
3. keyboard, screen-reader structure, contrast, zoom, and user preferences;
4. failure-space testing across navigation, history, narrow windows, no results,
   missing routes, long content, multiple tabs, and JavaScript degradation;
5. implementation feasibility under the server-rendered MPA, native-web-first,
   Baseline 2026, zero-JS write path, and the four walls.

The live probes used the real vault at `/Users/koopa/obsidian` and the local
loopback server. Automated accessibility output was treated as a lead and
checked against the DOM or source; it is not used as a standalone verdict.

Authority labels used below:

| Label | Meaning |
|---|---|
| `VERIFIED` | Reproduced in the live browser or direct HTTP response on 2026-07-13. |
| `SOURCE-VERIFIED` | Current source establishes the behavior; no live claim is implied. |
| `CANON-DRIFT` | Shipped browser behavior contradicts an existing ruling or product principle. |
| `QUEUED` | `program.md` already owns a named repair unit; this report must not duplicate it. |
| `NEEDS-RULING` | The pain is real, but canon does not yet authorize the proposed behavior. |
| `RISK` | A plausible edge remains unverified on the required browser or assistive technology. |
| `REJECT` | Deliberately do not build the superficially attractive solution. |

Severity is user consequence, not visual embarrassment:

- **P0** — data loss, unsafe write, or a core path impossible to complete;
- **P1** — product dishonesty, inaccessible core action, lost reading context,
  or a write/status control made unreachable;
- **P2** — recurring comprehension, navigation, preference, recovery, or brand
  failure with a workaround;
- **P3** — polish or resilience debt whose cost is intermittent.

No P0 was found in this review. That is not permission to understate the P1s.

## 2. Executive verdict

yomihon's article typography and scholar's-desk direction are distinctive, but
the surrounding product chrome is not yet a coherent product system. The
largest problems are not a missing shadow or animation curve:

- the header teaches the wrong mental model (`STOREHOUSE`) and presents a
  lifecycle-derived number as though it were actionable pending work;
- navigation motion and state preservation sometimes destroy rather than
  preserve context;
- important status controls and task semantics can become unreachable;
- the shipped interface still contradicts its Traditional Chinese chrome rule;
- bounded personalization is authorized in principle but has no coherent,
  discoverable surface;
- browser-tab identity, general error recovery, and several empty states have
  not been designed as product surfaces.

The correct response is not a redesign from scratch. Preserve the reading
column, status write contract, native elements, and server-rendered shell;
repair truth, context, accessibility, and global identity around them.

### 2.1 Cross-review consensus and resolved objections

The product/brand, accessibility/resilience, and frontend-feasibility reviews
were performed independently before findings were compared. The following
points survived explicit cross-questioning:

- **No-JS seal behavior stays, its presentation changes.** D27 intentionally
  keeps a real one-press submit when JavaScript is absent. The defect is that
  SSR still promises “Hold to certify” and shows hold-only ceremony. Baseline
  copy must describe one submit; only successful JS initialization may reveal
  hold instructions and change the accessible name. Making no-JS require a
  second confirmation would be a new ruling.
- **The drawer completes its existing modal-like contract.** A Tab loop is not
  background isolation. When open, content outside the drawer must be inert;
  opening the native search dialog closes the drawer first so one overlay owns
  focus. This fulfills the current contract and does not justify an overlay
  framework.
- **Status failures become recoverable HTML without weakening their HTTP
  semantics.** Expected 409/422/503 and commit-failure states need Traditional
  Chinese headings, exact consequences, and ordinary links back to fresh note
  state. Raw/static/report security refusals remain minimal.

One frontend suggestion was rejected during reconciliation: commit-failure
details cannot be moved only to the server log. `spec.md` §4 explicitly requires
the raw git text and manual-remediation instructions because the note may already
have been rewritten. The repair must present that information safely inside the
human recovery page, not erase it.

## 3. Findings register

| ID | Severity | Authority | Finding | Disposition |
|---|---:|---|---|---|
| UX-01 | P1 | `VERIFIED`, `QUEUED` | The header's bare `181` is the count of notes with an owner-held legal onward move, not a proven pending-decision queue. It links to generic Home. | Execute `program.md` 9c-iv: hide it until an independent pending signal exists. Do not solve this with a longer tooltip. |
| UX-02 | P1 | `VERIFIED` | Cross-document navigation can show old drawer/scrim and new content together before settling, creating double exposure and an apparent stall. | Re-open the transition recipe against the actual animated groups; prefer a shorter hard cut when continuity cannot be made truthful. Add a timing-aware browser lock. |
| UX-03 | P1 | `VERIFIED` | The independently scrolling reading container loses its scroll position on note navigation and browser Back. | The D face already owns durable per-note resume state; separately restore same-history scroll for ordinary Back/Forward without waiting for D. |
| UX-04 | P1 | `VERIFIED`, `QUEUED` | A long TOC can shrink the right-rail status panel until legal transition forms are permanently clipped. | Execute `program.md` 9c-viii and its real-page acceptance matrix. |
| UX-05 | P1 | `VERIFIED` | Rendered GFM task checkboxes have no accessible name; the L20 probe exposed 16 critical unnamed controls. | Give each disabled checkbox a name from its task text or render task semantics without exposing a nameless control. Lock the accessibility tree, not only the HTML substring. |
| UX-06 | P1 | `VERIFIED` | Muted/faint text and component boundaries repeatedly fall below usable contrast; automated runs reported serious contrast findings on Home, Search, and L20. | Rebalance the base tokens, then verify light, dark, focus, disabled, status, and non-text boundaries. Preferences may not be used to repair the default. |
| UX-07 | P2 | `SOURCE-VERIFIED`, `CANON-DRIFT` | `STOREHOUSE` is an undefined decorative storage metaphor, not a product mode or user task. | Remove it. Do not replace it with another slogan until a concrete comprehension problem requires one. |
| UX-08 | P2 | `VERIFIED`, `CANON-DRIFT`, `QUEUED` | Browser chrome, instructions, diagnostics, placeholders, counts, and accessible names remain heavily English despite D28. | Execute `program.md` 9c-vii. Keep authored content and technical proper names in their own language; do not add an i18n framework. |
| UX-09 | P2 | `VERIFIED`, `NEEDS-RULING` | There is no coherent reading-preference surface. `/settings` is 404; the current system-dark preference is ignored on a clean first visit; theme and ruby are isolated cryptic toggles. | Rule a bounded `閱讀顯示` surface under D48. A full generic Settings area is not automatically justified. |
| UX-10 | P2 | `VERIFIED`, `NEEDS-RULING` | The product has no favicon/app mark asset. The current text wordmark plus 5 px dot does not form a reusable small-scale identity. | Koopa rules the mark direction. Ship at least app mark, wordmark treatment, and SVG favicon; no marketing brand suite is required. |
| UX-11 | P2 | `VERIFIED`, `NEEDS-RULING` | General missing routes, `/settings`, `/favicon.ico`, manifest, and apple-touch paths all return the same unbranded plain-text 404. | Give human-facing UI routes a quiet shell 404 with recovery. Preserve silent/plain 404 behavior where raw/report/static security semantics require it. |
| UX-12 | P2 | `VERIFIED` | `/search` has no `h1`; the zero-result state is low-contrast English text in a large empty surface with no clear recovery action. | Add a real page heading and recovery controls such as clear query, edit filters, and return to the prior surface. Keep the plain GET path. |
| UX-13 | P2 | `SOURCE-VERIFIED`, `NEEDS-RULING` | The left rail is fixed at 264 px and long labels ellipsize without a reliable full-name affordance. The current UX plan explicitly ruled out resize. | Re-open only because daily-use pain is now reported. First compare better wrapping/context labels and narrow/default/wide presets; do not add a rearrangeable workspace. |
| UX-14 | P2 | `VERIFIED` | Navigating to a deep note resets the sidebar scroll to the top and can leave the current item far below the viewport. | After server wayfinding and disclosure restoration, scroll the current entry into view only when it is outside the rail viewport; avoid stealing the user's intentional rail position. |
| UX-15 | P2 | `VERIFIED` | Long unbroken search snippets can widen the live-results region and create a horizontal scrollbar. | Apply wrapping/overflow containment to result metadata/snippets while preserving local scrolling for real code blocks. |
| UX-16 | P2 | `VERIFIED` | TOC scroll-spy can mark a different item from the clicked hash target after navigation settles. | Define the clicked-target lock and settle oracle against the actual scroll container; add a probe that asserts hash, target, and `aria-current` agree. |
| UX-17 | P2 | `SOURCE-VERIFIED` | No skip link precedes the repeated global header and large navigation rail. | Add a Traditional Chinese skip link targeting a focusable `main`; verify it becomes visible on focus and lands inside the page's real scroll container. |
| UX-18 | P2 | `VERIFIED`, `QUEUED` | Real Chrome reproduced page-level horizontal travel at 360 CSS px, with shared header controls outside the viewport. | Execute `program.md` 9c-vi at 360/450/900. A separate in-app-browser probe did not reproduce the overflow, so keep the acceptance tied to system Chrome rather than declaring a universal layout fact. |
| UX-19 | P3 | `VERIFIED` | Two open tabs can display different themes after one tab changes the preference. | Treat cross-tab synchronization as optional resilience. If adopted, use a small shared preference channel; do not introduce a client store. |
| UX-20 | P3 | `SOURCE-VERIFIED` | With JavaScript unavailable, theme and ruby controls remain visible but cannot perform their advertised action. | Either provide a small form-based fallback or reveal the controls only when enhancement is active. Core reading and status writes must remain unaffected. |
| UX-21 | P3 | `SOURCE-VERIFIED` | `/`, `[`, and held `R` are deliberately global but have no discoverable shortcut reference. | A small help/shortcut disclosure may document the accepted D49 deviation; remapping or disabling still needs a separate D49 ruling. |
| UX-22 | P3 | `RISK` | Forced-colors, `prefers-contrast`, 200% zoom, VoiceOver reading order, and Safari transition behavior do not have current end-to-end evidence. | Do not claim support from token design or axe output. Run the matrix in section 8 before closing related findings. |
| UX-23 | P1 | `SOURCE-VERIFIED`, `CANON-DRIFT` | With no JavaScript, the seal submits on one press while SSR copy and ARIA promise a hold gesture that does not exist. | Preserve D27's one-press fallback; reveal hold copy and update the accessible name only after the hold enhancement initializes. |
| UX-24 | P2 | `SOURCE-VERIFIED` | The narrow drawer loops Tab but does not make background content inert and can overlap with the search dialog. | Complete the existing modal-like contract with outside inertness, one-overlay ownership, a named drawer, and deterministic focus return. |
| UX-25 | P1 | `SOURCE-VERIFIED`, `CANON-DRIFT` | Status 409/422/503/500 responses leave the app as English plain text, including the state where a note was rewritten but git commit failed. | Render browser-facing recovery HTML with the same status codes and D28 language. Preserve spec-required raw git output and manual remediation for commit failure. |
| UX-26 | P1 | `SOURCE-VERIFIED`, `CANON-DRIFT`, `NEEDS-RULING` | Every note article is hard-coded `lang="zh-Hant"`, including Japanese and English authored content. | Keep chrome `zh-Hant`; obtain article language from an authoritative content signal. Do not guess from domain or path. Keep local `lang="ja"` on known Japanese reading regions. |
| UX-27 | P2 | `SOURCE-VERIFIED` | The search dialog has no accessible name; the concept dialog is always named English “Grammar note” rather than the opened concept. | Give dialogs real headings and connect them with `aria-labelledby`; verify open/close and focus return in VoiceOver/Safari. |
| UX-28 | P2 | `SOURCE-VERIFIED` | Syllabus part and module labels look like headings but are spans, so the screen-reader heading outline contains only the page h1. | Establish an h2/h3 recursion policy and render real headings without flattening source order. |
| UX-29 | P2 | `SOURCE-VERIFIED`, `NEEDS-RULING` | The inline pre-paint script reveals the sidebar filter before the external behavior script is known to have initialized; a failed script leaves an inert-looking but nonfunctional field. | The pre-paint reveal is an explicit anti-pop-in ruling. Decide whether failed external JS is accepted degradation or whether that ruling is amended; this report does not authorize changing it. |
| UX-30 | P2 | `SOURCE-VERIFIED` | Filter Escape clears and calls `blur()`, leaving keyboard focus without a meaningful destination. | Restore the prior trigger/current item/main target; never use blur as the final focus-management instruction. |
| UX-31 | P2 | `SOURCE-VERIFIED` | Wide note pages expose two unnamed complementary landmarks, so assistive-tech users cannot distinguish the left and right rails. | Give each aside a stable Traditional Chinese accessible name matching its responsibility. |

## 4. Header, identity, and information architecture

### 4.1 `STOREHOUSE` teaches the wrong concept

`internal/ui/layouts/base.templ:54-57` renders the name, a decorative dot, and
`STOREHOUSE`; the same metaphor appears in the search placeholder at lines
85-90. No mode, route, decision, or task is named Storehouse. The product is a
human terminal for reading and adjudication, not a storage manager
(`docs/product.md` §§1 and 3). The label also violates D28's Traditional
Chinese browser-chrome rule.

This is not a translation task. Translating it to `倉庫` would preserve the
wrong mental model. Remove it.

### 4.2 The advanceable number is not honest enough for global chrome

`internal/note/handler.go:351-382` computes the figure from notes whose current
type/status has an owner-held onward transition, excluding the seal. The
implementation comment calls this “awaiting a decision,” but that interpretation
does not follow from the predicate. `internal/ui/layouts/base.templ:60-61` then
renders only the number, hides the explanation in English ARIA/title text, and
links to `/`.

Global chrome is too expensive for an aggregate that cannot answer “what should
I do next?” The accepted direction is:

1. hide the current number;
2. let the D face define an independent pending signal and a real queue;
3. if the signal lands, show a visible Traditional Chinese label with the unit
   and link directly to the filtered work, not generic Home.

### 4.3 Minimum identity system

The document head at `internal/ui/layouts/base.templ:25-30` carries no icon or
theme metadata, and the asset registry has no favicon/logo route. The current
wordmark treatment in `assets/css/components.css:120-123` is a useful seed, not
a complete identity system.

Minimum deliverable after a taste ruling:

- one app mark legible at 16, 24, and 32 CSS px;
- the existing `yomihon` wordmark refined to pair with it;
- monochrome and dark-surface behavior;
- one local SVG favicon served explicitly and referenced from `<head>`.

Do not add a manifest, apple-touch icon, offline shell, or install flow unless
installability becomes a product requirement.

### 4.4 Candidate header hierarchy

This is a discussion aid, not an approved mock:

```text
[drawer] [mark + yomihon]            [search ⌘K] [pending N*] [Aa reading]
```

`pending N` is absent until the D face supplies a truthful signal. `Aa reading`
consolidates theme, ruby, and future presentation preferences instead of adding
another control to an already overflowing header.

## 5. Bounded reading preferences

D48 permits presentation preferences but explicitly calls its dimensions
candidates, not build commitments (`docs/decisions.md:359-388`). The roadmap
allows an Aa popover or existing controls and permits a Settings page only when
grouping and explanation justify it (`docs/roadmap.md:66-75`). The current
implementation supports only `light|dark` and `ruby on|off`
(`internal/ui/pages/href.go:111-125`; `assets/js/yomihon.js:36-54`).

The 2026-07-13 clean-state probe found:

- `matchMedia('(prefers-color-scheme: dark)').matches === true`;
- root `data-theme === 'light'`;
- no saved theme preference;
- the theme control exposed only the English name `Toggle theme`;
- after changing theme in tab A, tab B stayed on the old theme until reload.

Candidate scope requiring a ruling:

| Control | Candidate shape | Constraint |
|---|---|---|
| Theme | system / light / dark | System should be the untouched-default behavior; SSR must avoid a wrong-theme flash. |
| Reading palette | at most a few validated paper/contrast presets | No arbitrary color picker; status/accent meaning and AA contrast stay fixed. |
| Reading font | curated self-hosted/system presets | Article typography only; no remote font URL. |
| Type size and line height | bounded steps with a live sample | Must survive ruby, code, tables, and 200% browser zoom. |
| Measure and paragraph spacing | bounded presets | No shell rearrangement and no authoritative content hidden. |
| Ruby | visibility, size, contrast | Keep authored ruby in the DOM and preserve `data-keep` semantics. |
| Density | quiet / compact reading presets | Do not turn the article into a dashboard. |
| Reset | one explicit return to house defaults | Every preference is reversible. |

Preferred native shape: an `Aa` trigger with one native popover backed by
ordinary form controls; CSS may present that same surface as a bottom sheet at
narrow widths. Do not duplicate its controls into separate popover and dialog
trees. A dedicated generic Settings application, client-side state store, theme
marketplace, custom CSS, remote fonts, and drag-and-drop layout remain rejected.

## 6. Navigation, motion, and state continuity

### 6.1 Motion currently contradicts its reason for existing

The UX plan says motion exists to preserve context, and the CSS deliberately
hard-cuts chrome while allowing the reading column and title to transition
(`docs/ux-plan.md` §2; `assets/css/components.css:903-935`). In a 900 px drawer
flow, navigating from L20 to L10 still showed the old drawer/scrim together
with new content at approximately 140 ms and settled only around 450 ms. Server
DOMContentLoaded timings were approximately 121–196 ms, so the perceived stall
was not explained by backend latency.

The repair must observe the instant the user sees, not merely the final computed
style. A browser probe should sample immediately after navigation, during the
transition, and after settlement. If the article/title choreography cannot avoid
double exposure, a fast hard cut is better than semantically false motion.

### 6.2 History and scroll position

Observed sequences:

- Home main scroll `1800` → open note → Back → Home main scroll `0`;
- syllabus scroll `900` → open L10 → Back → syllabus scroll `0`;
- left rail scroll around `1618` → deep note navigation → rail scroll `0`, with
  the current item still around `1146` px below the rail top.

The page uses `.y-main` as the real scroll container
(`assets/css/components.css:828-838`), so relying on browser window restoration
does not restore the user's reading context. Distinguish two responsibilities:

- same-history Back/Forward should restore the scroll container now;
- durable per-note resume across later sessions remains the D face's state and
  content-hash design (`docs/roadmap.md` §5a).

### 6.3 Sidebar width and context

The fixed 264 px width and “no resize” decision are explicit
(`docs/ux-plan.md` §4 and §9). The new report that the sidebar cannot be resized
is therefore not a missing implementation; it is evidence that the earlier
assumption may no longer fit daily use.

Before reversing it, compare three bounded answers:

1. allow two-line wrapping for selected/current entries and reveal full names on
   focus;
2. improve contextual labels so paths need less repeated text;
3. offer narrow/default/wide width presets with an obvious reset.

Any persisted width is local, per-device presentation state and needs the D48
storage/ruling discussion. A free drag handle would additionally need a
focusable separator, keyboard adjustment, min/max clamps, and its own evidence;
CSS `resize` alone is not an accessible solution. A draggable/rearrangeable
workspace remains out of scope.

## 7. Accessibility, recovery, and degraded states

### 7.1 Write-path honesty and recovery

The seal form deliberately remains a native POST when JavaScript is absent
(`internal/ui/pages/note.templ:526-540`). However, the server-rendered button
already says `Hold to certify`, shows `hold R`, and renders a hold poem, while
the actual guard is installed later by `assets/js/yomihon.js:176-195`.

The accepted repair is presentation-only:

1. SSR presents an honest one-submit action and no hold-only instruction;
2. successful JS hold initialization reveals the ceremony and updates the
   accessible name;
3. the completed hold still calls `form.requestSubmit()` and never `fetch`;
4. failure to load JS leaves a truthful, working D27 fallback.

`internal/status/handler.go:77-125` sends all expected failure classes through
`http.Error`. These are not developer-only exceptions. A stale tab, concurrent
write, dirty file, illegal transition, unavailable contract, and commit failure
are safety states the owner must understand and recover from.

Human-facing error responses should retain their precise HTTP code and render:

- one Traditional Chinese `h1` naming the state;
- whether the vault file changed;
- what the owner must do next;
- an ordinary link back to the original note or a fresh GET;
- for commit failure, the raw git text and manual-remediation instructions
  required by `spec.md` §4.

No automatic retry, resubmission, or client toast replaces this page.

### 7.2 Drawer and overlay ownership

`assets/js/yomihon.js:58-118` makes the closed rail inert and loops Tab inside
the open rail, but it does not isolate the background. Pointer users and an
assistive-technology virtual cursor can still reach the underlying header,
main, right rail, or fixed seal bar. The drawer can also coexist with the
native search dialog, leaving focus return ambiguous.

The existing modal-like contract should be completed with native `inert` on
outside regions, a stable drawer name, and a rule that opening a native modal
first closes the drawer. Do not replace this with a custom focus trap or overlay
manager.

### 7.3 Language and semantic outline

The document chrome correctly declares `zh-Hant`, but the note template also
forces every article to `lang="zh-Hant"`
(`internal/ui/pages/note.templ:86-98`). That contradicts D28's requirement that
vault content keep its authored language. It can alter pronunciation, voice
selection, and font fallback for Japanese or English notes.

The defect is verified; the source of note-level language is unresolved. Do not
infer it from a folder, domain, title, or the presence of ruby without a ruling.
Known Japanese interaction regions continue to carry local `lang="ja"` while
that source is designed.

Other structural defects are mechanical:

- `internal/ui/layouts/base.templ:85-98` gives the search dialog no name;
- `internal/ui/pages/note.templ:126-140` gives every concept dialog the fixed
  English name `Grammar note` instead of its dynamic title;
- `internal/ui/pages/syllabus.templ:67-104` styles part/module spans as headings
  without adding them to the heading outline;
- `internal/ui/pages/note.templ:181-193` and
  `internal/ui/pages/note.templ:416-428` expose two unnamed complementary
  landmarks.

### 7.4 Evidence from the live surface

The accessibility scan and DOM inspection found:

| Surface | Evidence |
|---|---|
| Home | 31 serious contrast findings plus a scrollable-region finding. |
| Search | 3 contrast findings and no `h1`. |
| L20 | 6 contrast findings and 16 critical unnamed task checkboxes. |
| Global shell | No skip link before the repeated header and navigation. |
| Header | Icon/toggle names and visible copy remain English; the number's meaning is not visible. |

Token calculations independently put several faint combinations below 4.5:1,
including approximately 3.79:1 on the light page background, 4.04:1 on a panel,
and 3.56:1 on an elevated surface. Borders were often around 1.3–1.9:1. These
values identify the token layer as the repair seam; they do not imply every use
of a faint token is body text or an essential component boundary.

### 7.5 Error and empty-state taxonomy

Direct HTTP probes returned `404 text/plain` with `404 page not found` for
`/settings`, `/favicon.ico`, `/manifest.webmanifest`,
`/apple-touch-icon.png`, and an arbitrary missing route. Not all 404s should
share a presentation:

| Route class | Presentation |
|---|---|
| Human-facing shell route | Quiet branded error in the shared shell; explain what happened and offer Home/Search/Back. |
| Missing note/file the user navigated to | Same shell plus the path and recovery, without guessing a replacement. |
| Raw, static, report allowlist, or security refusal | Preserve minimal/plain response when silence is part of the boundary. |

Search zero-results should explain the active query/filter, allow clearing or
editing it, and retain the ordinary GET form. “No results” is not an error and
must not use an urgent alert.

### 7.6 Long and hostile content

The query `GC` produced 87 results; one long JWT-like token made the dialog's
results region `687` px wide inside a `589` px client box. Paths, snippets,
titles, code, tables, ruby, and mixed CJK/Latin tokens need separate policies:

- prose metadata and snippets wrap (`overflow-wrap:anywhere` where appropriate);
- code and true tabular data scroll locally;
- no child is allowed to widen the page or hide the write face;
- truncation must retain a focus-visible way to learn the full value.

### 7.7 Unverified accessibility boundary

The following are not closed by this audit:

- VoiceOver + Safari reading order and dialog announcements;
- 200% text/browser zoom with every status action reachable;
- forced-colors mode and Windows High Contrast;
- `prefers-contrast: more` behavior for the intentionally quiet palette;
- Safari cross-document transition and customizable-select fallbacks;
- a real keyboard-only pass across long off-screen sidebar content.

These remain `RISK`, not assumed defects and not assumed support.

## 8. Browser acceptance inventory and representative scenarios

The axes below enumerate the failure space; they are **not** a Cartesian-product
test requirement. Each work unit selects the smallest pairwise/risk-based set
that covers its changed predicates and records why omitted combinations are
equivalent. A safety-critical state machine may still require its own full
predicate cross-product, but a global chrome repair does not inherit millions of
manual browser cases.

| Axis | Required cases |
|---|---|
| Viewport | 360×900, 450×900, 900×900, 1280×720, 1600×768, 1600×900 |
| Zoom | 100%, 200% |
| Appearance | clean/system, explicit light, explicit dark, reduced motion, forced colors |
| Input | pointer, keyboard-only, VoiceOver/Safari |
| Runtime | JavaScript on, JavaScript off, two tabs with different navigation histories |
| Page role | Home, search empty/many results, syllabus, long Japanese lesson, long Go note, no-heading note, diagnostics-heavy note, raw/file view, report |
| Status state | multiple legal transitions, one transition, sealed, no frontmatter, non-instance, contract closed, artifact policy unavailable |
| Navigation | normal link, drawer link, TOC link, Back, Forward, reload, direct deep link |
| Content stress | long title, long path, unbroken token, wide table, code block, dense ruby, 100+ results, empty vault section |

Baseline representative set:

| Scenario | Channel | Covers |
|---|---|---|
| Wide long lesson at 1600×768 and 1600×900 | automated system Chrome | right-rail status reachability, long TOC, diagnostics, keyboard focus, default contrast |
| Laptop note at 1280×720 | automated system Chrome | right-rail handoff to inline aids/seal bar, article width, status visibility |
| Narrow lesson at 900×900 and 360×900 | automated system Chrome | drawer isolation, header containment, overlay ownership, page-level overflow |
| Search with zero results and 100+ results including a long token | automated system Chrome | h1/landmarks, result count/state, wrapping, local versus page overflow |
| JavaScript disabled and external script unavailable | automated system Chrome | truthful seal fallback, filter/theme/ruby degradation, ordinary GET/POST paths |
| Reduced-motion navigation | automated system Chrome | no decorative transition/ghost while essential hold progress remains understandable |
| 200% zoom across a long note and search | manual Chrome/Safari | reflow, focus visibility, bottom status reachability, no two-dimensional page scrolling |
| Japanese lesson with search/concept dialog and drawer | manual Safari + VoiceOver | language changes, dialog names, landmark/heading rotor, overlay and focus return |
| Forced colors / increased contrast where an environment is available | manual | text, focus, current state, status/button boundaries without color-only meaning |
| Two tabs changing an explicit preference | automated or manual | live divergence, reload convergence, optional synchronization if ruled |

A work unit may add a targeted scenario or split one row when its mechanism
cannot be observed honestly through the listed channel. It must not claim the
entire inventory passed because one representative page was green.

Minimum assertions:

1. `scrollWidth <= clientWidth` except inside explicitly local scrolling boxes.
2. Every legal status control is visible, focusable, named, and operable.
3. Focus never enters an inert/closed drawer and returns to the initiating
   control after close.
4. Back/Forward restores the correct content and scroll container.
5. Hash, target heading, visual active TOC item, and accessibility state agree.
6. Decorative motion disappears under reduced motion; essential hold progress
   remains understandable.
7. No normal text is below 4.5:1, and essential component/state boundaries meet
   non-text contrast requirements.
8. The page has one useful `h1`, landmarks, a skip path, and named controls.
9. JavaScript failure never leaves a dangerous or falsely interactive write
   control.
10. The browser makes no third-party request for appearance or identity assets.

## 9. Proposed disposition and sequencing

This report does not create PR units. The guide should reconcile it as follows:

### Already owned; do not duplicate

- UX-01 → `program.md` 9c-iv, advanceable-chip truth;
- UX-18 → 9c-vi, narrow-header overflow;
- UX-08 → 9c-vii, interface-language debt;
- UX-04 → 9c-viii, right-rail flex clipping.

### Mechanical defects that do not need a new taste ruling

- UX-23 truthful no-JS seal presentation;
- UX-24 modal drawer isolation and overlay ownership;
- UX-25 browser-facing status recovery pages;
- UX-05 unnamed task semantics;
- UX-06 default contrast and non-text boundaries;
- UX-12 search heading and actionable empty state;
- UX-14 current-item rail visibility;
- UX-15 result overflow containment;
- UX-16 TOC target/current agreement;
- UX-17 skip link;
- UX-20 no-JS dead preference controls;
- UX-27 dialog naming;
- UX-28 syllabus heading semantics;
- UX-30 deterministic focus restoration;
- UX-31 landmark names.

These still need named, reviewable units and browser locks before implementation.

### Must be ruled before implementation

- UX-09 scope and storage of the reading-preference surface;
- UX-10 logo/app-mark direction;
- UX-11 which 404 route classes enter the shell;
- UX-13 whether the explicit no-resize ruling is reversed;
- UX-19 whether cross-tab preference synchronization earns code;
- UX-21 whether a shortcut-help surface is wanted;
- UX-26 the authoritative source of article language;
- UX-29 whether failed external JS is accepted degradation or reopens the
  ruled pre-paint filter reveal.

### Requires diagnosis/design before a repair is selected

- UX-02 transition double exposure;
- UX-03 same-history scroll restoration versus D-face durable resume state.

## 10. Rejected over-design

The following do not follow from the findings and should stay out:

- a frontend framework, client router, reactive store, or animation library;
- a generic account/profile/settings architecture for one local operator;
- arbitrary accent colors, custom CSS, theme marketplace, remote fonts, or
  rearrangeable panels;
- a PWA manifest, service worker, offline shell, or install flow merely because
  the favicon is missing;
- replacing native dialog/popover/details/forms with custom widgets;
- persisting every transient sidebar and layout movement forever;
- directional slide transitions merely because the platform supports them;
- a logo project large enough to delay product-truth and accessibility repairs;
- treating a perfect axe/Lighthouse score as acceptance.

## 11. Five decisions this review will defend

1. Hide the current `181`; a misleading global number is worse than no number.
2. Remove `STOREHOUSE`; product identity should name the reading/adjudication
   relationship, not decorate a storage metaphor.
3. Preserve the scholar's-desk reading surface and fix its default contrast;
   preferences add taste but never repair an inaccessible default.
4. Consolidate appearance into a bounded `閱讀顯示` surface only after its D48
   ruling; do not grow a workspace customization system.
5. Prefer context-preserving state and instant honest navigation over visible
   animation. When motion ghosts or stalls, remove it before making it fancier.

## 12. Review boundary

Verified in this round: current source, current canon, real-vault browser flows,
direct HTTP states, system-theme mismatch, two-tab divergence, current asset
absence, and the measurements named above.

Not verified in this round: Windows/Linux font rendering, forced-colors,
VoiceOver end-to-end behavior, Safari transition timing, and the final visual
design of any proposed app mark or preferences surface. Those require their own
evidence and, where marked, Koopa's ruling.

## 13. Native Web platform research report — 2026-07-13

This appendix answers the follow-up question about Obsidian-like link previews
and tests the current plan against the supplied Chrome and web.dev material. It
is still review evidence, not a change to product canon.

### 13.1 Questions and pre-research hypothesis

Questions:

1. Is a hover/focus preview for concept and document links missing from the
   plan, or merely not implemented yet?
2. Can it be built as a small native enhancement without a framework,
   positioning library, client router, or duplicated rendering system?
3. Which 2026 platform features improve this product now, which are safe only
   as progressive enhancements, and which should be rejected?

Pre-research hypothesis: the durable baseline must remain a real `<a href>`;
keyboard focus and hover may reveal one nonmodal preview; Popover and CSS anchor
positioning should remove overlay and geometry code, not justify more motion.
Any unsupported enhancement must fall back to ordinary navigation. Experimental
Chrome-only features must not become a Baseline-2026 core path.

### 13.2 Answer: planned, but the current ruling does not fully answer the report

**Status: `QUEUED`, not shipped.** `program.md` 9d already schedules
`PR-ux-c`: resolved-wikilink previews over a read-only fragment endpoint, plus
in-place diagnostic cards, implemented with Popover and CSS anchor positioning
(`program.md:174-176`). `ux-plan.md` specifies title, status and first rendered
blocks; about 250 ms of hover intent; focus activation; Esc dismissal; one
in-flight fetch; ETag freshness; and ordinary-link no-JavaScript behavior
(`ux-plan.md:304-345`).

Current source confirms the absence rather than a browser-only failure:

- the note handler registers only the full note, raw file and home routes; no
  note-preview fragment exists (`internal/note/handler.go:109-114`);
- the 797-line enhancement file has live-search fragment code and a lesson-only
  concept sheet, but no generic wikilink hover/focus owner
  (`assets/js/yomihon.js:366-430`, `assets/js/yomihon.js:740-765`);
- concept links are upgraded only inside governed lesson instances; activation
  clones pre-rendered content into a modal dialog, while the link remains the
  no-JavaScript navigation path (`internal/note/handler.go:211-224`,
  `internal/render/concept.go:15-46`).

There is one material mismatch with the reported need. The current ruling says
that concept-sheet triggers are **exempt** from generic link previews
(`ux-plan.md:329-335`). That means a concept link with the richer click surface
still gives no hover/focus glance. Recommended ruling: narrow the exemption.
Concept links should receive the same transient preview as other resolved
wikilinks; explicit activation may still open the existing concept sheet. On
activation, dismiss the preview before opening the sheet. The invariant should
be “never stack the two surfaces,” not “concept links never preview.” This is a
`NEEDS-RULING` correction to §11, not an implementation assumption.

### 13.3 Recommended native interaction contract

The smallest complete contract is:

1. **Baseline is navigation.** Every trigger remains a real resolved link. With
   no JavaScript, no Popover support or no anchor-positioning support, normal
   navigation remains intact. Coarse/touch input gets no hover preview or
   invented long-press gesture; activation follows the existing ordinary-link
   or concept-sheet contract. A failed or timed-out preview request never
   blocks activation.
2. **One shared transient surface.** Use one `popover="auto"` for the entire
   document. The top layer, light dismiss and Esc behavior are browser-owned;
   focus deliberately remains on the source link. The popover is nonmodal and
   does not make the reading page inert. It must not pretend to be a dialog:
   Popover has no inherent semantics.
3. **One active anchor.** Assign one stable CSS anchor name only to the active
   link and tether the shared popover to it. Use logical `position-area` values,
   `position-try-fallbacks: flip-block, flip-inline`, and
   `position-visibility: anchors-visible`. Reset the UA popover inset. No
   bounding-box calculations, resize listener, scroll listener, or positioning
   library.
4. **Intent, not pointer noise.** Delegated `pointerover`/`pointerout` handles
   fine pointers with the ruled delay; `focusin` opens without hover delay for
   keyboard users. `pointer: coarse` gets no invented long-press gesture.
   Pointer travel between a link and its card needs a short close grace so the
   card does not flicker at the boundary.
5. **One request owner.** A new intent aborts the prior request with
   `AbortController`; a monotonically increasing request identity rejects late
   winners, following the existing live-search race discipline. Browser HTTP
   caching and ETag revalidation own freshness; do not add a second unbounded JS
   cache.
6. **Bounded display furniture.** The card has a fixed content budget, no forms,
   buttons, media, script, embedded browsing contexts, or nested preview
   triggers. Keep focus on the source link. If supplementary semantics are
   exposed, connect the link and loaded card deliberately (for example with
   `aria-details`) and verify the actual VoiceOver announcement; visual anchor
   placement alone creates no semantic relationship.
7. **Overlay arbitration is explicit.** Opening search, the navigation drawer,
   or the concept sheet closes the preview. `pagehide` closes it before a View
   Transition snapshot. One Esc closes only the topmost surface.

Popover is a good fit because it is Baseline 2025 and supplies top-layer,
light-dismiss, keyboard and focus behavior. CSS anchor positioning is designed
to tether Popover/Dialog top-layer elements, provides overflow fallbacks and
subscroller visibility, and requires a feature-detected fallback. Sources:
[Popover API lands in Baseline](https://web.dev/blog/popover-api),
[CSS Anchor Positioning API](https://developer.chrome.com/docs/css-ui/anchor-positioning-api),
and the [web.dev HTML index](https://web.dev/html?hl=zh-tw).

### 13.4 The preview fragment needs a stricter content boundary

`ux-plan.md:314-317` currently says the fragment reuses the rendering pipeline
with the “same sanitization.” That phrase is not accurate enough for the current
implementation: the full reading route deliberately renders Markdown with raw
HTML enabled, including `<script>` (`internal/note/handler.go:165-175`). This is
an accepted property of opening a first-party local note, but it must not be
silently inherited by content fetched and inserted merely because the pointer
paused over a link.

Before `PR-ux-c` is implementable, its contract should state:

- the endpoint returns a server-owned preview view, not arbitrary beginning
  HTML copied from the full note page;
- title and status are explicit fields; body content is a bounded, read-only
  projection through a preview-safe renderer or allowlist;
- raw HTML, event-handler attributes, script/style, form controls, iframe,
  object/embed, audio/video and media loads are absent;
- remote images are absent, so hovering cannot cause a third-party request or
  reveal reading behavior;
- nested links are plain preview text or otherwise non-previewable, and never
  create a second floating surface;
- content type, `nosniff`, CSP compatibility, path validation, size/block
  limits, ETag behavior, 404/5xx response shape and cancellation are locked by
  handler and browser tests.

This is not a claim that the unimplemented route is vulnerable. It is a design
gate preventing the full-page trust boundary from being copied into a much
more ambient interaction.

### 13.5 Platform decisions for this repository

| Platform capability | Disposition | Product-specific reasoning |
|---|---|---|
| Semantic links, headings, named landmarks, native forms | **USE NOW** | These reduce custom keyboard/ARIA code and directly reinforce UX-05, UX-17, UX-27, UX-28 and UX-31. The supplied HTML guidance treats semantic structure as the accessibility baseline, not polish. |
| Popover API | **USE for the hover layer** | Correct nonmodal top-layer primitive; browser-owned light dismiss and Esc. It does not supply semantics, so the preview relationship still needs an explicit accessibility decision. |
| CSS anchor positioning | **USE as progressive enhancement** | Directly replaces geometry JS for the preview and supports flip fallbacks. Gate the feature; unsupported browsers keep ordinary links. Do not add the third-party polyfill to this local zero-dependency product. |
| `@starting-style` and `transition-behavior: allow-discrete` | **KEEP, do not embellish** | The project already uses both. They may give the preview a restrained opacity entry, but reduced-motion and instant dismissal remain authoritative. They are not a reason to animate navigation or card geometry. |
| `scrollIntoView({container: "nearest"})` and awaitable programmatic scroll | **EVALUATE for UX-14/UX-16** | This directly addresses keeping the current sidebar item visible without scrolling the whole page and may replace the TOC's fixed 900 ms settle guess after browser verification. It is more relevant than adding scroll effects. |
| `light-dark()` and preference media queries | **BOUNDED USE** | Useful only inside the ruled reading-display preference model. Do not create a second theme owner beside SSR `data-theme`/cookie state. Relative units and `prefers-contrast` deserve browser acceptance; `contrast-color()` must not replace curated accessible token pairs. |
| `scroll-target-group: auto` | **WATCH, not core** | It could eventually replace some scroll-spy ownership and supplies `aria-current`, but the supplied support table is Chromium-only. Running it beside the current observer would create two state owners. |
| Gap decorations | **WATCH / decorative only** | They can remove separator DOM/border workarounds, but Chrome/Edge 149-only support makes them unsuitable for required rail and Home boundaries. Adopt only during a later layout rewrite where missing decoration loses no information. |
| CSS `@scope`, style queries, custom functions and `if()` | **DEFER** | `@scope` may help only when the stylesheet is split; the others do not solve a current user problem and would make the token system harder to trace. |
| `<meta name="text-scale">` | **REJECT for now** | Chrome-only in the supplied table. Preserve rem-based sizing, browser zoom and the bounded user display controls; do not make the root sizing model engine-specific. |
| Element-scoped/two-stage View Transitions, scroll-triggered animation, hidey toolbars | **REJECT for current problems** | The observed navigation defect is double exposure and context loss. More transition stages and scroll-triggered motion add failure states; a disappearing header also weakens wayfinding in a reading tool. |
| `text-fit`, `text-box`, decorative shapes, JS pseudo-element access | **NO CURRENT CONSUMER** | These are visual mechanisms, not answers to the documented comprehension, accessibility, identity or continuity failures. |
| HTML-in-Canvas | **NOT APPLICABLE / REJECT** | It is an early Chrome 148-150 origin trial for canvas/WebGL/WebGPU workspaces. The product is already a semantic DOM reader; moving reading or graph navigation into canvas would add main-thread drawing/scroll constraints and reduce portability with no current need. |

Official sources used for the dispositions:

- the [Chrome CSS and UI collection](https://developer.chrome.com/docs/css-ui)
  and [New in Web UI at I/O 2026](https://developer.chrome.com/blog/new-in-web-ui-io26?hl=zh_tw)
  for theme functions, View Transitions, scroll targeting, anchor queries and
  the explicitly limited features;
- [Gap decorations: available in Chromium](https://developer.chrome.com/blog/gap-decorations-stable)
  for the Chrome/Edge 149 boundary and decorative progressive-enhancement
  rule;
- [Introducing the HTML-in-Canvas origin trial](https://developer.chrome.com/blog/html-in-canvas-origin-trial?hl=zh_tw)
  for its origin-trial status, intended canvas use cases and main-thread
  scrolling limitation;
- [web.dev HTML](https://web.dev/html?hl=zh-tw),
  [CSS](https://web.dev/css?hl=zh-tw), and
  [JavaScript](https://web.dev/javascript?hl=zh-tw) for semantic HTML,
  Baseline signals, user typography preferences and INP/long-task discipline.

### 13.6 Repository organization consequence

The platform review also trips two thresholds already recorded by the project:

- `assets/js/yomihon.js` is 797 lines; `ux-plan.md:614-616` says to evaluate
  plain ES modules when it nears 800. The hover layer would cross that threshold.
  Put its intent/fetch/overlay owner in a plain module (no bundler) instead of
  extending the monolith.
- `assets/css/components.css` is now 989 lines; `ux-plan.md:610-613` names the
  hover layer as the stylesheet split trigger. Split by the already-ruled
  surfaces before adding anchor/popover rules; do not change naming strategy.

These are not arguments for a framework. They are the point at which the
project's own small-file organization rule has become active.

### 13.7 Post-research conclusion and recommendation to the planner

Post-research conclusion: the original native-web direction is sound and the
preview is genuinely planned, but §11 should not be implemented unchanged. It
must first resolve concept-preview eligibility and replace “same sanitization”
with a restricted fragment trust boundary. The recent platform additions mostly
validate the chosen Popover + CSS-anchor architecture; they do not justify a
canvas surface, more navigation animation, a positioning polyfill, or a UI
framework.

Planner recommendation for `PR-ux-c`, in order:

1. record the concept-link ruling and preview-safe content contract;
2. split the CSS and JS at their already-recorded triggers;
3. add the bounded read-only fragment route and route-wall tests;
4. add one shared focus/hover Popover with anchor-position enhancement and
   ordinary-link fallback;
5. lock keyboard focus, coarse pointer, rapid pointer travel, stale/aborted
   fetches, 404/500, long titles/blocks, viewport edges, nested scrollers,
   reduced motion, overlay arbitration, pagehide/View Transition, raw HTML and
   remote-media exclusion in browser acceptance.

## 14. Corrected feature-by-feature Web Platform audit

This section supersedes the grouped judgments in §13.5 where they conflict.
The earlier table was an applicability triage, not the requested proof that
each mechanism had been understood before it was accepted or rejected. In
particular, `@scope`, style queries, CSS custom functions and CSS `if()` solve
different problems; grouping them under “defer” was not defensible.

### 14.1 Evidence and rejection standard

Every disposition below answers five separate questions:

1. What does the feature actually own?
2. Is it Baseline 2026, a safe enhancement, or a browser-specific experiment?
3. Which current source location would consume it?
4. What is the strongest case against the proposed use, rather than against
   the feature in general?
5. What concrete product or support change would overturn a “not now” answer?

`NOT NOW` therefore never means “bad API.” It means that the current product
has no honest consumer, the fallback would create two state owners, or the
user-visible benefit does not yet pay for the compatibility and testing cost.
An answer without an overturn condition is not a falsifiable design decision.

The current source and a live browser probe establish the local baseline:

- the theme is server-rendered as `data-theme`, has explicit light/dark token
  pairs and already sets `color-scheme` (`tokens.css:9-14,17-124`);
- the header height is repeated as `56px`, while the three shell scrollers use
  `calc(100vh - 56px)` (`components.css:113-184`);
- the concept sheet already uses `100dvh` and `min()`, but its narrow width is
  `100vw` (`components.css:627-642`);
- the page already uses `interpolate-size`, `transition-behavior`,
  `@starting-style`, a scroll timeline and cross-document View Transitions
  (`components.css:198-211,865-935`);
- TOC ownership is still `IntersectionObserver` plus `scrollend` and a fixed
  900 ms fallback (`yomihon.js:307-363`);
- the live browser exposed `STOREHOUSE`, `181`, no favicon link, and
  `form[role=search]` rather than `<search>`. Its capability probe accepted
  `light-dark()`, `contrast-color()`, CSS `if()`, `calc-size()`, anchor
  positioning, `scroll-target-group`, `row-rule`, `text-box` and `shape()`,
  while rejecting `text-fit`, `moveBefore()`, element-scoped View Transitions
  and JS pseudo-element access. A single engine probe is evidence about that
  engine, not a substitute for Baseline status.

The live Baseline index also changes two earlier assumptions:
container style queries and `contrast-color()` are in Baseline 2026, and the
`<search>` element is in Baseline 2023. They cannot be rejected merely as
“Chrome experiments.” Sources: [Baseline 2026](https://web.dev/baseline/2026),
[Baseline overview](https://web.dev/baseline), and
[New in Web UI at I/O 2026](https://developer.chrome.com/blog/new-in-web-ui-io26?hl=zh_tw).

### 14.2 CSS math and CSS author-defined logic are not interchangeable

| Mechanism | Actual job | Judgment in yomihon | Counterexample and overturn condition |
|---|---|---|---|
| `calc()` | Computes one value, including arithmetic across compatible units and custom properties. It does not branch and it does not make reusable functions. | **KEEP and improve the inputs.** `calc(100vh - 56px)` is semantically valid mixed-unit arithmetic; the defect is the repeated knowledge. Introduce one `--header-block-size` token and use `100dvh` for the shell scrollers after mobile/Safari acceptance. | Replacing it with JS layout would be worse. Replacing it with Grid is justified only if the shell can become a two-row grid without breaking three independent scrollers and sticky rails. |
| `min()` / `max()` | Select the minimum or maximum of computed candidates. | **KEEP.** `min(600px, 92vw)` and `min(460px, 92vw)` express a real upper bound plus viewport fit more directly than media-query duplication. | Use container units instead only when the surface is embedded in a component whose available width differs materially from the viewport. |
| `clamp()` | Bounds a preferred fluid value between a minimum and maximum. | **USE selectively, not as decoration.** It is a strong candidate for a ruled reading font/spacing preference expressed in `rem` plus a fluid term. It is not needed for fixed 32 px controls or the 56 px shell contract. | Do not use viewport-only type such as `clamp(..., 3vw, ...)`; it weakens zoom. Reopen per token only when the minimum, preferred response and maximum are each product requirements. |
| `interpolate-size` | Allows interpolation between a numeric size and an intrinsic keyword such as `auto`. | **KEEP current scoped use.** The rail disclosure is exactly the simple `0` to `auto` case. | Remove the motion, rather than reach for a more powerful function, if repeated disclosure animation makes navigation feel slower. |
| `calc-size()` | Performs math on exactly one intrinsic-size basis while preserving that basis's layout meaning. It is not a modern spelling of `calc()`. | **DO NOT SUBSTITUTE.** Current disclosures do not modify the intrinsic result; `interpolate-size` is simpler and the official guidance explicitly prefers it for that case. | Reopen if a real component must animate to `max-content + padding`, round an intrinsic size, or interpolate between calculations with the same intrinsic basis. Include the plain intrinsic declaration as fallback. |
| custom properties + `var()` | Name and cascade values; they are the existing HTML/CSS state and token contract. | **KEEP; add semantic tokens rather than raw repetitions.** `--header-block-size` is justified because `56px` is one piece of product knowledge. Do not tokenize every coincident `8px`. | Add `@property` only when typing, non-inheritance or animation of a particular custom property prevents a demonstrated invalid-value or interpolation defect. |
| CSS `@function` | Defines a reusable author function with scoped arguments and a returned value. It removes repeated value algorithms, not repeated constants. | **NOT NOW; Limited availability.** The stylesheet has repeated token values but no three-call value algorithm. A function would hide the provenance of carefully curated accessible color pairs without reducing a real formula. | Reopen when at least three consumers share the same derived algorithm—for example, an approved arbitrary accent produces hover, tint and on-color variants—and the fallback or Baseline target is explicit. |
| CSS `if()` | Selects a value from `style()`, `media()` or `supports()` conditions on the property being declared; unlike a style container query it can test the element itself. | **NOT NOW as core; Limited availability.** Existing root attributes make theme/ruby state inspectable, work at first paint and have simple selectors. Moving them inside declarations would produce a second styling dialect without removing state. | Reopen when one property has a measured combinatorial branch explosion and the preceding declaration is a complete fallback. A coarse-pointer control-size rule is a legitimate future example; replacing all media queries is not. |

The arithmetic definitions above follow
[web.dev Learn CSS: Functions](https://web.dev/learn/css/functions),
[Custom properties](https://web.dev/learn/css/custom-properties), and the
[official intrinsic-size guidance](https://developer.chrome.com/docs/css-ui/animate-to-height-auto).
The decisive `calc-size()` distinction is that it accepts one intrinsic basis
and preserves its layout semantics; `calc(max-content - min-content)` is not a
valid substitute.

### 14.3 I/O 2026 features 1–13: color, state, motion and surfaces

| # / feature | What it really does | Repository application and disposition | Strongest counterexample / reopen condition |
|---|---|---|---|
| 1 `contrast-color()` | Returns black or white against a supplied color to meet the algorithm's contrast choice; it is not a branded palette generator. Baseline 2026. | **DO NOT replace `--on-accent`.** Current light/dark pairs are curated and contrast-noted. Candidate only for a preview of a future user-supplied arbitrary color. | If arbitrary accents are approved, test both returned text color and non-text component contrast; a passing text pair does not validate borders, focus or disabled states. |
| 2 `light-dark()` | Resolves one of two colors from the element's effective `color-scheme`. Baseline Newly available since 2024. | **PILOT with the ruled system-theme mode, not before it.** It could consolidate exact token twins while `data-theme` remains the single owner by setting `color-scheme: light|dark` on that state. | Do not wholesale-migrate until inherited custom-property resolution is tested across nested schemes. Reopen with D48's system/light/dark state and visual/contrast parity for every semantic pair. |
| 3 `@function` | Author-defined value algorithm. | **NOT NOW**, for the reasons in §14.2. | The positive case is a repeated semantic derivation, not merely a long stylesheet. |
| 4 style queries | Query computed custom-property values on a containing ancestor, allowing a reusable component to react to its host's state. Baseline 2026. | **VALID API, no current consumer.** The slot machine directly consumes inherited `--c`; theme/ruby are root product state and clearer as attributes. | Reopen when an independently themeable nested component must react to host-owned computed state without adding product state to its markup. Do not use it just to hide a class. |
| 5 CSS `if()` | Inline conditional value selection. | **NOT NOW as core**, for §14.2. | Reopen for one bounded declaration with a complete prior fallback and less branching than equivalent queries. |
| 6 `@supports at-rule()` | Tests whether an at-rule is recognized. It does not prove support for that rule's prelude, descriptors or block semantics. | **NO blanket guards.** Unknown `@view-transition` already fails by being ignored. A guard would add text without changing fallback behavior. | Use it when companion declarations become harmful without the at-rule, and add a behavior probe because recognition alone is weak evidence. |
| 7 `<meta name="text-scale">` | Lets browser/OS text-scale preference affect the root size in supporting Chromium. It must not fight an author-set root font size. | **ACCESSIBILITY PILOT.** The project uses `rem` tokens and does not set an `html` pixel font size, so this is a plausible no-JS enhancement. It is not a replacement for browser zoom or the reading-display controls. | Ship only after Chrome and Safari/unsupported behavior, 200% text, header overflow, ruby, dialog and fixed-bottom controls are tested. Remove if it creates a browser-specific second scale owner. |
| 8 `linear()` easing | Defines a piecewise linear easing curve and can approximate spring/bounce shapes. | **AVAILABLE, not the navigation fix.** The current route complaint concerns snapshot double exposure and context loss, not the cubic-bezier curve. | Reopen for one microinteraction only after instant navigation is accepted and reduced motion is preserved. Do not add bounce to a calm reading surface. |
| 9 `@starting-style` | Supplies the before-open style for an element that did not previously render. | **KEEP.** Current toast, dialog and sheet entry are appropriate restrained consumers. | Entry support does not create an exit transition. Add exit behavior only if an abrupt close tests worse than the extra delay. |
| 10 `transition-behavior: allow-discrete` | Permits discrete properties such as `display`, `content-visibility` or overlay participation to switch at the useful end of a transition. | **KEEP on disclosures; evaluate for dialog exit only.** It cannot repair cross-document navigation. | A dialog close should remain immediate if users interpret a delayed disappearing modal as lag. Reopen only with an explicit exit-state contract and Escape/backdrop tests. |
| 11 `sibling-index()` / `sibling-count()` | Return the element's sibling position/count for computed values, commonly stagger timing. | **NOT NOW.** Sequentially delaying list/rail items increases time-to-stability and makes long lists feel slower. | Reopen for a visualization where order itself carries meaning; never stagger navigation, search results or reading content. |
| 12 `closedby="any"` | Declaratively opts a dialog into light dismiss in engines that implement it. | **KEEP current progressive use.** Search and concept dialogs already carry it; Escape and the concept-sheet fallback preserve core behavior. | Remove the JS backdrop fallback only when the Baseline target and live Safari acceptance prove parity. |
| 13 `corner-shape` | Changes the geometry of rounded corners while matching border, shadow and focus outline. | **WATCH for identity accents, never core controls yet.** It could express a seal/bookmark shape without extra DOM. | Reopen after the mark's geometry is designed and focus/forced-colors behavior is proven. It does not solve favicon delivery and must not become the only brand asset. |

### 14.4 I/O 2026 features 14–23: transitions and scrolling

| # / feature | What it really does | Repository application and disposition | Strongest counterexample / reopen condition |
|---|---|---|---|
| 14 same-document View Transitions | Animates a DOM state update in one document. | **NOT for routing:** yomihon is an MPA. Avoid on 180 ms live-search updates; animation would make typing feel behind. | Reopen for one infrequent, spatially meaningful same-page state change whose no-transition update remains correct. |
| 15 cross-document View Transitions | Animates between MPA documents when both opt in and named groups correspond. | **CURRENTLY SUSPECT.** The project uses it on the whole main region and title. The user's double-exposure/jank report is stronger product evidence than the API's novelty; first acceptance candidate is instant navigation or a much smaller named surface. | Keep only if repeated Chrome/Safari A/B trials show better orientation with no ghosting, delayed input, history jump or reduced-motion leak. |
| 16 element-scoped View Transitions | Limits transition capture to a DOM subtree and allows other page interaction; currently browser-limited. | **NOT NOW.** It could animate a result region, but live search values immediacy over choreography. | Reopen for a complex, infrequent local transformation where document-wide capture demonstrably blocks unrelated interaction. |
| 17 two-phase View Transitions | Splits a transition into an intermediate/skeleton phase and a final content phase. | **REJECT for local notes.** It adds a third visual state to a local-server navigation that should normally be immediate. | Reopen only if measured route latency needs a real intermediate state; do not manufacture skeleton latency for polish. |
| 18 scroll-driven animations | Ties animation progress to a scroll/view timeline. | **KEEP the guarded 1 px reading-position hairline.** It conveys position and fails to no line. | Remove if the line is mistaken for completion or costs paint on long content. Do not expand it into parallax/content motion. |
| 19 scroll-triggered animations | Starts/controls animations when scroll thresholds/ranges are crossed. | **NOT a TOC replacement.** It owns animation timing, not the semantic `aria-current` state and settle arbitration. | Reopen for a purely decorative, one-time reveal that remains visible without support; never hide core reading content pending a trigger. |
| 20 `scroll-target-group: auto` | Lets the UA choose the active fragment link, set `aria-current` and expose `:target-current`. | **WATCH / prototype as an exclusive owner.** It could remove part of the observer, but current JS also coordinates smooth-scroll lock and arrival echo. Running both creates conflicting truth. | Reopen when Chrome/Safari target coverage is acceptable and a prototype proves history, duplicate rail/inline TOCs, nested `.y-main`, manual scroll and arrival timing. Then delete, do not shadow, the corresponding JS ownership. |
| 21 `scrollIntoView({container:"nearest"})` | Restricts scrolling to the nearest scroll container instead of propagating through every ancestor. | **HIGH-VALUE PILOT for UX-14.** It directly fits “reveal the current sidebar row without moving the reading page.” | Unknown dictionary members can fall back to the old all-container behavior, so a naive call can move the page. Reopen implementation only with a reliable target-browser gate and a browser lock proving `.y-main` and document scroll offsets do not change. |
| 22 awaitable programmatic scroll | New implementations return a Promise from programmatic scroll methods, allowing arrival work after smooth scrolling completes. | **RESEARCH PROBE, not an assumption.** If supported in both targets it could replace the fixed 900 ms TOC guess. The current in-app engine did not expose `scrollend`, and this review did not mutate scroll solely to infer a return type. | Adopt only after a live Chrome/Safari probe proves the returned value and interruption semantics. Fallback remains `scrollend` plus a timeout; avoid UA sniffing. |
| 23 `scroll-state(scrolled: …)` queries | Let styles react to whether a scroll container is scrolled in a direction. | **DECORATIVE PILOT only:** show a rail edge shadow/fade when content exists beyond it. **Do not build a hidey header**; stable wayfinding is more valuable. | Reopen broader use when the state communicates hidden overflow without hiding controls, and the no-support appearance remains complete. |

### 14.5 I/O 2026 features 24–35: anchors, shapes, DOM and typography

| # / feature | What it really does | Repository application and disposition | Strongest counterexample / reopen condition |
|---|---|---|---|
| 24 anchored container queries | Let an anchored element query which fallback anchor placement won, so details such as an arrow can match the actual side. | **LATER preview polish.** It is relevant only after the shared Popover + anchor-position preview works. | The preview must remain understandable without an arrow. Reopen after edge/corner placement is accepted; never make arrow direction the only relationship cue. |
| 25 `border-shape` | Draws non-rectangular border/outline/shadow geometry. | **WATCH with `corner-shape` for brand accents.** No current information architecture problem requires it. | Reopen from an approved logo/component geometry, not from a desire to demonstrate the API. Forced-colors and focus shape must remain visible. |
| 26 `shape()` | Authors complex responsive paths inline for clipping, borders or float shapes. | **POSSIBLE code-native decorative mark**, but not a favicon solution. | An external SVG is still required for tab/favicon/social surfaces. Reopen for one responsive in-page ornament whose unclipped fallback is acceptable. |
| 27 per-axis sticky positioning | Allows sticky behavior to resolve against different scroll containers by axis under the new overflow model; the article still describes experimental enablement. | **NOT NOW.** It does not solve the reported fixed-width rail or flex-shrink problem. | Reopen for a real two-axis data surface with independently scrolling rows/columns and stable cross-browser support. |
| 28 overscroll gestures / swipeable areas | An Open UI proposal for declarative swipe/overscroll actions, not a stable shipped primitive. | **NO product dependency.** A mobile drawer swipe-close would be tempting but the proposal cannot carry core dismissal. | Reopen after standardization and Baseline; Escape, close button, backdrop and ordinary navigation stay primary. |
| 29 HTML-in-Canvas | Places DOM-backed HTML into a canvas/WebGL/WebGPU scene while retaining selection, find and accessibility integrations. The cited release is a Chrome 148–150 origin trial and its inner scroll/animation work remains main-thread dependent. | **NOT APPLICABLE to the reader.** Moving prose or navigation into canvas buys no capability and violates the Baseline rule. | Reopen only for a measured future graph workspace with thousands of spatial nodes that DOM/CSS cannot meet, with an equivalent semantic navigation path and no origin-trial dependency. |
| 30 `moveBefore()` | Reparents an existing DOM node while preserving state that `append`/`insertBefore` may reset, such as focus, video, iframe or animations. | **WATCH.** Current responsive design moves surfaces with CSS and does not reparent them; the live engine probe lacked the method. | Reopen when the same stateful component truly moves between rail and main at a breakpoint and duplication/CSS placement cannot work. |
| 31 `text-fit` | Scales text to fit a line/box. | **DO NOT use for reading, counts or controls.** It creates unpredictable hierarchy and zoom outcomes; the live probe lacked it. | Reopen only for a decorative single-line logo lockup with explicit min/max size, localization and 200% zoom evidence. |
| 32 `text-box` | Trims font metric space at selected text-box edges for optical alignment. | **BOUNDED PILOT for a single brand/glyph lockup or icon-button label.** The live engine accepted its component properties. | Never apply it to long-form CJK, ruby or multiline prose; trimming can collide with annotations and line rhythm. Reopen per isolated element with font-fallback screenshots. |
| 33 gap decorations | Draws row/column rules across Grid/Flex gaps without separator DOM and without affecting layout. Chromium 149 is the cited stable boundary. | **WATCH.** `.y-railsep` is extra decorative markup, but removing it would erase boundaries in unsupported Safari. Keeping both removes the simplification benefit. | Reopen when Safari/Baseline coverage arrives, or for a new grid whose missing rule is harmless and whose fallback needs no extra DOM. |
| 34 scrollbar-aware viewport units | Subtract a stable root scrollbar from viewport units when the root establishes the relevant stable gutter/scrollbar condition. | **NO direct fix for this shell.** The app scrolls `.y-main` and rails, not the root; its `scrollbar-gutter` is on `.y-main`. | For the mobile concept sheet, prefer logical `inline-size: 100%` over `100vw`. Reopen viewport-unit reliance only if root scrolling becomes the layout model. |
| 35 JS pseudo-element access | Exposes pseudo-elements and an event `pseudoTarget` to JavaScript. | **REJECT for core interaction.** The project correctly uses real links/buttons; turning generated content into an interaction target would lose native semantics. The live engine lacked the API. | Reopen only for diagnostics or animation tooling that never carries action, text alternative or focus. |

The feature semantics and browser boundaries above come from the complete
numbered list in
[New in Web UI at I/O 2026](https://developer.chrome.com/blog/new-in-web-ui-io26?hl=zh_tw),
the [Chrome CSS/UI collection](https://developer.chrome.com/docs/css-ui),
[Gap decorations](https://developer.chrome.com/blog/gap-decorations-stable?hl=zh_tw),
and the
[HTML-in-Canvas origin trial](https://developer.chrome.com/blog/html-in-canvas-origin-trial?hl=zh_tw).
Experimental and proposal status is a reason not to create a dependency; it is
not a claim that the underlying use case is invalid.

### 14.6 HTML, CSS and JavaScript changes this research actually supports

The supplied Learn indexes are not “use every new feature” lists. Applied to
this repository, they produce the following concrete corrections.

**HTML**

- Wrap the home and full search forms in `<search>` rather than retaining
  `role="search"` on the form. `<search>` is the landmark container; the real
  GET `<form>` remains inside it. This is a semantic migration, not a custom
  search widget.
- Give the command-palette `<dialog>` an accessible name from a visible heading
  or `aria-labelledby`. The search field's label names the field, not the
  dialog. Translate the concept sheet's `aria-label="Grammar note"` and all
  remaining browser chrome under the existing Traditional Chinese ruling.
- Build the link preview with one real `<a href>` plus one shared Popover. Do
  not replace links with buttons and do not trap focus in a preview.
- Use `inert` only when an actually open mobile drawer must make the obscured
  page unavailable. Do not leave content inert after resize, history or error;
  browser acceptance must prove restoration.
- Add a real favicon/SVG mark and explicit icon links. CSS shapes can echo that
  mark in the page, but they cannot supply the browser-tab asset.

**CSS**

- Add a semantic `--header-block-size` and replace the three repeated
  `56px`/`calc(100vh - 56px)` relationships. Prefer logical block/inline
  properties and `100dvh` after target-browser acceptance.
- Preserve explicit semantic color tokens. `light-dark()` is a consolidation
  candidate when the three-state theme is ruled; `contrast-color()` is not a
  substitute for curated `--on-*` pairs.
- `@scope` is now cross-engine enough to be considered, but its purpose is
  selector reach and scoping proximity—not file splitting. The existing
  `.yomihon` prefix already bounds product rules. `@scope` also does not stop
  inherited properties crossing a donut limit. Pilot it only for a surface
  whose descendant selectors are presently high-specificity or DOM-coupled;
  do not rewrite 989 lines for novelty.
- Size container queries are justified when a reusable surface responds to its
  allocated width. Style queries are justified when it responds to host-owned
  computed state. Neither should replace clear viewport shell breakpoints or
  root product-state attributes indiscriminately.
- Do not add `content-visibility:auto` across the reading article without a
  benchmark. TOC geometry, fragment arrival and long CJK/ruby content are more
  important than skipping paint in a local page that has not shown rendering
  cost. A large offscreen dashboard section with `contain-intrinsic-size` would
  be a more honest future consumer.

**JavaScript**

- The present search enhancement already uses the correct modern primitives:
  a real GET form, `AbortController`, a monotonic request identity, IME-aware
  input and DOM fragment import. Keep that race discipline for previews.
- Do not introduce `scheduler.yield()`, workers or task chunking without a
  measured long task. The route-transition complaint is currently CSS snapshot
  behavior, not evidence that 797 lines of source execute as one long task.
- Prototype `scrollIntoView({container:"nearest"})` for rail visibility and
  awaitable scrolling for TOC settlement in committed browser probes before
  editing production ownership.
- Split the 797-line enhancement file into plain ES modules when the hover
  preview lands, as the existing trigger already says. Modules are native and
  strict by default; the split is ownership, not a client framework.
- Any fetched preview fragment needs the restricted trust boundary in §13.4.
  A new API does not make ambient insertion of raw note HTML safe.

Sources for these conclusions:
[Learn semantic HTML](https://web.dev/learn/html/semantic-html),
[web.dev HTML](https://web.dev/html?hl=zh-tw),
[web.dev CSS](https://web.dev/css?hl=zh-tw),
[web.dev JavaScript](https://web.dev/javascript?hl=zh-tw),
[CSS `@scope`](https://developer.chrome.com/docs/css-ui/at-scope),
[style queries](https://developer.chrome.com/docs/css-ui/style-queries), and
[color themes with Baseline CSS](https://web.dev/articles/baseline-in-action-color-theme).

### 14.7 Changed conclusions

Research changed the earlier conclusions in five material ways:

1. `@scope` is not deferred because the file has not split; file organization
   and selector scope are unrelated. It is available but lacks a current
   selector-specific payoff.
2. Style queries and `contrast-color()` cannot be dismissed as Chromium-only;
   both are Baseline 2026. Their current rejection is product-semantic, not
   compatibility-based.
3. `<meta name="text-scale">` moves from rejection to an accessibility pilot
   because the current rem-based type system is structurally compatible.
4. `scrollIntoView({container:"nearest"})` is a high-value targeted prototype,
   while awaitable scroll remains an evidence gate rather than a claim copied
   from an announcement.
5. `light-dark()` is a plausible future simplification under the same
   `data-theme` owner; `@function` and CSS `if()` remain deferred because there
   is no repeated algorithm or conditional-value problem yet, not because the
   APIs were unread or categorically undesirable.

The resulting rule is stricter than “native first”: use the least powerful
native primitive that owns the real requirement, prove its fallback, and name
the evidence that would justify moving one rung higher.
