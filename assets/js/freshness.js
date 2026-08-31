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

// bannerBeside puts the notice above the article. It is not a live region: the
// flip receipt above it already is one, and two of them would talk over each
// other. Once it offers the reload there is nothing further to learn, because a
// later edit does not change what the sentence says.
function bannerBeside(column, article) {
  let banner = null;
  let shown = '';
  // The words are the server's: it knows which language this reader chose, and
  // keeping them here would be a second copy to translate.
  const newVersion = column.dataset.freshnessNewversion;
  const reloadLabel = column.dataset.freshnessReload;

  function place() {
    if (!banner) {
      banner = document.createElement('p');
      banner.className = 'y-freshness';
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

export function initFreshness() {
  const column = document.querySelector('[data-freshness-path][data-freshness-identity]');
  if (!column) return;
  const path = column.dataset.freshnessPath;
  const identity = column.dataset.freshnessIdentity;
  if (!path || !identity) return;

  const article = column.querySelector('.y-article');
  const present = article ? bannerBeside(column, article) : holdInvitation(column);
  if (!present) return;

  const segments = path.split('/').map(encodeURIComponent).join('/');
  const endpoint = `/freshness/${segments}?identity=${encodeURIComponent(identity)}`;
  let latched = false;
  let timer = null;

  async function ask() {
    try {
      const response = await fetch(endpoint, { headers: { Accept: 'text/plain' } });
      if (!response.ok) return null;
      return (await response.text()).trim();
    } catch {
      // A network blip is not news about the file. Saying nothing is the same
      // refusal the write face makes when it cannot confirm what it replaces.
      return null;
    }
  }

  function stop() {
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

  start();
  tick();
}
