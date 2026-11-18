// Behavior lock for the parts of search that exist only in a browser: the
// results page answers a new query in place without a page change and keeps
// the address true, a search the reader made is offered back next time, and the
// phone's full-screen surface opens, answers and closes. The handler's own
// answers are covered by Go tests; this reads what a reader would see.
//
// Env: YOMIHON_BASE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
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
  await keys.keyboard.press('Control+k');
  const palette = keys.locator('dialog[data-search][open]');
  await palette.waitFor();
  const room = await palette.boundingBox();
  check(room !== null && room.width <= 600, `the keyboard opened a ${room?.width}px dialog on a wide window, want a box of at most 600px`);
  await keys.keyboard.type('alpha', { delay: 30 });
  const rows = palette.locator('[data-live-search-row]');
  await rows.nth(1).waitFor();
  const second = new URL(await rows.nth(1).getAttribute('href'), BASE).pathname;
  await keys.keyboard.press('ArrowDown');
  check(await keys.evaluate(() => document.activeElement?.matches('[data-live-search-input]') === true), 'a press of ArrowDown took focus out of the field');
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
console.log('PASS search-live: the results page answers in place and keeps its address, the keyboard opens a palette whose rows ArrowDown and Enter reach, recent searches come back, and the phone surface opens, answers and closes');
