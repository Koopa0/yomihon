---
name: native-web-first
description: >-
   HTML-first / CSS-first / Baseline-Web-API discipline for server-rendered
   templ + vanilla-JS UIs. The decision ladder (semantic HTML → CSS → Baseline
   API → JS-as-enhancement) before writing any interactive behavior, a catalog
   of native elements (details/summary, dialog, popover, forms+PRG) and Baseline
   CSS/JS APIs with when-and-how, the progressive-enhancement contract
   (server-render state as data-* attributes, mandatory no-JS fallback for write
   paths), and the "never build a framework" anti-patterns. Ends with the
   kurodo-specific pins (the four walls, D27 zero-JS write path, D28 English
   chrome, the design bundle as the single UI reference).
when_to_use: >-
   Use before building or modifying any UI behavior in a server-rendered templ
   project — deciding whether an interaction needs HTML, CSS, a Baseline Web
   API, or JS; adding a toggle, dialog, disclosure, drawer, form flow, or
   keyboard affordance; writing the single vanilla-JS enhancement file; or
   whenever a requirement seems to want htmx / Alpine / React / any client
   library (that is a STOP-and-surface, not an implementation choice). Read it
   before the first line of markup or script, not after.
user_invocable: true
metadata:
   author: koopa
   version: "1.0"
   surface: "templ + vanilla web"
---

# Native-Web-First — build with the platform, not on top of it

The browser already ships a router, a modal, a disclosure widget, form
validation, focus management, and state persistence. Most "frontend work" is
choosing the native primitive and dressing it — not re-implementing it in JS.
This skill is the discipline that keeps a UI made of semantic HTML + CSS +
small, honest enhancement, and out of the framework tar pit.

## The decision ladder — answer these IN ORDER before writing a behavior

For every interactive feature, write one short note answering each, top to
bottom. The first "yes" is your implementation; you only descend when the
answer is a real "no", not an "it'd be easier in JS".

1. **Can this be semantic HTML?** — a `<button>`, `<a>`, `<form>`,
   `<details>/<summary>`, `<dialog>`, `<label>`, `<input>`, `<select>`,
   `<nav>/<main>/<aside>`, the `popover` attribute. Native elements come with
   keyboard support, focus, ARIA roles, and form participation for free.
2. **Can this be CSS?** — disclosure chevron rotation, hover/active/focus
   states, responsive layout, theme switching by attribute selector, show/hide
   by state, reduced-motion, even some counters and scroll-driven effects.
   `:has()`, container queries, `color-mix()`, custom properties, and
   `@media (prefers-reduced-motion)` remove enormous amounts of would-be JS.
3. **Is there a Baseline Web API for it?** — `dialog.showModal()`,
   `form.requestSubmit()`, the Popover API, `HTMLElement.togglePopover()`,
   `structuredClone`, `URLSearchParams`, `history.replaceState`,
   `element.closest`/`matches`, `IntersectionObserver`, View Transitions. If
   the platform already solves it and it is Baseline, use it directly.
4. **What is the no-JS fallback?** — every enhancement must degrade. For a
   _write_ path (anything that mutates server state) a working no-JS fallback
   is **mandatory, not "where practical"**: the plain `<form method="post">`
   must do the whole job before a line of JS is written. JS may only make it
   nicer.
5. **If a library seems necessary — STOP.** Adding htmx, Alpine, React, a
   validation lib, a modal lib, a date picker — none of these is an
   implementation decision. Surface it to the human with the specific
   requirement that seems to need it. The default answer is "the platform can
   do this"; the burden of proof is on the library.

If you catch yourself writing a client-side router, a reactive store, a DOM
differ, hydration, or a modal/dropdown/validation framework — you have left
this skill. Stop and re-run the ladder.

## Progressive enhancement — the shape

```
server renders semantic HTML  →  works with zero JS
        │  (state carried as data-* attributes + cookies, rendered SSR)
        ▼
CSS reacts to those attributes (theme, ruby, drawer, disclosure)  →  no FOUC, no JS
        │
        ▼
one small vanilla JS file reads data-* and adds ceremony  →  enhancement only
```

- **State lives on the server, is rendered into the HTML.** Persisted UI
  state (theme, a furigana toggle, a collapsed section) is a cookie the server
  reads and stamps onto the root element as a `data-*` attribute or class, so
  the correct state paints on first byte (no flash-of-wrong-theme). JS toggles
  the attribute _and_ the cookie; CSS does the visual switch by attribute
  selector.
- **`data-*` attributes are the ONLY contract between HTML and JS.** JS never
  hardcodes structure; it finds elements by `data-*` hooks and reads/writes
  `data-*` state. Rename-safe, framework-free, inspectable.
