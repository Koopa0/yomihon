// Cross-feature keyboard policy. Feature modules own their mechanics; this
// module alone decides shortcut priority, typing exclusions, and modifiers.
export function initShortcuts({ drawer, sidebar, search }) {
  const root = document.documentElement;
  window.addEventListener('keydown', (event) => {
    // An input method mid-composition owns the keyboard, and the exclusion has
    // to be read before any branch rather than beside the ones that look like
    // typing. Escape is how a half-formed word is abandoned, and it was decided
    // above the exclusion for the target element, so abandoning a word closed
    // the search dialog or the drawer and left the word standing. The event
    // carries the state itself, which is why nothing here tracks composition
    // start and end.
    if (event.isComposing) return;
    const target = event.target;
    const typing = target && (
      target.tagName === 'INPUT'
      || target.tagName === 'TEXTAREA'
      || target.isContentEditable
      || (target.closest && target.closest('select'))
    );

    if ((event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {
      event.preventDefault();
      search.toggle();
      return;
    }
    if (event.key === 'Escape') {
      if (search.isOpen()) {
        event.preventDefault();
        search.closeAndRestoreFocus();
        return;
      }
      if (drawer.isOpen()) drawer.closeAndRestoreFocus();
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (root.dataset.singleKeyShortcuts === 'off') return;
    if (typing || search.isOpen()) return;
    if (event.key === '/') {
      if (sidebar.canFocusFilter()) {
        event.preventDefault();
        if (drawer.isNarrow() && !drawer.isOpen()) drawer.open();
        sidebar.focusFilter();
      }
      return;
    }
    if (event.key === '[') {
      if (drawer.isNarrow()) {
        event.preventDefault();
        drawer.toggle();
      }
    }
  });
}
