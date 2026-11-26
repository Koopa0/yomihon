// Behavior lock for the pages the Atelier does not draw, set in its language:
// a mode's own page (the courses, the reports, the reader's marks), the
// journal's month, the whole-folder findings, two notes side by side, and a
// file that is not a note.
//
// A mode's page opens on a head closed by the double rule — a firm hairline
// over a finer one in a lighter ink, four pixels apart — as a kind of finding
// on the findings page does, and a file's head is the note's frontispiece,
// closed the same way. Nothing on them is boxed: a mark the reader left open
// stands on a 1.5px line down its start and on no ground; a day of the month
// has no square round it, the weeks parted by a hairline; a finding's weight is
// a mark and a word with no box and no ground, and an error is not drawn in
// red. The way a shelf is ordered, and the control that keeps two columns
// level, are segments under a hairline. Every line of these pages' own text is
// a whole pixel tall at every text size, every piece of their small print is
// 4.5:1 or more on its ground in light and in dark, and none of them scrolls
// sideways on a phone.
//
// The marks it leaves open are its own, made on the disposable fixture and
// taken off again before it ends.
//
// Env: YOMIHON_BASE and MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const MUTATE = process.env.MUTATE || '';
const COURSES = '/courses';
const REPORTS = '/reports';
const MARKS = '/marks';
const JOURNAL = '/journal?month=2026-07';
const HEALTH = '/health';
const COMPARE = '/compare/Notes/cutover.md?with=Notes%2Fcutover-zh-tw.md';
const FILE = '/notes/Examples/code-copy.go';
const MARKED = ['Notes/alpha.md', 'Notes/beta.md'];

const css = (rule, site) => ({ css: rule, site });
const MODES = {
  // A mode's head closed on a single rule.
  'index-head-single-rule': css('.y-home__head--index::after{border-bottom:0 !important}', 'thresholds'),
  // A kind of finding with no rule under its name.
  'finding-kind-without-rule': css('.y-healthgroup__title::after{content:none !important}', 'thresholds'),
  // A file's head left open onto its body.
  'file-head-without-rule': css('.y-frontis::after{content:none !important}', 'thresholds'),
  // A mark left open laid in a box.
  'open-mark-boxed': css('.y-list--open .y-openitem{border:1px solid var(--line) !important;background:var(--panel) !important}', 'boxes'),
  // A day of the month in a square again.
  'day-in-a-square': css('.y-month__cell{border:1px solid var(--line) !important}', 'boxes'),
  // A day written on, filled.
  'written-day-filled': css('.y-month__cell--written{background:var(--panel) !important}', 'boxes'),
  // The weeks run together with nothing between them.
  'weeks-unruled': css('.y-month tbody tr + tr{border-top:0 !important}', 'boxes'),
  // A finding's weight as a box round its word.
  'weight-boxed': css('.y-severity--warn{background:var(--panel) !important;border:1px solid var(--outline) !important}', 'boxes'),
  // An error drawn in the alarm's red.
  'error-in-red': css('.y-severity--error{color:var(--crimson) !important}', 'boxes'),
  // The shelf's order as loose words.
  'order-without-segments': css('.y-order button{border:0 !important}', 'segments'),
  // The control that levels the columns as loose words.
  'align-without-segment': css('.y-compare__align{border:0 !important}', 'segments'),
  // A column's head ruled off from the note under it.
  'column-head-ruled': css('.y-compare__head{border-bottom:1px solid var(--line) !important}', 'boxes'),
  // A shelf row on a line that is not a whole pixel at the larger sizes.
  'fractional-row-line': css('.y-row__title{line-height:1.55 !important}', 'lines'),
  // A finding's file on a fractional line.
  'fractional-finding-line': css('.y-healthlink{line-height:1.45 !important}', 'lines'),
  // The way back to the desk in the ornament ink.
  'back-too-faint': css('.y-back{color:var(--fg-ornament) !important}', 'contrast'),
  // A day's number in the ornament ink.
  'day-too-faint': css('.y-month__day{color:var(--fg-ornament) !important}', 'contrast'),
  // The count beside a finding in the ornament ink.
  'count-too-faint': css('.y-findings__count{color:var(--fg-ornament) !important}', 'contrast'),
  // A mode's head wider than a phone.
  'head-past-the-phone': css('@media (max-width:720px){.y-home__head--index .y-countline{min-width:520px !important}}', 'sideways'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`other-pages: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL other-pages: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN other-pages: ${message}`); };
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
// opaque sRGB bytes, read back through a canvas; the ground is every
// translucent background from the element out to the root, laid in order.
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
  // The crimson as this theme resolves it, read off an element that wears it.
  const swatch = document.createElement('span');
  swatch.style.color = 'var(--crimson)';
  document.body.append(swatch);
  const crimson = rgba(getComputedStyle(swatch).color);
  swatch.remove();
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
    return { what: `${element.tagName.toLowerCase()}.${element.className}`, ink: over(ink, ground), ground, crimson };
  });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
