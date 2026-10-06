import { initRailFilter } from './rail-filter.js';

export function initSidebar() {
  // Sidebar disclosure state and filtering. Manual disclosure choices persist for
  // the session, and so does the filter: a reader narrowing a folder of dated
  // entries to one month and then opening a day found the box emptied and the
  // whole folder back, and retyped the same seven characters once per entry they
  // read. The narrowing is where they are, not what they just did.
  const rail = document.querySelector('.y-rail-left');
  const input = rail?.querySelector('[data-nav-filter]');
  if (!input) {
    return { canFocusFilter: () => false, focusFilter() {} };
  }
  if (rail.sidebarController) return rail.sidebarController;

  const storageKey = 'yomihon.nav';
  const filterKey = 'yomihon.nav.filter';
  const state = initRailFilter(rail, input);
  state.restore();

  rail.addEventListener('toggle', (event) => {
    const details = event.target;
    if (state.filtering || !details.dataset?.key || details.hasAttribute('data-chain')) return;
    const stored = state.readDisclosureState();
    stored[details.dataset.key] = details.open;
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(stored));
    } catch {
      // A refused write leaves the native disclosure usable on this page.
    }
  }, true);

  function rememberFilter() {
    try {
      if (input.value) sessionStorage.setItem(filterKey, input.value);
      else sessionStorage.removeItem(filterKey);
    } catch {
      // A refused write costs the narrowing on the next page, not this one.
    }
  }

  input.addEventListener('input', () => {
    state.applyFilter();
    rememberFilter();
  });
  input.addEventListener('keydown', (event) => {
    // Both keys below are an input method's own while it is composing: Enter
    // commits the word being formed and Escape abandons it. Reading them here
    // first opened the top row on a word the reader was still choosing,
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
      state.applyFilter();
      rememberFilter();
      input.blur();
    }
  });

  rail.sidebarController = {
    canFocusFilter: () => !input.hidden,
    focusFilter: () => input.focus(),
  };
  return rail.sidebarController;
}
