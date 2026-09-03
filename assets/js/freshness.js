// A reading page's one question about itself: are the words below still the
// words in the file. The server answers it from the file rather than from the
// published generation, so a reader who just saved in Obsidian is not waiting
// on a rebuild to be told anything.
//
// The page never reloads itself. It offers the reload, and only once the
// server says a reload would actually deliver the newer version — an offer
// that returns the same words twice teaches a reader to stop believing it.
// Everything here is absent without this script: the server sends no banner to
// hide, so a page with no JavaScript is a page with nothing to explain.

const POLL_MS = 5000;

// bannerBeside puts the notice above the article, as a status live region: it
// appears while the reader is somewhere in the prose, so a reader who is not
// looking at the top of the column — or not looking at all — is told the same
// way. The flip receipt is a live region too, but the two never speak at once:
// the receipt says its sentence on arrival and never changes, while this one
// first gets words only after the file moves under an open page. Once it
// offers the reload there is nothing further to learn, because a later edit
// does not change what the sentence says.
function bannerBeside(column, article) {
  let banner = null;
  let shown = '';
  // The words are the server's: it knows which language this reader chose, and
  // keeping them here would be a second copy to translate.
  const newVersion = column.dataset.freshnessNewversion;
  const reloadLabel = column.dataset.freshnessReload;
  const preparing = column.dataset.freshnessPreparing;
  const gone = column.dataset.freshnessGone;
  const searchTitleLabel = column.dataset.freshnessSearchtitle;

  function place() {
    if (!banner) {
      banner = document.createElement('p');
      banner.className = 'y-freshness';
      banner.setAttribute('role', 'status');
      column.insertBefore(banner, article);
    }
    return banner;
  }

  function reloadButton() {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'y-freshness__action';
    button.textContent = reloadLabel;
    // A plain reload keeps the browser's own scroll restoration, which puts the
    // reader back at the paragraph they were reading. Assigning the same
    // address would be a fresh navigation and would lose it.
    button.addEventListener('click', () => location.reload());
    return button;
  }

  function searchLink() {
    const heading = column.querySelector('.y-article .y-title');
    const words = heading ? heading.textContent.trim() : '';
    if (!words) return null;
    const link = document.createElement('a');
    link.className = 'y-freshness__action';
    link.href = `/search?q=${encodeURIComponent(words)}`;
    link.textContent = searchTitleLabel;
    return link;
  }

  // Drawing is idempotent by state, so a banner standing for a minute neither
  // replays nor counts.
  return (state) => {
    if (state === 'unchanged') {
      if (banner) {
        banner.remove();
        banner = null;
        shown = '';
      }
      return false;
    }
    if (state !== 'preparing' && state !== 'stale' && state !== 'gone') return false;
    if (state === shown) return state === 'stale' || state === 'gone';
    const element = place();
    element.replaceChildren();
    element.dataset.freshness = state;
    if (state === 'preparing') {
      element.append(preparing);
    } else if (state === 'stale') {
      element.append(newVersion, ' ', reloadButton());
    } else {
      element.append(gone);
      const link = searchLink();
      if (link) element.append(' ', link);
    }
    shown = state;
    return state === 'stale' || state === 'gone';
  };
}

// holdInvitation gates the recovery page's way back to the note. Sending a
// reader into the same bytes a write was just refused against would stage that
// refusal a second time, so the link waits until the reading generation holds
// at least the version the refused write saw. Nothing here latches on a version
// change: unlike the banner's sentence, a released link can be owed the wait
// again the moment the file moves once more.
function holdInvitation(column) {
  // The words are the server's, read off the page the same way the banner's are.
  const preparingTitle = column.dataset.freshnessHoldtitle;
  const preparingDetail = column.dataset.freshnessHolddetail;
  const goneTitle = column.dataset.freshnessGonetitle;
  const goneDetail = column.dataset.freshnessGone;
  const link = column.querySelector('a.y-recovery__action[href^="/notes/"]');
  if (!link) return null;
  const href = link.getAttribute('href');
  const label = link.textContent;
  let note = null;

  function hold(linkText, explanation) {
    if (!note) {
      note = document.createElement('p');
      note.className = 'y-freshness';
      link.insertAdjacentElement('afterend', note);
    }
    // Dropping href rather than styling a dead control: an anchor without one
    // is not activated by a click, by Enter, or by a middle button either.
    link.removeAttribute('href');
    link.setAttribute('aria-disabled', 'true');
    link.textContent = linkText;
    note.textContent = explanation;
  }

  function release() {
    if (!note) return;
    link.setAttribute('href', href);
    link.removeAttribute('aria-disabled');
    link.textContent = label;
    note.remove();
    note = null;
  }

  return (state) => {
    if (state === 'preparing') {
      hold(preparingTitle, preparingDetail);
      return false;
    }
    if (state === 'gone') {
      hold(goneTitle, goneDetail);
      return true;
    }
    if (state === 'unchanged' || state === 'stale') release();
    return false;
  };
}

