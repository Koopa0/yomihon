// What a reader does with the page's text beyond reading it. Table-of-contents
// tracking: which entry is marked as the one being read. The jump itself is the
// browser's, and lands at once. Alongside it, the one reading a page holding two
// notes can ask for: moving through one of them takes the other to the same
// place in its headings. And what a reader selects and copies across a ruby (see
// initRubySelection).

// The line a heading has to cross to count as the one being read. It sits where
// a jump to a heading actually parks it — the same clearance the anchor spends
// — so clicking an entry marks that entry and not the next one that happens to
// fit above an arbitrary fraction of the viewport. Reading the clearance out of
// the stylesheet rather than repeating it here keeps the two in step if the
// offset ever changes, and reading it once keeps a scroll handler from forcing
// a style recalculation on every call: the value is a fixed length in the
// stylesheet and cannot move between renders.
function readingLineOf(heading) {
  return (Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0) + 8;
}

export function initContents() {
  const lists = [...document.querySelectorAll('.y-toc__list')];
  if (lists.length === 0) return;

  // A page can hold more than one note, and each note's contents list answers
  // only for the headings in its own column: one mark computed across two
  // columns would follow whichever of them the reader is not looking at. A
  // page showing one note has no column marked, and its two copies of the list
  // — the one beside the prose and the one folded above it — share the mark,
  // which is what makes them read as one surface.
  const scoped = new Map();
  for (const list of lists) {
    const column = list.closest('[data-note-column]') ?? document;
    if (!scoped.has(column)) scoped.set(column, []);
    scoped.get(column).push(list);
  }
  for (const own of scoped.values()) trackReading(own);
}

// trackReading marks which entry of one note's contents list is the one being
// read, and holds that mark still while a jump it started is travelling.
function trackReading(lists) {
  const root = document.documentElement;
  const links = lists.flatMap((list) => [...list.querySelectorAll('a[href^="#"]')]);
  if (links.length === 0) return;

  const targetID = (link) => decodeURIComponent(link.getAttribute('href').slice(1));
  const headings = [...new Set(links.map(targetID))]
    .map((id) => document.getElementById(id))
    .filter(Boolean);
  if (headings.length === 0) return;

  let locked = null;
  let settleTimer = null;
  const readingLine = readingLineOf(headings[0]);

  function mark(id) {
    links.forEach((link) => {
      const active = targetID(link) === id;
      link.classList.toggle('is-active', active);
      if (active) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    });
  }

  function recompute() {
    if (locked) return;
    // A heading's own viewport coordinate answers directly, whether the
    // document scrolls or the column around it does; measuring against the
    // article box made the comparison move with the page, which is why the
    // mark used to stay on the first entry.
    let current = headings[0].id;
    for (const heading of headings) {
      if (heading.getBoundingClientRect().top <= readingLine) current = heading.id;
      else break;
    }
    mark(current);
  }

  const observer = new IntersectionObserver(recompute, { rootMargin: '0px 0px -75% 0px' });
  headings.forEach((heading) => { observer.observe(heading); });
  recompute();

  // One travel can end two ways — the scroll reports itself finished, or the
  // wait runs out — and whichever arrives first has to take the other down.
  // The timer was already cleared; the listener was not, so a travel that
  // ended on the timer left it registered and an unrelated scroll later in the
  // page ran the settle it had already run. Holding both under one signal
  // makes the pair explicit rather than accidental.
  let travel = null;

  function settle() {
    clearTimeout(settleTimer);
    settleTimer = null;
    travel?.abort();
    travel = null;
    locked = null;
    delete root.dataset.traveling;
    recompute();
  }

  links.forEach((link) => {
    link.addEventListener('click', () => {
      travel?.abort();
      travel = new AbortController();
      locked = targetID(link);
      mark(locked);
      root.dataset.traveling = 'on';
      clearTimeout(settleTimer);
      document.addEventListener('scrollend', settle, { once: true, signal: travel.signal });
      settleTimer = setTimeout(settle, 900);
    });
  });
}

