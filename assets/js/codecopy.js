// Code stays selectable without scripting. The controls are inert server
// prototypes until this initializer can give each readable block its own act.
export function initCodeCopy() {
  function enhance(container) {
    for (const block of container.querySelectorAll('pre')) {
      if (block.closest('[aria-hidden="true"]') || block.previousElementSibling?.matches('.y-codecopy')) continue;
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
      block.before(fragment);
    }
  }

  enhance(document);
  return enhance;
}
