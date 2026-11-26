// The reader's own marks on a note: a passage highlighted, a thought written
// beside it, a question left open.
//
// A mark is one thing in one of three states, and the page changes its state, its
// words and whether it is resolved by sending the change to the server and
// drawing what the server answers. The server also draws every mark when the page
// arrives, so the page that has just made a mark and the page that arrives with
// it are the same page; this file makes nothing the server did not draw first,
// except the words of a passage it was just handed.
//
// Nothing here keeps a mark in the browser: no storage, no copy to reconcile. A
// change is a request, and the answer is what the page shows. Two tabs changing
// one mark are two requests; the newer is shown.

import { capture, slotFor, spanOf, unwrap, wrap } from './marks-text.js';
import { initIllumination, initMarksLayout } from './marks-layer.js';
import { createStatus } from './marks-status.js';

// The most steps a tab can take back, and how long a reader may stop typing
// before what they wrote is sent.
const UNDO_STEPS = 40;
const TYPING_PAUSE = 300;
// A mark is a line in a file with a bound on its size, and a page that is about
// to send more than that says so rather than guess at the reason.
const TOO_LONG = 32 * 1024;

const fmt = (text, value) => text.replace('%s', () => value);
const newId = () => [...crypto.getRandomValues(new Uint8Array(16))].map((b) => b.toString(16).padStart(2, '0')).join('');

