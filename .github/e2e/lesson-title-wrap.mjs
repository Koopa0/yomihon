// Behavior lock for a syllabus lesson name at phone width. The row used to
// nowrap-ellipsis the title beside a whole status badge, so a course whose
// names share a prefix became a column of identical stubs. The name wraps to
// at most two lines; the badge stays on the auto track and does not wrap.
//
// Go tests cannot see this. The probe stamps a deliberately long title onto
// a real lesson row and reads the box the browser painted.
//
// Env: YOMIHON_BASE, PAGE_PATH (a syllabus page that paints .y-lesson__title),
// and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/syllabus/Maps/study.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['title-fits-at-phone-width'];

// Longer than one 225px line at the 20px lesson face, short enough that two
// wrapped lines still hold it. The issue's own samples overflow a single line
// at 375px; this string is those samples joined so a wrap-without-clamp is
// not required for the lock to have something to measure.
const LONG_TITLE = "Bits, Bytes, and Words Integers Two's Complement and Overflow";

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

// Puts the single-line ellipsis back on the title. Appended outside the
// product layer so it outranks the wrap without touching the served file.
const restoreSingleLineEllipsis = async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({
      response,
      body: `${original}\n.y-lesson__title{display:block;-webkit-line-clamp:unset;line-clamp:unset;white-space:nowrap;text-overflow:ellipsis}\n`,
    });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the nowrap rule reached no page');
};

const MUTATIONS = {
  'restore-single-line-ellipsis': {
    target: 'title-fits-at-phone-width',
    apply: restoreSingleLineEllipsis,
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
  page.evaluate((title) => {
    const el = document.querySelector('.y-lesson__title');
    if (!el) return null;
    el.textContent = title;
    return {
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
      text: el.textContent,
    };
  }, LONG_TITLE);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 375, height: 812 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const box = await measureTitle(page);
  if (box === null) broken('the syllabus page paints no .y-lesson__title to measure');
  if (box.text !== LONG_TITLE) {
    broken(`the stamped title is ${JSON.stringify(box.text)}, want the long fixture name`);
  }
  if (box.clientWidth <= 0) broken('the title box has no width, so overflow cannot be judged');
  if (box.scrollWidth > box.clientWidth) {
    fail(
      'title-fits-at-phone-width',
      `at 375px .y-lesson__title scrollWidth=${box.scrollWidth} > clientWidth=${box.clientWidth}`,
    );
  }

  console.log('PASS lesson-title-wrap: a long lesson title fits its box at 375px');
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
