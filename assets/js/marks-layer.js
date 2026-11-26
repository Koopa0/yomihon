// Where the notes of the reader's marks stand.
//
// Below the width that has room for a margin, and in a page that runs no script,
// a note stands after the paragraph it is about, where the server drew it. Where
// the margin is there, the notes are taken out of the text and set down in it,
// each as near the passage it belongs to as the ones above it allow: in the
// order of the text, only ever pushed down, never over one another or over the
// controls beside a heading. The same elements move, so a field that has focus,
// and what is typed in it, goes with its place.
//
// The layout is worked out when the page is drawn and again when its size
// changes, not when it scrolls: a note is fixed to the document, so scrolling
// moves it with the words it is about and nothing has to be asked of it.

// How far a note may be pushed from its passage before it is put back after its
// paragraph instead, and how far before a short rule says it was pushed.
const MAX_DISPLACE = 160;
const TICK_AFTER = 24;
const GAP = 8;
const OBSTACLE_PAD = 4;

const NOTES = '.y-mnote, .y-mfield';

export function initMarksLayout(prose) {
  const root = document.documentElement;
  const layer = document.createElement('div');
  layer.className = 'y-mlayer';
  layer.dataset.markSkip = '';
  prose.append(layer);
  // Items taken out of the text, with the place they left in it.
  const lifted = new Map();
  let composing = false;
  let queued = 0;
  // True while items are being moved, so that the focus the move takes away for
  // a moment is not read as the reader leaving.
  let moving = false;

  // A note that has the keyboard shows all its words and its actions, so what
  // stands below it is moved again; one that loses it is short again.
  prose.addEventListener('focusin', () => schedule());
  prose.addEventListener('focusout', () => schedule());
  prose.addEventListener('compositionstart', () => { composing = true; });
  prose.addEventListener('compositionend', () => {
    composing = false;
    schedule();
  });

  // The page's own stylesheet says whether there is room: the layer is shown only
  // where the margin is.
  const capable = () => getComputedStyle(layer).display !== 'none';

  function lift(item) {
    if (lifted.has(item)) return;
    const place = document.createComment('mnote');
    item.replaceWith(place);
    lifted.set(item, place);
    layer.append(item);
  }

  function drop(item) {
    const place = lifted.get(item);
    if (!place) return;
    lifted.delete(item);
    if (place.parentNode) place.replaceWith(item);
    else item.remove();
    item.style.removeProperty('top');
    item.style.removeProperty('--shift');
    delete item.dataset.displaced;
  }

  // keepFocus moves nodes and puts back the focus, and the text selected in a
  // field, that the move took away.
  function keepFocus(move) {
    const active = document.activeElement;
    const inside = active && active !== document.body && prose.contains(active) && active.matches?.(`${NOTES}, ${NOTES} *, .y-mark`) ? active : null;
    const field = inside?.matches?.('textarea') ? { from: inside.selectionStart, to: inside.selectionEnd, dir: inside.selectionDirection } : null;
    moving = true;
    try {
      move();
    } finally {
      moving = false;
    }
    if (inside && document.activeElement !== inside && inside.isConnected) {
      inside.focus({ preventScroll: true });
      if (field) inside.setSelectionRange(field.from, field.to, field.dir);
    }
  }

  // placeOf is the node an item left behind, or the item itself where it stands.
  const placeOf = (item) => lifted.get(item) ?? item;

  // targetOf is where the words an item is about are, measured down from the top
  // of the text.
  function targetOf(item, top) {
    const id = item.dataset.mnote ?? item.dataset.mfield;
    if (id) {
      const piece = prose.querySelector(`mark[data-mark="${CSS.escape(id)}"]`);
      const rect = piece?.getClientRects()[0];
      if (rect && rect.height > 0) return rect.top - top;
    }
    // A mark of a whole section, or of words that are no longer there, stands at
    // what comes before it in the text.
    let before = placeOf(item).previousSibling;
    while (before && (before.nodeType !== Node.ELEMENT_NODE || before.matches(NOTES))) before = before.previousSibling;
    if (!before) return 0;
    const rect = before.getBoundingClientRect();
    return (before.matches('.y-sect') ? rect.top : rect.bottom) - top;
  }

  // obstaclesIn are the controls that stand in the margin column already.
  function obstacles(top) {
    return [...prose.querySelectorAll('.y-margin')]
      .map((margin) => margin.getBoundingClientRect())
      .filter((rect) => rect.height > 0)
      .map((rect) => ({ top: rect.top - top - OBSTACLE_PAD, bottom: rect.bottom - top + OBSTACLE_PAD }))
      .sort((a, b) => a.top - b.top);
  }

  function solve() {
    const top = prose.getBoundingClientRect().top;
    const items = [...lifted.keys()];
    const wants = new Map(items.map((item, index) => [item, { target: targetOf(item, top), index }]));
    items.sort((a, b) => wants.get(a).target - wants.get(b).target || wants.get(a).index - wants.get(b).index);
    const blocks = obstacles(top);
    let cursor = 0;
    const docked = [];
    for (const item of items) {
      const { target } = wants.get(item);
      const height = item.offsetHeight;
      let y = Math.max(target, cursor);
      for (let moved = true; moved;) {
        moved = false;
        for (const block of blocks) {
          if (y < block.bottom && y + height > block.top) {
            y = block.bottom;
            moved = true;
          }
        }
      }
      // A field is where the reader is writing: it is never put back in the text.
      if (y - target > MAX_DISPLACE && !item.matches('.y-mfield')) {
        docked.push(item);
        continue;
      }
      item.style.top = `${Math.round(y)}px`;
      if (y - target > TICK_AFTER) {
        item.dataset.displaced = '';
        item.style.setProperty('--shift', `${Math.round(y - target)}px`);
      } else {
        delete item.dataset.displaced;
        item.style.removeProperty('--shift');
      }
      cursor = y + height + GAP;
    }
    return docked;
  }

  function layout() {
    queued = 0;
    if (composing) return;
    for (const [item, place] of lifted) {
      if (!item.isConnected || !layer.contains(item)) {
        place.remove();
        lifted.delete(item);
      }
    }
    keepFocus(() => {
      if (!capable()) {
        for (const item of [...lifted.keys()]) drop(item);
        root.dataset.marksLifted = '';
        return;
      }
      for (const item of prose.querySelectorAll(NOTES)) {
        if (!layer.contains(item) && !item.hasAttribute('data-docked')) lift(item);
      }
      // Putting a note that has been pushed too far back into the text moves the
      // text, so the notes are laid out again against the words where they now are.
      for (let pass = 0; pass < 3; pass++) {
        const docked = solve();
        if (docked.length === 0) break;
        for (const item of docked) {
          drop(item);
          item.dataset.docked = '';
        }
      }
      root.dataset.marksLifted = '';
    });
  }

  function schedule() {
    if (queued) return;
    queued = requestAnimationFrame(layout);
  }

  // Every note that stands after its paragraph is tried in the margin again
  // when the window's size changes, because the room may have come back.
  function relayout({ retry = false } = {}) {
    if (retry) {
      for (const item of prose.querySelectorAll('[data-docked]')) item.removeAttribute('data-docked');
    }
    schedule();
  }

  let width = prose.getBoundingClientRect().width;
  new ResizeObserver(() => {
    const now = prose.getBoundingClientRect().width;
    relayout({ retry: now !== width });
    width = now;
  }).observe(prose);
  addEventListener('resize', () => relayout({ retry: true }));
  document.fonts?.ready.then(() => relayout());
  layout();
  // swap puts a note where an older one of the same mark stood, in the margin or
  // in the text, so what replaces it needs no laying out again from nothing.
  function swap(old, next) {
    const place = lifted.get(old);
    if (place) {
      lifted.delete(old);
      lifted.set(next, place);
    }
    old.replaceWith(next);
    schedule();
  }

  return { relayout, capable, swap, moving: () => moving };
}

