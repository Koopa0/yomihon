// Narrow-screen navigation drawer. The no-JS rail remains ordinary stacked
// content; this enhancement owns inertness, focus containment, and restoration.
export function initDrawer() {
  const root = document.documentElement;
  const media = window.matchMedia('(max-width: 900px)');
  const rail = document.querySelector('#nav-rail');
  const toggleButton = document.querySelector('[data-nav-toggle]');
  // What leaves the accessibility tree while the drawer acts as a modal. The
  // header is deliberately absent: the button that opened the drawer sits above
  // the scrim and is the visible way back out, so it has to stay reachable. The
  // skip link is here for the opposite reason — it also paints above the scrim,
  // but its destination is in this list, so an open drawer would leave it
  // offering a jump to somewhere nobody can go.
  const background = [
    document.querySelector('#main-content'),
    document.querySelector('.y-skiplink'),
  ].filter(Boolean);

  function isOpen() {
    return media.matches && root.dataset.nav === 'open';
  }

  function focusableElements() {
    if (!rail) return [];
    return [...rail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [contenteditable], [tabindex]:not([tabindex="-1"])')]
      .filter((element) => element.tabIndex >= 0 && !element.hidden && element.getClientRects().length > 0);
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

  function containFocus(event) {
    if (!isOpen() || event.key !== 'Tab') return;
    if (document.querySelector('dialog[open]')) return;
    const focusable = focusableElements();
    if (focusable.length === 0) {
      event.preventDefault();
      rail?.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable.at(-1);
    const active = document.activeElement;
    if (event.shiftKey && (active === first || !rail.contains(active))) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && (active === last || !rail.contains(active))) {
      event.preventDefault();
      first.focus();
    }
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
