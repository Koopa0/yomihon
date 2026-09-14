// Live lexical results and the native search dialog. Every surface remains an
// ordinary GET form; this module only updates results while the reader types.
export function initSearch() {
  const delay = 180;

  // What a region has to be asked when it is shown again. A dialog keeps its
  // rows while it is closed, so this is registered per region and run from the
  // one place that opens one.
  const onReopen = new Map();

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

    // The rows left on screen answer whichever query last reached the region.
    // Beside a sentence asking the reader to run the search again they would
    // read as an answer to what is in the box now, so the set is made to say
    // which query it does answer. The sentence travels with the rows, written
    // by the server that answered them, and stays hidden while they are still
    // the current answer.
    //
    // Assigned, never merely revealed: a reader who types their way back to the
    // query these rows do answer has stopped looking at an earlier answer, and
    // being told otherwise is the same false claim in the other direction. Both
    // sides are trimmed, because the field drops the padding an address can
    // carry and the same search must not read as two.
    function markStaleNote(query) {
      const note = results.querySelector('[data-live-search-stale]');
      if (note) note.hidden = (note.dataset.liveSearchStale ?? '').trim() === query;
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
        markStaleNote(query);
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
        // A local clear is a successful empty answer: the address must
        // drop the query the results no longer show, or a reload brings
        // it back. The dialog never sets the marker, so a note under
        // the palette keeps its own URL.
        if (syncsAddress) {
          history.replaceState(history.state, '', new URL(formURL));
        }
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
    if (region.tagName === 'DIALOG') {
      // Closing clears the attribute at once and announces itself afterwards,
      // so a reader who reopens quickly can have the announcement arrive after
      // the reopening. Cancelling then would throw away the search the reopen
      // had just asked for, and the palette would sit on the old rows with
      // nothing on its way. The announcement is about a dialog that is closed;
      // if it is open again by the time it lands, there is nothing to stop.
      region.addEventListener('close', () => {
        if (!region.open) cancelPending();
      });
      // Closing leaves the rows where they are and stops whatever was on its
      // way to replace them. So the box and the rows can disagree by the time
      // the reader comes back: they typed something these rows never answered.
      // Reopening is the only moment that can be noticed, and nothing was
      // noticing it — the rows went on standing for a search the reader had
      // already moved off. Asking again is the same debounce a keystroke uses,
      // and until it lands the rows say which query they do answer.
      onReopen.set(region, () => {
        const query = input.value.trim();
        if (!query) return;
        const note = results.querySelector('[data-live-search-stale]');
        if ((note?.dataset.liveSearchStale ?? '').trim() === query) return;
        markStaleNote(query);
        schedule();
      });
    }
  });

  const dialog = document.querySelector('[data-search]');

  // Every way the dialog is shown comes through here, so a way added later
  // cannot arrive without the region being asked whether what it is showing
  // still belongs to what is in the box.
  function open() {
    if (!dialog || dialog.open) return;
    dialog.showModal();
    onReopen.get(dialog)?.();
  }

  document.querySelector('[data-search-open]')?.addEventListener('click', (event) => {
    if (!dialog) return;
    event.preventDefault();
    open();
  });

  function toggle() {
    if (!dialog) return;
    if (dialog.open) dialog.close();
    else open();
  }

  function closeAndRestoreFocus() {
    if (!dialog?.open) return;
    // The return is the platform's and is not overridden.
    dialog.close();
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
  // A replacer function rather than a string: a string replacement reads $&,
  // $$, $` and $' as instructions, so a reader searching for one of them was
  // told about a different search than the one they made. One pass rather than
  // two, so a query that contains the other placeholder is not read as one.
  return template.replace(/\{query\}|\{count\}/g, (mark) => (mark === '{query}' ? query : String(count)));
}
