// Behavior lock for the course read as a line. A part joins the lessons of one
// run with a single vertical rule through the column their marks sit in, and
// the lesson the reader came from is the heavier point on it. Both halves are
// invisible to a Go test: the rule is a pseudo-element with no node to assert
// on, and whether it meets the marks is a number only a laid-out page has.
//
// The walk is the reader's: open a lesson, take the way into its course that
// the page drew, and look at what arrives. Then open the same course from its
// own address, the way the desk links to it, and look at what does not.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson inside a study path), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Course/C01.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'the-course-marks-the-lesson-it-was-reached-from',
  'a-course-reached-from-the-desk-marks-nobody',
  'the-line-joins-the-points',
  'the-course-fits-the-phone',
];

// The lesson PAGE is, as its course lists it, and the course's own address.
const LESSON_NAME = 'C01';
const COURSE_PATH = '/syllabus/Maps/branches.md';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN course-line: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL course-line: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN course-line: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED course-line: ${message}`);
};

// Appends a rule outside the product layer so it outranks what it stands in
// for, without touching the served file, and reads back what the browser
// resolved it to — a rule that was served but outranked reports itself rather
// than passing as a regression nobody noticed.
const appendRule = (rule, read, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await page.evaluate(read);
    if (got !== wanted) return `the page resolves ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

// Rewrites one served page. needle must be found, so a self-test that quietly
// died against rewritten markup turns red here rather than passing as a
// regression that walked through.
const rewritePage = (match, needle, replacement) => async (page) => {
  let hits = 0;
  await page.route(match, async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    if (!body.includes(needle)) {
      await route.fulfill({ response, body });
      return;
    }
    hits += 1;
    await route.fulfill({ response, body: body.replace(needle, replacement) });
  });
  return async () => (hits === 0 ? `nothing served carried ${JSON.stringify(needle)}` : '');
};

