// Persisted presentation preferences. The server renders the cookie-backed
// state on first byte; this enhancement changes the root attributes, keeps the
// controls' state current, and re-syncs both after a back/forward-cache
// restore revives HTML older than the cookies.
//
// Two places set these choices — the controls in the header, and the page that
// names each one and says what it does — and both leave through the same
// doors below, so there is one way a choice reaches this browser rather than
// one per control. A write is read back before it is believed: storage this
// address may not use is refused silently, and a choice nothing kept must not
// be left looking chosen.
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

  // The size reaches the root, the cookie, and the control that reports it
  // through one door. The control cycles three sizes, so its state lives in its
  // accessible name rather than in aria-pressed, and a write that moved the
  // size without renaming it would leave a reader who cannot see the type being
  // told the wrong thing by the very control that names it.
  function writeTextSize(size) {
    setPreference('textsize', size);
    textsizeToggle?.setAttribute('aria-label', textsizeLabel(size));
  }

  // Furigana and the single-key keys are switches, and their controls report
  // which state they are in, so each value and the control saying it move
  // together for the same reason the size does.
  function writeRuby(value) {
    setPreference('ruby', value);
    rubyToggle?.setAttribute('aria-pressed', String(value === 'on'));
  }

  function writeShortcuts(value) {
    setSingleKeyShortcuts(value);
    if (shortcutsToggle) shortcutsToggle.checked = value === 'on';
  }

  textsizeToggle?.addEventListener('click', () => {
    writeTextSize({ m: 'l', l: 'xl', xl: 'm' }[root.dataset.textsize] || 'l');
  });
  themeToggle?.addEventListener('click', () => {
    // The flip starts from what the reader sees: with no stored choice the
    // page may already be dark from the system, and the first press must then
    // choose light rather than restate dark.
    writeTheme(effectiveTheme() === 'dark' ? 'light' : 'dark', true);
  });
  rubyToggle?.addEventListener('click', () => {
    writeRuby(root.dataset.ruby === 'off' ? 'on' : 'off');
  });
  shortcutsToggle?.addEventListener('change', (event) => {
    const value = event.currentTarget.checked ? 'on' : 'off';
    writeShortcuts(value);
  });

  // The page a reader sets the reading on. Its choices are the same six, so
  // they leave through the doors above rather than through a second way of
  // storing the same thing. The form those radios sit in stays underneath,
  // unchanged and unused here: it is what applies a choice for a browser
  // running none of this, and it is still how the interface language is
  // applied for every browser, because the words on a rendered page are
  // written by the server and no script can rewrite them.
  const settingsForm = document.querySelector('[data-prefs]');

  // What each choice does to this browser when it is picked. A choice missing
  // from here is one nothing on this side can carry out, and is asked of the
  // server instead.
  const applyChoice = {
    theme(value, stored) {
      if (stored) {
        writeTheme(value, true);
        return;
      }
      // Following the system is held by the absence of a stored value, so
      // picking it removes one rather than storing a word meaning "none" —
      // the same deletion the server writes for this option.
      document.cookie = 'yomihon_theme=;path=/;max-age=0;samesite=lax';
      writeTheme(null, false);
    },
    textsize: writeTextSize,
    font(value) {
      setPreference('font', value);
    },
    ruby: writeRuby,
    shortcuts: writeShortcuts,
  };

  // Where each choice currently stands, so one this browser refuses can be put
  // back where it was instead of left sitting on a value nothing stored.
  const standing = new Map();

  function settingsFields() {
    return settingsForm ? settingsForm.querySelectorAll('[data-pref-field]') : [];
  }

  function optionsOf(fieldset) {
    return [...fieldset.querySelectorAll('input[type="radio"]')];
  }

  // Whether picking this option stores a value. The page marks the options the
  // server would accept into the cookie; the one it does not mark is held by
  // that cookie's absence.
  function storesAValue(option) {
    return option.dataset.prefStores !== undefined;
  }

  // Each choice lands in the cookie its own name spells, which is the shape
  // the server's table is held to.
  function storedValue(name) {
    return readCookie(`yomihon_${name}`);
  }

  function applyOption(name, option) {
    applyChoice[name](option.value, storesAValue(option));
  }

  for (const fieldset of settingsFields()) {
    const current = fieldset.querySelector('input[type="radio"]:checked');
    if (current) standing.set(fieldset.dataset.prefField, current);
  }

  settingsForm?.addEventListener('change', (event) => {
    const option = event.target;
    if (!(option instanceof HTMLInputElement) || option.type !== 'radio') return;
    const fieldset = option.closest('[data-pref-field]');
    if (!fieldset) return;
    const name = fieldset.dataset.prefField;
    const status = fieldset.querySelector('[data-pref-status]');
    if (!applyChoice[name]) {
      // Asking the server, because only the server can rewrite the words it
      // wrote — and asking in a way that comes back to this page, since a
      // reader who is still choosing has more to set. Nothing they have picked
      // so far is waiting on this, so nothing is lost by leaving.
      const next = settingsForm.querySelector('input[name="next"]');
      if (next) next.value = settingsForm.dataset.prefsSettings;
      settingsForm.requestSubmit();
      return;
    }
    applyOption(name, option);
    // A browser that refuses this address its storage takes the write in
    // silence, leaving the page showing a choice nothing kept. So the write is
    // read back before it is believed, and a choice that did not survive is
    // put back and said out loud rather than left looking chosen.
    const kept = storedValue(name);
    if (storesAValue(option) ? kept !== option.value : kept !== null) {
      const previous = standing.get(name);
      if (previous) {
        applyOption(name, previous);
        previous.checked = true;
      }
      if (status) status.textContent = status.dataset.prefRefused;
      return;
    }
    standing.set(name, option);
    // A sentence about a choice that could not be kept is answered by the same
    // choice being kept, and has to go when it is.
    if (status) status.textContent = '';
  });

  // A revived page carries the radios as the reader left them, while the
  // cookies may have moved on — a size chosen from the control above on a later
  // page arrives back here with the old one still marked. The options
  // themselves carry which values may be stored and which of them answers for
  // storing none, so the cookie is read through the list the server rendered
  // rather than through a second copy of its rules.
  function syncSettingsChoices() {
    for (const fieldset of settingsFields()) {
      const name = fieldset.dataset.prefField;
      if (!applyChoice[name]) continue;
      const kept = storedValue(name);
      const options = optionsOf(fieldset);
      const match =
        options.find((option) => storesAValue(option) && option.value === kept) ||
        options.find((option) => option.dataset.prefUnset !== undefined);
      if (!match) continue;
      match.checked = true;
      standing.set(name, match);
    }
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
    // On the page where these choices are set, the radios say which one is in
    // force, so they are as much a stale claim as the attributes above.
    syncSettingsChoices();
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
