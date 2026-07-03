# Native elements catalog

Reach for these before any JS. Each entry: what it is, when to use, the
keyboard/focus behavior you get free, and the gotchas.

## `<details>` / `<summary>` — disclosure

Native collapse/expand. Zero JS: open state is the `open` attribute; the
browser toggles it on summary click/Enter/Space.

```html
<details open>
   <summary>Lifecycle</summary>
   …content…
</details>
```

- **Server-render the default state** by emitting (or omitting) `open`.
- **Style the marker away** and supply your own chevron:
   ```css
   summary {
      list-style: none;
      cursor: pointer;
   }
   summary::-webkit-details-marker {
      display: none;
   }
   .chevron {
      transition: transform 120ms;
   }
   details:not([open]) .chevron {
      transform: rotate(-90deg);
   } /* CSS, not JS */
   ```
- **Collapsible callout / accordion**: a `<details class="callout">` with a
  `<summary class="callout-title">` is a complete, accessible, no-JS accordion.
- Gotcha: `<summary>` must be the **first child**. Content after it is the body.
- Gotcha: an in-page anchor to content inside a closed `<details>` auto-opens it
  in modern browsers (`hidden=until-found`/details behavior) — good for TOC links.

## `<dialog>` — modal & non-modal

The native modal. `showModal()` gives you: top-layer stacking, a `::backdrop`,
focus trapped inside, `Escape` to close, and focus returned to the opener — all
free. Use it for command palettes (⌘K), confirmations, detail panels.

```html
<dialog id="search">
   …
   <form method="dialog">…</form>
</dialog>
```

```js
document.getElementById("search").showModal(); // modal (backdrop + focus trap + Esc)
// dialog.show()   — non-modal (no backdrop, no trap)
dialog.close(returnValue); // sets dialog.returnValue
```

- `::backdrop` is the scrim — style it, don't build a scrim div.
- A `<form method="dialog">` inside closes the dialog on submit and reports the
  submitter's value as `returnValue` — no JS.
- Focus: put `autofocus` on the field you want focused when it opens.
- Gotcha: `showModal()` on an already-open dialog throws — guard with
  `dialog.open`.
- Gotcha: clicking the backdrop does **not** close by default; add a one-liner
  if you want it (compare `event.target === dialog`, or the click coordinates
  vs `getBoundingClientRect`).

## `popover` attribute — lightweight overlays

Baseline 2024. For menus, tooltips, non-modal poppers that need light-dismiss
(click-away + Escape) but **not** a focus trap. Pure HTML, zero JS:

```html
<button popovertarget="menu">Open</button>
<div id="menu" popover>…</div>
```

- `popover` (auto) = light-dismiss + one-at-a-time. `popover="manual"` = you
  control dismissal.
- Top-layer, so it escapes `overflow:hidden`/stacking-context traps that plague
  hand-rolled dropdowns.
- JS when needed: `el.showPopover()`, `hidePopover()`, `togglePopover()`.
- Choose `<dialog>` when you need a focus trap (true modal); choose `popover`
  for menus/tooltips where trapping focus would be wrong.

## Forms + PRG — the write primitive

A `<form method="post">` posting to a handler that responds `303 See Other` →
GET (Post/Redirect/Get) is a complete, no-JS, refresh-safe mutation. This is
the backbone of every write.

```html
<form method="post" action="/status">
   <input type="hidden" name="path" value="…" />
   <input type="hidden" name="from" value="draft" />
   <input type="hidden" name="to" value="ready" />
   <button type="submit">Certify ready</button>
</form>
```

- **One form per action.** Multiple submit buttons can carry different
  `name`/`value` or `formaction` if you must share a form, but separate forms
  are clearer.
- **PRG** (server redirects after POST) makes reload/back safe — no
  double-submit, no "resubmit form?" dialog.
- Native validation: `required`, `type=email`, `pattern`, `min`/`max`,
  `:user-invalid` styling — before you write a validation function.
- To enhance without breaking no-JS: intercept `submit`, do the gesture, then
  `form.requestSubmit()` (see baseline-apis.md). Never swap to `fetch` on a
  write path — that deletes the fallback.

## Buttons, links, labels — get the semantics right

- `<button type="button">` for JS actions; `<button type="submit">` inside a
  form; `<a href>` for navigation. Never a clickable `<div>` — you lose
  keyboard, focus, and role, then rebuild them badly.
- `<label>` wrapping or `for=`-associated with its control gives a bigger hit
  target and screen-reader association for free.
- A control that toggles state carries `aria-pressed` / `aria-current` /
  `aria-expanded` so its state is announced; CSS can key off the same attribute.

## Semantic sectioning — structure for free

`<header> <nav> <main> <aside> <article> <section> <footer>` give landmarks
(screen-reader navigation) and stable styling hooks without a single class.
Use them for the shell (header + nav rail + main + aside rail) instead of
`<div class="header">`.

- `<nav>` for the sidebar and the TOC; `<main>` for the reading column;
  `<aside>` for the right rail. One `<main>` per page.
- Headings (`<h1>`–`<h6>`) in order build the document outline the TOC mirrors.
