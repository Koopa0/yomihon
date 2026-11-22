// Behavior lock: a place is kept when the reader leaves, not only when they have
// been still for a moment.
//
// The page writes a place when the window has stopped moving, which a reader who
// scrolls and closes the tab, or follows a link, does not wait for. So the page
// also writes when it is left and when it is hidden, which is all a phone gives a
// reader who switches to another app. Both are driven here inside the moment
// the debounce would still be waiting: the place has to be in the browser's
// storage before it could have been written for having been still.
//
// Leaving is a real navigation. Hiding cannot be made to happen to a headless
// page, so the page is told it is hidden the way the browser tells it, with the
// state set and the event fired, and the storage is read in the same breath.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, openNote, scrollTop, storedPlaces } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const SLACK = 2;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-leave: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-leave: ${message}`);
};

const readTo = async (page, section, below) => {
  await page.evaluate(({ id, px }) => {
    const heading = document.getElementById(id);
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + px));
  }, { id: section, px: below });
  // The scroll event has been delivered, so the page knows it moved, and the
  // debounce has barely started.
  await frames(page);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();

  await openNote(page, BASE, NOTE);
  await readTo(page, 'the-third-section', 220);
  const left = await scrollTop(page);
  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  const written = (await storedPlaces(page))?.find((place) => place.path === PATH) ?? null;
  console.log(`position-leave: left at ${left} at once, the storage holds ${JSON.stringify(written)}`);
  if (written === null) fail(`the reader scrolled to ${left} and followed a link at once, and nothing was kept for ${PATH}: a place is written only for having been still`);
  await openNote(page, BASE, NOTE);
  const opened = await scrollTop(page);
  if (Math.abs(opened - left) > SLACK) fail(`the place written on leaving was ${left} and the note opened at ${opened}`);

  await readTo(page, 'the-fifth-section', 90);
  const hidden = await scrollTop(page);
  const snapshot = await page.evaluate(
    ({ key, path }) => {
      Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
      document.dispatchEvent(new Event('visibilitychange'));
      return JSON.parse(localStorage.getItem(key) ?? '[]').find((place) => place.path === path) ?? null;
    },
    { key: 'yomihon.positions', path: PATH },
  );
  console.log(`position-leave: hidden at ${hidden}, the storage holds ${JSON.stringify(snapshot)}`);
  if (snapshot === null) broken('no place was kept at all');
  const before = await openAtStoredPlace(page);
  if (Math.abs(before - hidden) > SLACK) {
    fail(`the page was hidden at ${hidden} and the place in the storage is a reading at ${before}: a place is written only for having been still`);
  }
  await context.close();
  console.log('PASS position-leave: a place is kept when the reader leaves and when the page is hidden');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);

// openAtStoredPlace is where a note opens given what the storage holds right
// now, in a page of its own so the place being measured is the one stored.
async function openAtStoredPlace(page) {
  const fresh = await page.context().newPage();
  await openNote(fresh, BASE, NOTE);
  const top = await scrollTop(fresh);
  await fresh.close();
  return top;
}
