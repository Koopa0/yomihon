// Behavior lock for where the page stands. Every page's block is centred on
// the stage, the 1680px band in the middle of the window that the header's row
// and the left column keep to, and the left column takes no room from it: so
// the text of every page stands on the window's middle, at its own measure,
// whether the column is open, folded or a drawer, and opening or folding the
// column moves none of it. It is measured where the heading's first glyph is,
// at the widths a window is opened at and with the column open and folded.
//
// It holds the column and the header to the stage. On a window wider than the
// stage the column stands at the stage's left edge, not the window's, and the
// header's row keeps between the stage's edges; on the pages whose column
// opens on a title, a book's or a folder's, or on a library's first row, those
// first words stand where the header's first control does, so the two read as
// one line down the page.
//
// And it holds the three checks the Atelier handoff names for this layout:
// nothing scrolls sideways at 1512 and 2560 with the column open and folded,
// nor at 1100; the margin beside a note's sections stays sixteen pixels in
// from the window's right side at 1512 with the column open; and at 1100, with
// the drawer closed, the window's left twenty pixels are the page's paper and
// nothing else, which is where a closed drawer's shadow would show. Beside
// those, from 1760 the margin stands outside the wide lines.
//
// Env: YOMIHON_BASE and MUTATE.
import { inflateSync } from 'node:zlib';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const MUTATE = process.env.MUTATE || '';

const STAGE = 1680;
const RAIL_WIDTH = 256;

const WITH_SIDEBAR = [
  ['a lesson', '/notes/Course/C02.md', '.y-article', '.y-article h1.y-title'],
  ['a course', '/courses/Maps/study.md', '.y-syl', '.y-syl h1.y-title'],
  ['a search', '/search?q=alpha', '.y-searchpage', '.y-searchpage h1.y-title'],
  ['a file', '/notes/Notes/plain.txt', '.y-article', '.y-article h1.y-title'],
  ['the desk', '/', '.y-home', '.y-home h1.y-title'],
  ['the courses', '/courses', '.y-home', '.y-home h1.y-title'],
  ['the folders', '/folders', '.y-home', '.y-home h1.y-title'],
  ['a folder', '/folders/Notes', '.y-home', '.y-home h1.y-title'],
  ['the marks', '/marks', '.y-home', '.y-home h1.y-title'],
];
// The pages whose sidebar opens on a title, a book's or a folder's, or on the
// first row of the library, set the first words on the header's line.
const OPENS_ON_A_TITLE = new Set(['a lesson', 'a course', 'a file', 'a search']);
const TITLES = '.y-railbook__title, .y-where__label, .y-here--flat > .ui-navitem';
// The findings, the journal, the reports and the listening page draw their
// sidebar only as a drawer; their text is centred all the same.
const DRAWER_ONLY = [
  ['the findings', '/health', '.y-healthpage', '.y-healthpage h1.y-title'],
  ['the journal', '/journal', '.y-home', '.y-home h1.y-title'],
  ['the reports', '/reports', '.y-home', '.y-home h1.y-title'],
  ['a course read aloud', '/listen/Maps/listen.md', '.y-listen', '.y-listen h1.y-title'],
];
const SETTINGS = ['the settings', '/preferences', '.y-prefs', '.y-prefs h1'];
// A note whose sections carry the reader's own thoughts, so its margin has
// something in it to measure.
const MARGINS = '/notes/Notes/Reading%20flow.md';

