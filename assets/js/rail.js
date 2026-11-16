// The left column folded away above the drawer width. The state is the site's
// own cookie, kept by the preferences module through the same doors as every
// other reading choice; this module owns the act — what a press, and the key,
// do to the page around the column — and nothing about how the choice is kept.
//
// The column is a drawer below the width the drawer module names, and this
// module keeps to the other side of that line: it never touches what the
// drawer sets, and the drawer never touches what this sets. Crossing the line
// therefore leaves nothing stale on either side, because neither side wrote to
// the other's state.
export function initRail(preferences) {
  const root = document.documentElement;
  const media = window.matchMedia('(max-width: 900px)');
  const rail = document.querySelector('#_y-nav-rail');
  const button = document.querySelector('[data-rail-toggle]');
  if (!rail || !button) {
    return { available: () => false, isCollapsed: () => false, toggle() {}, expand() {}, collapse() {} };
  }

  // Hiding the panel empties the rail's scroll box, and the browser clamps its
  // position to the top with nothing to say it was ever anywhere else. The
  // place is read before the panel goes and put back after it returns, on the
  // rail alone: asking the browser to bring a row into view would scroll the
  // document as well.
  let scrollTop = 0;

  // The column is moving from the press until its track has finished, and the
  // stylesheet holds the panel's width for that long. The end is the
  // transition's own, with a timer behind it for a page where none runs.
  let settle = 0;
  function startMoving() {
    root.dataset.railMoving = '';
    clearTimeout(settle);
    settle = setTimeout(() => delete root.dataset.railMoving, 400);
  }
  root.addEventListener('transitionend', (event) => {
    if (event.propertyName !== 'grid-template-columns') return;
    clearTimeout(settle);
    delete root.dataset.railMoving;
  });

  function isCollapsed() {
    return root.dataset.rail === 'collapsed';
  }

  // The column can be folded only where it is a column, and only by a control
  // that is drawn: below the drawer width the button is not on screen, and a
  // key that folded an invisible column would leave the reader with a strip and
  // no way to have asked for it.
  function available() {
    return !media.matches;
  }

  function collapse() {
    if (!available() || isCollapsed()) return;
    scrollTop = rail.scrollTop;
    startMoving();
    preferences.writeRail('collapsed');
  }

  function expand() {
    if (!available() || !isCollapsed()) return;
    startMoving();
    preferences.writeRail('open');
    rail.scrollTop = scrollTop;
  }

  function toggle() {
    if (isCollapsed()) expand();
    else collapse();
  }

  button.addEventListener('click', toggle);

  return { available, isCollapsed, toggle, expand, collapse };
}
