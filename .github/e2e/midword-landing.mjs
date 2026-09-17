// Behavior lock: a search whose match opens inside a word still puts that
// word on the screen. The reader typed the tail of a long word, and the note
// holding it is long enough that the word sits well below the opening.
//
// A text directive is a request to a browser, and the browser asks something
// of a term that arrives with no words ahead of it: it stops only where a word
// begins and where one ends. The tail of a word satisfies neither, so the
// directive used to name a stretch the page has nowhere and the note opened at
// the top, with the evidence the reader searched for still below the screen.
// The term goes out as the whole word instead.
//
// The block this match sits in is the reason the term has no words ahead of
// it: the page draws a ruby reading in among the characters it is spoken over,
// so a run named beside the match would be a run the page never shows in one
// piece. That is what makes the fixture's foot the right place to measure.
//
// The assertion is geometry, not the href. A correct-looking address and any
// nonzero scroll both pass while the word stays below the screen, and the one
// thing the reader is owed is seeing it. The probe refuses to run at all
// unless that word starts off-screen, because an assertion that the word is
// visible proves nothing on a page where it was visible all along.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=lybdenum';
const MUTATE = process.env.MUTATE || '';
const SITES = ['searched-word-in-view'];

// What the reader typed, and the word the note actually holds it in.
const TYPED = 'lybdenum';
const WORD = 'molybdenum';
const WIDTH = 1100;
const HEIGHT = 900;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN midword-landing: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL midword-landing: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN midword-landing: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED midword-landing: ${message}`); };

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
  // The regression itself: the term shrinks back to the stretch the reader
  // typed, which opens inside a word and so is looked for nowhere.
  'strip-the-growth': {
    target: 'searched-word-in-view',
    apply: rewritePath(PAGE, `text=${WORD}`, `text=${TYPED}`, 1, 'result directive'),
  },
  // With no directive at all the note opens at the top, which is what the
  // whole lock measures the absence of. It is the answer to "would this have
  // passed anyway": if it would, this mutation walks past.
  'strip-the-directive': {
    target: 'searched-word-in-view',
    apply: rewritePath(PAGE, `#:~:text=${WORD}`, '', 1, 'result directive'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`midword-landing: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`midword-landing: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`midword-landing: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// articleWord measures a range around the word inside the article, so the
// answer is where the reader's evidence sits rather than where its paragraph
// does. It reports every copy: a fixture that grew a second one would let this
// probe pass on whichever the browser happened to pick.
const articleWord = (page) => page.evaluate((w) => {
  const article = document.querySelector('main article') || document.querySelector('main');
  if (!article) return null;
  const walker = document.createTreeWalker(article, NodeFilter.SHOW_TEXT);
  const found = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    for (let i = node.textContent.indexOf(w); i >= 0; i = node.textContent.indexOf(w, i + 1)) {
      const range = document.createRange();
      range.setStart(node, i);
      range.setEnd(node, i + w.length);
      const box = range.getBoundingClientRect();
      found.push({
        top: Math.round(box.top + scrollY),
        // Whether a reader looking at the screen can see it. The scroll
        // position alone cannot say this: a page scrolled to the wrong place
        // has moved just as far as one scrolled to the right place.
        inView: box.top >= 0 && box.bottom <= innerHeight && box.height > 0,
      });
    }
  }
  return found;
}, WORD);

const oneArticleWord = async (page) => {
  const found = await articleWord(page);
  if (!found) broken('the note page rendered no article to measure');
  if (found.length !== 1) broken(`the article holds ${found.length} copies of ${JSON.stringify(WORD)}, want exactly 1`);
  return found[0];
};

// settle waits for the browser to finish placing the page. It is watched
// rather than assumed, because a directive that is never honoured leaves the
// scroll at rest immediately and would otherwise be measured before a working
// one had moved.
const settle = async (page) => {
  let last = -1;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(150);
    const now = await page.evaluate(() => Math.round(scrollY));
    if (i > 3 && now === last) break;
    last = now;
  }
  return last;
};

// land drives the flow a reader drives: type nothing, open the results the
// query already produced, click the row, and see where the note opens.
const land = async (browser, apply) => {
  const context = await browser.newContext({ viewport: { width: WIDTH, height: HEIGHT } });
  const page = await context.newPage();
  const proof = apply ? await apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const link = page.locator('a.y-result[href*="Audit%20cross%20block"]').first();
  if (await link.count() !== 1) {
    broken(`searching ${JSON.stringify(TYPED)} does not offer the note this probe was written around`);
  }
  const href = await link.getAttribute('href');

  await link.click();
  await page.waitForURL(/Audit%20cross%20block/);
  await page.waitForLoadState('load');
  const seen = { href, scrollY: await settle(page), word: await oneArticleWord(page) };
  await context.close();

  // The word has to start off the screen for "it is on the screen" to mean
  // anything. Its distance down the document answers that without a second
  // visit, so a fixture whose foot crept above the fold reports here rather
  // than passing quietly ever after.
  if (seen.word.top < HEIGHT) {
    broken(`${JSON.stringify(WORD)} sits ${seen.word.top}px down a ${HEIGHT}px screen, so it is in view before anything scrolls and this probe would pass without landing anywhere`);
  }
  return { seen, proof };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const apply = MUTATE ? MUTATIONS[MUTATE].apply : null;
  const landed = await land(browser, apply);
  proof = landed.proof;
  if (!landed.seen.word.inView) {
    fail('searched-word-in-view', `a match opening inside a word left ${JSON.stringify(WORD)} off the screen: ${JSON.stringify(landed.seen)}`);
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log(`PASS midword-landing: searching ${TYPED} puts ${WORD} on screen at ${WIDTH}px`);
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED midword-landing: ${MUTATE}: ${issue}`);
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
