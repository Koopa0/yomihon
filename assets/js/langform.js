// Carry the reading position through the language-switch round trip.
// The control stays a plain POST form: this only rewrites the hidden next
// field before submit, and applies the fragment the redirect comes back with.
// Without script the form still posts the bare path and lands at the top.

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

export function initLangForm() {
  restorePosition();
  const form = document.querySelector('.y-langform');
  if (!form) return;
  form.addEventListener('submit', () => {
    carryPosition(form);
  });
}
