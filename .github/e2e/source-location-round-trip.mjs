// Behavior lock for a claim's declared source locations, followed the way a
// reader follows them: from the claim to the Methods passage of its source,
// back to the claim, then on to the Limitations passage, and a section the
// source no longer has keeping its address as written and opening the source
// from the top with its reason in view. It
// runs at a wide and a phone width, where the list sits in the rail and in a
// disclosure respectively.
//
// The server-rendered half of the promise (anchors checked against the
// captured generation, authored order, fallback words) is held by Go tests;
// what only a browser shows is that the links land on the passage and that
// Back returns to the claim. The mutation modes rewrite bytes as they are
// served and touch no file on disk.
//
// Env: YOMIHON_BASE, PAGE_PATH (the claim note), and MUTATE.
import { chromium } from 'playwright-core';

const arrived = (page) => page.waitForFunction(
  async () => {
    if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return true;
  },
  null,
  { timeout: 3000 },
);

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/source-locations-claim.md';
const SOURCE = '/notes/Notes/source-locations-source.md';
const MUTATE = process.env.MUTATE || '';

const SITES = ['fragment-methods', 'fragment-limitations', 'return', 'fallback-address', 'fallback-reason'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN source-location-round-trip: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL source-location-round-trip: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN source-location-round-trip: ${message}`); };

// rewrite serves the matched GET responses with every occurrence of needle
// replaced, and reports whether the needle was met the number of times the
// page is known to carry it. The list is drawn twice, once for the wide rail
// and once for the narrow disclosure.
const rewrite = (pattern, needle, replacement, expected, label) => async (page) => {
  let matches = -1;
  await page.route(pattern, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    const response = await route.fetch();
    const original = await response.text();
    if (matches < 0) matches = original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => (matches === expected ? '' : `${label} needle matched ${matches} times, want ${expected}`);
};

const CLAIM_ROUTE = `${BASE}${PAGE}*`;
const SOURCE_LINK = `href="${SOURCE}`;

const MUTATIONS = {
  'drop-methods-fragment': {
    target: 'fragment-methods',
    apply: rewrite(CLAIM_ROUTE, `${SOURCE_LINK}#methods"`, `${SOURCE_LINK}"`, 2, 'Methods location link'),
  },
  'swap-limitations-target': {
    target: 'fragment-limitations',
    apply: rewrite(CLAIM_ROUTE, `.md#limitations"`, `.md#methods"`, 2, 'Limitations location link'),
  },
  'drop-missing-fragment': {
    target: 'fallback-address',
    apply: rewrite(CLAIM_ROUTE, `${SOURCE_LINK}#missing"`, `${SOURCE_LINK}"`, 2, 'missing section link'),
  },
  'hide-fallback-reason': {
    target: 'fallback-reason',
    apply: rewrite(CLAIM_ROUTE, '<span class="y-basedon__reason">', '<span class="y-basedon__reason" hidden>', 2, 'fallback reason'),
  },
  // A page script that pushes a second history entry for the address it is
  // already at makes Back stay on the source page.
  'push-duplicate-entry': {
    target: 'return',
    apply: async (page) => {
      let served = 0;
      await page.route('**/freshness.js{,?*}', async (route) => {
        const response = await route.fetch();
        served += 1;
        await route.fulfill({ response, body: `${await response.text()}\n;history.pushState(null, '', location.href);\n` });
      });
      return () => (served > 0 ? '' : 'freshness.js was never served through the rewrite');
    },
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`source-location-round-trip: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`source-location-round-trip: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`source-location-round-trip: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// openSources returns the one source list a reader can see, opening the
// narrow layout's disclosure when that is where it sits.
const openSources = async (page) => {
  const disclosure = page.locator('details.y-toc-inline').filter({ has: page.locator('.y-basedon') }).first();
  if (await disclosure.count() && await disclosure.isVisible() && !(await disclosure.evaluate((el) => el.open))) {
    await disclosure.locator('summary').click();
  }
  const list = page.locator('.y-basedon:visible');
  if (await list.count() !== 1) broken(`${await list.count()} source lists are visible, want 1`);
  return list;
};

const headingInView = (page, id) => page.waitForFunction((anchor) => {
  const el = document.getElementById(anchor);
  if (!el) return false;
  const { top } = el.getBoundingClientRect();
  return top >= 0 && top < window.innerHeight && scrollY > 800;
}, id, { timeout: 5000 });

const followLocation = async (page, list, label, id, site, passage) => {
  await list.getByRole('link', { name: label }).click();
  try {
    await page.waitForURL((url) => url.pathname === SOURCE && decodeURIComponent(url.hash.slice(1)) === id, { timeout: 5000 });
    await arrived(page);
    await headingInView(page, id);
  } catch {
    fail(site, `${label} did not land on #${id}; the page is at ${page.url()} scrolled to ${await page.evaluate(() => scrollY)}`);
  }
  if (!(await page.locator('main').getByText(passage, { exact: true }).isVisible())) {
    fail(site, `the passage under #${id} is not on screen after following ${label}`);
  }
};

const backToClaim = async (page, claimUrl) => {
  await page.goBack();
  try {
    await page.waitForURL(claimUrl, { timeout: 3000 });
  } catch {
    fail('return', `Back ended at ${page.url()}, want the claim ${claimUrl}`);
  }
  const question = page.locator('main').getByText('Does the frozen-handbook study justify', { exact: false });
  if (!(await question.first().isVisible())) fail('return', 'Back did not show the claim being checked');
};

const journey = async (browser, width) => {
  const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  try {
    const claimUrl = BASE + PAGE;
    const response = await page.goto(claimUrl);
    await arrived(page);
    if (!response || response.status() !== 200) broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
    await page.waitForSelector('html[data-js]');
    if (proof) {
      const issue = proof();
      if (issue) throw new NotApplied(`NOT-APPLIED source-location-round-trip: ${MUTATE}: ${issue}`);
    }

    let list = await openSources(page);
    await followLocation(page, list, 'Method evidence', 'methods', 'fragment-methods', 'Only a frozen event handbook was tested.');
    await backToClaim(page, claimUrl);
    list = await openSources(page);
    await followLocation(page, list, 'Study limitations', 'limitations', 'fragment-limitations', 'Changing team documents were not tested.');
    await backToClaim(page, claimUrl);

    list = await openSources(page);
    const missing = list.locator('a.wikilink-degraded').filter({ hasText: 'Missing evidence' });
    if (await missing.count() !== 1) broken('the claim shows no location the source lacks');
    const href = await missing.getAttribute('href');
    if (href !== `${SOURCE}#missing`) fail('fallback-address', `the missing section links to ${href}, want its address kept as ${SOURCE}#missing`);
    const reason = missing.locator('.y-basedon__reason');
    if (!(await reason.isVisible()) || (await reason.innerText()).trim() === '') {
      fail('fallback-reason', 'the missing location shows no reason without hovering');
    }
    await missing.click();
    await page.waitForURL((url) => url.pathname === SOURCE && url.hash === '#missing', { timeout: 5000 });
    if (await page.evaluate(() => scrollY) !== 0) fail('fallback-address', 'the missing section opened the source part-way down');
    return width;
  } finally {
    await context.close();
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const widths = [];
  for (const width of [1600, 390]) widths.push(await journey(browser, width));
  console.log(`PASS source-location-round-trip: claim to Methods, back, Limitations and the fallback at ${widths.join('px and ')}px`);
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
      else console.error(`no catch: ${MUTATE} injects a regression the ${target} assertion watches for, but the ${err.site} assertion fired first`);
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
