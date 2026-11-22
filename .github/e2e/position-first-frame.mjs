// Behavior lock for the first frame of a note that opens where the reader
// stopped.
//
// A note is placed by script, and the browser paints the first frame of a
// document without waiting for it, so a note put back after that frame is a
// note the reader sees at its top and then watches move. The page holds its
// column out of sight while the browser's storage names a place to open at, and
// shows it in the task that moves the window, so in every frame the column can
// be seen the window is at the place that was kept.
//
// The measurement is a frame recorder that starts before the document's own
// scripts and samples at every animation frame. The note is opened as it is
// served and with the module that places it arriving after the browser has
// painted: whether the old order showed the top depended on which of the two
// came first, and the second is the one that does not leave it to chance.
//
// What the reader sees is where the section they stopped in is in the window,
// not the number the window is scrolled to. The note drawn here has diagrams and
// code blocks above the place that are drawn after the first layout, and the
// browser keeps the section where it is in the window as they grow, which moves
// the number by hundreds of pixels without moving anything the reader sees. So
// each frame records where the section's heading is in the window.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, openNote, scrollTop, waitForPlace } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const KEEP_AT = [
  { width: 1280, height: 800, section: 'the-fourth-section', below: 300 },
  { width: 375, height: 800, section: 'the-fifth-section', below: 120 },
];
const TRIALS = 3;
const LATE_MODULE_MS = 80;
const CONDITIONS = [
  { name: 'as served', delay: 0 },
  { name: 'with the module late', delay: LATE_MODULE_MS },
];
const SLACK = 2;
// A place this far from the top, so a frame at the top cannot be taken for a
// frame at the place.
const APART = 400;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-first-frame: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-first-frame: ${message}`);
};

const recorder = ({ anchor }) => {
  if (window.top !== window) return;
  window.__frames = [];
  const started = performance.now();
  const sample = () => {
    const column = document.querySelector('.y-main');
    window.__frames.push({
      at: Math.round(performance.now() - started),
      column: Boolean(column),
      visible: column ? getComputedStyle(column).visibility === 'visible' : false,
      held: document.documentElement.dataset.arrival ?? null,
      y: Math.round(window.scrollY),
      heading: (() => {
        const element = document.getElementById(anchor);
        return element ? Math.round(element.getBoundingClientRect().top) : null;
      })(),
    });
    if (performance.now() - started < 1200) requestAnimationFrame(sample);
  };
  requestAnimationFrame(sample);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  console.log(`position-first-frame: Chrome ${browser.version()}`);
  let measured = 0;
  let opened = 0;
  for (const view of KEEP_AT) {
    const context = await browser.newContext({ viewport: { width: view.width, height: view.height }, reducedMotion: 'no-preference' });
    const keeper = await context.newPage();
    await openNote(keeper, BASE, NOTE);
    await keeper.evaluate(({ id, below }) => {
      const heading = document.getElementById(id);
      window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + below));
    }, { id: view.section, below: view.below });
    await frames(keeper);
    const place = await scrollTop(keeper);
    if (place < APART) broken(`at ${view.width}px the place is at ${place}, less than the ${APART}px that tells it from the top`);
    const stored = await waitForPlace(keeper, PATH);
    if (stored === null) fail(`at ${view.width}px nothing was kept in the browser's storage after the reader stopped at ${place}`);
    if (stored.anchor === '') broken(`at ${view.width}px the place was kept against no section, so there is no heading to follow in the window`);
    // Where the heading is in the window once the page has settled: the place.
    const shown = await keeper.evaluate((id) => Math.round(document.getElementById(id).getBoundingClientRect().top), stored.anchor);
    await keeper.close();

    await context.addInitScript(recorder, { anchor: stored.anchor });
    for (const condition of CONDITIONS) {
      for (let trial = 1; trial <= TRIALS; trial += 1) {
        const reader = await context.newPage();
        if (condition.delay > 0) {
          await reader.route('**/static/yomihon.js{,?*}', async (route) => {
            await new Promise((resolve) => {
              setTimeout(resolve, condition.delay);
            });
            await route.continue();
          });
        }
        await reader.goto(BASE + NOTE, { waitUntil: 'domcontentloaded' });
        await reader.waitForFunction(() => (window.__frames ?? []).at(-1)?.at >= 1150, null, { timeout: 8000 });
        const all = (await reader.evaluate(() => window.__frames ?? [])).filter((frame) => frame.column);
        if (all.length < 10) broken(`the document painted ${all.length} frames with its column at ${view.width}px, too few to say anything about them`);
        const held = all.filter((frame) => frame.held !== null).length;
        if (condition.delay > 0 && held < 2) {
          broken(`with the module ${condition.delay}ms late the column was held in only ${held} frames at ${view.width}px, so the first frame was painted before the module ran`);
        }
        const settled = all.at(-1);
        if (Math.abs(settled.heading - shown) > SLACK) {
          broken(`at ${view.width}px the page settled with the heading at ${settled.heading} in the window, not at the ${shown} it was kept at, so there is no place to be painted at`);
        }
        const seen = all.filter((frame) => frame.visible);
        if (seen.length === 0) broken(`the column was never visible in ${all.length} frames at ${view.width}px`);
        const wrong = seen.filter((frame) => frame.heading === null || Math.abs(frame.heading - shown) > SLACK);
        measured += seen.length;
        opened += 1;
        console.log(`position-first-frame: ${view.width}px ${condition.name} trial ${trial}: ${held} frames held, first visible frame ${JSON.stringify(seen[0])}, settled at ${settled.y}, ${wrong.length} of ${seen.length} visible frames with the heading away from ${shown}`);
        if (wrong.length > 0) {
          fail(`at ${view.width}px (${condition.name}) a frame showing the column was painted with the heading at ${wrong[0].heading} in the window, but the place kept has it at ${shown}: the reader saw the note somewhere else first ${JSON.stringify(wrong[0])}`);
        }
        await reader.close();
      }
    }
    await context.close();
  }
  console.log(`PASS position-first-frame: ${measured} visible frames over ${opened} openings, every one at the place kept`);
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
