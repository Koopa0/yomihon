// Behavior lock for a course that declares its branches. Every fact below is
// one the reader can see, checked through the pages they actually open: the
// number Home shows, where a side branch is drawn, what the step links offer,
// and what a block declared out of the course does not appear in.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';

const HOME = '/';
const COURSE_PAGE = '/syllabus/Maps/branches.md';
const SECOND_LESSON = '/notes/Course/C02.md';
const SIDE_LESSON = '/notes/Course/S01.md';
const MAP_PAGE = '/notes/Notes/alpha.md';
const ROUTINE_LESSON = '/notes/Course/R01.md';

const SITES = [
  'home-counts-the-main-line',
  'side-branch-under-its-lesson',
  'main-line-steps-over-the-side-branch',
  'side-branch-does-not-rejoin',
  'declared-out-stays-out',
  'declared-out-is-in-no-course',
  'general-maps-unchanged',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN study-path-branches: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL study-path-branches: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN study-path-branches: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED study-path-branches: ${message}`); };

// rewriteDocument replaces one needle in one page's HTML and proves it applied,
// so a self-test whose needle died against a rewritten template turns red here
// rather than reporting that the regression walked past the probe.
const rewriteDocument = (path, needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + path, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `${label} document was never requested`;
    if (matches < 1) return `${label} needle matched nothing`;
    return '';
  };
};

const MUTATIONS = {
  'inflate-the-home-count': {
    target: 'home-counts-the-main-line',
    apply: rewriteDocument(HOME, '>4 課<', '>6 課<', 'Home course card'),
  },
  'detach-the-side-branch': {
    target: 'side-branch-under-its-lesson',
    apply: rewriteDocument(COURSE_PAGE, 'y-module--local', 'y-module--detached', 'course page side branch'),
  },
  'send-the-main-line-into-the-side-branch': {
    target: 'main-line-steps-over-the-side-branch',
    apply: rewriteDocument(SECOND_LESSON, '/notes/Course/C03.md', '/notes/Course/S01.md', 'next step link'),
  },
  'rejoin-the-side-branch': {
    target: 'side-branch-does-not-rejoin',
    apply: rewriteDocument(
      SIDE_LESSON,
      '<div class="y-prose">',
      '<nav class="y-steps"><a rel="next" href="/notes/Course/C03.md">下一課</a></nav><div class="y-prose">',
      'side-branch article foot',
    ),
  },
  'let-the-routine-block-in': {
    target: 'declared-out-stays-out',
    apply: rewriteDocument(
      COURSE_PAGE,
      '<div class="y-syl">',
      '<div class="y-syl"><a class="y-lesson" href="/notes/Course/R01.md">R01</a>',
      'course page body',
    ),
  },
  'place-the-routine-lesson-in-the-course': {
    target: 'declared-out-is-in-no-course',
    apply: rewriteDocument(
      ROUTINE_LESSON,
      '<div class="y-prose">',
      '<nav class="y-steps" aria-label="Branch Course 課程順序"><a rel="next" href="/notes/Course/C01.md">下一課</a></nav><div class="y-prose">',
      'routine lesson article foot',
    ),
  },
  'empty-the-general-map': {
    target: 'general-maps-unchanged',
    apply: rewriteDocument(MAP_PAGE, 'data-map-tree="Maps/reading.md"', 'data-map-tree="Maps/removed.md"', 'general map drawer'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`study-path-branches: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`study-path-branches: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`study-path-branches: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;

// assertApplied refuses to call anything a catch until the mutation is shown to
// have reached the page. Without it a needle that matched nothing reads as
// "the probe saw no regression", which is the one answer a self-test must never
// be able to give.
const assertApplied = () => {
  if (!proof) return;
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  // Home counts the main line: three written lessons plus the one that is
  // planned and unwritten. The side branch and the routine block are not it.
  await page.goto(BASE + HOME, { waitUntil: 'domcontentloaded' });
  const card = page.locator('main a[href="/syllabus/Maps/branches.md"]');
  if (await card.count() !== 1) broken(`Home shows ${await card.count()} branch-course cards, want 1`);
  const homeCount = (await card.innerText()).match(/(\d+)\s*課/);
  if (!homeCount) broken(`the Home card shows no lesson count: ${JSON.stringify(await card.innerText())}`);
  if (homeCount[1] !== '4') {
    fail('home-counts-the-main-line',
      `Home shows ${homeCount[1]} 課, want 4: the main line's three written lessons and the one still to be written, without the side branch or the routine block`);
  }

  // The side branch is drawn where the author put it — inside the part, after
  // the lesson it hangs from — labelled a side branch and carrying its own
  // count, not the course's.
  await page.goto(BASE + COURSE_PAGE, { waitUntil: 'domcontentloaded' });
  const local = page.locator('main .y-module--local');
  if (await local.count() !== 1) {
    fail('side-branch-under-its-lesson', `the course page draws ${await local.count()} side branches, want 1`);
  }
  const localText = await local.innerText();
  if (!localText.includes('支線') || !/1\s*課/.test(localText)) {
    fail('side-branch-under-its-lesson',
      `the side branch is not labelled a side branch with its own count: ${JSON.stringify(localText)}`);
  }
  const order = await page.locator('main a.y-lesson, main .y-module--local').evaluateAll((nodes) =>
    nodes.map((node) => (node.classList.contains('y-module--local') ? 'SIDE' : node.getAttribute('href'))));
  const anchorAt = order.indexOf('/notes/Course/C02.md');
  if (anchorAt < 0 || order[anchorAt + 1] !== 'SIDE') {
    fail('side-branch-under-its-lesson', `the side branch is not drawn under C02: ${JSON.stringify(order)}`);
  }

  // A block declared out of the course is out of it: the course page never
  // lists it, and it holds no place in the course.
  if (await page.locator('main a[href="/notes/Course/R01.md"]').count() !== 0) {
    fail('declared-out-stays-out', 'the course page lists a lesson from the block declared out of the course');
  }

  // The main line steps over the side branch: the lesson after the one it
  // hangs from is the next main-line lesson.
  await page.goto(BASE + SECOND_LESSON, { waitUntil: 'domcontentloaded' });
  const next = page.locator('nav.y-steps a[rel="next"]');
  if (await next.count() !== 1) {
    fail('main-line-steps-over-the-side-branch', `the second lesson offers ${await next.count()} next steps, want 1`);
  }
  const nextHref = await next.getAttribute('href');
  if (nextHref !== '/notes/Course/C03.md') {
    fail('main-line-steps-over-the-side-branch',
      `the lesson after C02 is ${JSON.stringify(nextHref)}, want the next main-line lesson`);
  }
  // The routine block is absent from the course's own drawer too. The folder
  // tree still lists the file, because a folder is not a course.
  const drawer = page.locator('[data-map-tree="Maps/branches.md"]');
  if (await drawer.count() !== 1) broken(`the course drawer is present ${await drawer.count()} times, want 1`);
  if (await drawer.locator('a[href="/notes/Course/R01.md"]').count() !== 0) {
    fail('declared-out-stays-out', 'a lesson declared out of the course appears in the course drawer');
  }

  // A lesson the course declared itself out of is in no course. Opened on its
  // own page, it is offered no course order and no course places it — the
  // folder tree still lists the file, because a folder is not a course.
  await page.goto(BASE + ROUTINE_LESSON, { waitUntil: 'domcontentloaded' });
  // Folder order is not course order: the file sits beside others in a folder,
  // and that is a fact about the folder. What must not appear is a step
  // labelled with the course, which would say the course teaches it.
  const stepLabels = await page.locator('nav.y-steps').evaluateAll((nodes) =>
    nodes.map((node) => node.getAttribute('aria-label') || ''));
  if (stepLabels.some((label) => label.includes('Branch Course'))) {
    fail('declared-out-is-in-no-course',
      `a lesson declared out of the course is offered its course order: ${JSON.stringify(stepLabels)}`);
  }
  const routineDrawer = page.locator('[data-map-tree="Maps/branches.md"]');
  if (await routineDrawer.count() !== 1) broken(`the course drawer is present ${await routineDrawer.count()} times, want 1`);
  if (await routineDrawer.locator(`a[href="${ROUTINE_LESSON}"]`).count() !== 0) {
    fail('declared-out-is-in-no-course', 'the course drawer places a lesson the course declared itself out of');
  }
  const folderRow = page.locator(`.y-rail-left a[href="${ROUTINE_LESSON}"]`);
  if (await folderRow.count() === 0) {
    broken('the folder tree stopped listing the file, so this check no longer separates a folder from a course');
  }

  // A side branch's last lesson closes it: it does not rejoin the main line.
  await page.goto(BASE + SIDE_LESSON, { waitUntil: 'domcontentloaded' });
  if (await page.locator('nav.y-steps a[rel="next"]').count() !== 0) {
    fail('side-branch-does-not-rejoin',
      'the side branch\'s last lesson offers a next lesson, so the branch rejoined the course');
  }

  // Narrowing courses must not narrow maps: a general map still lists what it
  // lists, through the grammar it always used.
  await page.goto(BASE + MAP_PAGE, { waitUntil: 'domcontentloaded' });
  const mapTree = page.locator('[data-map-tree="Maps/reading.md"]');
  if (await mapTree.count() !== 1) {
    fail('general-maps-unchanged', `the general map drawer is present ${await mapTree.count()} times, want 1`);
  }

  assertApplied();
  if (MUTATE) {
    console.log(`MUTATE-RESULT: no-catch ${MUTATE}`);
    process.exitCode = 0;
  } else {
    console.log('PASS study-path-branches');
  }
} catch (error) {
  if (error instanceof LockFired) {
    // A mutation that never reached the page proves nothing, so an assertion
    // firing beside one is a real regression report, not a catch.
    if (MUTATE) assertApplied();
    console.error(error.message);
    if (MUTATE && MUTATIONS[MUTATE].target === error.site) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      process.exitCode = 1;
    } else if (MUTATE) {
      console.error(`study-path-branches: ${MUTATE} fired ${error.site}, which it does not aim at`);
      process.exitCode = 1;
    } else {
      process.exitCode = 1;
    }
  } else if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    console.error(error instanceof ProbeBroken ? error.message : `BROKEN study-path-branches: ${error.stack || error}`);
    process.exitCode = 2;
  }
} finally {
  if (proof && !MUTATE) broken('a mutation proof exists without a MUTATE mode');
  await browser.close();
}
