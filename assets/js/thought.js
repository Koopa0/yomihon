// The text stays selectable with scripting off or clipboard access unavailable.
export function initThought() {
  for (const stub of document.querySelectorAll('[data-thought-stub]')) {
    const button = stub.querySelector('[data-thought-copy]');
    const markdown = stub.querySelector('[data-thought-markdown]');
    const said = stub.querySelector('[data-thought-said]');
    if (!button || !markdown || !said) continue;
    button.hidden = false;
    button?.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(markdown.textContent);
        said.textContent = stub.dataset.thoughtCopied;
      } catch {
        const range = document.createRange();
        range.selectNodeContents(markdown);
        const selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
        said.textContent = stub.dataset.thoughtCopyfailed;
      }
    });
  }
}
