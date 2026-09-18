// Behavior lock for a course listened to end to end. The page gathers the
// paragraphs its lessons are marked to be read aloud in, each under the lesson
// that carries it, in the order the course is read; the bar that walks them
// opens the page rather than sitting inside its first lesson, and says there
// what the voice cannot be asked for. It is one page of reading, so it fits a
// phone.
//
// Env: YOMIHON_BASE, PAGE_PATH (the listening fixture course), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/listen/Maps/listen.md';
const MUTATE = process.env.MUTATE || '';
const BAR = '.y-ttsbar';
const ANCHOR = '[data-readaloud-bar]';
const LESSON = '.y-listen__lesson';

// The fixture's two lessons and what each marks, written out rather than read
// back off the page: an expectation gathered the way the page gathers it would
// agree with the page however wrong both were.
const COURSE = [
  { lesson: 'L01 わたしは学生です', spoken: ['今日は晴れ。'] },
  { lesson: 'L02 三つの段落', spoken: ['一つ目の段落です。', '二つ目の段落です。', '三つ目の段落です。'] },
];
const LIMITS = '語音合成不會回報可靠的長度，所以這裡沒有進度條、沒有已播時間，也不能拖曳。段落就是移動的單位：連續朗讀、停止、上一段、下一段、速度。';

const SITES = [
  'the-course-speaks-in-order',
  'the-bar-opens-the-course',
  'the-bar-says-what-cannot-be-asked-for',
  'the-page-fits-a-phone',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN listen-course: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL listen-course: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN listen-course: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED listen-course: ${message}`); };

const rewriteScript = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/lesson.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `${label}: the runtime was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label}: the runtime needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const rewriteDocument = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(`**${PAGE}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `${label}: the document was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label}: the document needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  // A lesson's paragraphs never reach the page. The collector itself is held to
  // the same thing by the Go test beside it; what this proves is that a course
  // gone short is something this probe can see.
  'drop-a-lessons-paragraphs': {
    target: 'the-course-speaks-in-order',
    apply: rewriteDocument(
      '<section class="y-listen__lesson"><h2 class="y-listen__title"><a href="/notes/Writing/lessons/japanese/L02.md">',
      '<section class="y-listen__lesson" hidden><h2 class="y-listen__title"><a href="/notes/">',
      "the second lesson's section",
    ),
  },
  // The bar falls back to the note's placement and lands inside the first
  // lesson, so the controls for a whole course sit under one lesson's heading.
  'bar-lands-inside-the-first-lesson': {
    target: 'the-bar-opens-the-course',
    apply: rewriteScript(
      "    const anchor = document.querySelector('[data-readaloud-bar]');\n    if (anchor) anchor.append(toolbar);\n    else readingButtons[0].closest('.y-reading')?.before(toolbar);",
      "    readingButtons[0].closest('.y-reading')?.before(toolbar);",
      'where the bar lands',
    ),
  },
  // The sentence about what speech synthesis cannot report is dropped, and the
  // page offers a paragraph-at-a-time bar with no word about why.
  'say-nothing-about-the-limits': {
    target: 'the-bar-says-what-cannot-be-asked-for',
    apply: rewriteScript(
      "      limits.textContent = limitsLabel;\n",
      "      limits.textContent = '';\n",
      'the sentence about what cannot be asked for',
    ),
  },
  // Something in the reading column is wider than the phone it is read on.
  'a-wide-block-in-the-column': {
    target: 'the-page-fits-a-phone',
    apply: rewriteDocument(
      '<div class="y-listen">',
      '<div class="y-listen"><div class="y-seam" style="width:2000px"></div>',
      'the reading column',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`listen-course: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`listen-course: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`listen-course: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  // A phone, because that is the width this page has to survive and the one a
  // course listened to is most likely to be read on.
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  await page.addInitScript(() => {
    speechSynthesis.speak = (utterance) => { setTimeout(() => utterance.dispatchEvent(new Event('start')), 0); };
    speechSynthesis.cancel = () => {};
  });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  await page.waitForSelector(BAR, { state: 'attached', timeout: 3000 });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  // Read as sections rather than as one flat list, so a paragraph that drifted
  // under the wrong lesson's heading is a different failure from one that is
  // missing altogether.
  const course = await page.evaluate((selectors) => Array.from(
    document.querySelectorAll(selectors.lesson),
    (section) => ({
      lesson: section.querySelector('.y-listen__title')?.textContent ?? null,
      spoken: Array.from(section.querySelectorAll('[data-tts]'), (button) => button.getAttribute('data-tts')),
    }),
  ), { lesson: LESSON });
  const shape = (rows) => rows.map((row) => `${row.lesson}: ${row.spoken.join(' / ')}`).join('\n');
  if (shape(course) !== shape(COURSE)) {
    fail('the-course-speaks-in-order', `the course reads\n${shape(course)}\nwant\n${shape(COURSE)}`);
  }

  const bar = await page.evaluate((selectors) => {
    const toolbar = document.querySelector(selectors.bar);
    return {
      atAnchor: toolbar?.parentElement?.matches(selectors.anchor) ?? false,
      insideALesson: Boolean(toolbar?.closest(selectors.lesson)),
      limits: toolbar?.querySelector('.y-ttsbar__limits')?.textContent ?? null,
      parent: toolbar?.parentElement?.className ?? null,
    };
  }, { bar: BAR, anchor: ANCHOR, lesson: LESSON });
  if (bar.insideALesson || !bar.atAnchor) {
    fail('the-bar-opens-the-course', `the bar sits in ${JSON.stringify(bar.parent)}, want the page's own anchor above the first lesson: controls for a whole course under one lesson's heading read as that lesson's own`);
  }
  if (bar.limits !== LIMITS) {
    fail('the-bar-says-what-cannot-be-asked-for', `the bar says ${JSON.stringify(bar.limits)} about what the voice cannot be asked for, want the sentence naming the absent seek bar and elapsed time`);
  }

  const wide = await page.evaluate(() => {
    const over = [];
    for (const element of document.querySelectorAll('*')) {
      if (element.clientWidth <= 0 || element.scrollWidth <= element.clientWidth + 1) continue;
      const style = getComputedStyle(element);
      if (style.overflowX === 'auto' || style.overflowX === 'scroll') continue;
      // The page's own column and what it holds. The shared chrome around it
      // is every other page's business and is locked where it is built.
      if (!element.closest('.y-listen')) continue;
      over.push(`${element.tagName.toLowerCase()}.${element.className || '(none)'} ${element.scrollWidth}>${element.clientWidth}`);
    }
    return { over, document: document.documentElement.scrollWidth, viewport: document.documentElement.clientWidth };
  });
  if (wide.over.length > 0) {
    fail('the-page-fits-a-phone', `at 390px the listening column overflows: ${wide.over.join(' | ')}`);
  }
  if (wide.document > wide.viewport + 1) {
    fail('the-page-fits-a-phone', `at 390px the page scrolls sideways (${wide.document} against ${wide.viewport})`);
  }

  console.log(`PASS listen-course: ${COURSE.reduce((n, row) => n + row.spoken.length, 0)} marked paragraphs across ${COURSE.length} lessons, each under its own lesson in course order, with the bar opening the page, saying what the voice cannot be asked for, and the column fitting 390px`);
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE && !mutationApplied) {
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
