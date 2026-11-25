// The running head: once the note's own head has gone up under the header, the
// header names the section being read, and a press on it opens the note's
// contents to go to another.
//
// Which section is being read is not decided here. The contents list in the
// popover is the contents' anatomy, so the module that marks the entry being
// read in the left column marks it in this list too, by the same reading line,
// and tells this module when the mark moves; the running head is a copy of the
// entry it marked. Whether the note's head has gone is asked of the browser by
// an intersection observer, so nothing here runs while the page scrolls.
//
// The popover is the browser's: the press names it, and opening, closing on
// Escape or a press outside, and handing focus back on Escape are all native.
// This adds the one return the browser leaves out, after a press outside that
// landed on nothing that takes focus, and closes the list when the reader
// picks a section from it.
//
// Every word shown comes from the page: the sections' own words, rendered by
// the server, and their numbers, counted here.

// The sections the stylesheet counts beside the text are the level-two
// headings that stand in the text itself, bare or wrapped with their margin; a
// level-two heading inside a quote or a callout is part of something else and
// is not counted. The list follows the same rule, so its numbers are the
// numbers in the margin.
const SECTION = ':scope > [data-level="2"], :scope > .y-sect > [data-level="2"]';

export function initRunningHead() {
  const button = document.querySelector('[data-running-head]');
  const list = document.querySelector('[data-running-contents]');
  const popover = list?.closest('[popover]');
  const article = document.querySelector('.y-article');
  const frontis = article?.querySelector('.y-frontis');
  const prose = article?.querySelector('.y-prose');
  // An engine without popovers would draw a press that opens nothing, so it
  // draws none, as a page with no script does.
  if (!button || !popover || !frontis || !prose || !('popover' in HTMLElement.prototype)) return null;

  if (!countSections(list, prose)) return null;

  const number = button.querySelector('[data-running-head-num]');
  const title = button.querySelector('[data-running-head-title]');
  let past = false;
  let marked = false;

  // follow copies the entry the contents module marked, and is what that
  // module calls each time the mark moves.
  function follow() {
    const entry = list.querySelector('a[aria-current="location"]');
    marked = entry !== null;
    if (entry) {
      const shownNumber = entry.querySelector('.y-contentspop__num')?.textContent ?? '';
      const shownTitle = entry.querySelector('.y-contentspop__title')?.textContent ?? '';
      if (number.textContent !== shownNumber) number.textContent = shownNumber;
      if (title.textContent !== shownTitle) title.textContent = shownTitle;
    }
    show();
  }

  function show() {
    const shown = past && marked;
    if (shown === button.hasAttribute('data-running-head-shown')) return;
    if (shown) {
      button.setAttribute('data-running-head-shown', '');
      return;
    }
    // A press that goes out of sight takes nothing with it: the list it
    // opened closes, and focus on it moves to the text rather than falling to
    // the document, which would start the next Tab from the top of the page.
    if (popover.matches(':popover-open')) popover.hidePopover();
    if (document.activeElement === button) document.getElementById('_y-main')?.focus({ preventScroll: true });
    button.removeAttribute('data-running-head-shown');
  }

  // The note's head has gone once all of it is above the header's lower edge,
  // which is where the text the reader is reading starts.
  const header = document.querySelector('.y-header');
  const under = Math.max(0, Math.round(header?.getBoundingClientRect().bottom ?? 0));
  const observer = new IntersectionObserver(([entry]) => {
    past = !entry.isIntersecting && entry.boundingClientRect.top < (entry.rootBounds?.top ?? under);
    show();
  }, { rootMargin: `-${under}px 0px 0px 0px` });
  observer.observe(frontis);

  // The running head fades in and out as the reader moves through the note,
  // and is simply there or not when the page arrives somewhere by itself: a
  // reload or a kept place puts the reader mid-note before they have done
  // anything, and a head fading in then would be motion nobody caused. So it
  // fades only once the reader has moved, scrolled or pressed something.
  const moved = new AbortController();
  const fades = () => {
    button.setAttribute('data-running-head-fades', '');
    moved.abort();
  };
  for (const kind of ['wheel', 'touchstart', 'keydown', 'pointerdown']) {
    window.addEventListener(kind, fades, { capture: true, passive: true, signal: moved.signal });
  }

  // The press says whether its list is open. The browser exposes that state
  // itself; the attribute says it to whatever reads the markup.
  popover.addEventListener('toggle', (event) => {
    const open = event.newState === 'open';
    button.setAttribute('aria-expanded', String(open));
    if (open) return;
    // Escape and a pick from the list hand focus back to the press already. A
    // press outside has put focus where it landed, on a control or on the text
    // column, and it stays there: that is where the reader went, and a press
    // that took it back up to the header would make the next space bar open
    // the list again instead of turning the page. Only a press on something
    // that takes no focus leaves it nowhere, and then it goes back to the
    // press that opened the list rather than falling to the document.
    const focused = document.activeElement;
    if (focused && focused !== document.body && !popover.contains(focused)) return;
    if (button.hasAttribute('data-running-head-shown')) button.focus({ preventScroll: true });
  });

  // Picking a section goes there and closes the list; the jump itself is the
  // link's own.
  list.addEventListener('click', (event) => {
    if (event.target instanceof Element && event.target.closest('a[href^="#"]')) popover.hidePopover();
  });

  button.setAttribute('data-running-head-ready', '');
  return { follow };
}

// countSections keeps the entries the stylesheet counts, in the order it counts
// them, and gives each its number where the note's sections are numbered. It
// reports whether any entry is left to name.
function countSections(list, prose) {
  const counted = new Set(prose.querySelectorAll(SECTION));
  const numbered = prose.hasAttribute('data-section-numbers');
  let ordinal = 0;
  for (const link of [...list.querySelectorAll('a[href^="#"]')]) {
    const heading = document.getElementById(decodeURIComponent(link.getAttribute('href').slice(1)));
    if (!counted.has(heading)) {
      link.closest('.y-toc__row')?.remove();
      continue;
    }
    ordinal += 1;
    const slot = link.querySelector('.y-contentspop__num');
    if (slot && numbered) slot.textContent = String(ordinal).padStart(2, '0');
  }
  return ordinal > 0;
}
