// Cross-feature keyboard policy. Feature modules own their mechanics; this
// module alone decides shortcut priority, typing exclusions, and modifiers.
export function initShortcuts({ drawer, sidebar, status, search }) {
  const root = document.documentElement;
  window.addEventListener('keydown', (event) => {
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
      status.cancelHold();
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
      return;
    }
    if ((event.key === 'r' || event.key === 'R') && !event.repeat && status.canStartShortcutHold()) {
      event.preventDefault();
      status.startShortcutHold();
    }
  });
  window.addEventListener('keyup', (event) => {
    if (event.key === 'r' || event.key === 'R') status.cancelHold();
  });
}
