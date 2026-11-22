// Behavior lock: with the setting off, nothing is written and what was kept is
// gone.
//
// Four things are pinned. Turning the setting off on the page that names it
// clears every place the browser kept, at once. A note read afterwards writes
// nothing, not on stopping and not on leaving, and opens at its top. A page
// drawn while the setting is off clears what is kept as well, which is how a
// choice made without the page's script, or in another tab, empties the browser
// when it is next seen. And turning the setting on again keeps places again, so
// the silence was the setting and not a broken page.
//
// Every step starts from a place that was really kept, so a probe that finds
// the storage empty cannot be finding it empty because nothing was ever written,
// and the silence is counted as writes made, not read off the storage afterwards.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, KEY, openNote, scrollTop, storedPlaces, waitForPlace } from './support/position.mjs';

const NOTE = '/notes/Notes/Back%20restore.md';
const PATH = 'Notes/Back restore.md';
const SETTINGS = '/preferences';
const FIELD = '[data-pref-field="position"]';

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-off: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-off: ${message}`);
};

const readTo = async (page, below) => {
  await page.evaluate((px) => {
    const heading = document.getElementById('the-third-section');
    window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + px));
  }, below);
  await frames(page);
};

const choose = async (page, value) => {
  await page.goto(BASE + SETTINGS, { waitUntil: 'load' });
  const option = page.locator(`${FIELD} input[type="radio"][value="${value}"]`);
  if ((await option.count()) !== 1) fail(`the settings page offers ${await option.count()} options named ${value} for the position, want 1`);
  await page.locator(`${FIELD} label:has(input[value="${value}"])`).click();
  await page.waitForFunction((want) => document.cookie.split(';').some((part) => part.trim() === `yomihon_position=${want}`), value, { timeout: 3000 });
};

// Every write to the key is counted from inside the page, as it is made. What
// the storage holds afterwards is not enough: a page drawn with the setting off
// empties it, so a write the page should not have made would be gone by the time
// the next page was read.
let writes = 0;
const countWrites = () => {
  const set = Storage.prototype.setItem;
  Storage.prototype.setItem = function setItem(key, value) {
    if (key === 'yomihon.positions') window.__positionWrite();
    return set.call(this, key, value);
  };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  await context.addInitScript(countWrites);
  const page = await context.newPage();
  await page.exposeFunction('__positionWrite', () => {
    writes += 1;
  });

  await openNote(page, BASE, NOTE);
  await readTo(page, 250);
  if ((await waitForPlace(page, PATH)) === null) fail('nothing was kept in the browser\'s storage after the reader stopped, so there is nothing for the setting to clear');

  await choose(page, 'off');
  const cleared = await storedPlaces(page);
  console.log(`position-off: after turning it off the storage holds ${JSON.stringify(cleared)}`);
  if (cleared !== null) fail(`turning the setting off left ${JSON.stringify(cleared)} in the browser's storage under ${KEY}, want nothing`);

  const before = writes;
  await openNote(page, BASE, NOTE);
  const opened = await scrollTop(page);
  if (opened > 2) fail(`with the setting off the note opened at ${opened}, want the top`);
  await readTo(page, 250);
  await new Promise((resolve) => {
    setTimeout(resolve, 1200);
  });
  const held = await storedPlaces(page);
  if (held !== null || writes !== before) fail(`after the reader stopped in a note with the setting off the storage holds ${JSON.stringify(held)} and ${writes - before} writes were made, want nothing written`);
  // Leaving is the other time a place is written, and the page that is left is
  // the only one that could write it.
  await readTo(page, 600);
  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  console.log(`position-off: after reading a note with it off and leaving, ${writes - before} writes were made`);
  if (writes !== before) fail(`reading a note with the setting off and leaving it made ${writes - before} writes to ${KEY}, want none`);

  // A page drawn with the setting off empties what is kept: the choice may have
  // been made where no script ran, or in another tab.
  await page.evaluate((key) => {
    localStorage.setItem(key, JSON.stringify([{ vault: '0'.repeat(32), path: 'Notes/alpha.md', anchor: '', offset: 120 }]));
  }, KEY);
  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  const drawn = await storedPlaces(page);
  console.log(`position-off: after drawing a page with it off over a kept place, the storage holds ${JSON.stringify(drawn)}`);
  if (drawn !== null) fail(`a page drawn with the setting off left ${JSON.stringify(drawn)} under ${KEY}, want nothing`);

  await choose(page, 'on');
  await openNote(page, BASE, NOTE);
  await readTo(page, 250);
  if ((await waitForPlace(page, PATH)) === null || writes === before) broken('with the setting on again nothing was written, so the silence above is not the setting');
  await context.close();
  console.log('PASS position-off: with the setting off nothing is written and what was kept is cleared');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