// The mark and its note light each other: pointing at or focusing either marks
// both, and the rest of the marks step back. Nothing here moves anything.
export function initIllumination(prose) {
  const idOf = (element) => element.dataset.mark ?? element.dataset.mnote;
  const lit = new Set();
  function light(id) {
    for (const element of prose.querySelectorAll(`[data-mark="${CSS.escape(id)}"], [data-mnote="${CSS.escape(id)}"]`)) {
      element.classList.add('is-lit');
      lit.add(element);
    }
    prose.classList.add('has-lit');
  }
  function dim() {
    for (const element of lit) element.classList.remove('is-lit');
    lit.clear();
    prose.classList.remove('has-lit');
  }
  const target = (event) => event.target.closest?.('mark[data-mark], .y-mnote[data-mnote]');
  prose.addEventListener('pointerover', (event) => {
    const element = target(event);
    if (!element) return;
    dim();
    light(idOf(element));
  });
  prose.addEventListener('pointerout', (event) => {
    const from = target(event);
    if (!from) return;
    const to = event.relatedTarget instanceof Element ? target({ target: event.relatedTarget }) : null;
    if (!to || idOf(to) !== idOf(from)) dim();
  });
  prose.addEventListener('focusin', (event) => {
    const element = target(event);
    if (!element) return;
    dim();
    light(idOf(element));
  });
  prose.addEventListener('focusout', (event) => {
    if (target(event)) dim();
  });
}
