// Behavior lock: a place kept against a section the note no longer has opens
// the note at its top.
//
// A place is a section and a distance below it, so that a note changed above it
// still opens in the right section. A note changed in the section itself, with
// the heading gone or renamed, has nothing to measure the distance from, and a
// depth guessed from it would drop the reader at an unrelated place. The note is
// opened once as it is, which proves the place is kept and put back, and once
// served with that heading's id changed.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, openNote, scrollTop, waitForPlace } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const SLACK = 2;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-gone: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-gone: ${message}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  await openNote(page, BASE, NOTE);
  await page.evaluate(() => {
    const heading = document.getElementById('the-third-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 250));
  });
  await frames(page);
  const kept = await scrollTop(page);
  const stored = await waitForPlace(page, PATH);
  if (stored === null) fail(`nothing was kept in the browser's storage after the reader stopped at ${kept}`);
  if (stored.anchor !== 'the-third-section') broken(`the place was kept against ${JSON.stringify(stored.anchor)}, want the-third-section, which is the section this probe removes`);

  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  await openNote(page, BASE, NOTE);
  const control = await scrollTop(page);
  console.log(`position-gone: with the section in the note it opened at ${control}, the place kept is ${kept}`);
  if (Math.abs(control - kept) > SLACK) fail(`with its section in the note it opened at ${control}, not at the place kept, ${kept}: the place is not put back, so a missing section proves nothing`);

  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  await page.route(`**${NOTE}`, async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    const edited = body.replace('id="the-third-section"', 'id="a-section-with-another-name"');
    if (edited === body) broken('the heading was not renamed, so the note was served unchanged');
    await route.fulfill({ response, body: edited });
  });
  await openNote(page, BASE, NOTE);
  const landed = await scrollTop(page);
  const column = await page.evaluate(() => getComputedStyle(document.querySelector('.y-main')).visibility);
  console.log(`position-gone: with the section gone it opened at ${landed}, the column is ${column}`);
  if (landed > SLACK) fail(`the section the place was kept against is not in the note and it opened at ${landed}, want the top`);
  if (column !== 'visible') fail(`the section is gone and the column is ${column}: a note that could not be placed is left hidden`);
  await context.close();
  console.log('PASS position-gone: a place kept against a section the note no longer has opens at the top');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
