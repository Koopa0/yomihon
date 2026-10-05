// A mark records a location, never the sentence choices currently on a card.
export function initUncertainty() {
  const articles = [...document.querySelectorAll('[data-uncertainty-endpoint]')];
  if (articles.length === 0) return;
  const endpoint = articles[0].dataset.uncertaintyEndpoint;
  const keys = new Set();
  let controls = [];
  let available = false;
  const keyOf = (path, anchor) => JSON.stringify([path, anchor]);
  const ready = fetch(endpoint, { cache: 'no-store', headers: { Accept: 'application/json' } })
    .then(async (response) => {
      if (!response.ok) throw new Error('marks unavailable');
      const marks = await response.json();
      for (const mark of marks) keys.add(keyOf(mark.path, mark.anchor));
      available = true;
    })
    .catch(() => {});

  function addControl(container, article, path, anchor) {
    const words = {
      uncertaintyLang: article.dataset.uncertaintyLang,
      uncertaintyLabel: article.dataset.uncertaintyLabel,
      uncertaintyClearLabel: article.dataset.uncertaintyClearLabel,
      uncertaintyScope: article.dataset.uncertaintyScope,
      uncertaintyUnavailable: article.dataset.uncertaintyUnavailable,
      uncertaintySaved: article.dataset.uncertaintySaved,
      uncertaintyCleared: article.dataset.uncertaintyCleared,
      uncertaintyFailed: article.dataset.uncertaintyFailed,
    };
    const key = keyOf(path, anchor);
    const group = document.createElement('div');
    group.dataset.uncertaintyControl = '';
    group.lang = words.uncertaintyLang;
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'y-xbtn';
    button.textContent = words.uncertaintyLabel;
    button.disabled = true;
    button.setAttribute('aria-pressed', 'false');
    const scope = document.createElement('p');
    scope.className = 'y-fileinfo__note';
    scope.textContent = words.uncertaintyScope;
    const said = document.createElement('p');
    said.className = 'y-uncertainty__said';
    said.setAttribute('role', 'status');
    group.append(button, scope, said);
    container.append(group);
    controls.push({ key, button });
    const reflect = () => {
      const marked = keys.has(key);
      button.setAttribute('aria-pressed', String(marked));
      button.textContent = marked ? words.uncertaintyClearLabel : words.uncertaintyLabel;
    };
    ready.then(() => {
      button.disabled = !available;
      reflect();
      if (!available) said.textContent = words.uncertaintyUnavailable;
    });
    button.addEventListener('click', async () => {
      controls = controls.filter((control) => control.button.isConnected);
      for (const control of controls) {
        if (control.key === key) control.button.disabled = true;
      }
      try {
        const response = await fetch(endpoint, {
          method: 'POST',
          body: new URLSearchParams({ path, anchor }),
          headers: { Accept: 'application/json' },
        });
        if (!response.ok) throw new Error('mark was not stored');
        const result = await response.json();
        if (result.marked) keys.add(key);
        else keys.delete(key);
        said.textContent = result.marked ? words.uncertaintySaved : words.uncertaintyCleared;
      } catch {
        said.textContent = words.uncertaintyFailed;
      } finally {
        for (const control of controls) {
          if (control.key !== key) continue;
          control.button.disabled = false;
          const marked = keys.has(key);
          control.button.setAttribute('aria-pressed', String(marked));
          control.button.textContent = marked ? words.uncertaintyClearLabel : words.uncertaintyLabel;
        }
      }
    });
  }

  for (const article of articles) {
    for (const card of article.querySelectorAll('.y-slotcard')) {
      const heading = card.querySelector('.y-slotcard__abstract[id]');
      if (!heading) continue;
      const prefix = article.dataset.uncertaintyPrefix;
      const anchor = prefix && heading.id.startsWith(prefix) ? heading.id.slice(prefix.length) : heading.id;
      addControl(card, article, article.dataset.uncertaintyPath, anchor);
    }
  }

  // lesson.js has already opened and filled this native dialog from the link.
  document.addEventListener('click', (event) => {
    const trigger = event.target.closest('[data-concept]');
    const article = trigger?.closest('[data-uncertainty-endpoint]');
    const dialog = document.querySelector('[data-concept-sheet]');
    const body = dialog?.querySelector('[data-concept-body]');
    if (!article || !dialog?.open || !body) return;
    dialog.querySelector('[data-uncertainty-control]')?.remove();
    const address = new URL(trigger.href, location.href);
    if (address.origin !== location.origin || !address.pathname.startsWith('/notes/')) return;
    try {
      addControl(body, article, decodeURIComponent(address.pathname.slice('/notes/'.length)), decodeURIComponent(address.hash.slice(1)));
    } catch {
      // A malformed address remains a link, never a guessed mark location.
    }
  });
}