export function initMarks() {
  const article = document.querySelector('.y-article');
  const prose = article?.querySelector(':scope > .y-prose');
  if (!prose || document.querySelectorAll('.y-article').length !== 1) return null;
  const root = document.documentElement;
  const keeps = Boolean(article.dataset.marksEndpoint);
  const drawn = prose.querySelector('mark[data-mark], .y-mnote');
  if (!keeps && !drawn) return null;

  const layout = initMarksLayout(prose);
  initIllumination(prose);
  const marks = { key: (event) => navigationKey(event), escape: () => false, undo: () => false };
  const strings = JSON.parse(article.dataset.marksStrings ?? '{}');
  let status = null;

  // --- showing and hiding -------------------------------------------------

  const hidden = () => root.dataset.marks === 'hide';
  // A mark that is not drawn is not somewhere the keyboard can be left.
  function syncFocusable() {
    for (const piece of prose.querySelectorAll('mark.y-mark[tabindex]')) piece.tabIndex = hidden() ? -1 : 0;
  }
  function setShown(shown) {
    root.dataset.marks = shown ? 'show' : 'hide';
    syncFocusable();
    document.cookie = `yomihon_marks=${shown ? 'show' : 'hide'};path=/;max-age=31536000;samesite=lax`;
    layout.relayout();
  }

  // The marks in the order they stand in the text.
  function stops() {
    const found = [...prose.querySelectorAll('mark.y-mark[tabindex], .y-mnote[data-section], .y-mnote[data-changed]')];
    const top = (element) => element.getBoundingClientRect().top + scrollY;
    return found.map((element, index) => ({ element, y: top(element), index })).sort((a, b) => a.y - b.y || a.index - b.index).map((s) => s.element);
  }

  function navigationKey(event) {
    if (event.key === 'm') {
      setShown(hidden());
      return true;
    }
    if (event.key !== '{' && event.key !== '}') return false;
    if (hidden()) {
      status?.say(strings.hiddenShow, { action: { label: strings.show, run: () => setShown(true) } });
      return true;
    }
    const all = stops();
    if (all.length === 0) return true;
    const at = all.indexOf(document.activeElement.closest?.('mark.y-mark, .y-mnote') ?? null);
    const next = event.key === '}' ? all[(at + 1) % all.length] : all[(at - 1 + all.length) % all.length];
    next.scrollIntoView({ block: 'center' });
    next.focus({ preventScroll: true });
    return true;
  }

  syncFocusable();
  if (!keeps) return marks;

  // --- the page's own parts ------------------------------------------------

  const endpoint = article.dataset.marksEndpoint;
  const path = article.dataset.marksPath;
  const toolbar = article.querySelector('[data-marks-toolbar]');
  status = createStatus(article.querySelector('[data-marks-status]'));

  // --- what is known of each mark ------------------------------------------

  const known = new Map(); // id -> the mark as the server last answered it
  const bases = new Map(); // id -> what the page has read of it
  const spans = new Map(); // id -> where its words are, which does not change
  const queues = new Map(); // id -> the request in flight, so changes go one at a time

  const piecesOf = (id) => [...prose.querySelectorAll(`mark[data-mark="${CSS.escape(id)}"]`)];
  const noteOf = (id) => document.getElementById(`mn-${id}`);

  for (const first of prose.querySelectorAll('mark.y-mark[tabindex]')) {
    const span = spanOf(prose, piecesOf(first.dataset.mark));
    if (span) spans.set(first.dataset.mark, span);
  }

  // send is one request, and what the page needs to know of its answer.
  async function send(method, url, body, { keepalive = false } = {}) {
    const response = await fetch(url, {
      method,
      keepalive,
      cache: 'no-store',
      headers: { Accept: 'application/json', ...(body ? { 'Content-Type': 'application/json' } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    let json = null;
    try {
      json = await response.clone().json();
    } catch {
      // A refusal is plain words, which are not for the page to show.
    }
    return { status: response.status, json };
  }

  async function ensure(id) {
    if (known.has(id)) return known.get(id);
    const answer = await send('GET', `${endpoint}/${id}`);
    if (answer.status !== 200) return null;
    remember(answer.json);
    return answer.json;
  }

  function remember(mark) {
    mark.body ??= '';
    mark.quote ??= '';
    mark.anchor ??= '';
    known.set(mark.id, mark);
    bases.set(mark.id, mark.seen);
  }

  // change is one change to one mark, made on top of what the page last read of
  // it. A page that is told it has not read the newest takes the newest from the
  // answer and sends the change again once, quietly: both changes are kept and
  // the newer is what is shown.
  function change(method, id, patch, options = {}) {
    const { rebase = true, keepalive = false, immediate = false } = options;
    const run = async () => {
      let base = bases.get(id) ?? (await ensure(id))?.seen;
      if (!base) return { ok: false, gone: true };
      for (let attempt = 0; attempt < 2; attempt++) {
        const answer = await send(method, `${endpoint}/${id}`, { seen: base, ...patch }, { keepalive });
        if (answer.status === 200) {
          draw(answer.json);
          return { ok: true, mark: answer.json };
        }
        if (answer.status === 409) {
          const reason = answer.json?.reason;
          if (reason === 'stale' && !rebase) return { ok: false, stale: true };
          if (reason === 'stale' && attempt === 0) {
            base = answer.json.seen;
            continue;
          }
          if (reason === 'superseded') return { ok: true, silent: true };
          if (reason === 'gone') {
            known.delete(id);
            bases.delete(id);
            const now = await send('GET', `${endpoint}/${id}`);
            if (now.status === 200) draw(now.json);
            return { ok: false, gone: true };
          }
        }
        return { ok: false, status: answer.status };
      }
      return { ok: false, status: 409 };
    };
    // A send made as the page is leaving does not wait behind another: the
    // server judges which of the two is newer by their edit numbers.
    const before = queues.get(id) ?? Promise.resolve();
    const queued = immediate ? run() : before.then(run, run);
    queues.set(id, Promise.allSettled([before, queued]));
    return queued.catch(() => ({ ok: false, status: 0 }));
  }

  // --- drawing what the server answered ------------------------------------

  const stateOf = (mark) => (mark.kind === 'question' && mark.resolved && !mark.resolved.startsWith('0001') ? 'resolved' : mark.kind);

  function fragmentOf(html) {
    const template = document.createElement('template');
    template.innerHTML = html;
    return template.content.firstElementChild;
  }

  function sectionSlot(anchor) {
    const heading = anchor ? prose.querySelector(`[id="${CSS.escape(anchor)}"]`) : null;
    const sect = heading?.closest('.y-sect') ?? heading;
    if (!sect) return { parent: prose, before: prose.querySelector(':scope > .y-mlayer') };
    let at = sect;
    while (at.nextElementSibling?.matches('.y-mnote[data-section], .y-mfield[data-section]')) at = at.nextElementSibling;
    return { parent: at.parentNode, before: at.nextSibling };
  }

  function slotOf(mark) {
    const pieces = piecesOf(mark.id);
    if (pieces.length > 0) return slotFor(prose, pieces[pieces.length - 1]);
    return sectionSlot(mark.anchor);
  }

  // draw is a mark as the server answered it: its words in the text, its note in
  // the margin, or neither where it is deleted.
  function draw(mark) {
    remember(mark);
    const had = noteOf(mark.id);
    if (mark.deleted) {
      unwrap(piecesOf(mark.id));
      had?.remove();
      layout.relayout();
      return;
    }
    if (mark.quote && piecesOf(mark.id).length === 0 && spans.has(mark.id)) {
      const { start, end } = spans.get(mark.id);
      wrap(prose, start, end, { class: 'y-mark', 'data-mark': mark.id }, { tabindex: '0' });
    }
    const pieces = piecesOf(mark.id);
    for (const piece of pieces) {
      piece.dataset.kind = mark.kind;
      piece.dataset.state = stateOf(mark);
    }
    let note = null;
    if (mark.note) {
      note = fragmentOf(mark.note);
      if (had) {
        layout.swap(had, note);
      } else {
        const slot = slotOf(mark);
        slot.parent.insertBefore(note, slot.before);
      }
      addActions(note);
    } else {
      had?.remove();
    }
    const first = pieces[0];
    if (first) {
      first.setAttribute('tabindex', '0');
      first.setAttribute('aria-label', note?.getAttribute('aria-label') ?? fmt(strings.nameHighlight, opening(mark.quote)));
      if (note) first.setAttribute('aria-describedby', note.id);
      else first.removeAttribute('aria-describedby');
      for (const rest of pieces.slice(1)) {
        rest.removeAttribute('tabindex');
        rest.removeAttribute('aria-label');
      }
    }
    if (pieces.length > 0 && !spans.has(mark.id)) {
      const span = spanOf(prose, pieces);
      if (span) spans.set(mark.id, span);
    }
    layout.relayout();
  }

  function opening(text) {
    const squeezed = text.replace(/\s+/g, ' ').trim();
    const runes = [...squeezed];
    return runes.length <= 20 ? squeezed : `${runes.slice(0, 20).join('').trim()}…`;
  }

  // --- what a mark can be asked to do ------------------------------------

  function button(label, run, extra = {}) {
    const element = document.createElement('button');
    element.type = 'button';
    element.className = 'y-mnote__act';
    element.textContent = label;
    for (const [name, value] of Object.entries(extra)) element.dataset[name] = value;
    element.addEventListener('click', run);
    return element;
  }

  // addActions gives a note the actions its state has. They are drawn into the
  // note and shown while it has focus.
  function addActions(note) {
    const id = note.dataset.mnote;
    note.querySelector('.y-mnote__acts')?.remove();
    const acts = document.createElement('div');
    acts.className = 'y-mnote__acts';
    acts.lang = document.documentElement.lang;
    const kind = note.dataset.kind;
    const resolved = note.dataset.state === 'resolved';
    const hasWords = () => (known.get(id)?.body ?? '') !== '';
    acts.append(button(strings.edit, () => openField(id), { act: 'edit' }));
    if (kind === 'question') {
      acts.append(button(resolved ? strings.reopen : strings.resolve, () => resolve(id, !resolved), { act: 'resolve' }));
    } else {
      acts.append(button(strings.ask, () => turn(id, 'question'), { act: 'ask' }));
    }
    const more = document.createElement('details');
    more.className = 'y-mnote__more';
    const summary = document.createElement('summary');
    summary.textContent = strings.more;
    more.append(summary);
    const list = document.createElement('div');
    list.className = 'y-mnote__morelist';
    if (kind === 'question') {
      list.append(button(strings.toThought, () => turn(id, 'thought'), { act: 'thought' }));
      list.append(button(strings.toHighlight, () => turn(id, 'highlight'), { act: 'highlight' }));
    }
    list.append(button(strings.delete, () => remove(id), { act: 'delete' }));
    more.append(list);
    acts.append(more);
    // A question's words decide which way it can be turned.
    note.addEventListener('focusin', () => {
      const words = hasWords();
      const toThought = list.querySelector('[data-act="thought"]');
      const toHighlight = list.querySelector('[data-act="highlight"]');
      if (toThought) toThought.hidden = !words;
      if (toHighlight) toHighlight.hidden = words || !spans.has(id);
    });
    note.append(acts);
  }

  for (const note of prose.querySelectorAll('.y-mnote[data-mnote]')) addActions(note);

  // --- the line at the bottom and what can be taken back -------------------

  const undoable = [];
  function record(label, run) {
    const entry = { label, run };
    undoable.unshift(entry);
    undoable.length = Math.min(undoable.length, UNDO_STEPS);
    return entry;
  }

  const reasonOf = (_result, size = 0) => (size > TOO_LONG ? strings.reasonTooLong : strings.reasonNotWrite);

  async function undoEntry(entry) {
    const at = undoable.indexOf(entry);
    if (at >= 0) undoable.splice(at, 1);
    const result = await entry.run();
    if (result?.ok) {
      status.say(fmt(strings.undone, entry.label));
      return;
    }
    const cannot = result?.gone || result?.stale;
    status.say(cannot ? strings.cannotUndo : fmt(strings.notUndone, reasonOf(result)), { failed: !cannot });
  }

  async function undoLast() {
    const entry = undoable[0];
    if (!entry) {
      status.say(strings.nothingToUndo);
      return;
    }
    await undoEntry(entry);
  }

  function sayWithUndo(text, entry) {
    status.say(text, { action: { label: strings.undo, run: () => undoEntry(entry) } });
  }

  // --- making a mark -------------------------------------------------------

  async function keep(captured, kind, body = '') {
    const id = newId();
    const request = {
      id, kind, path, anchor: captured.anchor, quote: captured.quote,
      before: captured.before, after: captured.after, body,
    };
    let answer;
    try {
      answer = await send('POST', endpoint, request);
    } catch {
      // The answer never came, which does not say the mark was not kept. The id
      // is the page's own, so it is asked for, and sent again only if it is not
      // there; either way nothing is kept twice.
      try {
        answer = await send('GET', `${endpoint}/${id}`);
        if (answer.status !== 200) answer = await send('POST', endpoint, request);
      } catch {
        return { ok: false, status: 0 };
      }
    }
    if (answer.status !== 200 && answer.status !== 201) return { ok: false, status: answer.status };
    if (hidden()) setShown(true);
    spans.set(id, { start: captured.start, end: captured.end });
    draw(answer.json);
    return { ok: true, mark: answer.json };
  }

  // sameWords finds a mark that is on exactly the words that were chosen.
  function sameWords(captured) {
    for (const [id, span] of spans) {
      if (span.start === captured.start && span.end === captured.end && known.get(id)?.deleted !== true && piecesOf(id).length > 0) return id;
    }
    return null;
  }

  function clearSelection() {
    getSelection()?.removeAllRanges();
    hideToolbar();
  }

  async function highlight(captured) {
    const existing = sameWords(captured);
    if (existing) {
      clearSelection();
      focusMark(existing);
      status.say(strings.alreadyLit);
      return;
    }
    clearSelection();
    const result = await keep(captured, 'highlight');
    if (!result.ok) {
      failedToKeep(result, captured);
      return;
    }
    const id = result.mark.id;
    record(strings.whatHighlight, () => change('DELETE', id, {}, { rebase: false }));
    openField(id, { fresh: true });
  }

  async function ask(captured) {
    const existing = sameWords(captured);
    if (existing) {
      clearSelection();
      focusMark(existing);
      status.say(strings.alreadyAsked);
      return;
    }
    clearSelection();
    const result = await keep(captured, 'question');
    if (!result.ok) {
      failedToKeep(result, captured);
      return;
    }
    const id = result.mark.id;
    const entry = record(strings.whatQuestion, () => change('DELETE', id, {}, { rebase: false }));
    sayWithUndo(strings.askedUndo, entry);
  }

  function failedToKeep(result, captured) {
    if (result.status === 422 && !captured.silent) {
      status.say(strings.ambiguous, { failed: true });
      return;
    }
    status.say(fmt(strings.notKept, reasonOf(result)), { failed: true });
  }

  // --- changing one --------------------------------------------------------

  async function turn(id, kind) {
    const before = await ensure(id);
    if (!before) return;
    if (before.kind === kind) {
      if (kind === 'question') status.say(strings.alreadyAsked);
      return;
    }
    const patch = { kind };
    if (kind === 'highlight') patch.body = '';
    const result = await change('PATCH', id, patch);
    if (!result.ok) {
      if (!result.gone) status.say(fmt(strings.notKept, reasonOf(result)), { failed: true });
      return;
    }
    const back = { kind: before.kind, body: before.body };
    const entry = record(kind === 'highlight' ? strings.whatToHighlight : kind === 'thought' ? strings.whatThought : strings.whatQuestion, () => change('PATCH', id, back, { rebase: false }));
    const said = kind === 'question' ? strings.askedUndo : kind === 'thought' ? strings.nowThought : strings.nowHighlight;
    sayWithUndo(said, entry);
    focusMark(id);
  }

  async function resolve(id, resolved) {
    const before = await ensure(id);
    if (!before || before.kind !== 'question') return;
    const result = await change('PATCH', id, { resolved });
    if (!result.ok) {
      if (!result.gone) status.say(fmt(strings.notKept, reasonOf(result)), { failed: true });
      return;
    }
    record(resolved ? strings.whatResolve : strings.whatQuestion, () => change('PATCH', id, { resolved: !resolved }, { rebase: false }));
    focusMark(id);
  }

  // focusMark puts the keyboard on a mark: its note, which holds the actions, or
  // the words themselves where it has none.
  function focusMark(id) {
    const target = noteOf(id)?.closest('[hidden]') ? null : noteOf(id);
    (target ?? piecesOf(id)[0])?.focus({ preventScroll: false });
  }

  // --- taking one away -----------------------------------------------------

  // After a mark goes, the keyboard goes to the next one, or the one before, or
  // the paragraph it was in: never to the page.
  function focusAfter(order, id, carrier) {
    const ours = (element) => (element.dataset.mark ?? element.dataset.mnote) === id;
    const index = order.findIndex(ours);
    const around = order.filter((element) => !ours(element) && element.isConnected);
    const after = order.slice(index + 1).find((element) => !ours(element) && element.isConnected);
    const next = after ?? around[around.length - 1];
    if (next) {
      next.focus({ preventScroll: true });
      return;
    }
    if (carrier?.isConnected) {
      carrier.setAttribute('tabindex', '-1');
      carrier.focus({ preventScroll: true });
    }
  }

  async function remove(id) {
    const piece = piecesOf(id)[0] ?? noteOf(id);
    const carrier = piece?.closest('p, li, pre, blockquote, td, th, h1, h2, h3, h4, h5, h6, div');
    const order = stops();
    const result = await change('DELETE', id, {});
    if (!result.ok) {
      if (!result.gone) status.say(fmt(strings.notDeleted, reasonOf(result)), { failed: true });
      return;
    }
    focusAfter(order, id, carrier);
    const entry = record(strings.whatDelete, async () => {
      const back = await change('PUT', id, {}, { rebase: false });
      if (back.ok) focusMark(id);
      return back;
    });
    sayWithUndo(strings.deleted, entry);
  }

  // --- the field the words are written in ----------------------------------

  let field = null;

  function buildField(section) {
    const element = document.createElement('div');
    element.className = 'y-mfield';
    element.lang = document.documentElement.lang;
    if (section) element.dataset.section = '';
    const label = document.createElement('p');
    label.className = 'y-mfield__label';
    label.textContent = strings.editing;
    const text = document.createElement('textarea');
    text.className = 'y-mfield__text';
    text.rows = 3;
    text.setAttribute('aria-label', strings.words);
    text.spellcheck = false;
    const said = document.createElement('p');
    said.className = 'y-mfield__said';
    said.hidden = true;
    const bar = document.createElement('div');
    bar.className = 'y-mfield__bar';
    const done = document.createElement('button');
    done.type = 'button';
    done.className = 'y-mfield__done';
    done.append(strings.done, ' ');
    const kbd = document.createElement('kbd');
    kbd.className = 'ui-kbd';
    kbd.textContent = '⌘↵';
    done.append(kbd);
    const cancel = document.createElement('button');
    cancel.type = 'button';
    cancel.className = 'y-mfield__cancel';
    cancel.textContent = strings.cancel;
    bar.append(done, cancel);
    element.append(label, text, said, bar);
    return { element, text, said, done, cancel };
  }

  // claimed is the one field that is being opened, claimed at once so that two
  // presses before the first has its mark cannot make two fields.
  let claimed = false;
  async function openField(id, options = {}) {
    if (claimed) return;
    claimed = true;
    try {
      await openOne(id, options);
    } finally {
      claimed = false;
    }
  }

  async function openOne(id, { fresh = false, anchor = '' } = {}) {
    if (field) {
      await finish(field);
      if (field) return;
    }
    const mark = id ? await ensure(id) : null;
    if (id && !mark) return;
    const section = id ? !mark.quote : true;
    const parts = buildField(section);
    const session = {
      id,
      anchor: id ? mark.anchor : anchor,
      fresh,
      parts,
      edit: { id: newId(), seq: 0 },
      // A thought on a section is not made yet, and is made under this id each
      // time it is tried, so that a try whose answer was lost is not a second.
      draftId: id ? null : newId(),
      creating: null,
      createdEntry: null,
      before: mark ? { kind: mark.kind, body: mark.body } : null,
      sent: mark?.body ?? '',
      timer: 0,
      composing: false,
      failed: false,
      note: id ? noteOf(id) : null,
    };
    if (id) parts.element.dataset.mfield = id;
    parts.text.value = mark?.body ?? '';
    const slot = id ? slotOf(mark) : sectionSlot(anchor);
    if (session.note) session.note.hidden = true;
    slot.parent.insertBefore(parts.element, slot.before);
    field = session;
    wireField(session);
    layout.relayout();
    parts.text.focus();
  }

  function wireField(session) {
    const { text, said, done, cancel, element } = session.parts;
    const settle = () => {
      clearTimeout(session.timer);
      session.timer = setTimeout(() => flush(session), TYPING_PAUSE);
    };
    text.addEventListener('input', () => {
      if (!session.composing) settle();
    });
    text.addEventListener('compositionstart', () => { session.composing = true; });
    text.addEventListener('compositionend', () => {
      session.composing = false;
      settle();
    });
    text.addEventListener('blur', () => {
      if (!session.composing) flush(session, { keepalive: true });
    });
    text.addEventListener('keydown', (event) => {
      if (event.isComposing) return;
      if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
        event.preventDefault();
        if (event.repeat) return;
        finish(session);
        return;
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        finish(session);
      }
    });
    text.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && (event.metaKey || event.ctrlKey) && event.isComposing) {
        status.say(strings.composing);
      }
    });
    done.addEventListener('click', () => finish(session));
    cancel.addEventListener('click', () => abandon(session));
    // Leaving the field for anywhere else closes it, as long as what was written
    // was kept.
    element.addEventListener('focusout', (event) => {
      if (session.closing || element.contains(event.relatedTarget)) return;
      if (event.relatedTarget === null && document.visibilityState === 'hidden') return;
      finish(session);
    });
    said.addEventListener('click', (event) => {
      if (event.target.matches('button')) flush(session);
    });
  }

  // intended says what the field now asks for. Nothing is asked of a mark that
  // has no words yet and none written.
  function intended(session, final) {
    const value = session.parts.text.value;
    const had = known.get(session.id);
    if (!session.id) return value.trim() === '' ? null : { create: value };
    if (had.kind === 'question') return value === (had.body ?? '') ? null : { patch: { body: value } };
    if (value.trim() === '') {
      if (!final) return null;
      if (!had.quote) return had.body === '' ? null : { sectionEmpty: true };
      return had.kind === 'thought' ? { patch: { kind: 'highlight', body: '' } } : null;
    }
    if (had.kind === 'thought' && value === had.body) return null;
    return { patch: { kind: 'thought', body: value } };
  }

  async function flush(session, { final = false, keepalive = false, leaving = false, restored = false } = {}) {
    // A thought that is being made is waited for, so that what comes after it is
    // a change of that thought and not the making of a second.
    while (session.creating) await session.creating;
    clearTimeout(session.timer);
    const want = intended(session, final);
    session.parts.said.hidden = true;
    if (!want) return { ok: true };
    if (want.sectionEmpty) {
      session.parts.text.value = known.get(session.id).body;
      status.say(strings.sectionNeedsWords, {
        action: { label: strings.delete, run: () => remove(session.id) },
      });
      return { ok: true };
    }
    const size = new Blob([session.parts.text.value]).size;
    // A send that has to outlive the page can carry only so much.
    const survive = keepalive && size <= TOO_LONG;
    let result;
    if (want.create !== undefined) {
      session.creating = keepSectionThought(session, want.create, { keepalive: survive }).finally(() => {
        session.creating = null;
      });
      result = await session.creating;
    } else {
      session.edit.seq += 1;
      result = await change('PATCH', session.id, { edit: { ...session.edit }, ...want.patch }, { keepalive: survive, immediate: leaving });
    }
    if (result.silent) return { ok: true };
    if (!result.ok) {
      if (result.gone && !restored && session.parts.text.value !== '') {
        return bringBack(session, { final, keepalive, leaving });
      }
      if (result.gone) {
        closeNow(session);
        return result;
      }
      session.failed = true;
      showFieldFailure(session, result, size);
      return result;
    }
    session.failed = false;
    session.sent = session.parts.text.value;
    return result;
  }

  // bringBack answers a mark that was deleted elsewhere while the reader was
  // writing in it. What was written is the newest thing said of the mark, so the
  // mark is brought back and the words are put on it; they are never dropped.
  async function bringBack(session, options) {
    const result = await change('PUT', session.id, {}, { rebase: false });
    if (!result.ok) {
      session.failed = true;
      showFieldFailure(session, result, new Blob([session.parts.text.value]).size);
      return result;
    }
    session.note = noteOf(session.id);
    if (session.note) session.note.hidden = true;
    return flush(session, { ...options, restored: true });
  }

  async function keepSectionThought(session, body, { keepalive = false } = {}) {
    const id = session.id ?? session.draftId;
    const request = { id, kind: 'thought', path, anchor: session.anchor, body };
    let answer;
    try {
      answer = await send('POST', endpoint, request, { keepalive });
    } catch {
      try {
        answer = await send('GET', `${endpoint}/${id}`);
        if (answer.status !== 200) answer = await send('POST', endpoint, request, { keepalive });
      } catch {
        return { ok: false, status: 0 };
      }
    }
    if (answer.status !== 200 && answer.status !== 201) return { ok: false, status: answer.status };
    if (hidden()) setShown(true);
    session.id = id;
    session.parts.element.dataset.mfield = id;
    session.before = { kind: 'thought', body: '' };
    session.created = true;
    draw(answer.json);
    // The note drawn for it is the field's own, hidden while the field is open.
    session.note = noteOf(id);
    if (session.note) session.note.hidden = true;
    session.createdEntry = record(strings.whatWords, () => change('DELETE', id, {}, { rebase: false }));
    return { ok: true, mark: answer.json };
  }

  function showFieldFailure(session, result, size) {
    const said = session.parts.said;
    said.textContent = '';
    said.hidden = false;
    said.setAttribute('role', 'alert');
    said.append(`${fmt(strings.notKept, reasonOf(result, size))} `);
    const retry = document.createElement('button');
    retry.type = 'button';
    retry.className = 'y-mfield__retry';
    retry.textContent = strings.retry;
    said.append(retry);
  }

  function closeNow(session) {
    session.closing = true;
    clearTimeout(session.timer);
    if (session.note) session.note.hidden = false;
    session.parts.element.remove();
    if (field === session) field = null;
    layout.relayout();
  }

  // finish closes the field, unless what was written has not been kept.
  async function finish(session) {
    if (session.closing) return;
    const result = await flush(session, { final: true });
    if (!result.ok && !result.gone) {
      session.parts.text.focus();
      return;
    }
    const now = known.get(session.id);
    closeNow(session);
    if (!now || now.deleted) return;
    const was = session.before;
    if (was && !session.created && (now.kind !== was.kind || now.body !== was.body)) {
      const id = session.id;
      const entry = record(now.kind === 'highlight' ? strings.whatToHighlight : strings.whatWords, () => change('PATCH', id, { kind: was.kind, body: was.body }, { rebase: false }));
      if (now.kind === 'highlight') sayWithUndo(strings.nowHighlight, entry);
    }
    focusMark(session.id);
  }

  // abandon throws away what was written since the field opened, and says so with
  // a way back.
  async function abandon(session) {
    if (session.closing) return;
    clearTimeout(session.timer);
    if (session.creating) await session.creating;
    const typed = session.parts.text.value;
    session.closing = true;
    const id = session.id;
    // Going back is made only on a mark that is as the field left it: another
    // tab's change since is not written over.
    let stale = false;
    let undoMade = null;
    if (id && session.before && !session.created) {
      const had = known.get(id);
      if (had && (had.kind !== session.before.kind || had.body !== session.before.body)) {
        const result = await change('PATCH', id, { kind: session.before.kind, body: session.before.body }, { rebase: false });
        stale = Boolean(result.stale);
      }
    } else if (session.created) {
      const result = await change('DELETE', id, {}, { rebase: false });
      stale = Boolean(result.stale);
      if (result.ok) {
        // The step that made the thought is gone with it, and what is left to
        // take back is that it was thrown away.
        const at = undoable.indexOf(session.createdEntry);
        if (at >= 0) undoable.splice(at, 1);
        undoMade = record(strings.whatWords, () => change('PUT', id, {}, { rebase: false }));
      }
    }
    if (session.note?.isConnected) session.note.hidden = false;
    session.parts.element.remove();
    if (field === session) field = null;
    layout.relayout();
    if (stale) {
      status.say(strings.cannotUndo);
    } else if (id && typed !== (session.before?.body ?? '') && !session.created) {
      const entry = record(strings.whatWords, () => change('PATCH', id, { kind: 'thought', body: typed }, { rebase: false }));
      sayWithUndo(strings.cancelled, entry);
    } else if (undoMade) {
      sayWithUndo(strings.cancelled, undoMade);
    } else {
      status.say(strings.cancelled);
    }
    if (id && known.get(id) && !known.get(id).deleted) focusMark(id);
  }

  // What is typed and not yet sent goes when the page is hidden or left.
  document.addEventListener('visibilitychange', () => {
    if (field && document.visibilityState === 'hidden') flush(field, { keepalive: true, leaving: true });
  });
  addEventListener('pagehide', () => {
    if (field) flush(field, { keepalive: true, leaving: true });
  });
  // A reader who closes the page with words that were not kept is told once.
  addEventListener('beforeunload', (event) => {
    if (field?.failed) event.preventDefault();
  });

  // --- the selection and its toolbar --------------------------------------

  let captured = null;
  let framing = 0;

  function hideToolbar() {
    captured = null;
    toolbar.hidden = true;
  }

  function showToolbar(selection) {
    const range = selection.getRangeAt(0);
    const rects = range.getClientRects();
    const last = rects[rects.length - 1];
    if (!last) return hideToolbar();
    const slot = slotFor(prose, range.endContainer);
    if (toolbar.parentNode !== slot.parent || toolbar.nextSibling !== slot.before) slot.parent.insertBefore(toolbar, slot.before);
    toolbar.hidden = false;
    const box = prose.getBoundingClientRect();
    const width = toolbar.offsetWidth;
    const left = Math.min(Math.max(last.right - box.left - width / 2, 0), Math.max(box.width - width, 0));
    toolbar.style.setProperty('--bar-left', `${Math.round(left)}px`);
    toolbar.style.setProperty('--bar-top', `${Math.round(last.bottom - box.top + 8)}px`);
  }

  function readSelection() {
    framing = 0;
    const selection = getSelection();
    if (!selection || selection.isCollapsed || selection.rangeCount === 0 || field) return hideToolbar();
    const got = capture(prose, selection.getRangeAt(0));
    if (!got) return hideToolbar();
    captured = got;
    showToolbar(selection);
  }

  document.addEventListener('selectionchange', () => {
    if (!framing) framing = requestAnimationFrame(readSelection);
  });

  // The buttons take the press without taking the selection.
  toolbar.addEventListener('pointerdown', (event) => event.preventDefault());
  toolbar.addEventListener('mousedown', (event) => event.preventDefault());
  toolbar.addEventListener('click', (event) => {
    const act = event.target.closest('[data-mbar]')?.dataset.mbar;
    const chosen = captured;
    if (!act || !chosen) return;
    if (act === 'highlight') highlight(chosen);
    else if (act === 'question') ask(chosen);
  });
  toolbar.addEventListener('keydown', (event) => {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
    const buttons = [...toolbar.querySelectorAll('button:not([hidden])')];
    const at = buttons.indexOf(document.activeElement);
    if (at < 0) return;
    event.preventDefault();
    buttons[(at + (event.key === 'ArrowRight' ? 1 : -1) + buttons.length) % buttons.length].focus();
  });

  // --- what a highlight can do --------------------------------------------

  // A highlight has no note to carry its actions, so while it has focus they
  // stand above its words.
  const actions = document.createElement('div');
  actions.className = 'y-mbar';
  actions.hidden = true;
  actions.setAttribute('role', 'group');
  actions.setAttribute('aria-label', strings.actions);
  actions.lang = document.documentElement.lang;
  actions.dataset.markSkip = '';
  for (const [name, label, run] of [
    ['write', strings.writeThought, (id) => openField(id)],
    ['ask', strings.ask, (id) => turn(id, 'question')],
    ['delete', strings.delete, (id) => remove(id)],
  ]) {
    const act = document.createElement('button');
    act.type = 'button';
    act.className = 'y-mbar__act';
    act.dataset.act = name;
    act.textContent = label;
    act.addEventListener('click', () => {
      const id = actions.dataset.for;
      actions.hidden = true;
      if (id) run(id);
    });
    actions.append(act);
  }
  prose.addEventListener('focusin', (event) => {
    const piece = event.target.closest?.("mark.y-mark[data-kind='highlight'][tabindex]");
    if (!piece || field || hidden()) return;
    const rect = piece.getClientRects()[0];
    if (!rect) return;
    const slot = slotFor(prose, piece);
    if (actions.parentNode !== slot.parent || actions.nextSibling !== slot.before) slot.parent.insertBefore(actions, slot.before);
    actions.dataset.for = piece.dataset.mark;
    actions.hidden = false;
    const box = prose.getBoundingClientRect();
    actions.style.setProperty('--bar-left', `${Math.round(Math.max(rect.left - box.left, 0))}px`);
    actions.style.setProperty('--bar-top', `${Math.round(rect.top - box.top - actions.offsetHeight - 6)}px`);
  });
  prose.addEventListener('focusout', (event) => {
    if (actions.hidden) return;
    const to = event.relatedTarget;
    if (to instanceof Element && (actions.contains(to) || to.dataset?.mark === actions.dataset.for)) return;
    actions.hidden = true;
  });

  // --- the section's own thought ------------------------------------------

  for (const control of prose.querySelectorAll('[data-section-thought]')) {
    control.disabled = false;
    control.addEventListener('click', () => openField(null, { anchor: control.dataset.sectionAnchor }));
  }

  // --- keys ---------------------------------------------------------------

  const focusedMark = () => {
    const active = document.activeElement;
    const container = active?.closest?.('mark.y-mark[tabindex], .y-mnote');
    return container === active ? container : null;
  };
  const idOfContainer = (element) => element.dataset.mark ?? element.dataset.mnote;

  marks.key = (event) => {
    const { key } = event;
    if (navigationKey(event)) return true;
    const container = focusedMark();
    const selection = getSelection();
    const hasSelection = Boolean(selection && !selection.isCollapsed && captured);
    if (key === 'h' || key === 'q') {
      if (hasSelection) {
        (key === 'h' ? highlight : ask)(captured);
        return true;
      }
      if (container) {
        const id = idOfContainer(container);
        if (key === 'h') openField(id);
        else turn(id, 'question');
        return true;
      }
      status.say(strings.selectFirst);
      return true;
    }
    if (key === 'Enter' && container) {
      if (event.repeat) return true;
      openField(idOfContainer(container));
      return true;
    }
    if (key === 'r' && container?.dataset.kind === 'question') {
      resolve(idOfContainer(container), container.dataset.state !== 'resolved');
      return true;
    }
    if (key === 'Delete' || key === 'Backspace') {
      if (hasSelection) {
        status.say(strings.pressEscFirst);
        return true;
      }
      if (container) {
        if (!event.repeat) remove(idOfContainer(container));
        return true;
      }
    }
    return false;
  };

  marks.escape = () => {
    if (captured && getSelection()?.isCollapsed === false) {
      clearSelection();
      return true;
    }
    const container = focusedMark();
    if (container) {
      const carrier = container.closest('p, li, pre, blockquote, td, th, h1, h2, h3, h4, h5, h6, div');
      if (carrier && carrier !== prose) {
        carrier.setAttribute('tabindex', '-1');
        carrier.focus({ preventScroll: true });
        return true;
      }
    }
    if (status.shown()) {
      status.clear();
      return true;
    }
    return false;
  };

  marks.undo = () => {
    undoLast();
    return true;
  };

  return marks;
}
