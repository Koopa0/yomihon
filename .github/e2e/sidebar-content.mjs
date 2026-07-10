// Behavior lock: the sidebar grows from the fixture vault's map and Diary
// content, opens the map that contains the current note, omits unresolved rows,
// and leaves lifecycle state in Home plus the shared topbar chip.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MAP_PAGE = '/notes/Maps/reading.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'group-order',
  'map-present',
  'map-open',
  'current-entry',
  'resolved-only',
  'map-page-unwritten-kept',
  'journal-present',
  'journal-collapsed',
  'lifecycle-retired',
  'pending-chip',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN sidebar-content: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL sidebar-content: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN sidebar-content: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED sidebar-content: ${message}`); };

const rewritePath = (path, transform) => async (page) => {
  let applied = false;
  await page.route(BASE + path, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = transform(original);
    if (body !== original) applied = true;
    return route.fulfill({ response, body });
  });
  return () => applied;
};

const rewriteDocument = (transform) => rewritePath(PAGE, transform);
const replaceEvery = (needle, replacement) => rewriteDocument((body) => body.replaceAll(needle, replacement));

const MUTATIONS = {
  'swap-paths-maps': {
    target: 'group-order',
    apply: rewriteDocument((body) => body
      .replaceAll('data-sidebar-group="paths"', 'data-sidebar-group="swap"')
      .replaceAll('data-sidebar-group="maps"', 'data-sidebar-group="paths"')
      .replaceAll('data-sidebar-group="swap"', 'data-sidebar-group="maps"')),
  },
  'drop-map': {
    target: 'map-present',
    apply: replaceEvery('data-map-tree="Maps/reading.md"', 'data-map-tree="Maps/missing.md"'),
  },
  'close-current-map': {
    target: 'map-open',
    apply: rewriteDocument((body) => body
      .replace(/<details open data-map-tree="Maps\/reading\.md"/g, '<details data-map-tree="Maps/reading.md"')
      .replaceAll(' data-chain data-key="map:Maps/reading.md"', ' data-key="map:Maps/reading.md"')),
  },
  'drop-current-entry': {
    target: 'current-entry',
    apply: replaceEvery(' aria-current="page"', ''),
  },
  'inject-unwritten': {
    target: 'resolved-only',
    apply: replaceEvery('id="nav-rail">', 'id="nav-rail"><span>Unwritten Note</span>'),
  },
  'drop-unwritten-map-row': {
    target: 'map-page-unwritten-kept',
    apply: rewritePath(MAP_PAGE, (body) => body.replaceAll('Unwritten Note', 'Removed map row')),
  },
  'drop-journal': {
    target: 'journal-present',
    apply: replaceEvery('href="/notes/Diary/2026-07-10.md"', 'href="/notes/Diary/missing.md"'),
  },
  'open-journal': {
    target: 'journal-collapsed',
    apply: replaceEvery('<details data-sidebar-group="journal"', '<details open data-sidebar-group="journal"'),
  },
  'inject-lifecycle': {
    target: 'lifecycle-retired',
    apply: replaceEvery('id="nav-rail">', 'id="nav-rail"><span>Lifecycle</span>'),
  },
  'rename-pending': {
    target: 'pending-chip',
    apply: replaceEvery('aria-label="1 to decide"', 'aria-label="pending"'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`sidebar-content: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`sidebar-content: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`sidebar-content: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const sidebar = page.locator('aside.y-rail-left');
  if (await sidebar.count() !== 1) broken('the page has no single sidebar');

  const groups = await sidebar.locator('[data-sidebar-group]').evaluateAll((elements) => elements.map((el) => el.dataset.sidebarGroup));
  if (groups.join(',') !== 'paths,maps,journal') {
    fail('group-order', `groups are ${groups.join(',') || 'absent'}, want paths,maps,journal`);
  }

  const map = sidebar.locator('details[data-map-tree="Maps/reading.md"]');
  if (await map.count() !== 1) fail('map-present', 'the fixture topic map did not become one map disclosure');
  if (!await map.evaluate((el) => el.open)) fail('map-open', 'the map containing the current note is closed');
  if (await map.locator('a[aria-current="page"][href="/notes/Notes/alpha.md"]').count() !== 1) {
    fail('current-entry', 'the current map entry is not marked');
  }
  if ((await sidebar.textContent()).includes('Unwritten Note')) {
    fail('resolved-only', 'an unresolved map row appears in navigation');
  }

  const journal = sidebar.locator('details[data-sidebar-group="journal"]');
  if (await journal.count() !== 1 || await journal.locator('a[href="/notes/Diary/2026-07-10.md"]').count() !== 1) {
    fail('journal-present', 'the untyped Diary fixture did not become a Journal entry');
  }
  if (await journal.evaluate((el) => el.open)) fail('journal-collapsed', 'Journal starts open');

  if (await sidebar.getByText('Lifecycle', { exact: true }).count() !== 0) {
    fail('lifecycle-retired', 'Lifecycle still appears in the sidebar');
  }
  const chip = page.locator('a[data-pending-chip]');
  if (await chip.count() !== 1 || await chip.getAttribute('aria-label') !== '1 to decide' || await chip.getAttribute('href') !== '/') {
    fail('pending-chip', 'the shared topbar chip is absent, mislabeled, or does not link Home');
  }

  await page.goto(BASE + MAP_PAGE, { waitUntil: 'domcontentloaded' });
  if (!((await page.locator('main').textContent()).includes('Unwritten Note'))) {
    fail('map-page-unwritten-kept', 'the unresolved row vanished from the map note itself');
  }
  if (proof && !proof()) notApplied(`the ${MUTATE} mutation changed nothing in the document`);

  console.log('PASS sidebar-content: Paths then Maps then collapsed Journal; map wayfinding, resolved-only navigation, unwritten map rows, and Home pending chip hold');
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
