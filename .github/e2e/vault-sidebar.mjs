// Behavior lock: pages that still mount the whole-vault sidebar — search among
// them — keep Paths-before-Maps-before-Journal order, start Journal collapsed,
// and open the map that holds the note a recovery page names.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search';
const STATUS = '/status';
const RECOVERY_NOTE = 'Notes/alpha.md';
const READING_MAP = 'Maps/reading.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'group-order',
  'map-open',
  'journal-collapsed',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN vault-sidebar: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL vault-sidebar: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN vault-sidebar: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED vault-sidebar: ${message}`); };

const fetchSameOrigin = (route) => route.fetch({
  headers: {
    ...route.request().headers(),
    origin: BASE,
    'sec-fetch-site': 'same-origin',
  },
});

const rewritePath = (path, transform) => async (page) => {
  let applied = false;
  await page.route(BASE + path, async (route) => {
    const response = route.request().method() === 'POST' ? await fetchSameOrigin(route) : await route.fetch();
    const original = await response.text();
    const body = transform(original);
    if (body !== original) applied = true;
    return route.fulfill({ response, body });
  });
  return () => applied;
};

const MUTATIONS = {
  'swap-paths-maps': {
    target: 'group-order',
    apply: rewritePath(PAGE, (body) => body
      .replaceAll('data-sidebar-group="paths"', 'data-sidebar-group="swap"')
      .replaceAll('data-sidebar-group="maps"', 'data-sidebar-group="paths"')
      .replaceAll('data-sidebar-group="swap"', 'data-sidebar-group="maps"')),
  },
  'close-current-map': {
    target: 'map-open',
    apply: rewritePath(STATUS, (body) => body
      .replace(new RegExp(`<details open data-map-tree="${READING_MAP.replace('/', '\\/')}"`, 'g'), `<details data-map-tree="${READING_MAP}"`)
      .replaceAll(` data-chain data-key="map:${READING_MAP}"`, ` data-key="map:${READING_MAP}"`)),
  },
  'open-journal': {
    target: 'journal-collapsed',
    apply: rewritePath(PAGE, (body) => body.replaceAll('<details data-sidebar-group="journal"', '<details open data-sidebar-group="journal"')),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`vault-sidebar: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`vault-sidebar: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`vault-sidebar: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const submitRecovery = async (page) => {
  await page.evaluate(({ base, notePath }) => {
    const form = document.createElement('form');
    form.method = 'POST';
    form.action = `${base}/status`;
    for (const [name, value] of [['path', notePath], ['from', 'draft'], ['to', '']]) {
      const input = document.createElement('input');
      input.type = 'hidden';
      input.name = name;
      input.value = value;
      form.appendChild(input);
    }
    document.body.appendChild(form);
    form.submit();
  }, { base: BASE, notePath: RECOVERY_NOTE });
  await page.waitForURL(`${BASE}${STATUS}`, { waitUntil: 'domcontentloaded' });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const sidebar = page.locator('aside.y-rail-left');
  if (await sidebar.count() !== 1) broken('the page has no single vault sidebar');

  const groups = await sidebar.locator('[data-sidebar-group]').evaluateAll((elements) => elements.map((el) => el.dataset.sidebarGroup));
  if (groups.join(',') !== 'paths,maps,journal,reports') {
    fail('group-order', `groups are ${groups.join(',') || 'absent'}, want paths,maps,journal,reports`);
  }

  const journal = sidebar.locator('details[data-sidebar-group="journal"]');
  if (await journal.count() !== 1) broken('the vault sidebar has no Journal group');
  if (await journal.evaluate((el) => el.open)) fail('journal-collapsed', 'Journal starts open');

  await submitRecovery(page);

  const recoverySidebar = page.locator('aside.y-rail-left');
  if (await recoverySidebar.count() !== 1) broken('the recovery page has no single vault sidebar');
  const map = recoverySidebar.locator(`details[data-map-tree="${READING_MAP}"]`);
  if (await map.count() !== 1) broken(`the recovery sidebar has no ${READING_MAP} disclosure`);
  if (!await map.evaluate((el) => el.open)) {
    fail('map-open', 'the map containing the recovery note is closed');
  }

  if (proof && !proof()) notApplied(`the ${MUTATE} mutation changed nothing in the response it targeted`);

  console.log('PASS vault-sidebar: search keeps vault drawer order and collapsed Journal; recovery opens the note\'s map');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE && (!proof || !proof())) {
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
    } else {
      if (MUTATE) {
        const { target } = MUTATIONS[MUTATE];
        if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
        else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
      }
      process.exitCode = 1;
    }
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
