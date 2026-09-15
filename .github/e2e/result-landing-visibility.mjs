// Behavior lock: clicking a search result puts the words that were searched
// for on the screen. The note here is long enough that its match starts about
// two thousand pixels down, and the phrase crosses paragraphs, so the link
// carries a text directive rather than a plain address.
//
// A directive is a request to a browser, not an instruction: it walks the
// whole page in order and stops at the first copy of the stretch it was given.
// The reader's navigation is drawn before the article and this fixture's rail
// carries a label spelling one of the searched-for words, so a directive whose
// opening stretch is that one bare word is answered by the rail and the
// article never scrolls. That is why the assertion is geometry and not the
// href: a correct-looking address and any nonzero scroll both pass while the
// evidence stays below the screen.
//
// The narrow width is the control. The rail is not drawn there, nothing
// competes, and the same link has always landed — so a change that only
// appears to work because the competition is gone is visible as a wide leg
// that fails while the narrow one passes.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=%22alpha%20beta%20gamma%22';
const MUTATE = process.env.MUTATE || '';
const SITES = ['narrow-endpoints-in-view', 'wide-endpoints-in-view'];

// The article's own copies of the two ends of the match, and the rail label
// that competes with the first of them.
const START_WORD = 'alpha';
const END_WORD = 'gamma';
const RAIL_LABEL = 'alpha';
const NARROW = 390;
const WIDE = 1600;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN result-landing-visibility: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL result-landing-visibility: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN result-landing-visibility: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED result-landing-visibility: ${message}`); };

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
  // The regression itself: the directive opens on one bare word again. The
  // rail answers for it at the wide width, and the narrow width, having no
  // rail, must still land — which is what makes it the control rather than a
  // second copy of the same assertion.
  'strip-the-prefix': {
    target: 'wide-endpoints-in-view',
    apply: rewritePath(PAGE, 'text=entry%20closes%20with-,', 'text=', 1, 'result prefix'),
  },
  // Without any directive the note opens at the top at either width, which is
  // what the whole lock is measuring the absence of. It is aimed at the narrow
  // leg because that leg runs first: a mutation that broke only the wide one
  // could not tell a working control from an assertion that never ran.
  'strip-the-directive': {
    target: 'narrow-endpoints-in-view',
    apply: rewritePath(PAGE, '#:~:text=entry%20closes%20with-,alpha,gamma', '', 1, 'result directive'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`result-landing-visibility: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`result-landing-visibility: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`result-landing-visibility: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// articleWord measures a range around one word inside the article, so the
// answer is where the reader's evidence sits rather than where its paragraph
// does. It reports every copy it finds: a fixture that grew a second one would
// let this probe pass on whichever the browser happened to pick.
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
}, word);

const oneArticleWord = async (page, word) => {
  const found = await articleWord(page, word);
  if (!found) broken('the note page rendered no article to measure');
  if (found.length !== 1) broken(`the article holds ${found.length} copies of ${JSON.stringify(word)}, want exactly 1`);
  return found[0];
};

// railLabelShown answers whether the navigation is on the screen carrying the
// word that competes with the article's. At the narrow width the rail is a
// drawer parked off the side of the screen, so it keeps a box and a computed
// style that read as visible; only asking what the pointer would land on tells
// a parked drawer from an open one. A rail that is absent altogether is an
// answer here rather than a fault.
const railLabelShown = (page, label) => page.evaluate((text) => {
  const rail = document.querySelector('.y-rail-left');
  if (!rail) return false;
  return [...rail.querySelectorAll('a span')].some((el) => {
    if (el.textContent.trim() !== text) return false;
    if (!el.checkVisibility({ opacityProperty: true, visibilityProperty: true, contentVisibilityAuto: true })) return false;
    const box = el.getBoundingClientRect();
    if (box.width <= 1 || box.height <= 1) return false;
    const x = box.left + box.width / 2;
    const y = box.top + box.height / 2;
    if (x < 0 || y < 0 || x > innerWidth || y > innerHeight) return false;
    const hit = document.elementFromPoint(x, y);
    return hit === el || el.contains(hit);
  });
}, label);

// land drives the flow a reader drives: open the results, click the row, and
// wait for the browser to finish placing the page. Settling is watched rather
// than assumed, because a directive that is never honoured leaves the scroll
// at rest immediately and would otherwise be measured before a working one
// had moved.
const land = async (browser, width, apply) => {
  const context = await browser.newContext({ viewport: { width, height: 900 } });
  const page = await context.newPage();
  const proof = apply ? await apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const rows = page.locator('ol.y-results > li a.y-result');
  const rowCount = await rows.count();
  if (rowCount < 1) broken(`the fixture search returned ${rowCount} rows, want the long note`);
  const link = page.locator('a.y-result[href*="Audit%20cross%20block"]').first();
  if (await link.count() !== 1) broken('the results do not offer the long note this probe was written around');
  const href = await link.getAttribute('href');

  await link.click();
  await page.waitForURL(/Audit%20cross%20block/);
  await page.waitForLoadState('load');
  let last = -1;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(150);
    const now = await page.evaluate(() => Math.round(scrollY));
    if (i > 3 && now === last) break;
    last = now;
  }
  const seen = {
    width,
    href,
    scrollY: last,
    rail: await railLabelShown(page, RAIL_LABEL),
    start: await oneArticleWord(page, START_WORD),
    end: await oneArticleWord(page, END_WORD),
  };
  await context.close();
  return { seen, proof };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const apply = MUTATE ? MUTATIONS[MUTATE].apply : null;

  // The narrow leg runs first, so a mutation aimed at the wide one has to
  // leave this leg passing to be counted as caught.
  const narrow = await land(browser, NARROW, apply);
  proof = narrow.proof;
  if (narrow.seen.rail) {
    broken(`the rail is on screen at ${NARROW}px carrying ${JSON.stringify(RAIL_LABEL)}, so this leg is not the control it is written as`);
  }
  if (!narrow.seen.start.inView || !narrow.seen.end.inView) {
    fail('narrow-endpoints-in-view', `at ${NARROW}px the result did not bring the match on screen: ${JSON.stringify(narrow.seen)}`);
  }

  const wide = await land(browser, WIDE, apply);
  proof = wide.proof;
  // Without the competing label the wide leg would pass whatever the
  // directive said, and this probe would be a second narrow leg.
  if (!wide.seen.rail) {
    broken(`no visible rail label reads ${JSON.stringify(RAIL_LABEL)} at ${WIDE}px, so nothing competes with the article here`);
  }
  if (!wide.seen.start.inView || !wide.seen.end.inView) {
    fail('wide-endpoints-in-view', `at ${WIDE}px a rail label decided where an article search result landed: ${JSON.stringify(wide.seen)}`);
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log(`PASS result-landing-visibility: a result click puts ${START_WORD} and ${END_WORD} on screen at ${NARROW}px and at ${WIDE}px`);
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED result-landing-visibility: ${MUTATE}: ${issue}`);
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
