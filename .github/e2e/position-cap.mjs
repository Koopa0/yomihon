// Behavior lock: the places kept are capped, and the cap drops the least
// recently read first.
//
// The browser's storage is the reader's, and a list that grows with every note
// they ever open is one that grows for years. The cap is found by what it does
// rather than by what it is called: the storage is filled with far more places
// than any cap would hold, the oldest first, and one more place is kept the way a
// reader keeps one. What is left has to be the newest of what was there, in
// order, with the new place last, and a place read again has to move to the
// end, so the notes dropped are the ones read longest ago and not the ones
// written first.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, KEY, openNote, storedPlaces, waitForPlace } from './support/position.mjs';

const PLENTY = 1000;
const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const OTHER = '/notes/Notes/Glass%20Tide.md';
const OTHER_PATH = 'Notes/Glass Tide.md';

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-cap: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-cap: ${message}`);
};

const keepSomewhere = async (page, path, changedFrom = null) => {
  await page.evaluate(() => {
    const scroller = document.scrollingElement ?? document.documentElement;
    window.scrollTo(0, Math.max(0, scroller.scrollHeight - innerHeight) / 2);
  });
  await frames(page);
  const place = await waitForPlace(page, path, { changedFrom });
  if (place === null) fail(`nothing was kept for ${path} after the reader stopped`);
  return place;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();

  await openNote(page, BASE, NOTE);
  const first = await keepSomewhere(page, PATH);
  const vault = first.vault;
  if (typeof vault !== 'string' || vault === '') broken(`the place kept carries no vault, it is ${JSON.stringify(first)}`);

  const seeded = Array.from({ length: PLENTY }, (_, i) => ({ vault, path: `Seeded/${String(i).padStart(4, '0')}.md`, anchor: '', offset: 100 + i }));
  await page.evaluate(({ key, places }) => localStorage.setItem(key, JSON.stringify(places)), { key: KEY, places: [...seeded, first] });

  await openNote(page, BASE, OTHER);
  const second = await keepSomewhere(page, OTHER_PATH);
  const kept = await storedPlaces(page);
  console.log(`position-cap: ${PLENTY + 1} places were there and ${kept.length} are left after one more was kept`);
  if (kept.length >= PLENTY + 1) fail(`${PLENTY + 1} places were stored and keeping one more left ${kept.length}: nothing is dropped`);
  if (kept.length < 20) fail(`keeping one more left ${kept.length} places, too few for a reader's recent notes to be among them`);
  if (JSON.stringify(kept.at(-1)) !== JSON.stringify(second)) fail(`the place kept last is ${JSON.stringify(kept.at(-1))}, want the one just kept, ${JSON.stringify(second)}`);
  if (JSON.stringify(kept.at(-2)) !== JSON.stringify(first)) fail(`the place before it is ${JSON.stringify(kept.at(-2))}, want the one read before, ${JSON.stringify(first)}`);
  const left = kept.slice(0, -2);
  const newest = seeded.slice(seeded.length - left.length);
  if (JSON.stringify(left) !== JSON.stringify(newest)) {
    fail(`what was dropped is not the oldest: the list starts at ${JSON.stringify(left[0])} and runs ${left.length} places, but the newest ${left.length} seeded places start at ${JSON.stringify(newest[0])}`);
  }

  // Read again, the first note is the most recent of all.
  await openNote(page, BASE, NOTE);
  await page.evaluate(() => {
    const heading = document.getElementById('the-fifth-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 40));
  });
  await frames(page);
  const again = await waitForPlace(page, PATH, { changedFrom: JSON.stringify(first) });
  if (again === null) fail('a note read again kept no new place');
  const reordered = await storedPlaces(page);
  if (reordered.filter((place) => place.path === PATH).length !== 1) fail(`a note read again is kept ${reordered.filter((place) => place.path === PATH).length} times, want once`);
  if (reordered.at(-1).path !== PATH) fail(`the note read again is at ${reordered.findIndex((place) => place.path === PATH)} of ${reordered.length}, want last: the list is in the order places were first written, not read`);
  await context.close();
  console.log('PASS position-cap: the places kept are capped and the cap drops the one read longest ago');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
