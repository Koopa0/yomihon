// Behavior lock for the note head's facts (#581): a phone-width reading with
// them stacked inside the disclosure fits — both the fold that holds them and
// the viewport the fold sits in — and the disclosure is the browser's own, so
// a plain click opens it with no script running at all and the dt/dd pairs are
// the content that arrives.
//
// Neither half is reachable from a Go test. Fitting is a number only a
// laid-out page has, and "no script running" is a browser context Go never
// drives at all.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note whose head carries every fact — type,
// status, updated, language, raw path), and MUTATE. MUTATE=list prints every
// watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/go/L01.md';
const MUTATE = process.env.MUTATE || '';
const WIDTH = 390;
const SITES = ['the-head-fits-the-phone-viewport', 'the-closed-fold-opens-with-no-script-running'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN note-head-facts: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL note-head-facts: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN note-head-facts: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED note-head-facts: ${message}`);
};

// Rewrites one served page. needle must be found, so a self-test that quietly
// died against rewritten markup turns red here rather than passing as a
// regression that walked through.
const rewritePage = (match, needle, replacement) => async (page) => {
  let hits = 0;
  await page.route(match, async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    if (!body.includes(needle)) {
      await route.fulfill({ response, body });
      return;
    }
    hits += 1;
    await route.fulfill({ response, body: body.replace(needle, replacement) });
  });
  return async () => (hits === 0 ? `nothing served carried ${JSON.stringify(needle)}` : '');
};

// Appends a rule outside the product layer so it outranks what it stands in
// for, without touching the served file, and reads back what the browser
// resolved it to — a rule that was served but outranked reports itself rather
// than passing as a regression nobody noticed.
const appendRule = (rule, read, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
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
  // Widens the fact list past the phone it is being read on, the same shape
  // course-line's phone-fit mutation uses.
  'stretch-the-facts-past-the-phone': {
    target: 'the-head-fits-the-phone-viewport',
    apply: appendRule('.y-notefacts{min-width:900px}', () => getComputedStyle(document.querySelector('.y-notefacts')).minWidth, '900px'),
  },
  // Swaps the native <summary> for a <div> that looks the same but carries
  // none of its behavior: nothing native is left to open on a click with no
  // script running, which is exactly the case this probe exists to catch.
  'break-the-native-disclosure': {
    target: 'the-closed-fold-opens-with-no-script-running',
    apply: rewritePage(
      (url) => url.pathname === PAGE,
      '<summary class="y-metarow__summary">',
      '<div class="y-metarow__summary" data-broken-disclosure>',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`note-head-facts: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`note-head-facts: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`note-head-facts: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: the phone reading, script running, measured for overflow in the
  // two places it can land. The fold that holds the facts cuts its body off at
  // its own edge, so a fact list wider than the fold is truncated where a
  // reader can see it while the document stays exactly as wide as the phone;
  // that is asked of the list against the fold's box. Everything else that
  // reaches past the phone still widens the document, and a box pinned to the
  // viewport cannot be what widened it, so its width is the gauge the
  // document's own scrollWidth is judged against.
  {
    const context = await browser.newContext({ viewport: { width: WIDTH, height: 844 } });
    const page = await context.newPage();
    const proof = MUTATE === 'stretch-the-facts-past-the-phone' ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => document.fonts.ready);
    // The disclosure starts closed, and closed content is not laid out at
    // all — a width rule on it would move nothing on the page and prove
    // nothing about the reading a phone actually has to fit. Opening it first
    // is what a reader who taps the fold does, and it is the only state in
    // which the facts row can overflow anything.
    await page.locator('details.y-metarow summary').first().click();
    // The fold opens by transitioning content-visibility (hidden to visible)
    // alongside height, with transition-behavior: allow-discrete — the flip to
    // visible lands at the start of that transition, but only once the browser
    // has actually started that transition, which needs a style-and-paint
    // cycle since the click's attribute change and can take more than one
    // frame under load. Reading computed style in the same script turn as the
    // click still sees the closed value, so this waits for the fact list to
    // report visible rather than assuming a fixed number of frames is enough;
    // a fold that never opens still falls through to the check below.
    await page
      .waitForFunction(() => document.querySelector('.y-metarow .y-notefacts dt')?.checkVisibility() ?? false, { timeout: 2000 })
      .catch(() => {});
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`stretch-the-facts-past-the-phone: ${issue}`);
    }
    const { docWidth, viewport, factsVisible, factsCount, factsWidth, foldWidth, clipMargin } = await page.evaluate(() => {
      const gauge = document.createElement('div');
      gauge.style.cssText = 'position:fixed;inset:0;pointer-events:none';
      document.body.append(gauge);
      const viewport = gauge.getBoundingClientRect().width;
      gauge.remove();
      // The same facts are drawn twice: once inside this fold, and once beside
      // the title for a window wide enough to hold them. The second copy is
      // not laid out at a phone's width, so it measures nothing and would
      // answer nothing; the copy inside the fold is the one a phone reads.
      const facts = document.querySelectorAll('.y-metarow .y-notefacts');
      const fold = facts.length === 1 ? facts[0].closest('details') : null;
      const content = fold && getComputedStyle(fold, '::details-content');
      return {
        docWidth: document.documentElement.scrollWidth,
        viewport,
        factsVisible: document.querySelector('.y-metarow .y-notefacts dt')?.checkVisibility() ?? false,
        factsCount: facts.length,
        // What the list needs, against what the fold gives it. The fold's body
        // is cut off at its own edge, so a list wider than that box loses the
        // end of every fact without widening anything the document can be
        // measured by.
        factsWidth: facts.length === 1 ? facts[0].scrollWidth : 0,
        foldWidth: content ? parseFloat(content.width) : 0,
        // Content is still drawn this far past the cut, so it is the margin
        // within which nothing is actually lost.
        clipMargin: content ? parseFloat(content.overflowClipMargin) || 0 : 0,
      };
    });
    if (factsCount !== 1) broken(`${PAGE} draws ${factsCount} fact lists inside the head's fold, want exactly 1 to measure`);
    if (!factsVisible) broken(`${PAGE}'s facts are not visible after opening the fold, so there is nothing here to measure`);
    if (!(factsWidth > 0) || !(foldWidth > 0)) {
      broken(`the facts measure ${factsWidth}px inside a fold measuring ${foldWidth}px, so neither number is a width anything can be judged against`);
    }
    if (factsWidth > foldWidth + clipMargin) {
      fail(
        'the-head-fits-the-phone-viewport',
        `at ${viewport}px the note's facts need ${factsWidth}px inside a fold ${foldWidth}px wide, so the end of every fact is cut off where the fold stops`,
      );
    }
    // The fold cuts off only what is inside it. Anything else on the page that
    // reaches past the phone still widens the document, and that is what this
    // second reading answers for.
    if (docWidth > viewport) {
      fail(
        'the-head-fits-the-phone-viewport',
        `at ${viewport}px the page lays out ${docWidth}px wide with the note's facts open in the head, so the reader has to scroll sideways`,
      );
    }
    await context.close();
  }

  // Case 2: no script at all, the disclosure closed by default, opened by one
  // plain click, and the facts it reveals read as dt/dd pairs.
  {
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: WIDTH, height: 844 } });
    const page = await context.newPage();
    const proof = MUTATE === 'break-the-native-disclosure' ? await MUTATIONS[MUTATE].apply(page) : null;
    // 'load', not 'domcontentloaded': at this width, and with no script to turn
    // the left rail into an overlay drawer, the whole navigation sits in flow
    // above the article — a real reader's no-JS page — so the summary this
    // probe clicks is well past one screen down and needs the stylesheet
    // settled before its position is trustworthy.
    await page.goto(BASE + PAGE, { waitUntil: 'load' });

    const details = page.locator('details.y-metarow');
    if ((await details.count()) !== 1) broken(`${PAGE} draws ${await details.count()} details.y-metarow, want exactly 1`);
    const before = await details.evaluate((el) => ({ open: el.hasAttribute('open'), dtVisible: el.querySelector('.y-notefacts dt')?.checkVisibility() ?? false }));
    if (before.open || before.dtVisible) {
      broken('the disclosure already carries open, or its facts already render visible, so a click opening it proves nothing about the click');
    }

    // A locator's own click scrolls its target into view first, which a raw
    // page.mouse.click at a stale boundingBox does not — the summary this
    // probe means to press sits off the first screen here, not on it. A short
    // timeout turns a summary a mutation left unclickable into the exact
    // finding below rather than Playwright's own 30s wait-and-retry error,
    // which this probe's own MUTATE contract has no way to read as a catch.
    const summary = page.locator('details.y-metarow summary, details.y-metarow > div.y-metarow__summary');
    if ((await summary.count()) !== 1) broken('the disclosure carries no summary line to click');
    let clickFailed = false;
    try {
      await summary.first().click({ timeout: 3000 });
    } catch {
      clickFailed = true;
    }
    // Same fold, same content-visibility transition as case 1 above — the
    // flip to visible still needs a style-and-paint cycle after the click's
    // attribute change, script or no script running the click itself, so
    // this waits for the fact list rather than a fixed number of frames. A
    // disclosure the mutation broke never opens, so the wait times out and
    // the check below reports it, same as before.
    if (!clickFailed) {
      await page
        .waitForFunction(() => document.querySelector('details.y-metarow .y-notefacts dt')?.checkVisibility() ?? false, { timeout: 2000 })
        .catch(() => {});
    }

    // Whether the mutation reached the page is asked before either of its two
    // outcomes is: a click that failed because the mutation was never served
    // is not-applied, not caught, and the ordinary assertion below still has
    // to answer for a click that succeeded regardless.
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`break-the-native-disclosure: ${issue}`);
    }
    if (clickFailed) {
      fail(
        'the-closed-fold-opens-with-no-script-running',
        'the disclosure carries a summary line, but nothing on the page lets a plain click reach it with no script running',
      );
    }

    const after = await details.evaluate((el) => ({ open: el.hasAttribute('open'), dtCount: el.querySelectorAll('.y-notefacts dt').length, dtVisible: el.querySelector('.y-notefacts dt')?.checkVisibility() ?? false }));
    if (!after.open || !after.dtVisible || after.dtCount === 0) {
      fail(
        'the-closed-fold-opens-with-no-script-running',
        `after the click, details.open is ${after.open} and its facts (${after.dtCount} <dt>) are visible = ${after.dtVisible}, so opening the fold with no script running did not reach the note's facts`,
      );
    }
    await context.close();
  }

  console.log(`PASS note-head-facts: the phone reading of ${PAGE} fits ${WIDTH}px, and its closed fold opens on a plain click with no script running`);
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
