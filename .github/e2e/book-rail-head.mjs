// Behavior lock for the head of a book rail: the course's name and its extent,
// and nothing else. The name is the strong line and the extent is quiet meta
// beneath it; a hairline closes the head; the part headings under it are
// authored titles, so they are set as sentence-case sans text and never as an
// uppercase, tracked label. Every rail sets them that way, a book's and the
// library sidebar's alike. The rail carries no previous/next pair: the article
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
const SITES = [
  'the-name-outranks-the-extent',
  'the-name-is-never-clipped',
  'part-headings-are-not-labels',
  'every-rail-sets-part-heads-in-sans',
];
// One unbroken run, far wider than the rail: only a title allowed to break
// anywhere holds it, so a rule that stopped wrapping cannot hide behind a name
// that happened to be short.
const LONG_NAME = 'Supercalifragilisticexpialidocious'.repeat(3);
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

// Rewrites one rule in of the served stylesheet. The rule has to be found exactly
// once: a needle that matched nothing would let the mutation pass as applied
// while changing nothing.
const editRule = (needle, replacement) => async (page) => {
  let served = 0;
  let matches = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    served += 1;
    matches = original.split(needle).length - 1;
    await route.fulfill({ response, body: matches === 1 ? original.replace(needle, replacement) : original });
  });
  return async () => {
    if (served === 0) return 'the stylesheet was never requested, so the rule reached no page';
    if (matches !== 1) return `the needle ${needle} matched ${matches} times in the stylesheet, want exactly 1`;
    return '';
  };
};

// Takes a rule out by renaming its selector to one nothing matches.
const dropRule = (needle) => editRule(needle, '.dropped-by-mutation {');

// Adds a rule after everything served, at equal specificity, so it wins.
const appendRule = (rule) => async (page) => {
  let served = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    served += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => (served === 0 ? 'the stylesheet was never requested, so the rule reached no page' : '');
};

// What a part heading used to be: mono capitals with tracking.
const LABEL_FACE = 'font-family:var(--font-mono);letter-spacing:.08em;text-transform:uppercase';

