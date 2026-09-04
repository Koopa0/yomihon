// Sidebar disclosure state and filtering. Manual disclosure choices persist for
// the session, and so does the filter: a reader narrowing a folder of dated
// entries to one month and then opening a day found the box emptied and the
// whole folder back, and retyped the same seven characters once per entry they
// read. The narrowing is where they are, not what they just did.
export function initSidebar() {
  const rail = document.querySelector('.y-rail-left');
  const input = rail?.querySelector('[data-nav-filter]');
  if (!input) {
    return { canFocusFilter: () => false, focusFilter() {} };
  }

  const storageKey = 'yomihon.nav';
  const filterKey = 'yomihon.nav.filter';
  let filtering = false;
  const serverOpen = new Map();

  function readDisclosureState() {
    try {
      return JSON.parse(sessionStorage.getItem(storageKey) || '{}') || {};
    } catch {
      return {};
    }
  }

  rail.querySelectorAll('details[data-key]').forEach((details) => {
    serverOpen.set(details, details.open);
  });
  rail.addEventListener('toggle', (event) => {
    const details = event.target;
    if (filtering || !details.dataset?.key || details.hasAttribute('data-chain')) return;
    const stored = readDisclosureState();
    stored[details.dataset.key] = details.open;
    sessionStorage.setItem(storageKey, JSON.stringify(stored));
  }, true);

  function restingState(details) {
    if (details.hasAttribute('data-chain')) return true;
    const stored = readDisclosureState()[details.dataset.key];
    return typeof stored === 'boolean' ? stored : serverOpen.get(details);
  }

  const empty = rail.querySelector('[data-filter-empty]');
  // The rail carries a neighbourhood of each folder, not the folder. Filtering
  // is a pass over what was rendered, so in a folder whose rows were trimmed
  // the box searches less than the reader believes — a diarist narrowing three
  // years to one month would be shown thirteen of that month's twenty-one
  // days, with nothing on screen saying so. A quiet wrong answer is worse than
  // the long list this trimming replaced, so the box says what it could not
  // reach and hands over the search that can.
  const partial = rail.querySelector('[data-filter-partial]');
  const trimmedFolders = [...rail.querySelectorAll('[data-rail-trimmed]')];
  const groups = [...rail.querySelectorAll('details, .y-here')];
  const rows = [...rail.querySelectorAll('a, span.ui-navitem')];

  function applyFilter() {
    const query = input.value.trim().toLowerCase();
    filtering = query !== '';
    if (empty) empty.hidden = true;
    if (partial) partial.hidden = true;
    if (!query) {
      rows.forEach((row) => { row.hidden = false; });
      groups.forEach((group) => {
        group.hidden = false;
        if (group.tagName === 'DETAILS') group.open = restingState(group);
      });
      return;
    }
    rows.forEach((row) => { row.hidden = !row.textContent.toLowerCase().includes(query); });
    groups.forEach((group) => {
      const hit = [...group.querySelectorAll('a, span.ui-navitem')].some((row) => !row.hidden);
      group.hidden = !hit;
      if (group.tagName === 'DETAILS') group.open = hit;
    });
    if (empty) empty.hidden = rows.some((row) => !row.hidden);
    announceReach(query);
  }

  // announceReach names what the filter could not see, and links the search
  // that covers the whole folder. `folder:` is an existing search filter, so
  // this is an exit that already worked and nothing had pointed at.
  function announceReach(query) {
    if (!partial) return;
    const unreached = trimmedFolders.reduce((sum, more) => sum + (Number(more.dataset.railTrimmed) || 0), 0);
    if (!query || unreached === 0) {
      partial.hidden = true;
      return;
    }
    const dir = trimmedFolders.length === 1 ? trimmedFolders[0].dataset.railDir : '';
    // The folder is quoted because a vault path may hold a space, and the
    // search grammar ends a bare filter value at the first one: an unquoted
    // `folder:Module 2` asks for a folder called "Module", finds nothing, and
    // hands the reader the empty answer this sentence exists to prevent.
    const search = `/search?q=${encodeURIComponent(dir ? `folder:"${dir}" ${query}` : query)}`;
    partial.replaceChildren();
    // The sentence and the link's words come from the page, in the language
    // its reader asked for. Two forms of the sentence rather than a number
    // spliced into one, because the plural does not sit in the same place in
    // both languages.
    const template = unreached === 1 ? partial.dataset.filterPartialOne : partial.dataset.filterPartialMany;
    partial.append((template ?? '').replace('{count}', String(unreached)));
    const link = document.createElement('a');
    link.href = search;
    link.textContent = partial.dataset.filterSearchall ?? '';
    partial.append(' ', link);
    partial.hidden = false;
  }

  function rememberFilter() {
    try {
      if (input.value) sessionStorage.setItem(filterKey, input.value);
      else sessionStorage.removeItem(filterKey);
    } catch {
      // A refused write costs the narrowing on the next page, not this one.
    }
  }

  // Restore before the first paint of this rail so the reader never sees the
  // unfiltered list flash past on the way to where they were.
  try {
    const remembered = sessionStorage.getItem(filterKey);
    if (remembered) {
      input.value = remembered;
      applyFilter();
    }
  } catch {
    // No stored narrowing is the same as never having narrowed.
  }

  input.addEventListener('input', () => {
    applyFilter();
    rememberFilter();
  });
  input.addEventListener('keydown', (event) => {
    // Both keys below are an input method's own while it is composing: Enter
    // commits the word being formed and Escape abandons it. Reading them here
    // first opened the top row on a word the reader had not finished choosing,
    // and the word went with the page. The event says whether a composition is
    // running, so the flag the live-search box has to keep does not belong
    // here — that box debounces its own input events, which is a different
    // question from what a single key press means.
    if (event.isComposing) return;
    if (event.key === 'Enter') {
      event.preventDefault();
      rail.querySelector('a:not([hidden])')?.click();
    } else if (event.key === 'Escape') {
      // Escape belongs to whatever it can actually dismiss, innermost first.
      // A box that narrows nothing can dismiss nothing, so the key passes to
      // what is behind it — on a narrow window that is the drawer, whose own
      // exit key this is. Held here unconditionally, the first press changed
      // nothing a reader could see and the drawer took two.
      //
      // What counts as narrowing nothing is the same test the filtering above
      // makes, trimmed: a box holding only spaces hides no row, so answering
      // its Escape would cost that second press again for a box the reader
      // sees as empty. This branch clears nothing on its way out — emptying
      // the box would be answering a key it has just declined — though the
      // browser may still revert the field's own text, which is its to do.
      if (!input.value.trim()) return;
      event.preventDefault();
      event.stopPropagation();
      input.value = '';
      applyFilter();
      rememberFilter();
      input.blur();
    }
  });

  return {
    canFocusFilter: () => !input.hidden,
    focusFilter: () => input.focus(),
  };
}
