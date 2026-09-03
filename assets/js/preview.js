// The hover card over a link to another note: an excerpt of where that link
// leads, shown beside it, so a reader checking one sentence does not lose the
// page they are on.
//
// The card is a sighted-reading convenience and is deliberately not announced.
// A reader on a screen reader activates the link and gets the note itself,
// which is a better answer than an excerpt read out of context; wiring this
// into a live region, or describing the link with it, would replace that
// better answer with a worse one. Nothing here moves focus for the same reason.
//
// Where the card lands is CSS's decision, made from an anchor name this module
// writes onto the link under the pointer and removes from the previous one. The
// module owns when the card opens, what is in it, and when it closes; it owns
// no geometry at all.
export function initPreview() {
  const root = document.querySelector('[data-preview-endpoint]');
  const card = document.querySelector('[data-preview-card]');
  if (!root || !card) return;

  // A tap is a navigation, not a hover: on a touch screen the card would open
  // over the note the reader just asked for.
  if (!matchMedia('(pointer: fine)').matches) return;
  // Without anchor positioning the card cannot be put beside its link, and
  // without the popover API it cannot enter the top layer at all. Either way
  // what is left is the plain link, which is what a reader had before.
  if (!CSS.supports('position-area: bottom')) return;
  if (typeof HTMLElement.prototype.togglePopover !== 'function') return;

  const endpoint = new URL(root.dataset.previewEndpoint, location.href);
  if (endpoint.origin !== location.origin) return;

  const openDelay = 250;
  const travelGrace = 120;
  const excerpts = new Map();

  let timer = null;
  let controller = null;
  let anchored = null;

  // The address of the excerpt one link asks for: the note's own path carried
  // over from the link verbatim, and the fragment it addresses read off the
  // link rather than worked out again here.
  function excerptURL(link) {
    const notes = '/notes/';
    if (!link.pathname.startsWith(notes)) return null;
    const url = new URL(endpoint.pathname + link.pathname.slice(notes.length), endpoint);
    const fragment = decodeURIComponent(link.hash.slice(1));
    if (fragment) url.searchParams.set('section', fragment);
    return url;
  }

  async function excerpt(url, signal) {
    const held = excerpts.get(url.href);
    if (held !== undefined) return held;
    const response = await fetch(url, { headers: { Accept: 'text/html' }, signal });
    const parsed = new DOMParser().parseFromString(await response.text(), 'text/html');
    const body = parsed.querySelector('[data-preview-body]');
    if (!body) throw new Error(`preview response for ${url.pathname} carries no body`);
    excerpts.set(url.href, body);
    return body;
  }

  function close() {
    clearTimeout(timer);
    timer = null;
    controller?.abort();
    controller = null;
    anchored?.removeAttribute('data-preview-open');
    anchored = null;
    if (card.matches(':popover-open')) card.hidePopover();
  }

  async function open(link) {
    const url = excerptURL(link);
    if (!url) return;
    const requestController = new AbortController();
    controller = requestController;
    try {
      const body = await excerpt(url, requestController.signal);
      if (controller !== requestController) return;
      // Imported rather than adopted: the parsed document is what the cache
      // holds, and moving its node into this page would empty the entry.
      card.replaceChildren(document.importNode(body, true));
      anchored?.removeAttribute('data-preview-open');
      anchored = link;
      link.setAttribute('data-preview-open', '');
      if (!card.matches(':popover-open')) card.showPopover();
      card.scrollTop = 0;
    } catch (error) {
      if (error.name !== 'AbortError') close();
    } finally {
      if (controller === requestController) controller = null;
    }
  }

  // A hover is a question only once it has been held; a pointer crossing three
  // links on its way somewhere asked nothing. Tabbing through six of them is
  // the same crossing made with a keyboard, so it waits the same.
  function schedule(link, delay) {
    if (link === anchored) return;
    // The note the reader is already on has nothing to preview, and a pointer
    // resting mid-selection is dragging over words rather than asking about a
    // link.
    if (link.pathname === location.pathname) return;
    if (!getSelection()?.isCollapsed) return;
    clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      open(link);
    }, delay);
  }

  // The grace is what makes the card reachable: the pointer has to be able to
  // leave the link, cross the gap, and land in the card to scroll it.
  function release() {
    clearTimeout(timer);
    timer = setTimeout(close, travelGrace);
  }

  // The one place that decides which links have a card. Every term is a class
  // or an address the renderer itself writes: a link whose fragment it could
  // not place carries a second class, a concept term in a lesson carries the
  // class of the sheet that opens on click, and a link out of the vault is not
  // a note link at all. Nothing on the server asks this question a second time.
  const links = root.querySelectorAll(
    '.y-prose a.wikilink:not(.wikilink-degraded):not(.concept-link)[href^="/notes/"]',
  );
  for (const link of links) {
    link.addEventListener('pointerenter', () => schedule(link, openDelay));
    link.addEventListener('pointerleave', release);
    link.addEventListener('focus', () => schedule(link, openDelay));
    link.addEventListener('blur', close);
  }

  card.addEventListener('pointerenter', () => {
    clearTimeout(timer);
    timer = null;
  });
  card.addEventListener('pointerleave', release);

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') close();
  });
  // Scrolling the page moves the link out from under the card, so the card
  // goes; scrolling inside the card is the reader reading it.
  document.addEventListener(
    'scroll',
    (event) => {
      if (!card.contains(event.target)) close();
    },
    { capture: true, passive: true },
  );
  window.addEventListener('pagehide', close);
}
