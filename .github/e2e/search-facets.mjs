// Behavior lock: the column beside the search results keeps the promises it
// makes. A count is a claim about the page its row leads to, so the probe
// follows the row and counts what arrives; the column is reachable at both
// widths, which at the wide one means reachable with the control that opens it
// taken away; and it is off the sheet when the page is printed.
//
// Markup is not the contract at any of these sites. A count in the DOM that
// leads somewhere else reads to a Go test exactly like a count that leads
// where it says, and a fold held open by a stylesheet can render its links
// while leaving them outside the tab order — which is a column a reader using
// a keyboard cannot reach at all. So each assertion ends at real geometry, a
// real navigation, or the browser's own focus.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=a';
const MUTATE = process.env.MUTATE || '';
const SITES = ['column-reachable', 'count-keeps-its-promise', 'folds-when-narrow', 'off-the-sheet'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN search-facets: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL search-facets: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN search-facets: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED search-facets: ${message}`); };

// rewriteAll proves it applied by counting, so a needle that rots against a
// rewritten template turns this run red rather than quietly mutating nothing.
// Every search document is rewritten, not just the first: following a row is
// a second navigation, and a mutation that stopped at the landing page would
// leave the very page the count is checked against untouched.
const rewriteAll = (needle, replacement, expected, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(`${BASE}/search*`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `${label}: no search document was requested`;
    if (matches < expected * requests) {
      return `${label}: needle matched ${matches} times over ${requests} request(s), want at least ${expected} each`;
    }
    return '';
  };
};

const styleInto = (css, label) => rewriteAll('</head>', `<style>${css}</style></head>`, 1, label);

const MUTATIONS = {
  // The column taken off the page entirely.
  'hide-the-column': {
    target: 'column-reachable',
    apply: styleInto('.y-facets{display:none!important}', 'column hide style'),
  },
  // Rendered, but left where the fold put it: every link present in the
  // markup and none of them reachable, which is the failure a Go test and a
  // markup assertion both call a pass.
  'leave-the-fold-shut-at-every-width': {
    target: 'column-reachable',
    apply: styleInto(
      '.y-searchpage .y-facets::details-content{content-visibility:hidden!important}',
      'fold shut style',
    ),
  },
  // The count says one thing and the row leads somewhere else. This is the
  // whole reason the probe navigates instead of reading the number twice.
  'print-a-count-that-is-not-the-answer': {
    target: 'count-keeps-its-promise',
    apply: rewriteAll('data-facet-count="', 'data-facet-count="9', 1, 'count rewrite'),
  },
  // The narrowed search a row leads to, pointed at the unnarrowed query.
  'send-the-row-to-the-unnarrowed-search': {
    target: 'count-keeps-its-promise',
    apply: rewriteAll('+status%3Adraft', '', 1, 'row href rewrite'),
  },
  // Held open at phone width, where it pushes the results the reader came for
  // off the first screen.
  'force-the-fold-open-when-narrow': {
    target: 'folds-when-narrow',
    apply: styleInto(
      '.y-facets::details-content{content-visibility:visible!important;block-size:auto!important}.y-facets>summary{display:none!important}',
      'fold open style',
    ),
  },
  // A column too wide for the screen, which on a phone is a row of counts the
  // reader has to scroll sideways to read. It is held below the width at which
  // the column moves beside the results, because the fold cuts its body off at
  // its own edge: a body widened there reaches past the window without
  // widening the document, so nothing scrolls to it and the first row lands
  // off screen — a true finding, but about the wide layout rather than about
  // the phone this mutation is named for, and a finding at the wrong site is
  // not the catch this mode owes.
  'let-the-column-run-off-the-phone': {
    target: 'folds-when-narrow',
    apply: styleInto('@media (width < 1280px){.y-facets__body{min-width:520px!important}}', 'wide column style'),
  },
  // Onto the sheet, where it is a list of links nobody can follow.
  'print-the-column-on-paper': {
    target: 'off-the-sheet',
    apply: styleInto('@media print{.y-facets{display:flex!important}}', 'print style'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`search-facets: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`search-facets: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`search-facets: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// onScreen asks whether this element's own box is somewhere the pointer lands
// on it, in ink. checkVisibility covers content-visibility and any opacity
// above the element, which is exactly how a fold left shut hides its contents
// while the markup still carries them.
const onScreen = (locator) => locator.evaluate((element) => {
  element.scrollIntoView({ block: 'center' });
  const rect = element.getBoundingClientRect();
  const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
  return {
    text: element.textContent.replace(/\s+/g, ' ').trim(),
    width: rect.width,
    height: rect.height,
    right: rect.right,
    visible: element.checkVisibility({
      opacityProperty: true,
      visibilityProperty: true,
      contentVisibilityAuto: true,
    }),
    hit: hit === element || element.contains(hit),
  };
});

const shown = (seen) => seen.width > 1 && seen.height > 1 && seen.visible && seen.hit;

// foldSettled waits for a disclosure that was just opened to finish opening.
// The engine wraps a fold's body in ::details-content, which grows from no
// height to the body's height and stops being hidden as that growth starts.
// Until it ends the body is cut off at the fold's edge, so a point in the
// middle of a row inside it lands on whatever is drawn there instead — the
// summary above, the results underneath — and the reading describes the fold
// opening rather than the opened fold a reader is looking at.
//
// The engine offers no animation for that growth through getAnimations() and
// fires no transitionend for it, on the fold or anywhere else, so what is
// watched is the wrapper's own computed height: once the body is no longer
// hidden and that height has held still across two frames, the fold is as open
// as it is going to get. A fold that opens instantly — reduced motion, or no
// transition declared — settles on the first comparison, and one that never
// opens runs out the deadline and falls through to the assertion below, which
// is what has to answer for it.
const foldSettled = (locator) => locator.evaluate((details) => new Promise((resolve) => {
  const deadline = performance.now() + 2000;
  let previous = null;
  let held = 0;
  const read = () => {
    const style = getComputedStyle(details, '::details-content');
    held = style.contentVisibility !== 'hidden' && style.height === previous ? held + 1 : 0;
    previous = style.height;
    if (held >= 2 || performance.now() > deadline) resolve();
    else requestAnimationFrame(read);
  };
  requestAnimationFrame(read);
}));

const resultCount = (page) => page.locator('.y-searchpage [data-live-search-results]')
  .evaluate((region) => Number(region.dataset.resultCount));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  // --- the column is reachable where there is a column ----------------------
  const column = page.locator('[data-search-facets]');
  if (await column.count() !== 1) broken(`the search page carries ${await column.count()} filter columns, want 1`);

  const groups = page.locator('[data-facet-key]');
  if (await groups.count() < 2) {
    broken(`the fixture answer divides along ${await groups.count()} keys, want at least two to check`);
  }
  const rows = page.locator('.y-searchpage a[data-facet-row]');
  const rowCount = await rows.count();
  if (rowCount < 2) broken(`the column offers ${rowCount} followable rows, want at least two`);

  const seenFirst = await onScreen(rows.first());
  if (!shown(seenFirst)) {
    fail('column-reachable', `the first row is not on screen at 1280: ${JSON.stringify(seenFirst)}`);
  }
  // A fold held open by a stylesheet can paint its links and still leave them
  // out of the tab order, which is the one way this column fails a reader
  // using a keyboard while looking perfectly correct in a screenshot.
  const focused = await rows.first().evaluate((link) => {
    link.focus();
    return document.activeElement === link;
  });
  if (!focused) fail('column-reachable', 'a row in the opened column cannot take focus, so no keyboard reaches it');

  // The region has to answer to a name once the summary is hidden, or a
  // reader arriving by ear meets an unnamed group of links.
  const named = await column.locator('nav').evaluate((nav) => nav.getAttribute('aria-label') || '');
  if (!named) fail('column-reachable', 'the column has no accessible name at the width where its summary is hidden');

  // --- a count is a promise about the page it leads to -----------------------
  let checked = 0;
  for (let i = 0; i < rowCount; i += 1) {
    const row = rows.nth(i);
    const claimed = Number(await row.locator('[data-facet-count]').getAttribute('data-facet-count'));
    const active = await row.evaluate((link) => link.hasAttribute('data-facet-active'));
    const href = await row.getAttribute('href');
    if (!Number.isInteger(claimed)) broken(`a row states ${JSON.stringify(claimed)} rather than a count`);

    await page.goto(BASE + href, { waitUntil: 'domcontentloaded' });
    const landed = await resultCount(page);
    if (landed !== claimed) {
      fail('count-keeps-its-promise', `a row said ${claimed} and its own page found ${landed} (${href})`);
    }
    // The query the row wrote is the query in the box, so the reader can see
    // what was added to their words and edit it.
    const inBox = await page.locator('.y-searchpage [data-live-search-input]').inputValue();
    if (!active && !inBox.includes(':')) {
      fail('count-keeps-its-promise', `following a row left the box reading ${JSON.stringify(inBox)}, with no constraint in it`);
    }
    checked += 1;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  }
  if (checked < 2) broken(`only ${checked} rows were followed, so this holds almost nothing`);

  // --- typing keeps the column and the rows answering the same query --------
  //
  // The column lives inside the fragment the live search replaces, which is
  // the whole reason it was put there: counts that stayed behind while the
  // rows moved on would be a set of promises about an answer nobody can see
  // any more. TYPED is a narrower query than the one the page arrived with, so
  // the totals have to move.
  const TYPED = 'les';
  const arrived = await resultCount(page);
  const box = page.locator('.y-searchpage [data-live-search-input]');
  await box.click();
  await box.fill('');
  await box.type(TYPED, { delay: 60 });
  await page.waitForFunction(
    (want) => {
      const region = document.querySelector('.y-searchpage [data-live-search-results]');
      return region?.getAttribute('aria-busy') === 'false' && Number(region.dataset.resultCount) !== want;
    },
    arrived,
  );
  const liveTotal = await resultCount(page);
  if (liveTotal === 0) broken(`typing ${JSON.stringify(TYPED)} left the fixture with no answer to divide`);
  const liveRows = page.locator('.y-searchpage a[data-facet-row]');
  const liveRowCount = await liveRows.count();
  if (liveRowCount === 0) {
    fail('count-keeps-its-promise', `typing left ${liveTotal} hits and no column, so the divisions did not follow the rows`);
  }
  for (let i = 0; i < liveRowCount; i += 1) {
    const claimed = Number(await liveRows.nth(i).locator('[data-facet-count]').getAttribute('data-facet-count'));
    if (claimed > liveTotal) {
      fail('count-keeps-its-promise', `after typing, a row claims ${claimed} of ${liveTotal} hits, so the column answers an older query`);
    }
  }

  // --- narrow: the column is a fold above the results ------------------------
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  if (await column.evaluate((details) => details.open)) {
    fail('folds-when-narrow', 'the column arrives open on a phone, pushing the results the reader came for down the page');
  }
  const summary = column.locator('summary');
  const seenSummary = await onScreen(summary);
  if (!shown(seenSummary)) {
    fail('folds-when-narrow', `the control that opens the column is not on screen at 390: ${JSON.stringify(seenSummary)}`);
  }
  await summary.click();
  if (await column.evaluate((details) => !details.open)) broken('the column did not open when its summary was clicked');
  await foldSettled(column);
  const openedRow = await onScreen(page.locator('.y-searchpage a[data-facet-row]').first());
  if (!shown(openedRow)) fail('folds-when-narrow', `an opened row is not on screen at 390: ${JSON.stringify(openedRow)}`);
  // The claim is about the column, not about the page: this search page
  // already scrolls sideways by a few pixels at this width for reasons that
  // predate the column and live in the shell, so a bare scrollWidth check here
  // would fail for something else and go on failing whatever the column did.
  // What is asked instead is that no part of the column reaches past the
  // viewport, which is the column's own share of that question.
  const overhang = await page.evaluate(() => {
    const client = document.documentElement.clientWidth;
    return [...document.querySelectorAll('.y-facets, .y-facets *')]
      .map((element) => Math.round(element.getBoundingClientRect().right))
      .filter((right) => right > client + 0.5);
  });
  if (overhang.length > 0) {
    fail('folds-when-narrow', `the opened column reaches past the phone's ${await page.evaluate(() => document.documentElement.clientWidth)}px to ${overhang.join(', ')}`);
  }

  // --- paper ----------------------------------------------------------------
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.emulateMedia({ media: 'print' });
  const onPaper = await column.evaluate((element) => element.checkVisibility({
    opacityProperty: true,
    visibilityProperty: true,
    contentVisibilityAuto: true,
  }));
  if (onPaper) fail('off-the-sheet', 'the filter column prints, and a sheet of paper takes no clicks');
  await page.emulateMedia({ media: 'screen' });

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log(`PASS search-facets: ${checked} rows led to the answer they counted, the column is reachable at 1280 and folded at 390, and none of it prints`);
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED search-facets: ${MUTATE}: ${issue}`);
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
