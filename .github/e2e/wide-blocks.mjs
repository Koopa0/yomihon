// Behavior lock for the wide measure and the code block's box.
//
// A block is set wider than the text only when its content needs it: the
// renderer names in data-wide every measure a code block's longest line does
// not fit, counted from the code face's advances, and here each fixture block
// is laid out in the browser at each text size, in a note that takes the
// reading measure and in one that takes the Latin one, and the line the browser
// actually draws is held to what the attribute claims. A block that does not
// need the room stays at the text's width; one inside a callout stays inside
// the callout.
//
// Where it is wide, the block spans the 1040px wide lines centred on the text's
// axis; it never reaches the open left column; and it never runs under the
// margin beside a section, at any width or text size, with or without script:
// below 1760, where the margin stands beside the text, the block gives up its
// right wing, so it ends where the text ends. Below a main of 1120px every block
// is at the text's width, nothing scrolls the page sideways, and the text stands
// where it would with no wide block on the page. The compare page's columns
// keep their blocks at the column's width.
//
// The box's head holds the copy control above the code, so the control is never
// over the first line, and what it copies is the code and nothing of the head.
// Where a pointer can rest the control is faint until it does; on a touch screen
// it is always whole. With no script, a head that would hold only the control is
// not drawn.
//
// Env: YOMIHON_BASE and MUTATE (MUTATE=list prints the self-test modes).
import { readFileSync } from 'node:fs';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const MUTATE = process.env.MUTATE || '';

const NOTE = '/notes/Notes/wide-code.md';
const LATIN = '/notes/Notes/wide-code-latin.md';
const COMPARE = '/compare/Notes/wide-code.md?with=Notes%2Fwide-code-latin.md';
const SIZES = ['m', 'l', 'xl'];
const WIDE = 1040;
const RAIL_GAP = 32;
// The block the long line stands in, its section's margin carrying a thought so
// the margin runs down beside the block's first lines.
const LONG_SECTION = '長的一行';
// The code that block holds, read from the fixture rather than the page, so the
// copied text is held to what the author wrote.
const LONG_CODE = (() => {
  const source = readFileSync(new URL('./vault/Notes/wide-code.md', import.meta.url), 'utf8');
  const found = source.match(new RegExp(`## ${LONG_SECTION}\\n\\n\`\`\`go\\n([\\s\\S]*?)\`\`\`\\n`));
  if (!found) throw new Error(`BROKEN wide-blocks: the fixture has no go block under ${LONG_SECTION}`);
  return found[1];
})();

