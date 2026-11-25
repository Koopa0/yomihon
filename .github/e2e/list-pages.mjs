// Behavior lock for the list pages as the Atelier sets them: the desk, a
// folder and the search page.
//
// Every structural threshold on them — a desk band's head, a folder's toolbar,
// the search page's query — closes on the double rule: a firm hairline over a
// half-weight one in a lighter ink, four pixels apart. (The browser draws the
// half pixel as the finest line it has, so the lock reads the two inks.) The place the reader kept stands on a
// plate lifted off the desk, the bookmark's ribbon hanging from the plate's top
// edge, and its one way back is the desk's one filled control, the paper's
// colour on the ink. The folders run across the desk as a strip three to a
// row, two on a phone. A folder's order is one segmented control in its
// toolbar. A hit on the results page lies on the faint hit ground, set
// heavier. Every line of these pages' own text is a whole pixel tall at every
// text size, and every piece of their small print is text at 4.5:1 or more on
// the ground it actually stands on, in light and in dark. None of the three
// scrolls sideways on a phone.
//
// It keeps a place of its own, so it stands among the probes that leave one
// behind (see probes.sh).
//
// Env: YOMIHON_BASE and MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const MUTATE = process.env.MUTATE || '';
const NOTE = 'Notes/alpha.md';
const DESK = '/';
const FOLDER = '/folders/Notes';
const FOLDER_BY_DATE = '/folders/Notes?sort=recent';
const SEARCH = '/search?q=alpha';

const css = (rule, site) => ({ css: rule, site });
const MODES = {
  // A band's head with a single rule under it, as the desk had before.
  'band-head-single-rule': css('.y-homeblock__head::after{border-bottom:0 !important}', 'thresholds'),
  // The folder's toolbar with no rule at all.
  'toolbar-without-rule': css('.y-listbar::after{content:none !important}', 'thresholds'),
  // The query over a rule of the wrong weight.
  'query-heavy-rule': css('.y-searchpage__title::after{border-top-width:2px !important}', 'thresholds'),
  // The kept place laid flat on the desk, with no plate under it.
  'place-without-plate': css('.y-continue{background:none !important;box-shadow:none !important;border:0 !important}', 'plate'),
  // The ribbon gone from the plate's edge.
  'plate-without-ribbon': css('.y-continue__ribbon{display:none !important}', 'plate'),
  // The way back drawn as an outline again.
  'way-back-as-outline': css('.y-continue__note[href]{background:transparent !important;color:var(--fg) !important}', 'plate'),
  // The folders as a single column.
  'folders-in-one-column': css("[data-home-block='folders'] .y-modeblock__rows{grid-template-columns:minmax(0,1fr) !important}", 'strip'),
  // The order as loose words with no control round them.
  'order-without-segments': css('.y-listbar .y-seg{border:0 !important}', 'toolbar'),
  // A hit set heavier but on no ground.
  'hit-without-ground': css('.y-found mark{background:none !important}', 'hits'),
  // A folder's row on a line that is not a whole pixel at the larger sizes.
  'fractional-row-line': css('.y-lrow__title{line-height:1.5 !important}', 'lines'),
  // A row's fact in the ornament ink, read where the folder is ordered by date.
  'fact-too-faint': css('.y-lrow__fact{color:var(--fg-ornament) !important}', 'contrast'),
  // A band's count in the ornament ink.
  'count-too-faint': css('.y-homeblock__head p{color:var(--fg-ornament) !important}', 'contrast'),
  // The way back's words in the faint ink on the ink.
  'way-back-unreadable': css('.y-continue__note[href]{color:var(--fg-faint) !important}', 'contrast'),
  // The desk's grid wider than a phone.
  'desk-past-the-phone': css('@media (max-width:720px){.y-homegrid{min-width:520px !important}}', 'sideways'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`list-pages: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL list-pages: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN list-pages: ${message}`); };
const fire = (site, message) => { throw new LockFired(site, message); };
const runs = (site) => !MUTATE || MODES[MUTATE].site === site;

// The relative luminance and contrast of WCAG 2, over sRGB.
const channel = (c) => {
  const v = c / 255;
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
};
const luminance = ([r, g, b]) => 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
const contrast = (a, b) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

