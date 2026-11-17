// Keep the place a reader meant to come back to, and put them back on it when
// they return to it from the desk.
//
// Both halves are here because they are one agreement about what a position
// is: the id of the nearest anchor above the top of the window, and how far
// below it the window sits. A pixel alone would not survive a change of text
// size or typeface, and a mark is meant to survive days.
//
// A page is drawn from the place the server held when it was made, and a page
// the browser shows again is older than that. So the place is also read back
// from the server whenever a page is shown, and drawn as it is now.
//
// The control exists only where this runs, which is why the page hides it
// until this file marks the document. What it sends is where the window is,
// and a browser with no script has no such thing to send.
//
// A reading page draws it once, in the header row. This finds every control
// there is and reads each one's own words, so a face added to some other part of
// the page needs nothing here.
//
// Every word this shows comes from the page, which is where the two languages
// are decided.

// The distance below the anchor rides on the address as a query, spent and
// removed here. The anchor itself is the fragment, so a browser running none
// of this still lands on the heading the reader stopped under.
const OFFSET_PARAM = 'at';

function readingColumn() {
  return document.querySelector('.y-article');
}

// documentTop is where an element sits measured from the top of the document,
// which is the frame the stored offset is in. A rectangle alone is measured
// from the top of the window and moves as the page scrolls.
function documentTop(element) {
  return Math.round(element.getBoundingClientRect().top + window.scrollY);
}

// positionNow is the place to keep: the last anchor at or above the top of the
// window in the rendered body, and the distance from it down to that top. A
// position with no anchor above it keeps the distance alone, which is
// the weaker promise a pixel can make and the only one available there.
function positionNow() {
  const top = Math.max(0, Math.round(window.scrollY));
  const article = readingColumn();
  let anchor = '';
  let anchorTop = Number.NEGATIVE_INFINITY;
  if (article) {
    for (const element of article.querySelectorAll('.y-prose [id]')) {
      if (!element.hasAttribute('data-mark-anchor')) continue;
      // A closed part of the note has IDs but no position in the document.
      if (element.getClientRects().length === 0) continue;
      const elementTop = documentTop(element);
      if (elementTop <= top && elementTop > anchorTop) {
        anchor = element.id;
        anchorTop = elementTop;
      }
    }
  }
  if (anchor === '') return { anchor: '', offset: top };
  return { anchor, offset: Math.max(0, top - anchorTop) };
}

// The sentence a press earns is a small label, not a message left standing: it
// is there for a few seconds, then fades and is emptied, so it never lingers
// over the page or under the row. The fade is the stylesheet's, and the
// emptying waits for as long as the stylesheet says the fade lasts, which is
// none for a reader who asked for less motion.
const SAID_FOR = 4000;
const saidTimers = new WeakMap();

export function lapse(said) {
  clearTimeout(saidTimers.get(said));
  delete said.dataset.replyFading;
  saidTimers.set(
    said,
    setTimeout(() => {
      const fade = (Number.parseFloat(getComputedStyle(said).transitionDuration) || 0) * 1000;
      const clear = () => {
        said.textContent = '';
        delete said.dataset.replyTone;
        delete said.dataset.replyFading;
      };
      if (fade < 1) {
        clear();
        return;
      }
      said.dataset.replyFading = '';
      saidTimers.set(said, setTimeout(clear, fade));
    }, SAID_FOR),
  );
}

// drawKept says on every face of the control whether the place kept is in this
// note, the way the server draws it for a note that already held one. A face that
// is a toggle says so in its pressed state too, which is what a screen reader is
// told by.
function drawKept(kept) {
  for (const face of document.querySelectorAll('[data-mark-control]')) {
    face.toggleAttribute('data-mark-kept', kept);
    face.querySelector('[data-mark-button][aria-pressed]')?.setAttribute('aria-pressed', String(kept));
  }
}

// A press that kept the place is also the place now being in this note, so every
// face of the control draws it.
function say(control, words, kept) {
  if (kept) drawKept(true);
  const said = control.querySelector('[data-mark-said]');
  if (!said) return;
  clearTimeout(saidTimers.get(said));
  delete said.dataset.replyFading;
  said.dataset.replyTone = kept ? 'kept' : 'refused';
  said.textContent = words ?? '';
  if (words) lapse(said);
}

// showPlace draws the mark of the place kept in the margin of the section that
// holds it, and takes it off every other: a reader keeps one place, so the page
// shows one. The section is the last heading at or above the anchor, which is how
// the server chose it for a note that already held the place.
function showPlace(anchor) {
  const article = readingColumn();
  if (!article) return;
  for (const label of article.querySelectorAll('[data-section-bookmark]')) label.hidden = true;
  const element = anchor ? document.getElementById(anchor) : null;
  if (!element) return;
  let held = null;
  for (const section of article.querySelectorAll('.y-prose .y-sect')) {
    const heading = section.firstElementChild;
    if (heading !== element && !(heading.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING)) break;
    held = section;
  }
  held?.querySelector('[data-section-bookmark]')?.removeAttribute('hidden');
}

