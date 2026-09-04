// Behavior lock: Home starts at the top and stays a plain GET form; /search and
// the command palette update in place from the lexical-results endpoint, and
// /search's own address follows its settled results in place; stale responses
// cannot replace the newest query; every GET form still works with and without
// JS.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (Home), and
// MUTATE. MUTATE=list prints every self-test mode.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
const RESULT_PATH = '/search/results';
const QUERY = 'tortoise';
const DEBOUNCE_MS = 180;
// A word deep inside a note long enough that arriving at it has to scroll: on a
// note that fits the viewport "the reader arrived at the hit" is true before
// anything moves, and a check that cannot fail is worth nothing.
const HIT_QUERY = 'Ballast';
const HIT_NOTE = '/notes/Notes/Glass%20Tide.md';
const HIT_DIRECTIVE = `${HIT_NOTE}#:~:text=${HIT_QUERY}`;

const SITES = [
  'home-starts-at-top',
  'home-does-not-take-focus',
  'home-remains-plain-get',
  'page-live-results',
  'page-url-follows-results',
  'dialog-live-results',
  'trailing-debounce',
  'ime-waits-for-composition',
  'superseded-request-is-aborted',
  'latest-query-wins',
  'settled-count',
  'live-error-recovery',
  'stale-label-follows-the-box',
  'stale-label-reads-a-padded-address',
  'hit-opens-at-the-match',
  'live-hit-opens-at-the-match',
  'enter-submits-get',
  'button-submits-get',
  'no-js-get',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN search-behavior: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL search-behavior: ${message}`);
};
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED search-behavior: ${message}`); };

const occurrences = (body, needle) => body.split(needle).length - 1;

// Rewrites a fetched document or script only after every exact needle has the
// expected cardinality. A stale mutation therefore exits 2 instead of letting
// an unchanged page masquerade as a caught regression.
const rewriteResponse = async (page, url, replacements, label) => {
  let seen = 0;
  let applied = 0;
  let issue = '';
  await page.route(url, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    seen += 1;
    const response = await route.fetch();
    const original = await response.text();
    for (const { needle, expected = 1 } of replacements) {
      const got = occurrences(original, needle);
      if (got !== expected) {
        issue = `${label} needle ${JSON.stringify(needle)} matched ${got} times, want ${expected}`;
        await route.fulfill({ response, body: original });
        return;
      }
    }
    let body = original;
    for (const { needle, replacement } of replacements) body = body.replaceAll(needle, replacement);
    applied += 1;
    await route.fulfill({ response, body });
  });
  return () => issue || (seen === 0 ? `${label} was never requested` : '') || (applied !== seen ? `${label} was not wholly rewritten` : '');
};

const rewriteHome = (replacements, label) => (page) => rewriteResponse(page, BASE + PAGE, replacements, label);
const rewriteSearchPage = (replacements, label) => (page) => rewriteResponse(page, BASE + '/search', replacements, label);
const rewriteScript = (replacements, label, moduleName = 'search.js') => (page) => rewriteResponse(page, `**/${moduleName}`, replacements, label);

