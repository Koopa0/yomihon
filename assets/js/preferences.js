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

  // The server stamps this control's pressed state from the stored choice,
  // which is all it can see: prefers-color-scheme never reaches it. With no
  // choice stored and a dark system preference the page paints dark and the
  // attribute says otherwise, so the first thing this does is agree with what
  // the reader is looking at.
  themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));

  textsizeToggle?.addEventListener('click', (event) => {
    const next = { m: 'l', l: 'xl', xl: 'm' }[root.dataset.textsize] || 'l';
    setPreference('textsize', next);
    // The control cycles three sizes, so its state lives in its accessible
    // name rather than in aria-pressed. Rewriting the name on the focused
    // button is what tells a reader who cannot see the type what the press
    // just did.
    event.currentTarget.setAttribute('aria-label', textsizeLabel(next));
  });
  themeToggle?.addEventListener('click', (event) => {
    // The flip starts from what the reader sees: with no stored choice the
    // page may already be dark from the system, and the first press must then
    // choose light rather than restate dark.
    setPreference('theme', effectiveTheme() === 'dark' ? 'light' : 'dark');
    event.currentTarget.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));
  });
  document.querySelector('[data-ruby-toggle]')?.addEventListener('click', (event) => {
    setPreference('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.ruby === 'on'));
  });
  document.querySelector('[data-single-key-shortcuts-toggle]')?.addEventListener('change', (event) => {
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
    if (theme === 'dark' || theme === 'light') {
      root.dataset.theme = theme;
    } else {
      delete root.dataset.theme;
    }
    themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));
    const stored = readCookie('yomihon_textsize');
    const size = stored === 'l' || stored === 'xl' ? stored : 'm';
    root.dataset.textsize = size;
    textsizeToggle?.setAttribute('aria-label', textsizeLabel(size));
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
}
