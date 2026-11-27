// Cross-feature keyboard policy. Feature modules own their mechanics; this
// module alone decides shortcut priority, typing exclusions, and modifiers.
export function initShortcuts({ drawer, rail, sidebar, search, marks }) {
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
      if (drawer.isOpen()) {
        drawer.closeAndRestoreFocus();
        return;
      }
      // A field closes itself on Escape; anything else that is open on a note
      // page is the reader's own marks.
      if (!typing && marks?.escape()) event.preventDefault();
      return;
    }
    // Taking back the last change to a mark. In a field the key is the text's own.
    if ((event.metaKey || event.ctrlKey) && !event.shiftKey && !event.altKey && (event.key === 'z' || event.key === 'Z')) {
      if (!typing && !document.querySelector('dialog[open]') && marks?.undo()) event.preventDefault();
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (root.dataset.singleKeyShortcuts === 'off') return;
    if (typing || document.querySelector('dialog[open]')) return;
    if (marks?.key(event)) {
      event.preventDefault();
      return;
    }
    // On a page with readings the reading is shown or hidden from the keyboard
    // too, and a question that has focus took the key before this.
    if (event.key === 'r' && !event.repeat) {
      const readings = document.querySelector('[data-ruby-toggle]');
      if (readings) {
        event.preventDefault();
        readings.click();
      }
      return;
    }
    if (event.key === '/') {
      // A page that is a list has its own field, and it is the nearer one.
      const listFilter = document.querySelector('[data-list-filter]');
      if (listFilter) {
        event.preventDefault();
        listFilter.focus();
        return;
      }
      if (sidebar.canFocusFilter()) {
        event.preventDefault();
        if (drawer.isNarrow() && !drawer.isOpen()) drawer.open();
        // A folded column holds a filter nobody can reach, and focus asked of
        // a hidden input lands nowhere. The column is opened first, as the
        // drawer is above.
        rail.expand();
        sidebar.focusFilter();
      }
      return;
    }
    if (event.key === '[') {
      if (drawer.isNarrow()) {
        event.preventDefault();
        drawer.toggle();
      } else if (rail.available()) {
        event.preventDefault();
        rail.toggle();
      }
      return;
    }
    if (event.key === '?') {
      const help = document.getElementById('_y-kbd-help');
      if (!help) return;
      event.preventDefault();
      if (help.matches(':popover-open')) {
        help.hidePopover();
        return;
      }
      // A key is not a press the markup can point at a surface, so the panel
      // that lists the keys is opened from here when its own key is pressed.
      help.showPopover();
    }
  });
}
