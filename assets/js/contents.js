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
    // The document scrolls, so the reading line is a quarter of the way down
    // the viewport and a heading's own viewport coordinate answers directly.
    // Measuring against the article box instead made the comparison move with
    // the page, which is why the mark used to stay on the first entry.
    const readingLine = window.innerHeight * 0.25;
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

  function settle() {
    clearTimeout(settleTimer);
    locked = null;
    delete root.dataset.traveling;
    recompute();
  }

  links.forEach((link) => {
    link.addEventListener('click', () => {
      locked = targetID(link);
      mark(locked);
      root.dataset.traveling = 'on';
      clearTimeout(settleTimer);
      document.addEventListener('scrollend', settle, { once: true });
      settleTimer = setTimeout(settle, 900);
    });
  });
}