const css = (rule, group) => ({ css: rule, group });
const MODES = {
  // The renderer's table and the stylesheet's padding drifting apart.
  'pad-the-code-wider': css('.y-prose pre{padding-inline:28px !important}', 'need'),
  // Every block in the text set wide, needed or not.
  'widen-every-block': css('.y-main > .y-article > .y-prose > .y-code{margin-inline:calc(-1 * var(--wing-start)) !important}', 'need'),
  // A block inside a callout set wide.
  'widen-blocks-in-callouts': css(".y-main > .y-article .callout .y-code[data-wide~='m']{margin-inline:-100px !important}", 'need'),
  // The wide block set off the text's axis.
  'set-wide-blocks-off-axis': css('.y-main > .y-article{--wing-end:0px !important}', 'axis'),
  // The open column forgotten when the wings are counted.
  'forget-the-open-column': css('.y-main > .y-article{--wing-edge:var(--gutter-read) !important}', 'rail'),
  // Both wings kept where the margin stands beside the text.
  'keep-both-wings-beside-the-margin': css('.y-main > .y-article:has(.y-margin){--wing-end:var(--wing-start) !important}', 'margin'),
  // The margin left beside the text where the wide lines run to it.
  'keep-the-margin-beside-the-text-past-1760': css('@media (min-width:1760px){.y-main > .y-article .y-margin{inset-inline-start:calc(100% + var(--margin-gap)) !important}}', 'margin'),
  // Wings on a window too narrow to tell the measures apart.
  'widen-below-1120': css('@container y-main (width < 70rem){.y-main > .y-article{--wing-start:120px;--wing-end:120px}}', 'narrow'),
  // A block as wide as its longest line rather than scrolling inside itself.
  'stop-scrolling-inside-the-block': css('.y-prose .y-code{width:max-content !important}', 'sideways'),
  // The text widened to make room for the wide blocks.
  'widen-the-article-for-wide-blocks': css('.y-main > .y-article:has([data-wide]){max-width:calc(var(--wide-read) + 2 * var(--gutter-read)) !important}', 'still'),
  // The head laid over the code instead of above it.
  'lay-the-head-over-the-code': css('.y-prose .y-code{position:relative}.y-prose .y-code > .y-code__head{position:absolute !important;inset:0 0 auto 0}', 'head'),
  // The copy taking the whole box, head and all.
  'copy-the-whole-box': {
    js: { path: '/static/codecopy.js', needle: "const text = block.querySelector('code') || block;", replacement: 'const text = block.parentElement;' },
    group: 'head',
  },
  // The control hidden at rest rather than faint.
  'hide-the-copy-at-rest': css('@media (pointer:fine){.y-code:not(:hover,:focus-within) .y-codecopy{opacity:0 !important}}', 'faint'),
  // The control never faint.
  'never-faint': css('.y-code__head .y-xbtn{color:var(--fg-muted) !important;box-shadow:inset 0 0 0 1px var(--line) !important}', 'faint'),
  // The control faint on a touch screen too.
  'faint-on-touch-too': css('.y-code:not(:hover,:focus-within) .y-code__head .y-xbtn{box-shadow:none !important;color:var(--fg-faint) !important}', 'faint'),
  // The compare page's columns setting their blocks wide.
  'widen-blocks-in-the-compare-columns': css(".y-compare .y-article > .y-prose > [data-wide~='m']{margin-inline:-100px !important}", 'compare'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`wide-blocks: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(group, message) {
    super(`FAIL wide-blocks: ${message}`);
    this.group = group;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN wide-blocks: ${message}`); };
const fire = (group, message) => { throw new LockFired(group, message); };
const runs = (group) => !MUTATE || MODES[MUTATE].group === group;
const near = (a, b) => Math.abs(a - b) <= 0.5;

