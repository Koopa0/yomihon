// Persisted presentation preferences. The server renders the cookie-backed
// state on first byte; this enhancement only changes the root attributes and
// keeps the controls' pressed state current.
export function initPreferences() {
  const root = document.documentElement;

  function setPreference(name, value) {
    root.dataset[name] = value;
    document.cookie = `yomihon_${name}=${value};path=/;max-age=31536000;samesite=lax`;
  }

  document.querySelector('[data-theme-toggle]')?.addEventListener('click', (event) => {
    setPreference('theme', root.dataset.theme === 'dark' ? 'light' : 'dark');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.theme === 'dark'));
  });
  document.querySelector('[data-ruby-toggle]')?.addEventListener('click', (event) => {
    setPreference('ruby', root.dataset.ruby === 'off' ? 'on' : 'off');
    event.currentTarget.setAttribute('aria-pressed', String(root.dataset.ruby === 'on'));
  });
}
