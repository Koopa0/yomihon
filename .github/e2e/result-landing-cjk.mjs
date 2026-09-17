// Behavior lock: clicking a search result written in Chinese puts the
// characters that were searched for on the screen. The reading chrome speaks
// Traditional Chinese and so do the notes it was built for, and this is the
// only place a browser is asked whether a result in that language arrives.
//
// What makes it its own lock rather than a second width of the English one is
// the shape of the text. Chinese parts no words with spaces, so the run a
// match follows is one unbroken stretch of characters, and a browser looks
// for a leading run only where the run begins at a word boundary — which in
// that script is a dictionary boundary, not a space. A run that begins
// between two characters of one word is found nowhere, and a leading run the
// browser cannot find costs it the term as well: the note opens at the top
// with the searched-for characters two thousand pixels below the screen.
//
// So the assertion is geometry rather than the href: a plausible-looking
// address and any nonzero scroll both pass while the evidence stays out of
// sight. The mutation below writes the run back in, which is the one regression
// this file exists to catch.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=%E7%8D%A8%E8%A7%92%E7%8D%B8';
const MUTATE = process.env.MUTATE || '';
const SITES = ['match-in-view'];

// The searched-for characters, the run the match follows inside its own
// block, and the last thirty characters of that run — which is where a
// character budget spent from the end lands, between 咖 and 啡.
const TERM = '獨角獸';
const BLOCK_RUN = '那本咖啡色封皮的舊冊子在星期四早晨被翻開來，接著在這一行裡面才輪到';
const CUT_RUN = BLOCK_RUN.slice(BLOCK_RUN.length - 30);
const WIDTH = 1600;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN result-landing-cjk: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL result-landing-cjk: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN result-landing-cjk: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED result-landing-cjk: ${message}`); };

// rewritePath proves it applied by counting, so a needle that rots against a
// rewritten directive turns this run red rather than quietly mutating nothing.
const rewritePath = (path, needle, replacement, expected, label) => async (page) => {
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
    if (matches !== expected * requests) return `${label} needle matched ${matches} times over ${requests} request(s), want ${expected} each`;
    return '';
  };
};

const MUTATIONS = {
  // The regression itself: the directive carries the run the match follows,
  // cut to a character budget, which in this sentence opens inside 咖啡色.
  'name-the-cut-run': {
    target: 'match-in-view',
    apply: rewritePath(
      PAGE,
      `text=${encodeURIComponent(TERM)}`,
      `text=${encodeURIComponent(CUT_RUN)}-,${encodeURIComponent(TERM)}`,
      1,
      'result directive',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`result-landing-cjk: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`result-landing-cjk: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`result-landing-cjk: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// wordCopies measures a range around each copy of the characters, inside the
// element asked for, so the answer is where the reader's evidence sits rather
// than where its paragraph does. Every copy is reported: a bare term is
// answered by the first one the browser walks past, so a page that grew a
// second copy would let this probe pass on whichever it happened to pick.
const wordCopies = (page, selector, word) => page.evaluate(([sel, w]) => {
  const root = document.querySelector(sel);
  if (!root) return null;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const found = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    for (let i = node.textContent.indexOf(w); i >= 0; i = node.textContent.indexOf(w, i + 1)) {
      const range = document.createRange();
      range.setStart(node, i);
      range.setEnd(node, i + w.length);
      const box = range.getBoundingClientRect();
      found.push({
        top: Math.round(box.top),
        bottom: Math.round(box.bottom),
        // Whether a reader looking at the screen can see it. The scroll
        // position alone cannot say this: a page scrolled to the wrong place
        // has moved just as far as one scrolled to the right place.
        inView: box.top >= 0 && box.bottom <= innerHeight && box.height > 0,
      });
    }
  }
  return found;
}, [selector, word]);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const apply = MUTATE ? MUTATIONS[MUTATE].apply : null;
  const context = await browser.newContext({ viewport: { width: WIDTH, height: 900 } });
  const page = await context.newPage();
  proof = apply ? await apply(page) : null;

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  const link = page.locator('a.y-result[href*="%E7%81%B0%E5%B8%83%E5%B8%B3%E5%86%8A"]').first();
  if (await link.count() !== 1) broken('the results do not offer the Chinese note this probe was written around');
  const href = await link.getAttribute('href');

  await link.click();
  await page.waitForURL(/%E7%81%B0%E5%B8%83%E5%B8%B3%E5%86%8A/);
  await page.waitForLoadState('load');
  // Settling is watched rather than assumed, because a directive that is
  // never honoured leaves the scroll at rest immediately and would otherwise
  // be measured before a working one had moved.
  let scroll = -1;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(150);
    const now = await page.evaluate(() => Math.round(scrollY));
    if (i > 3 && now === scroll) break;
    scroll = now;
  }

  // Nothing else on the page may spell those characters. The directive here
  // carries the bare term, and the browser stops at the first copy it walks
  // past — a copy in the navigation, which is drawn before the article, would
  // answer for the article's and leave this probe measuring the wrong one.
  const everywhere = await wordCopies(page, 'body', TERM);
  if (!everywhere) broken('the note page rendered no body to measure');
  if (everywhere.length !== 1) {
    broken(`the page holds ${everywhere.length} copies of ${JSON.stringify(TERM)}, want exactly 1, so a bare term could be answered by the wrong one`);
  }
  const inArticle = await wordCopies(page, 'main article', TERM);
  if (!inArticle || inArticle.length !== 1) {
    broken(`the article holds ${inArticle ? inArticle.length : 'no'} copies of ${JSON.stringify(TERM)}, want exactly 1`);
  }
  const seen = { href, scrollY: scroll, match: inArticle[0] };
  if (!inArticle[0].inView) {
    fail('match-in-view', `the result did not bring the searched-for characters on screen: ${JSON.stringify(seen)}`);
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  await context.close();
  console.log(`PASS result-landing-cjk: a result click puts ${TERM} on screen at ${WIDTH}px`);
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED result-landing-cjk: ${MUTATE}: ${issue}`);
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
      await browser.close();
      process.exit(2);
    }
  }
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
