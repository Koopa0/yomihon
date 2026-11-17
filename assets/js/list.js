// The list's filter and keys. The page answers the filter itself, so the form
// works with no script; this module asks for the list alone as the reader
// types and puts it in place. Behaviour only: nothing here decides what a list
// holds.
export function initList() {
  const delay = 180;
  document.querySelectorAll('[data-list]').forEach((list) => {
    const input = list.querySelector('[data-list-filter]');
    const endpoint = input?.getAttribute('data-list-fragment');
    const form = list.querySelector('[data-list-bar]');
    if (input && endpoint && form) filterAsYouType(list, form, input, endpoint, delay);
    arrowKeys(list, input);
  });
}

function filterAsYouType(list, form, input, endpoint, delay) {
  const base = new URL(endpoint, location.href);
  if (base.origin !== location.origin) return;
  let timer = null;
  let controller = null;
  let latest = 0;
  let composing = false;

  // The query the form would submit, so the address a reader copies is the one
  // that opens the same list.
  function queryFor() {
    const params = new URLSearchParams();
    for (const [name, value] of new FormData(form)) {
      if (typeof value === 'string' && value !== '') params.set(name, value);
    }
    return params;
  }

  function keepLinks(params) {
    list.querySelectorAll('a[data-list-link]').forEach((link) => {
      const url = new URL(link.href, location.href);
      if (params.has('filter')) url.searchParams.set('filter', params.get('filter'));
      else url.searchParams.delete('filter');
      link.href = url.pathname + url.search;
    });
  }

  async function run() {
    const params = queryFor();
    const page = new URL(base);
    page.search = params.toString();
    history.replaceState(null, '', page.pathname + page.search);
    keepLinks(params);
    params.set('fragment', '1');
    const url = new URL(base);
    url.search = params.toString();
    controller?.abort();
    controller = new AbortController();
    const mine = ++latest;
    try {
      const response = await fetch(url, { signal: controller.signal, headers: { Accept: 'text/html' } });
      if (!response.ok || mine !== latest) return;
      const html = await response.text();
      if (mine !== latest) return;
      const body = list.querySelector('[data-list-body]');
      const holder = document.createElement('template');
      holder.innerHTML = html;
      const next = holder.content.querySelector('[data-list-body]');
      if (body && next) body.replaceWith(next);
    } catch (error) {
      if (error.name !== 'AbortError') console.warn('[yomihon] list filter failed:', error);
    }
  }

  function later() {
    clearTimeout(timer);
    timer = setTimeout(run, delay);
  }

  input.addEventListener('compositionstart', () => { composing = true; });
  input.addEventListener('compositionend', () => { composing = false; later(); });
  input.addEventListener('input', () => { if (!composing) later(); });
  input.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && input.value !== '') {
      event.preventDefault();
      event.stopPropagation();
      input.value = '';
      clearTimeout(timer);
      run();
    }
  });
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    clearTimeout(timer);
    run();
  });
}

// Up and down move between rows, and Enter opens the one in focus: the rows are
// links, so Enter is the browser's own. From the filter, Down enters the rows.
function arrowKeys(list, input) {
  const rows = () => [...list.querySelectorAll('[data-list-row]')];
  list.addEventListener('keydown', (event) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    const all = rows();
    const at = all.indexOf(document.activeElement);
    if (at < 0 && document.activeElement !== input) return;
    if (at < 0 && event.key === 'ArrowUp') return;
    event.preventDefault();
    const next = event.key === 'ArrowDown' ? all[at + 1] : all[at - 1];
    if (next) next.focus();
    else if (at === 0 && input) input.focus();
  });
}