const MODES = {
  // A page's block hung off the left again, at the edge the old shell kept.
  'hang-the-findings-page-off-the-left': { css: '.y-main > .y-healthpage{margin-left:64px !important}', group: 'centred' },
  'hang-the-search-page-off-the-left': { css: '.y-main > .y-searchpage{margin-left:0 !important}', group: 'centred' },
  'pin-the-course-to-the-left': { css: '.y-main > .y-syl{margin-left:0 !important}', group: 'centred' },
  // The text making way for the column, as it did when the column was a track.
  'push-the-text-beside-the-open-column': { css: "html[data-rail='open'] .y-main{padding-left:256px !important}", group: 'unmoved' },
  // The column taking a track of its own again.
  'give-the-column-a-track': { css: '@media (min-width:1321px){.y-shell{display:grid !important;grid-template-columns:256px minmax(0,1fr) !important}.y-shell > .y-rail-left{position:sticky !important}}', group: 'centred' },
  // The column and the header left at the window's edges on a wide window.
  'pin-the-column-to-the-window': { css: '.y-shell > .y-rail-left{left:0 !important}', group: 'stage' },
  'run-the-header-to-the-window': { css: '.y-header{padding-inline:24px !important}', group: 'stage' },
  'crowd-the-sidebar-against-the-edge': { css: ':root{--rail-pad-start:6px}', group: 'stage' },
  'indent-the-library-row': { css: '.y-rail-left .y-here--flat > .ui-navitem{padding-left:20px !important}', group: 'stage' },
  'indent-the-folder-label': { css: '.y-where{padding-left:8px}', group: 'stage' },
  // A block wider than the window allows.
  'run-the-desk-past-the-window': { css: '@media (min-width:1500px){.y-home{min-width:2600px}}', group: 'no-sideways' },
  // The margin's column wider than the window has room for.
  // The margin set outside the wide lines below 1760, where the window has
  // no room for it there.
  'set-the-margin-outside-the-wide-lines-at-every-width': { css: '.y-main > .y-article .y-margin{inset-inline-start:calc(50% + var(--wide-read) / 2 + 36px) !important}', group: 'margin-in-window' },
  'widen-the-margin': { css: '.y-main > .y-article .y-margin{width:40rem !important}', group: 'margin-in-window' },
  // The margin left beside the text on a window wide enough to set it clear
  // of the wide lines.
  'keep-the-margin-beside-the-text-past-1760': { css: '@media (min-width:1760px){.y-main > .y-article .y-margin{inset-inline-start:calc(100% + var(--margin-gap)) !important}}', group: 'margin-outside-wide' },
  // A closed drawer that keeps its shadow, which falls across the left edge.
  'shadow-the-closed-drawer': { css: 'html[data-js] .y-rail-left{box-shadow:0 0 40px 12px rgb(0 0 0 / 0.45) !important}', group: 'closed-edge-is-paper' },
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`left-edge: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(group, message) {
    super(`FAIL left-edge: ${message}`);
    this.group = group;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN left-edge: ${message}`); };
const fire = (group, message) => { throw new LockFired(group, message); };
const runs = (group) => !MUTATE || MODES[MUTATE].group === group;

