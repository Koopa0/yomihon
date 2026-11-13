// Behavior lock for the note's facts at the end of its text: at phone width the
// list of them fits the window, with a long path wrapping rather than widening
// the page, and with no script running at all the dt/dd pairs are there, drawn,
// and in order after the prose, because they are the content that arrives.
//
// Neither half is reachable from a Go test. Fitting is a number only a laid-out
// page has, and "no script running" is a browser context Go never drives.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note whose end carries every fact it can: type,
// language, the author's checked day, prerequisites, and the raw path), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const WIDTH = 390;
const SITES = ['the-facts-fit-the-phone-viewport', 'the-facts-arrive-with-no-script-running'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN note-end-facts: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL note-end-facts: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN note-end-facts: ${message}`);
};

// Appends a rule to the served stylesheet and reads back what the browser
// resolved it to, so a rule that was served but outranked reports itself.
const appendRule = (rule, read, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
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

const MUTATIONS = {
  // The list is wider than the phone it is read on.
  'stretch-the-facts-past-the-phone': {
    target: 'the-facts-fit-the-phone-viewport',
    apply: appendRule('.y-notefacts{min-width:900px}', () => getComputedStyle(document.querySelector('.y-notefacts')).minWidth, '900px'),
  },
  // The facts wait for a script to draw them.
  'draw-the-facts-only-once-a-script-has-run': {
    target: 'the-facts-arrive-with-no-script-running',
    apply: appendRule(':root:not([data-js]) .y-facts{display:none}', () => getComputedStyle(document.querySelector('.y-facts')).display, 'none'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`note-end-facts: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`note-end-facts: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`note-end-facts: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const prove = async (proof, what) => {
  if (!proof) return;
  const issue = await proof();
  if (issue) throw new NotApplied(`NOT-APPLIED note-end-facts: ${MUTATE}: ${what}: ${issue}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: the phone reading, script running. The facts list has to sit inside
  // the window, and the page must be exactly as wide as the phone.
  {
    const context = await browser.newContext({ viewport: { width: WIDTH, height: 844 } });
    const page = await context.newPage();
    const proof = MUTATE === 'stretch-the-facts-past-the-phone' ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await page.evaluate(() => document.fonts.ready);
    await prove(proof, 'the phone reading');
    const measured = await page.evaluate(() => {
      const list = document.querySelector('.y-related .y-notefacts');
      if (!list) return null;
      const box = list.getBoundingClientRect();
      return { left: box.left, right: box.right, rows: list.querySelectorAll('dt').length, document: document.documentElement.scrollWidth, window: innerWidth };
    });
    if (measured === null) broken(`${PAGE} has no facts list at the end of its text`);
    if (measured.rows < 4) broken(`${PAGE} carries ${measured.rows} facts, want a note that declares every one`);
    if (measured.left < 0 || measured.right > measured.window + 0.5) {
      fail('the-facts-fit-the-phone-viewport', `the list spans ${measured.left}..${measured.right} in a ${measured.window}px window`);
    }
    if (measured.document > measured.window + 0.5) {
      fail('the-facts-fit-the-phone-viewport', `the page is ${measured.document}px wide in a ${measured.window}px window`);
    }
    await context.close();
  }

  // Case 2: no script running at all. The pairs are drawn, after the prose, with
  // their words and the author's checked day among them.
  {
    const context = await browser.newContext({ viewport: { width: WIDTH, height: 844 }, javaScriptEnabled: false });
    const page = await context.newPage();
    const proof = MUTATE === 'draw-the-facts-only-once-a-script-has-run' ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await prove(proof, 'the reading with no script');
    const arrived = await page.evaluate(() => {
      const list = document.querySelector('.y-related .y-notefacts');
      const prose = document.querySelector('.y-prose');
      if (!list || !prose) return null;
      return {
        drawn: list.checkVisibility(),
        afterTheProse: Boolean(prose.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING),
        pairs: [...list.querySelectorAll('dt')].map((dt) => dt.checkVisibility() && dt.nextElementSibling?.checkVisibility()),
        day: list.querySelector('time')?.getAttribute('datetime') ?? null,
      };
    });
    if (arrived === null) broken(`${PAGE} has no facts list at the end of its text with scripting off`);
    if (!arrived.drawn || arrived.pairs.some((shown) => !shown)) {
      fail('the-facts-arrive-with-no-script-running', `with no script running the facts are not all drawn: ${JSON.stringify(arrived)}`);
    }
    if (!arrived.afterTheProse) fail('the-facts-arrive-with-no-script-running', 'the facts do not follow the prose');
    if (arrived.day === null) broken(`${PAGE} shows no checked day, so the author's day is not judged`);
    await context.close();
  }
  console.log(`PASS note-end-facts: at ${WIDTH}px the facts at the end of the note fit the window and the page, and with no script running they are all drawn, after the prose, with the author's day`);
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    if (MUTATE && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
