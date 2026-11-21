// Behavior lock: a place kept where the reading line and the top of the window
// disagree still comes back where the window was. The bookmark keeps its place
// against the last anchor at or above the top of the window and the distance
// below it, while Copy path asks at the reading line under the header. Asking
// the bookmark at that line alone, with the distance still measured from the
// top of the window, would bring a reader back somewhere else: where a heading
// has just gone under the header, and over the last screen of a note, where the
// line moves down to the bottom of the window. The two change together or not
// at all.
//
// It keeps places of its own on the disposable fixture, so it is one of the
// probes that leave one behind and has to stand among the last (see probes.sh).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

// Subpixel layout and a heading's own rounding let a return differ from the
// place by a few pixels without the place having moved.
const SLACK = 4;

const CASES = [
  {
    name: 'a heading 16px under the header',
    note: '/notes/Notes/Back%20restore.md',
    place: () => {
      const header = document.querySelector('.y-header').getBoundingClientRect().bottom;
      const heading = document.getElementById('the-third-section');
      window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY - header + 16));
      const top = heading.getBoundingClientRect().top;
      return top > 0 && top < header ? '' : `the heading is at ${top}px, not under a header ending at ${header}px`;
    },
  },
  {
    name: '400px from the end, in the last screen',
    note: '/notes/Notes/Glass%20Tide.md',
    place: () => {
      const scroller = document.scrollingElement ?? document.documentElement;
      const range = scroller.scrollHeight - innerHeight;
      if (range < innerHeight + 400) return `the note scrolls ${range}px, too little to stand 400px from its end with a screen above`;
      window.scrollTo(0, range - 400);
      return '';
    },
  },
];

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL bookmark-return: ${message}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  for (const one of CASES) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const response = await page.goto(BASE + one.note, { waitUntil: 'networkidle' });
    if (response?.status() !== 200) throw new Error(`BROKEN bookmark-return: ${one.note} answered ${response?.status()}, want 200`);
    await arrived(page);
    await page.evaluate(async () => {
      await document.fonts.ready;
      let height = -1;
      while (height !== document.documentElement.scrollHeight) {
        height = document.documentElement.scrollHeight;
        await new Promise((resolve) => setTimeout(resolve, 200));
      }
    });
    const issue = await page.evaluate(one.place);
    if (issue) throw new Error(`BROKEN bookmark-return: ${one.name}: ${issue}`);
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    const kept = await page.evaluate(() => Math.round(window.scrollY));
    const posted = page.waitForResponse((answer) => new URL(answer.url()).pathname === '/bookmark' && answer.request().method() === 'POST', { timeout: 4000 });
    await page.locator('[data-mark-control] [data-mark-button]').click();
    const post = await posted;
    if (post.status() !== 204) throw new Error(`BROKEN bookmark-return: keeping the place answered ${post.status()}, want 204`);
    const form = new URLSearchParams(post.request().postData() ?? '');

    await page.goto(`${BASE}/`, { waitUntil: 'load' });
    await arrived(page);
    const link = page.locator('[data-home-continue] [data-continue-link]');
    if (await link.count() !== 1) throw new Error(`BROKEN bookmark-return: the desk offers ${await link.count()} ways back, want 1`);
    await Promise.all([page.waitForURL((url) => url.pathname !== '/'), link.click()]);
    await arrived(page);
    await page.evaluate(() => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 60)));
    }));
    const landed = await page.evaluate(() => Math.round(window.scrollY));
    console.log(`bookmark-return: ${one.name}: kept at ${kept} against ${JSON.stringify(form.get('anchor'))} ${form.get('offset')}px below it, landed at ${landed}`);
    if (Math.abs(landed - kept) > SLACK) {
      fail(`with ${one.name}, the place was kept at ${kept} against ${JSON.stringify(form.get('anchor'))} ${form.get('offset')}px below it and the return landed at ${landed}, ${landed - kept}px away`);
    }
    await context.close();
  }
  console.log('PASS bookmark-return: a place kept with a heading under the header, or in the last screen of a note, comes back where the window was');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
