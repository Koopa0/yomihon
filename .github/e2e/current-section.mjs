// Behavior lock: the section a reader is in is the one their eyes are in. The
// header stays over the top of the text, so the window's own top is under it,
// and the text a reader is reading starts below it. The line asked is the
// reading line, 24px below the header's lower edge, and the section is the last
// one whose heading is at or above that line. The bookmark keeps the place in
// that section, and Copy path copies the line of that section's heading: a
// heading the header has just covered is still the section being read, and a
// heading the text has not brought up to the reading line is not yet.
//
// In the last screen of a note the reading line moves down with the scroll and
// reaches the bottom of the window where the note ends, so a short section at
// the end, which could never be scrolled up to the line, is still read in its
// turn: walked from the top to the end, every section is the one being read
// somewhere, in order, none skipped.
//
// The two lines of the long note are counted by hand from the fixture.
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/Back%20restore.md';
// "## The Second section" is line 33 of the file, "## The Third section" line 57.
const SECOND = { id: 'the-second-section', line: 33 };
const THIRD = { id: 'the-third-section', line: 57 };
// A note that ends in three sections of a few lines each, the last of which
// the line under the header never reaches.
const SHORT_END = '/notes/Notes/Glass%20Tide.md';
// How far the walk scrolls at a time: less than the shortest of those sections
// moves past the line, so none can fall between two steps.
const STEP = 16;
const BELOW_HEADER = 24;

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL current-section: ${message}`);
};

const open = async (context, path) => {
  const page = await context.newPage();
  // No place is written to the fixture: a press is answered as the server
  // answers a kept place, without reaching it.
  await page.route('**/bookmark', (route) => (route.request().method() === 'POST' ? route.fulfill({ status: 204, body: '' }) : route.continue()));
  const response = await page.goto(BASE + path, { waitUntil: 'networkidle' });
  if (response?.status() !== 200) throw new Error(`BROKEN current-section: ${path} answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(async () => {
    await document.fonts.ready;
    // The page is measured once it has stopped changing height under what it
    // draws late.
    let height = -1;
    while (height !== document.documentElement.scrollHeight) {
      height = document.documentElement.scrollHeight;
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  });
  if (!(await page.locator('[data-path-control]').getAttribute('data-path'))) fail(`${path} carries no path to copy`);
  return page;
};

const settle = (page) => page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));

