// Behavior lock for the parts of search that exist only in a browser: the
// results page answers a new query in place without a page change and keeps
// the address true, a search the reader made is offered back next time, the
// header's field on a wider window answers under itself as the reader types —
// reached from the keyboard, followed with the arrows and Enter, put away by
// Escape or a press outside it, never a dialog over the page (D94) — and the
// phone's full-screen surface opens, answers and closes. The handler's own
// answers are covered by Go tests; this reads what a reader would see.
//
// Env: YOMIHON_BASE.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const failures = [];
const check = (ok, message) => {
  if (!ok) failures.push(message);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // The page's own field is the phone's; a wider window asks through the header.
  const wide = await (await browser.newContext({ viewport: { width: 390, height: 844 } })).newPage();
  await wide.goto(`${BASE}/search?q=alpha`, { waitUntil: 'load' });
  await wide.waitForSelector('html[data-js]');
  const before = await wide.locator('.y-found__item').evaluateAll((rows) => rows.map((row) => row.getAttribute('href')));
  check(before.length > 0, 'a search for alpha drew no results');
  await wide.evaluate(() => {
    window.stayedOnPage = true;
  });
  const entries = await wide.evaluate(() => history.length);
  await wide.locator('.y-searchpage__form [data-live-search-input]').fill('beta');
  await wide.waitForFunction(() => new URL(location.href).searchParams.get('q') === 'beta');
  await wide.waitForFunction((was) => {
    const now = [...document.querySelectorAll('.y-found__item')].map((row) => row.getAttribute('href'));
    return now.length > 0 && now.join() !== was.join();
  }, before);
  check(await wide.evaluate(() => window.stayedOnPage === true), 'a new query changed the page instead of its results');
  check((await wide.evaluate(() => history.length)) === entries, 'a new query added a history entry for every answer');

  await wide.locator('.y-searchpage__form [data-live-search-input]').press('Enter');
  await wide.waitForURL(/q=beta/);
  await wide.goto(`${BASE}/search`, { waitUntil: 'load' });
  await wide.waitForSelector('html[data-js]');
  const offered = await wide.locator('[data-recent-list] a').allTextContents();
  check(offered.some((text) => text.includes('beta')), `a search the reader made is not offered back: ${JSON.stringify(offered)}`);

  // An answer to an older query that arrives after a newer one's must not
  // replace its rows: the older is held back until the newer has been drawn.
  const racing = await (await browser.newContext({ viewport: { width: 390, height: 844 } })).newPage();
  await racing.route('**/search/results?*', async (route) => {
    if (new URL(route.request().url()).searchParams.get('q') === 'alpha') await new Promise((resolve) => setTimeout(resolve, 1200));
    await route.continue().catch(() => {});
  });
  await racing.goto(`${BASE}/search`, { waitUntil: 'load' });
  await racing.waitForSelector('html[data-js]');
  const field = racing.locator('.y-searchpage__form [data-live-search-input]');
  await field.fill('alpha');
  await racing.waitForTimeout(400);
  await field.fill('beta');
  await racing.waitForFunction(() => document.querySelector('.y-found__item') !== null);
  await racing.waitForTimeout(1800);
  const wanted = await racing.evaluate(async () => {
    const answer = await fetch('/search/results?q=beta');
    const doc = new DOMParser().parseFromString(await answer.text(), 'text/html');
    return [...doc.querySelectorAll('.y-found__item')].map((row) => row.getAttribute('href'));
  });
  const drawn = await racing.locator('.y-found__item').evaluateAll((rows) => rows.map((row) => row.getAttribute('href')));
  check(drawn.length > 0 && drawn.join() === wanted.join(), `an older query's answer replaced the newer one's rows: drew ${drawn.length}, want the ${wanted.length} for beta`);
  check(new URL(racing.url()).searchParams.get('q') === 'beta', `the address names ${new URL(racing.url()).searchParams.get('q')} after the newer query was drawn`);

  const keys = await (await browser.newContext({ viewport: { width: 1280, height: 900 } })).newPage();
  await keys.goto(`${BASE}/`, { waitUntil: 'load' });
  await keys.waitForSelector('html[data-js]');
  const headerField = '.y-searchfield [data-live-search-input]';
  const panel = keys.locator('.y-searchfield [data-live-search-panel]');
  const inField = () => keys.evaluate((selector) => document.activeElement?.matches(selector) === true, headerField);
  const anyDialogOpen = () => keys.evaluate(() => document.querySelector('dialog[open]') !== null);
  await keys.keyboard.press('ControlOrMeta+k');
  check(await inField(), 'on a wide window the keyboard chord did not put the reader in the header\'s search field');
  check(!(await anyDialogOpen()), 'on a wide window the keyboard chord opened a dialog over the page, want the header\'s field');
  await keys.keyboard.type('alpha', { delay: 30 });
  const rows = panel.locator('[data-live-search-row]');
  await rows.nth(1).waitFor();
  // The answer drops under the field, inside the window, and is not a dialog.
  const placed = await keys.evaluate((selector) => {
    const form = document.querySelector(selector).closest('form').getBoundingClientRect();
    const list = document.querySelector('.y-searchfield [data-live-search-panel]').getBoundingClientRect();
    return { top: list.top, formBottom: form.bottom, left: list.left, right: list.right, width: innerWidth };
  }, headerField);
  check(placed.top >= placed.formBottom - 0.5 && placed.left >= 0 && placed.right <= placed.width,
    `the field's answer is drawn at ${JSON.stringify(placed)}, want it under the field and inside the window`);
  check(!(await anyDialogOpen()), 'typing in the header\'s field opened a dialog over the page');
  // Escape puts the list away and leaves the reader in the field, typing.
  await keys.keyboard.press('Escape');
  check(await panel.isHidden(), 'Escape left the field\'s answer open');
  check(await inField(), 'the first Escape took the reader out of the field, want only the list put away');
  // A press anywhere outside the field puts the list away too.
  await keys.keyboard.press('Backspace');
  await rows.first().waitFor();
  const spacer = await keys.locator('.y-header__spacer').boundingBox();
  check(spacer !== null && spacer.width > 4, `the header has no room beside the field to press: ${JSON.stringify(spacer)}`);
  if (spacer) await keys.mouse.click(spacer.x + spacer.width / 2, spacer.y + spacer.height / 2);
  check(await panel.isHidden(), 'a press outside the field left its answer open');
  // Back by the keyboard, the arrows choose a row and Enter opens it.
  // The field still holds what was typed before and answers it again on
  // focus, so the rows are read only once the answer to this query is drawn.
  const answered = keys.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === '/search/results' && url.searchParams.get('q') === 'alpha';
  });
  await keys.keyboard.press('ControlOrMeta+k');
  check(await inField(), 'the chord did not bring the reader back to the field');
  await keys.keyboard.type('alpha', { delay: 30 });
  await answered;
  await keys.waitForFunction(() => document.querySelector('.y-searchfield [data-live-search-panel]')?.getAttribute('aria-busy') === 'false');
  await rows.nth(1).waitFor();
  const second = new URL(await rows.nth(1).getAttribute('href'), BASE).pathname;
  await keys.keyboard.press('ArrowDown');
  check(await inField(), 'a press of ArrowDown took focus out of the field');
  await keys.keyboard.press('Enter');
  await keys.waitForURL((url) => url.pathname !== '/');
  check(new URL(keys.url()).pathname === second, `Enter opened ${new URL(keys.url()).pathname}, want the row ArrowDown chose, ${second}`);

  const phone = await (await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true })).newPage();
  await phone.goto(`${BASE}/`, { waitUntil: 'load' });
  await phone.waitForSelector('html[data-js]');
  await phone.locator('[data-search-open]').click();
  const surface = phone.locator('dialog[data-search][open]');
  await surface.waitFor();
  const box = await surface.boundingBox();
  check(box !== null && Math.round(box.width) === 390 && Math.round(box.height) === 844, `the phone surface is ${JSON.stringify(box)}, want the whole screen`);
  await surface.locator('[data-live-search-input]').pressSequentially('alpha', { delay: 30 });
  await surface.locator('[data-live-search-row]').first().waitFor();
  check((await surface.locator('[data-live-search-row]').count()) > 0, 'the phone surface drew no rows for alpha');
  const overflow = await phone.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  check(overflow <= 0, `the page scrolls ${overflow}px sideways with the surface open`);
  await phone.keyboard.press('Escape');
  await phone.waitForFunction(() => !document.querySelector('dialog[data-search]')?.open);
} finally {
  await browser.close();
}

if (failures.length > 0) {
  console.error(`FAIL search-live: ${failures.join('; ')}`);
  process.exit(1);
}
console.log('PASS search-live: the results page answers in place and keeps its address, recent searches come back, on a wide window the keyboard reaches the header\'s field whose answer drops under it, is put away by Escape or a press outside, and whose rows ArrowDown and Enter reach, and the phone surface opens, answers and closes');
