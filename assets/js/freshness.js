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

export function initFreshness() {
  const column = document.querySelector('.y-main[data-freshness-path]');
  const article = column && column.querySelector('.y-article');
  if (!article) return;
  const path = column.dataset.freshnessPath;
  const identity = column.dataset.freshnessIdentity;
  if (!path || !identity) return;

  const segments = path.split('/').map(encodeURIComponent).join('/');
  const endpoint = `/freshness/${segments}?identity=${encodeURIComponent(identity)}`;

  let banner = null;
  let shown = '';
  let latched = false;
  let timer = null;

  // The banner sits between the flip receipt and the article: what the reader
  // just did is announced first, then what the file has done since. It is not a
  // live region — the receipt above it already is one, and two of them would
  // talk over each other.
  function place() {
    if (!banner) {
      banner = document.createElement('p');
      banner.className = 'y-freshness';
      column.insertBefore(banner, article);
    }
    return banner;
  }

  function clear() {
    if (!banner) return;
    banner.remove();
    banner = null;
    shown = '';
  }

  function reloadButton() {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'y-freshness__action';
    button.textContent = '重新載入';
    // A plain reload keeps the browser's own scroll restoration, which puts the
    // reader back at the paragraph they were reading. Assigning the same
    // address instead would be a fresh navigation and would lose it.
    button.addEventListener('click', () => location.reload());
    return button;
  }

  function searchLink() {
    const heading = document.querySelector('.y-article .y-title');
    const words = heading ? heading.textContent.trim() : '';
    if (!words) return null;
    const link = document.createElement('a');
    link.className = 'y-freshness__action';
    link.href = `/search?q=${encodeURIComponent(words)}`;
    link.textContent = '搜尋這個標題';
    return link;
  }

  // render is idempotent by state: an unchanged answer redraws nothing, so a
  // banner that has been standing for a minute neither replays nor counts.
  function render(state) {
    if (state === shown) return;
    const element = place();
    element.replaceChildren();
    element.dataset.freshness = state;
    if (state === 'preparing') {
      element.append('此筆記已有新版本，頁面資料準備中…');
    } else if (state === 'stale') {
      element.append('此筆記已有新版本。', ' ', reloadButton());
    } else if (state === 'gone') {
      element.append('此筆記已經不在原本的位置了，可能被搬到別處，也可能已刪除。');
      const link = searchLink();
      if (link) element.append(' ', link);
    }
    shown = state;
  }

  async function ask() {
    try {
      const response = await fetch(endpoint, { headers: { Accept: 'text/plain' } });
      if (!response.ok) return null;
      return (await response.text()).trim();
    } catch {
      // A network blip is not news about the file. Saying nothing is the same
      // refusal the write face makes when it cannot confirm what it is replacing.
      return null;
    }
  }

  async function tick() {
    if (latched) return;
    const state = await ask();
    if (state === null) return;
    if (state === 'unchanged') {
      clear();
    } else if (state === 'preparing') {
      // Still worth asking: what this state is waiting for is the generation
      // catching up, and only the next answer can report that.
      render('preparing');
    } else if (state === 'stale' || state === 'gone') {
      render(state);
      latch();
    }
    // 'unreadable', and any answer a later server might add, leave the banner
    // as it stands and keep the question open.
  }

  function start() {
    if (timer !== null || latched) return;
    timer = setInterval(tick, POLL_MS);
  }

  function stop() {
    if (timer === null) return;
    clearInterval(timer);
    timer = null;
  }

  // A terminal answer has nothing further to teach: the reload is offered, or
  // the file is gone. Asking again would only repeat it.
  function latch() {
    latched = true;
    stop();
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
