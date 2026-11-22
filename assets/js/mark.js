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
// Beside it in the row is the control that copies the note's path for the
// reader's own editor, with the line of the section they are in. That section
// is asked at the reading line under the header. The bookmark still asks at the
// top of the window: the anchor a place is kept against and the distance kept
// below it are one agreement, and moving the anchor alone would put a reader
// back somewhere else.
//
// A note also opens where the reader last stopped in it, without their having
// pressed anything. That place is kept in this browser's own storage, against
// the section the reading line is in, and is put back by the same agreement the
// bookmark's is: an anchor and a distance below it, here measured at the
// reading line rather than at the top of the window, with the line's own
// movement in the last screen run the other way so the window comes back where
// it was. The bookmark and this are two places and neither stands in for the
// other.
//
// Every word this shows comes from the page, which is where the two languages
// are decided.

import { forgetPosition, positionOn, recallPosition, rememberPosition } from './positions.js';

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

// The reading line is where the reader's eyes are taken to be. The header stays
// over the top of the window, so the text being read starts under it, and the
// line is set this far below its lower edge.
const READING_LINE_BELOW_HEADER = 24;

// readingGeometry is what the reading line depends on: its distance under the
// header, the window's height, and how far the document scrolls.
function readingGeometry() {
  const header = document.querySelector('.y-header');
  const under = Math.max(0, header?.getBoundingClientRect().bottom ?? 0) + READING_LINE_BELOW_HEADER;
  const scroller = document.scrollingElement ?? document.documentElement;
  const view = window.innerHeight;
  return { under, view, range: Math.max(0, scroller.scrollHeight - view) };
}

// readingLineAt is the reading line's distance from the top of the window when
// the window is scrolled to top. In the last screen of the document it moves
// down with the scroll, evenly, and is at the bottom of the window where the
// document ends: a short section at the end can never be scrolled up to the
// line under the header, and would otherwise never be the one being read.
function readingLineAt(top, { under, view, range } = readingGeometry()) {
  const left = range - top;
  if (left >= view) return under;
  return under + (1 - Math.max(0, left) / view) * (view - under);
}

function readingLine() {
  return readingLineAt(window.scrollY);
}

// scrollForPoint is the window top that puts the reading line on a point of the
// document, which is readingLineAt run the other way: the point the line is on
// grows with the top, so there is one answer. Above the last screen the line
// stands a fixed distance down and the answer is that distance back from the
// point. Inside it the line moves, along a straight line, and the answer is
// solved from that line. Asked for a point the document does not reach, it
// answers the nearest top there is.
function scrollForPoint(point) {
  const { under, view, range } = readingGeometry();
  const plain = point - under;
  if (plain <= range - view) return Math.max(0, plain);
  // The line moves this far down for each pixel scrolled in the last screen.
  const slope = Math.max(0, (view - under) / view);
  return Math.min(range, Math.max(0, (point - view + range * slope) / (1 + slope)));
}

// lastAnchorAbove is the last anchor in the rendered body at or above the
// point of the document at point, and where it sits; none where there is no
// such anchor.
function lastAnchorAbove(point) {
  const article = readingColumn();
  let anchor = '';
  let anchorTop = Number.NEGATIVE_INFINITY;
  if (article) {
    for (const element of article.querySelectorAll('.y-prose [id]')) {
      if (!element.hasAttribute('data-mark-anchor')) continue;
      // A closed part of the note has IDs but no position in the document.
      if (element.getClientRects().length === 0) continue;
      const elementTop = documentTop(element);
      if (elementTop <= point && elementTop > anchorTop) {
        anchor = element.id;
        anchorTop = elementTop;
      }
    }
  }
  return { anchor, anchorTop };
}

