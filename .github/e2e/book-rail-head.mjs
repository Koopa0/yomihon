// Behavior lock for the head of a book rail: the course's name and its extent,
// and nothing else. The name is the strong line and the extent is quiet meta
// beneath it; a hairline closes the head; the part headings under it are
// authored titles, so they read in the sans face and are not uppercased the way
// an interface label is. The rail carries no previous/next pair: the article
// foot is the one place a lesson offers the step onward.
//
// Go tests cannot see this: what matters is what the browser resolved the
// rules to, and whether the name is clipped or the page scrolls sideways.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson page inside a course), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Course/C02.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['the-name-outranks-the-extent', 'part-headings-are-not-labels'];
const WIDTHS = [1440, 1024, 901];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN book-rail-head: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL book-rail-head: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN book-rail-head: ${message}`);
};

// Takes one rule out of the served stylesheet. The rule has to be found exactly
// once: a needle that matched nothing would let the mutation pass as applied
// while changing nothing.
const dropRule = (needle) => async (page) => {
  let served = 0;
  let matches = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    served += 1;
    matches = original.split(needle).length - 1;
    await route.fulfill({ response, body: matches === 1 ? original.replace(needle, '.dropped-by-mutation{') : original });
  });
  return async () => {
    if (served === 0) return 'the stylesheet was never requested, so the rule reached no page';
    if (matches !== 1) return `the needle ${needle} matched ${matches} times in the stylesheet, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  // Takes back the head's own styling: the name and the extent inherit the same
  // body face again.
  'drop-title-rule': { target: 'the-name-outranks-the-extent', apply: dropRule('.y-railbook__title{') },
  // Puts part headings back on the interface label face.
  'drop-summary-override': { target: 'part-headings-are-not-labels', apply: dropRule('.y-railbook .y-railsummary{') },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`book-rail-head: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`book-rail-head: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`book-rail-head: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const measure = (page) =>
  page.evaluate(() => {
    const book = document.querySelector('.y-railbook');
    if (!book) return null;
    const title = book.querySelector('.y-railbook__title');
    const span = book.querySelector('.y-railbook__span');
    const summary = book.querySelector('.y-railsummary');
    if (!title || !span || !summary) return { missing: true };
    const t = getComputedStyle(title);
    const e = getComputedStyle(span);
    const s = getComputedStyle(summary);
    return {
      titleSize: parseFloat(t.fontSize),
      spanSize: parseFloat(e.fontSize),
      titleWeight: Number(t.fontWeight),
      spanWeight: Number(e.fontWeight),
      hairline: e.borderBottomWidth,
      transform: s.textTransform,
      spacing: s.letterSpacing,
      summaryFace: s.fontFamily,
      spanFace: e.fontFamily,
      titleFits: title.scrollWidth <= title.clientWidth,
      documentFits: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      steps: document.querySelectorAll('.y-railbook nav, #nav-rail .y-lessonsteps').length,
    };
  });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: WIDTHS[0], height: 900 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  for (const width of WIDTHS) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => document.fonts.ready);
    if (proof) {
      const issue = await proof();
      if (issue) throw new NotApplied(`NOT-APPLIED book-rail-head: ${MUTATE}: ${issue}`);
    }
    const m = await measure(page);
    if (m === null) broken(`${PAGE} carries no book rail`);
    if (m.missing) broken('the book rail draws no title, extent and part heading to measure');

    if (m.steps !== 0) {
      broken(`at ${width}px the book rail carries ${m.steps} step navigation(s); the foot is the one place a lesson offers the step onward`);
    }
    if (!(m.titleSize > m.spanSize && m.titleWeight > m.spanWeight)) {
      fail(
        'the-name-outranks-the-extent',
        `at ${width}px the course name computes ${m.titleSize}px/${m.titleWeight} against the extent's ${m.spanSize}px/${m.spanWeight}; the name must be larger and heavier`,
      );
    }
    if (m.hairline !== '1px') {
      fail('the-name-outranks-the-extent', `at ${width}px the head closes with a ${m.hairline} border, want a 1px hairline`);
    }
    if (m.transform !== 'none' || m.spacing !== 'normal' || m.summaryFace === m.spanFace) {
      fail(
        'part-headings-are-not-labels',
        `at ${width}px a part heading inside the book rail resolves text-transform=${m.transform}, letter-spacing=${m.spacing}, face=${m.summaryFace}; it reads as an interface label`,
      );
    }
    if (!m.titleFits) broken(`at ${width}px the course name is clipped`);
    if (!m.documentFits) broken(`at ${width}px the page scrolls sideways`);
  }

  // The name is a link, and reaching it from the keyboard has to show a ring.
  await page.setViewportSize({ width: WIDTHS[0], height: 900 });
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  let ring = null;
  for (let i = 0; i < 40 && ring === null; i += 1) {
    await page.keyboard.press('Tab');
    ring = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el || !el.matches('.y-railbook__title')) return null;
      const s = getComputedStyle(el);
      return { style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
    });
  }
  if (ring === null) broken('Tab never reached the course name');
  if (ring.style === 'none' || ring.width < 1) broken(`the course name shows no focus ring from the keyboard: ${JSON.stringify(ring)}`);

  console.log(`PASS book-rail-head: at ${WIDTHS.join(', ')}px the name outranks the extent, part headings read as titles, the head is not clipped and shows a focus ring`);
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