- **The write path has zero JS dependency.** A form submits and the server
  redirects (POST → 303 → GET, the PRG pattern). JS may intercept to add a
  gesture (a hold-to-confirm, an optimistic hint) but must end by calling
  `form.requestSubmit()` — **never `fetch`** — so the server sees exactly the
  no-JS request. Turn JS off and the button still works.
- **One hand-written JS file, no build step, no Node.** Precedent: five real
  interactions in ~200 lines. If it is growing a module system, you are
  building a framework — stop.

## Accessibility is not optional

Every interactive feature keeps: keyboard operability (Tab/Enter/Space/Escape,
arrow keys where a native widget implies them), visible focus, a real label
(visible text or `aria-label` in the chrome's language), `prefers-reduced-
motion` honored, and the no-JS fallback. Native elements give most of this for
free — which is the point of reaching for them first.

## Browser target

A single-user local tool targets the operator's own latest Chrome/Safari. Use
Baseline features directly — `oklch`, `color-mix`, `:has`, `text-wrap`,
`dialog`, `popover` — and do **not** "fix" a modern design down to
old-browser compatibility. Feature-detect only genuinely exotic or
newly-shipped APIs, and never use a Chrome-only origin-trial API.

## Anti-patterns (the "never" list)

- A client-side **router**, **store**, **DOM differ**, **hydration** layer, or
  a **modal/dropdown/validation framework** — the platform or a native element
  already does each.
- Adding **any** client library as an implementation decision.
- `fetch` on a write path (breaks the no-JS fallback; use `requestSubmit`).
- Porting a mock's **inline styles** verbatim, or a mock's throwaway JS runtime
  — mocks are tool output; re-express behavior in the real classes + one JS file.
- "Fixing" `oklch`/`color-mix`/`text-wrap` down to legacy CSS for browsers the
  operator does not use.

## kurodo-specific pins (this project)

Read `docs/decisions.md` D26–D28 and `docs/design.md` §2 (the do-not-introduce
list) before UI work; the four walls (`CLAUDE.md`) and that list override
everything above.

- **UI reference = the design bundle only**: DS `ui-*` classes + `theme.css`
  tokens + the four `*.dc.html` mocks. goilerplate/templui is **not** a
  reference; templui must never enter `go.mod`. For templ patterns, follow the
  existing files under `internal/ui/`.
- **Write face (D27)**: one `<form method="post" action="/status">` per legal
  transition, and the legal set is **only** what `schema.Transitions(type,
current)` returns — never fabricate a key. `ready` gets the hold-to-seal
  ceremony (JS `requestSubmit` on completion); every other legal transition is
  a quiet one-click form; zero transitions → "No legal transitions". The
  post-seal animation fires from a one-shot PRG signal (`?sealed=1`, stripped
  by `history.replaceState`). The git short hash is a read-only `git log -1
--format=%h` in `internal/status` only.
- **Sidebar (D26)**: Lifecycle groups/labels/statuses come from the schema
  contract + snapshot counts — zero hardcoded status lists (wall 3). Reports =
  daily-briefing HTML; Folders = the existing tree in a collapsed `<details>`.
- **Text (D28)**: functional chrome is English (sentences, diagnostics, counts,
  `aria-label`s); CJK appears only as single-glyph seals (済/印/振) or the
  `CJK · English` paired pattern where English alone carries the meaning. Note
  content is vault material — untouched.
- **Reading column**: style the renderer's _existing_ emitted classes with CSS;
  never reshape renderer HTML — dialect fixtures lock it.
- **State mechanism**: one dark-mode signal and one ruby signal, consistent
  everywhere — kurodo uses root `data-theme` / `data-ruby` / `data-nav`
  attributes (the `data-*` HTML↔JS contract), cookie-persisted, SSR, no FOUC.

## Navigation

| Topic                                                                                                                                                                                                    | File                                    | When to read                                                                  |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------------------------------------------- |
| Native elements catalog — `<details>`, `<dialog>`, `popover`, forms + PRG, buttons/links/labels, semantic sectioning; each with when-to-use, keyboard/focus behavior, and gotchas                        | `references/native-elements.md`         | Before implementing any disclosure, modal, menu, or form flow                 |
| Baseline CSS + JS APIs — attribute-driven theming, `:has()`, container queries, `color-mix`/`oklch`, `text-wrap`, reduced-motion, `requestSubmit`, `showModal`, View Transitions, `history.replaceState` | `references/baseline-apis.md`           | When choosing the CSS or Baseline-API rung of the ladder                      |
| Progressive-enhancement recipes — SSR state as data-* + cookie (no FOUC), the hold-to-submit gesture over a plain form, one-shot PRG signals, the single-file JS structure                               | `references/progressive-enhancement.md` | When writing the vanilla JS enhancement layer or wiring server-rendered state |