// The page's geometry: the text, the open column, the visible margins, and
// every code block with the measures it claims and whether it stands directly
// in the text.
const read = (page) => page.evaluate(() => {
  const box = (element) => {
    const r = element.getBoundingClientRect();
    return { left: r.left, right: r.right, top: r.top + scrollY, bottom: r.bottom + scrollY, width: r.width };
  };
  const prose = document.querySelector('.y-main > .y-article > .y-prose, .y-compare .y-prose');
  const rail = document.querySelector('.y-rail-left');
  const railOpen = rail && getComputedStyle(rail).visibility === 'visible' && getComputedStyle(rail).position === 'fixed';
  return {
    room: window.innerWidth,
    scroll: document.documentElement.scrollWidth,
    prose: prose ? box(prose) : null,
    rail: railOpen ? box(rail) : null,
    margins: [...document.querySelectorAll('.y-main > .y-article .y-margin')]
      .filter((m) => m.getClientRects().length > 0 && m.getBoundingClientRect().width > 0)
      .map((m) => ({ ...box(m), section: m.closest('.y-sect')?.textContent.trim() ?? '' })),
    blocks: [...document.querySelectorAll('.y-code')].map((c) => ({
      ...box(c),
      wide: (c.dataset.wide ?? '').split(' ').filter(Boolean),
      direct: c.parentElement.matches('.y-main > .y-article > .y-prose'),
      container: box(c.parentElement),
      section: (() => {
        let at = c;
        while (at && !at.matches?.('.y-sect')) at = at.previousElementSibling;
        return at ? at.textContent.trim() : '';
      })(),
    })),
  };
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`wide-blocks: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  const plant = async (page) => {
    if (!mode) return;
    const path = mode.css ? '**/static/app.css{,?*}' : `**${mode.js.path}{,?*}`;
    await page.route(path, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      requests += 1;
      if (mode.css) {
        planted += 1;
        await route.fulfill({ response, body: `${original}\n${mode.css}\n` });
        return;
      }
      // A needle that is not there exactly once leaves the script as served,
      // and the count below says the mutation never applied.
      const sites = original.split(mode.js.needle).length - 1;
      if (sites === 1) planted += 1;
      await route.fulfill({ response, body: sites === 1 ? original.replace(mode.js.needle, mode.js.replacement) : original });
    });
  };
  const open = async ({ width, rail = 'collapsed', size = 'm', js = true, options = {} }) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce', javaScriptEnabled: js, ...options });
    await context.addCookies([
      { name: 'yomihon_rail', value: rail, url: BASE },
      { name: 'yomihon_textsize', value: size, url: BASE },
      { name: 'yomihon_sectionbuttons', value: 'always', url: BASE },
    ]);
    const page = await context.newPage();
    await plant(page);
    return { context, page };
  };
  const visit = async (page, path) => {
    const response = await page.goto(BASE + path, { waitUntil: 'load' });
    if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
    await page.evaluate(() => document.fonts.ready);
  };
  const at = async (settings, path, fn) => {
    const { context, page } = await open(settings);
    try {
      await visit(page, path);
      return await fn(page);
    } finally {
      await context.close();
    }
  };

  // Wide only where needed. At a window wide enough to hold the wide measure
  // with the column folded, every block standing in the text is laid out once
  // more with its lines unwrapped, and the longest line the browser draws is
  // set against the room inside the text's measure. The block claims the
  // measure exactly when the line does not fit, and is drawn wider than the
  // text exactly when it claims it.
  if (runs('need')) {
    for (const [path, latin] of [[NOTE, false], [LATIN, true]]) {
      for (const size of SIZES) {
        const token = `${latin ? 'latin-' : ''}${size}`;
        await at({ width: 1920, size }, path, async (page) => {
          // The renderer counts a Han or kana character as one em, which is what
          // every reader's system face draws. A machine with no East Asian face
          // (a bare CI runner) draws them narrower, and there the browser's line
          // is no witness for those blocks; the count itself is held by the
          // renderer's own boundary test.
          const hanEm = await page.evaluate(() => {
            const pre = document.querySelector('.y-main > .y-article > .y-prose > .y-code pre');
            const probe = document.createElement('span');
            probe.textContent = '漢';
            pre.append(probe);
            const ratio = probe.getBoundingClientRect().width / Number.parseFloat(getComputedStyle(pre).fontSize);
            probe.remove();
            return ratio;
          });
          const eastAsianFace = hanEm > 0.95;
          const measured = await page.evaluate(() => [...document.querySelectorAll('.y-main > .y-article > .y-prose > .y-code')].map((block) => {
            const pre = block.querySelector('pre');
            const style = getComputedStyle(pre);
            const lines = [...pre.querySelectorAll('.line')];
            const was = pre.style.whiteSpace;
            pre.style.whiteSpace = 'pre';
            // A line's own box is as wide as the block (the highlighter lays
            // each out as a row), so what is measured is the text in it.
            const range = document.createRange();
            const longest = Math.max(...lines.map((line) => {
              range.selectNodeContents(line);
              const rects = [...range.getClientRects()];
              return rects.length ? Math.max(...rects.map((r) => r.right)) - Math.min(...rects.map((r) => r.left)) : 0;
            }));
            pre.style.whiteSpace = was;
            const prose = block.parentElement.getBoundingClientRect().width;
            return {
              lines: lines.length,
              longest,
              room: prose - Number.parseFloat(style.paddingLeft) - Number.parseFloat(style.paddingRight),
              prose,
              width: block.getBoundingClientRect().width,
              wide: (block.dataset.wide ?? '').split(' ').filter(Boolean),
              first: pre.textContent.split('\n')[0].slice(0, 40),
              eastAsian: /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}\uFF01-\uFF60\uFFE0-\uFFE6]/u.test(pre.textContent),
            };
          }));
          if (measured.length < 2) broken(`${path} has ${measured.length} blocks in its text`);
          let claimed = 0;
          for (const block of measured) {
            if (block.lines === 0) broken(`${path}: a block has no highlighted lines to measure: ${block.first}`);
            const needs = block.longest > block.room + 0.01;
            const claims = block.wide.includes(token);
            if (claims) claimed += 1;
            if (needs !== claims && (eastAsianFace || !block.eastAsian)) {
              fire('need', `${path} at ${size}: the block "${block.first}" draws a ${block.longest}px line in ${block.room}px of room but its data-wide is "${block.wide.join(' ')}"`);
            }
            if ((block.width > block.prose + 0.5) !== claims) {
              fire('need', `${path} at ${size}: the block "${block.first}" is ${block.width}px in a ${block.prose}px text, claiming "${block.wide.join(' ')}"`);
            }
          }
          if (!eastAsianFace && measured.some((block) => block.eastAsian)) console.log(`NOTE wide-blocks: no East Asian face here (Han draws ${hanEm.toFixed(2)}em); East Asian blocks are held by the renderer's count test`);
          if (claimed === 0 || claimed === measured.length) broken(`${path} at ${size}: ${claimed} of ${measured.length} blocks claim ${token}, so the boundary is not exercised`);
          const nested = (await read(page)).blocks.filter((block) => !block.direct);
          if (path === NOTE && !nested.some((block) => block.wide.length > 0)) broken(`${path} has no block in a callout that names a measure`);
          for (const block of nested) {
            if (block.left < block.container.left - 0.5 || block.right > block.container.right + 0.5) {
              fire('need', `${path} at ${size}: a block inside ${block.section || 'a container'} runs ${block.left}-${block.right}, outside its container ${block.container.left}-${block.container.right}`);
            }
          }
        });
      }
    }
    console.log('wide-blocks: every block claims the measures its longest line does not fit, in both measures at every size, and only those are drawn wide');
  }

  // At 1920 with the column folded the margin stands outside the wide lines,
  // so a wide block takes both wings: 1040px, centred on the text.
  if (runs('axis')) {
    for (const size of SIZES) {
      await at({ width: 1920, size }, NOTE, async (page) => {
        const got = await read(page);
        const wide = got.blocks.filter((block) => block.direct && block.wide.includes(size));
        if (wide.length === 0) broken(`no block claims ${size}`);
        const axis = (got.prose.left + got.prose.right) / 2;
        for (const block of wide) {
          if (!near(block.width, WIDE) || !near((block.left + block.right) / 2, axis)) {
            fire('axis', `at 1920 and ${size} a wide block runs ${block.left}-${block.right} (${block.width}px), want ${WIDE}px centred on the text's axis ${axis}`);
          }
        }
      });
    }
    console.log(`wide-blocks: at 1920 every wide block is ${WIDE}px and centred on the text at every size`);
  }

  // With the column open beside the page, a wide block stops 32px short of it,
  // or at the text's own edge where that is nearer.
  if (runs('rail')) {
    for (const width of [1321, 1440, 1512, 1720]) {
      for (const size of SIZES) {
        await at({ width, size, rail: 'open' }, NOTE, async (page) => {
          const got = await read(page);
          if (!got.rail) broken(`the column is not open beside the page at ${width}`);
          for (const block of got.blocks.filter((b) => b.direct && b.wide.includes(size))) {
            // A block out past the text's edge stops 32px short of the column;
            // where the text itself stands nearer than that, the block takes no
            // wing on that side at all.
            const limit = Math.min(got.prose.left, got.rail.right + RAIL_GAP);
            if (block.left < limit - 0.5) {
              fire('rail', `at ${width} and ${size} with the column open a wide block starts at ${block.left}, past ${limit}: the column ends at ${got.rail.right} and the text starts at ${got.prose.left}`);
            }
          }
        });
      }
    }
    console.log(`wide-blocks: with the column open a wide block stays ${RAIL_GAP}px clear of it at 1321, 1440, 1512 and 1720 at every size`);
  }

  // Never under the margin. At each width and size, open and folded, with and
  // without script, no wide block meets a margin; and wherever the margin
  // stands beside the text, every wide block ends before the margin's column
  // begins, wherever the margin's words fall down the page.
  if (runs('margin')) {
    const cases = [];
    for (const width of [1321, 1440, 1512, 1720, 1760, 2560]) {
      for (const size of SIZES) {
        for (const rail of ['open', 'collapsed']) cases.push({ width, size, rail, js: true });
      }
    }
    for (const width of [1440, 1512, 1760]) for (const size of SIZES) cases.push({ width, size, rail: 'collapsed', js: false });
    let beside = 0;
    for (const settings of cases) {
      await at(settings, NOTE, async (page) => {
        const got = await read(page);
        const named = `${settings.width} at ${settings.size}, column ${settings.rail}${settings.js ? '' : ', no script'}`;
        if (got.margins.length === 0) broken(`no margin is drawn at ${named}`);
        const wide = got.blocks.filter((block) => block.direct && block.wide.includes(settings.size));
        for (const margin of got.margins) {
          const besideText = margin.left >= got.prose.right;
          if (besideText) beside += 1;
          for (const block of wide) {
            const meets = block.left < margin.right - 0.5 && block.right > margin.left + 0.5 && block.top < margin.bottom - 0.5 && block.bottom > margin.top + 0.5;
            if (meets) fire('margin', `at ${named} the wide block under "${block.section}" (${block.left}-${block.right}, ${block.top}-${block.bottom}) meets the margin beside "${margin.section}" (${margin.left}-${margin.right}, ${margin.top}-${margin.bottom})`);
            if (besideText && block.right > margin.left - 0.5) fire('margin', `at ${named} the wide block under "${block.section}" ends at ${block.right}, into the margin's column from ${margin.left}`);
          }
        }
        // The case the rule exists for: at 1512 the thought beside the long
        // line's heading runs down beside the block's first lines.
        if (settings.width === 1512 && settings.size === 'm' && settings.js) {
          const margin = got.margins.find((m) => m.section.startsWith(LONG_SECTION));
          const block = wide.find((b) => b.section.startsWith(LONG_SECTION));
          if (!margin || !block) broken(`at 1512 the ${LONG_SECTION} section has no margin or no wide block`);
          if (!(block.top < margin.bottom && margin.top < block.bottom)) broken(`at 1512 the margin beside ${LONG_SECTION} (${margin.top}-${margin.bottom}) does not run down beside its block (${block.top}-${block.bottom}), so this proves nothing`);
        }
      });
    }
    if (beside === 0) broken('no margin stood beside the text at any width');
    console.log(`wide-blocks: no wide block meets a margin across ${cases.length} layouts, and every one ends before a margin standing beside the text`);
  }

  // A main narrower than 1120px sets every block at the text's width.
  if (runs('narrow')) {
    for (const width of [390, 1100]) {
      for (const path of [NOTE, LATIN]) {
        await at({ width }, path, async (page) => {
          const got = await read(page);
          for (const block of got.blocks.filter((b) => b.direct)) {
            if (block.width > got.prose.width + 0.5) fire('narrow', `${path} at ${width}: a block is ${block.width}px in a ${got.prose.width}px text`);
          }
        });
      }
    }
    console.log('wide-blocks: at 390 and 1100 every block is at the text\'s width');
  }

  // Nothing scrolls sideways.
  if (runs('sideways')) {
    for (const [width, rail] of [[390, 'collapsed'], [1100, 'collapsed'], [1512, 'open'], [2560, 'open'], [2560, 'collapsed']]) {
      for (const size of ['m', 'xl']) {
        for (const path of [NOTE, LATIN]) {
          await at({ width, rail, size }, path, async (page) => {
            const got = await read(page);
            if (got.scroll > got.room) fire('sideways', `${path} at ${width} and ${size} with the column ${rail} is ${got.scroll}px wide in a ${got.room}px window`);
          });
        }
      }
    }
    console.log('wide-blocks: nothing scrolls sideways at 390, 1100, 1512 with the column open and 2560');
  }

  // The text stands where it would with no wide block: every paragraph and
  // heading keeps its left and right edges when no block on the page claims a
  // measure any longer.
  if (runs('still')) {
    for (const [width, rail] of [[1512, 'collapsed'], [1512, 'open'], [1920, 'collapsed']]) {
      await at({ width, rail }, NOTE, async (page) => {
        const edges = () => page.evaluate(() => [...document.querySelectorAll('.y-main > .y-article > .y-prose > :is(p, .y-sect), .y-main > .y-article h1')]
          .map((element) => { const r = element.getBoundingClientRect(); return [r.left, r.right]; }));
        const served = await edges();
        await page.evaluate(() => { for (const block of document.querySelectorAll('[data-wide]')) block.removeAttribute('data-wide'); });
        const plain = await edges();
        if (served.length < 5) broken(`only ${served.length} paragraphs and headings to compare`);
        served.forEach(([left, right], i) => {
          if (!near(left, plain[i][0]) || !near(right, plain[i][1])) {
            fire('still', `at ${width} with the column ${rail} text element ${i} stands at ${left}-${right} with wide blocks and ${plain[i][0]}-${plain[i][1]} without`);
          }
        });
      });
    }
    console.log('wide-blocks: the text stands where it would with no wide block');
  }

  // The head: the control in it, above the code's first line, copying exactly
  // the code; and with no script a head with nothing to name is not drawn.
  if (runs('head')) {
    const { context, page } = await open({ width: 1512, options: { permissions: ['clipboard-read', 'clipboard-write'] } });
    try {
      await visit(page, NOTE);
      await page.waitForSelector('.y-code__head .y-codecopy [data-codecopy-button]');
      const placed = await page.evaluate(() => [...document.querySelectorAll('.y-main .y-code')].map((block) => {
        const head = block.querySelector(':scope > .y-code__head').getBoundingClientRect();
        const button = block.querySelector('[data-codecopy-button]')?.getBoundingClientRect() ?? null;
        const line = block.querySelector('pre .line')?.getClientRects()[0] ?? null;
        return { head: { top: head.top, bottom: head.bottom, left: head.left, right: head.right }, button: button && { top: button.top, bottom: button.bottom, left: button.left, right: button.right }, lineTop: line?.top ?? null };
      }));
      for (const [i, block] of placed.entries()) {
        if (!block.button) fire('head', `block ${i} has no copy control`);
        if (block.lineTop === null) broken(`block ${i} has no line to measure`);
        const inside = block.button.top >= block.head.top - 0.5 && block.button.bottom <= block.head.bottom + 0.5 && block.button.left >= block.head.left - 0.5 && block.button.right <= block.head.right + 0.5;
        if (!inside) fire('head', `block ${i}'s control (${JSON.stringify(block.button)}) is outside its head (${JSON.stringify(block.head)})`);
        if (block.button.bottom > block.lineTop + 0.5 || block.head.bottom > block.lineTop + 0.5) fire('head', `block ${i}'s head or control reaches ${Math.max(block.button.bottom, block.head.bottom)}, over its first line at ${block.lineTop}`);
      }
      const long = page.locator('.y-main > .y-article > .y-prose > .y-code').filter({ hasText: 'RunSerializable' }).first();
      await long.locator('[data-codecopy-button]').click();
      await long.locator('[data-codecopy-reply]').filter({ hasText: /Copied\.|已複製。/ }).waitFor();
      const copied = await page.evaluate(() => navigator.clipboard.readText());
      if (copied !== LONG_CODE) fire('head', `the control copied ${JSON.stringify(copied)}, want the authored ${JSON.stringify(LONG_CODE)}`);
    } finally {
      await context.close();
    }
    for (const js of [true, false]) {
      await at({ width: 1512, js }, NOTE, async (page) => {
        const heads = await page.evaluate(() => [...document.querySelectorAll('.y-main .y-code > .y-code__head')].map((head) => ({
          shown: head.getClientRects().length > 0,
          height: head.getBoundingClientRect().height,
          children: head.children.length,
          lang: head.querySelector('.y-code__lang') ? getComputedStyle(head.querySelector('.y-code__lang'), '::before').content : null,
        })));
        const bare = heads.filter((head) => head.lang === null);
        if (bare.length === 0) broken('no block without a language to look at');
        for (const head of heads) {
          if (head.lang !== null && head.lang !== '"go"') fire('head', `a head names ${head.lang}, want "go"`);
          const want = js || head.lang !== null;
          if (head.shown !== want) fire('head', `with${js ? '' : 'out'} script a head ${head.lang ?? 'with no language'} is ${head.shown ? '' : 'not '}drawn`);
          if (head.shown && head.height !== 36) fire('head', `a head is ${head.height}px tall, want 36`);
          if (head.children !== (head.lang === null ? 1 : 2)) fire('head', `a head holds ${head.children} elements with${js ? '' : 'out'} script, so the page's elements change when script runs`);
        }
      });
    }
    console.log('wide-blocks: every control stands in its head above the first line and copies exactly the code; with no script a head with no language is not drawn');
  }

  // Faint where a pointer can rest, whole when it does or when focus is inside,
  // and whole on a touch screen.
  if (runs('faint')) {
    const look = (page) => page.evaluate(() => {
      const button = document.querySelector('.y-main .y-code [data-codecopy-button]');
      const ink = (value) => {
        const probe = document.createElement('span');
        probe.style.color = value;
        document.body.append(probe);
        const colour = getComputedStyle(probe).color;
        probe.remove();
        return colour;
      };
      const style = getComputedStyle(button);
      const control = getComputedStyle(button.closest('.y-codecopy'));
      return { color: style.color, ring: style.boxShadow, opacity: Number(control.opacity) * Number(style.opacity), visible: button.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true }), faint: ink('var(--fg-faint)'), muted: ink('var(--fg-muted)') };
    });
    const whole = (got) => got.color === got.muted && got.ring !== 'none' && got.opacity === 1;
    await at({ width: 1512 }, NOTE, async (page) => {
      await page.waitForSelector('.y-code__head [data-codecopy-button]');
      await page.mouse.move(5, 300);
      const rest = await look(page);
      if (!rest.visible || rest.opacity !== 1) fire('faint', `at rest the copy control is not drawn (opacity ${rest.opacity})`);
      if (rest.color !== rest.faint || rest.ring !== 'none') fire('faint', `at rest the copy control is ${rest.color} with ${rest.ring}, want the faint ink ${rest.faint} and no ring`);
      await page.locator('.y-main .y-code').first().hover();
      const hovered = await look(page);
      if (!whole(hovered)) fire('faint', `with the pointer on the block the control is ${JSON.stringify(hovered)}, want it whole`);
      await page.mouse.move(5, 300);
      await page.locator('.y-main .y-code [data-codecopy-button]').first().focus();
      const focused = await look(page);
      if (!whole(focused)) fire('faint', `with focus on the control it is ${JSON.stringify(focused)}, want it whole`);
    });
    await at({ width: 1512, options: { hasTouch: true } }, NOTE, async (page) => {
      if (!await page.evaluate(() => matchMedia('(pointer: coarse)').matches)) broken('the touch context has no coarse pointer');
      await page.waitForSelector('.y-code__head [data-codecopy-button]');
      const touch = await look(page);
      if (!whole(touch)) fire('faint', `on a touch screen the control at rest is ${JSON.stringify(touch)}, want it whole`);
    });
    console.log('wide-blocks: the control is faint at rest, whole under the pointer and with focus inside, and whole on a touch screen');
  }

  // The compare page: two notes in two columns, every block at its column's
  // width.
  if (runs('compare')) {
    await at({ width: 1920 }, COMPARE, async (page) => {
      const blocks = await page.evaluate(() => [...document.querySelectorAll('.y-compare__column .y-code')].map((block) => {
        const prose = block.closest('.y-prose').getBoundingClientRect();
        const r = block.getBoundingClientRect();
        return { left: r.left, right: r.right, proseLeft: prose.left, proseRight: prose.right, wide: block.dataset.wide ?? '' };
      }));
      if (!blocks.some((block) => block.wide.split(' ').includes('m'))) broken('the compare page holds no block that names the base measure');
      for (const block of blocks) {
        if (block.left < block.proseLeft - 0.5 || block.right > block.proseRight + 0.5) {
          fire('compare', `a block in a compare column runs ${block.left}-${block.right}, outside its column's text ${block.proseLeft}-${block.proseRight}`);
        }
      }
    });
    console.log('wide-blocks: the compare page keeps every block at its column\'s width');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED wide-blocks: the mutation reached ${planted} of ${requests} requests`);
  console.log('PASS wide-blocks: blocks are wide exactly where their lines need it, span the wide lines on the text\'s axis, keep clear of the open column and the margin at every width and size with and without script, fall back to the text\'s width on a narrow window and in the compare columns, move no text and scroll nothing sideways; the copy control stands in the head above the code, copies only the code, and is faint only where a pointer can rest');
} catch (error) {
  failure = error;
} finally {
  await browser.close();
}
if (failure) {
  console.error(failure.message);
  if (failure instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && failure instanceof LockFired && failure.group === MODES[MUTATE].group) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
}
