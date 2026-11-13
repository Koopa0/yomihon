// Browser lock for the mat an author's drawing sits on: a picture alone in its
// paragraph, and a figure's frame. It reads what a reader gets, in the eight
// contexts that decide it — a phone and a desk, a light and a dark system, a
// light and a dark page — because the mat answers to the system and the page
// separately, and a lock that never put the two in disagreement could not tell
// them apart.
//
// A green run means: the mat of a picture and of a figure nobody approved takes
// the system's tone whatever the page is, and the picture's own dark variant
// agrees with it; an approved figure's mat takes the page's; both are one 8px
// inset; on a phone a drawing wider than the screen is held at 560px inside a
// mat that scrolls, the page does not, and the mat is a named stop in the tab
// order that a key scrolls; on a desk nothing scrolls and nothing is a stop; a
// small picture is left at the size it was given; and a picture inside a
// sentence is never held to the floor.
//
// Env: YOMIHON_BASE, YOMIHON_FIXTURE_ROOT, YOMIHON_FIXTURE_HOME, PAGE_PATH and
// MUTATE. MUTATE=list prints the modes.
import { rm } from 'node:fs/promises';

import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { approveFigures } from './support/figure-approval.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/figure-mat.md';
const MUTATE = process.env.MUTATE || '';

// The tones are written out, not read from the stylesheet: a lock that took
// its expected colour from the rule under test would agree with any rule.
const SYSTEM_TONE = { light: 'rgb(230, 226, 216)', dark: 'rgb(42, 41, 37)' };
const PAGE_TONE = { light: 'rgb(238, 233, 220)', dark: 'rgb(30, 29, 26)' };
const FLOOR = 560;

const SITES = [
  'mat-ground',
  'mat-themed',
  'mat-inset',
  'mat-scrolls',
  'small-picture-kept',
  'floor-stays-on-the-mat',
  'mat-focus',
];

// Each mode breaks one thing a site protects, in the served stylesheet or
// script. An appended rule wins over the one it names by coming later at the
// same weight, so it needs no knowledge of the text around it.
const MUTATIONS = {
  // The mat takes the page's scheme instead of the system's.
  'mat-follows-page': { site: 'mat-ground', file: 'app.css', replace: ['color-scheme: light dark;', 'color-scheme: inherit;'] },
  // The approved figure's mat takes the system's scheme instead of the page's.
  'approved-mat-follows-system': { site: 'mat-themed', file: 'app.css', replace: ['color-scheme: inherit;', 'color-scheme: light dark;'] },
  // The figure's mat gets a different inset from the picture's.
  'uneven-inset': { site: 'mat-inset', file: 'app.css', append: '.y-prose .figure-embed .mat { padding: 6px; }' },
  // The mat no longer holds a wide drawing at a floor.
  'shrink-wide-picture': { site: 'mat-scrolls', file: 'app.css', replace: ['--mat-floor: 560px;', '--mat-floor: 0%;'] },
  // The mat no longer scrolls what it holds.
  'no-mat-scroll': { site: 'mat-scrolls', file: 'app.css', append: '.y-prose .mat { overflow-x: visible; }' },
  // The floor becomes a minimum, which makes a small picture larger.
  'upscale-small-picture': { site: 'small-picture-kept', file: 'app.css', append: '.y-prose .mat > img { min-width: var(--mat-floor); }' },
  // A picture inside a sentence is held to the floor too.
  'floor-outside-mat': { site: 'floor-stays-on-the-mat', file: 'app.css', replace: ['max-width: max(100%, var(--mat-floor, 0%));', 'max-width: max(100%, 560px);'] },
  // A mat that overflows is not given a place in the tab order.
  'no-focus-stop': { site: 'mat-focus', file: 'diagrams.js', replace: ["mat.setAttribute('tabindex', '0');", ''] },
  // Every mat is a tab stop, whether or not there is anything to scroll to.
  'always-focusable': { site: 'mat-focus', file: 'diagrams.js', replace: ['const scrolls = mat.scrollWidth - mat.clientWidth > 1;', 'const scrolls = true;'] },
  // A mat in the tab order is not named by its drawing.
  'unnamed-mat': { site: 'mat-focus', file: 'diagrams.js', replace: ["mat.setAttribute('aria-label', name);", ''] },
};

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

