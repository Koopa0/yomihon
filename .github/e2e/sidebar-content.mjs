// Behavior lock: the sidebar grows from the fixture vault's map and Diary
// content, opens the map that contains the current note, omits unresolved rows
// from general maps while retaining study-path warnings, and leaves lifecycle
// state in Home plus the shared advanceable-note topbar chip.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const HOME = '/';
const MAP_PAGE = '/notes/Maps/reading.md';
const STUDY_PAGE = '/syllabus/Maps/study.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'group-order',
  'map-present',
  'map-open',
  'current-entry',
  'map-resolved-only',
  'path-noninstance-warning',
  'path-unresolved-warning',
  'map-page-unwritten-kept',
  'path-page-unresolved-kept',
  'path-page-noninstance-kept',
  'journal-present',
  'journal-collapsed',
  'lifecycle-retired',
  'advanceable-chip',
  'home-start-top',
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
  'inject-unwritten-map-row': {
    target: 'map-resolved-only',
    apply: replaceEvery('<span class="y-railitem__name">Reading Map</span>', '<span class="y-railitem__name">Reading Map</span><span>Unwritten Note</span>'),
  },
  'drop-path-warning': {
    target: 'path-unresolved-warning',
    apply: replaceEvery('Unwritten Lesson', 'Removed path warning'),
  },
  'drop-path-noninstance-warning': {
    target: 'path-noninstance-warning',
    apply: replaceEvery('Template-only lesson', 'Removed non-instance warning'),
  },
  'drop-unwritten-map-row': {
    target: 'map-page-unwritten-kept',
    apply: rewritePath(MAP_PAGE, (body) => body.replaceAll('Unwritten Note', 'Removed map row')),
  },
  'drop-unwritten-path-row': {
    target: 'path-page-unresolved-kept',
    apply: rewritePath(STUDY_PAGE, (body) => body.replaceAll('Unwritten Lesson', 'Removed path row')),
  },
  'drop-noninstance-path-row': {
    target: 'path-page-noninstance-kept',
    apply: rewritePath(STUDY_PAGE, (body) => body.replaceAll('Template-only lesson', 'Removed non-instance row')),
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
    apply: replaceEvery('id="nav-rail" aria-label="Vault 導覽">', 'id="nav-rail" aria-label="Vault 導覽"><span>Lifecycle</span>'),
  },
  'rename-advanceable': {
    target: 'advanceable-chip',
    apply: replaceEvery('aria-label="1 篇筆記的狀態還有下一步"', 'aria-label="advanceable"'),
  },
  'autofocus-home-search': {
    target: 'home-start-top',
    apply: rewritePath(HOME, (body) => body.replaceAll(
      'placeholder="搜尋書庫…" aria-label="搜尋筆記">',
      'placeholder="搜尋書庫…" aria-label="搜尋筆記" autofocus>',
    )),
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
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const sidebar = page.locator('aside.y-rail-left');
  if (await sidebar.count() !== 1) broken('the page has no single sidebar');

  const groups = await sidebar.locator('[data-sidebar-group]').evaluateAll((elements) => elements.map((el) => el.dataset.sidebarGroup));
  if (groups.join(',') !== 'paths,maps,journal,reports') {
    fail('group-order', `groups are ${groups.join(',') || 'absent'}, want paths,maps,journal,reports`);
  }

  const map = sidebar.locator('details[data-map-tree="Maps/reading.md"]');
  if (await map.count() !== 1) fail('map-present', 'the fixture topic map did not become one map disclosure');
  if (!await map.evaluate((el) => el.open)) fail('map-open', 'the map containing the current note is closed');
  if (await map.locator('a[aria-current="page"][href="/notes/Notes/alpha.md"]').count() !== 1) {
    fail('current-entry', 'the current map entry is not marked');
  }
  if ((await map.textContent()).includes('Unwritten Note')) {
    fail('map-resolved-only', 'an unresolved general-map row appears in navigation');
  }

  const studyPath = sidebar.locator('details[data-map-tree="Maps/study.md"]');
  const policyWarning = studyPath.locator('[data-resolution="non-instance"]', { hasText: 'Template-only lesson' });
  const policyOrder = await studyPath.locator('a[href="/notes/Notes/alpha.md"], [data-resolution="non-instance"], [data-resolution="unresolved"], a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.dataset.resolution || row.getAttribute('href')));
  if (await studyPath.count() !== 1 || await policyWarning.count() !== 1 || await policyWarning.locator('.y-navmark--warn').count() === 0 || await studyPath.getByRole('link', { name: 'Template-only lesson', exact: true }).count() !== 0 || policyOrder.join(',') !== '/notes/Notes/alpha.md,non-instance,unresolved,/notes/Notes/beta.md') {
    fail('path-noninstance-warning', 'the non-instance study-path row is not one ordered, non-link policy warning in navigation');
  }
  const pathWarning = studyPath.locator('[data-resolution="unresolved"]', { hasText: 'Unwritten Lesson' });
  const sidebarPathOrder = await studyPath.locator('a[href="/notes/Notes/alpha.md"], [data-resolution="unresolved"], a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.hasAttribute('data-resolution') ? 'warning' : row.getAttribute('href')));
  if (await studyPath.count() !== 1 || await pathWarning.count() !== 1 || await pathWarning.locator('.y-navmark--warn').count() === 0 || await studyPath.getByRole('link', { name: 'Unwritten Lesson', exact: true }).count() !== 0 || sidebarPathOrder.join(',') !== '/notes/Notes/alpha.md,warning,/notes/Notes/beta.md') {
    fail('path-unresolved-warning', 'the unresolved study-path row is not one ordered, non-link warning in navigation');
  }

  const journal = sidebar.locator('details[data-sidebar-group="journal"]');
  if (await journal.count() !== 1 || await journal.locator('a[href="/notes/Diary/2026-07-10.md"]').count() !== 1) {
    fail('journal-present', 'the untyped Diary fixture did not become a Journal entry');
  }
  if (await journal.evaluate((el) => el.open)) fail('journal-collapsed', 'Journal starts open');

  if (await sidebar.getByText('Lifecycle', { exact: true }).count() !== 0) {
    fail('lifecycle-retired', 'Lifecycle still appears in the sidebar');
  }
  const chip = page.locator('a[data-advanceable-chip]');
  if (await chip.count() !== 1 || await chip.getAttribute('aria-label') !== '1 篇筆記的狀態還有下一步' || await chip.getAttribute('href') !== '/') {
    fail('advanceable-chip', 'the shared advanceable-note chip is absent, mislabeled, or does not link Home');
  }

  await page.setViewportSize({ width: 1270, height: 720 });
  await page.goto(BASE + HOME, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const homeMain = page.locator('main.y-main');
  const homeSearch = page.locator('form.y-homesearch input[type="search"]');
  if (await homeMain.count() !== 1 || await homeSearch.count() !== 1 || await homeSearch.getAttribute('autofocus') !== null || await homeMain.evaluate((element) => element.scrollTop) !== 0) {
    fail('home-start-top', 'Home did not start at the top without native search autofocus at 1270×720');
  }

  await page.goto(BASE + MAP_PAGE, { waitUntil: 'domcontentloaded' });
  if (!((await page.locator('main').textContent()).includes('Unwritten Note'))) {
    fail('map-page-unwritten-kept', 'the unresolved row vanished from the map note itself');
  }

  await page.goto(BASE + STUDY_PAGE, { waitUntil: 'domcontentloaded' });
  const syllabusPolicyWarning = page.locator('main .y-lesson--broken[data-resolution="non-instance"]', { hasText: 'Template-only lesson' });
  const syllabusPolicyOrder = await page.locator('main a[href="/notes/Notes/alpha.md"], main [data-resolution="non-instance"], main [data-resolution="unresolved"], main a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.dataset.resolution || row.getAttribute('href')));
  if (await syllabusPolicyWarning.count() !== 1 || await syllabusPolicyWarning.locator('.y-navmark--warn').count() === 0 || await page.locator('main a', { hasText: 'Template-only lesson' }).count() !== 0 || syllabusPolicyOrder.join(',') !== '/notes/Notes/alpha.md,non-instance,unresolved,/notes/Notes/beta.md') {
    fail('path-page-noninstance-kept', 'the non-instance study-path row is not one ordered, non-link policy warning on the syllabus page');
  }
  const syllabusWarning = page.locator('main .y-lesson--broken[data-resolution="unresolved"]', { hasText: 'Unwritten Lesson' });
  const syllabusPathOrder = await page.locator('main a[href="/notes/Notes/alpha.md"], main [data-resolution="unresolved"], main a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.hasAttribute('data-resolution') ? 'warning' : row.getAttribute('href')));
  if (await syllabusWarning.count() !== 1 || await syllabusWarning.locator('.y-navmark--warn').count() === 0 || await page.locator('main a', { hasText: 'Unwritten Lesson' }).count() !== 0 || syllabusPathOrder.join(',') !== '/notes/Notes/alpha.md,warning,/notes/Notes/beta.md') {
    fail('path-page-unresolved-kept', 'the unresolved study-path row is not one ordered, non-link warning on the syllabus page');
  }
  if (proof && !proof()) notApplied(`the ${MUTATE} mutation changed nothing in the document`);

  console.log('PASS sidebar-content: Paths then Maps then collapsed Journal then Reports; general maps resolve only, study paths retain warnings, map pages retain source rows, and Home starts at the top');
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
