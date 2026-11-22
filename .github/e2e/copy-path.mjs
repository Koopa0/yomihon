// Behavior lock: the header's Copy path hands the note to the reader's own
// editor from wherever they are reading. Pressed in the middle of a section, it
// writes the note's full path, a colon and the line of the file that section's
// heading opens on, and the reader stays where they were: the window does not
// scroll, nothing on the page moves, and focus goes nowhere it would scroll to.
// Above the first section it writes the path alone. Where the browser refuses
// the clipboard, the path is shown, selected, with the sentence asking for it
// to be copied by hand, and the reader still has not moved.
//
// The line is counted by hand from the fixture note. The path is the one the
// page carries, which the Go tests hold to the file.
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/Back%20restore.md';
const NOTE = '/Notes/Back restore.md';
// "## The Third section" is line 57 of the file; the fourth section follows it.
const SECTION = 'the-third-section';
const NEXT = 'the-fourth-section';
const LINE = 57;
// The control is drawn where the left column is not a drawer, 1024px and up.
const WIDTHS = [1280, 1024];
// Subpixel layout lets a box read a fraction of a pixel differently without
// anything having moved.
const TOLERANCE = 0.5;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL copy-path: ${message}`);
};

const open = async (context, width) => {
  const page = await context.newPage();
  await page.setViewportSize({ width, height: 800 });
  const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  if (response?.status() !== 200) throw new Error(`BROKEN copy-path: ${PAGE} answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
  if (await page.locator('[data-path-control]').count() !== 1) fail(`${PAGE} draws no single Copy path control`);
  return page;
};

// Puts the top of the window halfway into the section, once the page has
// stopped changing height under the diagrams it draws late.
const scrollIntoSection = (page) => page.evaluate(async ([section, next]) => {
  const frame = () => new Promise((resolve) => requestAnimationFrame(() => resolve()));
  const top = (id) => document.getElementById(id).getBoundingClientRect().top + window.scrollY;
  let height = -1;
  while (height !== document.documentElement.scrollHeight) {
    height = document.documentElement.scrollHeight;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  window.scrollTo(0, Math.round((top(section) + top(next)) / 2));
  await frame();
  await frame();
  return { y: window.scrollY, from: top(section), to: top(next) };
}, [SECTION, NEXT]);

// What a reader would see move: where the window is, every box of the header
// row, every block of the text that is in view, and the height of the page.
const reading = (page) => page.evaluate(() => {
  const box = (element) => {
    const r = element.getBoundingClientRect();
    return [r.left, r.top, r.width, r.height].map((n) => Math.round(n * 10) / 10);
  };
  const header = [...document.querySelectorAll('.y-header > *, .y-header button, .y-header a')]
    .filter((element) => !element.matches('.y-reply'))
    .map(box);
  const text = [...document.querySelectorAll('.y-prose > *')]
    .filter((element) => {
      const r = element.getBoundingClientRect();
      return r.bottom > 0 && r.top < innerHeight;
    })
    .map(box);
  return { y: window.scrollY, height: document.documentElement.scrollHeight, header, text };
});

const watchShifts = (page) => page.evaluate(() => {
  window.copyPathShifts = [];
  new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) window.copyPathShifts.push(entry.value);
  }).observe({ type: 'layout-shift' });
});

const settled = (page) => page.evaluate(() => new Promise((resolve) => {
  requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 300)));
}));

const sameReading = (where, before, after, shifts) => {
  if (Math.abs(before.y - after.y) > TOLERANCE) fail(`${where}: the window moved from ${before.y} to ${after.y}`);
  if (before.height !== after.height) fail(`${where}: the page changed height from ${before.height} to ${after.height}`);
  for (const part of ['header', 'text']) {
    const moved = before[part].some((one, i) => !after[part][i] || one.some((n, j) => Math.abs(n - after[part][i][j]) > TOLERANCE));
    if (moved || before[part].length !== after[part].length) {
      fail(`${where}: the ${part} moved: ${JSON.stringify(before[part])} then ${JSON.stringify(after[part])}`);
    }
  }
  const total = shifts.reduce((sum, value) => sum + value, 0);
  if (total > 0) fail(`${where}: the browser recorded layout shifts ${JSON.stringify(shifts)}`);
};

