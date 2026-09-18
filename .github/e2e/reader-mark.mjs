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
// The probe drives two widths. At the first the reading page draws its right
// rail, which is where the control lives: the rail is its own scroll container,
// so reaching the control does not move the article out from under the position
// being kept. Below the rail's width there is no rail and so no control a
// reader can reach — the way back still works there, and that gap is measured
// below rather than left for a reader to find.
const VIEWPORT = { width: 1600, height: 900 };
const NARROW = { width: 390, height: 844 };
const LANDING_SLACK = 4;

const SITES = [
  'kept-place-is-offered-back',
  'following-it-lands-where-the-window-was',
  'a-changed-note-says-so',
  'keeping-another-replaces-it',
  'a-narrow-reader-cannot-reach-the-control',
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
  'say-nothing-about-a-changed-note': {
    target: 'a-changed-note-says-so',
    apply: rewriteDesk('y-continue__notice', 'y-continue__silent', 'the row notice class'),
  },
  'keep-the-previous-mark': {
    target: 'keeping-another-replaces-it',
    apply: dropSecondPost(),
  },
  'show-the-rail-when-narrow': {
    target: 'a-narrow-reader-cannot-reach-the-control',
    apply: rewriteStylesheet(
      '.y-rail-right{display:none!important}',
      '.y-rail-right{display:flex!important}',
      'the rule that drops the right rail below its width',
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
// to have something to offer rather than for a fixed delay.
const keepThePlace = async (page, path, { tamperIdentity = false } = {}) => {
  await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
  // The control is drawn once, in the right rail, and this helper is only ever
  // driven at a width that has one. Asking for the visible one rather than the
  // first in document order is what makes that a measurement instead of an
  // assumption: a rail that stopped being drawn, or one hidden by its own width
  // rule, reads here as zero rather than passing on a node nobody can press.
  const control = page.locator('[data-mark-control]:visible');
  if (await page.locator('[data-mark-control]').count() === 0) {
    broken(`${path} carries no mark control`);
  }
  if (await control.count() !== 1) {
    broken(`${path} shows ${await control.count()} mark controls at this width, want exactly 1`);
  }
  if (tamperIdentity) {
    // The stored identity is made to disagree with the note's own, which is
    // what an edit between keeping the place and coming back produces. No file
    // is touched: the disagreement is the whole of what the row reads.
    await control.evaluate((node) => {
      node.dataset.markIdentity = 'f'.repeat(64);
    });
  }
  await page.evaluate((y) => window.scrollTo(0, y), SCROLL_TO);
  const posted = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/marks',
    { timeout: 4000 },
  ).catch(() => null);
  await control.locator('[data-mark-button]').click();
  await posted;
  // The confirmation is the page's own word that the round trip finished, so
  // the desk is not asked before the file exists.
  await control.locator('[data-mark-said]').waitFor({ state: 'attached', timeout: 4000 });
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

  // --- Below the rail's width the control is out of reach ---------------
  //
  // The rail is the control's only home and the rail is dropped below its own
  // width, so a reader on a phone is offered a place back and cannot keep one.
  // That is the shape of the page today, measured rather than assumed: when a
  // control does reach this width, this is what says so.
  {
    const context = await browser.newContext({ viewport: NARROW });
    const page = await context.newPage();
    const proof = await applyMutation(page, 'a-narrow-reader-cannot-reach-the-control');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (await page.locator('[data-mark-control]').count() === 0) {
      broken(`${PAGE} carries no mark control at all, so what this width can reach proves nothing`);
    }
    const reachable = await page.locator('[data-mark-control]:visible').count();
    checkProof(proof);
    if (reachable !== 0) {
      fail(
        'a-narrow-reader-cannot-reach-the-control',
        `at ${NARROW.width}px ${reachable} mark control(s) can be reached; the control has no home at this`
        + ' width yet, so a reachable one means the page has moved on and this probe has to move with it',
      );
    }
    await context.close();
  }

  // --- And the way back still works there -------------------------------
  //
  // Kept at the width that has a control and followed on a phone, which is the
  // only order available while the control lives in the rail. The column
  // reflows between the two, so the place comes back at a different pixel than
  // it was taken at — which is why a mark is an anchor and a distance below it
  // rather than a pixel.
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
    + ' says when the note changed, is replaced by the next one, and survives a'
    + ' reflow onto a width whose reader cannot keep one',
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
