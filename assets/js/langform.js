// Carry the reading position through the round trips that leave a note and
// come back to it: the language form, and the walk out to the reading choices
// and back. Both end in a redirect the server builds from an address the page
// handed it, so the position rides on that address and nothing new is stored.
// The controls stay a plain POST form and a plain link; this only rewrites the
// address just before it is followed, and applies the fragment the redirect
// comes back with. Without script both still carry the bare path and land at
// the top.

const MARK = 'y-at:';

function pathOnly(address) {
  const hash = address.indexOf('#');
  return hash === -1 ? address : address.slice(0, hash);
}

function carryPosition(form) {
  const next = form.querySelector('input[name="next"]');
  if (!next) return;
  const address = next.value;
  if (!address.startsWith('/') || address.startsWith('//')) return;
  const y = Math.round(window.scrollY);
  if (y <= 0) return;
  const path = pathOnly(address);
  next.value = path + '#' + MARK + y;
}

function restorePosition() {
  let raw;
  try {
    raw = decodeURIComponent(location.hash.slice(1));
  } catch {
    return;
  }
  if (!raw.startsWith(MARK)) return;
  const y = Number(raw.slice(MARK.length));
  if (!Number.isFinite(y) || y < 0 || !Number.isInteger(y)) return;
  history.replaceState(null, '', `${location.pathname}${location.search}`);
  const apply = () => {
    window.scrollTo(0, y);
  };
  apply();
  requestAnimationFrame(() => {
    requestAnimationFrame(apply);
  });
  // An arrival is announced once the document that receives it is revealed,
  // which on a navigation can be after this module first ran; the position is
  // applied again there, or the first write lands on a document that has not
  // been handed over yet.
  window.addEventListener('pagereveal', () => {
    apply();
  }, { once: true });
}

// The walk to the reading choices is a link, so the position goes on the
// address it already carries to come back by. It is written into that address
// rather than into any form on the page it leads to: which forms that page has
// is its own business and has been rearranged before, while the return address
// is the one thing every one of them sends the reader home by.
function carryPositionToLink(link) {
  const address = new URL(link.href, location.href);
  const from = address.searchParams.get('from');
  if (!from || !from.startsWith('/') || from.startsWith('//')) return;
  const y = Math.round(window.scrollY);
  if (y <= 0) return;
  address.searchParams.set('from', pathOnly(from) + '#' + MARK + y);
  link.href = address.pathname + address.search;
}

export function initLangForm() {
  restorePosition();
  for (const link of document.querySelectorAll('[data-carry-position]')) {
    link.addEventListener('click', () => {
      carryPositionToLink(link);
    });
  }
  const form = document.querySelector('.y-langform');
  if (!form) return;
  form.addEventListener('submit', () => {
    carryPosition(form);
  });
}