// positionNow is the place to keep: the last anchor at or above the top of the
// window in the rendered body, and the distance from it down to that top. A
// position with no anchor above it keeps the distance alone, which is
// the weaker promise a pixel can make and the only one available there.
function positionNow() {
  const top = Math.max(0, Math.round(window.scrollY));
  const { anchor, anchorTop } = lastAnchorAbove(top);
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

// hush empties a sentence at once, with whatever was waiting to empty it.
function hush(said) {
  clearTimeout(saidTimers.get(said));
  said.textContent = '';
  delete said.dataset.replyTone;
  delete said.dataset.replyFading;
}

export function lapse(said) {
  clearTimeout(saidTimers.get(said));
  delete said.dataset.replyFading;
  saidTimers.set(
    said,
    setTimeout(() => {
      const fade = (Number.parseFloat(getComputedStyle(said).transitionDuration) || 0) * 1000;
      if (fade < 1) {
        hush(said);
        return;
      }
      said.dataset.replyFading = '';
      saidTimers.set(said, setTimeout(() => hush(said), fade));
    }, SAID_FOR),
  );
}

// The row's sentences hang in one place, so the one a press earns takes the
// place of any other still there.
function quietRow(except) {
  for (const said of document.querySelectorAll('.y-header .y-reply')) {
    if (said !== except && said.textContent !== '') hush(said);
  }
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
  quietRow(said);
  clearTimeout(saidTimers.get(said));
  delete said.dataset.replyFading;
  said.dataset.replyTone = kept ? 'kept' : 'refused';
  said.textContent = words ?? '';
  if (words) lapse(said);
}

// sectionAt is the section the element with the id anchor stands in: the last
// one whose heading is that element or comes before it, which is how the server
// chose it for a note that already held the place. It is null where there is no
// such element or it stands above the first section.
function sectionAt(anchor) {
  const article = readingColumn();
  const element = anchor ? document.getElementById(anchor) : null;
  if (!article || !element) return null;
  let held = null;
  for (const section of article.querySelectorAll('.y-prose .y-sect')) {
    const heading = section.firstElementChild;
    if (heading !== element && !(heading.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING)) break;
    held = section;
  }
  return held;
}

// showPlace draws the mark of the place kept in the margin of the section that
// holds it, and takes it off every other: a reader keeps one place, so the page
// shows one.
function showPlace(anchor) {
  const article = readingColumn();
  if (!article) return;
  for (const label of article.querySelectorAll('[data-section-bookmark]')) label.hidden = true;
  sectionAt(anchor)?.querySelector('[data-section-bookmark]')?.removeAttribute('hidden');
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

// settle puts the window where apply puts it and reveals the column, in the
// order that leaves nothing seen at a wrong place. The placing waits for the
// browser to have finished its own move, and the column is revealed in the same
// task: the first frame the reader sees is then already at the place. The two
// later placements stay because the page can still change height after that, and
// they correct the place without anything having been seen at the wrong one;
// done is called after the last of them, when the window is where it will stay
// until the reader moves it.
function settle(apply, reveal, done) {
  afterBrowserPlacement(() => {
    apply();
    reveal();
  });
  // The column's own measurements settle a frame or two after this file first
  // runs — a web font arrives, a diagram takes its height — and a position
  // applied before that lands against a page that has since moved.
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      apply();
      done();
    });
  });
  window.addEventListener(
    'pagereveal',
    () => {
      apply();
    },
    { once: true },
  );
}

// The note this document is, as the browser keeps a reader's place in it, or
// null where the document is not a note. The page names both halves on the root
// for the head script, which has to ask before the column is painted.
function readingNote() {
  const root = document.documentElement;
  const vault = root.dataset.noteVault;
  const path = root.dataset.notePath;
  return vault && path ? { vault, path } : null;
}

// How long the reader has to have stopped before the place is written. A place
// written mid-scroll is replaced by the next, so this only spares the storage.
const STOPPED_FOR = 400;

// placeAtReadingLine is the place to keep for a note: the last anchor at or
// above the reading line, and the distance from it down to the line. The
// bookmark asks at the top of the window; this is the section the reader's eyes
// are in. A line with no anchor above it keeps its depth alone, the weaker
// promise a pixel can make.
function placeAtReadingLine() {
  const point = Math.round(window.scrollY + readingLine());
  const { anchor, anchorTop } = lastAnchorAbove(point);
  if (anchor === '') return { anchor: '', offset: point };
  return { anchor, offset: Math.max(0, point - anchorTop) };
}

// windowTopFor is where the window has to be for the reading line to stand on a
// kept place, or null where the note no longer has the section the place was
// kept against, or has it closed. A depth measured from nothing would drop the
// reader at an unrelated place, so such a note opens at its top.
function windowTopFor({ anchor, offset }) {
  if (anchor === '') return scrollForPoint(offset);
  const element = document.getElementById(anchor);
  const prose = readingColumn()?.querySelector('.y-prose');
  if (!element || !prose?.contains(element) || !element.hasAttribute('data-mark-anchor')) return null;
  if (element.getClientRects().length === 0) return null;
  return scrollForPoint(documentTop(element) + offset);
}

// placeToOpenAt is the place kept for this note, when this document is one to
// put it back in. It is asked the three things the head script asks before the
// first frame: the setting is on, the address names no section, because a
// fragment is where the reader asked to go, and the document was opened afresh,
// because Back, Forward and a reload are put back by the browser.
function placeToOpenAt() {
  const note = readingNote();
  if (!note || !positionOn() || location.hash !== '') return null;
  if (performance.getEntriesByType('navigation')[0]?.type !== 'navigate') return null;
  return recallPosition(note.vault, note.path);
}

