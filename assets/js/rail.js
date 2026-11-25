// The left column standing beside the text, opened and folded. The state is the
// site's own cookie, kept by the preferences module through the same doors as
// every other reading choice; this module owns the act — what a press of the
// header's button, and the key, do to the column — and nothing about how the
// choice is kept.
//
// The column is a drawer below the width the drawer module names, and this
// module keeps to the other side of that line: it never touches what the
// drawer sets, and the drawer never touches what this sets. Crossing the line
// therefore leaves nothing stale on either side, because neither side wrote to
// the other's state. The button is the same on both sides; each module answers
// its press only on its own side.
//
// The column takes no room from the page, so opening or folding it moves
// nothing the reader is reading, and a folded column is hidden rather than
// removed, so it keeps its own scroll position: there is nothing here to hold
// still or put back.
import { drawerQuery } from './drawer.js';

export function initRail(preferences) {
  const root = document.documentElement;
  const rail = document.querySelector('#_y-nav-rail');
  const button = document.querySelector('[data-nav-toggle]');
  if (!rail || !button) {
    return { available: () => false, isCollapsed: () => false, toggle() {}, expand() {}, collapse() {} };
  }
  const media = drawerQuery(rail);

  function isCollapsed() {
    return root.dataset.rail === 'collapsed';
  }

  // The column can be opened or folded only where it stands beside the text.
  // Below that width the same press and the same key open the drawer, and a
  // fold written there would be a choice the reader never saw being made.
  function available() {
    return !media.matches;
  }

  function collapse() {
    if (!available() || isCollapsed()) return;
    preferences.writeRail('collapsed');
  }

  function expand() {
    if (!available() || !isCollapsed()) return;
    preferences.writeRail('open');
  }

  function toggle() {
    if (isCollapsed()) expand();
    else collapse();
  }

  button.addEventListener('click', () => {
    if (available()) toggle();
  });

  return { available, isCollapsed, toggle, expand, collapse };
}