const MUTATIONS = {
  // Takes back the head's own styling: the name and the extent inherit the same
  // body face again.
  'drop-title-rule': { target: 'the-name-outranks-the-extent', apply: dropRule('.y-railbook__title {') },
  // Stops the name wrapping: one line, cut off with an ellipsis.
  'clip-title': {
    target: 'the-name-is-never-clipped',
    apply: appendRule('.y-railbook__title{overflow:hidden;overflow-wrap:normal;text-overflow:ellipsis;white-space:nowrap}'),
  },
  // Puts a book rail's part headings back on an uppercase, tracked mono label.
  'label-face-in-book-rails': {
    target: 'part-headings-are-not-labels',
    apply: appendRule(`#nav-rail:has(.y-railbook) .y-railsummary{${LABEL_FACE}}`),
  },
  // The same label face on the rails that are not a book's, which the book
  // rail's own measurement never reaches.
  'label-face-in-other-rails': {
    target: 'every-rail-sets-part-heads-in-sans',
    apply: appendRule(`#nav-rail:not(:has(.y-railbook)) .y-railsummary{${LABEL_FACE}}`),
  },
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

// Whether a computed font stack is the sans stack the page declares and not the
// mono one. The stack is read back from the page's own token, with the quoting
// the two serialisations disagree about taken out, so the lock follows the face
// the sheet names as sans rather than a family written into this file.
const bare = (stack) => stack.replace(/["']/g, '').replace(/\s+/g, '');
const isSans = (stack, sans) => sans !== '' && bare(stack) === bare(sans) && !/mono/i.test(stack);

const measure = (page) =>
  page.evaluate((longName) => {
    const book = document.querySelector('.y-railbook');
    if (!book) return null;
    const title = book.querySelector('.y-railbook__title');
    const span = book.querySelector('.y-railbook__span');
    const summary = book.querySelector('.y-railsummary');
    if (!title || !span || !summary) return { missing: true };
    title.textContent = longName;
    const t = getComputedStyle(title);
    const e = getComputedStyle(span);
    const s = getComputedStyle(summary);
    return {
      titleSize: parseFloat(t.fontSize),
      summarySize: parseFloat(s.fontSize),
      summaryWeight: Number(s.fontWeight),
      spanSize: parseFloat(e.fontSize),
      titleWeight: Number(t.fontWeight),
      spanWeight: Number(e.fontWeight),
      hairline: e.borderBottomWidth,
      transform: s.textTransform,
      spacing: s.letterSpacing,
      summaryFace: s.fontFamily,
      sansFace: getComputedStyle(document.documentElement).getPropertyValue('--font-sans'),
      titleFits: title.scrollWidth <= title.clientWidth,
      documentFits: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      steps: document.querySelectorAll('.y-railbook nav, #nav-rail .y-lessonsteps').length,
    };
  }, LONG_NAME);

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
    // The name is larger than both lines under it, and heavier than the extent.
    // A part heading is a title too, so it may weigh as much as the name does;
    // it may not weigh more.
    if (!(m.titleSize > m.spanSize && m.titleWeight > m.spanWeight && m.titleSize > m.summarySize && m.titleWeight >= m.summaryWeight)) {
      fail(
        'the-name-outranks-the-extent',
        `at ${width}px the course name computes ${m.titleSize}px/${m.titleWeight} against the extent's ${m.spanSize}px/${m.spanWeight} and a part heading's ${m.summarySize}px/${m.summaryWeight}; the name must be larger than both lines under it, heavier than the extent and no lighter than a part heading`,
      );
    }
    if (m.hairline !== '1px') {
      fail('the-name-outranks-the-extent', `at ${width}px the head closes with a ${m.hairline} border, want a 1px hairline`);
    }
    if (m.transform !== 'none' || m.spacing !== 'normal' || !isSans(m.summaryFace, m.sansFace)) {
      fail(
        'part-headings-are-not-labels',
        `at ${width}px a part heading inside the book rail resolves text-transform=${m.transform}, letter-spacing=${m.spacing}, face=${m.summaryFace}; it reads as an interface label, not as sentence-case sans text`,
      );
    }
    if (!m.titleFits) {
      fail('the-name-is-never-clipped', `at ${width}px a ${LONG_NAME.length}-character unbroken course name overflows its box, so it is clipped or pushes the rail`);
    }
    if (!m.documentFits) broken(`at ${width}px the page scrolls sideways`);
  }

  // The other rails set part headings the same way: the library sidebar that
  // /health draws is not left on a label face of its own.
  await page.goto(`${BASE}/health`, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = await proof();
    if (issue) throw new NotApplied(`NOT-APPLIED book-rail-head: ${MUTATE}: ${issue}`);
  }
  const other = await page.evaluate(() => {
    const el = document.querySelector('#nav-rail .y-railsummary');
    if (!el) return null;
    const s = getComputedStyle(el);
    return {
      transform: s.textTransform,
      spacing: s.letterSpacing,
      face: s.fontFamily,
      sans: getComputedStyle(document.documentElement).getPropertyValue('--font-sans'),
      book: document.querySelectorAll('#nav-rail .y-railbook').length,
    };
  });
  if (other === null) broken('/health draws no sidebar part heading to measure');
  if (other.book !== 0) broken('/health draws a book rail, so it cannot stand for the other rails');
  if (other.transform !== 'none' || other.spacing !== 'normal' || !isSans(other.face, other.sans)) {
    fail(
      'every-rail-sets-part-heads-in-sans',
      `a sidebar part heading outside a book rail resolves text-transform=${other.transform}, letter-spacing=${other.spacing}, face=${other.face}; every rail sets part headings as sentence-case sans text`,
    );
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
