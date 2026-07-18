// Narrow-screen navigation drawer. The no-JS rail remains ordinary stacked
// content; this enhancement owns inertness, focus containment, and restoration.
export function initDrawer() {
  const root = document.documentElement;
  const media = window.matchMedia('(max-width: 900px)');
  const rail = document.querySelector('#nav-rail');
  const toggleButton = document.querySelector('[data-nav-toggle]');

  function isOpen() {
    return media.matches && root.dataset.nav === 'open';
  }

  function focusableElements() {
    if (!rail) return [];
    return [...rail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [contenteditable], [tabindex]:not([tabindex="-1"])')]
      .filter((element) => element.tabIndex >= 0 && !element.hidden && element.getClientRects().length > 0);
  }

  function setOpen(open) {
    root.dataset.nav = open ? 'open' : 'closed';
    toggleButton?.setAttribute('aria-expanded', String(open));
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
