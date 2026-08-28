// Persisted presentation preferences. The server renders the cookie-backed
// state on first byte; this enhancement only changes the root attributes and
// keeps the controls' pressed state current.
export function initPreferences() {
  const root = document.documentElement;

  function setPreference(name, value) {
    root.dataset[name] = value;
    document.cookie = `yomihon_${name}=${value};path=/;max-age=31536000;samesite=lax`;
  }

  function setSingleKeyShortcuts(value) {
    root.dataset.singleKeyShortcuts = value;
    document.cookie = `yomihon_shortcuts=${value};path=/;max-age=31536000;samesite=lax`;
  }

  document.querySelector('[data-textsize-toggle]')?.addEventListener('click', (event) => {
    const next = { m: 'l', l: 'xl', xl: 'm' }[root.dataset.textsize] || 'l';
    setPreference('textsize', next);
    // The control cycles three sizes, so its state lives in its accessible
    // name rather than in aria-pressed. Rewriting the name on the focused
    // button is what tells a reader who cannot see the type what the press
    // just did; the server renders the same three strings on load.
    event.currentTarget.setAttribute('aria-label', { m: '字級：中', l: '字級：大', xl: '字級：特大' }[next]);
  });
  document.querySelector('[data-theme-toggle]')?.addEventListener('click', (event) => {
    setPreference('theme', root.dataset.theme === 'dark' ? 'light' : 'dark');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.theme === 'dark'));
  });
  document.querySelector('[data-ruby-toggle]')?.addEventListener('click', (event) => {
    setPreference('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.ruby === 'on'));
  });
  // Language is the one preference no stylesheet can act on: the words are the
  // server's, so the choice is stored and the page asked for again. A plain
  // reload is what keeps the reader's place; a navigation to the same address
  // would not.
  document.querySelector('[data-lang-toggle]')?.addEventListener('click', () => {
    const next = root.lang === 'en' ? 'zh-Hant' : 'en';
    document.cookie = `yomihon_lang=${next};path=/;max-age=31536000;samesite=lax`;
    location.reload();
  });
  document.querySelector('[data-single-key-shortcuts-toggle]')?.addEventListener('change', (event) => {
    const value = event.currentTarget.checked ? 'on' : 'off';
    setSingleKeyShortcuts(value);
  });
}
