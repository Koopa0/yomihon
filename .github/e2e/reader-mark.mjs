// Behavior lock for the place a reader keeps and comes back to.
//
// The whole of this feature is one round trip the reader can see: press the
// control on a note, find the desk offering that note back, follow it, and
// arrive where the window was rather than at the top. Each half is written in
// a different place — the position in a client module, the row in a template
// built from a file the server read — so only a live browser sees the two
// halves agree.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note long enough to scroll and carrying
// headings to anchor against), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const OTHER = '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';

// How far down the note the reader is taken before keeping the place. Far
// enough that an anchor sits above the top of the window and that landing at
// the top of the document would be an obvious miss.
const SCROLL_TO = 900;
// The probe drives two widths, and the control has a face at each. At the wide
// one the reading page draws its right rail: the rail is its own scroll
// container, so reaching the control does not move the article out from under
// the position being kept. Below the rail's width there is no rail, and the
// face is inside the header's folded panel — the header stays put while the
// note scrolls, which is the same property said a second way.
const VIEWPORT = { width: 1600, height: 900 };
const NARROW = { width: 390, height: 844 };
const LANDING_SLACK = 4;
// The button the folded panel hangs from, and the panel itself: how a reader at
// the narrow width reaches anything that is in it.
const FOLD_BUTTON = '[popovertarget="header-fold"]';
const FOLD_PANEL = '.y-headerfold';

