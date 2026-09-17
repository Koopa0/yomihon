// Behavior lock: a search whose phrase stops inside a word at its far end
// still puts that phrase on the screen. The reader typed a run that crosses
// paragraphs and cut the last word short; the note holding it is long enough
// that the run sits well below the opening.
//
// A phrase that crosses paragraphs is not one term a browser can find, so the
// address names the two ends and asks for the range between them. The far end
// is held to both its edges — it arrives with nothing ahead of it to relax
// that — so a word cut short is asked for at a place the page has nowhere, and
// the browser drops the whole request rather than the one end it could not
// match. The reader then lands at the top of a long note with nothing said.
// The far end goes out grown to the word instead.
//
// This is the same fixture and the same three paragraphs the whole-word phrase
// is driven against; only the query differs, and that difference is the point.
//
// The assertion is geometry, not the href. A correct-looking address and any
// nonzero scroll both pass while the words stay below the screen, and the one
// thing the reader is owed is seeing them. The probe refuses to run at all
// unless they start off-screen, because an assertion that they are visible
// proves nothing on a page where they were visible all along.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=%22alpha%20beta%20gamm%22';
const MUTATE = process.env.MUTATE || '';
const SITES = ['range-ends-in-view'];

// The two ends of the match as the article spells them, and the run of words
// the note has ahead of the first, which the address carries so a rail label
// spelling that one word cannot answer for it.
const START_WORD = 'alpha';
const END_WORD = 'gamma';
const TYPED_END = 'gamm';
const DIRECTIVE = `text=entry%20closes%20with-,${START_WORD},${END_WORD}`;
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN range-end-landing: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL range-end-landing: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN range-end-landing: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED range-end-landing: ${message}`); };

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
  // The regression itself: the far end shrinks back to the run the reader
  // typed, which stops inside a word and so is looked for nowhere.
  'shrink-the-far-end': {
    target: 'range-ends-in-view',
    apply: rewritePath(PAGE, DIRECTIVE, `text=entry%20closes%20with-,${START_WORD},${TYPED_END}`, 1, 'result directive'),
  },
  // With no address to honour the note opens at the top, which is what the
  // whole lock measures the absence of. It is the answer to "would this have
  // passed anyway": if it would, this mutation walks past.
  'strip-the-directive': {
    target: 'range-ends-in-view',
    apply: rewritePath(PAGE, `#:~:${DIRECTIVE}`, '', 1, 'result directive'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`range-end-landing: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`range-end-landing: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`range-end-landing: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// articleWord measures a range around one word inside the article, so the
// answer is where the reader's evidence sits rather than where its paragraph
// does. It reports every copy: a fixture that grew a second one would let this
// probe pass on whichever the browser happened to pick.
const articleWord = (page, word) => page.evaluate((w) => {
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
        // How far down the document it sits, which does not move when the
        // page does, and whether a reader looking at the screen can see it.
        // The scroll position alone cannot say the second: a page scrolled to
        // the wrong place has moved just as far as one scrolled to the right
        // place.
        top: Math.round(box.top + scrollY),
        inView: box.top >= 0 && box.bottom <= innerHeight && box.height > 0,
      });
    }
  }
  return found;
}, word);

const oneArticleWord = async (page, word) => {
  const found = await articleWord(page, word);
  if (!found) broken('the note page rendered no article to measure');
  if (found.length !== 1) broken(`the article holds ${found.length} copies of ${JSON.stringify(word)}, want exactly 1`);
  return found[0];
};

// settle waits for the browser to finish placing the page. It is watched
// rather than assumed, because a request that is never honoured leaves the
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
    broken('the fixture search does not offer the note this probe was written around');
  }
  const href = await link.getAttribute('href');

  await link.click();
  await page.waitForURL(/Audit%20cross%20block/);
  await page.waitForLoadState('load');
  const seen = {
    href,
    scrollY: await settle(page),
    start: await oneArticleWord(page, START_WORD),
    end: await oneArticleWord(page, END_WORD),
  };
  await context.close();

  // The words have to start off the screen for "they are on the screen" to
  // mean anything. Their distance down the document answers that without a
  // second visit, so a fixture whose foot crept above the fold reports here
  // rather than passing quietly ever after.
  for (const [word, at] of [[START_WORD, seen.start], [END_WORD, seen.end]]) {
    if (at.top < HEIGHT) {
      broken(`${JSON.stringify(word)} sits ${at.top}px down a ${HEIGHT}px screen, so it is in view before anything scrolls and this probe would pass without landing anywhere`);
    }
  }
  return { seen, proof };
};

// The query has to be one whose far end stops inside a word, or this probe is
// a second copy of the whole-word phrase beside it and locks nothing new.
if (!PAGE.includes(`${TYPED_END}%22`) || PAGE.includes(`${END_WORD}%22`)) {
  console.error(`BROKEN range-end-landing: ${PAGE} does not end the phrase inside ${JSON.stringify(END_WORD)}`);
  process.exit(1);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const apply = MUTATE ? MUTATIONS[MUTATE].apply : null;
  const landed = await land(browser, apply);
  proof = landed.proof;
  if (!landed.seen.start.inView || !landed.seen.end.inView) {
    fail('range-ends-in-view', `a phrase whose far end stops inside a word left it off the screen: ${JSON.stringify(landed.seen)}`);
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log(`PASS range-end-landing: searching a phrase cut at ${TYPED_END} puts ${START_WORD} and ${END_WORD} on screen at ${WIDTH}px`);
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED range-end-landing: ${MUTATE}: ${issue}`);
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
