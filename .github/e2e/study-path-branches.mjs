// Behavior lock for a course that declares its branches. Every fact below is
// one the reader can see, checked through the pages they actually open: the
// number the course index shows, where a side branch is drawn, what the step
// links offer, and what a block declared out of the course does not appear in.
//
// The count is read off the course index rather than the desk. The desk shows
// a corner of that listing — its first few rows — so a vault with one more
// course than fits would have hidden this one and failed the check for having
// too much in it rather than for counting wrongly.
//
// Env: YOMIHON_BASE and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';

const PATH_INDEX = '/paths';
const COURSE_PAGE = '/syllabus/Maps/branches.md';
const SECOND_LESSON = '/notes/Course/C02.md';
const SIDE_LESSON = '/notes/Course/S01.md';
const MAP_PAGE = '/notes/Maps/reading.md';
const ROUTINE_LESSON = '/notes/Course/R01.md';

const SITES = [
  'the-index-counts-the-main-line',
  'a-path-of-notes-is-counted-in-items',
  'side-branch-under-its-lesson',
  'main-line-steps-over-the-side-branch',
  'side-branch-does-not-rejoin',
  'side-branch-offers-a-labelled-way-back-and-on',
  'a-lesson-names-its-book-above-its-title',
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
  'inflate-the-course-count': {
    target: 'the-index-counts-the-main-line',
    apply: rewriteDocument(PATH_INDEX, '>4 課<', '>6 課<', 'course index row'),
  },
  // The path of notes the fixture holds is called a course, though nothing in
  // it is a lesson.
  'call-the-notes-lessons': {
    target: 'a-path-of-notes-is-counted-in-items',
    apply: rewriteDocument(PATH_INDEX, '>4 篇<', '>4 課<', 'study path index row'),
  },
  'detach-the-side-branch': {
    target: 'side-branch-under-its-lesson',
    apply: rewriteDocument(COURSE_PAGE, 'y-module--local', 'y-module--detached', 'course page side branch'),
  },
  'send-the-main-line-into-the-side-branch': {
    target: 'main-line-steps-over-the-side-branch',
    apply: rewriteDocument(SECOND_LESSON, '/notes/Course/C03.md', '/notes/Course/S01.md', 'next step link'),
  },
  // The branch's only lesson is left with nothing but the step it never had:
  // the labelled way on to the main line is gone.
  'drop-the-way-on': {
    target: 'side-branch-offers-a-labelled-way-back-and-on',
    apply: rewriteDocument(SIDE_LESSON, 'y-steps__link--onward', 'y-steps__link--detached', 'side-branch article foot'),
  },
  // A lesson of the course loses the line that names its book.
  'drop-the-running-head': {
    target: 'a-lesson-names-its-book-above-its-title',
    apply: rewriteDocument(SECOND_LESSON, 'y-crumbs--course', 'y-crumbs--plain', 'second lesson article head'),
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
    apply: rewriteDocument(MAP_PAGE, 'Unwritten Note', 'Removed map row', 'general map note body'),
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

  // The course index counts the main line: three written lessons plus the one
  // that is planned and unwritten. The side branch and the routine block are
  // not it.
  await page.goto(BASE + PATH_INDEX, { waitUntil: 'domcontentloaded' });
  const row = page.locator('main a[href="/syllabus/Maps/branches.md"]');
  if (await row.count() !== 1) broken(`the course index lists the branch course ${await row.count()} times, want 1`);
  const listed = (await row.innerText()).match(/(\d+)\s*課/);
  if (!listed) broken(`the course index row shows no lesson count: ${JSON.stringify(await row.innerText())}`);
  if (listed[1] !== '4') {
    fail('the-index-counts-the-main-line',
      `the course index shows ${listed[1]} 課, want 4: the main line's three written lessons and the one still to be written, without the side branch or the routine block`);
  }

  // A path of notes that are not lessons is counted in items. The fixture's
  // study path lists two concept notes, a note outside the governed set and a
  // lesson nobody wrote, so it is the case a noun chosen without looking at
  // what the rows reached would get wrong.
  const notes = page.locator('main a[href="/syllabus/Maps/study.md"]');
  if (await notes.count() !== 1) broken(`the course index lists the study path ${await notes.count()} times, want 1`);
  const notesRow = await notes.innerText();
  if (!/4\s*篇/.test(notesRow) || notesRow.includes('課')) {
    fail('a-path-of-notes-is-counted-in-items',
      `the course index counts a path of concept notes as ${JSON.stringify(notesRow)}, want 4 篇 and no 課`);
  }

  // The side branch is drawn where the author put it — inside the part, under
  // the lesson it hangs from, in that lesson's own list item so the line the
  // main line is read along can pass it — and labelled a side branch. It states
  // no count of its own: the head counts it beside the course.
  await page.goto(BASE + COURSE_PAGE, { waitUntil: 'domcontentloaded' });
  const local = page.locator('main .y-module--local');
  if (await local.count() !== 1) {
    fail('side-branch-under-its-lesson', `the course page draws ${await local.count()} side branches, want 1`);
  }
  const localText = await local.innerText();
  if (!localText.includes('支線')) {
    fail('side-branch-under-its-lesson',
      `the side branch is not labelled a side branch: ${JSON.stringify(localText)}`);
  }
  const hung = page.locator('main li:has(> a.y-lesson[href="/notes/Course/C02.md"]) > .y-module--local');
  if (await hung.count() !== 1) {
    fail('side-branch-under-its-lesson',
      `the side branch is not inside the list item of the lesson it hangs from (found ${await hung.count()} there)`);
  }
  const order = await page.locator('main a.y-lesson, main .y-module--local').evaluateAll((nodes) =>
    nodes.map((node) => (node.classList.contains('y-module--local') ? 'SIDE' : node.getAttribute('href'))));
  const anchorAt = order.indexOf('/notes/Course/C02.md');
  if (anchorAt < 0 || order[anchorAt + 1] !== 'SIDE') {
    fail('side-branch-under-its-lesson', `the side branch is not drawn under C02: ${JSON.stringify(order)}`);
  }

  // A block declared out of the course is out of it: the course page lists it
  // beneath the contents as a reference, never as one of its lessons, and it
  // holds no place in the course.
  if (await page.locator('main a.y-lesson[href="/notes/Course/R01.md"]').count() !== 0) {
    fail('declared-out-stays-out', 'the course page lists a lesson from the block declared out of the course');
  }
  if (await page.locator('main .y-appendix a.y-ref[href="/notes/Course/R01.md"]').count() !== 1) {
    broken('the block declared out of the course is not listed beneath the contents, so this check no longer separates a reference from a lesson');
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
  const book = page.locator('[data-book-path="Maps/branches.md"]');
  if (await book.count() !== 1) broken(`the course book rail is present ${await book.count()} times, want 1`);
  if (await book.locator('a[href="/notes/Course/R01.md"]').count() !== 0) {
    fail('declared-out-stays-out', 'a lesson declared out of the course appears in the course book rail');
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
  const routineBook = page.locator('[data-book-path="Maps/branches.md"]');
  if (await routineBook.count() !== 0) {
    fail('declared-out-is-in-no-course', 'a lesson declared out of the course still carries a course book rail');
  }
  const folderRow = page.locator(`.y-rail-left a[href="${ROUTINE_LESSON}"]`);
  if (await folderRow.count() === 0) {
    broken('the folder tree stopped listing the file, so this check no longer separates a folder from a course');
  }

  // A side branch's last lesson closes it: it offers no next lesson in the
  // course's order, so the branch does not rejoin the main line's steps.
  await page.goto(BASE + SIDE_LESSON, { waitUntil: 'domcontentloaded' });
  if (await page.locator('nav.y-steps a[rel="next"]').count() !== 0) {
    fail('side-branch-does-not-rejoin',
      'the side branch\'s last lesson offers a next lesson, so the branch rejoined the course\'s steps');
  }

  // The branch's way off is labelled and is not a step: back to the lesson it
  // hangs from, on to the main line's next lesson, neither of them a previous
  // or a next in the course's order. The lesson it hangs from points at it.
  const back = page.locator('nav.y-steps a.y-steps__link--back');
  const onward = page.locator('nav.y-steps a.y-steps__link--onward');
  if (await back.count() !== 1 || await onward.count() !== 1) {
    fail('side-branch-offers-a-labelled-way-back-and-on',
      `the side branch's only lesson offers ${await back.count()} ways back and ${await onward.count()} ways on, want 1 and 1`);
  }
  if ((await back.getAttribute('href')) !== SECOND_LESSON || (await onward.getAttribute('href')) !== '/notes/Course/C03.md') {
    fail('side-branch-offers-a-labelled-way-back-and-on',
      `the way back leads to ${await back.getAttribute('href')} and the way on to ${await onward.getAttribute('href')}, want C02 and C03`);
  }
  if (await back.getAttribute('rel') !== null || await onward.getAttribute('rel') !== null) {
    fail('side-branch-offers-a-labelled-way-back-and-on', 'a way off the branch claims to be the previous or the next lesson');
  }
  await page.goto(BASE + SECOND_LESSON, { waitUntil: 'domcontentloaded' });
  if (await page.locator(`nav.y-steps p.y-steps__aside a[href="${SIDE_LESSON}"]`).count() !== 1) {
    fail('side-branch-offers-a-labelled-way-back-and-on', 'the lesson the side branch hangs from does not point at it');
  }

  // Every lesson of the course names its book above its title, as a link to the
  // contents; a side branch's lesson says so. A note in no course keeps the
  // folder's breadcrumb.
  const headOf = async (path) => {
    await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
    return page.locator('.y-article .y-crumbs--course a.y-crumbs__link');
  };
  const mainHead = await headOf(SECOND_LESSON);
  if (await mainHead.count() !== 1 || (await mainHead.innerText()).trim() !== 'Branch Course · 主線') {
    fail('a-lesson-names-its-book-above-its-title',
      `the second lesson's running head reads ${JSON.stringify(await mainHead.allInnerTexts())}, want "Branch Course · 主線"`);
  }
  if (!(await mainHead.getAttribute('href')).startsWith(COURSE_PAGE)) {
    fail('a-lesson-names-its-book-above-its-title', 'the running head does not lead to the course contents');
  }
  const sideHead = await headOf(SIDE_LESSON);
  if ((await sideHead.innerText()).trim() !== 'Branch Course · 主線 · 支線') {
    fail('a-lesson-names-its-book-above-its-title',
      `the side branch's running head reads ${JSON.stringify(await sideHead.allInnerTexts())}, want "Branch Course · 主線 · 支線"`);
  }
  await page.goto(BASE + ROUTINE_LESSON, { waitUntil: 'domcontentloaded' });
  if (await page.locator('.y-article .y-crumbs--course').count() !== 0) {
    fail('a-lesson-names-its-book-above-its-title', 'a lesson declared out of the course carries a running head naming it');
  }

  // Narrowing courses must not narrow maps: a general map still lists what it
  // lists, through the grammar it always used.
  await page.goto(BASE + MAP_PAGE, { waitUntil: 'domcontentloaded' });
  if (!((await page.locator('main').textContent()).includes('Unwritten Note'))) {
    fail('general-maps-unchanged', 'the general map note no longer lists its unresolved row');
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