// openWhereStopped puts a note where the reader last stopped in it. A note with
// no place to open at is only revealed, which the head script has not held back
// for it either.
function openWhereStopped(reveal) {
  const place = placeToOpenAt();
  if (place === null) {
    reveal();
    watchStops();
    return;
  }
  settle(
    () => {
      const top = windowTopFor(place);
      if (top !== null) window.scrollTo({ top, behavior: 'instant' });
    },
    reveal,
    watchStops,
  );
}

let watching = false;

// watchStops keeps the place the reader is at whenever they stop: after the
// window has been still for a moment, and when the page is hidden or left,
// which a reader who scrolls and closes the tab within that moment would
// otherwise lose. It starts only after the note has been put where it opens, so
// its own moving is not taken for the reader's. A note read from its top keeps
// no place, which is what a note opens at when it keeps none.
function watchStops() {
  const note = readingNote();
  if (watching || !note) return;
  watching = true;
  let moved = false;
  let still = 0;
  const keep = () => {
    clearTimeout(still);
    if (!moved) return;
    moved = false;
    if (!positionOn()) return;
    if (window.scrollY < 1) forgetPosition(note.vault, note.path);
    else rememberPosition(note.vault, note.path, placeAtReadingLine());
  };
  window.addEventListener(
    'scroll',
    () => {
      moved = true;
      clearTimeout(still);
      still = setTimeout(keep, STOPPED_FOR);
    },
    { passive: true },
  );
  window.addEventListener('pagehide', keep);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') keep();
  });
}

// land finishes the return to a kept place the address asked for, and opens a
// note where its reader stopped when the address asked for nothing. The browser
// goes to the fragment by itself; what is left is the distance below it,
// measured from the same anchor the mark was taken against.
//
// The head script held the column out of sight for this document, so every way
// out of here reveals it. The distance is applied once the browser has moved to
// the fragment, and the column is revealed in the same task: the first frame
// the reader sees is then already at the place, where revealing it as soon as
// this file ran showed the heading for a frame and then moved.
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
    openWhereStopped(reveal);
    return;
  }
  address.searchParams.delete(OFFSET_PARAM);
  history.replaceState(null, '', address.pathname + address.search + address.hash);

  const offset = Number(carried);
  if (!Number.isInteger(offset) || offset < 0) {
    reveal();
    watchStops();
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
      watchStops();
      return;
    }
  }
  settle(
    () => {
      window.scrollTo(0, (anchor ? documentTop(anchor) : 0) + offset);
    },
    reveal,
    watchStops,
  );
}

// pathNow is what the copy control writes: the note's full path, then a colon
// and the line its current section's heading opens on in the file. The section
// is the one that holds the last anchor at or above the reading line. Where
// there is none, or the page could not vouch for its line, the path stands
// alone: a wrong line is worse than none.
function pathNow(control) {
  const path = control.dataset.path ?? '';
  const { anchor } = lastAnchorAbove(Math.round(window.scrollY + readingLine()));
  const line = sectionAt(anchor)?.dataset.sourceLine;
  return line ? `${path}:${line}` : path;
}

// A refused copy leaves the path in the sentence for the reader to copy by
// hand, so it stays until they have copied it, pressed Escape, or pressed
// somewhere else. This ends whatever is waiting for that.
let byHand = null;

// copyPath writes the path to the clipboard. Nothing it does scrolls or moves
// focus: the reader pressed it from the middle of the note and is still there.
async function copyPath(control) {
  const said = control.querySelector('[data-path-said]');
  const value = pathNow(control);
  let copied = false;
  try {
    await navigator.clipboard.writeText(value);
    copied = true;
  } catch {
    copied = false;
  }
  if (!said) return;
  byHand?.abort();
  byHand = null;
  quietRow(said);
  hush(said);
  if (copied) {
    said.dataset.replyTone = 'kept';
    said.textContent = control.dataset.pathCopied ?? '';
    lapse(said);
    return;
  }
  const path = document.createElement('span');
  path.className = 'y-headerpath__path';
  path.translate = false;
  path.textContent = value;
  said.dataset.replyTone = 'refused';
  said.append(control.dataset.pathFailed ?? '', path);
  getSelection()?.selectAllChildren(path);
  const done = new AbortController();
  byHand = done;
  const end = () => {
    done.abort();
    if (byHand === done) byHand = null;
  };
  document.addEventListener('copy', () => {
    end();
    lapse(said);
  }, { signal: done.signal });
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    end();
    hush(said);
  }, { signal: done.signal });
  document.addEventListener('pointerdown', (event) => {
    if (said.contains(event.target) || control.contains(event.target)) return;
    end();
    hush(said);
  }, { signal: done.signal });
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
  for (const control of document.querySelectorAll('[data-path-control]')) {
    control.querySelector('[data-path-button]')?.addEventListener('click', () => {
      copyPath(control);
    });
  }
}
