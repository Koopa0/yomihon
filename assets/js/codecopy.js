// Code stays selectable without scripting. The controls are inert server
// prototypes until this initializer can give each readable block its own act.
// The server leaves a slot in each block's head, above the code, and the
// control takes its place, so the page keeps the same elements and the
// browser's record of where a reader left off still names the same one. A
// block with no control gives its slot up.
export function initCodeCopy() {
  // Where a block's control stands: its head, or ahead of a block the server
  // drew without one.
  const placeOf = (block) => (block.parentElement?.matches('.y-code') && block.parentElement.querySelector(':scope > .y-code__head')) || null;
  const enhanced = (block) => {
    const head = placeOf(block);
    return head ? head.querySelector(':scope > .y-codecopy') !== null : block.previousElementSibling?.matches('.y-codecopy');
  };
  function enhance(container) {
    for (const block of container.querySelectorAll('pre')) {
      if (block.closest('[aria-hidden="true"]') || enhanced(block)) continue;
      // An immediate prototype declares which reading container owns the act.
      // The dialog keeps that declaration outside the body it replaces.
      let prototype = null;
      for (let scope = block.parentElement; scope; scope = scope.parentElement) {
        prototype = scope.querySelector(':scope > template[data-codecopy-template]');
        if (prototype) break;
      }
      if (!prototype) continue;
      const fragment = prototype.content.cloneNode(true);
      const button = fragment.querySelector('[data-codecopy-button]');
      const reply = fragment.querySelector('[data-codecopy-reply]');
      if (!button || !reply) continue;
      const text = block.querySelector('code') || block;
      button.addEventListener('click', async () => {
        if (button.disabled) return;
        button.disabled = true;
        reply.textContent = '';
        try {
          if (typeof navigator.clipboard?.writeText !== 'function') throw new Error('clipboard unavailable');
          await navigator.clipboard.writeText(text.textContent);
          reply.textContent = button.dataset.codecopyCopied;
        } catch {
          const selection = getSelection();
          if (selection) {
            const range = document.createRange();
            range.selectNodeContents(text);
            selection.removeAllRanges();
            selection.addRange(range);
          }
          reply.textContent = button.dataset.codecopyFailed;
        } finally {
          button.disabled = false;
        }
      });
      const head = placeOf(block);
      const slot = head ? head.querySelector(':scope > .y-codecopy-slot') : block.previousElementSibling;
      if (slot?.matches('.y-codecopy-slot')) slot.replaceWith(fragment);
      else if (head) head.append(fragment);
      else block.before(fragment);
    }
    for (const slot of container.querySelectorAll('.y-codecopy-slot')) slot.remove();
  }

  enhance(document);
  return enhance;
}
