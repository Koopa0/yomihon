// Live search. Three surfaces share one mechanism: the search page, whose
// results are a list; a field's dropdown, whose panel opens under it; and the
// phone's full-screen surface, a dialog whose panel is always open. Each is an
// ordinary GET form that works without this module; the module only answers
// while the reader types.
import { fillRecent, recentSearches } from './recent.js';

export function initSearch() {
  const delay = 180;

  // What a surface has to be asked when it is shown again. A dialog keeps its
  // rows while it is closed, so this is registered per region and run from the
  // one place that opens one.
  const onReopen = new Map();
  // The fields that open a dropdown, for the ones that ask whether one is open
  // or want to open one.
  const fields = [];

  document.querySelectorAll('[data-live-search]').forEach((region) => {
    const form = region.querySelector('[data-live-search-form]');
    const input = region.querySelector('[data-live-search-input]');
    const panel = region.querySelector('[data-live-search-panel]');
    const target = panel ?? region.querySelector('[data-live-search-results]');
    const status = region.querySelector('[data-live-search-status]');
    const endpoint = region.getAttribute('data-live-search-endpoint');
    if (!form || !input || !target || !endpoint) return;

    const endpointURL = new URL(endpoint, location.href);
    if (endpointURL.origin !== location.origin) return;

    const isDialog = region.tagName === 'DIALOG';
    // The search page marks itself as the one region whose results the address
    // bar describes; the others float over pages whose address they do not
    // own. form.action is the no-JS truth, so the synced URL is exactly what
    // submitting the form would produce.
    const formURL = new URL(form.action, location.href);
    const syncsAddress =
      region.hasAttribute('data-live-search-sync-url') && formURL.origin === location.origin;

    let timer = null;
    let activeController = null;
    let latestRequest = 0;
    let composing = false;

    const rows = () => [...target.querySelectorAll('[data-live-search-row]')];
    const currentRow = () => target.querySelector('[data-current]');

    // The row Enter opens. The first is current while the reader is typing a
    // query; with the field empty nothing is, so Enter does nothing.
    function setCurrent(row) {
      for (const other of rows()) {
        if (other === row) other.setAttribute('data-current', '');
        else other.removeAttribute('data-current');
      }
      row?.scrollIntoView({ block: 'nearest' });
    }

    function cancelPending() {
      clearTimeout(timer);
      timer = null;
      latestRequest += 1;
      activeController?.abort();
      activeController = null;
      target.setAttribute('aria-busy', 'false');
    }

    // Written by the server hidden, shown when the reader has recent searches.
    function showRecent() {
      let any = 0;
      for (const section of target.querySelectorAll('[data-recent-searches]')) {
        any += fillRecent(section);
        section.querySelector('[data-recent-clear]')?.addEventListener('click', () => {
          recentSearches.clear();
          fillRecent(section);
          afterRender();
        });
      }
      return any;
    }

    function afterRender() {
      const typed = input.value.trim() !== '';
      const recent = showRecent();
      if (panel) {
        const first = rows()[0];
        setCurrent(typed ? first : null);
        // A dropdown under a field with nothing typed and nothing remembered
        // has nothing to say, so it is not drawn.
        if (!isDialog) panel.hidden = !typed && recent === 0;
      }
    }

    // The rows left on screen answer whichever query last reached the region.
    // Beside a sentence asking the reader to run the search again they would
    // read as an answer to what is in the box now, so the set is made to say
    // which query it does answer. The sentence travels with the rows, written
    // by the server that answered them, and stays hidden while they are still
    // the current answer. Both sides are trimmed, because the field drops the
    // padding an address can carry and the same search must not read as two.
    function markStaleNote(query) {
      const note = target.querySelector('[data-live-search-stale]');
      if (note) note.hidden = (note.dataset.liveSearchStale ?? '').trim() === query;
    }

    // What the page keeps of the reader's reading of an answer while they type:
    // the order, not the narrowing, which belonged to the words before.
    function pageParams(query) {
      const params = new URLSearchParams({ q: query });
      const sort = new URLSearchParams(location.search).get('sort');
      if (syncsAddress && sort && query) params.set('sort', sort);
      return params;
    }

    // ask fetches the fragment for these parameters and writes it into the
    // region. The request is the only one in flight: asking again abandons the
    // one before.
    async function ask(params, requestID) {
      const requestController = new AbortController();
      activeController = requestController;
      const url = new URL(endpointURL);
      for (const [name, value] of params) url.searchParams.set(name, value);
      if (panel) url.searchParams.set('shape', 'panel');
      const query = (params.get('q') ?? '').trim();

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
        if (panel) {
          panel.replaceChildren(imported);
        } else {
          target.replaceChildren(...imported.childNodes);
          target.dataset.resultCount = String(count);
        }
        afterRender();
        if (status) status.textContent = resultCount(status, query, count);
        if (syncsAddress) {
          // The results on screen just changed, so the address follows and a
          // copied URL reproduces them. Replaced, not pushed: typing is one
          // continuing search, not a trail of history entries.
          const address = new URL(formURL);
          for (const [name, value] of params) if (value) address.searchParams.set(name, value);
          history.replaceState(history.state, '', address);
        }
      } catch (error) {
        if (error.name === 'AbortError' || requestID !== latestRequest) return;
        if (status) status.textContent = status.dataset.liveSearchOffline ?? '';
        markStaleNote(query);
      } finally {
        if (requestID === latestRequest) {
          target.setAttribute('aria-busy', 'false');
          if (activeController === requestController) activeController = null;
        }
      }
    }

    function schedule(now = false) {
      clearTimeout(timer);
      const query = input.value.trim();
      const requestID = ++latestRequest;
      activeController?.abort();
      activeController = null;
      target.setAttribute('aria-busy', 'true');
      const run = () => {
        timer = null;
        ask(pageParams(query), requestID);
      };
      if (now) run();
      else timer = setTimeout(run, delay);
    }

    // A choice on the page — an order, a kind, a place, the way on — asks the
    // same endpoint for the same answer read another way, and the list is
    // rewritten in place so the reader keeps their scroll.
    function readAnother(href) {
      const params = new URL(href, location.href).searchParams;
      cancelPending();
      const requestID = ++latestRequest;
      target.setAttribute('aria-busy', 'true');
      ask(params, requestID);
    }

    if (!panel) {
      target.addEventListener('click', (event) => {
        const choice = event.target.closest?.('[data-search-choice]');
        if (!choice || event.metaKey || event.ctrlKey || event.shiftKey) return;
        event.preventDefault();
        readAnother(choice.getAttribute('href'));
      });
      target.addEventListener('change', (event) => {
        const select = event.target.closest?.('[data-search-select]');
        if (select) readAnother(select.value);
      });
    }

    // Following a result or a panel row is the reader having searched for what
    // is in the field; a recent search is remembered as itself.
    region.addEventListener('click', (event) => {
      const row = event.target.closest?.('[data-live-search-row], .y-found__item');
      if (!row || !target.contains(row)) return;
      recentSearches.add(row.dataset.recentQuery ?? input.value);
    });
    form.addEventListener('submit', () => {
      recentSearches.add(input.value);
      cancelPending();
    });

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

    if (panel) {
      const field = { region, input, panel, isDialog };
      if (!isDialog) fields.push(field);

      input.setAttribute('role', 'combobox');
      input.setAttribute('aria-autocomplete', 'list');
      if (!panel.id) panel.id = `_y-search-panel-${fields.length}-${isDialog ? 'd' : 'f'}`;
      input.setAttribute('aria-controls', panel.id);

      // Focus stays in the field, where the reader is typing. A press on the
      // panel is kept from taking it, so the row that was pressed is followed
      // rather than the panel being closed from under the press.
      panel.addEventListener('pointerdown', (event) => event.preventDefault());

      input.addEventListener('keydown', (event) => {
        // A key that is still part of composing a word belongs to the input method.
        if (event.isComposing) return;
        const found = rows();
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          if (found.length === 0) return;
          event.preventDefault();
          const at = found.indexOf(currentRow());
          const step = event.key === 'ArrowDown' ? 1 : -1;
          const start = step > 0 ? -1 : found.length;
          setCurrent(found[(((at < 0 ? start : at) + step) % found.length + found.length) % found.length]);
        } else if (event.key === 'Enter' && !event.shiftKey) {
          const row = currentRow();
          if (row) {
            event.preventDefault();
            row.click();
          } else if (input.value.trim() === '') {
            // An empty field has nothing to search for.
            event.preventDefault();
          }
        } else if (event.key === 'Enter' && event.shiftKey) {
          event.preventDefault();
          form.requestSubmit();
        }
      });

      if (!isDialog) {
        input.addEventListener('focus', () => schedule(true));
        region.addEventListener('focusout', (event) => {
          if (!region.contains(event.relatedTarget)) panel.hidden = true;
        });
        document.addEventListener('pointerdown', (event) => {
          if (!region.contains(event.target)) panel.hidden = true;
        });
      }
    }

    // A clear that empties the field and keeps the reader in it, on every
    // surface that draws one: the browser's own is blue, which is the reader's
    // marks' colour and nothing else's.
    const clear = region.querySelector('[data-live-search-clear]');
    if (clear) {
      const sync = () => {
        clear.hidden = input.value === '';
      };
      input.addEventListener('input', sync);
      clear.addEventListener('click', () => {
        input.value = '';
        sync();
        input.focus();
        schedule(true);
      });
      sync();
    }

    if (isDialog) {
      // The phone's surface is the dialog, with a cancel that closes it.
      region.querySelector('[data-search-close]')?.addEventListener('click', () => region.close());
      // Closing clears the attribute at once and announces itself afterwards,
      // so a reader who reopens quickly can have the announcement arrive after
      // the reopening. Cancelling then would throw away the search the reopen
      // had just asked for. The announcement is about a surface that is closed;
      // if it is open again by the time it lands, there is nothing to stop.
      region.addEventListener('close', () => {
        if (!region.open) cancelPending();
      });
      // Closing leaves the rows where they are, so the box and the rows can
      // disagree by the time the reader comes back; the rows are asked for
      // again, and until they land say which query they do answer.
      onReopen.set(region, () => {
        const query = input.value.trim();
        const note = target.querySelector('[data-live-search-stale]');
        if (query && (note?.dataset.liveSearchStale ?? '').trim() === query) return;
        if (query) markStaleNote(query);
        schedule(true);
      });
    }

    // The page's own list may arrive holding recent searches to fill, and a
    // dropdown's panel is empty until the field is focused.
    if (!panel) afterRender();
  });

  const dialog = document.querySelector('dialog[data-search]');

  // The header's press opens the dialog from the markup and the keyboard opens
  // it from here, so the question "what is it showing now" is asked of the
  // dialog itself rather than of any one way in. The browser announces every
  // opening the same way, whichever asked for it, so a way added later arrives
  // already answered.
  dialog?.addEventListener('toggle', (event) => {
    if (event.newState === 'open') {
      onReopen.get(dialog)?.();
      dialog.querySelector('[data-live-search-input]')?.focus();
    }
  });

  function open() {
    if (!dialog || dialog.open) return;
    // The keyboard has no markup to declare this with: a chord is not a press
    // on anything the page can name.
    dialog.showModal();
  }

  // An engine that has not shipped the markup form of the press yet. The press
  // is then done by hand, which is what this module did for every engine until
  // the platform took it over. Nothing else is stood in for.
  if (!('commandForElement' in HTMLButtonElement.prototype)) {
    document.querySelector('[data-search-open]')?.addEventListener('click', (event) => {
      if (!dialog) return;
      event.preventDefault();
      open();
    });
  }

  // The field drawn in the header, where there is one to type in.
  const visibleField = () => fields.find((field) => field.input.offsetParent !== null);
  const openField = () => fields.find((field) => !field.panel.hidden);

  // Where the reader was when the keyboard sent them to the field, so the
  // way out of it can put them back there rather than on the page's body.
  let cameFrom = null;
  // Focus that goes anywhere else first makes that place stale.
  document.addEventListener('focusin', (event) => {
    if (!fields.some((field) => field.region.contains(event.target))) cameFrom = null;
  });

  function leaveField(field) {
    field.panel.hidden = true;
    const back = cameFrom;
    cameFrom = null;
    if (back?.isConnected) back.focus();
    else field.input.blur();
  }

  function toggle() {
    const field = visibleField();
    if (field) {
      if (document.activeElement === field.input) leaveField(field);
      else {
        const here = document.activeElement;
        cameFrom = here && here !== document.body ? here : null;
        field.input.focus();
        field.input.select();
      }
      return;
    }
    if (!dialog) return;
    if (dialog.open) dialog.close();
    else open();
  }

  // Escape closes the list first and leaves the reader in the field, where
  // they were typing; a second Escape leaves the field for where they were
  // before it.
  function closeAndRestoreFocus() {
    const field = openField();
    if (field) {
      field.panel.hidden = true;
      return;
    }
    const focused = fields.find((candidate) => document.activeElement === candidate.input);
    if (focused) {
      leaveField(focused);
      return;
    }
    // The return is the platform's and is not overridden.
    if (dialog?.open) dialog.close();
  }

  return {
    isOpen: () =>
      Boolean(dialog?.open) ||
      Boolean(openField()) ||
      fields.some((field) => document.activeElement === field.input),
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
