// Behavior lock for what each page is about being the largest text on it. The
// search page is about what the reader is looking for, so the query they type
// is set larger than anything else there and is 28px; the answers under it
// are rows with no line between them. The size is what the browser computed,
// read off every piece of visible text on the page and compared, so a heading
// restored to its old size or a result title enlarged is read as the loss it is
// whatever class it wears.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';

const QUERY_SITE = 'the-query-is-the-largest-text-on-the-search-page';
const RULES_SITE = 'results-have-no-line-between-them';
const SITES = [QUERY_SITE, RULES_SITE];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL largest-text: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN largest-text: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED largest-text: ${message}`); };

// Appends a rule to the product's own stylesheet, past the layer it declares,
// so the injected rule outranks what it stands in for without an importance
// flag: the shape a regression written into the stylesheet itself would have.
const appendStyle = (css) => (page) => {
  let seen = 0;
  return page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${css}\n` });
  }).then(() => () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page'));
};

const MUTATIONS = {
  // A result title set at a heading's size: the answer outshouts the question.
  'enlarge-a-result-title': { target: QUERY_SITE, apply: appendStyle('.y-result__title{font-size:3rem}') },
  // The query put back at the size of body text.
  'shrink-the-query': { target: QUERY_SITE, apply: appendStyle('.y-searchpage__form input{font-size:1rem}') },
  // A hairline under every hit, which is the line the answer was drawn with.
  'rule-between-results': { target: RULES_SITE, apply: appendStyle('.y-result{border-bottom:1px solid #000}') },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`largest-text: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`largest-text: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`largest-text: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Every piece of text a reader can see, with the size the browser set it at:
// an element's own words, or the value or hint of a field. Text clipped to a
// pixel for a listener, and anything not drawn, is not text a reader sees.
const readSizes = (page) => page.evaluate(() => {
  const found = [];
  const seen = (element) => {
    const rect = element.getBoundingClientRect();
    return rect.width > 2 && rect.height > 2 && element.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true });
  };
  for (const element of document.body.querySelectorAll('*')) {
    if (['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE'].includes(element.tagName)) continue;
    const own = [...element.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() !== '');
    const field = element.matches('input, textarea') && (element.value !== '' || element.placeholder !== '');
    if (!own && !field) continue;
    if (!seen(element)) continue;
    found.push({
      size: Number.parseFloat(getComputedStyle(element).fontSize),
      what: `${element.tagName.toLowerCase()}${element.className ? `.${String(element.className).trim().split(/\s+/).join('.')}` : ''}`,
      words: (element.value || element.textContent || '').trim().slice(0, 30),
    });
  }
  return found;
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  const response = await page.goto(`${BASE}/search?q=alpha`, { waitUntil: 'load' });
  if (response?.status() !== 200) broken(`/search?q=alpha answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const query = await page.locator('.y-searchpage__form input').evaluate((input) => Number.parseFloat(getComputedStyle(input).fontSize));
  if (!(query > 0)) broken('the search page draws no query field to measure');
  const sizes = await readSizes(page);
  const others = sizes.filter((text) => text.what !== 'input.' && !text.what.startsWith('input'));
  const larger = others.filter((text) => text.size > query);
  if (query !== 28) {
    fail(QUERY_SITE, `the query is set at ${query}px, want 28px`);
  }
  if (larger.length > 0) {
    const [first] = larger.sort((a, b) => b.size - a.size);
    fail(QUERY_SITE, `${first.what} (${JSON.stringify(first.words)}) is set at ${first.size}px, larger than the 28px query the page is about`);
  }

  const hits = await page.locator('.y-result').count();
  if (hits < 2) broken(`the search for alpha has ${hits} hits, too few for a line between them to be read`);
  const lines = await page.locator('.y-result').evaluateAll((rows) => rows.map((row) => {
    const style = getComputedStyle(row);
    return { top: style.borderTopWidth, bottom: style.borderBottomWidth };
  }));
  const drawn = lines.filter((line) => line.top !== '0px' || line.bottom !== '0px');
  if (drawn.length > 0) {
    fail(RULES_SITE, `${drawn.length} of ${hits} hits are drawn with a line: ${JSON.stringify(drawn[0])}`);
  }

  await context.close();
  console.log(`PASS largest-text: the query is 28px, the largest text on the search page, over ${hits} hits drawn with no line between them`);
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
