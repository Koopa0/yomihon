/* yomihon runtime — the one client-side script this repo ships. All vanilla,
   zero framework, no build step, loaded `defer` from the layout. Every behavior
   here is progressive enhancement over a page that already works with JS off:

     - theme / furigana toggles: flip the root data-* attribute + cookie; the
       server renders the correct state on first byte and CSS does the visual
       switch, so JS only persists the choice.
     - nav drawer (narrow screens): the no-JS sidebar stays stacked and usable;
       JS alone adds the root data-nav state, inert closed state, and focus cycle.
     - the seal: a press-and-hold gesture layered on a plain <form method="post">.
       On completion it calls form.requestSubmit() — never fetch — so the server
       sees exactly the no-JS submit; with JS off the button is a one-press seal.
     - the ?sealed=1 one-shot: the panel already renders its settle animation
       server-side; JS just strips the param so a refresh does not replay it.
     - sidebar disclosures: one state owner — the wayfinding chain is always
       forced open and never recorded, manual toggles persist for the session
       (an inline sidebar script restores them pre-paint), and the filter's
       opens are transient.
     - TOC scroll-spy: marks the heading being read; a TOC click locks its
       target as active until the scroll settles, and holds the arrival echo
       back for the same window.
     - live search: /search and the ⌘K dialog fetch the same server-rendered
       lexical results after a short debounce; their GET forms remain the
       Enter/button/no-JS path. Home intentionally remains a plain GET form.
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
    document.cookie = `yomihon_${name}=${value};path=/;max-age=31536000;samesite=lax`;
  }
  // The server renders each toggle's pressed state on the first byte; here the
  // script keeps it current after a click, reading the flipped state back from
  // the root so the button announces its new state to assistive tech.
  function initToggles() {
    document.querySelector('[data-theme-toggle]')?.addEventListener('click', (e) => {
      setToggle('theme', root.dataset.theme === 'dark' ? 'light' : 'dark');
      e.currentTarget.setAttribute('aria-pressed', String(root.dataset.theme === 'dark'));
    });
    document.querySelector('[data-ruby-toggle]')?.addEventListener('click', (e) => {
      setToggle('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
      e.currentTarget.setAttribute('aria-pressed', String(root.dataset.ruby === 'on'));
    });
  }

  // ---- nav drawer (≤900) ---------------------------------------------------
  // Without this module the rail is ordinary stacked content and the hamburger
  // is hidden. At narrow widths JS upgrades that baseline into a modal-like
  // drawer: the closed rail is inert/hidden from AT, opening moves focus in,
  // Tab wraps inside, and every close path restores the trigger. data-nav is
  // client-only state; the server never stamps an initial value.
  const drawerMedia = window.matchMedia('(max-width: 900px)');
  let drawerRail = null;
  let drawerToggle = null;
  function drawerOpen() {
    return drawerMedia.matches && root.dataset.nav === 'open';
  }
  function drawerFocusables() {
    if (!drawerRail) return [];
    return [...drawerRail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [contenteditable], [tabindex]:not([tabindex="-1"])')]
      .filter((el) => el.tabIndex >= 0 && !el.hidden && el.getClientRects().length > 0);
  }
  function setNav(open) {
    root.dataset.nav = open ? 'open' : 'closed';
    if (drawerToggle) drawerToggle.setAttribute('aria-expanded', String(open));
    if (!drawerRail) return;
    if (!drawerMedia.matches) {
      drawerRail.inert = false;
      drawerRail.removeAttribute('aria-hidden');
      return;
    }
    drawerRail.inert = !open;
    if (open) { drawerRail.removeAttribute('aria-hidden'); } else { drawerRail.setAttribute('aria-hidden', 'true'); }
  }
  function focusDrawer() {
    const target = drawerFocusables()[0] || drawerRail;
    target?.focus();
  }
  function openDrawer() {
    if (!drawerMedia.matches || !drawerRail) return;
    setNav(true);
    focusDrawer();
  }
  function closeDrawer(restoreFocus) {
    setNav(false);
    if (restoreFocus && drawerMedia.matches) drawerToggle?.focus();
  }
  function containDrawerFocus(e) {
    if (!drawerOpen() || e.key !== 'Tab') return;
    if (document.querySelector('dialog[open]')) return; // the native modal owns focus while it is open
    const focusable = drawerFocusables();
    if (focusable.length === 0) {
      e.preventDefault();
      drawerRail?.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable.at(-1);
    const active = document.activeElement;
    if (e.shiftKey && (active === first || !drawerRail.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || !drawerRail.contains(active))) {
      e.preventDefault();
      first.focus();
    }
  }
  function initDrawer() {
    drawerRail = document.querySelector('#nav-rail');
    drawerToggle = document.querySelector('[data-nav-toggle]');
    if (!drawerRail || !drawerToggle) {
      if (drawerToggle) drawerToggle.hidden = true;
      return;
    }
    drawerRail.tabIndex = -1;
    setNav(false);
    drawerToggle.addEventListener('click', () => {
      if (drawerOpen()) { closeDrawer(true); } else { openDrawer(); }
    });
    document.querySelector('[data-nav-close]')?.addEventListener('click', () => {
      closeDrawer(true);
    });
    window.addEventListener('keydown', containDrawerFocus);
    drawerMedia.addEventListener('change', () => {
      const restoreFocus = drawerMedia.matches && drawerRail.contains(document.activeElement);
      setNav(false);
      if (restoreFocus) drawerToggle.focus();
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
    return document.querySelectorAll('.y-sealfill');
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

  // ---- sidebar disclosure state: one owner for <details open> ---------------
  // Three forces act on the sidebar's disclosures: the server's wayfinding
  // chain (data-chain — always forced open, never persisted), the reader's
  // manual toggles (persisted per data-key for the session; an inline script
  // in the sidebar reapplies them before first paint), and the filter's
  // transient opens while a query is active. This module is the only writer
  // of the stored state, and the filter asks it for the resting open state
  // instead of keeping a competing map of its own.
  const navState = (() => {
    const KEY = 'yomihon.nav';
    let filtering = false;
    const serverOpen = new Map();
    function read() {
      try { return JSON.parse(sessionStorage.getItem(KEY) || '{}') || {}; } catch { return {}; }
    }
    function init(rail) {
      rail.querySelectorAll('details[data-key]').forEach((d) => { serverOpen.set(d, d.open); });
      // A single delegated listener: toggle events bubble from every details.
      rail.addEventListener('toggle', (e) => {
        const d = e.target;
        if (filtering || !d.dataset || !d.dataset.key) return;
        if (d.hasAttribute('data-chain')) return; // the active chain is never recorded
        const stored = read();
        stored[d.dataset.key] = d.open;
        sessionStorage.setItem(KEY, JSON.stringify(stored));
      }, true);
    }
    // resting is the open state with no filter active: the wayfinding chain,
    // then the session's stored toggle, then the server's rendered default.
    function resting(d) {
      if (d.hasAttribute('data-chain')) return true;
      const want = read()[d.dataset.key];
      return typeof want === 'boolean' ? want : serverOpen.get(d);
    }
    return { init, resting, setFiltering(on) { filtering = on; } };
  })();

  // ---- sidebar filter (progressive enhancement) ----------------------------
  // A text box — shipped hidden and revealed by the sidebar's pre-paint
  // script, so it is there in the first frame and stays absent with JS off —
  // that narrows the sidebar to rows whose visible text matches: links and
  // non-link rows alike (an unresolved lesson is a span, and it matters for
  // wayfinding). A group (a
  // disclosure or the "here" list) with no surviving row collapses away; a
  // match keeps its ancestor disclosures open, so the path to it stays
  // visible. Clearing the query hands every disclosure back to its resting
  // state; a query with no match says so instead of leaving a silently
  // empty column. Enter follows the first link match; Esc clears and hands
  // focus back to the page.
  function initNavFilter() {
    const rail = document.querySelector('.y-rail-left');
    const input = rail && rail.querySelector('[data-nav-filter]');
    if (!input) return;
    navState.init(rail);
    const empty = rail.querySelector('[data-filter-empty]');
    const groups = [...rail.querySelectorAll('details, .y-here')];
    const rows = [...rail.querySelectorAll('a, span.ui-navitem')];
    function apply() {
      const q = input.value.trim().toLowerCase();
      navState.setFiltering(q !== '');
      if (empty) empty.hidden = true;
      if (!q) {
        rows.forEach((r) => { r.hidden = false; });
        groups.forEach((g) => {
          g.hidden = false;
          if (g.tagName === 'DETAILS') g.open = navState.resting(g);
        });
        return;
      }
      rows.forEach((r) => { r.hidden = !r.textContent.toLowerCase().includes(q); });
      groups.forEach((g) => {
        const hit = [...g.querySelectorAll('a, span.ui-navitem')].some((r) => !r.hidden);
        g.hidden = !hit;
        if (g.tagName === 'DETAILS') g.open = hit;
      });
      if (empty) empty.hidden = rows.some((r) => !r.hidden);
    }
    input.addEventListener('input', apply);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        const first = rail.querySelector('a:not([hidden])');
        if (first) first.click();
      } else if (e.key === 'Escape') {
        // Clear the filter and stop here: without stopPropagation the same Esc
        // would bubble to the global handler and also close the nav drawer, so
        // one press would do two things. A second Esc (filter now empty, focus
        // returned to the page) reaches the global handler and closes the drawer.
        e.preventDefault();
        e.stopPropagation();
        input.value = '';
        apply();
        input.blur();
      }
    });
  }

  // ---- TOC scroll-spy, coordinated with smooth scrolling --------------------
  // Marks the table-of-contents entry whose heading the reader is at, in the
  // rail and the inline disclosure alike. On a TOC click the clicked target
  // is locked as the active entry until the scroll settles (scrollend, with
  // a timeout fallback), so the spy never flickers through every heading in
  // between; the root carries data-traveling for the same window, which
  // holds the arrival echo back until the eye has arrived.
  function initTocSpy() {
    const main = document.querySelector('.y-main');
    const links = [...document.querySelectorAll('.y-toc__list a[href^="#"]')];
    if (!main || links.length === 0) return;
    const targetId = (a) => decodeURIComponent(a.getAttribute('href').slice(1));
    const headings = [...new Set(links.map(targetId))]
      .map((id) => document.getElementById(id))
      .filter(Boolean);
    if (headings.length === 0) return;
    let locked = null;
    let settleTimer = null;
    function mark(id) {
      links.forEach((a) => {
        const on = targetId(a) === id;
        a.classList.toggle('is-active', on);
        if (on) { a.setAttribute('aria-current', 'true'); } else { a.removeAttribute('aria-current'); }
      });
    }
    // The active heading is the last one at or above the reading line (a
    // quarter down the column). The observer only says when that answer can
    // have changed; the band edges are where headings cross it.
    function recompute() {
      if (locked) return;
      const top = main.getBoundingClientRect().top;
      const line = main.clientHeight * 0.25;
      let current = headings[0].id;
      for (const h of headings) {
        if (h.getBoundingClientRect().top - top <= line) { current = h.id; } else { break; }
      }
      mark(current);
    }
    const io = new IntersectionObserver(recompute, { root: main, rootMargin: '0px 0px -75% 0px' });
    headings.forEach((h) => { io.observe(h); });
    recompute();
    function settle() {
      clearTimeout(settleTimer);
      locked = null;
      delete root.dataset.traveling;
      recompute();
    }
    links.forEach((a) => {
      a.addEventListener('click', () => {
        locked = targetId(a);
        mark(locked);
        root.dataset.traveling = 'on';
        clearTimeout(settleTimer);
        main.addEventListener('scrollend', settle, { once: true });
        settleTimer = setTimeout(settle, 900);
      });
    });
  }

  // ---- live lexical search + the ⌘K dialog ---------------------------------
  // Each search surface remains an ordinary GET form. This read-only layer
  // listens while the reader types, asks the server for the same lexical result
  // markup, and replaces only that surface's result region. The monotonic
  // request ID is kept alongside AbortController because an abort can arrive
  // after a response has already won the network race.
  const LIVE_SEARCH_DELAY_MS = 180;
  function initLiveSearch() {
    document.querySelectorAll('[data-live-search]').forEach((region) => {
      const form = region.querySelector('[data-live-search-form]');
      const input = region.querySelector('[data-live-search-input]');
      const results = region.querySelector('[data-live-search-results]');
      const status = region.querySelector('[data-live-search-status]');
      const endpoint = region.getAttribute('data-live-search-endpoint');
      if (!form || !input || !results || !status || !endpoint) return;

      const endpointURL = new URL(endpoint, location.href);
      if (endpointURL.origin !== location.origin) return;

      let timer = null;
      let activeController = null;
      let latestRequest = 0;
      let composing = false;

      function cancelPending() {
        clearTimeout(timer);
        timer = null;
        latestRequest += 1;
        activeController?.abort();
        activeController = null;
        results.setAttribute('aria-busy', 'false');
      }

      function clearResults() {
        results.replaceChildren();
        results.dataset.resultCount = '0';
        status.textContent = '';
      }

      async function refresh(query, requestID) {
        const requestController = new AbortController();
        activeController = requestController;
        const url = new URL(endpointURL);
        url.searchParams.set('q', query);

        try {
          const response = await fetch(url, {
            headers: { Accept: 'text/html' },
            signal: requestController.signal,
          });
          if (!response.ok) throw new Error(`search response ${response.status}`);

          const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
          const fragment = doc.querySelector('[data-live-search-results]');
          const count = fragment && Number(fragment.dataset.resultCount);
          if (!fragment || !Number.isInteger(count) || count < 0) {
            throw new Error('search response has no valid result fragment');
          }
          if (requestID !== latestRequest) return;

          const imported = document.importNode(fragment, true);
          results.replaceChildren(...imported.childNodes);
          results.dataset.resultCount = String(count);
          status.textContent = `${count} ${count === 1 ? 'result' : 'results'} for “${query}”.`;
        } catch (err) {
          if (err.name === 'AbortError' || requestID !== latestRequest) return;
          status.textContent = 'Live results unavailable. Press Enter to search.';
        } finally {
          if (requestID === latestRequest) {
            results.setAttribute('aria-busy', 'false');
            if (activeController === requestController) activeController = null;
          }
        }
      }

      function schedule() {
        clearTimeout(timer);
        const query = input.value.trim();
        const requestID = ++latestRequest;
        activeController?.abort();
        activeController = null;
        if (!query) {
          timer = null;
          results.setAttribute('aria-busy', 'false');
          clearResults();
          return;
        }

        results.setAttribute('aria-busy', 'true');
        timer = setTimeout(() => {
          timer = null;
          refresh(query, requestID);
        }, LIVE_SEARCH_DELAY_MS);
      }

      input.addEventListener('compositionstart', () => {
        composing = true;
        cancelPending();
      });
      input.addEventListener('compositionend', () => {
        composing = false;
        schedule();
      });
      input.addEventListener('input', () => {
        if (!composing) schedule();
      });
      form.addEventListener('submit', cancelPending);
      if (region.tagName === 'DIALOG') region.addEventListener('close', cancelPending);
    });
  }

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
      // A focused <select> counts as typing too: its own letter keys jump to
      // matching options, so the single-key page shortcuts (including the held
      // R that starts the seal) must stay out of its way while it has focus.
      // The whole select subtree counts, not just the element itself: with the
      // customizable picker open, key events target the focused <option>.
      const typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable || (t.closest && t.closest('select')));
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        if (dialog) { dialog.open ? dialog.close() : dialog.showModal(); }
        return;
      }
      if (e.key === 'Escape') {
        if (dialog && dialog.open) {
          e.preventDefault();
          dialog.close();
          document.querySelector('[data-search-open]')?.focus();
          return;
        }
        if (drawerOpen()) closeDrawer(true);
        holdEnd();
        return;
      }
      if (typing || (dialog && dialog.open)) return;
      if (e.key === '/') {
        const filter = document.querySelector('[data-nav-filter]');
        if (filter && !filter.hidden) {
          e.preventDefault();
          if (drawerMedia.matches && !drawerOpen()) openDrawer();
          filter.focus();
        }
        return;
      }
      if (e.key === '[') {
        if (drawerMedia.matches) {
          e.preventDefault();
          if (drawerOpen()) { closeDrawer(true); } else { openDrawer(); }
        }
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
        // Hand the block back to its plain source text: the shimmer that
        // promised a render must not keep playing over a failure.
        el.setAttribute('data-mermaid-error', '');
        console.warn('[yomihon] mermaid diagram failed to render:', err);
      }
    }
  }

  // ---- speech (Web Speech), shared by TTS sentences + the slot machine -----
  // No-op when there is no engine; the UI is gated on the data-speech signal
  // (set at boot) so speak controls never appear when they could not work.
  let speechRate = 0.8;
  let speechGeneration = 0;
  let activeSpeakButton = null;
  let speechStatus = null;
  function resetSpeechButton() {
    if (!activeSpeakButton) return;
    activeSpeakButton.removeAttribute('data-speaking');
    activeSpeakButton.setAttribute('aria-label', '朗讀這段日文');
    activeSpeakButton = null;
  }
  function stopSpeech() {
    if (!('speechSynthesis' in window)) return;
    speechGeneration += 1;
    speechSynthesis.cancel();
    resetSpeechButton();
    if (speechStatus) speechStatus.textContent = '已停止';
  }
  function speakJa(text, trigger = null) {
    if (!text || !('speechSynthesis' in window)) return;
    if (trigger && trigger === activeSpeakButton) {
      stopSpeech();
      return;
    }
    stopSpeech(); // stop any in-flight utterance first
    const generation = speechGeneration;
    const u = new SpeechSynthesisUtterance(text);
    u.lang = 'ja-JP';
    u.rate = speechRate;
    if (trigger) {
      activeSpeakButton = trigger;
      trigger.setAttribute('data-speaking', '');
      trigger.setAttribute('aria-label', '停止朗讀');
    }
    u.addEventListener('start', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '播放中';
    }, { once: true });
    u.addEventListener('end', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '播放完成';
      if (generation === speechGeneration) resetSpeechButton();
    }, { once: true });
    u.addEventListener('error', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '目前無法播放日語語音';
      if (generation === speechGeneration) resetSpeechButton();
    }, { once: true });
    speechSynthesis.speak(u);
  }

  // ---- TTS: read a lesson sentence aloud -----------------------------------
  // The speak buttons are server-rendered (internal/render.InjectTTS) with the
  // reading-stripped text already in data-tts — this never crawls the DOM for
  // <rt>. CSS reveals them only under [data-speech], so with no JS or no engine
  // the affordance is simply absent and the sentence still reads.
  function initTTS() {
    if (!('speechSynthesis' in window)) return;
    const buttons = [...document.querySelectorAll('[data-tts]')];
    if (buttons.length === 0) return;

    const toolbar = document.createElement('div');
    toolbar.className = 'y-ttsbar';
    toolbar.setAttribute('role', 'group');
    toolbar.setAttribute('aria-label', '日文朗讀控制');
    const label = document.createElement('span');
    label.className = 'y-ttsbar__label';
    label.textContent = '朗讀速度';
    toolbar.append(label);
    [0.8, 1].forEach((rate) => {
      const rateButton = document.createElement('button');
      rateButton.type = 'button';
      rateButton.textContent = `${rate.toFixed(1)}×`;
      rateButton.setAttribute('aria-pressed', String(rate === speechRate));
      rateButton.addEventListener('click', () => {
        speechRate = rate;
        toolbar.querySelectorAll('button[data-speech-rate]').forEach((candidate) => {
          candidate.setAttribute('aria-pressed', String(candidate === rateButton));
        });
        stopSpeech();
        if (speechStatus) speechStatus.textContent = `速度 ${rate.toFixed(1)}×`;
      });
      rateButton.dataset.speechRate = String(rate);
      toolbar.append(rateButton);
    });
    speechStatus = document.createElement('span');
    speechStatus.className = 'y-ttsbar__status';
    speechStatus.setAttribute('aria-live', 'polite');
    toolbar.append(speechStatus);
    const stopButton = document.createElement('button');
    stopButton.type = 'button';
    stopButton.className = 'y-ttsbar__stop';
    stopButton.textContent = '停止';
    stopButton.addEventListener('click', stopSpeech);
    toolbar.append(stopButton);
    const firstLine = buttons[0].closest('.y-reading');
    if (firstLine) firstLine.before(toolbar);

    buttons.forEach((btn) => {
      btn.addEventListener('click', () => speakJa(btn.getAttribute('data-tts'), btn));
    });
  }

  // ---- slot machine: swap fills, recolour, shuffle, speak ------------------
  // Each card carries its data in a <script class="y-slotdata"> JSON blob; the
  // sentence is already server-rendered on the first fill, so the card reads
  // with JS off. This upgrades it: a <select> change (or shuffle) rewrites every
  // matching output span's base (ruby>span) and reading (rt) and re-substitutes
  // the gloss. data-* names are the whole HTML<->JS contract.
  function initSlots() {
    document.querySelectorAll('.y-slotcard').forEach(initSlotCard);
  }
  function initSlotCard(card) {
    const dataEl = card.querySelector('script.y-slotdata');
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
        card.querySelectorAll(`.y-slotout[data-slot-key="${k}"]`).forEach((out) => {
          const base = out.querySelector('ruby > span');
          const rt = out.querySelector('rt');
          if (base) base.textContent = f.jp;
          if (rt) rt.textContent = f.reading;
        });
      });
      const g = card.querySelector('.y-slotgloss');
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
    initTocSpy();
    initLiveSearch();
    initSearch();
    initKeys();
    initTTS();
    initSlots();
    initConceptSheet();
    renderMermaidDiagrams().catch((err) => {
      root.setAttribute('data-mermaid-error', '');
      console.warn('[yomihon] mermaid rendering failed outside a diagram:', err);
    });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
