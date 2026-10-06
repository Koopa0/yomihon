// Behavior lock for the wide reading rail. A long table of contents, the
// status controls, and diagnostics retain their full content height; the rail
// is the one vertical scroller that makes each focus target reachable. It also
// holds the order the reader meets them in: the note's own shape, then the
// sources it declared, then what leads to it from the text of other notes, and
// the ruling last — a small verb beside the reading rather than the frame
// around it. Both rails sit against the window's edges however wide the window
// is, the left rail's text starts where the header's first control starts, and
// the right rail's text ends where the header's last control ends.
//
// Env: YOMIHON_BASE, PAGE_PATH (the long-TOC diagnostic fixture), and MUTATE.
import { chromium } from 'playwright-core';

const arrived = (page) => page.waitForFunction(
  async () => {
    if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return true;
  },
  null,
  { timeout: 3000 },
);

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITE = 'rail-content-reachable';
const ORDER_SITE = 'reading-precedes-the-ruling';
const DOOR_SITE = 'contents-door-shares-its-row';
const EDGE_SITE = 'rails-sit-at-the-window-edges';
const SITES = [SITE, ORDER_SITE, DOOR_SITE, EDGE_SITE];

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
const failDoor = (message) => { throw new LockFired(DOOR_SITE, `FAIL right-rail-contract: ${message}`); };
const failEdge = (message) => { throw new LockFired(EDGE_SITE, `FAIL right-rail-contract: ${message}`); };
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

// Restores the door as a row of its own: a row that stacks its children puts
// the door under the heading link, which is the doubled list the door was
// folded into the row to end.
const separateDoorRow = async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n.y-toc__row:has(.y-toc__door){display:block}\n` });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

// Gives the door its own strip again beside the heading, drawn or not.
const reserveDoorStrip = async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n.y-toc__list .y-toc__door{flex:none;min-width:28px;position:static}\n` });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

// Appends one rule to the product's own stylesheet. Each mutation of the rails'
// place is a single rule that puts back what the layout stopped doing.
const appendRule = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

// The fixture's contract names no answer type, so its pages draw no thought
// doors, and naming one would change the outline every other probe counts and
// walks. The rows are the product's own, one per heading; each is given the
// door the template would write into it, so what is measured is the product's
// stylesheet against a row that holds a door. The door's markup is locked by
// the render test in internal/ui/pages.
const standUpDoors = (page) => page.evaluate(() => {
  const icon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M21 15a2 2 0 0 1-2 2H8l-5 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>';
  const rows = document.querySelectorAll('.y-toc__row');
  for (const row of rows) {
    row.insertAdjacentHTML('beforeend', `<a class="y-toc__door" href="/thought/probe" aria-label="Leave a thought: ${row.firstElementChild.textContent}">${icon}</a>`);
  }
  return rows.length;
});

