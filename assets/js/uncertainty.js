// A mark records a location, never the sentence choices currently on a card.
export function initUncertainty() {
  const articles = [...document.querySelectorAll('[data-uncertainty-endpoint]:not([data-uncertainty-remove])')];
  const removals = [...document.querySelectorAll('[data-uncertainty-remove]')];
  if (articles.length === 0 && removals.length === 0) return;
  const endpoint = (articles[0] || removals[0]).dataset.uncertaintyEndpoint;
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

  async function toggle(path, anchor) {
    const response = await fetch(endpoint, {
      method: 'POST',
      body: new URLSearchParams({ path, anchor }),
      headers: { Accept: 'application/json' },
    });
    if (!response.ok) throw new Error('mark was not stored');
    const result = await response.json();
    if (typeof result.marked !== 'boolean') throw new Error('invalid mark response');
    return result;
  }

  for (const button of removals) {
    const { uncertaintyPath: path, uncertaintyAnchor: anchor } = button.dataset;
    const key = keyOf(path, anchor);
    const said = button.parentElement.querySelector('.y-uncertainty__said');
    controls.push({ key, button });
    ready.then(() => {
      button.disabled = !available || !keys.has(key);
      if (!available) said.textContent = button.dataset.uncertaintyUnavailable;
      else if (!keys.has(key)) location.reload();
    });
    button.addEventListener('click', async () => {
      button.disabled = true;
      try {
        const result = await toggle(path, anchor);
        if (result.marked) keys.add(key);
        else keys.delete(key);
        if (!result.marked) {
          said.textContent = button.dataset.uncertaintyCleared;
          location.reload();
          return;
        }
        said.textContent = button.dataset.uncertaintyFailed;
      } catch {
        said.textContent = button.dataset.uncertaintyFailed;
      }
      button.disabled = !keys.has(key);
    });
  }

  function addControl(container, article, path, anchor, section = null) {
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
    const group = section ? container : document.createElement('div');
    const button = document.createElement('button');
    button.type = 'button';
    button.lang = words.uncertaintyLang;
    button.className = section ? 'y-iconbtn y-toc__uncertainty' : 'y-xbtn';
    button.disabled = true;
    button.setAttribute('aria-pressed', 'false');
    let said;
    if (section) {
      button.textContent = '?';
      button.setAttribute('aria-label', section.label);
      said = section.said;
      group.append(button);
    } else {
      group.dataset.uncertaintyControl = '';
      group.lang = words.uncertaintyLang;
      const scope = document.createElement('p');
      scope.className = 'y-fileinfo__note';
      scope.textContent = words.uncertaintyScope;
      said = document.createElement('p');
      said.className = 'y-uncertainty__said';
      said.setAttribute('role', 'status');
      group.append(button, scope, said);
      container.append(group);
    }
    const reflect = () => {
      const marked = keys.has(key);
      button.setAttribute('aria-pressed', String(marked));
      if (!section) button.textContent = marked ? words.uncertaintyClearLabel : words.uncertaintyLabel;
    };
    controls.push({ key, button, reflect });
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
        const result = await toggle(path, anchor);
        if (result.marked) keys.add(key);
        else keys.delete(key);
        said.textContent = result.marked ? words.uncertaintySaved : words.uncertaintyCleared;
      } catch {
        said.textContent = words.uncertaintyFailed;
      } finally {
        for (const control of controls) {
          if (control.key !== key) continue;
          control.button.disabled = false;
          control.reflect?.();
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

  for (const article of articles) {
    const column = article.closest('[data-note-column]') || document;
    for (const list of column.querySelectorAll('.y-toc__list')) {
      const rows = [...list.querySelectorAll('.y-toc__row')];
      let said;
      for (const row of rows) {
        const link = row.querySelector('a:first-child');
        if (!link || !link.getAttribute('href')?.startsWith('#')) continue;
        let id;
        try { id = decodeURIComponent(link.getAttribute('href').slice(1)); } catch { continue; }
        const target = document.getElementById(id);
        if (!target?.hasAttribute('data-mark-anchor') || !article.contains(target)) continue;
        if (!said) {
          const sectionSaid = document.createElement('p');
          sectionSaid.className = 'y-uncertainty__said';
          sectionSaid.setAttribute('role', 'status');
          sectionSaid.lang = article.dataset.uncertaintyLang;
          said = sectionSaid;
          list.append(said);
        }
        const prefix = article.dataset.uncertaintyPrefix;
        const anchor = prefix && id.startsWith(prefix) ? id.slice(prefix.length) : id;
        const label = article.dataset.uncertaintySectionFmt.replace('{section}', link.textContent);
        addControl(row, article, article.dataset.uncertaintyPath, anchor, { label, said });
      }
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
