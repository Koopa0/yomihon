// Behavior lock: a reading page carries one map — the book inside a study path
// on alpha, with the current entry marked and study-path warnings kept — and
// never the whole-vault drawers the desk offers.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const HOME = '/';
const MAP_PAGE = '/notes/Maps/reading.md';
const STUDY_PAGE = '/syllabus/Maps/study.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'reading-rail-book',
  'book-path-present',
  'current-entry',
  'path-noninstance-warning',
  'path-unresolved-warning',
  'vault-drawers-absent',
  'map-page-unwritten-kept',
  'path-page-unresolved-kept',
  'path-page-noninstance-kept',
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
  'drop-book-rail': {
    target: 'reading-rail-book',
    apply: replaceEvery('data-reading-rail="book"', 'data-reading-rail="folder"'),
  },
  'drop-book-path': {
    target: 'book-path-present',
    apply: replaceEvery('data-book-path="Maps/study.md"', 'data-book-path="Maps/missing.md"'),
  },
  'drop-current-entry': {
    target: 'current-entry',
    apply: replaceEvery(' aria-current="page"', ''),
  },
  'drop-path-warning': {
    target: 'path-unresolved-warning',
    apply: replaceEvery('Unwritten Lesson', 'Removed path warning'),
  },
  'drop-path-noninstance-warning': {
    target: 'path-noninstance-warning',
    apply: replaceEvery('Template-only lesson', 'Removed non-instance warning'),
  },
  'restore-vault-drawers': {
    target: 'vault-drawers-absent',
    apply: replaceEvery('<div class="y-railgroup">', '<div class="y-railgroup" data-sidebar-group="paths">'),
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
  'autofocus-home-search': {
    target: 'home-start-top',
    apply: rewritePath(HOME, (body) => body.replaceAll(
      'placeholder="搜尋書庫…" aria-label="搜尋書庫">',
      'placeholder="搜尋書庫…" aria-label="搜尋書庫" autofocus>',
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

  const kind = await sidebar.getAttribute('data-reading-rail');
  if (kind !== 'book') fail('reading-rail-book', `reading rail kind is ${kind ?? 'absent'}, want book`);

  const book = sidebar.locator('[data-book-path="Maps/study.md"]');
  if (await book.count() !== 1) fail('book-path-present', 'the fixture study path did not become the book rail');

  if (await book.locator('a[aria-current="page"][href="/notes/Notes/alpha.md"]').count() !== 1) {
    fail('current-entry', 'the current lesson is not marked in the book rail');
  }

  const entries = book.locator('.y-railgroup');
  const policyWarning = entries.locator('[data-resolution="non-instance"]', { hasText: 'Template-only lesson' });
  const policyOrder = await entries.locator('a[href="/notes/Notes/alpha.md"], [data-resolution="non-instance"], [data-resolution="unresolved"], a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.dataset.resolution || row.getAttribute('href')));
  if (await policyWarning.count() !== 1 || await policyWarning.locator('.y-navmark--warn').count() === 0 || await book.getByRole('link', { name: 'Template-only lesson', exact: true }).count() !== 0 || policyOrder.join(',') !== '/notes/Notes/alpha.md,non-instance,unresolved,/notes/Notes/beta.md') {
    fail('path-noninstance-warning', 'the non-instance study-path row is not one ordered, non-link policy warning in the book rail');
  }
  const pathWarning = entries.locator('[data-resolution="unresolved"]', { hasText: 'Unwritten Lesson' });
  const bookPathOrder = await entries.locator('a[href="/notes/Notes/alpha.md"], [data-resolution="unresolved"], a[href="/notes/Notes/beta.md"]').evaluateAll((rows) => rows.map((row) => row.hasAttribute('data-resolution') ? 'warning' : row.getAttribute('href')));
  if (await pathWarning.count() !== 1 || await pathWarning.locator('.y-navmark--warn').count() === 0 || await book.getByRole('link', { name: 'Unwritten Lesson', exact: true }).count() !== 0 || bookPathOrder.join(',') !== '/notes/Notes/alpha.md,warning,/notes/Notes/beta.md') {
    fail('path-unresolved-warning', 'the unresolved study-path row is not one ordered, non-link warning in the book rail');
  }

  const groups = await sidebar.locator('[data-sidebar-group]').count();
  if (groups !== 0) fail('vault-drawers-absent', `the reading rail still carries ${groups} vault drawer group(s)`);

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

  console.log('PASS sidebar-content: one book rail on alpha; study-path warnings kept; vault drawers absent; map and syllabus pages unchanged');
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
