// Where the notes of the reader's marks stand.
//
// Below the width that has room for a margin, and in a page that runs no script,
// a note stands after the paragraph it is about, where the server drew it. Where
// the margin is there, each note is set down in it, as near the passage it
// belongs to as the ones above it allow: in the order of the text, only ever
// pushed down, never over one another or over the controls beside a heading.
//
// A note never leaves its place in the text. It is set in the margin by its
// position and nothing else, so the keyboard meets it where the words are: after
// the mark it is about, before whatever comes next, as in a page without script.
// A field that has focus, and what is typed in it, is not touched by the move.
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
const SET = '[data-lifted]';

export function initMarksLayout(prose) {
  const root = document.documentElement;
  // The layer holds nothing. It is where the margin is: the page's own stylesheet
  // shows it only where there is room, and the notes are set to its box.
  const layer = document.createElement('div');
  layer.className = 'y-mlayer';
  layer.dataset.markSkip = '';
  prose.append(layer);
  let composing = false;
  let queued = 0;

  // A note that has the keyboard shows all its words and its actions, and a field
  // grows as it is written in, so what stands below is moved again; one that
  // loses the keyboard is short again.
  prose.addEventListener('focusin', () => schedule());
  prose.addEventListener('focusout', () => schedule());
  prose.addEventListener('compositionstart', () => { composing = true; });
  prose.addEventListener('compositionend', () => {
    composing = false;
    schedule();
  });
  const grown = new ResizeObserver(() => schedule());

  // The page's own stylesheet says whether there is room.
  const capable = () => getComputedStyle(layer).display !== 'none';

  function lift(item) {
    item.dataset.lifted = '';
    grown.observe(item);
  }

  function drop(item) {
    grown.unobserve(item);
    delete item.dataset.lifted;
    delete item.dataset.displaced;
    for (const property of ['top', 'left', 'width', '--shift']) item.style.removeProperty(property);
  }

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
    let before = item.previousElementSibling;
    while (before?.matches(NOTES)) before = before.previousElementSibling;
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

  // setBox puts an item at a height of the text and in the column of the margin.
  // The box it is measured from is the nearest one the item is positioned in,
  // which is the text itself unless something between has a position of its own.
  function setBox(item, y, top, column) {
    const from = item.offsetParent ?? prose;
    const box = from.getBoundingClientRect();
    item.style.top = `${Math.round(y + top - box.top - from.clientTop)}px`;
    item.style.left = `${Math.round(column.left - box.left - from.clientLeft)}px`;
    item.style.width = `${Math.round(column.width)}px`;
  }

  function solve() {
    const top = prose.getBoundingClientRect().top;
    const column = layer.getBoundingClientRect();
    const items = [...prose.querySelectorAll(SET)].filter((item) => !item.hidden);
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
      setBox(item, y, top, column);
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
    if (!capable()) {
      for (const item of prose.querySelectorAll(SET)) drop(item);
      root.dataset.marksLifted = '';
      return;
    }
    for (const item of prose.querySelectorAll(NOTES)) {
      if (!item.hasAttribute('data-lifted') && !item.hasAttribute('data-docked')) lift(item);
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
    for (const name of ['data-lifted', 'data-docked', 'data-displaced']) {
      if (old.hasAttribute(name)) next.setAttribute(name, '');
    }
    next.style.cssText = old.style.cssText;
    grown.unobserve(old);
    if (next.hasAttribute('data-lifted')) grown.observe(next);
    old.replaceWith(next);
    schedule();
  }

  return { relayout, capable, swap };
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
