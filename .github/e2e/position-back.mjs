// Behavior lock: Back and Forward keep the browser's own scroll restoration.
//
// A note opened from a link opens where the reader last stopped in it, but a note
// the browser shows again from its history is put back by the browser, to the
// place the reader left it at that visit, and that is not always the place most
// recently kept: the same note may have been read in another tab since. The
// browser's restoration is not repeated or overridden by the page.
//
// The history entry is left at one place and the place kept is then made
// another, so the two answers differ. The browser runs with its back/forward
// cache off, which is what makes Back load the note again and run its script.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, KEY, openNote, scrollTop, waitForPlace } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const AWAY = '/notes/Notes/Reading%20flow%20detour.md';
const SLACK = 2;
const APART = 400;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-back: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-back: ${message}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true, args: ['--disable-features=BackForwardCache'] });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();

  await openNote(page, BASE, NOTE);
  await page.evaluate(() => {
    const heading = document.getElementById('the-second-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 100));
  });
  await frames(page);
  const first = await scrollTop(page);
  const stored = await waitForPlace(page, PATH);
  if (stored === null) fail(`nothing was kept in the browser's storage after the reader stopped at ${first}`);

  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  await openNote(page, BASE, NOTE);
  const control = await scrollTop(page);
  console.log(`position-back: opened from a link the note is at ${control}, the place kept is ${first}`);
  if (Math.abs(control - first) > SLACK) fail(`opened from a link the note is at ${control}, not at the place kept, ${first}: the place is not put back, so Back keeping the browser's own proves nothing`);

  await page.evaluate(() => {
    const heading = document.getElementById('the-fifth-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 100));
  });
  await frames(page);
  const left = await scrollTop(page);
  if (Math.abs(left - first) < APART) broken(`the history entry is left at ${left} and the place kept is ${first}, less than ${APART}px apart`);
  const rewritten = await waitForPlace(page, PATH, { changedFrom: JSON.stringify(stored) });
  if (rewritten === null) fail('the second stop was not kept');
  // Another tab read the note and stopped higher up: the place kept is no longer
  // where this history entry was left.
  await page.evaluate(({ key, place }) => {
    const places = JSON.parse(localStorage.getItem(key));
    localStorage.setItem(key, JSON.stringify(places.map((one) => (one.path === place.path ? place : one))));
  }, { key: KEY, place: stored });

  await page.goto(BASE + AWAY, { waitUntil: 'load' });
  await page.goBack({ waitUntil: 'load' });
  await frames(page, 6);
  await new Promise((resolve) => {
    setTimeout(resolve, 600);
  });
  const back = await scrollTop(page);
  console.log(`position-back: Back to the note left at ${left}, with ${first} kept: the note is at ${back}`);
  if (Math.abs(back - left) > SLACK) fail(`Back returned the note to ${back}, not to ${left} where the history entry was left: the place kept (${first}) was put over the browser's restoration`);
  await context.close();
  console.log('PASS position-back: Back returns the note to where the browser left it, not to the place kept');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
