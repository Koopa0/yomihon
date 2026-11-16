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
// way. It first gets words only after the file moves under an open page. Once
// it offers the reload there is nothing further to learn, because a later edit
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

export function initFreshness() {
  const column = document.querySelector('[data-freshness-path][data-freshness-identity]');
  if (!column) return;
  const path = column.dataset.freshnessPath;
  const identity = column.dataset.freshnessIdentity;
  if (!path || !identity) return;
  // The identity of what every embed on this page came to, stamped only on a
  // page that carries an embed. The host's identity cannot cover those bytes,
  // nor their not being there, so without this half of the ask neither an edit
  // to an embedded source nor a missing one arriving reaches an open page.
  const transcluded = column.dataset.freshnessEmbeds;

  const article = column.querySelector('.y-article');
  if (!article) return;
  const present = bannerBeside(column, article);

  const segments = path.split('/').map(encodeURIComponent).join('/');
  const embedsQuery = transcluded ? `&embeds=${encodeURIComponent(transcluded)}` : '';
  const endpoint = `/freshness/${segments}?identity=${encodeURIComponent(identity)}${embedsQuery}`;
  let latched = false;
  let timer = null;
  // The one request that may be open. A watch that is called off has to reach
  // the question it already asked, not only the ones it has not asked yet: an
  // answer landing on a page the reader has left latches the banner with nothing on screen to say why, and a request still
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
      // this page withdrew, so nothing is said.
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
    // its way: once one answer has latched there is nothing further to learn.
    if (latched) return;
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

  // A document that loads already hidden never gets a visibilitychange, so
  // starting here only when the page is in view is what keeps a background
  // tab quiet. Becoming visible later is the same listener a tab switch uses.
  if (document.visibilityState === 'visible') {
    start();
    tick();
  }
}
