// Behavior lock: the search results list stays inside the phone viewport even
// when a result's meta line or snippet carries a run with no space in it — a
// vault path, or a URL surfacing in the indexed text. Neither element wraps
// such a run without overflow-wrap, so its own width pushes the row past the
// results column and the reader has to scroll the whole page sideways to read
// anything in it.
//
// Env: YOMIHON_BASE, PAGE_PATH (a search results page whose fixture carries an
// unbreakable path or URL — the browser fixture vault's /search?q=a does), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=a';
const MUTATE = process.env.MUTATE || '';
const WIDTHS = [390, 375];
const SITES = ['results-list-fits-the-phone-viewport'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN search-overflow: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL search-overflow: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN search-overflow: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED search-overflow: ${message}`);
};

// Appending to the product's own stylesheet lands outside the layer it
// declares, so the appended rule outranks the one it stands in for without an
// importance flag. The proof reads the property back from a live element
// instead of trusting that the stylesheet was merely requested: a selector
// that stopped matching would still let the route fire and the rule sit
// unused, which is the shape of a kill-test whose needle matches nothing.
const overrideProperty = (selector, property, rule, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await page.evaluate(
      ({ selector, property }) => {
        const el = document.querySelector(selector);
        return el ? getComputedStyle(el).getPropertyValue(property) : null;
      },
      { selector, property },
    );
    if (got === null) return `no ${selector} element is on the page to read ${property} from`;
    if (got !== wanted) return `${selector} resolves ${property} to ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

const MUTATIONS = {
  // The snippet is the one element the browser fixture vault's own content
  // proves this lock over: the indexed report carries an unbroken URL long
  // enough on its own to push the whole results column past a phone's width.
  'let-the-snippet-refuse-to-wrap': {
    target: 'results-list-fits-the-phone-viewport',
    before: overrideProperty('.y-result__snippet', 'overflow-wrap', '.y-result__snippet{overflow-wrap:normal}', 'normal'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`search-overflow: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`search-overflow: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`search-overflow: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const width of WIDTHS) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage();
    const proof = MUTATE ? await MUTATIONS[MUTATE].before(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(200);
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`${MUTATE} at ${width}px: ${issue}`);
    }

    const measured = await page.evaluate(() => {
      const results = document.querySelector('ol.y-results');
      return {
        docScrollWidth: document.documentElement.scrollWidth,
        docClientWidth: document.documentElement.clientWidth,
        resultsScrollWidth: results ? results.scrollWidth : null,
        resultsClientWidth: results ? results.clientWidth : null,
      };
    });
    if (measured.resultsScrollWidth === null) {
      broken(`the page at ${width}px carries no ol.y-results to measure`);
    }
    if (measured.docScrollWidth > measured.docClientWidth) {
      fail(
        'results-list-fits-the-phone-viewport',
        `at ${width}px the document is ${measured.docScrollWidth}px wide against a ${measured.docClientWidth}px viewport`,
      );
    }
    if (measured.resultsScrollWidth > measured.resultsClientWidth) {
      fail(
        'results-list-fits-the-phone-viewport',
        `at ${width}px ol.y-results scrollWidth=${measured.resultsScrollWidth} > clientWidth=${measured.resultsClientWidth}`,
      );
    }
    await context.close();
  }

  console.log(`PASS search-overflow: the results list fits the viewport at ${WIDTHS.join('px and ')}px`);
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
