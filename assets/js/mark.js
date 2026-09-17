// Keep the place a reader meant to come back to, and put them back on it when
// they return to it from the desk.
//
// Both halves are here because they are one agreement about what a position
// is: the id of the nearest anchor above the top of the window, and how far
// below it the window sits. A pixel alone would not survive a change of text
// size or typeface, and a mark is meant to survive days.
//
// The control exists only where this runs, which is why the page hides it
// until this file marks the document. What it sends is where the window is,
// and a browser with no script has no such thing to send.
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
// window, and the distance from it down to that top. A position with no anchor
// above it — the opening of a short note — keeps the distance alone, which is
// the weaker promise a pixel can make and the only one available there.
function positionNow() {
  const top = Math.max(0, Math.round(window.scrollY));
  const article = readingColumn();
  let anchor = '';
  let anchorTop = Number.NEGATIVE_INFINITY;
  if (article) {
    for (const element of article.querySelectorAll('[id]')) {
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

function say(control, words) {
  const said = control.querySelector('[data-mark-said]');
  if (!said) return;
  said.textContent = words ?? '';
}

async function keep(control) {
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
  say(control, kept ? control.dataset.markSaved : control.dataset.markFailed);
}

// land finishes an arrival the address asked for. The browser has already gone
// to the fragment; what is left is the distance below it, measured from the
// same anchor the mark was taken against.
//
// The query is spent whatever happens to the number in it, so a reload after
// landing is an ordinary reading of the note rather than a second arrival.
function land() {
  const address = new URL(location.href);
  const carried = address.searchParams.get(OFFSET_PARAM);
  if (carried === null) return;
  address.searchParams.delete(OFFSET_PARAM);
  history.replaceState(null, '', address.pathname + address.search + address.hash);

  const offset = Number(carried);
  if (!Number.isInteger(offset) || offset < 0) return;

  let anchor = null;
  if (address.hash.length > 1) {
    let id = '';
    try {
      id = decodeURIComponent(address.hash.slice(1));
    } catch {
      id = address.hash.slice(1);
    }
    anchor = document.getElementById(id);
  }
  const apply = () => {
    window.scrollTo(0, (anchor ? documentTop(anchor) : 0) + offset);
  };
  apply();
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
  for (const control of document.querySelectorAll('[data-mark-control]')) {
    const button = control.querySelector('[data-mark-button]');
    if (!button) continue;
    button.addEventListener('click', () => {
      keep(control);
    });
  }
}
