# Progressive-enhancement recipes

Working patterns for the "server renders truth → CSS reacts → one JS file adds
ceremony" shape. Each has a no-JS baseline that already works.

## 1. Persisted UI state: cookie → server-rendered attribute (no FOUC)

Theme, a furigana toggle, a collapsed section — anything that must survive a
reload and paint correctly on the first byte.

**Server** reads the cookie and stamps the root:
```go
theme := "light"
if c, err := r.Cookie("kurodo_theme"); err == nil && c.Value == "dark" {
    theme = "dark"
}
// templ: <html data-theme={ theme } data-ruby={ ruby }> … </html>
```
**CSS** does the visual switch by attribute selector (see baseline-apis.md).
**JS** flips the attribute *and* writes the cookie so the next load matches:
```js
function setToggle(name, value) {
  document.documentElement.dataset[name] = value;               // instant visual (CSS reacts)
  document.cookie = `kurodo_${name}=${value};path=/;max-age=31536000;samesite=lax`;
}
```
- No-JS baseline: without JS the page still renders in whatever state the cookie
  last held; the toggle button just doesn't do anything until JS loads. (If a
  no-JS toggle is required, make the button a tiny GET form that sets the cookie
  server-side and redirects — but for a personal tool the cookie+JS flip is
  enough.)
- Why the attribute, not a class: it composes with sibling state attributes
  (`data-ruby`, `data-nav`) as one uniform `data-*` contract, and CSS attribute
  selectors read it directly.

## 2. A gesture on top of a plain form (hold-to-confirm)

The write path is a plain `<form method="post">` (works with no JS). JS adds a
430 ms press-and-hold as a misclick guard, then submits the *same* form.

```html
<form method="post" action="/status" data-seal>
  <input type="hidden" name="path" value="…">
  <input type="hidden" name="from" value="draft">
  <input type="hidden" name="to"   value="ready">
  <button type="submit" data-seal-btn aria-label="hold to certify ready">Certify ready</button>
</form>
```
```js
const form = document.querySelector('[data-seal]');
if (form) {
  const btn = form.querySelector('[data-seal-btn]');
  let timer = null, firing = false;
  const start = (e) => {
    if (firing) return;
    e.preventDefault();                 // suppress the instant click; the hold is the gesture
    fill(btn, true);                    // CSS-driven wipe animation
    timer = setTimeout(() => { firing = true; form.requestSubmit(); }, 450);
  };
  const cancel = () => { clearTimeout(timer); if (!firing) fill(btn, false); };
  btn.addEventListener('pointerdown', start);
  btn.addEventListener('pointerup', cancel);
  btn.addEventListener('pointerleave', cancel);
  // keyboard parity: hold Enter/Space; and a global key (R) may start it
  btn.addEventListener('keydown', (e) => { if ((e.key === 'Enter' || e.key === ' ') && !e.repeat) start(e); });
  btn.addEventListener('keyup', cancel);
}
```
- **`requestSubmit()`, never `fetch`.** The server sees exactly the no-JS POST;
  PRG redirects; the page reloads. Turn JS off and the button is a normal
  one-press submit — still correct, just without the ritual.
- The fill/wipe is a CSS animation toggled by a class; JS only adds/removes it.
- Only `ready` gets the hold; every other transition is a bare one-click form.

## 3. One-shot post-action signal (animate once, survive refresh)

After a write, you want a one-time animation but not a replay on refresh. Carry
a query param on the PRG redirect, act on it, then strip it.
```go
// handler, on success: 303 to /notes/<path>?sealed=1
http.Redirect(w, r, "/notes/"+path+"?sealed=1", http.StatusSeeOther)
```
```js
const u = new URL(location.href);
if (u.searchParams.get('sealed') === '1') {
  document.querySelector('[data-seal-panel]')?.classList.add('just-sealed'); // CSS plays keyframes
  u.searchParams.delete('sealed');
  history.replaceState(null, '', u);           // refresh no longer replays it
}
```
- Alternative carrier: a short-lived flash cookie the server sets then clears on
  read — truly one-time even without JS, at the cost of server state. Pick one
  and write down which; here it's the query param + `replaceState`.
- No-JS baseline: without JS the note simply renders in its new (sealed) state,
  no animation — correct, just quiet.

## 4. Native `<dialog>` for a command palette (⌘K)

```js
const dlg = document.querySelector('[data-search]');
addEventListener('keydown', (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    dlg.open ? dlg.close() : dlg.showModal();   // backdrop + focus trap + Esc, all native
  }
});
```
- No-JS baseline: the header's search control is also a real link to `/search`
  (or a GET form), so search works without the palette. The palette is a faster
  path, not the only path.

## 5. The single JS file — structure & rules

```js
(() => {
  'use strict';
  // one IIFE, strict mode, no exports, no modules, no build step.
  // find hooks by data-* only; never by tag structure or a generated class.
  // each behavior is a small function guarded by "is its element present?".
  theme();      // toggle + cookie
  ruby();       // toggle + cookie
  drawer();     // nav open/close + scrim + Esc
  seal();       // hold-to-submit gesture (recipe 2) + one-shot signal (recipe 3)
  search();     // ⌘K dialog (recipe 4)
  mermaid();    // pre-existing: render diagram fences lazily
})();
```
- **Budget discipline**: five interactions in ~200 lines is the precedent. If it
  wants a module loader, a state object shared across behaviors, or a generic
  event bus — you are building a framework. Stop.
- **`data-*` is the contract.** HTML declares hooks (`data-seal`, `data-search`,
  `data-toggle="theme"`); JS reads them. Renaming a CSS class never breaks JS.
- **Every behavior no-ops gracefully** when its element is absent, and the page
  is fully usable with the file removed.
