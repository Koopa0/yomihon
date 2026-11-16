// Behavior lock for where a note's contents and its aids live. The contents
// hang in the left sidebar under the row that is the note, headings in order
// and last a row that leads down to what the note ends on, named with that
// block's own first heading; every other aid waits in that one block after the
// text, at every width, in one order. There is no right rail and no folded row
// of aids between the title and the first sentence.
//
// However long the contents are, each of their rows can be reached with the
// keyboard: focus brings it into the part of the window the sidebar shows, under
// the header and not under the row that sticks to the sidebar's top.
//
// A heading in the contents is one row and the row is its link: the door that
// leaves a thought on a section stands in that section's own margin, beside its
// heading in the text, and the sidebar carries none.
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
const NO_DOORS = 'contents-rows-carry-no-door';
const REACH = 'every-contents-row-is-reachable';

// The text a mutation plants, the response it is planted in, and the site of
// the lock that must catch it.
const MODES = {
  'drop-contents-under-the-row': { document: ['<nav class="y-railtoc"', '<nav class="y-railtoc-gone"'], site: CONTENTS },
  'misaim-the-last-row': { document: ['href="#_y-end"', 'href="#_y-ended"'], site: LEADS },
  'hide-the-end-block-when-narrow': { css: '@media (max-width:1279px){.y-related{display:none}}', site: ENDS },
  'put-the-cited-by-block-first': { css: '.y-related{flex-direction:column-reverse}', site: ORDER },
  'restore-the-right-rail': { document: ['</main>', '</main><aside class="y-rail-right"></aside>'], site: NO_RAIL },
  // A sidebar that is as tall as its contents and sticks to the window has
  // nothing to scroll with: the rows past the window's bottom are out of reach.
  'let-the-sidebar-outgrow-the-window': { css: '.y-rail-left{height:auto !important;overflow:visible !important}', site: REACH },
  // A door stood up again in a row of the contents: the sidebar says the same
  // thing the section's own margin does.
  'put-a-door-back-in-the-end-row': { document: ['</a></div></div></nav>', '</a><a class="y-toc__door" href="/thought/Notes/x.md" aria-label="Leave a thought"></a></div></div></nav>'], site: NO_DOORS },
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

const open = async (width, height, touch = false) => {
  const context = await browser.newContext({ viewport: { width, height }, reducedMotion: 'no-preference', hasTouch: touch });
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
const RANKS = ['y-pair', 'y-basedon', 'y-basedon-declared', 'y-citedby', 'y-diag', 'y-facts'];

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
    fail(ORDER, `the end block holds ${JSON.stringify(end.kids)}, want the note to read beside, the sources, the notes declared on it, the notes citing it, the findings and the facts, in that order`);
  }
  console.log(`sidebar-contents: ${contents.headings.length} headings and a last row ${JSON.stringify(contents.last.text)}; the end block holds ${end.kids.join(', ')}`);

  // Following the last row lands on the end block, under the header.
  await page.locator('.y-railtoc .y-toc__row--end a').first().click();
  await page.waitForFunction(() => document.getElementById('_y-end').getBoundingClientRect().top < innerHeight);
  const landed = await page.evaluate(() => ({ top: document.getElementById('_y-end').getBoundingClientRect().top, header: document.querySelector('.y-header').getBoundingClientRect().bottom }));
  if (landed.top < landed.header - 1) fail(LEADS, `following the last row parks the end block at ${landed.top}px, under the ${landed.header}px header`);

  // The contents' rows are their links and nothing else: no door to a thought
  // beside a heading, and none leading anywhere under /thought/.
  const doored = await page.evaluate(() => [...document.querySelectorAll('.y-rail-left .y-railtoc .y-toc__row')].filter((row) => row.children.length !== 1 || row.querySelector('[href^="/thought/"]')).length);
  if (doored > 0) fail(NO_DOORS, `${doored} rows of the contents hold more than their link or lead to a thought`);
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

  // A window shorter than the contents, so the sidebar has to scroll them.
  const short = await open(1440, 768);
  const links = short.page.locator('.y-rail-left .y-railtoc .y-toc__row a:first-child');
  const total = await links.count();
  if (total < 20) broken(`${PAGE} has ${total} contents rows, which is too few to overflow a 768px window`);
  for (let i = 0; i < total; i += 1) {
    await links.nth(i).focus();
    const reached = await links.nth(i).evaluate(async (link) => {
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      const box = link.getBoundingClientRect();
      const hit = document.elementFromPoint((box.left + box.right) / 2, (box.top + box.bottom) / 2);
      return { top: box.top, bottom: box.bottom, header: document.querySelector('.y-header').getBoundingClientRect().bottom, window: innerHeight, hit: Boolean(hit && (hit === link || link.contains(hit))) };
    });
    if (!(reached.top >= reached.header - 1 && reached.bottom <= reached.window + 1 && reached.hit)) {
      fail(REACH, `focus on contents row ${i + 1} of ${total} leaves it out of reach in a 768px window: ${JSON.stringify(reached)}`);
    }
  }
  short.proof();
  await short.context.close();

  console.log('PASS sidebar-contents: the contents hang under the note in the sidebar and end on a row that leads to the end block, which holds every other aid in one order at every width, with no right rail; each heading is one row that is its link');
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