// holdRulings parks the status controls once the page knows the words it
// shows have moved on — in its own file, or in a note it excerpts — or the
// file is not there any more: a press from there would make a ruling over
// words the reader has not seen. Where only an excerpt moved, the host's
// identity still matches and the write face would accept the press, so
// parking here is the only guard. The controls keep their explanation
// beside them, read off the column like every other sentence of the watch,
// and a reload re-renders them live — which is what the sentence asks for.
// Both answers that park them are latched by the banner, so nothing here
// needs a way back.
function holdRulings(column) {
  const explanation = column.dataset.freshnessWritehold;
  let held = false;
  return (state) => {
    if (held || (state !== 'stale' && state !== 'gone')) return;
    held = true;
    const faces = new Set();
    for (const form of document.querySelectorAll('form.y-statusform')) {
      for (const button of form.querySelectorAll('button')) button.disabled = true;
      const face = form.closest('.y-statuspanel, .y-sealbar');
      if (face) faces.add(face);
    }
    if (!explanation) return;
    for (const face of faces) {
      const note = document.createElement('p');
      note.className = face.classList.contains('y-sealbar')
        ? 'y-sealbar__notice y-statusflag'
        : 'y-statuspanel__notice y-statusflag';
      note.textContent = explanation;
      face.append(note);
    }
  };
}

export function initFreshness() {
  const column = document.querySelector('[data-freshness-path][data-freshness-identity]');
  if (!column) return;
  const path = column.dataset.freshnessPath;
  const identity = column.dataset.freshnessIdentity;
  if (!path || !identity) return;
  // The status the page printed beside the title, rendered by the server. The
  // identity leaves that one value out, so the ask carries it separately; the
  // recovery column stamps no status and asks about its identity alone.
  const printedStatus = column.dataset.freshnessStatus;
  // The identity of what the render pulled in from other notes, stamped only
  // on a page that actually did. The host's identity cannot cover those
  // bytes, so without this half of the ask an edit to an embedded source
  // never reaches an open page.
  const transcluded = column.dataset.freshnessEmbeds;

  const article = column.querySelector('.y-article');
  const present = article ? bannerBeside(column, article) : holdInvitation(column);
  if (!present) return;
  const rulings = article ? holdRulings(column) : null;

  const segments = path.split('/').map(encodeURIComponent).join('/');
  const statusQuery =
    printedStatus === undefined ? '' : `&status=${encodeURIComponent(printedStatus)}`;
  const embedsQuery = transcluded ? `&embeds=${encodeURIComponent(transcluded)}` : '';
  const endpoint = `/freshness/${segments}?identity=${encodeURIComponent(identity)}${statusQuery}${embedsQuery}`;
  let latched = false;
  let timer = null;
  // The one request that may be open. A watch that is called off has to reach
  // the question it already asked, not only the ones it has not asked yet: an
  // answer landing on a page the reader has left parks the write face and
  // latches the banner with nothing on screen to say why, and a request still
  // open is one of the things that keeps a document out of the store the back
  // button reads from.
  let activeController = null;

  async function ask() {
    // An earlier ask that has not answered describes a page state this one
    // supersedes, so it is dropped rather than raced: with exactly one request
    // open, calling the watch off always reaches it.
    activeController?.abort();
    const requestController = new AbortController();
    activeController = requestController;
    try {
      const response = await fetch(endpoint, {
        headers: { Accept: 'text/plain' },
        signal: requestController.signal,
      });
      if (!response.ok) return null;
      return (await response.text()).trim();
    } catch {
      // A network blip is not news about the file, and neither is a question
      // this page withdrew. Saying nothing is the same refusal the write face
      // makes when it cannot confirm what it replaces.
      return null;
    } finally {
      if (activeController === requestController) activeController = null;
    }
  }

  function stop() {
    activeController?.abort();
    activeController = null;
    if (timer === null) return;
    clearInterval(timer);
    timer = null;
  }

  function start() {
    if (timer !== null || latched) return;
    timer = setInterval(tick, POLL_MS);
  }

  async function tick() {
    if (latched) return;
    const state = await ask();
    // 'unreadable', a failed request, and any answer a later server might add
    // leave the page as it stands and keep the question open.
    if (state === null || state === 'unreadable') return;
    // Asked again, because the page may have settled while this answer was on
    // its way: once one answer has latched there is nothing further to learn,
    // and acting on a second would rule twice over one change.
    if (latched) return;
    if (rulings) rulings(state);
    if (present(state)) {
      latched = true;
      stop();
    }
  }

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible') {
      stop();
      return;
    }
    start();
    tick();
  });
  // Leaving the page ends the watch outright. The reader who comes back gets
  // it again through the visibility change that restores the document, which
  // is the same door a tab switch already used.
  window.addEventListener('pagehide', stop);

  start();
  tick();
}