// Focus may stay on the control the reader pressed, which is in the row that
// does not scroll; anywhere in the text would be a place the browser scrolls to.
const focusStaysInTheRow = async (page, where) => {
  const at = await page.evaluate(() => {
    const active = document.activeElement;
    return { body: active === document.body, inRow: Boolean(active?.closest('.y-header')), tag: active?.tagName };
  });
  if (!at.body && !at.inRow) fail(`${where}: focus moved to a ${at.tag} outside the header row`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  for (const width of WIDTHS) {
    const where = `${width}px`;
    const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
    const page = await open(context, width);
    const { path, copied } = await page.locator('[data-path-control]').evaluate((control) => ({
      path: control.dataset.path,
      copied: control.dataset.pathCopied,
    }));
    if (!path || !(path.startsWith('/') || /^[A-Za-z]:\\/.test(path)) || !path.replaceAll('\\', '/').endsWith(NOTE)) {
      fail(`${where}: the control carries ${JSON.stringify(path)}, want the full path of ${NOTE}`);
    }

    const placed = await scrollIntoSection(page);
    if (placed.y <= placed.from || placed.y >= placed.to) fail(`${where}: could not put the window inside the section: ${JSON.stringify(placed)}`);
    const before = await reading(page);
    await watchShifts(page);
    await page.locator('[data-path-button]').click();
    await page.locator('[data-path-said]').filter({ hasText: copied }).waitFor({ timeout: 3000 }).catch(() => {
      fail(`${where}: the press was not answered with ${JSON.stringify(copied)}`);
    });
    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    if (clipboard !== `${path}:${LINE}`) fail(`${where}: the clipboard holds ${JSON.stringify(clipboard)}, want ${JSON.stringify(`${path}:${LINE}`)}`);
    await settled(page);
    sameReading(where, before, await reading(page), await page.evaluate(() => window.copyPathShifts));
    await focusStaysInTheRow(page, where);
    console.log(`copy-path ${where}: scrollY ${before.y} before the press, ${(await reading(page)).y} after; copied ${clipboard}`);

    // Above the first section there is no section, so no line.
    await page.evaluate(async () => {
      window.scrollTo(0, 0);
      await navigator.clipboard.writeText('');
    });
    await settled(page);
    await page.locator('[data-path-button]').click();
    await page.waitForFunction(async () => (await navigator.clipboard.readText()) !== '', null, { timeout: 3000 }).catch(() => {
      fail(`${where}: a press above the first section wrote nothing`);
    });
    const top = await page.evaluate(() => navigator.clipboard.readText());
    if (top !== path) fail(`${where}: above the first section the clipboard holds ${JSON.stringify(top)}, want the path alone`);
    await context.close();

    // The browser refuses: the path is shown to be copied by hand.
    const refused = await browser.newContext();
    await refused.addInitScript(() => {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText: () => Promise.reject(new DOMException('refused', 'NotAllowedError')) },
      });
    });
    const shown = await open(refused, width);
    const failed = await shown.locator('[data-path-control]').evaluate((control) => control.dataset.pathFailed);
    await scrollIntoSection(shown);
    const still = await reading(shown);
    await watchShifts(shown);
    await shown.locator('[data-path-button]').click();
    await shown.locator('[data-path-said]').filter({ hasText: failed }).waitFor({ timeout: 3000 }).catch(() => {
      fail(`${where}: a refused copy did not say ${JSON.stringify(failed)}`);
    });
    const byHand = await shown.evaluate(() => {
      const said = document.querySelector('[data-path-said]');
      const value = said.querySelector('.y-headerpath__path');
      const r = said.getBoundingClientRect();
      return {
        value: value?.textContent,
        selected: getSelection()?.toString(),
        pointer: getComputedStyle(said).pointerEvents,
        select: value ? getComputedStyle(value).userSelect : '',
        inside: r.left >= 0 && r.top >= 0 && r.right <= innerWidth && r.bottom <= innerHeight,
      };
    });
    const want = `${path}:${LINE}`;
    if (byHand.value !== want || byHand.selected !== want) {
      fail(`${where}: a refused copy shows ${JSON.stringify(byHand.value)} with ${JSON.stringify(byHand.selected)} selected, want ${JSON.stringify(want)} for both`);
    }
    if (byHand.pointer === 'none' || byHand.select === 'none' || !byHand.inside) {
      fail(`${where}: the path shown for copying by hand cannot be selected within the window: ${JSON.stringify(byHand)}`);
    }
    await settled(shown);
    sameReading(`${where}, refused`, still, await reading(shown), await shown.evaluate(() => window.copyPathShifts));
    await focusStaysInTheRow(shown, `${where}, refused`);
    console.log(`copy-path ${where}, refused: scrollY ${still.y} before the press, ${(await reading(shown)).y} after`);
    await shown.keyboard.press('Escape');
    if (await shown.locator('[data-path-said]').textContent() !== '') fail(`${where}: Escape left the path shown`);
    await refused.close();
  }
  console.log('PASS copy-path: in the middle of a section the press copies the full path and the line of its heading, above the first section the path alone, and where the browser refuses it shows the path selected with the sentence asking for it to be copied by hand; at 1280px and 1024px nothing scrolls, moves or takes focus out of the row');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