// In the page: the ink of an element and the ground it stands on, each as
// opaque sRGB bytes. A colour in any syntax is read back through a canvas, and
// the ground is every translucent background from the element out to the
// root, laid over each other in order.
const readInk = (selector) => {
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  const rgba = (value) => {
    ctx.clearRect(0, 0, 1, 1);
    ctx.fillStyle = '#000';
    ctx.fillStyle = value;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
    return [r, g, b, a / 255];
  };
  const over = (top, under) => [0, 1, 2].map((i) => top[i] * top[3] + under[i] * (1 - top[3]));
  return [...document.querySelectorAll(selector)].filter((element) => element.getClientRects().length > 0).map((element) => {
    const layers = [];
    for (let node = element; node; node = node.parentElement) {
      const ground = rgba(getComputedStyle(node).backgroundColor);
      if (ground[3] > 0) layers.push(ground);
      if (ground[3] >= 1) break;
    }
    let ground = [255, 255, 255];
    for (const layer of layers.reverse()) ground = over(layer, ground);
    const ink = rgba(getComputedStyle(element).color);
    return { what: `${element.tagName.toLowerCase()}.${element.className}`, ink: over(ink, ground), ground };
  });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`list-pages: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  const at = async ({ path, width = 1512, theme = 'light', size = '' }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: theme, reducedMotion: 'reduce' });
    await context.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
      ...(size ? [{ name: 'yomihon_textsize', value: size, url: BASE }] : []),
    ]);
    const page = await context.newPage();
    try {
      if (mode) {
        await page.route('**/static/app.css{,?*}', async (route) => {
          const response = await route.fetch();
          const original = await response.text();
          planted += 1;
          return route.fulfill({ response, body: `${original}\n${mode.css}\n` });
        });
      }
      const response = await page.goto(BASE + path, { waitUntil: 'load' });
      if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
      await page.evaluate(() => document.fonts.ready);
      await arrived(page);
      if (size && (await page.evaluate(() => document.documentElement.dataset.textsize)) !== size) broken(`${path} did not take the text size ${size}`);
      return await fn(page);
    } finally {
      await context.close();
    }
  };
  const count = async (page, selector, path) => {
    const n = await page.locator(selector).count();
    if (n < 1) broken(`${path} holds no ${selector}`);
    return n;
  };

  // The desk draws the plate only for a place that was kept.
  const keeper = await browser.newContext();
  const kept = await keeper.request.post(`${BASE}/bookmark`, {
    form: { path: NOTE, anchor: '', offset: '0', identity: '0'.repeat(64) },
  });
  await keeper.close();
  if (kept.status() !== 204) broken(`keeping a place in ${NOTE} answered ${kept.status()}, want 204`);

  // The double rule under every threshold: one pixel of the firm ink over the
  // finer hairline in its lighter ink, in a box four pixels tall.
  if (runs('thresholds')) {
    for (const [path, selector] of [[DESK, '.y-modeblock .y-homeblock__head'], [FOLDER, '.y-listbar'], [SEARCH, '.y-searchpage__title']]) {
      await at({ path }, async (page) => {
        await count(page, selector, path);
        const rules = await page.locator(selector).evaluateAll((heads) => heads.map((head) => {
          const after = getComputedStyle(head, '::after');
          return { content: after.content, height: after.height, top: after.borderTopWidth, bottom: after.borderBottomWidth, topStyle: after.borderTopStyle, bottomStyle: after.borderBottomStyle, topInk: after.borderTopColor, bottomInk: after.borderBottomColor };
        }));
        for (const rule of rules) {
          const fine = rule.content !== 'none' && rule.height === '4px' && rule.top === '1px' && rule.topStyle === 'solid' && rule.bottomStyle === 'solid' && Number.parseFloat(rule.bottom) > 0 && Number.parseFloat(rule.bottom) <= 1 && rule.bottomInk !== rule.topInk;
          if (!fine) fire('thresholds', `${selector} on ${path} closes on ${JSON.stringify(rule)}, want a 1px firm rule over a finer hairline in a lighter ink, 4px apart`);
        }
      });
    }
    console.log('list-pages: the desk\'s bands, a folder\'s toolbar and the query close on the double rule');
  }

  // The kept place on its plate, the ribbon at the plate's top edge, and the
  // one way back in the ink.
  if (runs('plate')) {
    await at({ path: DESK }, async (page) => {
      await count(page, '[data-home-continue]', DESK);
      const plate = await page.locator('.y-continue').evaluate((section) => {
        const style = getComputedStyle(section);
        const box = section.getBoundingClientRect();
        const ribbon = section.querySelector('.y-continue__ribbon');
        const way = section.querySelector('.y-continue__note[href]');
        const rb = ribbon && ribbon.getClientRects().length ? ribbon.getBoundingClientRect() : null;
        const body = getComputedStyle(document.body).backgroundColor;
        return {
          ground: style.backgroundColor, body, shadow: style.boxShadow, edge: style.borderTopWidth,
          ribbon: rb && { top: rb.top - box.top, right: box.right - rb.right, width: rb.width, height: rb.height },
          way: way && { ground: getComputedStyle(way).backgroundColor, ink: getComputedStyle(way).color, fg: getComputedStyle(section.querySelector('.y-continue__title')).color },
        };
      });
      if (plate.ground === plate.body || plate.ground === 'rgba(0, 0, 0, 0)' || plate.shadow === 'none' || plate.edge !== '1px') fire('plate', `the kept place is not on a plate: ${JSON.stringify(plate)}`);
      if (!plate.ribbon || Math.abs(plate.ribbon.top) > 1 || plate.ribbon.right < 8 || plate.ribbon.right > 40 || plate.ribbon.height < 30) fire('plate', `the ribbon does not hang from the plate's top edge near its end: ${JSON.stringify(plate.ribbon)}`);
      if (!plate.way || plate.way.ground !== plate.way.fg) fire('plate', `the way back is not filled with the ink: ${JSON.stringify(plate.way)}`);
    });
    console.log('list-pages: the kept place stands on a plate with its ribbon, and the way back is the ink');
  }

  // The folders as a strip: three to a row on a wide window, two on a phone.
  if (runs('strip')) {
    for (const [width, want] of [[1512, 3], [390, 2]]) {
      await at({ path: DESK, width }, async (page) => {
        const rows = "[data-home-block='folders'] .y-modeblock__rows > [data-desk-item]";
        if (await count(page, rows, DESK) < want) broken(`the fixture's desk shows fewer than ${want} folders`);
        const tops = await page.locator(rows).evaluateAll((cells) => cells.map((cell) => Math.round(cell.getBoundingClientRect().top)));
        const first = tops.filter((top) => top === tops[0]).length;
        if (first !== want) fire('strip', `at ${width}px the folder strip's first row holds ${first} folders, want ${want}`);
      });
    }
    console.log('list-pages: the folders run three to a row, two on a phone');
  }

  // The order in the folder's toolbar is one segmented control.
  if (runs('toolbar')) {
    await at({ path: FOLDER }, async (page) => {
      await count(page, '.y-listbar .y-seg', FOLDER);
      const seg = await page.locator('.y-listbar .y-seg').evaluate((group) => {
        const style = getComputedStyle(group);
        return { edge: style.borderTopWidth, style: style.borderTopStyle, members: group.querySelectorAll('a').length };
      });
      if (seg.edge !== '1px' || seg.style !== 'solid' || seg.members < 2) fire('toolbar', `the order is not one segmented control in the toolbar: ${JSON.stringify(seg)}`);
    });
    console.log('list-pages: a folder\'s order is one segmented control in its toolbar');
  }

  // A hit on the results page lies on the hit ground and is set heavier.
  if (runs('hits')) {
    await at({ path: SEARCH }, async (page) => {
      await count(page, '.y-found mark', SEARCH);
      const marks = await page.locator('.y-found mark').evaluateAll((all) => all.map((mark) => {
        const style = getComputedStyle(mark);
        return { ground: style.backgroundColor, weight: Number(style.fontWeight) };
      }));
      for (const mark of marks) {
        if (mark.ground === 'rgba(0, 0, 0, 0)' || mark.weight < 600) fire('hits', `a hit is drawn ${JSON.stringify(mark)}, want the hit ground and a heavier weight`);
      }
    });
    console.log('list-pages: every hit lies on the hit ground, set heavier');
  }

  // Every line of the pages' own text is a whole pixel, at every text size.
  if (runs('lines')) {
    const text = '.y-continue__head, .y-continue__title, .y-continue__section, .y-homeblock__head h2, .y-modeblock .ui-navitem[data-desk-item], .y-modeblock__opening, .y-openitem__name, .y-indexcount, .y-home__head .y-title, .y-crumbs, .y-lgroup__name, .y-lrow__title, .y-cover, .y-searchpage__title, .y-found__title, .y-found__lead, .y-found__where';
    for (const size of ['', 'l', 'xl']) {
      for (const path of [DESK, FOLDER, SEARCH]) {
        await at({ path, size }, async (page) => {
          const lines = await page.evaluate((selector) => [...document.querySelectorAll(selector)].filter((element) => element.getClientRects().length > 0).map((element) => ({
            what: `${element.tagName.toLowerCase()}.${element.className}`,
            line: getComputedStyle(element).lineHeight,
          })), text);
          if (lines.length < 5) broken(`only ${lines.length} lines were read on ${path}`);
          for (const { what, line } of lines) {
            const px = Number.parseFloat(line);
            if (!line.endsWith('px') || Math.abs(px - Math.round(px)) > 0.001) fire('lines', `at size ${size || 'm'} ${what} on ${path} is set on a ${line} line, want a whole pixel`);
          }
        });
      }
    }
    console.log('list-pages: every line on the three pages is a whole pixel tall at all three text sizes');
  }

  // The small print is text: 4.5:1 or more on the ground it stands on.
  if (runs('contrast')) {
    const print = '.y-continue__tag, .y-continue__kicker, .y-continue__device, .y-continue__note[href], .y-homeblock__head p, .y-shelfall, .y-modeblock .ui-navitem__count, .y-modeblock .ui-navitem__when, .y-modeblock__opening, .y-indexcount, .y-lgroup__name, .y-lgroup__count, .y-lrow__fact, .y-seg a, .y-seg__count, .y-searchfilter__label, .y-results__count, .y-found__where, .y-found mark, .y-deskvault, .y-deskhealth a, .y-crumbs__link';
    for (const theme of ['light', 'dark']) {
      for (const path of [DESK, FOLDER, FOLDER_BY_DATE, SEARCH]) {
        await at({ path, theme }, async (page) => {
          const inks = await page.evaluate(readInk, print);
          if (inks.length < 5) broken(`only ${inks.length} pieces of small print were read on ${path}`);
          for (const { what, ink, ground } of inks) {
            const ratio = contrast(ink, ground);
            if (ratio < 4.5) fire('contrast', `in ${theme} ${what} on ${path} stands at ${ratio.toFixed(2)}:1 on its ground, want 4.5:1`);
          }
        });
      }
    }
    console.log('list-pages: the small print on the three pages is 4.5:1 or more in light and dark');
  }

  // Nothing scrolls sideways on a phone.
  if (runs('sideways')) {
    for (const path of [DESK, FOLDER, SEARCH]) {
      await at({ path, width: 390 }, async (page) => {
        const [scroll, inner] = await page.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
        if (scroll > inner) fire('sideways', `${path} at 390px is ${scroll}px wide, wider than the window`);
      });
    }
    console.log('list-pages: none of the three pages scrolls sideways at 390px');
  }

  if (mode && planted === 0) throw new NotApplied(`the stylesheet was never requested, so ${MUTATE} reached no page`);
  if (MUTATE) failure = new Error(`no catch: ${MUTATE} left every lock standing`);
  else console.log('PASS list-pages');
} catch (err) {
  failure = err;
} finally {
  await browser.close();
}

if (failure instanceof NotApplied) {
  console.error(failure.message);
  console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
  process.exit(2);
}
if (failure instanceof LockFired) {
  console.error(failure.message);
  if (MUTATE) {
    if (failure.site === MODES[MUTATE].site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    else console.error(`no catch: ${MUTATE} targets ${MODES[MUTATE].site}, but ${failure.site} fired`);
  }
  process.exit(1);
}
if (failure) {
  console.error(failure.message || failure);
  process.exit(1);
}