for (const { site } of Object.values(MUTATIONS)) {
  if (!SITES.includes(site)) {
    console.error(`figure-mat: a mutation aims at unknown site ${site}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.site === site)) {
    console.error(`figure-mat: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const mode of Object.keys(MUTATIONS)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`figure-mat: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN figure-mat: unknown site ${site}`);
  throw new LockFired(site, `FAIL figure-mat: ${message}`);
};
let applied = false;
// A needle that matches none or several sites is a mutation that did not
// apply, which is reported rather than read as green.
const rewrite = (body, mutation) => {
  if (mutation.append !== undefined) {
    applied = true;
    return `${body}\n${mutation.append}\n`;
  }
  const [needle, replacement] = mutation.replace;
  const count = body.split(needle).length - 1;
  if (count !== 1) throw new NotApplied(`NOT-APPLIED figure-mat: ${MUTATE} matched ${count} sites in ${mutation.file}`);
  applied = true;
  return body.replace(needle, replacement);
};

// What the page shows about its drawings, read in one pass so every assertion
// answers for the same moment.
const readPage = () => {
  const mats = [...document.querySelectorAll('.y-prose .mat')].map((mat) => {
    const drawing = mat.firstElementChild;
    const rect = drawing.getBoundingClientRect();
    const style = getComputedStyle(mat);
    const figure = drawing.matches('iframe');
    return {
      kind: figure ? (drawing.hasAttribute('data-figure-scripts') ? 'approved' : 'figure') : (drawing.getAttribute('src').includes('wide-drawing') ? 'wide' : 'small'),
      background: style.backgroundColor,
      padding: [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft],
      overflowX: style.overflowX,
      clientWidth: mat.clientWidth,
      scrollWidth: mat.scrollWidth,
      drawingWidth: rect.width,
      tabindex: mat.getAttribute('tabindex'),
      role: mat.getAttribute('role'),
      label: mat.getAttribute('aria-label'),
      name: drawing.getAttribute('alt') || drawing.getAttribute('title'),
    };
  });
  const inline = document.querySelector('.y-prose p:not(.mat) > img');
  const column = document.querySelector('.y-prose').getBoundingClientRect();
  const wide = document.querySelector('.y-prose .mat > img[src*="wide-drawing"]');
  let variant = null;
  if (wide) {
    const canvas = document.createElement('canvas');
    canvas.width = 4;
    canvas.height = 4;
    const context = canvas.getContext('2d');
    context.drawImage(wide, 0, 0, 4, 4);
    const [r, g, b] = context.getImageData(1, 1, 1, 1).data;
    variant = `rgb(${r}, ${g}, ${b})`;
  }
  return {
    mats,
    inline: inline && { width: inline.getBoundingClientRect().width, inMat: inline.closest('.mat') !== null },
    column: column.width,
    pageWidth: document.documentElement.scrollWidth,
    innerWidth,
    variant,
  };
};

let browser;
let approvalsFile;
let routeError;
try {
  approvalsFile = await approveFigures('Notes/figures', ['mat-live.html']);
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  const mutation = MUTATE ? MUTATIONS[MUTATE] : null;

  for (const width of [375, 1280]) for (const system of ['light', 'dark']) for (const theme of ['light', 'dark']) {
    const where = `${width}px, ${system} system, ${theme} page`;
    const context = await browser.newContext({ viewport: { width, height: 900 }, colorScheme: system });
    try {
      await context.addCookies([
        { name: 'yomihon_lang', value: 'en', url: BASE },
        { name: 'yomihon_theme', value: theme, url: BASE },
      ]);
      if (mutation) {
        await context.route(`${BASE}/static/*`, async (route) => {
          try {
            const url = new URL(route.request().url());
            if (url.pathname !== `/static/${mutation.file}`) return route.continue();
            const response = await route.fetch();
            return route.fulfill({ response, body: rewrite(await response.text(), mutation) });
          } catch (error) {
            routeError ??= error;
            return route.abort();
          }
        });
      }
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (response?.status() !== 200) throw new ProbeBroken(`BROKEN figure-mat: ${PAGE} answered ${response?.status()}`);
      await arrived(page, 10000);
      if (routeError) throw routeError;
      if (!(await page.content()).includes('MAT-END')) throw new ProbeBroken('BROKEN figure-mat: the fixture page did not arrive whole');
      await page.waitForFunction(() => [...document.images].every((image) => image.complete));
      // Resizing is observed, and what the observer decides lands a frame
      // later; two frames is as long as the page has to settle.
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await page.waitForTimeout(300);
      if (routeError) throw routeError;

      const seen = await page.evaluate(readPage);
      if (errors.length) throw new ProbeBroken(`BROKEN figure-mat: ${errors.join('; ')}`);
      const byKind = Object.fromEntries(seen.mats.map((mat) => [mat.kind, mat]));
      if (seen.mats.length !== 4 || ['wide', 'small', 'figure', 'approved'].some((kind) => !byKind[kind])) {
        throw new ProbeBroken(`BROKEN figure-mat: the fixture holds ${JSON.stringify(seen.mats.map((mat) => mat.kind))}, want a wide picture, a small one, a figure and an approved figure`);
      }

      // mat-ground
      for (const kind of ['wide', 'small', 'figure']) {
        if (byKind[kind].background !== SYSTEM_TONE[system]) {
          fail('mat-ground', `${where}: the ${kind} mat is ${byKind[kind].background}, want the system's ${SYSTEM_TONE[system]}`);
        }
      }
      if (seen.variant !== (system === 'dark' ? 'rgb(31, 45, 39)' : 'rgb(232, 240, 228)')) {
        fail('mat-ground', `${where}: the picture drew ${seen.variant}, which is not its ${system} variant, so its drawing and its mat are not the same light or dark`);
      }

      // mat-themed
      if (byKind.approved.background !== PAGE_TONE[theme]) {
        fail('mat-themed', `${where}: the approved figure's mat is ${byKind.approved.background}, want the page's ${PAGE_TONE[theme]}`);
      }

      // mat-inset
      for (const mat of seen.mats) {
        if (mat.padding.some((side) => side !== '8px')) fail('mat-inset', `${where}: the ${mat.kind} mat has padding ${mat.padding.join(' ')}, want 8px on every side`);
      }

      // floor-stays-on-the-mat: read before the page is, since a picture held to
      // a floor it does not belong to also makes the page scroll, and the finding
      // should name the picture.
      if (seen.inline.inMat || seen.inline.width > seen.column + 1) {
        fail('floor-stays-on-the-mat', `${where}: the picture inside a sentence is ${seen.inline.width}px wide in a ${seen.column}px column, want it no wider than the column`);
      }

      // mat-scrolls and small-picture-kept
      const small = byKind.small;
      if (seen.pageWidth > seen.innerWidth) fail('mat-scrolls', `${where}: the page itself is ${seen.pageWidth}px wide in a ${seen.innerWidth}px window`);
      for (const mat of seen.mats) {
        if (mat.overflowX !== 'auto') fail('mat-scrolls', `${where}: the ${mat.kind} mat has overflow-x ${mat.overflowX}, want auto`);
      }
      if (Math.abs(small.drawingWidth - 120) > 1) fail('small-picture-kept', `${where}: the small picture is ${small.drawingWidth}px wide, want the 120px it was given`);
      if (small.scrollWidth > small.clientWidth + 1) fail('small-picture-kept', `${where}: the small picture's mat scrolls`);
      const wideMats = seen.mats.filter((mat) => mat.kind !== 'small');
      for (const mat of wideMats) {
        const room = mat.clientWidth - 16;
        if (room < FLOOR) {
          if (mat.drawingWidth < FLOOR - 1) fail('mat-scrolls', `${where}: the ${mat.kind} drawing is ${mat.drawingWidth}px wide in ${room}px of room, want it held at ${FLOOR}px`);
          if (mat.scrollWidth <= mat.clientWidth + 1) fail('mat-scrolls', `${where}: the ${mat.kind} mat does not scroll though its drawing is wider than it`);
        } else if (mat.scrollWidth > mat.clientWidth + 1) {
          fail('mat-scrolls', `${where}: the ${mat.kind} mat scrolls though its drawing fits in ${room}px`);
        }
      }

      // mat-focus
      const scrolling = new Set(wideMats.filter((mat) => mat.clientWidth - 16 < FLOOR).map((mat) => mat.kind));
      for (const mat of seen.mats) {
        const stop = mat.tabindex === '0';
        if (stop !== scrolling.has(mat.kind)) {
          fail('mat-focus', `${where}: the ${mat.kind} mat is ${stop ? '' : 'not '}a tab stop and ${scrolling.has(mat.kind) ? '' : 'not '}scrolling; a stop belongs to a mat exactly while it scrolls`);
        }
        if (stop && (mat.role !== 'group' || !mat.name || mat.label !== mat.name)) {
          fail('mat-focus', `${where}: the ${mat.kind} mat is a tab stop named ${JSON.stringify(mat.label)} with role ${JSON.stringify(mat.role)}, want a group named ${JSON.stringify(mat.name)}`);
        }
      }
      if (scrolling.size) {
        // Reached the way a reader without a pointer reaches it, and moved the
        // way they move it.
        await page.evaluate(() => {
          for (const [index, mat] of document.querySelectorAll('.y-prose .mat').entries()) mat.setAttribute('data-probe-mat', String(index));
        });
        const reached = new Set();
        for (let press = 0; press < 120 && reached.size < scrolling.size; press++) {
          await page.keyboard.press('Tab');
          const index = await page.evaluate(() => document.activeElement?.getAttribute('data-probe-mat') ?? null);
          if (index === null) continue;
          reached.add(index);
          const before = await page.evaluate((i) => document.querySelectorAll('.y-prose .mat')[Number(i)].scrollLeft, index);
          await page.keyboard.press('ArrowRight');
          await page.waitForTimeout(150);
          const after = await page.evaluate((i) => document.querySelectorAll('.y-prose .mat')[Number(i)].scrollLeft, index);
          if (after <= before) fail('mat-focus', `${where}: a key pressed on the mat at position ${index} did not scroll it (${before} to ${after})`);
        }
        if (reached.size < scrolling.size) fail('mat-focus', `${where}: Tab reached ${reached.size} of the ${scrolling.size} mats that scroll`);
      }
    } finally {
      await context.close();
    }
  }
  if (MUTATE && !applied) throw new NotApplied(`NOT-APPLIED figure-mat: ${MUTATE} never applied`);
  console.log('figure-mat: ok');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE && MUTATIONS[MUTATE].site === error.site && applied) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    console.error(error instanceof ProbeBroken ? error.message : `BROKEN figure-mat: ${error.stack || error}`);
    process.exitCode = 1;
  }
} finally {
  if (browser) await browser.close();
  if (approvalsFile) await rm(approvalsFile, { force: true });
}