// How many times this document has kept a place. A reading of where the place is
// that began before a press is older than the press, and is not drawn after it.
let presses = 0;

async function keep(control) {
  presses += 1;
  const place = positionNow();
  const body = new URLSearchParams({
    path: control.dataset.markPath ?? '',
    anchor: place.anchor,
    offset: String(place.offset),
    identity: control.dataset.markIdentity ?? '',
  });
  let kept = false;
  try {
    const response = await fetch(control.dataset.markEndpoint ?? '', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    });
    kept = response.ok;
  } catch {
    kept = false;
  }
  if (kept) showPlace(place.anchor);
  say(control, kept ? control.dataset.markSaved : control.dataset.markFailed, kept);
}

// recall draws the place as the server holds it now. A page is drawn from the
// file as it stood when the page was made, so one that is shown again from the
// browser's HTTP cache, or fetched ahead of the visit, is older than the place
// the reader last left, and one restored from the back/forward cache is older
// still and runs nothing of its own. What is asked is where the place is, and the
// answer is drawn as the server draws it for a note that holds the place, or taken
// down where the place is in another note or in none.
//
// A page that cannot be answered keeps what it was drawn with, which is what the
// server last knew.
async function recall() {
  const face = document.querySelector('[data-mark-control]');
  const endpoint = face?.dataset.markEndpoint;
  if (!endpoint) return;
  const asked = presses;
  let held;
  try {
    const response = await fetch(endpoint, {
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    });
    if (!response.ok) return;
    held = response.status === 204 ? null : await response.json();
  } catch {
    return;
  }
  if (presses !== asked) return;
  const here = held !== null && held.path === face.dataset.markPath;
  drawKept(here);
  showPlace(here ? held.anchor : '');
}

// afterBrowserPlacement calls back once the browser has finished moving the
// window to the address's fragment. It does that on its own, when the document
// has been parsed, which is after this file first runs, and a page placed
// before the move is moved off its place by it. DOMContentLoaded is dispatched
// just ahead of the move, so what a task queued from it does comes after; the
// load event is the latest this waits, for a document whose parsing was over
// before this file ran.
function afterBrowserPlacement(then) {
  let called = false;
  const once = () => {
    if (called) return;
    called = true;
    then();
  };
  if (document.readyState === 'complete') {
    setTimeout(once);
    return;
  }
  document.addEventListener('DOMContentLoaded', () => setTimeout(once), { once: true });
  window.addEventListener('load', once, { once: true });
}

// land finishes the return to a kept place the address asked for. The browser
// goes to the fragment by itself; what is left is the distance below it,
// measured from the same anchor the mark was taken against.
//
// The head script held the column out of sight for this document, so every way
// out of here reveals it. The distance is applied once the browser has moved to
// the fragment, and the column is revealed in the same task: the first frame
// the reader sees is then already at the place, where revealing it as soon as
// this file ran showed the heading for a frame and then moved. The two later
// placements stay because the page can still change height after that, and they
// correct the place without anything having been seen at the wrong one.
//
// The query is spent whatever happens to the number in it, so a reload after
// landing is an ordinary reading of the note rather than a second return.
function land() {
  const root = document.documentElement;
  const reveal = () => {
    delete root.dataset.arrival;
  };
  const address = new URL(location.href);
  const carried = address.searchParams.get(OFFSET_PARAM);
  if (carried === null) {
    reveal();
    return;
  }
  address.searchParams.delete(OFFSET_PARAM);
  history.replaceState(null, '', address.pathname + address.search + address.hash);

  const offset = Number(carried);
  if (!Number.isInteger(offset) || offset < 0) {
    reveal();
    return;
  }

  let anchor = null;
  if (address.hash.length > 1) {
    let id = '';
    try {
      id = decodeURIComponent(address.hash.slice(1));
    } catch {
      id = address.hash.slice(1);
    }
    anchor = document.getElementById(id);
    // The note no longer carries the anchor this mark was taken against, so
    // the distance below it measures from nothing and would drop the reader at
    // an unrelated depth. A browser running none of this is left at the top of
    // the document by the same address; this leaves them there too.
    if (!anchor) {
      reveal();
      return;
    }
  }
  const apply = () => {
    window.scrollTo(0, (anchor ? documentTop(anchor) : 0) + offset);
  };
  afterBrowserPlacement(() => {
    apply();
    reveal();
  });
  // The column's own measurements settle a frame or two after this file first
  // runs — a web font arrives, a diagram takes its height — and a position
  // applied before that lands against a page that has since moved.
  requestAnimationFrame(() => {
    requestAnimationFrame(apply);
  });
  window.addEventListener(
    'pagereveal',
    () => {
      apply();
    },
    { once: true },
  );
}

export function initMark() {
  land();
  recall();
  window.addEventListener('pageshow', (event) => {
    if (event.persisted) recall();
  });
  for (const control of document.querySelectorAll('[data-mark-control]')) {
    const button = control.querySelector('[data-mark-button]');
    if (!button) continue;
    button.addEventListener('click', () => {
      keep(control);
    });
  }
}
