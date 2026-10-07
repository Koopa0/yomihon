// The original rows and disclosure defaults are captured before settlement
// or filtering changes them. A deferred initializer keeps this same state,
// so a generated search link is never recaptured as an original row.
function initRailFilter(rail, input) {
  if (rail.railFilterState) return rail.railFilterState;
  const storageKey = 'yomihon.nav';
  const filterKey = 'yomihon.nav.filter';
  let filtering = false;
  let restored = false;
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
  function restingState(details) {
    if (details.hasAttribute('data-chain')) return true;
    const stored = readDisclosureState()[details.dataset.key];
    return typeof stored === 'boolean' ? stored : serverOpen.get(details);
  }

  const empty = rail.querySelector('[data-filter-empty]');
  // The rendered neighbourhood may be trimmed; say what the filter did not
  // reach and keep the whole-folder search available.
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
    // Quotes keep a space-bearing directory inside one search filter value.
    const search = `/search?q=${encodeURIComponent(dir ? `folder:"${dir}" ${query}` : query)}`;
    partial.replaceChildren();
    // The server owns both language-specific count forms and the link label.
    const template = unreached === 1 ? partial.dataset.filterPartialOne : partial.dataset.filterPartialMany;
    partial.append((template ?? '').replace('{count}', String(unreached)));
    const link = document.createElement('a');
    link.href = search;
    link.textContent = partial.dataset.filterSearchall ?? '';
    partial.append(' ', link);
    partial.hidden = false;
  }

  function restore() {
    if (restored) return;
    restored = true;
    try {
      const remembered = sessionStorage.getItem(filterKey);
      if (remembered) {
        rail.dataset.railRestoring = '';
        try {
          input.value = remembered;
          applyFilter();
          void rail.offsetWidth;
        } finally {
          delete rail.dataset.railRestoring;
        }
      }
    } catch {
      // No stored narrowing is the same as never having narrowed.
    }
  }

  const state = { applyFilter, restore, readDisclosureState, get filtering() { return filtering; } };
  rail.railFilterState = state;
  return state;
}

export { initRailFilter };
