// Behavior lock for where a note's contents and its aids live. The contents
// hang in the left sidebar under the row that is the note, headings in order
// and last a row that leads down to what the note ends on, named with that
// block's own first heading; every other aid waits in that one block after the
// text, at every width, in one order. There is no right rail and no folded row
// of aids between the title and the first sentence.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson with an outline and every aid), and
// MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

const CONTENTS = 'contents-under-the-row';
const LEADS = 'last-row-leads-to-the-end';
const ENDS = 'the-end-block-stays';
const ORDER = 'aids-in-one-order';
const NO_RAIL = 'no-right-rail';

// The text a mutation plants, the response it is planted in, and the site of
// the lock that must catch it.
const MODES = {
  'drop-contents-under-the-row': { document: ['<nav class="y-railtoc"', '<nav class="y-railtoc-gone"'], site: CONTENTS },
  'misaim-the-last-row': { document: ['href="#_y-end"', 'href="#_y-ended"'], site: LEADS },
  'hide-the-end-block-when-narrow': { css: '@media (max-width:1279px){.y-related{display:none}}', site: ENDS },
  'put-the-cited-by-block-first': { css: '.y-related{flex-direction:column-reverse}', site: ORDER },
  'restore-the-right-rail': { document: ['</main>', '</main><aside class="y-rail-right"></aside>'], site: NO_RAIL },
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`sidebar-contents: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL sidebar-contents: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const fail = (site, message) => { throw new LockFired(site, message); };
const broken = (message) => { throw new Error(`BROKEN sidebar-contents: ${message}`); };

const browser = await chromium.launch({ channel: 'chrome', headless: true });

const open = async (width, height) => {
  const context = await browser.newContext({ viewport: { width, height }, reducedMotion: 'no-preference' });
  const page = await context.newPage();
  const mode = MUTATE ? MODES[MUTATE] : null;
  const seen = { requests: 0, matches: 0 };
  if (mode?.document) {
    const [needle, replacement] = mode.document;
    await page.route((url) => url.pathname === decodeURI(PAGE) || url.pathname === PAGE, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      seen.requests += 1;
      seen.matches += original.split(needle).length - 1;
      await route.fulfill({ response, body: original.replace(needle, replacement) });
    });
  }
  if (mode?.css) {
    await page.route('**/static/app.css{,?*}', async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      seen.requests += 1;
      seen.matches += 1;
      await route.fulfill({ response, body: `${original}\n${mode.css}\n` });
    });
  }
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  return {
    page,
    context,
    proof() {
      if (!MUTATE) return;
      if (seen.requests < 1 || (mode.document && seen.matches !== seen.requests)) {
        throw new NotApplied(`NOT-APPLIED sidebar-contents: the planted change matched ${seen.matches} times across ${seen.requests} responses`);
      }
    },
  };
};

// Which of the end block's blocks a child is, in the order the page draws them.
const RANKS = ['y-pair', 'y-basedon', 'y-basedon-declared', 'y-citedby', 'y-diag'];

try {
  console.log(`sidebar-contents: Chrome ${browser.version()}`);

  const wide = await open(1440, 900);
  const { page } = wide;

  if (await page.locator('.y-rail-right').count() !== 0) fail(NO_RAIL, 'the page draws a right rail');
  if (await page.locator('.y-inlineaids').count() !== 0) fail(NO_RAIL, 'the page draws a folded row of aids above the prose');

  const contents = await page.evaluate(() => {
    const row = document.querySelector('.y-rail-left .ui-navitem[aria-current="page"]');
    const list = row?.nextElementSibling;
    const under = Boolean(list?.matches('nav.y-railtoc'));
    const links = [...(list?.querySelectorAll('.y-toc__row a') ?? [])];
    const headings = links.filter((link) => !link.closest('.y-toc__row--end')).map((link) => {
      const target = document.getElementById(decodeURIComponent(link.getAttribute('href').slice(1)));
      return { text: link.textContent.trim(), target: target?.textContent.trim() ?? null, inArticle: Boolean(target?.closest('.y-article')), top: target?.getBoundingClientRect().top ?? null };
    });
    const last = list?.querySelector('.y-toc__row--end a');
    return { under, headings, last: last ? { href: last.getAttribute('href'), text: last.textContent.trim() } : null };
  });
  if (!contents.under) fail(CONTENTS, 'the contents do not open directly after the row of the sidebar that is the note');
  if (contents.headings.length < 3) broken(`${PAGE} has ${contents.headings.length} headings, which is too few to say their order`);
  for (const heading of contents.headings) {
    if (!heading.inArticle || heading.target !== heading.text) fail(CONTENTS, `the contents row ${JSON.stringify(heading.text)} leads to ${JSON.stringify(heading.target)}`);
  }
  if (contents.headings.some((heading, i) => i > 0 && heading.top < contents.headings[i - 1].top)) fail(CONTENTS, 'the contents are not in the order of the headings');

  const end = await page.evaluate(() => {
    const block = document.getElementById('_y-end');
    if (!block) return null;
    const label = block.querySelector('.ui-side__label');
    const clone = label?.cloneNode(true);
    for (const counter of clone?.querySelectorAll('.ui-navitem__count, .y-diagcount') ?? []) counter.remove();
    // In the order a reader meets them on the page, which is where each one
    // is drawn and not the order the markup happens to list them in.
    const kids = [...block.children].map((child) => ({
      top: child.getBoundingClientRect().top,
      kind: child.matches('nav.y-basedon[data-declared-by]') ? 'y-basedon-declared'
        : child.querySelector('.y-diaglist') ? 'y-diag'
          : [...child.classList].find((name) => ['y-pair', 'y-basedon', 'y-citedby'].includes(name)) ?? child.className,
    })).sort((a, b) => a.top - b.top).map((child) => child.kind);
    const style = getComputedStyle(block);
    return { first: clone?.textContent.trim() ?? null, kids, display: style.display, prose: document.querySelector('.y-prose')?.getBoundingClientRect().top ?? null, top: block.getBoundingClientRect().top + scrollY };
  });
  if (!end) fail(LEADS, 'the page has no block with the id the last contents row leads to');
  if (!contents.last || contents.last.href !== '#_y-end') fail(LEADS, `the contents do not end on a row that leads to #_y-end: ${JSON.stringify(contents.last)}`);
  if (contents.last.text !== end.first) fail(LEADS, `the last contents row says ${JSON.stringify(contents.last.text)}, but the block it leads to opens with ${JSON.stringify(end.first)}`);
  const ranks = end.kids.map((kid) => RANKS.indexOf(kid));
  if (end.kids.length < 4 || ranks.includes(-1) || ranks.some((rank, i) => i > 0 && rank < ranks[i - 1])) {
    fail(ORDER, `the end block holds ${JSON.stringify(end.kids)}, want the note to read beside, the sources, the notes declared on it, the notes citing it, and the findings, in that order`);
  }
  console.log(`sidebar-contents: ${contents.headings.length} headings and a last row ${JSON.stringify(contents.last.text)}; the end block holds ${end.kids.join(', ')}`);

  // Following the last row lands on the end block, under the header.
  await page.locator('.y-railtoc .y-toc__row--end a').click();
  await page.waitForFunction(() => document.getElementById('_y-end').getBoundingClientRect().top < innerHeight);
  const landed = await page.evaluate(() => ({ top: document.getElementById('_y-end').getBoundingClientRect().top, header: document.querySelector('.y-header').getBoundingClientRect().bottom }));
  if (landed.top < landed.header - 1) fail(LEADS, `following the last row parks the end block at ${landed.top}px, under the ${landed.header}px header`);
  wide.proof();
  await wide.context.close();

  // At a narrow width the end block is where the aids are, as it is at a wide
  // one, and the contents are in the drawer.
  const narrow = await open(390, 800);
  const shown = await narrow.page.evaluate(() => {
    const block = document.getElementById('_y-end');
    const prose = document.querySelector('.y-prose');
    return { visible: Boolean(block && block.getClientRects().length > 0 && getComputedStyle(block).visibility !== 'hidden'), after: Boolean(block && prose && (prose.compareDocumentPosition(block) & Node.DOCUMENT_POSITION_FOLLOWING)) };
  });
  if (!shown.visible || !shown.after) fail(ENDS, `at 390px the end block is visible = ${shown.visible} and after the text = ${shown.after}`);
  await narrow.page.locator('[data-nav-toggle]').click();
  await narrow.page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
  if (!(await narrow.page.locator('.y-railtoc').first().isVisible())) fail(CONTENTS, 'at 390px the drawer does not show the contents');
  narrow.proof();
  await narrow.context.close();

  console.log('PASS sidebar-contents: the contents hang under the note in the sidebar and end on a row that leads to the end block, which holds every other aid in one order at every width, with no right rail');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.site === MODES[MUTATE].site) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
