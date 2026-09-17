// Behavior lock for a fold the author closed in a note's prose: it withholds
// its body on screen, and hands it over on paper.
//
// A closed disclosure is an offer the reader takes by clicking. Paper takes no
// click, so a fold that printed closed would put its one-line summary on the
// sheet and drop the explanation under it, with nobody for the reader to ask.
// Both halves are asserted, because a rule that simply revealed every fold
// everywhere would satisfy the paper half while destroying the point of a fold
// on screen.
//
// The assertions read rendered output rather than the element's own declared
// display: the engine wraps a disclosure's body in ::details-content and skips
// it while the element is closed, and skipped content still reports a box.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note carrying a closed prose fold), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'fold-closed-on-screen',
  'fold-open-on-paper',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN print-fold-open: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL print-fold-open: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN print-fold-open: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED print-fold-open: ${message}`); };

const weakenStylesheet = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'app.css was never requested, so the override reached no page');
};

const MUTATIONS = {
  // The regression this lock exists to catch: the print rule gone, so the
  // engine's own skip stands and the sheet loses the folded half.
  'hide-fold-on-paper': {
    target: 'fold-open-on-paper',
    apply: weakenStylesheet('@media print{.y-prose details:not([open])::details-content{content-visibility:hidden !important}}'),
  },
  // The cheap way to satisfy the paper half — reveal every fold on every
  // medium — which would leave no fold on screen at all.
  'open-fold-on-screen': {
    target: 'fold-closed-on-screen',
    apply: weakenStylesheet('.y-prose details::details-content{content-visibility:visible !important}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`print-fold-open: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`print-fold-open: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`print-fold-open: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// laidOut answers whether the element's own box carries ink on the current
// medium. A box alone is not enough: content the engine has skipped keeps its
// measured rectangle, and checkVisibility is what reports the skip.
const laidOut = (locator) => locator.first().evaluate((element) => {
  element.scrollIntoView({ block: 'nearest' });
  const rect = element.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return false;
  const style = getComputedStyle(element);
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = style.color;
  ctx.fillRect(0, 0, 1, 1);
  const inkAlpha = ctx.getImageData(0, 0, 1, 1).data[3];
  if (inkAlpha === 0) return false;
  return element.checkVisibility({
    opacityProperty: true,
    visibilityProperty: true,
    contentVisibilityAuto: true,
  });
});

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

  // The lock is about the fold, not about any one callout vocabulary word, so
  // it asks for a disclosure in the note's prose that nobody has opened. A
  // fixture that stopped carrying one would otherwise let both halves below
  // pass over nothing.
  const fold = page.locator('.y-prose details.callout:not([open])');
  if ((await fold.count()) === 0) {
    broken(`${PAGE} carries no closed fold in its prose, so neither medium can be judged`);
  }
  const summary = fold.first().locator('> summary.callout-title');
  const folded = fold.first().locator('> .callout-body');
  if ((await folded.count()) === 0) {
    broken('the closed fold has no body under its summary, so there is nothing to withhold or hand over');
  }

  await page.emulateMedia({ media: 'screen' });
  if (!(await laidOut(summary))) {
    broken('the closed fold does not even show its summary on screen, so the page is not the one this lock reads');
  }
  if (await laidOut(folded)) {
    fail('fold-closed-on-screen', 'a fold nobody opened already shows its body on screen, so the reader is told before being asked');
  }

  await page.emulateMedia({ media: 'print' });
  if (!(await laidOut(folded))) {
    fail('fold-open-on-paper', 'a closed fold prints its summary alone, so the sheet carries the question and loses the answer');
  }

  await context.close();
  console.log('PASS print-fold-open: a closed prose fold withholds its body on screen and opens on paper');
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
