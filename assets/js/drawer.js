// Narrow-screen navigation drawer. The no-JS rail remains ordinary stacked
// content; this enhancement owns inertness, focus containment, and restoration.
export function initDrawer() {
  const root = document.documentElement;
  const media = window.matchMedia('(max-width: 900px)');
  const rail = document.querySelector('#nav-rail');
  const toggleButton = document.querySelector('[data-nav-toggle]');
  // What leaves the accessibility tree while the drawer acts as a modal. The
  // test is whether the region paints under the scrim: anything covered is
  // unreachable by sight and by pointer, so leaving it reachable by keyboard
  // and by screen reader is the mismatch. The header fails that test on
  // purpose — the button that opened the drawer sits above the scrim and is
  // the visible way back out, so it stays reachable. The skip link is here for
  // the opposite reason: it also paints above the scrim, but its destination is
  // in this list, so an open drawer would leave it offering a jump to somewhere
  // nobody can go. The status bar is under the scrim and is a control surface,
  // and it is a sibling of the shell rather than part of the article, so it is
  // named here rather than covered by the main region. The right rail needs no
  // entry: it is display:none at every width the drawer exists, which is
  // already out of the tree.
  const background = [
    document.querySelector('#main-content'),
    document.querySelector('.y-skiplink'),
    document.querySelector('.y-sealbar'),
  ].filter(Boolean);

  function isOpen() {
    return media.matches && root.dataset.nav === 'open';
  }

  function focusableElements() {
    if (!rail) return [];
    return [...rail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [contenteditable], [tabindex]:not([tabindex="-1"])')]
      .filter((element) => element.tabIndex >= 0 && !element.hidden && element.getClientRects().length > 0);
  }

  // What a Tab walks while the drawer is modal: the rail, and the button that
  // opened it. That button is named just above as the visible way back out,
  // and a cycle drawn around the rail alone withheld it from the one input
  // the cycle exists to serve — a reader on the keyboard could see the exit
  // and never arrive at it. It leads because it leads in the document, and
  // the rest of the header lies between it and the rail, which is why moving
  // between the two is decided here instead of left to the browser's own
  // order. Entering the drawer is a separate question with a separate answer:
  // opening it puts the reader in the rail, not on the button they just left.
  function focusCycle() {
    return [toggleButton, ...focusableElements()];
  }

  // An open drawer is a modal, so the page behind the scrim is not merely
  // dimmed: it leaves the accessibility tree, and a reading cursor can no
  // longer walk into an article it can no longer see. Inertness was treated as
  // the rail's own property — set on the rail while closed, never on anything
  // else while open — so the whole article stayed reachable behind a curtain
  // that only ever covered it visually.
  //
  // One expression decides it, and it is written before any early return: this
  // function leaves by three doors, and a door that skips the line is a page
  // left unusable after the drawer has closed.
  function setOpen(open) {
    const modal = open && media.matches && Boolean(rail);
    root.dataset.nav = open ? 'open' : 'closed';
    toggleButton?.setAttribute('aria-expanded', String(open));
    for (const region of background) region.inert = modal;
    if (!rail) return;
    if (!media.matches) {
      rail.inert = false;
      rail.removeAttribute('aria-hidden');
      return;
    }
    rail.inert = !open;
    if (open) rail.removeAttribute('aria-hidden');
    else rail.setAttribute('aria-hidden', 'true');
  }

  function focusFirst() {
    const target = focusableElements()[0] || rail;
    target?.focus();
  }

  function open() {
    if (!media.matches || !rail) return;
    setOpen(true);
    focusFirst();
  }

  function closeAndRestoreFocus() {
    setOpen(false);
    if (media.matches) toggleButton?.focus();
  }

  function toggle() {
    if (!media.matches) return;
    if (isOpen()) closeAndRestoreFocus();
    else open();
  }

  // One rule for every position: while the drawer is modal, Tab steps along
  // the cycle and stops nowhere else. The edges used to be forced and the
  // middle left to the browser, which worked only while the cycle was one
  // contiguous subtree. With the exit button in it and the rest of the header
  // sitting between that button and the rail, an unforced step from the
  // button would have walked into the chrome behind the scrim — so the step
  // itself is what this owns, and the wrap is just the step at the end.
  //
  // Focus the cycle does not name is the page behind the scrim, a header
  // control beside the exit, or the rail itself holding focus because it had
  // nothing focusable to offer: Tab enters at the near edge rather than
  // carrying on through. The exit is in the cycle, so there is always an edge
  // to enter by.
  //
  // The query builds a list of what looks focusable; only the browser knows
  // what is. A row inside a collapsed disclosure reports a box and still
  // refuses focus — two rail rows in three, here — and the browser skips such
  // a row on its own, which is why leaving the middle to it used to work. Now
  // that every step is taken here, a step onto a refused row would land
  // nowhere, and the press after it would recompute the same position and
  // retry the same row for as long as the reader kept pressing. So each
  // landing is checked against the document, and a refusal moves on in the
  // same direction. Asking who holds focus is the one test that cannot drift
  // from the browser's rule; a list of the shapes it refuses would be a
  // second guess at that rule, which is how this list went wrong already.
  function containFocus(event) {
    if (!isOpen() || event.key !== 'Tab') return;
    if (document.querySelector('dialog[open]')) return;
    const cycle = focusCycle();
    const step = event.shiftKey ? -1 : 1;
    const from = cycle.indexOf(document.activeElement);
    let index = from < 0
      ? (event.shiftKey ? cycle.length - 1 : 0)
      : (from + step + cycle.length) % cycle.length;
    event.preventDefault();
    for (let tried = 0; tried < cycle.length; tried += 1) {
      cycle[index].focus();
      if (document.activeElement === cycle[index]) return;
      index = (index + step + cycle.length) % cycle.length;
    }
    // Nothing in the cycle would take focus. Parking it on the rail keeps the
    // reader inside the drawer they can still see and still leave by Escape,
    // rather than stranded on whatever the page behind the scrim last held.
    rail.focus();
  }

  if (!rail || !toggleButton) {
    if (toggleButton) toggleButton.hidden = true;
    return { isNarrow: () => media.matches, isOpen, open, closeAndRestoreFocus, toggle };
  }

  rail.tabIndex = -1;
  setOpen(false);
  toggleButton.addEventListener('click', toggle);
  document.querySelector('[data-nav-close]')?.addEventListener('click', closeAndRestoreFocus);
  window.addEventListener('keydown', containFocus);
  media.addEventListener('change', () => {
    const restoreFocus = media.matches && rail.contains(document.activeElement);
    setOpen(false);
    if (restoreFocus) toggleButton.focus();
  });

  return { isNarrow: () => media.matches, isOpen, open, closeAndRestoreFocus, toggle };
}
