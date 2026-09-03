// Live lexical results and the native search dialog. Every surface remains an
// ordinary GET form; this module only updates results while the reader types.
export function initSearch() {
  const delay = 180;

  document.querySelectorAll('[data-live-search]').forEach((region) => {
    const form = region.querySelector('[data-live-search-form]');
    const input = region.querySelector('[data-live-search-input]');
    const results = region.querySelector('[data-live-search-results]');
    const status = region.querySelector('[data-live-search-status]');
    const endpoint = region.getAttribute('data-live-search-endpoint');
    if (!form || !input || !results || !status || !endpoint) return;

    const endpointURL = new URL(endpoint, location.href);
    if (endpointURL.origin !== location.origin) return;

    // The search page marks itself as the one region whose results the
    // address bar describes; the dialog never does, because it floats over
    // pages whose address it does not own. form.action is the no-JS truth,
    // so the synced URL is exactly what submitting the form would produce.
    const formURL = new URL(form.action, location.href);
    const syncsAddress =
      region.hasAttribute('data-live-search-sync-url') && formURL.origin === location.origin;

    let timer = null;
    let activeController = null;
    let latestRequest = 0;
    let composing = false;

    function cancelPending() {
      clearTimeout(timer);
      timer = null;
      latestRequest += 1;
      activeController?.abort();
      activeController = null;
      results.setAttribute('aria-busy', 'false');
    }

    function clearResults() {
      results.replaceChildren();
      results.dataset.resultCount = '0';
      status.textContent = '';
    }

    async function refresh(query, requestID) {
      const requestController = new AbortController();
      activeController = requestController;
      const url = new URL(endpointURL);
      url.searchParams.set('q', query);

      try {
        const response = await fetch(url, {
          headers: { Accept: 'text/html' },
          signal: requestController.signal,
        });
        if (!response.ok) throw new Error(`search response ${response.status}`);

        const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
        const fragment = doc.querySelector('[data-live-search-results]');
        const count = fragment && Number(fragment.dataset.resultCount);
        if (!fragment || !Number.isInteger(count) || count < 0) {
          throw new Error('search response has no valid result fragment');
        }
        if (requestID !== latestRequest) return;

        const imported = document.importNode(fragment, true);
        results.replaceChildren(...imported.childNodes);
        results.dataset.resultCount = String(count);
        status.textContent = resultCount(status, query, count);
        if (syncsAddress) {
          // The results on screen just changed, so the address follows and a
          // copied URL reproduces them. Replaced, not pushed: typing is one
          // continuing search, not a trail of history entries.
          const address = new URL(formURL);
          address.searchParams.set('q', query);
          history.replaceState(history.state, '', address);
        }
      } catch (error) {
        if (error.name === 'AbortError' || requestID !== latestRequest) return;
        status.textContent = status.dataset.liveSearchOffline ?? '';
      } finally {
        if (requestID === latestRequest) {
          results.setAttribute('aria-busy', 'false');
          if (activeController === requestController) activeController = null;
        }
      }
    }

    function schedule() {
      clearTimeout(timer);
      const query = input.value.trim();
      const requestID = ++latestRequest;
      activeController?.abort();
      activeController = null;
      if (!query) {
        timer = null;
        results.setAttribute('aria-busy', 'false');
        clearResults();
        return;
      }

      results.setAttribute('aria-busy', 'true');
      timer = setTimeout(() => {
        timer = null;
        refresh(query, requestID);
      }, delay);
    }

    input.addEventListener('compositionstart', () => {
      composing = true;
      cancelPending();
    });
    input.addEventListener('compositionend', () => {
      composing = false;
      schedule();
    });
    input.addEventListener('input', () => {
      if (!composing) schedule();
    });
    form.addEventListener('submit', cancelPending);
    if (region.tagName === 'DIALOG') region.addEventListener('close', cancelPending);
  });

  const dialog = document.querySelector('[data-search]');
  document.querySelector('[data-search-open]')?.addEventListener('click', (event) => {
    if (!dialog) return;
    event.preventDefault();
    if (!dialog.open) dialog.showModal();
  });

  function toggle() {
    if (!dialog) return;
    if (dialog.open) dialog.close();
    else dialog.showModal();
  }

  function closeAndRestoreFocus() {
    if (!dialog?.open) return;
    dialog.close();
    document.querySelector('[data-search-open]')?.focus();
  }

  return {
    isOpen: () => Boolean(dialog?.open),
    toggle,
    closeAndRestoreFocus,
  };
}

// resultCount fills in the sentence the page carries for this reader. Two
// forms rather than one with a number spliced in: the two languages put the
// plural in different places, and a sentence assembled from fragments here
// would be right in whichever one it was assembled for.
function resultCount(status, query, count) {
  const template = count === 1 ? status.dataset.liveSearchCountone : status.dataset.liveSearchCountmany;
  if (!template) return '';
  return template.replace('{query}', query).replace('{count}', String(count));
}