// initCompareAlign offers the other way of reading two notes at once. On their
// own the columns scroll separately, which is what a reader wants while holding
// one passage against another; pressed, moving through one column takes the
// other to the heading of the same ordinal, so a translation stays beside the
// passage it translates. Nothing moves by itself and nothing animates, so a
// reader who asked for less motion gets exactly this.
//
// The columns stop being scrollers of their own where they no longer fit side
// by side, and the stylesheet withdraws the control there: levelling two
// columns the document scrolls as one means nothing.
export function initCompareAlign() {
  const toggle = document.querySelector('[data-compare-align]');
  const columns = [...document.querySelectorAll('[data-note-column]')];
  if (!toggle || columns.length !== 2) return;

  const headings = columns.map((column) => [...column.querySelectorAll('.y-prose [data-level]')]);
  // The shorter note runs out first, and beyond its last heading there is no
  // ordinal left to answer with, so levelling stops there rather than guessing.
  const shared = Math.min(headings[0].length, headings[1].length);
  if (shared === 0) return;

  // Which column a scroll was caused in, so the answering scroll does not come
  // back as a second question. It is cleared again when the assignment moved
  // nothing — a column already at the end of its travel reports no scroll, and
  // a flag left standing there would swallow the reader's next one.
  let driven = null;
  let listening = null;

  // follow takes the column at ordinal to the place the other one is being
  // read at, counted in headings rather than in pixels: the notes are the same
  // words in two languages and never the same length.
  function follow(ordinal) {
    const moving = columns[ordinal];
    const leading = headings[1 - ordinal];
    const line = readingLineOf(headings[ordinal][0]);
    let at = -1;
    for (let i = 0; i < shared; i += 1) {
      if (leading[i].getBoundingClientRect().top <= line) at = i;
      else break;
    }
    if (at < 0) return;
    const delta = headings[ordinal][at].getBoundingClientRect().top - line;
    if (Math.abs(delta) < 1) return;
    const before = moving.scrollTop;
    moving.scrollTop = before + delta;
    driven = moving.scrollTop === before ? null : moving;
  }

  function onScroll(event) {
    const source = event.currentTarget;
    if (driven === source) {
      driven = null;
      return;
    }
    follow(columns[0] === source ? 1 : 0);
  }

  toggle.addEventListener('click', () => {
    if (listening) {
      listening.abort();
      listening = null;
      driven = null;
      toggle.setAttribute('aria-pressed', 'false');
      return;
    }
    listening = new AbortController();
    columns.forEach((column) => {
      column.addEventListener('scroll', onScroll, { passive: true, signal: listening.signal });
    });
    toggle.setAttribute('aria-pressed', 'true');
    // Pressing it is the reader asking for the columns to be level now, not
    // only from the next time they scroll.
    follow(1);
  });
}

// A ruby's reading is not part of what a reader selects: the stylesheet does
// not let it be, so a selection across a ruby is its base text and the server
// places a selection by exactly that. It leaves two things for the browser to
// get wrong, and this puts them right.
//
// A triple click stops at the first reading, because a reading that cannot be
// selected ends the paragraph the browser is selecting; here the third click
// selects the whole block instead. And a copy carries the reading the author
// wrote: a ruby whose base text the selection covers in full is copied as its
// base followed by its reading in full-width parentheses, each pair of base and
// reading in turn. A base the selection covers only in part is copied as the
// base text alone, and a ruby with no reading as it is.
//
// Only the plain text is written then, because a rich-text target that took the
// markup instead would drop the readings again. Any other selection is copied
// as the browser copies it.
export function initRubySelection() {
  document.addEventListener('click', selectTheBlockOnTheThirdClick);
  document.addEventListener('copy', copyWithReadings);
}

// The blocks whose text a triple click selects, and the elements a click on
// means something else by: a link follows, a summary folds.
const TEXT_BLOCK = 'p, li, dd, dt, td, th, blockquote, figcaption, h1, h2, h3, h4, h5, h6';
const ACTS_ON_CLICK = 'a, button, input, label, select, summary, textarea';
// What starts a block of its own inside another, such as the list nested in a
// list item, whose lines a triple click on the item's own text leaves out.
const STARTS_A_BLOCK = /^(?:block|flex|flow-root|grid|list-item|table)/;