const SITES = [
  'kept-place-is-offered-back',
  'following-it-lands-where-the-window-was',
  'a-missing-anchor-lands-at-the-top',
  'a-changed-note-says-so',
  'keeping-another-replaces-it',
  'a-narrow-reader-can-keep-a-place',
  'a-narrow-reader-is-offered-the-place-back',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN reader-mark: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL reader-mark: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN reader-mark: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED reader-mark: ${message}`); };

// rewriteModule injects a regression into the client module itself, which is
// where two of these behaviours are decided. The needle has to match exactly
// once in every copy served: a mutation aimed at a line that has been rewritten
// proves nothing, and saying so is the difference between a probe that died and
// one that passed.
//
// The count is kept per load rather than added up, because a run that visits
// the note, the desk and the note again is served the module three times. A
// single total would have to be three, and would then be a number that changes
// whenever a site navigates once more — which is a proof that stops proving
// anything the day someone adds a step.
const rewriteModule = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/mark.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the module was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

// rewriteStylesheet injects a regression into the stylesheet, which is where
// the two widths are decided. It counts per load for the same reason the
// module rewrite does: a run that visits the note, the desk and the note again
// is served the sheet more than once.
const rewriteStylesheet = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the stylesheet was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

// rewriteDesk injects a regression into the page the row is drawn on, for the
// two behaviours the server decides from the file it read. The row itself is
// server-rendered, so there is no module to rewrite for these.
const rewriteDesk = (pattern, replacement, label) => async (page) => {
  let matches = 0;
  await page.route(`${BASE}/`, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(pattern).length - 1;
    await route.fulfill({ response, body: original.split(pattern).join(replacement) });
  });
  return () => (matches >= 1 ? '' : `${label} needle matched nothing, want at least 1`);
};

// dropSecondPost lets the first mark through and refuses the one that should
// replace it, which is the shape of a replacement that silently does not
// happen.
const dropSecondPost = () => async (page) => {
  let seen = 0;
  await page.route('**/marks', async (route) => {
    seen += 1;
    if (seen >= 2) {
      await route.fulfill({ status: 204, body: '' });
      return;
    }
    await route.continue();
  });
  return () => (seen >= 2 ? '' : `only ${seen} mark(s) were posted, want at least 2`);
};

const MUTATIONS = {
  'never-post-the-mark': {
    target: 'kept-place-is-offered-back',
    apply: rewriteModule(
      "    const response = await fetch(control.dataset.markEndpoint ?? '', {",
      "    const response = await fetch('/marks-nowhere', {",
      'the post that keeps the place',
    ),
  },
  'land-without-the-offset': {
    target: 'following-it-lands-where-the-window-was',
    apply: rewriteModule(
      'window.scrollTo(0, (anchor ? documentTop(anchor) : 0) + offset);',
      'window.scrollTo(0, anchor ? documentTop(anchor) : 0);',
      'the offset added on landing',
    ),
  },
  'land-past-a-missing-anchor': {
    target: 'a-missing-anchor-lands-at-the-top',
    apply: rewriteModule(
      '    if (!anchor) return;',
      '    if (!anchor) { /* land anyway */ }',
      'the guard on an anchor the note no longer carries',
    ),
  },
  'say-nothing-about-a-changed-note': {
    target: 'a-changed-note-says-so',
    apply: rewriteDesk('y-continue__notice', 'y-continue__silent', 'the row notice class'),
  },
  'keep-the-previous-mark': {
    target: 'keeping-another-replaces-it',
    apply: dropSecondPost(),
  },
  'keep-the-panel-face-hidden': {
    target: 'a-narrow-reader-can-keep-a-place',
    apply: rewriteStylesheet(
      '[data-js] .y-headerfold:popover-open .y-headermark{display:block}',
      '[data-js] .y-headerfold:popover-open .y-headermark{display:none}',
      'the rule that draws the control inside the open panel',
    ),
  },
  // What the narrow face exists to prevent, injected: the place kept is no
  // longer the one the window was at. A control the reader has to travel to
  // records the travelling; this is that failure with the travel left out.
  'keep-the-top-instead-of-the-window': {
    target: 'a-narrow-reader-can-keep-a-place',
    apply: rewriteModule(
      '  const top = Math.max(0, Math.round(window.scrollY));',
      '  const top = 0;',
      'the position the control reads at the press',
    ),
  },
  // The needle is the rule's opening rather than the whole of it, because what
  // is injected wins from anywhere inside the block and the rest of the rule is
  // then free to be reordered without this going stale. An opening that stops
  // matching is reported as a mutation that never applied, not as a pass.
  'hide-the-way-back-when-narrow': {
    target: 'a-narrow-reader-is-offered-the-place-back',
    apply: rewriteStylesheet(
      '.y-continue{',
      '.y-continue{display:none;',
      'the way back drawn on the desk',
    ),
  },
  'narrow-landing-drops-the-offset': {
    target: 'a-narrow-reader-is-offered-the-place-back',
    apply: rewriteModule(
      'window.scrollTo(0, (anchor ? documentTop(anchor) : 0) + offset);',
      'window.scrollTo(0, anchor ? documentTop(anchor) : 0);',
      'the offset added on landing',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`reader-mark: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`reader-mark: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reader-mark: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const applyMutation = async (page, site) => {
  if (!MUTATE || MUTATIONS[MUTATE].target !== site) return () => '';
  return MUTATIONS[MUTATE].apply(page);
};
const checkProof = (proof) => {
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

// keepThePlace scrolls the note and presses the control, waiting for the row
// to have something to offer rather than for a fixed delay. It returns where
// the window was on either side of opening the panel, which is nothing at the
// wide width and the whole question at the narrow one.
const keepThePlace = async (page, path, { tamperIdentity = false, fold = null } = {}) => {
  await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
  if (await page.locator('[data-mark-control]').count() === 0) {
    broken(`${path} carries no mark control`);
  }
  await page.evaluate((y) => window.scrollTo(0, y), SCROLL_TO);
  const reached = {};
  if (fold) {
    // Below the rail's width the face the reader can press is inside the
    // header's folded panel, and a closed panel draws nothing it holds. It is
    // opened by its own button, the way a reader opens it, and the window's
    // position is read on both sides of that press: a control that moved the
    // page on the way to being reached would keep where it took the reader.
    const button = page.locator(FOLD_BUTTON);
    if (await button.count() !== 1) {
      broken(`${path} carries ${await button.count()} buttons for the folded panel, want exactly 1`);
    }
    reached.before = await page.evaluate(() => Math.round(window.scrollY));
    await button.click();
    await page.locator(FOLD_PANEL).waitFor({ state: 'visible', timeout: 4000 });
    reached.after = await page.evaluate(() => Math.round(window.scrollY));
  }
  // Asking for the visible control rather than the first in document order is
  // what makes the face a measurement instead of an assumption: a face hidden
  // by its own rule reads here as zero rather than passing on a node nobody
  // can press. The note carries two, and exactly one of them is ever drawn.
  const control = page.locator('[data-mark-control]:visible');
  const drawn = await control.count();
  if (drawn !== 1) {
    // An opened panel with no control in it is the regression the narrow site
    // exists for, so it is that site firing rather than the probe giving up:
    // the two are different verdicts and only one of them is about the page.
    if (fold) {
      fail(fold.site, `${path} shows ${drawn} mark controls inside the opened panel, want exactly 1`);
    }
    broken(`${path} shows ${drawn} mark controls at this width, want exactly 1`);
  }
  if (tamperIdentity) {
    // The stored identity is made to disagree with the note's own, which is
    // what an edit between keeping the place and coming back produces. No file
    // is touched: the disagreement is the whole of what the row reads.
    await control.evaluate((node) => {
      node.dataset.markIdentity = 'f'.repeat(64);
    });
  }
  const posted = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/marks',
    { timeout: 4000 },
  ).catch(() => null);
  await control.locator('[data-mark-button]').click();
  await posted;
  // The confirmation is the page's own word that the round trip finished, so
  // the desk is not asked before the file exists. The element itself is in
  // every rendering of the control, so waiting for it to exist waits for
  // nothing; what arrives only once the client is done is the text in it.
  await control.locator('[data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
  return reached;
};

const deskRow = async (page) => {
  await page.goto(`${BASE}/`, { waitUntil: 'domcontentloaded' });
  const row = page.locator('[data-home-continue]');
  return { row, present: await row.count() > 0 };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // --- The kept place comes back on the desk ---------------------------
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'kept-place-is-offered-back');
    await keepThePlace(page, PAGE);
    checkProof(proof);
    // The control's own word for this press is read before anything about the
    // desk, and against the word the page itself carries for a kept place: a
    // place another reading session already left behind sits on the desk
    // either way, so only what the control just said tells this press apart
    // from one that reached nowhere.
    const control = page.locator('[data-mark-control]:visible');
    const said = await control.locator('[data-mark-said]').textContent();
    const savedWord = await control.getAttribute('data-mark-saved');
    if (said !== savedWord) {
      fail(
        'kept-place-is-offered-back',
        `the control said ${JSON.stringify(said)} after being pressed, want the word for a kept place `
        + `(${JSON.stringify(savedWord)})`,
      );
    }
    const { row, present } = await deskRow(page);
    if (!present) {
      fail('kept-place-is-offered-back', 'the desk offers no way back after a place was kept');
    }
    const href = await row.locator('[data-continue-link]').first().getAttribute('href');
    if (!href || !href.startsWith('/notes/')) {
      fail('kept-place-is-offered-back', `the row leads to ${href}, want the note that was marked`);
    }
    await context.close();
  }

  // --- Following it lands where the window was -------------------------
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'following-it-lands-where-the-window-was');
    await keepThePlace(page, PAGE);
    const { row, present } = await deskRow(page);
    if (!present) broken('no row to follow, so landing proves nothing');
    await row.locator('[data-continue-link]').first().click();
    await page.waitForLoadState('domcontentloaded');
    checkProof(proof);
    // Two frames, the same wait the module itself takes: a web font or a
    // diagram settling moves the column after the first write.
    await page.evaluate(() => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 60)));
    }));
    const landed = await page.evaluate(() => Math.round(window.scrollY));
    if (Math.abs(landed - SCROLL_TO) > LANDING_SLACK) {
      fail(
        'following-it-lands-where-the-window-was',
        `following the row landed at ${landed}, want ${SCROLL_TO} give or take ${LANDING_SLACK}`,
      );
    }
    const address = new URL(page.url());
    if (address.searchParams.has('at')) {
      fail(
        'following-it-lands-where-the-window-was',
        `the address still carries at=${address.searchParams.get('at')} after landing`,
      );
    }
    await context.close();
  }

  // --- An address naming an anchor the note lost lands at the top -------
  //
  // A mark is an anchor and a distance below it, so an anchor the note no
  // longer carries leaves the distance measuring from nothing. The address is
  // driven directly here rather than through a kept place, because what is
  // being asked is what the page does with an address, and an edit that
  // removes a heading produces exactly this one.
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'a-missing-anchor-lands-at-the-top');
    const missing = 'an-anchor-this-note-does-not-carry';
    await page.goto(`${BASE}${PAGE}?at=${SCROLL_TO}#${missing}`, { waitUntil: 'domcontentloaded' });
    if (await page.locator(`#${missing}`).count() !== 0) {
      broken(`${PAGE} carries an id named ${missing}, so this says nothing about a missing one`);
    }
    await page.evaluate(() => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 60)));
    }));
    const landed = await page.evaluate(() => Math.round(window.scrollY));
    checkProof(proof);
    if (landed !== 0) {
      fail(
        'a-missing-anchor-lands-at-the-top',
        `an address naming an anchor the note no longer carries landed at ${landed}, want the top of the`
        + ' document, which is where a browser running none of this leaves the same address',
      );
    }
    await context.close();
  }

  // --- A note that changed says so -------------------------------------
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'a-changed-note-says-so');
    await keepThePlace(page, PAGE, { tamperIdentity: true });
    const { row, present } = await deskRow(page);
    checkProof(proof);
    if (!present) broken('no row, so what it says about a changed note proves nothing');
    if (await row.locator('.y-continue__notice').count() === 0) {
      fail(
        'a-changed-note-says-so',
        'the note changed since the place was kept and the row says nothing about it',
      );
    }
    const link = await row.locator('[data-continue-link]').count();
    if (link === 0) {
      fail('a-changed-note-says-so', 'a changed note withheld the way back, which is the reader’s own pointer');
    }
    await context.close();
  }

  // --- Keeping another place replaces the first ------------------------
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'keeping-another-replaces-it');
    await keepThePlace(page, PAGE);
    await keepThePlace(page, OTHER);
    checkProof(proof);
    const { row, present } = await deskRow(page);
    if (!present) broken('no row after two places were kept');
    const hrefs = await row.locator('[data-continue-link]').evaluateAll(
      (nodes) => nodes.map((node) => node.getAttribute('href')),
    );
    if (hrefs.length !== 1) {
      fail('keeping-another-replaces-it', `the desk offers ${hrefs.length} ways back, want exactly 1`);
    }
    if (!decodeURIComponent(hrefs[0]).startsWith(decodeURIComponent(OTHER))) {
      fail(
        'keeping-another-replaces-it',
        `after keeping a place in ${OTHER} the desk still leads to ${hrefs[0]}`,
      );
    }
    await context.close();
  }

  // --- A reader on a phone can keep one too ------------------------------
  //
  // The rail is dropped below its own width, so the face the reader reaches
  // here is the one in the header's folded panel. The whole of it is asked in
  // the reader's own order: scroll, open the panel, press, and follow the place
  // back — at this width from beginning to end, which is the round trip a
  // reader on a phone actually makes.
  //
  // The reason the control is in the header rather than at the end of the
  // article is asserted directly: opening the panel leaves the window where it
  // was, so the place kept is the reader's and not the control's.
  {
    const context = await browser.newContext({ viewport: NARROW });
    const page = await context.newPage();
    const site = 'a-narrow-reader-can-keep-a-place';
    const proof = await applyMutation(page, site);
    const reached = await keepThePlace(page, PAGE, { fold: { site } });
    checkProof(proof);
    if (reached.before !== SCROLL_TO || reached.after !== SCROLL_TO) {
      fail(
        site,
        `reaching the control moved the window from ${reached.before} to ${reached.after};`
        + ` a control reached at ${NARROW.width}px has to leave the reader where they were reading`,
      );
    }
    const { row, present } = await deskRow(page);
    if (!present) {
      fail(site, `a place kept at ${NARROW.width}px is not offered back on the desk`);
    }
    await row.locator('[data-continue-link]').first().click();
    await page.waitForLoadState('domcontentloaded');
    await page.evaluate(() => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 60)));
    }));
    const landed = await page.evaluate(() => Math.round(window.scrollY));
    if (Math.abs(landed - SCROLL_TO) > LANDING_SLACK) {
      fail(
        site,
        `a place kept and followed at ${NARROW.width}px landed at ${landed},`
        + ` want ${SCROLL_TO} give or take ${LANDING_SLACK}`,
      );
    }
    await context.close();
  }

  // --- And it survives being kept at one width and followed at another ---
  //
  // Kept at the wide width and followed on a phone. The column reflows between
  // the two, so the place comes back at a different pixel than it was taken at
  // — which is why a mark is an anchor and a distance below it rather than a
  // pixel, and why this crossing is driven rather than two runs at one width.
  {
    const context = await browser.newContext({ viewport: VIEWPORT });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'a-narrow-reader-is-offered-the-place-back');
    await keepThePlace(page, PAGE);
    await page.setViewportSize(NARROW);
    const { row, present } = await deskRow(page);
    checkProof(proof);
    if (!present) {
      fail('a-narrow-reader-is-offered-the-place-back', `the desk offers no way back at ${NARROW.width}px`);
    }
    const link = row.locator('[data-continue-link]').first();
    if (!await link.isVisible()) {
      fail(
        'a-narrow-reader-is-offered-the-place-back',
        `the desk holds a way back that no reader at ${NARROW.width}px can see`,
      );
    }
    const href = await link.getAttribute('href');
    if (!href) broken('the way back carries no address');
    const address = new URL(href, BASE);
    const below = Number(address.searchParams.get('at') ?? '0');
    const anchorId = address.hash.length > 1 ? decodeURIComponent(address.hash.slice(1)) : '';
    // A place kept a hair below its anchor would land in the same pixel whether
    // the distance was applied or dropped, and a comparison that cannot tell
    // those apart is not one.
    if (!Number.isInteger(below) || below <= LANDING_SLACK) {
      broken(`the kept place sits ${below} below its anchor, too little to tell landing from not landing`);
    }
    await link.click();
    await page.waitForLoadState('domcontentloaded');
    await page.evaluate(() => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 60)));
    }));
    // Measured at the width being landed at, and held against the end of the
    // document, because a position past the last screen is one the browser
    // clamps rather than one the page got wrong.
    const { landed, wanted } = await page.evaluate(({ id, distance }) => {
      const element = id ? document.getElementById(id) : null;
      const top = element ? Math.round(element.getBoundingClientRect().top + window.scrollY) : 0;
      const scroller = document.scrollingElement ?? document.documentElement;
      const furthest = Math.max(0, scroller.scrollHeight - window.innerHeight);
      return { landed: Math.round(window.scrollY), wanted: Math.min(top + distance, furthest) };
    }, { id: anchorId, distance: below });
    if (Math.abs(landed - wanted) > LANDING_SLACK) {
      fail(
        'a-narrow-reader-is-offered-the-place-back',
        `following the row at ${NARROW.width}px landed at ${landed}, want ${wanted} give or take ${LANDING_SLACK}`,
      );
    }
    await context.close();
  }

  console.log(
    'PASS reader-mark: a kept place returns on the desk, lands where the window was,'
    + ' leaves a reader at the top when the anchor is gone, says when the note changed,'
    + ' is replaced by the next one, can be kept from the header on a phone without'
    + ' moving the page, and survives a reflow between the width it was kept at and'
    + ' the one it is followed at',
  );
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
