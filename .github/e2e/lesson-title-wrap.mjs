// Behavior lock for a syllabus lesson name at phone width. The row used to
// nowrap-ellipsis the title beside a whole status badge, so a course whose
// names share a prefix became a column of identical stubs. The name wraps, and
// every line it wraps to is inside the box the reader can see: a title cut
// back to one visible line is the same column of stubs reached another way.
//
// Go tests cannot see this. The probe grows a name against a real lesson row
// until one more word would take a third line, stamps it there, and counts the
// lines the browser laid out against the lines the box shows.
//
// Env: YOMIHON_BASE, PAGE_PATH (a syllabus page that paints .y-lesson__title),
// and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/syllabus/Maps/study.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['title-fits-at-phone-width', 'every-line-of-the-title-is-visible'];

// The words a fixture name is grown from, in the order they are added. How
// many of them it takes is decided against the row itself, further down.
const NAME_WORDS = ['Bits,', 'Bytes,', 'and', 'Words', 'Integers', "Two's", 'Complement', 'and', 'Overflow'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN lesson-title-wrap: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL lesson-title-wrap: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN lesson-title-wrap: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED lesson-title-wrap: ${message}`);
};

// Reads back what the browser resolved the title to, so a rule that was served
// but outranked reports itself rather than passing as a regression nobody
// noticed.
const computedTitle = (page, property) =>
  page.evaluate((name) => {
    const el = document.querySelector('.y-lesson__title');
    return el ? getComputedStyle(el).getPropertyValue(name) : '';
  }, property);

// Appends a rule outside the product layer so it outranks what it stands in
// for, without touching the served file.
const appendRule = (rule, property, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await computedTitle(page, property);
    if (got !== wanted) return `the title resolves ${property} to ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

const MUTATIONS = {
  // Puts the single-line ellipsis back on the title.
  'restore-single-line-ellipsis': {
    target: 'title-fits-at-phone-width',
    apply: appendRule(
      '.y-lesson__title{display:block;-webkit-line-clamp:unset;line-clamp:unset;white-space:nowrap;text-overflow:ellipsis}',
      'white-space',
      'nowrap',
    ),
  },
  // Keeps the wrap and takes back the second line, which is the same stub
  // reached by clipping rather than by refusing to wrap: the box holds one
  // line of a name that lays out in two, and nothing about its width says so.
  'clamp-the-title-to-one-line': {
    target: 'every-line-of-the-title-is-visible',
    apply: appendRule('.y-lesson__title{-webkit-line-clamp:1;line-clamp:1}', '-webkit-line-clamp', '1'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`lesson-title-wrap: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`lesson-title-wrap: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`lesson-title-wrap: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const measureTitle = (page) =>
  page.evaluate((words) => {
    const el = document.querySelector('.y-lesson__title');
    if (!el) return null;
    // One line, measured on the row itself rather than parsed out of a
    // line-height, so every count below is in the units the browser laid out.
    el.textContent = words[0];
    const line = el.scrollHeight;
    if (line <= 0) return { line };
    // The longest name these words make that still lays out in two lines. It is
    // grown here rather than written down because the lesson face is a system
    // stack: the same words come to different widths on a machine with other
    // fonts installed, and a name written to need two lines here could need
    // three there — which the box would clip however well the row behaved.
    // Grown against the row, the fixture is a two-line name wherever it is
    // read, and what is left for the assertions to measure is the box.
    let name = words[0];
    for (const word of words.slice(1)) {
      const longer = `${name} ${word}`;
      el.textContent = longer;
      if (el.scrollHeight > line * 2) break;
      name = longer;
    }
    el.textContent = name;
    const style = getComputedStyle(el);
    return {
      name,
      lines: Math.round(el.scrollHeight / line),
      shown: Math.round(el.clientHeight / line),
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
      face: style.fontFamily,
    };
  }, NAME_WORDS);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 375, height: 812 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  // Every number below is a line count in disguise, and a line count taken
  // while the page is still holding a fallback face is a number about a face
  // no reader will see.
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const box = await measureTitle(page);
  if (box === null) broken('the syllabus page paints no .y-lesson__title to measure');
  if (!box.name) broken('the lesson row lays out no line to grow a name in, so nothing below can be counted');
  if (box.clientWidth <= 0) broken('the title box has no width, so overflow cannot be judged');
  if (box.scrollWidth > box.clientWidth) {
    fail(
      'title-fits-at-phone-width',
      `at 375px .y-lesson__title scrollWidth=${box.scrollWidth} > clientWidth=${box.clientWidth}`,
    );
  }
  // A box narrower than its name says the title refused to wrap; a box shorter
  // than its name says it wrapped and the reader was handed the first line of
  // it. Nothing about the width can see the second, so it is asked for here,
  // and the answer is a count of whole lines rather than a length: a name shown
  // short is short by a line, never by a few pixels of measurement.
  if (box.scrollHeight > box.clientHeight) {
    fail(
      'every-line-of-the-title-is-visible',
      `at 375px .y-lesson__title lays ${JSON.stringify(box.name)} out on ${box.lines} lines and shows ${box.shown} of them, so the reader is handed a stub`,
    );
  }
  // The two sentences above are about a name that needs a second line. If these
  // words no longer make one on this row, in this face, the first is true of a
  // name that never wrapped and says nothing about wrapping.
  if (box.lines < 2) {
    broken(
      `the name grown for this row, ${JSON.stringify(box.name)}, lays out on ${box.lines} line in ${box.face}, so a title that refused to wrap would measure the same`,
    );
  }

  console.log(
    `PASS lesson-title-wrap: ${JSON.stringify(box.name)} wraps to ${box.lines} lines inside its box at 375px and the reader is shown all ${box.shown} of them`,
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