function selectTheBlockOnTheThirdClick(event) {
  if (event.detail < 3 || event.button !== 0 || event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return;
  if (!(event.target instanceof Element) || event.target.closest(ACTS_ON_CLICK)) return;
  const block = event.target.closest(TEXT_BLOCK);
  if (!block) return;
  const range = document.createRange();
  range.selectNodeContents(block);
  const nested = [...block.children].find((child) => STARTS_A_BLOCK.test(getComputedStyle(child).display));
  if (nested) range.setEndBefore(nested);
  // A block with no reading in it is selected whole by the browser already.
  if (![...block.querySelectorAll('rt')].some((reading) => range.intersectsNode(reading))) return;
  const selection = getSelection();
  selection.removeAllRanges();
  selection.addRange(range);
}

function copyWithReadings(event) {
  const selection = getSelection();
  if (!event.clipboardData || !selection || selection.rangeCount !== 1 || selection.isCollapsed) return;
  if (document.activeElement?.matches('input, textarea')) return;
  const pairs = pairsCovered(selection.getRangeAt(0));
  if (pairs.length === 0) return;
  const text = textWithReadings(selection, pairs);
  if (text === null) return;
  event.clipboardData.setData('text/plain', text);
  event.preventDefault();
}

// pairsCovered is each pair of base text and reading, in document order, whose
// base text the range covers in full: the point the base ends at, and the
// reading as it is copied. The enclosing ruby is read as well, for a selection
// made inside one.
function pairsCovered(range) {
  const container = range.commonAncestorContainer;
  const scope = container.nodeType === Node.TEXT_NODE ? container.parentElement : container;
  const enclosing = scope.closest?.('ruby');
  const rubies = [...(enclosing ? [enclosing] : []), ...scope.querySelectorAll('ruby')];
  const covered = [];
  for (const ruby of rubies) {
    if (!range.intersectsNode(ruby)) continue;
    for (const pair of pairsOf(ruby)) {
      const solid = pair.texts.filter((text) => /\S/.test(text.data));
      if (solid.length === 0) continue;
      const first = solid[0];
      const last = solid[solid.length - 1];
      const end = last.data.trimEnd().length;
      if (range.comparePoint(first, first.data.search(/\S/)) === 0 && range.comparePoint(last, end) === 0) {
        covered.push({ endNode: last, endOffset: end, reading: pair.readings.map((reading) => `\uFF08${reading}\uFF09`).join('') });
      }
    }
  }
  return covered;
}

// pairsOf is the pairs of base text and reading a ruby holds: each run of
// content that is not a reading, with the readings that follow it. A parenthesis
// is the fallback for a browser that draws no ruby, and is no reading. A reading
// with no words is nothing to write.
function pairsOf(ruby) {
  const pairs = [];
  let open = null;
  for (const child of ruby.childNodes) {
    if (child.nodeType !== Node.ELEMENT_NODE && child.nodeType !== Node.TEXT_NODE) continue;
    if (child.localName === 'rp') continue;
    if (child.localName === 'rt') {
      const reading = child.textContent.replace(/\s+/g, ' ').trim();
      if (open && reading) open.readings.push(reading);
      continue;
    }
    if (open?.readings.length) {
      pairs.push(open);
      open = null;
    }
    open ??= { texts: [], readings: [] };
    open.texts.push(...baseText(child));
  }
  if (open?.readings.length) pairs.push(open);
  return pairs;
}

// baseText is the text nodes of a ruby's content that are base text, and not
// the reading of a ruby nested in it.
function baseText(node) {
  if (node.nodeType === Node.TEXT_NODE) return [node];
  const found = [];
  const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
  for (let text = walker.nextNode(); text; text = walker.nextNode()) {
    if (!text.parentElement.closest('rt, rp')) found.push(text);
  }
  return found;
}

// textWithReadings is what the browser copies for the selection with each
// pair's reading put where its base text ends. Where that is in the copied text
// is asked of the browser itself, by selecting as far as the base's end and
// reading how much it would copy, so that the text between is whatever the
// browser makes of it, and then the selection is put back as it was. It is null
// when the browser answers in a way that does not add up, and the copy is then
// the browser's own.
function textWithReadings(selection, pairs) {
  const whole = selection.toString();
  const { anchorNode, anchorOffset, focusNode, focusOffset } = selection;
  const range = selection.getRangeAt(0);
  const { startContainer, startOffset } = range;
  const upTo = [];
  try {
    for (const { endNode, endOffset } of pairs) {
      selection.setBaseAndExtent(startContainer, startOffset, endNode, endOffset);
      upTo.push(selection.toString());
    }
  } finally {
    selection.setBaseAndExtent(anchorNode, anchorOffset, focusNode, focusOffset);
  }
  let text = '';
  let taken = 0;
  for (const [i, before] of upTo.entries()) {
    if (before.length < taken || !whole.startsWith(before)) return null;
    text += whole.slice(taken, before.length) + pairs[i].reading;
    taken = before.length;
  }
  return text + whole.slice(taken);
}