const MUTATIONS = {
  'restore-the-separate-door-row': {
    target: DOOR_SITE,
    apply: separateDoorRow,
  },
  'reserve-the-door-strip': {
    target: DOOR_SITE,
    apply: reserveDoorStrip,
  },
  'restore-child-shrink': {
    target: SITE,
    apply: restoreChildShrink,
  },
  'put-the-ruling-first': {
    target: ORDER_SITE,
    apply: rulingFirst,
  },
  'centre-the-shell-again': {
    target: EDGE_SITE,
    apply: appendRule('.y-shell{margin-inline:auto;max-width:1280px}'),
  },
  'leave-the-right-rail-short-of-the-edge': {
    target: EDGE_SITE,
    apply: appendRule('.y-shell{padding-right:80px}'),
  },
  'crowd-the-left-rail-against-the-edge': {
    target: EDGE_SITE,
    apply: appendRule(':root{--rail-pad-start:6px}'),
  },
  'pad-the-rail-past-the-header': {
    target: EDGE_SITE,
    apply: appendRule('.y-rail-right{padding-right:24px}'),
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
    if (shape.children.length !== 7) broken(`the fixture has ${shape.children.length} rail children, want the outline, the note that belongs beside this one, declared sources, cited-by, diagnostics, the mark control, and the status panel`);
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
    if (await rail.locator('.y-basedon').count() !== 1) broken('the fixture has no declared-source block');
    if (await rail.locator('.y-pair').count() !== 1) broken('the fixture is offered no note to read beside this one');
    if (await rail.locator('.y-markset').count() !== 1) broken('the fixture has no control for keeping a reading place');
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
      return { outline: top('nav .y-toc__list'), pair: top('.y-pair'), based: top('.y-basedon'), cited: top('.y-citedby'), ruling: top('.y-statuspanel') };
    });
    for (const [name, value] of Object.entries(tops)) {
      if (value === null) broken(`the fixture has no ${name} block, so the order it stands in proves nothing`);
    }
    if (!(tops.outline < tops.pair)) {
      failOrder(`the note that belongs beside this one is painted above this note's own shape at 1600×${height}: ${JSON.stringify(tops)}`);
    }
    if (!(tops.pair < tops.based)) {
      failOrder(`declared sources are painted above the offer to read this note beside another at 1600×${height}: ${JSON.stringify(tops)}`);
    }
    if (!(tops.based < tops.cited)) {
      failOrder(`what leads to this note is painted above the sources it declared at 1600×${height}: ${JSON.stringify(tops)}`);
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

  // Case: the section doors. One heading stays one row with its door at the
  // row's end inside it; the door is drawn on hover and on focus, and always
  // where there is no hover, without widening the phone. Where a pointer can
  // reveal it, it takes no width of its own, so the heading's link runs the
  // whole row and a long heading wraps where it would with no door at all.
  {
    const wide = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await wide.newPage();
    const proof = MUTATE && MUTATIONS[MUTATE].target === DOOR_SITE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => document.fonts.ready);
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    if (await standUpDoors(page) < 2) broken(`${PAGE} has fewer than two headings, so there is no row to measure`);
    const railRows = page.locator('.y-rail-right .y-toc__row');
    const rowCount = await railRows.count();
    if (rowCount < 2) broken('the rail draws fewer than two contents rows');
    for (let i = 0; i < rowCount; i += 1) {
      const shape = await railRows.nth(i).evaluate((row) => {
        const box = (element) => element.getBoundingClientRect();
        const link = row.firstElementChild;
        const door = row.querySelector('.y-toc__door');
        return { row: box(row), link: box(link), door: box(door), opacity: getComputedStyle(door).opacity, doors: row.querySelectorAll('.y-toc__door').length };
      });
      if (shape.doors !== 1) broken(`contents row ${i + 1} holds ${shape.doors} doors, want 1`);
      if (shape.row.height > shape.link.height + 1) {
        failDoor(`contents row ${i + 1} is ${shape.row.height}px tall around a ${shape.link.height}px heading link, so its door is a row of its own`);
      }
      if (!(shape.door.left >= shape.row.left && shape.door.right <= shape.row.right + 1 && shape.door.right >= shape.row.right - 1 && shape.door.top >= shape.row.top - 1 && shape.door.bottom <= shape.row.bottom + 1)) {
        failDoor(`contents row ${i + 1} does not hold its door at the row's end: ${JSON.stringify(shape)}`);
      }
      if (shape.link.width < shape.row.width - 1) {
        failDoor(`contents row ${i + 1} gives its heading ${shape.link.width}px of a ${shape.row.width}px row, so the door's hidden strip still narrows the heading`);
      }
      if (shape.opacity !== '0') failDoor(`contents row ${i + 1} draws its door (opacity ${shape.opacity}) with no pointer on the row and no focus in it`);
    }
    const door = page.locator('.y-rail-right .y-toc__door').first();
    await railRows.first().hover();
    if (await door.evaluate((element) => getComputedStyle(element).opacity) !== '1') failDoor('hovering a contents row does not draw its door');
    await page.mouse.move(700, 500);
    if (await door.evaluate((element) => getComputedStyle(element).opacity) !== '0') failDoor('the door stays drawn after the pointer leaves its row');
    await door.focus();
    if (await door.evaluate((element) => getComputedStyle(element).opacity) !== '1') failDoor('focus inside a contents row does not draw its door');
    await wide.close();

    const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true });
    const narrow = await phone.newPage();
    await narrow.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await arrived(narrow);
    await narrow.evaluate(() => document.fonts.ready);
    await standUpDoors(narrow);
    await narrow.locator('details.y-toc-inline summary').first().click();
    const inline = narrow.locator('.y-toc-inline .y-toc__door').first();
    await inline.waitFor({ state: 'visible' });
    // A box pinned to the viewport is the gauge the document's own width is
    // judged against: nothing that widens the page can be what widened it.
    const coarse = await narrow.evaluate(() => {
      const gauge = document.createElement('div');
      gauge.style.cssText = 'position:fixed;inset:0;pointer-events:none';
      document.body.append(gauge);
      const viewport = gauge.getBoundingClientRect().width;
      gauge.remove();
      // The fold cuts off what is inside it, so a door pushed past the phone is
      // lost without the document growing; its own edge is measured too.
      const door = document.querySelector('.y-toc-inline .y-toc__door').getBoundingClientRect();
      return { coarse: matchMedia('(pointer: coarse)').matches, overflow: Math.max(document.documentElement.scrollWidth - viewport, door.right - viewport) };
    });
    if (!coarse.coarse) broken('the phone context does not report a coarse pointer');
    if (await inline.evaluate((element) => getComputedStyle(element).opacity) !== '1') failDoor('under a coarse pointer the door is not drawn without a hover');
    if (coarse.overflow > 0) failDoor(`the doors widen the phone by ${coarse.overflow}px`);
    await phone.close();
  }

  // Case: the rails are the window's edges. In a window wider than the three
  // columns want, the left rail starts at the window's left side, the right rail
  // ends at its right side, and what each holds stands as far in from its side
  // as the header's control at that side does, so each reads as one line down
  // the page with the header above it. Measured at a width where a capped,
  // centred shell would stand well in from both sides.
  {
    const wide = await browser.newContext({ viewport: { width: 1920, height: 900 } });
    const page = await wide.newPage();
    const proof = MUTATE && MUTATIONS[MUTATE].target === EDGE_SITE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => document.fonts.ready);
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const edges = await page.evaluate(() => {
      const left = document.querySelector('.y-rail-left');
      const right = document.querySelector('.y-rail-right');
      if (!left || !right || getComputedStyle(right).display === 'none') return null;
      // The header spans the page's own scroll box, so its right end is the
      // window's right side as the reader sees it, with a classic scrollbar
      // (drawn on Linux, overlaid on macOS) already taken out of the width.
      const viewport = document.querySelector('.y-header').getBoundingClientRect().right;
      const controls = [...document.querySelectorAll('.y-header a, .y-header button')]
        .filter((element) => element.getClientRects().length > 0)
        .map((element) => element.getBoundingClientRect());
      // The first words the left rail's body draws, measured where the glyphs
      // are rather than where their box begins. Text in a folded group or a
      // hidden label has no box and is passed over.
      const walker = document.createTreeWalker(left.querySelector('.y-railbody'), NodeFilter.SHOW_TEXT, {
        acceptNode: (node) => (node.textContent.trim() ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT),
      });
      let words = null;
      let wordsIn = '';
      for (let node = walker.nextNode(); node && !words; node = walker.nextNode()) {
        const range = document.createRange();
        range.selectNodeContents(node);
        const box = [...range.getClientRects()].find((rect) => rect.width > 1 && rect.height > 1);
        if (box) {
          words = box;
          wordsIn = `${node.parentElement.tagName.toLowerCase()}.${node.parentElement.className}`;
        }
      }
      if (!words) return { noRailText: true };
      const rail = right.getBoundingClientRect();
      return {
        viewport,
        leftRailStart: left.getBoundingClientRect().left,
        leftRailTextStart: words.left,
        leftRailTextIn: wordsIn,
        headerStart: Math.min(...controls.map((box) => box.left)),
        rightRailEnd: rail.right,
        railTextEndFromEdge: viewport - (rail.right - parseFloat(getComputedStyle(right).paddingRight)),
        headerEndFromEdge: viewport - Math.max(...controls.map((box) => box.right)),
      };
    });
    if (!edges) broken(`${PAGE} draws no pair of rails at 1920 wide`);
    if (edges.noRailText) broken(`${PAGE} draws no words in the left rail, so where they start proves nothing`);
    if (Math.abs(edges.leftRailStart) > 0.5) {
      failEdge(`the left rail starts ${edges.leftRailStart}px from the window's left side at 1920 wide, want 0: ${JSON.stringify(edges)}`);
    }
    if (Math.abs(edges.viewport - edges.rightRailEnd) > 0.5) {
      failEdge(`the right rail ends ${edges.viewport - edges.rightRailEnd}px short of the window's right side at 1920 wide, want 0: ${JSON.stringify(edges)}`);
    }
    if (Math.abs(edges.leftRailTextStart - edges.headerStart) > 0.5) {
      failEdge(`the left rail's words start ${edges.leftRailTextStart}px from the window's left side and the header's first control ${edges.headerStart}px: ${JSON.stringify(edges)}`);
    }
    if (Math.abs(edges.railTextEndFromEdge - edges.headerEndFromEdge) > 0.5) {
      failEdge(`the right rail's text ends ${edges.railTextEndFromEdge}px from the window's right side and the header's last control ${edges.headerEndFromEdge}px: ${JSON.stringify(edges)}`);
    }
    await wide.close();
  }

  console.log('PASS right-rail-contract: the reading leads and the ruling closes the rail, every block stays reachable at 1600×768 and 1600×900, each contents row holds its own door, and both rails stand against the window\'s edges, each holding its text as far in as the header does, at 1920');
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
