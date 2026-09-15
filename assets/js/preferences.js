// Persisted presentation preferences, from both ends: the header's own
// toggles, and the reading choices page where the same six are set out in
// full. The server renders the cookie-backed state on first byte; this
// enhancement changes the root attributes, keeps every control that reports
// them current, and re-syncs all of it after a back/forward-cache restore
// revives HTML older than the cookies.
//
// Nothing here writes the cookies the choices page owns. Its form is posted
// and the server's own answer carries them, so a submission the server refuses
// leaves this browser holding exactly what it held before — and the controls
// are put back to match it.
//
// Language is the one choice no script can apply: its words are the server's,
// so its control navigates. In the header that is a plain form; on the choices
// page the same form is submitted and comes back to the page it was sent from.
// A cache restore gets the same treatment from the other side — the revived
// document is checked against the language the cookie names, and a fresh page
// asked for when they differ, since no script can retranslate a rendered one.
export function initPreferences() {
  const root = document.documentElement;

  function readCookie(name) {
    for (const part of document.cookie.split(';')) {
      const eq = part.indexOf('=');
      if (eq > -1 && part.slice(0, eq).trim() === name) {
        return part.slice(eq + 1).trim();
      }
    }
    return null;
  }

  function setPreference(name, value) {
    root.dataset[name] = value;
    document.cookie = `yomihon_${name}=${value};path=/;max-age=31536000;samesite=lax`;
  }

  function setSingleKeyShortcuts(value) {
    root.dataset.singleKeyShortcuts = value;
    document.cookie = `yomihon_shortcuts=${value};path=/;max-age=31536000;samesite=lax`;
  }

  const themeToggle = document.querySelector('[data-theme-toggle]');
  const textsizeToggle = document.querySelector('[data-textsize-toggle]');
  const rubyToggle = document.querySelector('[data-ruby-toggle]');
  const shortcutsToggle = document.querySelector('[data-single-key-shortcuts-toggle]');

  // The size names come from the control itself, where the server wrote them
  // in the page's language; a copy here would be a second dictionary.
  function textsizeLabel(size) {
    return textsizeToggle?.dataset[{ m: 'labelM', l: 'labelL', xl: 'labelXl' }[size]];
  }

  // What the reader currently sees: their stored choice when one is on the
  // root, otherwise whichever theme the system preference painted.
  function effectiveTheme() {
    return (
      root.dataset.theme ||
      (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    );
  }

  // Anything drawn in the theme's own colours, rather than styled by the
  // stylesheet, has to be told when the theme moves. One listener, because one
  // thing is drawn that way.
  let themeChanged = () => {};

  // The server stamps this control's pressed state from the stored choice,
  // which is all it can see: prefers-color-scheme never reaches it. With no
  // choice stored and a dark system preference the page paints dark and the
  // attribute says otherwise. So the control is told from here, from what the
  // reader is actually looking at — on arrival, after a write, and when the
  // system moves underneath a reader who stored nothing.
  function markThemePressed() {
    themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));
  }

  // The only place the reader's theme reaches the root element. A choice they
  // made is stored as well; the rewrite from the cookie after a cache restore
  // is not, and may have no value to write at all. Both leave through the same
  // announcement, so a write site added later cannot leave a drawing a frame
  // behind the page by not thinking to mention itself.
  function writeTheme(choice, stored) {
    const before = effectiveTheme();
    if (stored) {
      setPreference('theme', choice);
    } else if (choice) {
      root.dataset.theme = choice;
    } else {
      delete root.dataset.theme;
    }
    const after = effectiveTheme();
    markThemePressed();
    // A write that left the theme where it was is not news. The cookie read
    // after a cache restore usually names the theme the page is already
    // showing, and announcing that would redraw every diagram on the page each
    // time the reader stepped back to it.
    if (after !== before) themeChanged();
  }

  // Nothing has moved yet: this is the server's own stamp going through the
  // same door, so the control agrees with what the reader is looking at from
  // the first paint rather than only after they touch it.
  writeTheme(root.dataset.theme || null, false);

  // A reader who stored no choice is following the system, and the system can
  // change while they are reading: the stylesheet repaints on its own, but the
  // control that reports the theme and anything drawn in the theme's colours
  // are told here. A stored choice fixes the theme, so the same change moves
  // nothing and says nothing.
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (root.dataset.theme) return;
    markThemePressed();
    themeChanged();
  });

  textsizeToggle?.addEventListener('click', (event) => {
    const next = { m: 'l', l: 'xl', xl: 'm' }[root.dataset.textsize] || 'l';
    setPreference('textsize', next);
    // The control cycles three sizes, so its state lives in its accessible
    // name rather than in aria-pressed. Rewriting the name on the focused
    // button is what tells a reader who cannot see the type what the press
    // just did.
    event.currentTarget.setAttribute('aria-label', textsizeLabel(next));
  });
  themeToggle?.addEventListener('click', () => {
    // The flip starts from what the reader sees: with no stored choice the
    // page may already be dark from the system, and the first press must then
    // choose light rather than restate dark.
    writeTheme(effectiveTheme() === 'dark' ? 'light' : 'dark', true);
  });
  rubyToggle?.addEventListener('click', (event) => {
    setPreference('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.ruby === 'on'));
  });
  shortcutsToggle?.addEventListener('change', (event) => {
    const value = event.currentTarget.checked ? 'on' : 'off';
    setSingleKeyShortcuts(value);
  });

  // The page a reader sets the reading on carries the same six choices as one
  // form of radios, and that form is what a browser with no scripting submits
  // by hand. What follows does not replace it: picking an option applies the
  // five the root can answer and sends the whole form at once, so the button
  // below the choices has nothing left to do and the stylesheet takes it off
  // the page.
  const settingsForm = document.querySelector('[data-preferences-form]');
  const settingsFailure = document.querySelector('[data-preferences-failed]');

  // What applying one choice means here, one entry per field the page names.
  // The optimistic apply and the undoing after a refusal both go through this
  // table, so a choice and its reversal can never move different things — the
  // header's own controls, which report the same state from the other end of
  // the page, included.
  const applyChoice = {
    theme(value) {
      // Following the system is the absence of a stored answer, and the root
      // holds that state by carrying no attribute at all.
      writeTheme(value === 'system' ? null : value, false);
    },
    textsize(value) {
      root.dataset.textsize = value;
      // The header control reports the size in its accessible name, from the
      // words the server wrote on it. The page's own words for the same three
      // sizes are a separate set and stay where they are.
      textsizeToggle?.setAttribute('aria-label', textsizeLabel(value));
    },
    font(value) {
      // Every face the page offers is one a cookie may carry, and the
      // stylesheet writes the base face out under its own name, so stamping
      // the chosen face reads the same as leaving the root bare.
      root.dataset.font = value;
    },
    ruby(value) {
      root.dataset.ruby = value;
      rubyToggle?.setAttribute('aria-pressed', String(value === 'on'));
    },
    shortcuts(value) {
      root.dataset.singleKeyShortcuts = value;
      if (shortcutsToggle) shortcutsToggle.checked = value === 'on';
    },
  };

  // The fields this page actually rendered. Which choices exist is the
  // server's answer and has been shortened before; reaching for one that is
  // not there would throw during boot, and a throw here takes every other
  // enhancement on the page down with it.
  const settingsFields = settingsForm
    ? Object.keys(applyChoice).filter((name) => settingsForm.elements[name])
    : [];

  // The answer the server last accepted for each field: what the page was
  // rendered from, and what a refused submission puts back.
  const accepted = {};

  let sending = null; // the submission in flight, or null
  let sendAgain = false; // a choice moved while that submission was running
  let handedOver = false; // a language change is navigating; nothing else goes

  // One submission at a time. Every one of them carries every field, so two in
  // the air at once can be answered in either order and the older answer would
  // put a field back where the reader had just moved it from. Waiting means
  // the next submission is built after the last is answered, out of what the
  // page shows by then — so the final submission carries every choice the
  // reader has made and no answer can land after it.
  function saveSettings() {
    if (handedOver) return;
    if (sending) {
      sendAgain = true;
      return;
    }
    void sendSettings();
  }

  async function sendSettings() {
    sendAgain = false;
    const sent = {};
    for (const name of settingsFields) sent[name] = settingsForm.elements[name].value;
    const body = new URLSearchParams(new FormData(settingsForm));
    const attempt = new AbortController();
    sending = attempt;
    let stored = false;
    try {
      const response = await fetch(settingsForm.action, {
        method: 'POST',
        body,
        credentials: 'same-origin',
        // A stored submission is answered with a redirect back to the reading,
        // which is what a form navigation wants and what this has no use for:
        // the cookies ride on that answer's own headers. Declining to follow
        // it keeps this to the one round trip, and the opaque answer that
        // comes back is the server saying it stored them.
        redirect: 'manual',
        signal: attempt.signal,
      });
      stored = response.type === 'opaqueredirect' || response.ok;
    } catch {
      stored = false;
    }
    sending = null;
    if (attempt.signal.aborted) return;
    if (!stored) {
      sendAgain = false;
      refuseSettings(sent);
      return;
    }
    Object.assign(accepted, sent);
    showSettingsFailure(false);
    if (sendAgain) void sendSettings();
  }

  // A submission that was not stored leaves the page showing choices this
  // browser is not in fact set to. Every field it carried goes back to the
  // answer the server last accepted — the controls and the reading together —
  // and the page says so, because a choice left sitting there looking chosen
  // is the one failure a reader has no way of noticing.
  function refuseSettings(sent) {
    for (const [name, value] of Object.entries(sent)) {
      if (value === accepted[name]) continue;
      settingsForm.elements[name].value = accepted[name];
      applyChoice[name](accepted[name]);
    }
    showSettingsFailure(true);
  }

  function showSettingsFailure(shown) {
    if (settingsFailure) settingsFailure.hidden = !shown;
  }

  // The interface language is the choice this page cannot answer: the words on
  // a rendered page are the server's, so the form is submitted and a whole new
  // page comes back. It comes back to this page rather than to the reading, so
  // a reader setting several things up is not thrown out of the page by the
  // first of them; the address they arrived from rides along in the query, so
  // the way back to the reading is still the one they came by. Whatever was in
  // flight is dropped: this submission carries every field, the newest values
  // included, and an older answer landing after it would undo one of them.
  function handOverForLanguage() {
    handedOver = true;
    sending?.abort();
    const next = settingsForm.elements.next;
    if (next) next.value = location.pathname + location.search;
    settingsForm.requestSubmit();
  }

  // Put the page's own controls back on the answers this browser holds, and
  // take those as the accepted ones. Called where a revived document's
  // controls have gone stale against the cookies, so that a later refusal
  // reverts to what is stored rather than to what the page was drawn with.
  function syncSettings(values) {
    if (!settingsForm) return;
    for (const [name, value] of Object.entries(values)) {
      if (!settingsFields.includes(name)) continue;
      settingsForm.elements[name].value = value;
      accepted[name] = value;
    }
    showSettingsFailure(false);
  }

  if (settingsForm) {
    for (const name of settingsFields) accepted[name] = settingsForm.elements[name].value;
    settingsForm.addEventListener('change', (event) => {
      const name = event.target?.name;
      if (name === 'lang') {
        handOverForLanguage();
        return;
      }
      if (!settingsFields.includes(name)) return;
      applyChoice[name](event.target.value);
      saveSettings();
    });
    // The button below the choices is a browser-without-scripting's whole way
    // of being heard, and it goes only once the code that replaces it is
    // running. Saying so on the form is what takes it away, so a page whose
    // enhancement never started keeps it.
    settingsForm.dataset.preferencesLive = 'on';
  }

  // A back/forward-cache restore revives the document exactly as it left,
  // while the cookies may have moved on — a theme chosen on the next page
  // arrives back on a page still stamped with the old one. The cookies are
  // the truth, so the root attributes and the controls' state are rewritten
  // from them, honouring only the values the server honours. An ordinary
  // load needs none of this and is gated out: the server just stamped the
  // same cookies itself.
  window.addEventListener('pageshow', (event) => {
    if (!event.persisted) {
      return;
    }
    const theme = readCookie('yomihon_theme');
    writeTheme(theme === 'dark' || theme === 'light' ? theme : null, false);
    const stored = readCookie('yomihon_textsize');
    const size = stored === 'l' || stored === 'xl' ? stored : 'm';
    root.dataset.textsize = size;
    textsizeToggle?.setAttribute('aria-label', textsizeLabel(size));
    // Only the literal "off" is off, matching the server's stamp: any other
    // cookie, or none, is the default on. Both the root and the control have
    // to move together or a restored page shows furigana the cookie refused
    // and a button that claims the opposite.
    const ruby = readCookie('yomihon_ruby') === 'off' ? 'off' : 'on';
    root.dataset.ruby = ruby;
    rubyToggle?.setAttribute('aria-pressed', String(ruby === 'on'));
    // The reading typeface honours the same three names the server does, and
    // like the theme it may have nothing to write: a reader who chose none
    // leaves the attribute off the root rather than on a default.
    const font = readCookie('yomihon_font');
    if (font === 'serif' || font === 'sans' || font === 'kai') {
      root.dataset.font = font;
    } else {
      delete root.dataset.font;
    }
    const shortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';
    root.dataset.singleKeyShortcuts = shortcuts;
    if (shortcutsToggle) shortcutsToggle.checked = shortcuts === 'on';
    // The document's language cannot be rewritten in place, so a stale one
    // means asking for the page again. The comparison normalises the cookie
    // exactly as the server does — anything but "en" reads as the default —
    // so a value the server would ignore can never reload in a loop: the
    // fresh page always satisfies the same comparison.
    const lang = readCookie('yomihon_lang') === 'en' ? 'en' : 'zh-Hant';
    if (root.lang !== lang) {
      location.reload();
    }
    // The reading choices page shows these same answers as controls of its
    // own, and a revived copy of it is still showing whatever was chosen
    // before the reader left — a theme moved from the header two pages later
    // leaves a radio here claiming the old one. The values are taken from the
    // root this handler has just corrected rather than read from the cookies
    // a second time, so there is one reading of them and the two surfaces
    // cannot disagree. An unstamped root is the option that stores nothing,
    // which is what each of those two choices offers in its place.
    syncSettings({
      theme: root.dataset.theme ?? 'system',
      textsize: size,
      font: root.dataset.font ?? 'serif',
      ruby,
      shortcuts,
    });
  });

  // What the reader is looking at, and a way to be told when that changes.
  return {
    theme: effectiveTheme,
    onThemeChange(listener) {
      themeChanged = listener;
    },
  };
}
