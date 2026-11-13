import { lapse } from './mark.js';

// The day a mark was made, as the page prints it: the stored time's own date, or
// today's where the mark was made a moment ago.
const dayOf = (time) => time.slice(0, 10);
const today = () => {
  const now = new Date();
  const two = (n) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
};

// A mark records a location, never the sentence choices currently on a card.
export function initUncertainty() {
  const articles = [...document.querySelectorAll('[data-uncertainty-endpoint]:not([data-uncertainty-remove])')];
  const removals = [...document.querySelectorAll('[data-uncertainty-remove]')];
  if (articles.length === 0 && removals.length === 0) return;
  const endpoint = (articles[0] || removals[0]).dataset.uncertaintyEndpoint;
  const keys = new Map();
  let controls = [];
  let available = false;
  const keyOf = (path, anchor) => JSON.stringify([path, anchor]);
  const ready = fetch(endpoint, { cache: 'no-store', headers: { Accept: 'application/json' } })
    .then(async (response) => {
      if (!response.ok) throw new Error('marks unavailable');
      const marks = await response.json();
      for (const mark of marks) keys.set(keyOf(mark.path, mark.anchor), dayOf(mark.time));
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
        if (result.marked) keys.set(key, today());
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

  function addControl(container, article, path, anchor) {
    const words = {
      uncertaintyLang: article.dataset.uncertaintyLang,
      uncertaintyLabel: article.dataset.uncertaintyLabel,
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
      button.setAttribute('aria-pressed', String(keys.has(key)));
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
        const result = await toggle(path, anchor);
        if (result.marked) keys.set(key, today());
        else keys.delete(key);
        said.textContent = result.marked ? words.uncertaintySaved : words.uncertaintyCleared;
      } catch {
        said.textContent = words.uncertaintyFailed;
      } finally {
        for (const control of controls) {
          if (control.key !== key) continue;
          control.button.disabled = false;
          control.button.setAttribute('aria-pressed', String(keys.has(key)));
        }
      }
    });
  }

  // Beside every section the page already drew the button, with the mark and its
  // day where the reader left one. What is left to do is to make it work, and to
  // set what was drawn against what the file says now.
  function bindSection(article, button) {
    const anchor = button.dataset.sectionAnchor;
    const key = keyOf(article.dataset.uncertaintyPath, anchor);
    const margin = button.closest('[data-section-margin]');
    const said = margin.querySelector('[data-section-said]');
    const date = margin.querySelector('[data-section-date]');
    const reflect = () => {
      const day = keys.get(key);
      button.setAttribute('aria-pressed', String(day !== undefined));
      date.hidden = day === undefined;
      if (day !== undefined) date.dateTime = day;
    };
    const say = (words) => {
      said.textContent = words;
      lapse(said);
    };
    ready.then(() => {
      button.disabled = false;
      if (available) reflect();
      else button.setAttribute('aria-disabled', 'true');
    });
    button.addEventListener('click', async () => {
      if (button.getAttribute('aria-disabled') === 'true') {
        if (!available) say(article.dataset.uncertaintyUnavailable);
        return;
      }
      button.setAttribute('aria-disabled', 'true');
      try {
        const result = await toggle(article.dataset.uncertaintyPath, anchor);
        if (result.marked) keys.set(key, today());
        else keys.delete(key);
        say(result.marked ? article.dataset.uncertaintySaved : article.dataset.uncertaintyCleared);
      } catch {
        say(article.dataset.uncertaintyFailed);
      } finally {
        button.removeAttribute('aria-disabled');
        reflect();
      }
    });
  }

  for (const article of articles) {
    for (const button of article.querySelectorAll('[data-section-mark]')) bindSection(article, button);
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
