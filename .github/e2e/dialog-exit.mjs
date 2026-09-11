// Behavior lock: the three top-layer surfaces close on the same curve they
// open. The transition used to be declared only on [open] / :popover-open, so
// native close dropped the declaration and the surface vanished mid-curve.
// Go tests cannot see CSS. The probe opens each surface, closes it the way a
// reader does, and reads the animations that started. The preview is also
// asked to stay where it was: its close used to drop the CSS anchor before
// the exit had painted, and the card jumped to the viewport corner.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson that carries a concept term and a
// plain wikilink), and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SEARCH = '[data-search]';
const SEARCH_OPEN = '[data-search-open]';
const SHEET = '[data-concept-sheet]';
const CONCEPT = '[data-concept]';
const PREVIEW = '[data-preview-card]';
const PREVIEW_LINK = '.y-prose a.wikilink[href="/notes/Notes/Glass%20Tide.md"]:not(.concept-link)';
const SITES = [
  'search-exit-has-frames',
  'sheet-exit-has-frames',
  'preview-exit-has-frames',
  'preview-exit-stays-put',
  'reduced-motion-cuts-through',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN dialog-exit: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL dialog-exit: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN dialog-exit: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED dialog-exit: ${message}`);
};

// Appends after the product stylesheet so the rule outranks what it stands in
// for without an importance flag, except the reduced-motion override which
// has to match the blanket's own !important.
const appendStylesheet = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({
      response,
      body: `${original}\n${rule}\n`,
    });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the appended rule reached no page');
};

// Rewrites the preview module. The replacement is counted, so a needle that
// no longer matches the source reports itself rather than passing as a
// mutation nobody noticed.
const rewritePreview = (needle, replacement) => async (page) => {
  let matched = -1;
  await page.route('**/static/preview.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matched = original.split(needle).length - 1;
    await route.fulfill({ response, body: original.split(needle).join(replacement) });
  });
  return () =>
    matched === 1
      ? ''
      : `the module needle ${JSON.stringify(needle)} matched ${matched === -1 ? 'nothing, because the module was never fetched' : `${matched} times, want 1`}`;
};

const MUTATIONS = {
  // The original defect on the search dialog: the transition lives only while
  // [open] is set, so close() has nowhere for exit frames to run.
  'restore-search-open-only': {
    target: 'search-exit-has-frames',
    apply: appendStylesheet(
      '.y-searchdialog{transition:none;opacity:1;transform:none}.y-searchdialog[open]{transition:opacity 200ms,transform 200ms}',
    ),
  },
  'restore-sheet-open-only': {
    target: 'sheet-exit-has-frames',
    apply: appendStylesheet(
      '.y-conceptsheet{transition:none;opacity:1}.y-conceptsheet[open]{transition:opacity 200ms}',
    ),
  },
  'restore-preview-open-only': {
    target: 'preview-exit-has-frames',
    apply: appendStylesheet(
      '.y-preview{transition:none;opacity:1}.y-preview:popover-open{transition:opacity 120ms}',
    ),
  },
  // The original defect on the hover card: the CSS anchor is stripped in
  // close() before hidePopover() returns, so the painted exit has no
  // position-anchor and the card teleports to the viewport corner.
  'strip-anchor-before-hide': {
    target: 'preview-exit-stays-put',
    apply: rewritePreview(
      `    if (card.matches(':popover-open')) {
      card.hidePopover();
      return;
    }
    anchored?.removeAttribute('data-preview-open');
    anchored = null;`,
      `    anchored?.removeAttribute('data-preview-open');
    anchored = null;
    if (card.matches(':popover-open')) card.hidePopover();`,
    ),
  },
  // Keeps a visible exit under reduced motion, which the blanket already cuts.
  'keep-exit-under-reduced-motion': {
    target: 'reduced-motion-cuts-through',
    apply: appendStylesheet(
      '@media (prefers-reduced-motion: reduce){.yomihon .y-searchdialog,.yomihon .y-conceptsheet,.yomihon .y-preview{transition-duration:200ms!important}}',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`dialog-exit: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`dialog-exit: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`dialog-exit: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const waitSettled = (page, selector) =>
  page.evaluate(async (sel) => {
    const el = document.querySelector(sel);
    if (!el) return;
    await Promise.all(el.getAnimations().map((a) => a.finished.catch(() => {})));
  }, selector);

const closeAndRead = (page, selector, kind) =>
  page.evaluate(({ sel, kind: closeKind }) => {
    const el = document.querySelector(sel);
    if (!el) return { error: 'missing' };
    if (closeKind === 'popover') {
      if (!el.matches(':popover-open')) return { error: 'not-open' };
      el.hidePopover();
    } else {
      if (!el.open) return { error: 'not-open' };
      el.close();
    }
    const anims = el.getAnimations();
    return {
      count: anims.length,
      durations: anims.map((a) => {
        const duration = a.effect?.getComputedTiming().duration;
        return typeof duration === 'number' ? duration : 0;
      }),
    };
  }, { sel: selector, kind });

const longest = (reading) => (reading.durations ?? []).reduce((max, n) => (n > max ? n : max), 0);

const boxShift = (reading) => {
  const dx = Math.abs((reading.next?.x ?? 0) - (reading.start?.x ?? 0));
  const dy = Math.abs((reading.next?.y ?? 0) - (reading.start?.y ?? 0));
  return { dx, dy };
};

// The preview's close is the module's close(), reached the way a reader
// reaches it — Escape — not a direct hidePopover(). hidePopover() never
// drops the CSS anchor, so a jump the production close used to cause
// could not appear.
const closePreviewAsReader = (page, selector) =>
  page.evaluate(async (sel) => {
    const el = document.querySelector(sel);
    if (!el) return { error: 'missing' };
    if (!el.matches(':popover-open')) return { error: 'not-open' };
    const start = el.getBoundingClientRect();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    const anims = el.getAnimations();
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    const next = el.getBoundingClientRect();
    return {
      count: anims.length,
      durations: anims.map((a) => {
        const duration = a.effect?.getComputedTiming().duration;
        return typeof duration === 'number' ? duration : 0;
      }),
      start: { x: start.x, y: start.y },
      next: { x: next.x, y: next.y },
    };
  }, selector);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.waitForLoadState('load');
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  await page.locator(SEARCH_OPEN).click();
  await page.locator(`${SEARCH}[open]`).waitFor();
  await waitSettled(page, SEARCH);
  const searchExit = await closeAndRead(page, SEARCH, 'dialog');
  if (searchExit.error) broken(`search close: ${searchExit.error}`);
  if (searchExit.count < 1 || longest(searchExit) < 50) {
    fail(
      'search-exit-has-frames',
      `search close started ${searchExit.count} animations, longest ${longest(searchExit)}ms`,
    );
  }

  const concept = page.locator(CONCEPT).first();
  if ((await concept.count()) < 1) broken('the lesson paints no [data-concept] to open the sheet');
  await concept.click();
  await page.locator(`${SHEET}[open]`).waitFor();
  await waitSettled(page, SHEET);
  const sheetExit = await closeAndRead(page, SHEET, 'dialog');
  if (sheetExit.error) broken(`sheet close: ${sheetExit.error}`);
  if (sheetExit.count < 1 || longest(sheetExit) < 50) {
    fail(
      'sheet-exit-has-frames',
      `sheet close started ${sheetExit.count} animations, longest ${longest(sheetExit)}ms`,
    );
  }

  const previewLink = page.locator(PREVIEW_LINK);
  if ((await previewLink.count()) < 1) broken('the lesson paints no Glass Tide wikilink to open a card');
  await previewLink.hover();
  await page.waitForFunction((sel) => document.querySelector(sel)?.matches(':popover-open'), PREVIEW, {
    timeout: 4000,
  });
  await waitSettled(page, PREVIEW);
  const previewExit = await closePreviewAsReader(page, PREVIEW);
  if (previewExit.error) broken(`preview close: ${previewExit.error}`);
  if (previewExit.start.x === 0 && previewExit.start.y === 0) {
    broken('preview opened at the viewport origin, so a stay-put check would pass over a card that was never anchored');
  }
  if (previewExit.count < 1 || longest(previewExit) < 50) {
    fail(
      'preview-exit-has-frames',
      `preview close started ${previewExit.count} animations, longest ${longest(previewExit)}ms`,
    );
  }
  const shift = boxShift(previewExit);
  if (shift.dx > 8 || shift.dy > 8) {
    fail(
      'preview-exit-stays-put',
      `preview jumped from x=${Math.round(previewExit.start.x)},y=${Math.round(previewExit.start.y)} to x=${Math.round(previewExit.next.x)},y=${Math.round(previewExit.next.y)} on close`,
    );
  }

  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.locator(SEARCH_OPEN).click();
  await page.locator(`${SEARCH}[open]`).waitFor();
  await waitSettled(page, SEARCH);
  const reduced = await closeAndRead(page, SEARCH, 'dialog');
  if (reduced.error) broken(`reduced-motion close: ${reduced.error}`);
  if (longest(reduced) >= 50) {
    fail(
      'reduced-motion-cuts-through',
      `search close under reduced motion lasted ${longest(reduced)}ms`,
    );
  }

  console.log('PASS dialog-exit: search, sheet, and preview close with frames; preview stays put; reduced motion cuts through');
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
