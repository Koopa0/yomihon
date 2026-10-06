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
// writes onto the link under the pointer and removes from the previous one.
// The name stays until the card has finished hiding: the exit is painted, and
// without the name the card has no place to be. The module owns when the card
// opens, what is in it, and when it closes; it owns no geometry at all.
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

  let timer = null;
  let askedAt = 0;
  let controller = null;
  let anchored = null;
  let exiting = null;
  let hideGen = 0;

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
    const response = await fetch(url, { headers: { Accept: 'text/html' }, signal });
    const parsed = new DOMParser().parseFromString(await response.text(), 'text/html');
    const body = parsed.querySelector('[data-preview-body]');
    if (!body) throw new Error(`preview response for ${url.pathname} carries no body`);
    return body;
  }

  function close() {
    askedAt = 0;
    clearTimeout(timer);
    timer = null;
    controller?.abort();
    controller = null;
    // The attribute is the CSS anchor. hidePopover() starts the exit and
    // returns at once. The toggle is queued as a task, so by the time it
    // runs card.getAnimations() already holds the exit transitions — the
    // empty branch is reduced motion or transition: none. Dropping the
    // name here, or in that handler without waiting, would leave a painted
    // card with no position-anchor. The name stays until those animations
    // finish. close() itself forgets nothing: the toggle moves `anchored`
    // aside so a hover that arrives mid-fade is a new open.
    if (card.matches(':popover-open')) {
      card.hidePopover();
      return;
    }
    anchored?.removeAttribute('data-preview-open');
    anchored = null;
    exiting?.removeAttribute('data-preview-open');
    exiting = null;
  }

  card.addEventListener('toggle', (event) => {
    if (event.newState !== 'closed') return;
    exiting = anchored;
    anchored = null;
    const leaving = exiting;
    const gen = ++hideGen;
    const drop = () => {
      if (hideGen !== gen || exiting !== leaving || !leaving) return;
      leaving.removeAttribute('data-preview-open');
      exiting = null;
    };
    const anims = card.getAnimations();
    if (anims.length === 0) {
      drop();
      return;
    }
    Promise.all(anims.map((animation) => animation.finished.catch(() => {}))).then(drop);
  });

  async function open(link) {
    const url = excerptURL(link);
    if (!url) return;
    const requestController = new AbortController();
    controller = requestController;
    try {
      const body = await excerpt(url, requestController.signal);
      if (controller !== requestController) return;
      // Only the fragment is imported from the separately parsed response;
      // its surrounding document never becomes part of this reading page.
      card.replaceChildren(document.importNode(body, true));
      hideGen += 1;
      if (anchored && anchored !== link) anchored.removeAttribute('data-preview-open');
      if (exiting && exiting !== link) exiting.removeAttribute('data-preview-open');
      exiting = null;
      anchored = link;
      link.setAttribute('data-preview-open', '');
      // There is no press for the markup to declare. What opens this card is a
      // pointer that has rested on a link for a quarter of a second and an
      // excerpt that has since arrived, and no attribute says either of those
      // — which is why the card is in the state the platform reserves for a
      // surface only its own page opens.
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
  function schedule(link, delay, timeStamp) {
    if (link === anchored) return;
    // The note the reader is already on has nothing to preview, and a pointer
    // resting mid-selection is dragging over words rather than asking about a
    // link.
    if (link.pathname === location.pathname) return;
    if (!getSelection()?.isCollapsed) return;
    clearTimeout(timer);
    askedAt = timeStamp;
    timer = setTimeout(() => {
      timer = null;
      open(link);
    }, delay);
  }

  // The grace is what makes the card reachable: the pointer has to be able to
  // leave the link, cross the gap, and land in the card to scroll it.
  function release() {
    askedAt = 0;
    clearTimeout(timer);
    timer = setTimeout(close, travelGrace);
  }

  // The one place that decides which links have a card. The address is read off
  // the link, which is the renderer's own word for where it leads: a link whose
  // fragment it could not place carries a second class, and a link out of the
  // vault is not a note link at all. Nothing on the server asks this question a
  // second time.
  //
  // The last term is the name at the end of the address, because a wikilink may
  // name any file the vault holds and the renderer marks a picture or a plain
  // text file exactly as it marks a note. They travel the same route, so what
  // tells them apart is that a note's name does end in .md and a file's does
  // not, and only a note has anything a card could cut.
  //
  // Prose links and declared-source rows answer to this one test, so a row
  // whose place the source lacks is refused for the reason a prose link to a
  // missing section is: its address falls back to the top of the file, and a
  // card would show that opening as though it were the passage named.
  const eligible = (link) => link.matches(':not(.wikilink-degraded)[href^="/notes/"]') && link.pathname.endsWith('.md');

  // A concept term in a lesson carries the class of the sheet that opens on
  // click, so a card on it would be a second affordance on one element.
  const links = [...root.querySelectorAll('.y-prose a.wikilink:not(.concept-link)')].filter(eligible);

  // The sources a claim declares sit in the rail, which is outside the main
  // element, and in the disclosure the narrow layout folds above the text,
  // which is inside it, so they are asked for by their own block rather than
  // by widening the root. The outline, the course navigation, the list of
  // notes citing this one and the list of notes that declare this one as their
  // source are not this note's declared sources and are never asked. The last
  // shares the block's class, so it is excluded by the attribute the renderer
  // puts on it.
  const sources = [...document.querySelectorAll('.y-basedon:not([data-declared-by]) a.ui-navitem')].filter(eligible);
  links.push(...sources);

  for (const link of links) {
    link.addEventListener('pointerenter', (event) => schedule(link, openDelay, event.timeStamp));
    link.addEventListener('pointerleave', release);
    link.addEventListener('focus', (event) => schedule(link, openDelay, event.timeStamp));
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
      // Layout can queue a scroll before a hover that reaches us first. That
      // earlier movement must not cancel the reader's newer question.
      if (!card.contains(event.target) && event.timeStamp >= askedAt) close();
    },
    { capture: true, passive: true },
  );
  window.addEventListener('pagehide', close);
}