// The rows of a screenshot, decoded: an 8-bit RGB or RGBA PNG, which is what
// the browser takes, unfiltered row by row.
const decodePng = (buffer) => {
  let offset = 8;
  let width = 0;
  let height = 0;
  let channels = 0;
  const data = [];
  while (offset < buffer.length) {
    const length = buffer.readUInt32BE(offset);
    const type = buffer.toString('ascii', offset + 4, offset + 8);
    const chunk = buffer.subarray(offset + 8, offset + 8 + length);
    if (type === 'IHDR') {
      width = chunk.readUInt32BE(0);
      height = chunk.readUInt32BE(4);
      if (chunk[8] !== 8 || chunk[12] !== 0) broken('the screenshot is not an 8-bit, non-interlaced PNG');
      channels = { 2: 3, 6: 4 }[chunk[9]];
      if (!channels) broken(`the screenshot's colour type ${chunk[9]} is neither RGB nor RGBA`);
    } else if (type === 'IDAT') {
      data.push(chunk);
    }
    offset += 12 + length;
  }
  const raw = inflateSync(Buffer.concat(data));
  const stride = width * channels;
  const pixels = Buffer.alloc(height * stride);
  for (let y = 0; y < height; y += 1) {
    const filter = raw[y * (stride + 1)];
    for (let x = 0; x < stride; x += 1) {
      const value = raw[y * (stride + 1) + 1 + x];
      const left = x >= channels ? pixels[y * stride + x - channels] : 0;
      const up = y > 0 ? pixels[(y - 1) * stride + x] : 0;
      const corner = y > 0 && x >= channels ? pixels[(y - 1) * stride + x - channels] : 0;
      let predicted = 0;
      if (filter === 1) predicted = left;
      else if (filter === 2) predicted = up;
      else if (filter === 3) predicted = Math.floor((left + up) / 2);
      else if (filter === 4) {
        const p = left + up - corner;
        const pa = Math.abs(p - left);
        const pb = Math.abs(p - up);
        const pc = Math.abs(p - corner);
        predicted = pa <= pb && pa <= pc ? left : (pb <= pc ? up : corner);
      }
      pixels[y * stride + x] = (value + predicted) & 0xff;
    }
  }
  return { width, height, at: (x, y) => [...pixels.subarray(y * stride + x * channels, y * stride + x * channels + 3)] };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  console.log(`left-edge: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  const plant = async (page) => {
    if (!mode) return;
    await page.route('**/static/app.css{,?*}', async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      requests += 1;
      planted += 1;
      await route.fulfill({ response, body: `${original}\n${mode.css}\n` });
    });
  };
  const open = async (width, rail, options = {}) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce', ...options });
    await context.addCookies([{ name: 'yomihon_rail', value: rail, url: BASE }]);
    const page = await context.newPage();
    await plant(page);
    return { context, page };
  };
  const visit = async (page, path) => {
    const response = await page.goto(BASE + path, { waitUntil: 'load' });
    if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
    await page.evaluate(() => document.fonts.ready);
  };

  // The stage. On a window wider than it, the column stands at its left edge
  // and the header's row keeps between its edges, and the column's first words
  // stand on the header's line. Measured before the rest, so a mutation aimed
  // here is read here first.
  if (runs('stage')) {
    for (const width of [1920, 2560]) {
      const { context, page } = await open(width, 'open');
      for (const [name, path] of WITH_SIDEBAR) {
        await visit(page, path);
        const edge = await page.evaluate(([titles]) => {
          const sidebar = document.querySelector('#_y-nav-rail');
          if (!sidebar || getComputedStyle(sidebar).visibility !== 'visible') return null;
          const controls = [...document.querySelectorAll('.y-header a, .y-header button')]
            .filter((element) => element.getClientRects().length > 0)
            .map((element) => element.getBoundingClientRect());
          // The first words the column draws, measured where the glyphs are
          // rather than where their box begins.
          const walker = document.createTreeWalker(sidebar, NodeFilter.SHOW_TEXT, {
            acceptNode: (node) => (node.textContent.trim() && !node.parentElement.closest('script, [hidden]') ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT),
          });
          let words = null;
          let wordsIn = null;
          for (let node = walker.nextNode(); node && !words; node = walker.nextNode()) {
            const range = document.createRange();
            range.selectNodeContents(node);
            words = [...range.getClientRects()].find((rect) => rect.width > 1 && rect.height > 1) ?? null;
            if (words) wordsIn = node.parentElement;
          }
          const box = sidebar.getBoundingClientRect();
          return {
            // The width the page is laid out in: the window less the gutter
            // its scrollbar keeps, which the header spans.
            room: document.querySelector('.y-header').getBoundingClientRect().width,
            sidebarStart: box.left,
            sidebarWidth: box.width,
            wordsStart: words?.left ?? null,
            wordsAreATitle: Boolean(wordsIn?.closest(titles)),
            headerStart: Math.min(...controls.map((rect) => rect.left)),
            headerEnd: Math.max(...controls.map((rect) => rect.right)),
          };
        }, [TITLES]);
        if (!edge) broken(`${path} draws no open column at ${width}px with the column chosen open`);
        if (edge.wordsStart === null) broken(`${path} draws no words in the column, so where they start proves nothing`);
        const stageStart = (edge.room - STAGE) / 2;
        console.log(`left-edge: at ${width}px, ${name}: the stage starts at ${stageStart}, the column at ${edge.sidebarStart} (${edge.sidebarWidth} wide), its words at ${edge.wordsStart}, the header's row ${edge.headerStart}-${edge.headerEnd}`);
        if (Math.abs(edge.sidebarStart - stageStart) > 0.5 || Math.abs(edge.sidebarWidth - RAIL_WIDTH) > 0.5) {
          fire('stage', `${name} at ${width}px: the column stands at ${edge.sidebarStart}px, ${edge.sidebarWidth}px wide, want ${RAIL_WIDTH}px at the stage's edge ${stageStart}px`);
        }
        if (edge.headerStart < stageStart || edge.headerEnd > stageStart + STAGE) {
          fire('stage', `${name} at ${width}px: the header's row runs ${edge.headerStart}-${edge.headerEnd}px, outside the stage ${stageStart}-${stageStart + STAGE}px`);
        }
        if (!OPENS_ON_A_TITLE.has(name)) continue;
        if (!edge.wordsAreATitle) broken(`${name} no longer opens its column on a title (${TITLES}), so the header's line is not what its first words are measured against`);
        if (Math.abs(edge.wordsStart - edge.headerStart) > 0.5) {
          fire('stage', `${name} at ${width}px: the column's words start at ${edge.wordsStart}px and the header's first control at ${edge.headerStart}px: ${JSON.stringify(edge)}`);
        }
        if (name !== 'a search') continue;
        // The library is one flat list on the same edge.
        const rows = await page.evaluate(() => [...document.querySelectorAll('.y-rail-left .y-here--flat > :is(.ui-navitem, .y-here__label, .y-here__more)')].map((row) => {
          const range = document.createRange();
          range.selectNodeContents(row.querySelector('span') ?? row);
          return { text: row.textContent.trim().slice(0, 24), left: Math.round(range.getBoundingClientRect().left * 10) / 10 };
        }));
        if (rows.length < 4) broken(`the library draws ${rows.length} rows at ${width}px, too few to say they stand on one edge`);
        const off = rows.filter((row) => Math.abs(row.left - edge.headerStart) > 0.5);
        if (off.length > 0) fire('stage', `the library's rows at ${width}px stand off the header's line at ${edge.headerStart}px: ${JSON.stringify(off)}`);
      }
      await context.close();
    }
  }

  // Centred, and unmoved by the column: at each width, each page's heading is
  // read with the column open and with it folded, the two must agree, and the
  // page's block must stand on the middle of the window.
  if (runs('centred') || runs('unmoved')) {
    const pages = [...WITH_SIDEBAR, ...DRAWER_ONLY, SETTINGS];
    for (const width of [1100, 1321, 1440, 1512, 1700, 2560]) {
      const found = {};
      for (const rail of ['open', 'collapsed']) {
        const { context, page } = await open(width, rail);
        for (const [name, path, block, heading] of pages) {
          await visit(page, path);
          found[`${name}/${rail}`] = await page.evaluate(([blockSelector, headingSelector]) => {
            const box = document.querySelector(blockSelector)?.getBoundingClientRect();
            const title = document.querySelector(headingSelector)?.getBoundingClientRect();
            if (!box || !title) return null;
            const band = document.querySelector('.y-header').getBoundingClientRect();
            return { middle: band.left + band.width / 2, centre: box.left + box.width / 2, width: box.width, heading: title.left };
          }, [block, heading]);
          const got = found[`${name}/${rail}`];
          if (!got) broken(`${path} draws no ${block} or no heading at ${width}px`);
          if (runs('centred') && Math.abs(got.centre - got.middle) > 0.5) {
            fire('centred', `${name} at ${width}px with the column ${rail}: the block's middle is at ${got.centre}px, the window's at ${got.middle}px`);
          }
        }
        await context.close();
      }
      for (const [name] of pages) {
        const open = found[`${name}/open`];
        const folded = found[`${name}/collapsed`];
        if (Math.abs(open.heading - folded.heading) > 0.5) {
          fire('unmoved', `${name} at ${width}px: the heading starts at ${open.heading}px with the column open and at ${folded.heading}px with it folded; opening the column moved the text`);
        }
      }
      console.log(`left-edge: at ${width}px every page's block is centred and its heading stands where it does with the column folded`);
    }
  }

  // Nothing scrolls sideways at the widths the handoff names, with the column
  // open and folded, on every page that has a column, every page with only a
  // drawer, the settings and a note with a margin.
  if (runs('no-sideways')) {
    const pages = [...WITH_SIDEBAR, ...DRAWER_ONLY, SETTINGS, ['a note with a margin', MARGINS]];
    for (const [width, rails] of [[1512, ['open', 'collapsed']], [2560, ['open', 'collapsed']], [1100, ['collapsed']]]) {
      for (const rail of rails) {
        const { context, page } = await open(width, rail);
        for (const [name, path] of pages) {
          await visit(page, path);
          const sideways = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, room: window.innerWidth }));
          if (sideways.scroll > sideways.room) {
            fire('no-sideways', `${name} at ${width}px with the column ${rail} is ${sideways.scroll}px wide in a ${sideways.room}px window`);
          }
        }
        await context.close();
      }
    }
    console.log('left-edge: nothing scrolls sideways at 1512 and 2560 with the column open and folded, nor at 1100');
  }

  // The margin at 1512 with the column open: drawn beside the text and sixteen
  // pixels in from the window's right side at the most.
  if (runs('margin-in-window')) {
    const { context, page } = await open(1512, 'open');
    await context.addCookies([{ name: 'yomihon_sectionbuttons', value: 'always', url: BASE }]);
    await visit(page, MARGINS);
    const margin = await page.evaluate(() => {
      const prose = document.querySelector('.y-prose').getBoundingClientRect();
      return {
        room: window.innerWidth,
        proseEnd: prose.right,
        items: [...document.querySelectorAll('.y-main > .y-article .y-margin')]
          .filter((item) => item.getClientRects().length > 0)
          .map((item) => ({ left: item.getBoundingClientRect().left, right: item.getBoundingClientRect().right })),
      };
    });
    if (margin.items.length === 0) broken(`${MARGINS} draws no margin at 1512px`);
    for (const item of margin.items) {
      if (item.left < margin.proseEnd) broken(`a margin at 1512px starts at ${item.left}px, inside the text ending at ${margin.proseEnd}px, so it is not the margin beside it`);
      if (item.right > margin.room - 16) {
        fire('margin-in-window', `a margin at 1512px with the column open ends at ${item.right}px, past ${margin.room - 16}px, sixteen in from the window's right side`);
      }
    }
    console.log(`left-edge: at 1512px with the column open ${margin.items.length} margins end by ${Math.max(...margin.items.map((item) => item.right))}px of ${margin.room}`);
    await context.close();
  }

  // From 1760 the margin stands outside the wide lines: 520 from the middle of
  // the window and 36 beyond, at every text size.
  if (runs('margin-outside-wide')) {
    for (const size of ['m', 'l', 'xl']) {
      const { context, page } = await open(2560, 'open');
      await context.addCookies([{ name: 'yomihon_textsize', value: size, url: BASE }, { name: 'yomihon_sectionbuttons', value: 'always', url: BASE }]);
      await visit(page, MARGINS);
      const placed = await page.evaluate(() => {
        const band = document.querySelector('.y-header').getBoundingClientRect();
        return {
          middle: band.left + band.width / 2,
          starts: [...document.querySelectorAll('.y-main > .y-article .y-margin')]
            .filter((item) => item.getClientRects().length > 0)
            .map((item) => item.getBoundingClientRect().left),
        };
      });
      if (placed.starts.length === 0) broken(`${MARGINS} draws no margin at 2560px`);
      for (const start of placed.starts) {
        if (Math.abs(start - (placed.middle + 520 + 36)) > 0.5) {
          fire('margin-outside-wide', `at 2560px and text size ${size} a margin starts at ${start}px, want ${placed.middle + 556}px, 36 beyond the wide lines`);
        }
      }
      await context.close();
    }
    console.log('left-edge: at 2560px the margin stands 36px outside the wide lines at every text size');
  }

  // At 1100 with the drawer closed, the window's left twenty pixels below the
  // header are the paper and nothing else. The light falling on the page is
  // taken off for this reading, since it is a gradient across the paper that
  // has nothing to do with the drawer; what is left must be the page's own
  // ground, pixel for pixel.
  if (runs('closed-edge-is-paper')) {
    for (const scheme of ['light', 'dark']) {
      const { context, page } = await open(1100, 'collapsed', { colorScheme: scheme });
      await visit(page, '/notes/Course/C02.md');
      await page.waitForSelector('html[data-js]');
      await page.addStyleTag({ content: ':root, :root * { --bg-glow: none !important; }' });
      if (await page.evaluate(() => document.documentElement.dataset.nav) === 'open') broken('the drawer is open at 1100 before anything was pressed');
      const ground = await page.evaluate(() => {
        const probe = document.createElement('div');
        probe.style.cssText = 'position:fixed;inset:0;background:var(--bg);visibility:hidden';
        document.body.append(probe);
        const colour = getComputedStyle(probe).backgroundColor;
        probe.remove();
        return colour.match(/\d+/g).slice(0, 3).map(Number);
      });
      const header = await page.evaluate(() => document.querySelector('.y-header').getBoundingClientRect().bottom);
      const top = Math.ceil(header) + 2;
      const shot = decodePng(await page.screenshot({ clip: { x: 0, y: top, width: 20, height: 900 - top - 2 } }));
      for (let y = 0; y < shot.height; y += 1) {
        for (let x = 0; x < shot.width; x += 1) {
          const pixel = shot.at(x, y);
          if (pixel.some((value, i) => Math.abs(value - ground[i]) > 1)) {
            fire('closed-edge-is-paper', `${scheme}: at 1100px with the drawer closed the pixel at ${x},${top + y} is rgb(${pixel.join(', ')}), not the paper rgb(${ground.join(', ')})`);
          }
        }
      }
      await context.close();
    }
    console.log('left-edge: at 1100px with the drawer closed the left twenty pixels are the paper, light and dark');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED left-edge: the planted rule reached ${planted} of ${requests} stylesheet requests`);
  console.log('PASS left-edge: the column and the header keep to the stage with the column\'s words on the header\'s line; every page\'s block is centred and does not move when the column opens or folds; nothing scrolls sideways at 1512, 2560 and 1100; the margin stays in the window at 1512 with the column open and stands outside the wide lines at 2560; and a closed drawer leaves the window\'s left edge to the paper');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.group === MODES[MUTATE].group) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
