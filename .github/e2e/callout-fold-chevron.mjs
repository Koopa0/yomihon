// Behavior lock for the fold indicator on a foldable callout (`> [!tip]-`).
//
// A foldable callout is a native disclosure whose summary has no marker, so
// without a drawn indicator a closed one reads as a title with no body and
// nothing on screen says it can be opened. The summary therefore carries a
// chevron in its ::after, and the chevron turns when the fold is opened.
// Two halves are asserted: the chevron exists on the closed fold with a box
// and a shape, and its transform differs between closed and open.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note carrying a closed foldable callout), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'chevron-drawn',
  'chevron-turns',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN callout-fold-chevron: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL callout-fold-chevron: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN callout-fold-chevron: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED callout-fold-chevron: ${message}`); };

const weakenStylesheet = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'app.css was never requested, so the override reached no page');
};

const MUTATIONS = {
  // The indicator removed outright: the closed fold is a title and nothing else.
  'no-chevron': {
    target: 'chevron-drawn',
    apply: weakenStylesheet('.y-prose details.callout > .callout-title::after{content:none !important}'),
  },
  // The glyph drawn but never turned, so closed and open look the same.
  'chevron-never-turns': {
    target: 'chevron-turns',
    apply: weakenStylesheet('.y-prose details.callout > .callout-title::after{transform:none !important}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`callout-fold-chevron: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`callout-fold-chevron: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`callout-fold-chevron: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 900, height: 900 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const fold = page.locator('.y-prose details.callout:not([open])').first();
  if ((await fold.count()) === 0) {
    broken(`${PAGE} carries no closed foldable callout, so no indicator can be judged`);
  }
  const summary = fold.locator('> summary.callout-title');
  await summary.scrollIntoViewIfNeeded();

  const readChevron = () => summary.evaluate((element) => {
    const style = getComputedStyle(element, '::after');
    return {
      content: style.content,
      height: parseFloat(style.height) || 0,
      mask: style.maskImage || style.webkitMaskImage || 'none',
      transform: style.transform,
      width: parseFloat(style.width) || 0,
    };
  });

  const closed = await readChevron();
  const drawn = closed.content !== 'none' && closed.content !== 'normal'
    && closed.width > 0 && closed.height > 0 && closed.mask !== 'none';
  if (!drawn) {
    fail('chevron-drawn', `a closed foldable callout draws no indicator on its summary (${JSON.stringify(closed)})`);
  }

  await summary.click();
  if (!(await fold.evaluate((element) => element.open))) {
    broken('clicking the summary did not open the fold, so the open state cannot be read');
  }
  // The chevron turns over a transition, so the open reading is taken once it
  // has settled away from the closed one, or not at all within the wait.
  let open = await readChevron();
  for (let i = 0; i < 40 && open.transform === closed.transform; i += 1) {
    await page.waitForTimeout(50);
    open = await readChevron();
  }
  if (open.transform === closed.transform) {
    fail('chevron-turns', `the indicator looks the same open and closed (transform ${closed.transform})`);
  }

  await context.close();
  console.log('PASS callout-fold-chevron: a closed foldable callout draws a chevron that turns when opened');
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
