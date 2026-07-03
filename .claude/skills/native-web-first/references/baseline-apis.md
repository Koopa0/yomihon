# Baseline CSS + JS APIs

The platform features that replace whole categories of JS. "Baseline" = the
[web-features](https://github.com/web-platform-dx/web-features) status shared
across the core browsers (Chrome, Edge, Firefox, Safari): _Newly available_
when all four ship it, _Widely available_ ~30 months later.

## Where to check — pick the source by the question (read this first)

| The question you're actually asking                       | Go here                                                                                                    |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| 1. **Learn the modern way to do X** (DX, patterns)        | Chrome for Developers (`developer.chrome.com`) · `web.dev`                                                 |
| 2. **Can I use this API yet?** (cross-browser / Baseline) | **`web.dev/baseline`** (zh-tw: `web.dev/baseline?hl=zh-tw`) + **MDN** compatibility table / Baseline badge |
| 3. **Does Chrome support it, and since which version?**   | Chrome Platform Status (`chromestatus.com`)                                                                |
| 4. **What does the standard actually say?**               | WHATWG HTML (`html.spec.whatwg.org`) · W3C specs                                                           |
| 5. **How does it behave across browsers, tested?**        | Web Platform Tests — `wpt.fyi`                                                                             |
| 6. **How is it implemented inside Chromium?**             | Chromium Docs · `source.chromium.org`                                                                      |

The trap this table prevents: rows 1, 3, and 6 are **Chrome-first**. A feature
being on `developer.chrome.com` or `chromestatus.com` tells you _Chrome_ ships
it — **not** that it is Baseline. The "can I ship this?" decision is **row 2
only** (`web.dev/baseline` + MDN); otherwise you can reach for a Chrome-only or
origin-trial API, which the charter forbids.

The operator target here is _latest Chrome/Safari on one machine_, so you may
use Baseline features directly without polyfills — but "Baseline" is still the
bar that keeps a nicety (e.g. `text-wrap: pretty`) from becoming a hard
dependency. Verify current status on row 2 when unsure; do **not** trust this
file's status notes blindly — they were written at a point in time and the live
sources are canonical.

## CSS — the second rung of the ladder

### Attribute-driven state (theming, toggles) — no JS for the visual switch

Stamp state on the root as an attribute server-side; CSS reacts. JS only flips
the attribute + cookie.

```css
:root,
[data-theme="light"] {
   --bg: …;
   --fg: …;
}
[data-theme="dark"] {
   --bg: …;
   --fg: …;
}
[data-ruby="off"] rt:not([data-keep="1"]) {
   visibility: hidden;
} /* zero reflow */
[data-nav="open"] .rail {
   transform: none;
}
```

Custom properties (`--x`) cascade and re-resolve on attribute change with no
markup change — the engine repaints. Baseline for years.

### `:has()` — the "parent"/previous selector

Style a container based on its contents or sibling state, killing a lot of
class-toggling JS.

```css
.field:has(:user-invalid) .hint {
   color: var(--error);
}
.card:has(> img) {
   padding-top: 0;
}
```

Baseline 2023.

### Container queries — component-responsive without global breakpoints

```css
.panel {
   container-type: inline-size;
}
@container (min-width: 30rem) {
   .panel .row {
      grid-template-columns: 1fr 1fr;
   }
}
```

Baseline 2023. Prefer when a component must adapt to _its_ space, not the
viewport.

### `color-mix()` + `oklch()` — perceptual color & tints from one token

```css
background: color-mix(in oklch, var(--panel) 82%, transparent);
--accent-muted: oklch(0.55 0.18 33 / 0.12);
```

Both Baseline 2023. One accent token → hovers, tints, translucency without a
palette of hardcoded hex.

### `text-wrap: balance` / `pretty`

`balance` (Baseline 2024) evens short headings; `pretty` (Chrome/Safari; Firefox
lagging at time of writing) avoids orphans in prose. Both are _pure niceties_
that degrade to normal wrapping — safe to use, never depend on.

### `@media (prefers-reduced-motion: reduce)`

Mandatory. Gate every animation/transition; collapse durations to ~0 under
reduce. Baseline for years.

### CSS nesting

Baseline 2023 — usable, but the standalone Tailwind/PostCSS build here already
flattens; author flat if a rule must be greppable.

## JS — the third rung (Baseline APIs, used directly)

### `form.requestSubmit()` — the enhancement hinge

Submits a form **as if a submit button was pressed**: fires the `submit` event,
runs native validation, respects `formaction`. This is how you enhance a write
path without breaking no-JS.

```js
btn.addEventListener("pointerup", () => {
   /* after the hold gesture */ form.requestSubmit();
});
```

Contrast `form.submit()` (skips validation + the submit event) and `fetch`
(deletes the no-JS fallback — banned on write paths). Baseline (widely
available).

### `dialog.showModal()` / `.close()` — see native-elements.md

Top layer, backdrop, focus trap, Escape — all free. Baseline.

### Popover API — `togglePopover()` / `showPopover()`

For menus/tooltips. Baseline 2024.

### `history.replaceState()` — consume a one-shot URL signal

Strip a query param after acting on it, so a refresh doesn't replay:

```js
const u = new URL(location.href);
if (u.searchParams.has("sealed")) {
   // …play the one-time animation…
   u.searchParams.delete("sealed");
   history.replaceState(null, "", u); // refresh is now clean
}
```

`URL` + `URLSearchParams` + `history.replaceState`: Baseline for years.

### `IntersectionObserver` — visibility without scroll handlers

For lazy work or (later) TOC scrollspy. Baseline. **Not for v0 here** — anchor
links suffice; add only when the lack is felt.

### View Transitions

Same-document `document.startViewTransition(cb)` animates DOM changes; Baseline
_Newly_ (Chrome/Safari solid, Firefox catching up). Treat as pure enhancement —
wrap in a feature check and the un-transitioned change must be correct on its
own. Cross-document VT is newer still.

### Small sharp tools

`element.closest(sel)` / `matches(sel)` (event delegation without libraries),
`structuredClone`, `AbortController` (cancel listeners/fetches), `Element.
toggleAttribute`. All Baseline. These are usually all the "framework" a page
needs.

## The through-line

Every item above is something a library used to provide. Before importing one,
check whether the platform now ships it Baseline — it usually does. The library
is the fallback of last resort, and adding it is the human's decision, not the
implementer's.