// Presses the bookmark and then Copy path where the window is, and answers the
// section whose margin says the place is kept there and what was copied. Each
// reply and the clipboard are emptied first, so neither answer is an old one.
const pressBoth = async (page) => {
  await page.evaluate(async () => {
    for (const said of document.querySelectorAll('[data-mark-said], [data-path-said]')) said.textContent = '';
    await navigator.clipboard.writeText('');
  });
  await page.locator('[data-mark-control] [data-mark-button]').click();
  await page.locator('[data-mark-said]:not(:empty)').waitFor({ timeout: 3000 });
  await page.locator('[data-path-button]').click();
  await page.waitForFunction(async () => (await navigator.clipboard.readText()) !== '', null, { timeout: 3000 });
  return page.evaluate(async () => {
    const labels = [...document.querySelectorAll('.y-prose [data-section-bookmark]')].filter((node) => !node.hidden);
    const section = labels.length === 1 ? labels[0].closest('.y-sect') : null;
    return {
      kept: labels.length === 1 ? section?.firstElementChild?.id ?? null : labels.length === 0 ? null : `${labels.length} sections`,
      line: section?.dataset.sourceLine ?? '',
      copied: await navigator.clipboard.readText(),
      y: Math.round(window.scrollY),
    };
  });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, permissions: ['clipboard-read', 'clipboard-write'] });

  // Around the header, on a long note.
  const page = await open(context, PAGE);
  const path = await page.locator('[data-path-control]').getAttribute('data-path');
  const cases = [
    // The heading is going under the header: the reader is reading its text.
    { name: 'a heading the header is covering', below: -16, want: THIRD, covered: true },
    // The heading is in view, just above the reading line.
    { name: 'a heading just above the reading line', below: BELOW_HEADER - 4, want: THIRD, covered: false },
    // The heading is in view below the reading line: the text above it, which
    // the reader is reading, is still the section before.
    { name: 'a heading below the reading line', below: BELOW_HEADER + 30, want: SECOND, covered: false },
  ];
  for (const one of cases) {
    const at = await page.evaluate(([id, below]) => {
      const header = document.querySelector('.y-header').getBoundingClientRect().bottom;
      const element = document.getElementById(id);
      window.scrollTo(0, Math.round(element.getBoundingClientRect().top + window.scrollY - header - below));
      const scroller = document.scrollingElement ?? document.documentElement;
      return { header, left: scroller.scrollHeight - innerHeight - window.scrollY };
    }, [THIRD.id, one.below]);
    await settle(page);
    const top = await page.evaluate((id) => document.getElementById(id).getBoundingClientRect().top, THIRD.id);
    const covered = top < at.header - 0.5 && top >= -0.5;
    // Far from the end, where the reading line stands still under the header.
    if (one.covered !== covered || at.left < 800) {
      throw new Error(`BROKEN current-section: ${one.name} put the heading at ${top}px under a header ending at ${at.header}px, ${at.left}px from the end`);
    }
    const said = await pressBoth(page);
    console.log(`current-section: ${one.name} (heading at ${Math.round(top)}px, header to ${at.header}px): bookmark kept in ${said.kept}, copied ${said.copied.slice(path.length)}`);
    if (said.kept !== one.want.id) fail(`with ${one.name}, the bookmark keeps the place in ${JSON.stringify(said.kept)}, want ${JSON.stringify(one.want.id)}`);
    if (said.copied !== `${path}:${one.want.line}`) fail(`with ${one.name}, Copy path copies ${JSON.stringify(said.copied.slice(path.length))} after the path, want ":${one.want.line}"`);
  }
  await page.close();

  // From the top to the end of a note whose last sections are short.
  const short = await open(context, SHORT_END);
  const shortPath = await short.locator('[data-path-control]').getAttribute('data-path');
  const sections = await short.evaluate(() => [...document.querySelectorAll('.y-prose .y-sect')].map((section) => section.firstElementChild.id));
  const { range, under, view, tail } = await short.evaluate((ids) => ({
    range: (document.scrollingElement ?? document.documentElement).scrollHeight - innerHeight,
    under: document.querySelector('.y-header').getBoundingClientRect().bottom + 24,
    view: innerHeight,
    tail: ids.slice(-3).map((id) => document.getElementById(id).getBoundingClientRect().top + window.scrollY),
  }), sections);
  if (sections.length < 4 || range < view || tail[2] - tail[0] > view || tail[2] <= range + under) {
    throw new Error(`BROKEN current-section: ${SHORT_END} has ${sections.length} sections over ${range}px of scroll, the last three opening at ${JSON.stringify(tail)}, want them within one screen and the last below where the line under the header can reach`);
  }
  const reached = [];
  for (let y = 0; ; y = Math.min(range, y + STEP)) {
    await short.evaluate((to) => window.scrollTo(0, to), y);
    await settle(short);
    const said = await pressBoth(short);
    const want = said.line ? `${shortPath}:${said.line}` : shortPath;
    if (said.copied !== want) fail(`at ${said.y}px of ${SHORT_END} the bookmark keeps the place in ${JSON.stringify(said.kept)} but Copy path copies ${JSON.stringify(said.copied.slice(shortPath.length))}, want ${JSON.stringify(want.slice(shortPath.length))}`);
    if (said.kept !== null && said.kept !== reached.at(-1)) reached.push(said.kept);
    if (y === range) break;
  }
  console.log(`current-section: ${SHORT_END} walked to ${range}px: ${reached.length} of ${sections.length} sections read in turn, ending ${JSON.stringify(reached.slice(-3))}`);
  if (JSON.stringify(reached) !== JSON.stringify(sections)) {
    const missing = sections.filter((id) => !reached.includes(id));
    fail(`walking ${SHORT_END} from the top to the end read ${JSON.stringify(reached.slice(-4))} last, and never ${JSON.stringify(missing)}; want every section in order`);
  }
  await context.close();
  console.log('PASS current-section: the bookmark and Copy path both take the section at the reading line, 24px under the header: a heading the header is covering and one just above the line are the section being read, and one below the line is not yet; in the last screen the line moves to the bottom of the window, so walking a note whose last sections are short reads every section in order');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
