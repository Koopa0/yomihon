// Behavior lock for the pages the browser fetches ahead. A page is written
// under the cookies of the moment it was fetched, so a choice the reader makes
// afterwards would come back undone on the next page: the theme chosen here,
// and the page after it still in the old one. The preferences module takes the
// speculation rules out of the document when a cookie changes, the browser
// drops what it fetched, and the next page is asked for under the new choice.
//
// Two things are held. With nothing changed, the next page is served from the
// prefetch, which is also what shows the browser here fetches ahead at all: a
// harness that did not would let the second assertion pass for the wrong
// reason. And after the theme is toggled, the next page arrives in the theme
// the reader chose and was not served from a prefetch.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a next link), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['next-page-is-served-from-the-prefetch', 'chosen-theme-reaches-the-next-page'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN prefetch-stale-preference: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL prefetch-stale-preference: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN prefetch-stale-preference: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED prefetch-stale-preference: ${message}`); };

// Rewrites what a route serves and reports, once the page has loaded, whether
// the needle was there to rewrite.
const rewrite = (glob, needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route(glob, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.split(needle).join(replacement) });
  });
  return () => (matches < 1 ? `${label} needle matched ${matches} times, want at least 1` : '');
};

const MUTATIONS = {
  // The page carries no rules, so the browser fetches nothing ahead.
  'strip-the-rules': {
    target: 'next-page-is-served-from-the-prefetch',
    apply: async (page) => {
      let matches = 0;
      await page.route('**/notes/**', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        const stripped = original.replace(/<script type="speculationrules"[^>]*>[\s\S]*?<\/script>/, () => {
          matches += 1;
          return '';
        });
        await route.fulfill({ response, body: stripped });
      });
      return () => (matches < 1 ? 'speculation rules element matched 0 times, want at least 1' : '');
    },
  },
  // A change to a cookie no longer takes the rules away.
  'keep-the-rules-after-a-change': {
    target: 'chosen-theme-reaches-the-next-page',
    apply: rewrite(
      '**/preferences.js{,?*}',
      `document.querySelector('script[type="speculationrules"]')?.remove();`,
      `document.querySelector('script[type="speculationrules"]');`,
      'rule removal',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`prefetch-stale-preference: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`prefetch-stale-preference: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`prefetch-stale-preference: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// Opens PAGE in a fresh context, waits for the browser to hold the next page's
// prefetch, and hands back what a click on the next link then shows.
async function visit(browser, beforeClick) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  try {
    const page = await context.newPage();
    const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    const cdp = await context.newCDPSession(page);
    const states = [];
    cdp.on('Preload.prefetchStatusUpdated', (event) => states.push(event.status));
    await cdp.send('Preload.enable');
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const next = page.locator('a[rel="next"]').first();
    if ((await next.count()) === 0) broken(`${PAGE} carries no next link to click`);
    for (let waited = 0; waited < 5000 && !states.includes('Ready'); waited += 100) await sleep(100);
    await beforeClick(page);
    const here = page.url();
    await Promise.all([page.waitForURL((url) => url.href !== here), next.click()]);
    await page.waitForLoadState('load');
    return await page.evaluate(() => ({
      path: location.pathname,
      theme: document.documentElement.dataset.theme ?? '',
      delivery: performance.getEntriesByType('navigation')[0]?.deliveryType ?? '',
    }));
  } finally {
    await context.close();
  }
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const start = new URL(BASE + PAGE).pathname;

  const untouched = await visit(browser, async () => {});
  if (untouched.path === start) broken('the click on the next link did not leave the page');
  if (untouched.delivery !== 'navigational-prefetch') {
    fail('next-page-is-served-from-the-prefetch', `with nothing changed the next page was delivered as ${JSON.stringify(untouched.delivery)}, want "navigational-prefetch"`);
  }

  const toggled = await visit(browser, async (page) => {
    if (!(await page.locator('[data-theme-toggle]').isVisible())) await page.click('[popovertarget="_y-header-fold"]');
    await page.click('[data-theme-toggle]');
    await page.waitForFunction(() => !document.querySelector('script[type="speculationrules"]'), null, { timeout: 1500 }).catch(() => {});
  });
  if (toggled.path === start) broken('the click on the next link did not leave the page');
  if (toggled.theme !== 'dark') {
    fail('chosen-theme-reaches-the-next-page', `after choosing dark the next page arrived with theme ${JSON.stringify(toggled.theme)}, delivered as ${JSON.stringify(toggled.delivery)}, want "dark"`);
  }

  console.log('PASS prefetch-stale-preference: the next page is served from the prefetch, and a theme chosen after it arrives on the next page');
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
