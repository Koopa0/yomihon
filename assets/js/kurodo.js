/* kurodo runtime — the one client-side script this repo ships. All vanilla,
   zero framework, no build step, loaded `defer` from the layout. Every behavior
   here is progressive enhancement over a page that already works with JS off:

     - theme / furigana toggles: flip the root data-* attribute + cookie; the
       server renders the correct state on first byte and CSS does the visual
       switch, so JS only persists the choice.
     - nav drawer (narrow screens): open/close via the root data-nav attribute.
     - the seal: a press-and-hold gesture layered on a plain <form method="post">.
       On completion it calls form.requestSubmit() — never fetch — so the server
       sees exactly the no-JS submit; with JS off the button is a one-press seal.
     - the ?sealed=1 one-shot: the panel already renders its settle animation
       server-side; JS just strips the param so a refresh does not replay it.
     - ⌘K search: opens the native <dialog>; the header link to /search is the
       no-JS fallback.
     - mermaid: renders ```mermaid fences into SVG, lazily, only on pages that
       have one. Unchanged from the original runtime. */
(() => {
  'use strict';

  const root = document.documentElement;

  // ---- persisted toggles (theme, furigana) --------------------------------
  // Flip the root data-* attribute (CSS reacts instantly) and write the cookie
  // so the next server render matches. data-* is the whole HTML↔JS contract.
  function setToggle(name, value) {
    root.dataset[name] = value;
    document.cookie = `kurodo_${name}=${value};path=/;max-age=31536000;samesite=lax`;
  }
  function initToggles() {
    document.querySelector('[data-theme-toggle]')?.addEventListener('click', () => {
      setToggle('theme', root.dataset.theme === 'dark' ? 'light' : 'dark');
    });
    document.querySelector('[data-ruby-toggle]')?.addEventListener('click', () => {
      setToggle('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
    });
  }

  // ---- nav drawer (≤900) ---------------------------------------------------
  function initDrawer() {
    document.querySelector('[data-nav-toggle]')?.addEventListener('click', () => {
      root.dataset.nav = root.dataset.nav === 'open' ? 'closed' : 'open';
    });
    document.querySelector('[data-nav-close]')?.addEventListener('click', () => {
      root.dataset.nav = 'closed';
    });
  }

  // ---- the seal: hold-to-submit over a plain form --------------------------
  // The write path is a real POST form (no-JS: one press submits + PRG). JS adds
  // a ~430ms press-and-hold as a misclick guard, then submits the SAME form.
  const HOLD_MS = 430;
  let holdTimer = null;
  let holding = false;
  let sealing = false;
  // Every seal fill in the DOM animates together — the right-rail button and the
  // narrow-screen bar share the class, so whichever is visible fills identically.
  function sealFills() {
    return document.querySelectorAll('.k-sealfill');
  }
  function holdStart(form) {
    if (sealing || holding || !form) return;
    holding = true;
    sealFills().forEach((f) => {
      f.style.transition = `width ${HOLD_MS}ms linear`;
      f.style.width = '100%';
    });
    holdTimer = setTimeout(() => {
      holding = false;
      sealing = true;
      form.requestSubmit(); // NOT fetch: the server sees exactly the no-JS submit
    }, HOLD_MS + 20);
  }
  function holdEnd() {
    if (!holding) return;
    holding = false;
    clearTimeout(holdTimer);
    sealFills().forEach((f) => {
      f.style.transition = 'width 150ms ease';
      f.style.width = '0';
    });
  }
  function initSeal() {
    document.querySelectorAll('[data-seal]').forEach((form) => {
      const btn = form.querySelector('[data-seal-btn]');
      if (!btn) return;
      // The button stays a real type=submit so it seals in one press with JS
      // off. With JS on, only the completed hold may commit, so the NATIVE
      // submit must be suppressed. preventDefault on pointerdown is not enough:
      // for a mouse it cancels only the compat mouse events, not the click, so a
      // quick click would still submit — defeating the whole misclick guard.
      // Cancel the click itself; requestSubmit (the hold's path) never fires click.
      btn.addEventListener('click', (e) => e.preventDefault());
      btn.addEventListener('pointerdown', (e) => { e.preventDefault(); holdStart(form); });
      btn.addEventListener('pointerup', holdEnd);
      btn.addEventListener('pointerleave', holdEnd);
      btn.addEventListener('keydown', (e) => {
        if ((e.key === 'Enter' || e.key === ' ') && !e.repeat) { e.preventDefault(); holdStart(form); }
      });
      btn.addEventListener('keyup', (e) => { if (e.key === 'Enter' || e.key === ' ') holdEnd(); });
    });
  }
  // The settle animation renders server-side when ?sealed=1 is present; strip the
  // param (without reloading) so a manual refresh does not replay it.
  function stripSealSignal() {
    const u = new URL(location.href);
    if (u.searchParams.get('sealed') !== '1') return;
    u.searchParams.delete('sealed');
    history.replaceState(null, '', u);
  }

  // ---- ⌘K search dialog + global keys --------------------------------------
  function initSearch() {
    const dialog = document.querySelector('[data-search]');
    if (!dialog) return;
    document.querySelector('[data-search-open]')?.addEventListener('click', (e) => {
      e.preventDefault(); // no-JS falls through this link to /search
      if (!dialog.open) dialog.showModal();
    });
  }
  function initKeys() {
    const dialog = document.querySelector('[data-search]');
    const sealForm = document.querySelector('[data-seal]');
    window.addEventListener('keydown', (e) => {
      const t = e.target;
      const typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable);
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        if (dialog) { dialog.open ? dialog.close() : dialog.showModal(); }
        return;
      }
      if (e.key === 'Escape') {
        if (root.dataset.nav === 'open') root.dataset.nav = 'closed';
        holdEnd();
        return; // <dialog> closes itself on Escape
      }
      if (typing || (dialog && dialog.open)) return;
      if ((e.key === 'r' || e.key === 'R') && !e.repeat && sealForm && !sealing) {
        e.preventDefault();
        holdStart(sealForm);
      }
    });
    window.addEventListener('keyup', (e) => { if (e.key === 'r' || e.key === 'R') holdEnd(); });
  }

  // ---- mermaid (unchanged) -------------------------------------------------
  // internal/render's consumeMermaid encodes data-mermaid-code with Go's
  // net/url.QueryEscape, which — unlike JS's encodeURIComponent — encodes a
  // literal space as "+", not "%20". decodeURIComponent alone leaves a stray "+"
  // untouched, so every space in the diagram source would otherwise survive as a
  // literal "+" and break mermaid's parser. Un-doing the "+"-for-space convention
  // first, exactly like decoding an application/x-www-form-urlencoded value, keeps
  // the two sides in sync.
  function decodeMermaidCode(raw) {
    return decodeURIComponent(raw.replace(/\+/g, ' '));
  }
  async function renderMermaidDiagrams() {
    const blocks = document.querySelectorAll('.mermaid-diagram');
    if (blocks.length === 0) return; // never fetch the mermaid bundle on a page with no diagrams

    const { default: mermaid } = await import('/static/mermaid.esm.min.mjs');
    mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' });

    let next = 0;
    for (const el of blocks) {
      const code = decodeMermaidCode(el.getAttribute('data-mermaid-code') || '');
      if (!code) continue;
      const id = 'mermaid-diagram-' + next++;
      try {
        const { svg } = await mermaid.render(id, code);
        const parsed = new DOMParser().parseFromString(svg, 'image/svg+xml');
        const svgEl = parsed.documentElement;
        if (svgEl.nodeName.toLowerCase() !== 'svg') continue; // parse error — keep the source-text fallback
        // DOM-API replacement (no innerHTML); mermaid securityLevel:'strict'
        // sanitizes diagram content, DOMParser does not execute script.
        el.replaceChildren();
        el.appendChild(document.importNode(svgEl, true));
      } catch (err) {
        console.warn('[kurodo] mermaid diagram failed to render:', err);
      }
    }
  }

  // ---- boot ----------------------------------------------------------------
  function init() {
    initToggles();
    initDrawer();
    initSeal();
    stripSealSignal();
    initSearch();
    initKeys();
    renderMermaidDiagrams();
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
