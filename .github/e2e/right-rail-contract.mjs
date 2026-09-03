// Behavior lock for the wide reading rail. A long table of contents, the
// status controls, and diagnostics retain their full content height; the rail
// is the one vertical scroller that makes each focus target reachable. It also
// holds the order the reader meets them in: the note's own shape, then what
// leads to it from elsewhere in the vault, and the ruling last — a small verb
// beside the reading rather than the frame around it.
//
// Env: YOMIHON_BASE, PAGE_PATH (the long-TOC diagnostic fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITE = 'rail-content-reachable';
const ORDER_SITE = 'reading-precedes-the-ruling';
const SITES = [SITE, ORDER_SITE];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (message) => { throw new LockFired(SITE, `FAIL right-rail-contract: ${message}`); };
const failOrder = (message) => { throw new LockFired(ORDER_SITE, `FAIL right-rail-contract: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN right-rail-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED right-rail-contract: ${message}`); };

const restoreChildShrink = async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const needle = '</head>';
    const count = original.split(needle).length - 1;
    matches += count;
    const style = '<style data-e2e-rail-shrink>.y-rail-right > *{flex-shrink:1!important}</style>';
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, `${style}${needle}`) : original });
  });
  return () => {
    if (requests !== 1) return `document was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `document head needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

// Appends to the product's own stylesheet, landing outside the layer it
// declares, so the rule outranks what it stands in for without an importance
// flag. The rail is a column flex container, so an order of -1 lifts the panel
// back above the reading without moving one byte of the document.
const rulingFirst = async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n.y-rail-right .y-statuspanel{order:-1}\n` });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

const MUTATIONS = {
  'restore-child-shrink': {
    target: SITE,
    apply: restoreChildShrink,
  },
  'put-the-ruling-first': {
    target: ORDER_SITE,
    apply: rulingFirst,
  },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`right-rail-contract: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`right-rail-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`right-rail-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const inspectTarget = (target) => target.evaluate(async (element) => {
  const rail = element.closest('.y-rail-right');
  element.scrollIntoView({ block: 'center' });
  element.focus();
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));

  const rect = element.getBoundingClientRect();
  let left = rect.left;
  let top = rect.top;
  let right = rect.right;
  let bottom = rect.bottom;
  for (let current = element.parentElement; current; current = current.parentElement) {
    const style = getComputedStyle(current);
    if (/(auto|scroll|hidden|clip)/.test(`${style.overflowX} ${style.overflowY}`)) {
      const clip = current.getBoundingClientRect();
      left = Math.max(left, clip.left);
      top = Math.max(top, clip.top);
      right = Math.min(right, clip.right);
      bottom = Math.min(bottom, clip.bottom);
    }
    if (current === rail) break;
  }
  const paintedHeight = Math.max(0, bottom - top);
  const paintedWidth = Math.max(0, right - left);
  const hit = paintedWidth > 0 && paintedHeight > 0
    ? document.elementFromPoint((left + right) / 2, (top + bottom) / 2)
    : null;
  const style = getComputedStyle(element);
  return {
    active: document.activeElement === element,
    focusVisible: element.matches(':focus-visible'),
    outline: `${style.outlineStyle}/${style.outlineWidth}`,
    tabIndex: element.tabIndex,
    paintedHeight,
    paintedWidth,
    targetHeight: rect.height,
    hit: Boolean(hit && (hit === element || element.contains(hit))),
  };
});

