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
       have one. Unchanged from the original runtime.
     - TTS: lesson sentences carry a server-rendered speak button (data-tts, the
       reading already stripped); click reads it via the Web Speech API. Buttons
       stay hidden unless a speech engine exists, so no-JS / no-engine degrades
       to silent absence and the sentence still reads. */
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
      btn.addEventListener('pointercancel', holdEnd);
      btn.addEventListener('keydown', (e) => {
        if ((e.key === 'Enter' || e.key === ' ') && !e.repeat) { e.preventDefault(); holdStart(form); }
      });
      btn.addEventListener('keyup', (e) => { if (e.key === 'Enter' || e.key === ' ') holdEnd(); });
    });
    // Losing focus mid-hold (cmd-tab, alt-tab) means the pointerup/keyup that
    // would cancel never arrives — cancel the hold so the running timer can't
    // seal unwatched.
    window.addEventListener('blur', holdEnd);
  }
  // The settle animation renders server-side when ?sealed=1 is present; strip the
  // param (without reloading) so a manual refresh does not replay it.
  function stripSealSignal() {
    const u = new URL(location.href);
    if (u.searchParams.get('sealed') !== '1') return;
    u.searchParams.delete('sealed');
    history.replaceState(null, '', u);
  }

  // ---- sidebar filter (progressive enhancement) ----------------------------
  // A text box, hidden until this runs, that narrows the sidebar to entries
  // whose visible text matches. A group (a disclosure or the "here" list) with
  // no surviving entry collapses away; a match keeps its ancestor disclosures
  // open, so the path to it stays visible. Enter follows the first match; Esc
  // clears and hands focus back to the page. Zero network, zero persisted state.
  function initNavFilter() {
    const rail = document.querySelector('.k-rail-left');
    const input = rail && rail.querySelector('[data-nav-filter]');
    if (!input) return;
    input.hidden = false;
    const groups = [...rail.querySelectorAll('details, .k-here')];
    const wasOpen = new Map();
    rail.querySelectorAll('details').forEach((d) => { wasOpen.set(d, d.open); });
    const links = [...rail.querySelectorAll('a')];
    function apply() {
      const q = input.value.trim().toLowerCase();
      if (!q) {
        links.forEach((a) => { a.hidden = false; });
        groups.forEach((g) => {
          g.hidden = false;
          if (g.tagName === 'DETAILS') g.open = wasOpen.get(g);
        });
        return;
      }
      links.forEach((a) => { a.hidden = !a.textContent.toLowerCase().includes(q); });
      groups.forEach((g) => {
        const hit = [...g.querySelectorAll('a')].some((a) => !a.hidden);
        g.hidden = !hit;
        if (g.tagName === 'DETAILS') g.open = hit;
      });
    }
    input.addEventListener('input', apply);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        const first = rail.querySelector('a:not([hidden])');
        if (first) first.click();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        input.value = '';
        apply();
        input.blur();
      }
    });
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
      if (e.key === '/') {
        const filter = document.querySelector('[data-nav-filter]');
        if (filter && !filter.hidden) { e.preventDefault(); filter.focus(); }
        return;
      }
      if (e.key === '[') {
        e.preventDefault();
        root.dataset.nav = root.dataset.nav === 'open' ? 'closed' : 'open';
        return;
      }
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

  // ---- speech (Web Speech), shared by TTS sentences + the slot machine -----
  // No-op when there is no engine; the UI is gated on the data-speech signal
  // (set at boot) so speak controls never appear when they could not work.
  function speakJa(text) {
    if (!text || !('speechSynthesis' in window)) return;
    speechSynthesis.cancel(); // stop any in-flight utterance first
    const u = new SpeechSynthesisUtterance(text);
    u.lang = 'ja-JP';
    speechSynthesis.speak(u);
  }

  // ---- TTS: read a lesson sentence aloud -----------------------------------
  // The speak buttons are server-rendered (internal/render.InjectTTS) with the
  // reading-stripped text already in data-tts — this never crawls the DOM for
  // <rt>. CSS reveals them only under [data-speech], so with no JS or no engine
  // the affordance is simply absent and the sentence still reads.
  function initTTS() {
    document.querySelectorAll('[data-tts]').forEach((btn) => {
      btn.addEventListener('click', () => speakJa(btn.getAttribute('data-tts')));
    });
  }

  // ---- slot machine: swap fills, recolour, shuffle, speak ------------------
  // Each card carries its data in a <script class="k-slotdata"> JSON blob; the
  // sentence is already server-rendered on the first fill, so the card reads
  // with JS off. This upgrades it: a <select> change (or shuffle) rewrites every
  // matching output span's base (ruby>span) and reading (rt) and re-substitutes
  // the gloss. data-* names are the whole HTML<->JS contract.
  function initSlots() {
    document.querySelectorAll('.k-slotcard').forEach(initSlotCard);
  }
  function initSlotCard(card) {
    const dataEl = card.querySelector('script.k-slotdata');
    if (!dataEl) return;
    let data;
    try { data = JSON.parse(dataEl.textContent); } catch { return; } // keep server state
    const keys = data.keys || [];
    const sel = {};
    keys.forEach((k) => { sel[k] = 0; });
    const fill = (k) => {
      const s = data.slots[k];
      if (!s || !s.fills.length) return null;
      return s.fills[sel[k]] || s.fills[0];
    };
    function render() {
      keys.forEach((k) => {
        const f = fill(k);
        if (!f) return;
        card.querySelectorAll(`.k-slotout[data-slot-key="${k}"]`).forEach((out) => {
          const base = out.querySelector('ruby > span');
          const rt = out.querySelector('rt');
          if (base) base.textContent = f.jp;
          if (rt) rt.textContent = f.reading;
        });
      });
      const g = card.querySelector('.k-slotgloss');
      if (g) g.textContent = data.gloss.replace(/\{([A-Za-z0-9]+)\}/g, (_, k) => {
        const f = fill(k);
        return f ? f.zh : '{' + k + '}';
      });
    }
    card.querySelectorAll('select[data-slot-key]').forEach((s) => {
      s.addEventListener('change', () => {
        sel[s.getAttribute('data-slot-key')] = Number(s.value) || 0;
        render();
      });
    });
    card.querySelector('[data-slot-action="speak"]')?.addEventListener('click', () => {
      speakJa(data.template.replace(/\{([A-Za-z0-9]+)\}/g, (_, k) => { const f = fill(k); return f ? f.jp : ''; }));
    });
    card.querySelector('[data-slot-action="shuffle"]')?.addEventListener('click', () => {
      keys.forEach((k) => {
        const n = (data.slots[k] && data.slots[k].fills.length) || 0;
        if (!n) return;
        sel[k] = Math.floor(Math.random() * n);
        const s = card.querySelector(`select[data-slot-key="${k}"]`);
        if (s) s.value = String(sel[k]);
      });
      render();
    });
  }

  // ---- concept sheet: open a grammar note in a native <dialog> -------------
  // Triggers are real <a> links to the concept note (no-JS: they navigate).
  // With JS, a click clones the concept's pre-rendered <template> into one
  // shared <dialog> and opens it, so the reader never leaves the lesson. The
  // content is inline, so it opens instantly with no fetch.
  function initConceptSheet() {
    const dialog = document.querySelector('[data-concept-sheet]');
    if (!dialog) return;
    const titleEl = dialog.querySelector('[data-concept-title]');
    const bodyEl = dialog.querySelector('[data-concept-body]');
    document.addEventListener('click', (e) => {
      const trigger = e.target.closest('[data-concept]');
      if (trigger) {
        const tpl = document.getElementById('concept-' + trigger.getAttribute('data-concept'));
        if (!tpl) return; // no template → let the link navigate as the fallback
        e.preventDefault();
        titleEl.textContent = tpl.dataset.title || '';
        bodyEl.replaceChildren(tpl.content.cloneNode(true));
        bodyEl.scrollTop = 0;
        if (!dialog.open) dialog.showModal();
        return;
      }
      if (e.target.closest('[data-concept-close]')) { dialog.close(); return; }
      if (e.target === dialog) dialog.close(); // click on the backdrop
    });
  }

  // ---- boot ----------------------------------------------------------------
  function init() {
    // Reveal signals for progressive enhancement: [data-js] shows controls that
    // only work with JS (slot selects, shuffle); [data-speech] shows the speak
    // controls only when an engine exists. CSS keys on these; the server never
    // sets them, so no-JS pages hide the controls and still read.
    root.dataset.js = 'on';
    if ('speechSynthesis' in window) root.dataset.speech = 'on';
    initToggles();
    initDrawer();
    initNavFilter();
    initSeal();
    stripSealSignal();
    initSearch();
    initKeys();
    initTTS();
    initSlots();
    initConceptSheet();
    renderMermaidDiagrams();
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
