// Table-of-contents tracking: which entry is marked as the one being read.
// The jump itself is the browser's, and lands at once.
export function initContents() {
  const root = document.documentElement;
  const links = [...document.querySelectorAll('.y-toc__list a[href^="#"]')];
  if (links.length === 0) return;

  const targetID = (link) => decodeURIComponent(link.getAttribute('href').slice(1));
  const headings = [...new Set(links.map(targetID))]
    .map((id) => document.getElementById(id))
    .filter(Boolean);
  if (headings.length === 0) return;

  let locked = null;
  let settleTimer = null;

  // The line a heading has to cross to count as the one being read. It sits
  // where a jump to a heading actually parks it — the same clearance the
  // anchor spends — so clicking an entry marks that entry and not the next one
  // that happens to fit above an arbitrary fraction of the viewport. Reading
  // the clearance out of the stylesheet rather than repeating it here keeps
  // the two in step if the offset ever changes, and reading it once keeps the
  // observer's callback from forcing a style recalculation on every
  // intersection: the value is a fixed length in the stylesheet and cannot
  // move between renders.
  const readingLine = (Number.parseFloat(getComputedStyle(headings[0]).scrollMarginTop) || 0) + 8;

  function mark(id) {
    links.forEach((link) => {
      const active = targetID(link) === id;
      link.classList.toggle('is-active', active);
      if (active) link.setAttribute('aria-current', 'true');
      else link.removeAttribute('aria-current');
    });
  }

  function recompute() {
    if (locked) return;
    // The document scrolls, so a heading's own viewport coordinate answers
    // directly; measuring against the article box made the comparison move
    // with the page, which is why the mark used to stay on the first entry.
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