const maker = await browser.newContext();
const made = [];
try {
  console.log(`other-pages: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  const at = async ({ path, width = 1512, theme = 'light', size = '', script = true }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: script });
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

  // The shelf of marks lists only what the reader left open, so the probe
  // leaves two of its own.
  for (const path of MARKED) {
    const answer = await maker.request.post(`${BASE}/questions`, { form: { path, anchor: '' } });
    if (answer.status() !== 200 || (await answer.json()).marked !== true) broken(`leaving a question on ${path} answered ${answer.status()}`);
    made.push(path);
  }

  // The double rule under every threshold.
  if (runs('thresholds')) {
    for (const [path, selector] of [[COURSES, '.y-home__head--index'], [REPORTS, '.y-home__head--index'], [MARKS, '.y-home__head--index'], [JOURNAL, '.y-home__head--index'], [HEALTH, '.y-healthgroup__title'], [FILE, '.y-frontis']]) {
      await at({ path }, async (page) => {
        await count(page, selector, path);
        const rules = await page.locator(selector).evaluateAll((heads) => heads.map((head) => {
          const after = getComputedStyle(head, '::after');
          return { content: after.content, height: after.height, top: after.borderTopWidth, bottom: after.borderBottomWidth, topStyle: after.borderTopStyle, bottomStyle: after.borderBottomStyle, topInk: after.borderTopColor, bottomInk: after.borderBottomColor };
        }));
        for (const rule of rules) {
          const height = Number.parseFloat(rule.height);
          const fine = rule.content !== 'none' && height >= 4 && height <= 5 && rule.top === '1px' && rule.topStyle === 'solid' && rule.bottomStyle === 'solid' && Number.parseFloat(rule.bottom) > 0 && Number.parseFloat(rule.bottom) <= 1 && rule.bottomInk !== rule.topInk;
          if (!fine) fire('thresholds', `${selector} on ${path} closes on ${JSON.stringify(rule)}, want a 1px firm rule over a finer hairline in a lighter ink, 4px apart`);
        }
      });
    }
    console.log('other-pages: a mode\'s head, a kind of finding and a file\'s head close on the double rule');
  }

  // Nothing is boxed: the open marks, the days, the weights, the column heads.
  if (runs('boxes')) {
    await at({ path: MARKS }, async (page) => {
      if (await count(page, '.y-list--open .y-openitem', MARKS) < MARKED.length) broken(`${MARKS} lists fewer marks than the probe left`);
      const items = await page.locator('.y-list--open .y-openitem').evaluateAll((all) => all.map((item) => {
        const s = getComputedStyle(item);
        return { start: s.borderInlineStartWidth, startStyle: s.borderInlineStartStyle, top: s.borderTopWidth, end: s.borderInlineEndWidth, ground: s.backgroundColor };
      }));
      for (const item of items) {
        // The engine snaps a border to the device's pixels, so the 1.5px line is
        // read as a line of at least one pixel and at most two.
        const start = Number.parseFloat(item.start);
        if (!(start >= 1 && start <= 2) || item.startStyle !== 'solid') fire('boxes', `a mark left open stands on ${item.start} ${item.startStyle} at its start, want a 1.5px line`);
        if (item.top !== '0px' || item.end !== '0px' || item.ground !== 'rgba(0, 0, 0, 0)') fire('boxes', `a mark left open is boxed: ${JSON.stringify(item)}`);
      }
    });
    await at({ path: JOURNAL }, async (page) => {
      await count(page, '.y-month__cell--written', JOURNAL);
      const month = await page.locator('.y-month').evaluate((table) => ({
        cells: [...table.querySelectorAll('.y-month__cell')].map((cell) => {
          const s = getComputedStyle(cell);
          return { sides: [s.borderTopWidth, s.borderRightWidth, s.borderBottomWidth, s.borderLeftWidth].map(Number.parseFloat), ground: s.backgroundColor };
        }),
        weeks: [...table.querySelectorAll('tbody tr')].slice(1).map((row) => Number.parseFloat(getComputedStyle(row).borderTopWidth)),
      }));
      for (const cell of month.cells) {
        if (cell.sides.some((side) => side > 0) || cell.ground !== 'rgba(0, 0, 0, 0)') fire('boxes', `a day of the month is boxed or filled: ${JSON.stringify(cell)}`);
      }
      if (month.weeks.length < 3 || month.weeks.some((line) => !(line > 0))) fire('boxes', `the weeks are not each parted by a hairline: ${JSON.stringify(month.weeks)}`);
    });
    await at({ path: HEALTH }, async (page) => {
      await count(page, '.y-findings .y-severity', HEALTH);
      const weights = await page.locator('.y-severity').evaluateAll((all) => all.map((mark) => {
        const s = getComputedStyle(mark);
        return { what: mark.className, ground: s.backgroundColor, edge: Number.parseFloat(s.borderTopWidth) };
      }));
      for (const weight of weights) {
        if (weight.ground !== 'rgba(0, 0, 0, 0)' || weight.edge > 0) fire('boxes', `a finding's weight is drawn as a box: ${JSON.stringify(weight)}`);
      }
      const inks = await page.evaluate(readInk, '.y-severity--error');
      if (inks.length < 1) broken(`${HEALTH} shows no error to read the ink of`);
      for (const { ink, crimson } of inks) {
        if (ink.every((value, i) => Math.abs(value - crimson[i]) < 2)) fire('boxes', 'an error is drawn in the crimson that is kept for a failed write');
      }
    });
    await at({ path: COMPARE }, async (page) => {
      await count(page, '.y-compare__head', COMPARE);
      const heads = await page.locator('.y-compare__head').evaluateAll((all) => all.map((head) => Number.parseFloat(getComputedStyle(head).borderBottomWidth)));
      if (heads.some((line) => line > 0)) fire('boxes', `a column's head is ruled off from its note: ${JSON.stringify(heads)}`);
    });
    console.log('other-pages: nothing on the pages is boxed, and an error is not red');
  }

  // The order of a shelf and the control that levels two columns are segments.
  if (runs('segments')) {
    await at({ path: MARKS }, async (page) => {
      await count(page, '.y-order button', MARKS);
      const buttons = await page.locator('.y-order button').evaluateAll((all) => all.map((button) => {
        const s = getComputedStyle(button);
        return { top: s.borderTopWidth, bottom: s.borderBottomWidth, style: s.borderTopStyle };
      }));
      for (const button of buttons) {
        if (button.top !== '1px' || button.bottom !== '1px' || button.style !== 'solid') fire('segments', `the shelf's order is not one segmented control: ${JSON.stringify(buttons)}`);
      }
    });
    await at({ path: COMPARE }, async (page) => {
      await count(page, '.y-compare__align', COMPARE);
      const edge = await page.locator('.y-compare__align').evaluate((button) => ({ width: getComputedStyle(button).borderTopWidth, style: getComputedStyle(button).borderTopStyle }));
      if (edge.width !== '1px' || edge.style !== 'solid') fire('segments', `the control that levels the columns is not a segment under a hairline: ${JSON.stringify(edge)}`);
    });
    console.log('other-pages: the shelf\'s order and the levelling control are segments under a hairline');
  }

  // Every line of the pages' own text is a whole pixel, at every text size.
  if (runs('lines')) {
    const text = '.y-home__head .y-title, .y-home__head > p, .y-indexcount, .y-back, .y-row__title, .y-row__opening, .y-openitem__name, .y-openitem__meta, .y-month__caption, .y-month__weekday, .y-month__day, .y-month__entry, .y-healthlede, .y-healthshape, .y-healthrange, .y-healthgroup__title, .y-healthlink, .y-findings__head, .y-severity, .y-findings__detail, .y-compare__alone, .y-frontis .y-title, .y-frontis .y-crumbs, .y-frontis .y-metarow';
    for (const size of ['', 'l', 'xl']) {
      for (const path of [REPORTS, MARKS, JOURNAL, HEALTH, COMPARE, FILE]) {
        await at({ path, size }, async (page) => {
          const lines = await page.evaluate((selector) => [...document.querySelectorAll(selector)].filter((element) => element.getClientRects().length > 0).map((element) => ({
            what: `${element.tagName.toLowerCase()}.${element.className}`,
            line: getComputedStyle(element).lineHeight,
          })), text);
          if (lines.length < 2) broken(`only ${lines.length} lines were read on ${path}`);
          for (const { what, line } of lines) {
            const px = Number.parseFloat(line);
            if (!line.endsWith('px') || Math.abs(px - Math.round(px)) > 0.001) fire('lines', `at size ${size || 'm'} ${what} on ${path} is set on a ${line} line, want a whole pixel`);
          }
        });
      }
    }
    console.log('other-pages: every line on the pages is a whole pixel tall at all three text sizes');
  }

  // The small print is text: 4.5:1 or more on the ground it stands on.
  if (runs('contrast')) {
    const print = '.y-indexcount, .y-back, .y-order__label, .y-order button, .y-row__when, .y-row__measure, .y-row__opening, .y-openitem__meta time, .y-months__step, .y-month__weekday, .y-month__day, .y-healthshape, .y-healthrange, .y-healthabout__head, .y-findings__head, .y-findings__detail, .y-findings__count, .y-severity, .y-compare__alone, .y-compare__align, .y-frontis .y-crumbs__link, .y-frontis .y-metarow';
    for (const theme of ['light', 'dark']) {
      for (const path of [REPORTS, MARKS, JOURNAL, HEALTH, COMPARE, FILE]) {
        await at({ path, theme }, async (page) => {
          const inks = await page.evaluate(readInk, print);
          if (inks.length < 2) broken(`only ${inks.length} pieces of small print were read on ${path}`);
          for (const { what, ink, ground } of inks) {
            const ratio = contrast(ink, ground);
            if (ratio < 4.5) fire('contrast', `in ${theme} ${what} on ${path} stands at ${ratio.toFixed(2)}:1 on its ground, want 4.5:1`);
          }
        });
      }
    }
    console.log('other-pages: the small print on the pages is 4.5:1 or more in light and dark');
  }

  // Nothing scrolls sideways on a phone, with or without script.
  if (runs('sideways')) {
    for (const script of [true, false]) {
      for (const path of [COURSES, REPORTS, MARKS, JOURNAL, HEALTH, COMPARE, FILE]) {
        await at({ path, width: 390, script }, async (page) => {
          const [scroll, inner] = await page.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
          if (scroll > inner) fire('sideways', `${path} at 390px ${script ? 'with' : 'without'} script is ${scroll}px wide, wider than the window`);
        });
      }
    }
    console.log('other-pages: none of the pages scrolls sideways at 390px, with or without script');
  }

  if (mode && planted === 0) throw new NotApplied(`the stylesheet was never requested, so ${MUTATE} reached no page`);
  if (MUTATE) failure = new Error(`no catch: ${MUTATE} left every lock standing`);
  else console.log('PASS other-pages');
} catch (err) {
  failure = err;
} finally {
  for (const path of made) {
    await maker.request.post(`${BASE}/questions`, { form: { path, anchor: '' } }).catch(() => {});
  }
  await maker.close();
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
