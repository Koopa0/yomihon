// Persisted presentation preferences. The server renders the cookie-backed
// state on first byte; this enhancement changes the root attributes, keeps the
// controls' state current, and re-syncs both after a back/forward-cache
// restore revives HTML older than the cookies.
//
// Language is not handled here: its words are the server's, so its control is
// a plain form the server answers with a redirect. This module only checks,
// after a cache restore, that the revived document still speaks the language
// the cookie names — and asks for a fresh page when it does not, since no
// script can retranslate a rendered document.
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
  });

  // What the reader is looking at, and a way to be told when that changes.
  return {
    theme: effectiveTheme,
    onThemeChange(listener) {
      themeChanged = listener;
    },
  };
}