const MUTATIONS = {
  'home-autofocus': {
    target: 'home-does-not-take-focus',
    before: rewriteHome([
      {
        needle: 'name="q" placeholder="搜尋書庫…" aria-label="搜尋筆記">',
        replacement: 'name="q" placeholder="搜尋書庫…" aria-label="搜尋筆記" autofocus>',
      },
    ], 'Home document'),
  },
  'scroll-home-main': {
    target: 'home-starts-at-top',
    after: async (page) => {
      // The document is the scroller, so this moves the document. Mutating
      // .y-main would leave the probe green whatever happened, which is what
      // it did the first time this layout changed underneath it.
      const state = await page.evaluate(() => {
        const overflow = document.createElement('div');
        overflow.style.blockSize = '2000px';
        overflow.dataset.e2eScrollMutation = '';
        document.body.append(overflow);
        document.documentElement.style.scrollBehavior = 'auto';
        document.scrollingElement.scrollTop = 80;
        return {
          scrollTop: document.scrollingElement.scrollTop,
          marked: overflow.hasAttribute('data-e2e-scroll-mutation'),
        };
      });
      return () => state.marked && state.scrollTop > 0 ? '' : `the document stayed at ${state.scrollTop} after the scroll mutation`;
    },
  },
  'focus-home-without-scroll': {
    target: 'home-does-not-take-focus',
    before: rewriteScript([
      {
        needle: '  const search = initSearch();',
        replacement: "  document.querySelector('[data-home-block=\"search\"] input[name=\"q\"]')?.focus({ preventScroll: true });\n  const search = initSearch();",
      },
    ], 'Home focus script', 'yomihon.js'),
  },
  'enhance-home-search': {
    target: 'home-remains-plain-get',
    before: rewriteHome([
      {
        needle: 'class="y-homesearchrow" data-home-block="search"',
        replacement: 'class="y-homesearchrow" data-home-block="search" data-live-search data-live-search-endpoint="/search/results"',
        expected: 1,
      },
      {
        needle: 'class="y-homesearch" method="get" action="/search"',
        replacement: 'class="y-homesearch" method="get" action="/search" data-live-search-form',
        expected: 1,
      },
      {
        needle: 'name="q" placeholder="搜尋書庫…" aria-label="搜尋筆記">',
        replacement: 'name="q" placeholder="搜尋書庫…" aria-label="搜尋筆記" data-live-search-input>',
        expected: 1,
      },
      {
        // The status and the results go inside the search landmark, which is
        // the element the enhancement scopes itself to. They sat before a
        // closing section when the desk's search was a block; the row has none,
        // and appending after the landmark would put them outside the scope.
        needle: '<button type="submit" class="y-xbtn">搜尋</button></form>',
        replacement: '<button type="submit" class="y-xbtn">搜尋</button></form><p class="y-live-search__status" data-live-search-status role="status" aria-live="polite" aria-atomic="true"></p><div class="y-searchresults" data-live-search-results data-result-count="0" aria-busy="false"></div>',
        expected: 1,
      },
    ], 'Home plain search'),
  },
  'post-home-form': {
    target: 'button-submits-get',
    before: rewriteHome([
      {
        needle: 'class="y-homesearch" method="get" action="/search"',
        replacement: 'class="y-homesearch" method="post" action="/search"',
      },
    ], 'Home GET form'),
  },
  'drop-home-query-name': {
    target: 'button-submits-get',
    before: rewriteHome([
      {
        needle: 'name="q" placeholder="搜尋書庫…" aria-label="搜尋筆記">',
        replacement: 'name="query" placeholder="搜尋書庫…" aria-label="搜尋筆記">',
      },
    ], 'Home query field'),
  },
  'page-wrong-endpoint': {
    target: 'page-live-results',
    before: rewriteSearchPage([
      {
        needle: 'class="y-searchpage" data-live-search data-live-search-endpoint="/search/results"',
        replacement: 'class="y-searchpage" data-live-search data-live-search-endpoint="/missing"',
      },
    ], 'search-page endpoint'),
  },
  'hide-page-results': {
    target: 'page-live-results',
    before: rewriteSearchPage([
      {
        needle: 'class="y-searchresults" data-live-search-results',
        replacement: 'class="y-searchresults" style="opacity:0" data-live-search-results',
        expected: 2,
      },
    ], 'search-page result surfaces'),
  },
  'dialog-wrong-endpoint': {
    target: 'dialog-live-results',
    before: rewriteHome([
      {
        needle: 'data-search data-live-search data-live-search-endpoint="/search/results"',
        replacement: 'data-search data-live-search data-live-search-endpoint="/missing"',
      },
    ], 'dialog endpoint'),
  },
  'hide-dialog-results': {
    target: 'dialog-live-results',
    before: rewriteHome([
      {
        needle: 'class="y-searchresults" data-live-search-results',
        replacement: 'class="y-searchresults" style="opacity:0" data-live-search-results',
        expected: 1,
      },
    ], 'dialog result surfaces'),
  },
  'drop-url-sync': {
    target: 'page-url-follows-results',
    before: rewriteScript([
      {
        needle: "          history.replaceState(history.state, '', address);",
        replacement: '          void address;',
      },
    ], 'live-search URL sync'),
  },
  'push-url-instead': {
    target: 'page-url-follows-results',
    before: rewriteScript([
      {
        needle: "          history.replaceState(history.state, '', address);",
        replacement: "          history.pushState(history.state, '', address);",
      },
    ], 'live-search URL replace'),
  },
  'drop-debounce-cancel': {
    target: 'trailing-debounce',
    before: rewriteScript([
      {
        needle: '    function schedule() {\n      clearTimeout(timer);',
        replacement: '    function schedule() {\n      void timer;',
      },
    ], 'live-search script'),
  },
  'ignore-ime-composition': {
    target: 'ime-waits-for-composition',
    before: rewriteScript([
      {
        needle: '      if (!composing) schedule();',
        replacement: '      schedule();',
      },
    ], 'live-search IME guard'),
  },
  'keep-superseded-request': {
    target: 'superseded-request-is-aborted',
    before: rewriteScript([
      {
        needle: '      const requestID = ++latestRequest;\n      activeController?.abort();\n      activeController = null;',
        replacement: '      const requestID = ++latestRequest;\n      void activeController;\n      activeController = null;',
      },
    ], 'live-search abort guard'),
  },
  'accept-stale-response': {
    target: 'latest-query-wins',
    before: rewriteScript([
      {
        needle: '        if (requestID !== latestRequest) return;',
        replacement: '        if (false) return;',
      },
    ], 'live-search request-ID guard'),
  },
  'wrong-settled-count': {
    target: 'settled-count',
    after: (page) => rewriteResponse(page, '**/search/results?*', [
      { needle: 'data-result-count="1"', replacement: 'data-result-count="9"' },
    ], 'result fragment'),
    proofPhase: 'action',
  },
  'remove-error-recovery': {
    target: 'live-error-recovery',
    before: rewriteScript([
      {
        needle: "        status.textContent = status.dataset.liveSearchOffline ?? '';",
        replacement: "        status.textContent = '';",
      },
    ], 'live-search recovery message'),
  },
  'remove-stale-label': {
    target: 'live-error-recovery',
    before: rewriteScript([
      {
        needle: '        markStaleNote(query);',
        replacement: '        void query;',
      },
    ], 'live-search stale-results label'),
  },
  'drop-hit-fragment': {
    target: 'hit-opens-at-the-match',
    before: (page) => rewriteResponse(page, '**/search?*', [
      { needle: `href="${HIT_DIRECTIVE}"`, replacement: `href="${HIT_NOTE}"` },
    ], 'search-page hit directive'),
  },
  'drop-live-hit-fragment': {
    target: 'live-hit-opens-at-the-match',
    after: (page) => rewriteResponse(page, '**/search/results?*', [
      { needle: `href="${HIT_DIRECTIVE}"`, replacement: `href="${HIT_NOTE}"` },
    ], 'live-result hit directive'),
    proofPhase: 'action',
  },
  'stale-label-never-hides': {
    target: 'stale-label-follows-the-box',
    before: rewriteScript([
      {
        needle: "      if (note) note.hidden = (note.dataset.liveSearchStale ?? '').trim() === query;",
        replacement: "      if (note && (note.dataset.liveSearchStale ?? '').trim() !== query) note.hidden = false;",
      },
    ], 'live-search stale-label assignment'),
  },
  'stale-label-ignores-padding': {
    target: 'stale-label-reads-a-padded-address',
    before: rewriteScript([
      {
        needle: "      if (note) note.hidden = (note.dataset.liveSearchStale ?? '').trim() === query;",
        replacement: "      if (note) note.hidden = (note.dataset.liveSearchStale ?? '') === query;",
      },
    ], 'live-search stale-label trim'),
  },
  'prevent-enter-submit': {
    target: 'enter-submits-get',
    before: rewriteScript([
      {
        needle: "    form.addEventListener('submit', cancelPending);",
        replacement: "    form.addEventListener('submit', (event) => { event.preventDefault(); cancelPending(); });",
      },
    ], 'live-search submit observer'),
  },
  'disable-search-button': {
    target: 'button-submits-get',
    after: async (page) => {
      const button = page.locator('[data-home-block="search"] button[type="submit"]');
      const count = await button.count();
      if (count !== 1) return () => `Home submit button count was ${count}, want 1`;
      await button.evaluate((element) => { element.type = 'button'; });
      return () => '';
    },
  },
  'post-no-js-form': {
    target: 'no-js-get',
    before: rewriteSearchPage([
      {
        needle: 'class="y-searchpage__form" method="get" action="/search"',
        replacement: 'class="y-searchpage__form" method="post" action="/search"',
      },
    ], 'no-JS search form'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`search-behavior: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`search-behavior: site ${site} has no mutation, so nothing proves it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`search-behavior: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

let mutationApplied = false;

const arm = async (page, site) => {
  const mutation = MUTATE && MUTATIONS[MUTATE].target === site ? MUTATIONS[MUTATE] : null;
  const proofs = [];
  if (mutation?.before) proofs.push({ phase: 'load', check: await mutation.before(page) });
  return {
    afterLoad: async () => {
      if (mutation?.after) proofs.push({ phase: mutation.proofPhase || 'load', check: await mutation.after(page) });
    },
    prove: (phase) => {
      if (!mutation) return;
      for (const proof of proofs.filter((item) => item.phase === phase)) {
        const issue = proof.check();
        if (issue) notApplied(`${MUTATE}: ${issue}`);
      }
      if (proofs.some((item) => item.phase === phase)) mutationApplied = true;
    },
  };
};

const start = async (browser, site, {
  path = PAGE,
  javaScriptEnabled = true,
  clock = false,
  initScript = null,
} = {}) => {
  const context = await browser.newContext({ javaScriptEnabled, viewport: { width: 1270, height: 720 } });
  const page = await context.newPage();
  if (initScript) await page.addInitScript(initScript);
  if (clock) await page.clock.install({ time: new Date('2026-07-11T00:00:00Z') });
  const armed = await arm(page, site);
  await page.goto(BASE + path, { waitUntil: 'load' });
  if (clock) await page.clock.pauseAt(new Date('2026-07-11T00:01:00Z'));
  await armed.afterLoad();
  armed.prove('load');
  return { context, page, armed };
};

const waitFor = async (page, site, fn, arg, message, timeout = 3000) => {
  try {
    await page.waitForFunction(fn, arg, { timeout });
  } catch {
    fail(site, message);
  }
};

const resultRequests = (page) => {
  const requests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.pathname === RESULT_PATH) requests.push({ method: request.method(), url });
  });
  return requests;
};

const assertResultRequest = (site, requests, query) => {
  if (requests.length !== 1) fail(site, `result requests = ${requests.length}, want 1`);
  const [request] = requests;
  if (request.method !== 'GET' || request.url.pathname !== RESULT_PATH || request.url.searchParams.get('q') !== query) {
    fail(site, `request was ${request.method} ${request.url.pathname}${request.url.search}, want GET ${RESULT_PATH}?q=${query}`);
  }
};

const waitForCount = (page, site, scope, count) => waitFor(
  page,
  site,
  ({ scope, count }) => document.querySelector(scope)?.querySelector('[data-live-search-results]')?.dataset.resultCount === count,
  { scope, count: String(count) },
  `${scope} did not settle with ${count} result(s)`,
);

const assertPainted = async (site, locator, label) => {
  const painted = await locator.evaluate(async (element) => {
    const inspect = () => {
      const rect = element.getBoundingClientRect();
      const clips = [];
      let left = Math.max(0, rect.left);
      let top = Math.max(0, rect.top);
      let right = Math.min(innerWidth, rect.right);
      let bottom = Math.min(innerHeight, rect.bottom);
      let opacity = 1;

      for (let current = element; current; current = current.parentElement) {
        const style = getComputedStyle(current);
        if (style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse') {
          return { visible: false, reason: `${current.tagName.toLowerCase()} is ${style.display}/${style.visibility}` };
        }
        opacity *= Number.parseFloat(style.opacity);
        if (current !== element && /(auto|scroll|hidden|clip)/.test(`${style.overflowX} ${style.overflowY}`)) {
          const clip = current.getBoundingClientRect();
          clips.push(`${current.tagName.toLowerCase()}.${current.className || '-'}=[${clip.left},${clip.top},${clip.right},${clip.bottom}] ${style.overflowX}/${style.overflowY}`);
          left = Math.max(left, clip.left);
          top = Math.max(top, clip.top);
          right = Math.min(right, clip.right);
          bottom = Math.min(bottom, clip.bottom);
        }
      }

      if (!(opacity > 0)) return { visible: false, reason: `cumulative opacity is ${opacity}` };
      if (!(right > left && bottom > top)) {
        return { visible: false, reason: `painted area [${rect.left},${rect.top},${rect.right},${rect.bottom}] was clipped or empty by ${clips.join('; ')}` };
      }
      const hit = document.elementFromPoint((left + right) / 2, (top + bottom) / 2);
      if (!hit || !(hit === element || element.contains(hit))) {
        return { visible: false, reason: `center hit ${hit?.tagName.toLowerCase() || 'nothing'}` };
      }
      return { visible: true, reason: '' };
    };

    let result = inspect();
    for (let frame = 0; !result.visible && frame < 90; frame += 1) {
      await new Promise((resolve) => requestAnimationFrame(resolve));
      result = inspect();
    }
    return result;
  });
  if (!painted.visible) fail(site, `${label} was in the DOM but not visibly painted: ${painted.reason}`);
};

const exerciseScope = async (browser, site, { path = PAGE, scope, openDialog = false, syncsURL = false }) => {
  const { context, page } = await start(browser, site, { path });
  try {
    if (openDialog) {
      await page.keyboard.press('ControlOrMeta+k');
      await page.locator(`${scope}[open]`).waitFor({ state: 'visible' });
    }
    const initialURL = page.url();
    const requests = resultRequests(page);
    const input = page.locator(`${scope} [data-live-search-input]`);
    await input.fill(QUERY);
    await waitForCount(page, site, scope, 1);
    assertResultRequest(site, requests, QUERY);
    if (syncsURL) {
      // The search page rewrites its own address after a live refresh; the
      // settled URL is exactly what submitting the form would have produced.
      const settled = new URL(initialURL);
      settled.searchParams.set('q', QUERY);
      if (page.url() !== settled.href) fail(site, `${scope} settled at ${page.url()}, want ${settled.href}`);
    } else if (page.url() !== initialURL) {
      fail(site, `${scope} navigated while producing live results`);
    }
    if (await input.inputValue() !== QUERY) fail(site, `${scope} replaced the active input`);
    // Matched by prefix: a result link carries the words the query found as a
    // text directive, so the note's address is where its href starts rather
    // than the whole of it.
    const resultLink = page.locator(`${scope} a[href^="/notes/Notes/alpha.md"]`);
    if (await resultLink.count() !== 1) {
      fail(site, `${scope} did not render the tortoise result link`);
    }
    await assertPainted(site, resultLink, `${scope} tortoise result link`);
  } finally {
    await context.close();
  }
};

const withDeadline = (promise, message, timeout = 3000) => Promise.race([
  promise,
  new Promise((_, reject) => setTimeout(() => reject(new Error(message)), timeout)),
]);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Home top at the viewport that reproduced the autofocus scroll.
  {
    const site = 'home-starts-at-top';
    const { context, page } = await start(browser, site);
    try {
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      // The document is the scroller for prose, so arriving at the top is a
      // fact about the document. Asking .y-main would answer zero whatever
      // happened, because it no longer scrolls at all.
      const scrollTop = await page.evaluate(() => document.scrollingElement.scrollTop);
      if (scrollTop !== 0) fail(site, `document.scrollTop = ${scrollTop}, want 0 at 1270x720`);
    } finally {
      await context.close();
    }
  }

  // Home opens on the ways into the library, not on a text field: avoiding
  // scroll is not enough if a script still steals focus with preventScroll.
  // The search field must remain unfocused.
  {
    const site = 'home-does-not-take-focus';
    const scope = '[data-home-block="search"]';
    const { context, page } = await start(browser, site);
    try {
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      const focused = await page.locator(`${scope} input[name="q"]`).evaluate((element) => document.activeElement === element);
      if (focused) fail(site, 'the Home search input took initial focus');
    } finally {
      await context.close();
    }
  }

  // Home is intentionally not a live-search surface. Cross the same debounce
  // window used by the enhanced fields and prove typing stays local: no
  // fragment request, no result region, no navigation.
  {
    const site = 'home-remains-plain-get';
    const scope = '[data-home-block="search"]';
    const { context, page } = await start(browser, site, { clock: true });
    try {
      const initialURL = page.url();
      const requests = resultRequests(page);
      await page.locator(`${scope} input[type="search"]`).fill(QUERY);
      await page.clock.runFor(DEBOUNCE_MS);
      await page.clock.resume();
      await page.waitForTimeout(500);
      if (requests.length !== 0) fail(site, `Home issued ${requests.length} live-result request(s), want 0`);
      if (await page.locator(`${scope} [data-live-search], ${scope} [data-live-search-results], ${scope} [data-live-search-status]`).count() !== 0) {
        fail(site, 'Home gained live-search hooks or a dynamic result region');
      }
      if (page.url() !== initialURL) fail(site, 'Home navigated from input alone');
    } finally {
      await context.close();
    }
  }

  await exerciseScope(browser, 'page-live-results', {
    path: '/search',
    scope: '.y-searchpage[data-live-search]',
    syncsURL: true,
  });
  await exerciseScope(browser, 'dialog-live-results', {
    scope: 'dialog[data-search][data-live-search]',
    openDialog: true,
  });

  // A copied address must reproduce the results on screen, so the page's URL
  // follows each settled live refresh — rewritten in place, never pushed, and
  // never by navigating. The dialog's exercise above locks the other half:
  // typing into the palette leaves the page it floats over untouched.
  {
    const site = 'page-url-follows-results';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: '/search' });
    try {
      const depthBefore = await page.evaluate(() => {
        window.__urlSyncDocumentMarker = true;
        return history.length;
      });
      await page.locator(`${scope} [data-live-search-input]`).fill(QUERY);
      await waitForCount(page, site, scope, 1);
      await waitFor(
        page,
        site,
        (query) => new URL(location.href).searchParams.get('q') === query,
        QUERY,
        'the address bar never received the settled live query',
      );
      const url = new URL(page.url());
      if (url.pathname !== '/search' || url.search !== `?q=${QUERY}`) {
        fail(site, `live URL settled at ${url.pathname}${url.search}, want /search?q=${QUERY}`);
      }
      const after = await page.evaluate(() => ({
        depth: history.length,
        sameDocument: window.__urlSyncDocumentMarker === true,
      }));
      if (!after.sameDocument) fail(site, 'the URL sync navigated to a new document instead of rewriting in place');
      if (after.depth !== depthBefore) {
        fail(site, `history depth ${depthBefore} -> ${after.depth}: the live query was pushed, not replaced`);
      }
    } finally {
      await context.close();
    }
  }

  // The eight input events share one trailing timer. Advancing to one
  // millisecond before the literal contract must issue nothing; the last
  // millisecond issues exactly one request.
  {
    const site = 'trailing-debounce';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: '/search', clock: true });
    try {
      const requests = resultRequests(page);
      await page.locator(`${scope} [data-live-search-input]`).pressSequentially(QUERY);
      await page.clock.runFor(DEBOUNCE_MS - 1);
      if (requests.length !== 0) fail(site, `issued ${requests.length} request(s) before ${DEBOUNCE_MS}ms settled`);
      await page.clock.runFor(1);
      await page.clock.resume();
      await waitForCount(page, site, scope, 1);
      assertResultRequest(site, requests, QUERY);
    } finally {
      await context.close();
    }
  }

  // Composition input is provisional. It must not query until compositionend,
  // which then enters the same single trailing-debounce path.
  {
    const site = 'ime-waits-for-composition';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: '/search', clock: true });
    try {
      const requests = resultRequests(page);
      const input = page.locator(`${scope} [data-live-search-input]`);
      await input.evaluate((element, query) => {
        element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true, data: query }));
        element.value = query;
        element.dispatchEvent(new InputEvent('input', {
          bubbles: true,
          data: query,
          inputType: 'insertCompositionText',
          isComposing: true,
        }));
      }, QUERY);
      await page.clock.runFor(DEBOUNCE_MS);
      if (requests.length !== 0) fail(site, `issued ${requests.length} request(s) before compositionend`);
      await input.evaluate((element, query) => {
        element.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: query }));
      }, QUERY);
      await page.clock.runFor(DEBOUNCE_MS);
      await page.clock.resume();
      await waitForCount(page, site, scope, 1);
      assertResultRequest(site, requests, QUERY);
    } finally {
      await context.close();
    }
  }

  // A superseded in-flight request must receive an abort signal. The request-ID
  // guard below protects the DOM too, but it is not a substitute for cancelling
  // work the reader no longer wants.
  {
    const site = 'superseded-request-is-aborted';
    const scope = 'dialog[data-search][data-live-search]';
    const { context, page } = await start(browser, site, {
      initScript: () => {
        const nativeFetch = window.fetch.bind(window);
        window.__searchAbortState = { started: false, aborted: false };
        window.fetch = (resource, options = {}) => {
          const requestURL = new URL(resource instanceof Request ? resource.url : String(resource), location.href);
          if (requestURL.pathname === '/search/results' && requestURL.searchParams.get('q') === 'beta') {
            window.__searchAbortState.started = true;
            return new Promise((_, reject) => {
              const abort = () => {
                window.__searchAbortState.aborted = true;
                reject(new DOMException('The operation was aborted.', 'AbortError'));
              };
              if (options.signal?.aborted) abort();
              else options.signal?.addEventListener('abort', abort, { once: true });
            });
          }
          return nativeFetch(resource, options);
        };
      },
    });
    try {
      await page.keyboard.press('ControlOrMeta+k');
      const input = page.locator(`${scope} [data-live-search-input]`);
      await input.fill('beta');
      await waitFor(page, site, () => window.__searchAbortState?.started, undefined, 'the beta request never started');
      await input.fill(QUERY);
      await waitFor(page, site, () => window.__searchAbortState?.aborted, undefined, 'the superseded beta request was not aborted');
    } finally {
      await context.close();
    }
  }

  // Hold an older response in a fetch wrapper that has already won the network
  // race and deliberately ignores the later abort. The monotonic request ID is
  // the independent final guard that keeps it out of the current DOM.
  {
    const site = 'latest-query-wins';
    const scope = 'dialog[data-search][data-live-search]';
    const { context, page } = await start(browser, site, {
      initScript: () => {
        const nativeFetch = window.fetch.bind(window);
        let release;
        const gate = new Promise((resolve) => { release = resolve; });
        window.__staleResponseState = { started: false, released: false };
        window.__releaseStaleResponse = () => release();
        window.fetch = async (resource, options = {}) => {
          const requestURL = new URL(resource instanceof Request ? resource.url : String(resource), location.href);
          if (requestURL.pathname === '/search/results' && requestURL.searchParams.get('q') === 'beta') {
            const response = await nativeFetch(resource, { ...options, signal: undefined });
            window.__staleResponseState.started = true;
            await gate;
            window.__staleResponseState.released = true;
            return response;
          }
          return nativeFetch(resource, options);
        };
      },
    });
    try {
      await page.keyboard.press('ControlOrMeta+k');
      const input = page.locator(`${scope} [data-live-search-input]`);
      await input.fill('beta');
      await waitFor(page, site, () => window.__staleResponseState?.started, undefined, 'the delayed beta response never arrived');
      await input.fill(QUERY);
      await waitForCount(page, site, scope, 1);
      await page.evaluate(() => window.__releaseStaleResponse());
      await waitFor(page, site, () => window.__staleResponseState?.released, undefined, 'the delayed beta response never resumed');
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      const results = page.locator(`${scope} [data-live-search-results]`);
      if (await results.getAttribute('data-result-count') !== '1') fail(site, 'the stale beta count replaced the tortoise count');
      if (await results.locator('a[href^="/notes/Notes/beta.md"]').count() !== 0) fail(site, 'the stale beta response replaced the current results');
    } finally {
      await page.evaluate(() => window.__releaseStaleResponse?.()).catch(() => {});
      await context.close();
    }
  }

  {
    const site = 'settled-count';
    const scope = 'dialog[data-search][data-live-search]';
    const { context, page, armed } = await start(browser, site);
    try {
      await page.keyboard.press('ControlOrMeta+k');
      await page.locator(`${scope} [data-live-search-input]`).fill(QUERY);
      await waitFor(
        page,
        site,
        (scope) => document.querySelector(scope)?.querySelector('[data-live-search-status]')?.textContent !== '',
        scope,
        'the settled result count was never announced',
      );
      armed.prove('action');
      const status = page.locator(`${scope} [data-live-search-status]`);
      if (await status.getAttribute('role') !== 'status') fail(site, 'settled count has no status role');
      if (await status.getAttribute('aria-live') !== 'polite') fail(site, 'settled count is not polite');
      if (await status.getAttribute('aria-atomic') !== 'true') fail(site, 'settled count is not atomic');
      const text = await status.textContent();
      if (text !== '「tortoise」有 1 筆結果。') fail(site, `settled count = ${JSON.stringify(text)}, want the literal one-result announcement`);
    } finally {
      await context.close();
    }
  }

  // A failed enhancement must leave the last useful results in place and tell
  // the reader exactly how to recover through the still-native GET form.
  {
    const site = 'live-error-recovery';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: '/search' });
    await page.route('**/search/results?*', async (route) => {
      const url = new URL(route.request().url());
      if (url.searchParams.get('q') === 'offline') {
        await route.fulfill({ status: 503, contentType: 'text/plain', body: 'offline' });
      } else {
        await route.continue();
      }
    });
    try {
      const input = page.locator(`${scope} [data-live-search-input]`);
      await input.fill(QUERY);
      await waitForCount(page, site, scope, 1);
      await input.fill('offline');
      const status = page.locator(`${scope} [data-live-search-status]`);
      await waitFor(
        page,
        site,
        (scope) => document.querySelector(scope)?.querySelector('[data-live-search-status]')?.textContent === '即時結果目前無法使用；按 Enter 執行完整搜尋。',
        scope,
        'the actionable live-search recovery message was not announced',
      );
      if (await status.getAttribute('role') !== 'status' || await status.getAttribute('aria-live') !== 'polite' || await status.getAttribute('aria-atomic') !== 'true') {
        fail(site, 'the live-search recovery message is not an atomic polite status');
      }
      const results = page.locator(`${scope} [data-live-search-results]`);
      if (await results.getAttribute('aria-busy') !== 'false') fail(site, 'failed live search left the result region busy');
      if (await results.getAttribute('data-result-count') !== '1' || await results.locator('a[href^="/notes/Notes/alpha.md"]').count() !== 1) {
        fail(site, 'failed live search discarded the last useful results');
      }
      // Kept rows are the previous query's answer, and they now sit under a
      // sentence asking the reader to search again. Unlabelled they read as an
      // answer to what is in the box, so the set has to name the query it does
      // answer, in words and in a value a check can compare.
      const stale = results.locator('[data-live-search-stale]');
      const staleCount = await stale.count();
      if (staleCount !== 1) fail(site, `the kept rows carry ${staleCount} query labels, want 1`);
      if (await stale.isHidden()) fail(site, 'the kept rows carry no visible note saying which query they answer');
      if (await stale.getAttribute('data-live-search-stale') !== QUERY) {
        fail(site, `the kept rows are labelled for ${await stale.getAttribute('data-live-search-stale')}, want ${QUERY}`);
      }
      const staleText = (await stale.textContent()) || '';
      if (!staleText.includes(QUERY)) fail(site, `the kept rows' note reads ${JSON.stringify(staleText)}, which does not name ${QUERY}`);
    } finally {
      await context.close();
    }
  }

  // A hit is a place inside a note, not just the note. The result link carries
  // the words the query found as a text directive, and the browser opens the
  // note at them; the proof is that the document is no longer at its top. Not
  // :target — a text directive sets none, so reading for one would go red for
  // the wrong reason and green for no reason.
  //
  // Both surfaces a reader can click a result from are exercised, because they
  // are different documents: the server-rendered page, and the region the live
  // search imports into the page it is already on.
  {
    const site = 'hit-opens-at-the-match';
    const { context, page } = await start(browser, site, { path: `/search?q=${HIT_QUERY}` });
    try {
      const link = page.locator(`.y-searchpage a.y-result[href="${HIT_DIRECTIVE}"]`);
      if (await link.count() !== 1) {
        const hrefs = await page.locator('.y-searchpage a.y-result').evaluateAll((as) => as.map((a) => a.getAttribute('href')));
        fail(site, `the server-rendered result does not point at the match; hrefs = ${JSON.stringify(hrefs)}`);
      }
      await link.click();
      await page.waitForURL((url) => new URL(url).pathname === HIT_NOTE, { timeout: 5000 });
      await page.waitForLoadState('load');
      await waitFor(
        page,
        site,
        () => document.scrollingElement.scrollTop > 0,
        undefined,
        'the note opened at its top rather than at the words the query found',
      );
    } finally {
      await context.close();
    }
  }

  {
    const site = 'live-hit-opens-at-the-match';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page, armed } = await start(browser, site, { path: '/search' });
    try {
      await page.locator(`${scope} [data-live-search-input]`).fill(HIT_QUERY);
      await waitForCount(page, site, scope, 1);
      armed.prove('action');
      const link = page.locator(`${scope} [data-live-search-results] a.y-result[href="${HIT_DIRECTIVE}"]`);
      if (await link.count() !== 1) {
        const hrefs = await page.locator(`${scope} [data-live-search-results] a.y-result`).evaluateAll((as) => as.map((a) => a.getAttribute('href')));
        fail(site, `the imported result does not point at the match; hrefs = ${JSON.stringify(hrefs)}`);
      }
      await link.click();
      await page.waitForURL((url) => new URL(url).pathname === HIT_NOTE, { timeout: 5000 });
      await page.waitForLoadState('load');
      await waitFor(
        page,
        site,
        () => document.scrollingElement.scrollTop > 0,
        undefined,
        'a hit clicked out of the live results opened the note at its top',
      );
    } finally {
      await context.close();
    }
  }

  // The note has to follow the box in both directions. A reader who types their
  // way back to the query the kept rows do answer is looking at the current
  // answer again, and a note still calling it an earlier one is the same false
  // claim the note exists to end, said the other way round.
  {
    const site = 'stale-label-follows-the-box';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: '/search' });
    const refused = new Set();
    await page.route('**/search/results?*', async (route) => {
      if (refused.has(new URL(route.request().url()).searchParams.get('q'))) {
        await route.fulfill({ status: 503, contentType: 'text/plain', body: 'offline' });
      } else {
        await route.continue();
      }
    });
    try {
      const input = page.locator(`${scope} [data-live-search-input]`);
      const note = page.locator(`${scope} [data-live-search-results] [data-live-search-stale]`);
      await input.fill(QUERY);
      await waitForCount(page, site, scope, 1);
      if (await note.isVisible()) fail(site, 'the current answer was labelled as an earlier one');

      refused.add('offline');
      await input.fill('offline');
      await waitFor(page, site, () => {
        const el = document.querySelector('.y-searchpage [data-live-search-results] [data-live-search-stale]');
        return el !== null && !el.hidden;
      }, undefined, 'the rows kept after a failed refresh were never labelled');

      // Back to the query those rows answer, still with the endpoint down.
      refused.add(QUERY);
      await input.fill(QUERY);
      await waitFor(page, site, () => {
        const el = document.querySelector('.y-searchpage [data-live-search-results] [data-live-search-stale]');
        return el !== null && el.hidden;
      }, undefined, 'the rows kept on answering the query in the box while being called an earlier answer');
    } finally {
      await context.close();
    }
  }

  // An address may pad the query the field then trims, so the same search
  // reaches the note under two spellings. One search is one search.
  {
    const site = 'stale-label-reads-a-padded-address';
    const scope = '.y-searchpage[data-live-search]';
    const { context, page } = await start(browser, site, { path: `/search?q=%20${QUERY}` });
    await page.route('**/search/results?*', (route) => route.fulfill({ status: 503, contentType: 'text/plain', body: 'offline' }));
    try {
      const input = page.locator(`${scope} [data-live-search-input]`);
      const note = page.locator(`${scope} [data-live-search-results] [data-live-search-stale]`);
      if (await note.count() !== 1) fail(site, `the padded address rendered ${await note.count()} query labels, want 1`);

      // The control: a different query does leave the served rows behind.
      await input.fill(`${QUERY}x`);
      await waitFor(page, site, () => {
        const el = document.querySelector('.y-searchpage [data-live-search-results] [data-live-search-stale]');
        return el !== null && !el.hidden;
      }, undefined, 'a genuinely earlier answer was not labelled');

      await input.fill(` ${QUERY}`);
      await waitFor(page, site, () => {
        const el = document.querySelector('.y-searchpage [data-live-search-results] [data-live-search-stale]');
        return el !== null && el.hidden;
      }, undefined, 'the padding in the address made one search read as two');
    } finally {
      await context.close();
    }
  }

  {
    const site = 'enter-submits-get';
    const scope = 'dialog[data-search][data-live-search]';
    const { context, page } = await start(browser, site);
    try {
      await page.keyboard.press('ControlOrMeta+k');
      await page.locator(`${scope} [data-live-search-input]`).fill(QUERY);
      const requestPromise = page.waitForRequest((request) => request.isNavigationRequest() && new URL(request.url()).pathname === '/search');
      await page.locator(`${scope} [data-live-search-input]`).press('Enter');
      let request;
      try {
        request = await withDeadline(requestPromise, 'Enter did not submit the search form');
      } catch {
        fail(site, 'Enter did not submit the search form');
      }
      const url = new URL(request.url());
      if (request.method() !== 'GET' || url.searchParams.get('q') !== QUERY) fail(site, `Enter submitted ${request.method()} ${url.pathname}${url.search}, want native GET`);
    } finally {
      await context.close();
    }
  }

  {
    const site = 'button-submits-get';
    const scope = '[data-home-block="search"]';
    const { context, page } = await start(browser, site);
    try {
      await page.locator(`${scope} input[type="search"]`).fill(QUERY);
      const requestPromise = page.waitForRequest((request) => request.isNavigationRequest() && new URL(request.url()).pathname === '/search');
      await page.locator(`${scope} button`).click();
      let request;
      try {
        request = await withDeadline(requestPromise, 'Search button did not submit the form');
      } catch {
        fail(site, 'Search button did not submit the form');
      }
      const url = new URL(request.url());
      if (request.method() !== 'GET' || url.searchParams.get('q') !== QUERY) fail(site, `button submitted ${request.method()} ${url.pathname}${url.search}, want native GET`);
    } finally {
      await context.close();
    }
  }

  {
    const site = 'no-js-get';
    const { context, page } = await start(browser, site, { path: '/search', javaScriptEnabled: false });
    try {
      await page.locator('.y-searchpage [data-live-search-input]').fill(QUERY);
      const requestPromise = page.waitForRequest((request) => request.isNavigationRequest() && new URL(request.url()).pathname === '/search');
      await page.locator('.y-searchpage button').click();
      let request;
      try {
        request = await withDeadline(requestPromise, 'no-JS form did not navigate');
      } catch {
        fail(site, 'no-JS form did not navigate');
      }
      const url = new URL(request.url());
      if (request.method() !== 'GET' || url.searchParams.get('q') !== QUERY) fail(site, `no-JS form submitted ${request.method()} ${url.pathname}${url.search}, want GET`);
    } finally {
      await context.close();
    }
  }

  console.log('PASS search-behavior: Home top/focus/plain GET; two painted live scopes; page URL sync; debounce; abort/stale guards; count/error status; the kept-rows label follows the box; hits open at the match on both surfaces; native and no-JS GET');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (err.site === target && mutationApplied) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else if (err.site === target) console.error(`no catch: ${MUTATE} reached ${target}, but the mutation was not proved applied`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
    }
    process.exitCode = 1;
  } else if (err instanceof ProbeBroken) {
    console.error(err.message);
    process.exitCode = 1;
  } else {
    console.error(err);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
