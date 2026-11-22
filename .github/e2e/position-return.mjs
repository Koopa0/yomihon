// Behavior lock: a note opens where the reader last stopped in it, and the place
// it opens at is the one they left.
//
// A place is kept against the last anchor at or above the reading line and the
// distance below it, and put back by the same agreement run the other way. The
// reading line is not a fixed distance from the top of the window: in the last
// screen of a note it moves down with the scroll, evenly, and is at the bottom
// of the window where the note ends. A return that took the line to be fixed
// would bring a reader who stopped there back somewhere else, so the cases here
// stop where the line is under the header, where a heading has just gone under
// it, and 400px from the end of a note.
//
// The place is also kept against a section, so a note that changed above the
// section the reader stopped in still opens in that section. That is run by
// serving the note again with a tall block written above its body, which
// moves every section down by the same height; a place kept as a pixel would
// open at the old depth.
//
// The reader stops, the browser writes, the reader leaves, and the note is
// opened again by address: nothing here goes through the page's own scripts.
// Each case starts a new browser context, so a place from an earlier case is
// not what is being returned to.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { frames, openNote, scrollTop, waitForPlace } from './support/position.mjs';

// Subpixel layout and a heading's own rounding let a return differ from the
// place by a few pixels without the place having moved.
const SLACK = 2;
const SPACER = 500;

const CASES = [
  {
    name: 'a heading 16px under the header',
    note: 'Back restore',
    width: 1280,
    place: () => {
      const header = document.querySelector('.y-header').getBoundingClientRect().bottom;
      const heading = document.getElementById('the-third-section');
      window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY - header + 16));
      const top = heading.getBoundingClientRect().top;
      return top > 0 && top < header ? '' : `the heading is at ${top}px, not under a header ending at ${header}px`;
    },
    // The heading is above the reading line, which is the section the reader is
    // in; the window top is still in the section before it.
    anchor: 'the-third-section',
    edited: true,
  },
  {
    name: '300px into a section, in a narrow window',
    note: 'Back restore',
    width: 375,
    place: () => {
      const heading = document.getElementById('the-fourth-section');
      window.scrollTo(0, Math.round(heading.getBoundingClientRect().top + window.scrollY + 300));
      return '';
    },
    anchor: 'the-fourth-section',
    edited: true,
  },
  {
    name: '400px from the end, in the last screen',
    note: 'Glass Tide',
    width: 1280,
    place: () => {
      const scroller = document.scrollingElement ?? document.documentElement;
      const range = scroller.scrollHeight - innerHeight;
      if (range < innerHeight + 400) return `the note scrolls ${range}px, too little to stand 400px from its end with a screen above`;
      window.scrollTo(0, range - 400);
      return '';
    },
    anchor: null,
    edited: false,
  },
  {
    name: 'the last pixel of a note',
    note: 'Glass Tide',
    width: 1280,
    place: () => {
      const scroller = document.scrollingElement ?? document.documentElement;
      window.scrollTo(0, scroller.scrollHeight - innerHeight);
      return '';
    },
    anchor: null,
    edited: false,
  },
];

class LockFired extends Error {}
const fail = (message) => {
  throw new LockFired(`FAIL position-return: ${message}`);
};
const broken = (message) => {
  throw new Error(`BROKEN position-return: ${message}`);
};

const pathOf = (note) => `Notes/${note}.md`;
const addressOf = (note) => `/notes/Notes/${encodeURIComponent(note)}.md`;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  for (const one of CASES) {
    const context = await browser.newContext({ viewport: { width: one.width, height: 800 } });
    const page = await context.newPage();
    await openNote(page, BASE, addressOf(one.note));
    const issue = await page.evaluate(one.place);
    if (issue) broken(`${one.name}: ${issue}`);
    await frames(page);
    const kept = await scrollTop(page);

    const stored = await waitForPlace(page, pathOf(one.note));
    const sectionTop = () => page.evaluate((id) => {
      const element = document.getElementById(id);
      return element ? Math.round(element.getBoundingClientRect().top + window.scrollY) : null;
    }, stored?.anchor ?? '');
    if (stored === null) fail(`with ${one.name}, nothing was kept in the browser's storage for ${pathOf(one.note)} after the reader stopped at ${kept}`);
    if (one.anchor !== null && stored.anchor !== one.anchor) {
      fail(`with ${one.name}, the place was kept against ${JSON.stringify(stored.anchor)}, want ${JSON.stringify(one.anchor)}: the section the reading line is in`);
    }

    await page.goto(`${BASE}/`, { waitUntil: 'load' });
    await openNote(page, BASE, addressOf(one.note));
    const landed = await scrollTop(page);
    console.log(`position-return: ${one.name}: kept at ${kept} against ${JSON.stringify(stored.anchor)} ${stored.offset}px below it, opened at ${landed}`);
    if (Math.abs(landed - kept) > SLACK) {
      fail(`with ${one.name}, the place was kept at ${kept} against ${JSON.stringify(stored.anchor)} ${stored.offset}px below it and the note opened at ${landed}, ${landed - kept}px away`);
    }

    if (one.edited) {
      // The same note with a block above everything the reader read. The place
      // is in the same section, so the window follows the section down by as
      // far as the section moved, which is measured and not assumed: the block
      // has margins of its own.
      const before = await sectionTop();
      await page.goto(`${BASE}/`, { waitUntil: 'load' });
      await page.route(`**${addressOf(one.note)}`, async (route) => {
        const response = await route.fetch();
        const body = await response.text();
        const edited = body.replace('<div class="y-prose">', `<div class="y-prose"><div style="height:${SPACER}px" aria-hidden="true"></div>`);
        if (edited === body) broken('the block above the body was not written, so the note was served unchanged');
        await route.fulfill({ response, body: edited });
      });
      await openNote(page, BASE, addressOf(one.note));
      const followed = await scrollTop(page);
      const moved = (await sectionTop()) - before;
      console.log(`position-return: ${one.name}: the note edited above it moved the section by ${moved}px, opened at ${followed}`);
      if (moved < SPACER) broken(`${one.name}: the block above the body moved the section by ${moved}px, less than the ${SPACER}px it is`);
      if (Math.abs(followed - (kept + moved)) > SLACK) {
        fail(`with ${one.name}, the note was edited above the section, which moved ${moved}px, and opened at ${followed}, want ${kept + moved}: a place kept as a depth would have opened at ${kept}`);
      }
    }
    await context.close();
  }
  console.log('PASS position-return: a note opens at the place the reader stopped, against its section, wherever the reading line was');
} catch (error) {
  console.error(error instanceof LockFired ? error.message : error);
  exit = 1;
} finally {
  await browser.close();
}
process.exit(exit);
