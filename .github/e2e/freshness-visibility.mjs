// Behavior lock for the freshness watch's visibility gate. A hidden page does
// not poll: that is what the visibilitychange branch is for. Starting the
// watch only when the document is already visible is what keeps a tab that
// loads in the background quiet, because visibilitychange fires only on a
// change.
//
// A back/forward-cache restore stops on pagehide and starts again only if
// visibilitychange fires on the way back. Chrome fires that event before
// pageshow for a restore into a visible tab — measured here on a real restore
// when Chrome will give one, and locked by the resume that listener performs.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note the watch can ask about), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const HIDDEN_WINDOW_MS = 6200;

const SITES = [
  'hidden-at-load-does-not-poll',
  'restore-into-visible-resumes',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN freshness-visibility: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL freshness-visibility: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN freshness-visibility: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED freshness-visibility: ${message}`); };

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/freshness.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'start-unconditionally': {
    target: 'hidden-at-load-does-not-poll',
    apply: rewriteModule(
      "  if (document.visibilityState === 'visible') {\n    start();\n    tick();\n  }\n",
      '  start();\n  tick();\n',
      'visibility start guard',
    ),
  },
  'drop-visibilitychange-listener': {
    target: 'restore-into-visible-resumes',
    apply: rewriteModule(
      "  document.addEventListener('visibilitychange', () => {\n    if (document.visibilityState !== 'visible') {\n      stop();\n      return;\n    }\n    start();\n    tick();\n  });\n",
      '',
      'visibilitychange listener',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`freshness-visibility: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`freshness-visibility: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`freshness-visibility: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const isFreshness = (url) => {
  try {
    return new URL(url).pathname.startsWith('/freshness/');
  } catch {
    return false;
  }
};

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const applyMutation = async (page) => {
  if (!MUTATE) return () => '';
  return MUTATIONS[MUTATE].apply(page);
};

const checkProof = (proof) => {
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const browser = await chromium.launch({
  channel: 'chrome',
  headless: true,
  args: ['--disable-features=BackForwardCacheMemoryControls'],
});
try {
  // --- Hidden at load -------------------------------------------------
  // The production guard reads document.visibilityState. A tab that loads
  // already hidden never receives visibilitychange, so the getter has to
  // say hidden before the module runs. Overriding the property on every
  // document in this context is what makes that load-time state hold under
  // headless Chrome, which otherwise paints every page visible.
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page);
    await page.addInitScript(() => {
      Object.defineProperty(document, 'visibilityState', {
        configurable: true,
        get: () => 'hidden',
      });
      Object.defineProperty(document, 'hidden', {
        configurable: true,
        get: () => true,
      });
    });
    const polls = [];
    page.on('request', (request) => {
      if (isFreshness(request.url())) polls.push(request.url());
    });
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    checkProof(proof);
    const visibility = await page.evaluate(() => document.visibilityState);
    if (visibility !== 'hidden') {
      broken(`hidden-at-load tab reported visibilityState=${visibility}, want hidden`);
    }
    const watching = await page.evaluate(() =>
      Boolean(document.querySelector('[data-freshness-path][data-freshness-identity]')));
    if (!watching) broken('the note page carries no freshness watch to start');
    await wait(HIDDEN_WINDOW_MS);
    if (polls.length !== 0) {
      fail(
        'hidden-at-load-does-not-poll',
        `a tab hidden at load issued ${polls.length} /freshness/ request(s) across ${HIDDEN_WINDOW_MS}ms`,
      );
    }
    await context.close();
  }

  // --- Chrome restore order, measured on a page nobody rewrites --------
  // The watch stops on pagehide and starts only on visibilitychange. A
  // restore into a visible tab therefore lives or dies on Chrome firing
  // that change before pageshow. The measurement uses no request rewrite:
  // intercepting the module is itself a reason Chrome refuses the cache.
  let chromeRestore = false;
  let chromeRestoreEvents = [];
  let chromeRestoreReasons = null;
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    await page.addInitScript(() => {
      window.__lifecycle = [];
      const record = (type, extra = {}) => {
        window.__lifecycle.push({
          type,
          visibility: document.visibilityState,
          persisted: extra.persisted ?? null,
          t: performance.now(),
        });
      };
      document.addEventListener('visibilitychange', () => record('visibilitychange'));
      window.addEventListener('pageshow', (event) => record('pageshow', { persisted: event.persisted }));
      window.addEventListener('pagehide', (event) => record('pagehide', { persisted: event.persisted }));
    });
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await wait(400);
    await page.goto(`${BASE}/`, { waitUntil: 'domcontentloaded' });
    await page.goBack({ waitUntil: 'domcontentloaded' });
    const measured = await page.evaluate(() => {
      const events = window.__lifecycle || [];
      const nav = performance.getEntriesByType('navigation')[0];
      return {
        events,
        persistedShow: events.some((event) => event.type === 'pageshow' && event.persisted === true),
        reasons: nav?.notRestoredReasons ?? null,
      };
    });
    chromeRestore = measured.persistedShow;
    chromeRestoreEvents = measured.events;
    chromeRestoreReasons = measured.reasons;
    if (chromeRestore) {
      const firstPersisted = chromeRestoreEvents.find((event) => event.type === 'pageshow' && event.persisted === true);
      const visibleChangeBefore = chromeRestoreEvents.find((event) =>
        event.type === 'visibilitychange'
        && event.visibility === 'visible'
        && event.t <= firstPersisted.t);
      if (!visibleChangeBefore && !MUTATE) {
        fail(
          'restore-into-visible-resumes',
          `Chrome restored the note but fired no visibilitychange to visible before pageshow (events=${JSON.stringify(chromeRestoreEvents)})`,
        );
      }
    }
    await context.close();
  }

  // --- Restore into a visible tab -------------------------------------
  // Driven on the same document: hide stops the watch, show starts it
  // again. That is the visibilitychange a restore into a visible tab
  // fires, and it is the only door left after pagehide. A goBack that
  // reloads would re-run the load-time start and would not be a restore.
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page);

    const polls = [];
    page.on('request', (request) => {
      if (isFreshness(request.url())) polls.push(request.url());
    });

    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    checkProof(proof);

    const column = await page.evaluate(() => {
      const node = document.querySelector('[data-freshness-path][data-freshness-identity]');
      return node ? node.dataset.freshnessPath : '';
    });
    if (!column) broken('the note page carries no freshness watch to start');

    try {
      await page.waitForEvent('request', {
        predicate: (request) => isFreshness(request.url()),
        timeout: 4000,
      });
    } catch {
      broken('a visible note never asked /freshness/, so a restore resume would prove nothing');
    }

    await page.evaluate(() => {
      Object.defineProperty(document, 'visibilityState', {
        configurable: true,
        get: () => 'hidden',
      });
      document.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
    });

    const waiter = page.waitForEvent('request', {
      predicate: (request) => isFreshness(request.url()),
      timeout: 4000,
    });
    await page.evaluate(() => {
      Object.defineProperty(document, 'visibilityState', {
        configurable: true,
        get: () => 'visible',
      });
      document.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
    });
    try {
      await waiter;
    } catch {
      fail(
        'restore-into-visible-resumes',
        `restore into a visible tab issued no /freshness/ request (chromeRestore=${chromeRestore}, reasons=${JSON.stringify(chromeRestoreReasons)})`,
      );
    }

    console.log(
      `PASS freshness-visibility: hidden-at-load issued 0 /freshness/ requests; restore into a visible tab resumed`
      + ` (chromeRestore=${chromeRestore}, events=${JSON.stringify(chromeRestoreEvents)})`,
    );
    await context.close();
  }
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