const assertTarget = async (target, label, height) => {
  const got = await inspectTarget(target);
  if (!got.active || !got.focusVisible || got.tabIndex < 0 || got.outline.startsWith('none/') || got.paintedHeight < got.targetHeight * 0.75 || !got.hit) {
    fail(`${label} is not keyboard-visible at 1600×${height}: ${JSON.stringify(got)}`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  for (const height of [768, 900]) {
    const page = await browser.newPage({ viewport: { width: 1600, height } });
    proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }

    const rail = page.locator('.y-rail-right');
    if (await rail.count() !== 1) broken(`the fixture has ${await rail.count()} right rails, want 1`);
    const shape = await rail.evaluate((element) => ({
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      overflowY: getComputedStyle(element).overflowY,
      children: [...element.children].map((child) => ({
        name: `${child.tagName.toLowerCase()}.${child.className || '-'}`,
        clientHeight: child.clientHeight,
        scrollHeight: child.scrollHeight,
        flexShrink: getComputedStyle(child).flexShrink,
      })),
    }));
    if (shape.children.length !== 4) broken(`the fixture has ${shape.children.length} rail children, want the outline, cited-by, diagnostics, and the status panel`);
    if (shape.overflowY !== 'auto' || shape.scrollHeight <= shape.clientHeight) {
      broken(`the fixture does not exercise one overflowing rail at 1600×${height}: ${JSON.stringify(shape)}`);
    }
    for (const child of shape.children) {
      if (child.clientHeight + 1 < child.scrollHeight) {
        fail(`${child.name} clipped content at 1600×${height}: ${JSON.stringify(child)}`);
      }
    }
    for (const child of shape.children) {
      if (child.flexShrink !== '0') {
        fail(`${child.name} can still shrink at 1600×${height}: ${JSON.stringify(child)}`);
      }
    }

    const tocLinks = rail.locator('.y-toc__list a');
    // The panel's reachable controls in its resting state: plain transition
    // buttons plus the summary of a terminal target's closed confirm. The
    // confirm's inner submit is deliberately unreachable until the disclosure
    // is opened — that second step is the point — so only visible controls
    // belong in the keyboard walk.
    const statusControls = rail.locator('.y-statuspanel button:visible, .y-statuspanel summary:visible');
    const diagnostics = rail.locator('.y-diag');
    // The cited-by block never leaves the rail: it says "nothing cites this"
    // when nothing does, so a missing block is the answer going missing rather
    // than the answer being empty.
    if (await rail.locator('.y-citedby').count() !== 1) broken('the fixture has no cited-by block');
    if (await tocLinks.count() !== 24) broken(`the fixture has ${await tocLinks.count()} TOC links, want 24`);
    if (await statusControls.count() === 0) broken('the fixture has no status control');
    if (await diagnostics.count() === 0) broken('the fixture has no diagnostic card');

    for (let i = 0; i < await tocLinks.count(); i += 1) {
      await assertTarget(tocLinks.nth(i), `TOC link ${i + 1}`, height);
    }
    for (let i = 0; i < await statusControls.count(); i += 1) {
      await assertTarget(statusControls.nth(i), `status control ${i + 1}`, height);
    }
    // What the reader meets, and in which order. The rail is its own scroll
    // container, so whatever sits at the top is what is seen there without
    // scrolling: the note's own shape, then what leads to it, and the ruling
    // last. Measured where each block is painted rather than where it is
    // written, because a stylesheet can reorder a flex column without moving a
    // byte of the document.
    const tops = await rail.evaluate((element) => {
      const top = (selector) => {
        const found = element.querySelector(selector);
        return found ? found.getBoundingClientRect().top + element.scrollTop : null;
      };
      return { outline: top('nav .y-toc__list'), cited: top('.y-citedby'), ruling: top('.y-statuspanel') };
    });
    for (const [name, value] of Object.entries(tops)) {
      if (value === null) broken(`the fixture has no ${name} block, so the order it stands in proves nothing`);
    }
    if (!(tops.outline < tops.cited)) {
      failOrder(`what leads to this note is painted above the note's own shape at 1600×${height}: ${JSON.stringify(tops)}`);
    }
    if (!(tops.cited < tops.ruling)) {
      failOrder(`the ruling is painted above what leads to this note at 1600×${height}: ${JSON.stringify(tops)}; the verb has taken the frame's place`);
    }

    // Walking every control in the panel locks its own internal order, and
    // walking on out of the outline locks that the reading hands downward
    // rather than being reached last.
    await statusControls.first().focus();
    for (let i = 1; i < await statusControls.count(); i += 1) {
      await page.keyboard.press('Tab');
      const reached = await statusControls.nth(i).evaluate((element) => document.activeElement === element);
      if (!reached) {
        fail(`Tab order did not reach status control ${i + 1} at 1600×${height}`);
      }
    }
    await tocLinks.last().focus();
    await page.keyboard.press('Tab');
    const wentOn = await rail.evaluate((element) => {
      const active = document.activeElement;
      if (!element.contains(active)) return 'left the rail';
      const list = element.querySelector('.y-toc__list');
      return list.contains(active) ? 'stayed inside the outline' : '';
    });
    if (wentOn) {
      failOrder(`Tab from the end of the outline ${wentOn} at 1600×${height}, so the reading does not hand on to the rest of the rail`);
    }
    const diagnosticPaint = await diagnostics.first().evaluate((element) => {
      element.scrollIntoView({ block: 'center' });
      const rect = element.getBoundingClientRect();
      const railRect = element.closest('.y-rail-right').getBoundingClientRect();
      return {
        paintedHeight: Math.max(0, Math.min(rect.bottom, railRect.bottom) - Math.max(rect.top, railRect.top)),
        targetHeight: rect.height,
      };
    });
    if (diagnosticPaint.paintedHeight < diagnosticPaint.targetHeight * 0.75) {
      fail(`diagnostic card is unreachable at 1600×${height}: ${JSON.stringify(diagnosticPaint)}`);
    }
    await page.close();
  }

  console.log('PASS right-rail-contract: the reading leads and the ruling closes the rail, and every block stays reachable at 1600×768 and 1600×900');
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