const MUTATIONS = {
  // Takes the reader's lesson back out of the way into the course, which is
  // the whole of how the page knows where they are.
  'drop-the-name-from-the-link': {
    target: 'the-course-marks-the-lesson-it-was-reached-from',
    apply: rewritePage(
      (url) => url.pathname === PAGE,
      `href="${COURSE_PATH}?from=`,
      `href="${COURSE_PATH}" data-dropped="`,
    ),
  },
  // Marks a row on a course nobody arrived at from a lesson, which is the
  // reader being told where they are by a page that was never told.
  'mark-a-row-nobody-arrived-at': {
    target: 'a-course-reached-from-the-desk-marks-nobody',
    // Nothing it rewrites is served until the course is opened by its own
    // address, so it has had no chance to apply before the second walk.
    at: 'plain',
    apply: rewritePage(
      (url) => url.pathname === COURSE_PATH && url.search === '',
      '<a class="y-lesson" href=',
      '<a class="y-lesson y-lesson--here" aria-current="location" href=',
    ),
  },
  // Takes the line away and leaves the points unjoined.
  'take-the-line-away': {
    target: 'the-line-joins-the-points',
    apply: appendRule(
      '.y-lesson::before{border-left-width:0}',
      () => getComputedStyle(document.querySelector('.y-lesson'), '::before').borderLeftWidth,
      '0px',
    ),
  },
  // Leaves the line drawn and slides it off the column the marks sit in, which
  // is a rule beside the course rather than the course read along one.
  'slide-the-line-off-the-points': {
    target: 'the-line-joins-the-points',
    apply: appendRule(
      '.y-lesson::before{left:0}',
      () => getComputedStyle(document.querySelector('.y-lesson'), '::before').left,
      '0px',
    ),
  },
  // Evens the marked point out with the rest, which leaves where the reader is
  // said only to a reader who can hear the page.
  'even-out-the-points': {
    target: 'the-course-marks-the-lesson-it-was-reached-from',
    apply: appendRule(
      '.y-lesson--here .y-navdot{width:7px;height:7px}',
      () => getComputedStyle(document.querySelector('.y-lesson--here .y-navdot')).width,
      '7px',
    ),
  },
  // Widens the course past the phone it is being read on.
  'stretch-the-course-past-the-phone': {
    target: 'the-course-fits-the-phone',
    apply: appendRule(
      '.y-syl{min-width:900px}',
      () => getComputedStyle(document.querySelector('.y-syl')).minWidth,
      '900px',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`course-line: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`course-line: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`course-line: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What a course page says about where the reader is, and where its line runs.
// Every length is read off the laid-out page: the rule is a pseudo-element, so
// its box is asked for through the resolved style and placed against the row it
// belongs to, and the mark's own box comes from the element.
const readCourse = (page) =>
  page.evaluate(() => {
    const rows = [...document.querySelectorAll('.y-lesson')].map((row) => {
      const style = getComputedStyle(row, '::before');
      const box = row.getBoundingClientRect();
      const mark = row.querySelector('.y-navdot, .y-navmark');
      const markBox = mark ? mark.getBoundingClientRect() : null;
      const isPoint = mark ? mark.classList.contains('y-navdot') : false;
      const top = box.top + parseFloat(style.top);
      return {
        text: row.querySelector('.y-lesson__title')?.textContent ?? '',
        here: row.classList.contains('y-lesson--here'),
        current: row.getAttribute('aria-current'),
        listed: row.parentElement.tagName === 'LI',
        ruleWidth: parseFloat(style.borderLeftWidth),
        ruleCentreX: box.left + parseFloat(style.left) + parseFloat(style.borderLeftWidth) / 2,
        ruleTop: top,
        ruleBottom: top + parseFloat(style.height),
        markCentreX: markBox ? markBox.left + markBox.width / 2 : null,
        markCentreY: markBox ? markBox.top + markBox.height / 2 : null,
        markWidth: markBox ? markBox.width : null,
        isPoint,
      };
    });
    // A box pinned to the viewport cannot be what is widening the document, so
    // the width it reports is the page's own to be judged against.
    const gauge = document.createElement('div');
    gauge.style.cssText = 'position:fixed;inset:0;pointer-events:none';
    document.body.append(gauge);
    const viewport = gauge.getBoundingClientRect().width;
    gauge.remove();
    return { rows, viewport, documentWidth: document.documentElement.scrollWidth };
  });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  // A phone, because the narrow width is where a line in a fixed gutter and a
  // row that may wrap to two lines are hardest to keep together.
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  const wayIn = await page.evaluate(() => document.querySelector('.y-railbook__title')?.getAttribute('href') ?? '');
  if (!wayIn) broken(`${PAGE} draws no way into its course, so there is no walk to take`);
  if (!wayIn.startsWith(COURSE_PATH)) {
    broken(`${PAGE} leads to ${wayIn}, which is not the course this probe reads`);
  }

  await page.goto(BASE + wayIn, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  const reached = await readCourse(page);
  if (reached.rows.length < 3) {
    broken(`the course draws ${reached.rows.length} rows, too few for a line to join anything`);
  }
  const proveApplied = async (walk) => {
    if (!proof || (MUTATIONS[MUTATE].at ?? 'reached') !== walk) return;
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  };
  await proveApplied('reached');

  const marked = reached.rows.filter((row) => row.here);
  if (marked.length !== 1 || marked[0].text !== LESSON_NAME) {
    fail(
      'the-course-marks-the-lesson-it-was-reached-from',
      `the course reached from ${LESSON_NAME} marks ${JSON.stringify(marked.map((row) => row.text))}, want exactly [${LESSON_NAME}]`,
    );
  }
  if (marked[0].current !== 'location') {
    fail(
      'the-course-marks-the-lesson-it-was-reached-from',
      `the marked row announces aria-current=${JSON.stringify(marked[0].current)}, so only a reader who can see it is told where they are`,
    );
  }
  // The mark is the heavier point, which is what says so without words. It is
  // weighed against the other points and not against the warning glyph beside
  // a row that opens nothing: that glyph is smaller than any point, so a
  // marked point that had lost its weight would still clear it.
  const others = reached.rows.filter((row) => !row.here && row.isPoint && row.listed);
  if (others.length === 0) broken('no other row draws a point, so nothing says the marked one is heavier');
  if (!marked[0].isPoint) broken('the marked row draws no point of its own to weigh');
  const lightest = Math.min(...others.map((row) => row.markWidth));
  if (!(marked[0].markWidth > lightest)) {
    fail(
      'the-course-marks-the-lesson-it-was-reached-from',
      `the marked point is ${marked[0].markWidth}px across and the others are ${lightest}px, so it is not the heavier one`,
    );
  }

  // The line: every rule sits on the marks' own centre line, and a run's rule
  // runs from the first point to the last without a break.
  for (const row of reached.rows) {
    if (row.markCentreX === null) broken(`the row ${JSON.stringify(row.text)} draws no mark to place a line against`);
    const off = Math.abs(row.ruleCentreX - row.markCentreX);
    if (row.ruleWidth < 1) {
      fail('the-line-joins-the-points', `the rule on ${JSON.stringify(row.text)} is ${row.ruleWidth}px wide, so nothing joins its point to the next`);
    }
    if (off > 0.5) {
      fail(
        'the-line-joins-the-points',
        `the rule on ${JSON.stringify(row.text)} runs at x=${row.ruleCentreX} and its point sits at x=${row.markCentreX}, ${off.toFixed(2)}px apart`,
      );
    }
  }
  const joined = reached.rows.filter((row) => row.ruleBottom - row.ruleTop > 0);
  if (joined.length < 2) {
    broken(`${joined.length} rows carry a line of any length, so a course that joined nothing would read the same`);
  }
  for (const row of joined) {
    const reaches = row.ruleTop <= row.markCentreY + 0.5 && row.ruleBottom >= row.markCentreY - 0.5;
    if (!reaches) {
      fail(
        'the-line-joins-the-points',
        `the line beside ${JSON.stringify(row.text)} runs ${row.ruleTop.toFixed(1)}–${row.ruleBottom.toFixed(1)} and its point is at ${row.markCentreY.toFixed(1)}, so the line passes its own point by`,
      );
    }
  }

  if (reached.documentWidth > reached.viewport) {
    fail(
      'the-course-fits-the-phone',
      `at ${reached.viewport}px the course lays out ${reached.documentWidth}px wide, so the reader has to scroll sideways`,
    );
  }

  // The same course by its own address, which is how the desk links to it.
  await page.goto(BASE + COURSE_PATH, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  const plain = await readCourse(page);
  await proveApplied('plain');
  if (plain.rows.length !== reached.rows.length) {
    broken(`the course draws ${plain.rows.length} rows by its own address and ${reached.rows.length} by the lesson's, so the two are not the same course`);
  }
  const strays = plain.rows.filter((row) => row.here || row.current === 'location');
  if (strays.length > 0) {
    fail(
      'a-course-reached-from-the-desk-marks-nobody',
      `a course opened from the desk marks ${JSON.stringify(strays.map((row) => row.text))}, telling the reader they are somewhere nothing said they were`,
    );
  }

  console.log(
    `PASS course-line: the course reached from ${LESSON_NAME} marks it alone (${marked[0].markWidth}px against ${lightest}px), ${joined.length} rows carry a line on their points' own centre, the page fits ${reached.viewport}px, and its own address marks nobody`,
  );
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
