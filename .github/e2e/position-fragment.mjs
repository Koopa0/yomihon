// Behavior lock: an address that names a section goes to that section, and the
// place the reader kept in the note is not put back over it.
//
// A link to a heading is the author's or the reader's own statement of where
// they mean to be, and it is the more recent of the two. The note is first
// opened without a fragment, which is what proves a place is kept and is being
// put back in this very browser; then with one, which has to land where the
// same address lands in a browser that kept nothing, and stay there.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, openNote, scrollTop, waitForPlace } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const SECTION = 'the-first-section';
const SLACK = 2;
const APART = 400;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-fragment: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-fragment: ${message}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const bare = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const barePage = await bare.newPage();
  await openNote(barePage, BASE, NOTE, { hash: `#${SECTION}` });
  const wanted = await scrollTop(barePage);
  await bare.close();

  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  await openNote(page, BASE, NOTE);
  await page.evaluate(() => {
    const heading = document.getElementById('the-fifth-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 200));
  });
  await frames(page);
  const kept = await scrollTop(page);
  if (Math.abs(kept - wanted) < APART) broken(`the place at ${kept} and the section at ${wanted} are less than ${APART}px apart, too close to tell which one the note opened at`);
  if ((await waitForPlace(page, PATH)) === null) fail(`nothing was kept in the browser's storage after the reader stopped at ${kept}`);

  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  await openNote(page, BASE, NOTE);
  const control = await scrollTop(page);
  console.log(`position-fragment: without a fragment the note opened at ${control}, the place kept is ${kept}`);
  if (Math.abs(control - kept) > SLACK) fail(`without a fragment the note opened at ${control}, not at the place kept, ${kept}: the place is not put back, so a fragment winning over it proves nothing`);

  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  await openNote(page, BASE, NOTE, { hash: `#${SECTION}` });
  const landed = await scrollTop(page);
  await new Promise((resolve) => {
    setTimeout(resolve, 600);
  });
  const later = await scrollTop(page);
  console.log(`position-fragment: with #${SECTION} the note opened at ${landed}, and was at ${later} after 600ms; the section is at ${wanted}`);
  if (Math.abs(landed - wanted) > SLACK) fail(`the address names ${SECTION} and the note opened at ${landed}, where the same address lands at ${wanted} in a browser that kept nothing`);
  if (Math.abs(later - wanted) > SLACK) fail(`the note opened at the section and then moved to ${later}: the place kept at ${kept} was put back over the fragment`);
  await context.close();
  console.log('PASS position-fragment: an address that names a section opens there and is not moved to the place kept');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
