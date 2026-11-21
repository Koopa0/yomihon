// Behavior lock: the section Copy path names is the one the reader's eyes are
// in. The header stays over the top of the text, so the window's own top is
// under it, and the text a reader is reading starts below it. The line asked is
// the reading line, 24px below the header's lower edge, and the section is the
// last one whose heading is at or above that line: a heading the header has
// just covered is still the section being read, and a heading the text has not
// brought up to the reading line is not yet.
//
// In the last screen of a note the reading line moves down with the scroll and
// reaches the bottom of the window where the note ends, so a short section at
// the end, which could never be scrolled up to the line, is still read in its
// turn: walked from the top to the end, every section is the one Copy path
// names somewhere, in order, none skipped.
//
// The bookmark is not asked here. It keeps its place at the top of the window,
// as the probes about it hold.
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

// Presses Copy path where the window is and answers what was copied. The
// clipboard is emptied first, so the answer is never an old one.
const press = async (page) => {
  await page.evaluate(() => navigator.clipboard.writeText(''));
  await page.locator('[data-path-button]').click();
  await page.waitForFunction(async () => (await navigator.clipboard.readText()) !== '', null, { timeout: 3000 });
  return page.evaluate(() => navigator.clipboard.readText());
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
    const copied = await press(page);
    console.log(`current-section: ${one.name} (heading at ${Math.round(top)}px, header to ${at.header}px): copied ${copied.slice(path.length)}`);
    if (copied !== `${path}:${one.want.line}`) fail(`with ${one.name}, Copy path copies ${JSON.stringify(copied.slice(path.length))} after the path, want ":${one.want.line}"`);
  }
  await page.close();

  // From the top to the end of a note whose last sections are short.
  const short = await open(context, SHORT_END);
  const shortPath = await short.locator('[data-path-control]').getAttribute('data-path');
  // Each section is told apart by the line it names, so every one has to name one.
  const sections = await short.evaluate(() => [...document.querySelectorAll('.y-prose .y-sect')].map((section) => section.dataset.sourceLine ?? ''));
  if (sections.some((line) => line === '') || new Set(sections).size !== sections.length) {
    throw new Error(`BROKEN current-section: the sections of ${SHORT_END} name the lines ${JSON.stringify(sections)}, want one line each, all different`);
  }
  const { range, under, view, tail } = await short.evaluate(() => ({
    range: (document.scrollingElement ?? document.documentElement).scrollHeight - innerHeight,
    under: document.querySelector('.y-header').getBoundingClientRect().bottom + 24,
    view: innerHeight,
    tail: [...document.querySelectorAll('.y-prose .y-sect')].slice(-3).map((section) => section.getBoundingClientRect().top + window.scrollY),
  }));
  if (sections.length < 4 || range < view || tail[2] - tail[0] > view || tail[2] <= range + under) {
    throw new Error(`BROKEN current-section: ${SHORT_END} has ${sections.length} sections over ${range}px of scroll, the last three opening at ${JSON.stringify(tail)}, want them within one screen and the last below where the line under the header can reach`);
  }
  const reached = [];
  for (let y = 0; ; y = Math.min(range, y + STEP)) {
    await short.evaluate((to) => window.scrollTo(0, to), y);
    await settle(short);
    const copied = await press(short);
    if (copied !== shortPath && !copied.startsWith(`${shortPath}:`)) fail(`at ${y}px of ${SHORT_END} Copy path copies ${JSON.stringify(copied)}, which is not the note's path`);
    const line = copied.slice(shortPath.length + 1);
    if (line !== '' && line !== reached.at(-1)) reached.push(line);
    if (y === range) break;
  }
  console.log(`current-section: ${SHORT_END} walked to ${range}px: ${reached.length} of ${sections.length} sections read in turn, ending ${JSON.stringify(reached.slice(-3))}`);
  if (JSON.stringify(reached) !== JSON.stringify(sections)) {
    const missing = sections.filter((line) => !reached.includes(line));
    fail(`walking ${SHORT_END} from the top to the end, Copy path named the lines ${JSON.stringify(reached.slice(-4))} last, and never ${JSON.stringify(missing)}; want every section's line in order`);
  }
  await context.close();
  console.log('PASS current-section: Copy path names the section at the reading line, 24px under the header: a heading the header is covering and one just above the line are the section being read, and one below the line is not yet; in the last screen the line moves to the bottom of the window, so walking a note whose last sections are short names every section in order');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
