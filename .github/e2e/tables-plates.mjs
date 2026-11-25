// Behavior lock for a table set as a book sets it and for the plate a picture
// or a diagram stands on.
//
// A table is ruled in booktabs: the heavier rule over it and under it, the
// lighter one under its header, the faintest between rows, and nothing down
// its columns or round its outside. A column is aligned as its delimiter row
// says. The row under the pointer is washed where a pointer can rest, and only
// there. A table wider than its room scrolls inside its wrapper, which is then a
// stop in the tab order that a key scrolls, and is no stop while it fits.
//
// A picture alone in its paragraph and a diagram stand on a plate: the mat's
// tone, a faint ring and the small radius. Under it stands a caption where the
// author gave one, and nowhere else; the caption is under what it names and on
// its plate. A drawing's plate takes the system's light or dark, a raster
// picture's and a diagram's the page's, and the diagram itself is drawn in the
// page's theme. A diagram that runs down the page keeps the text's width; one
// that runs across stands at the wide measure.
//
// The wide measure itself, for tables and plates as for code, is wide-blocks'.
//
// Env: YOMIHON_BASE, PAGE_PATH and MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/tables-plates.md';
const MUTATE = process.env.MUTATE || '';
const WIDE = 1040;

// The tones are written out, not read from the stylesheet: a lock that took its
// expected colour from the rule under test would agree with any rule.
const MAT_TONE = { light: 'rgb(236, 232, 223)', dark: 'rgb(27, 27, 24)' };

// What the fixture's plates are, in order, and the caption each was given.
const PLATES = [
  { kind: 'drawing', scheme: 'system', caption: '兩筆交易的讀寫依賴環' },
  { kind: 'drawing', scheme: 'system', caption: null },
  { kind: 'raster', scheme: 'page', caption: '一張點陣圖' },
  { kind: 'diagram', scheme: 'page', caption: '兩筆交易各自讀取對方即將改寫的條件', wide: true },
  { kind: 'diagram', scheme: 'page', caption: null, wide: false },
];

const css = (rule, site) => ({ css: rule, site });
const MODES = {
  // An outer box drawn round the table.
  'box-the-table': css('.y-prose table{border:1px solid var(--line-strong) !important}', 'booktabs'),
  // A rule drawn down every column.
  'rule-the-columns': css('.y-prose :is(th, td){border-inline:1px solid var(--line) !important}', 'booktabs'),
  // The heavy rule over the table gone.
  'drop-the-top-rule': css('.y-prose .y-tablewrap{box-shadow:inset 0 -1.5px 0 var(--rule-double) !important}', 'booktabs'),
  // The heavy rule under the table gone.
  'drop-the-bottom-rule': css('.y-prose .y-tablewrap{box-shadow:inset 0 1.5px 0 var(--rule-double) !important}', 'booktabs'),
  // The rule under the header as heavy as the frame's.
  'weigh-the-header-rule': css('.y-prose th{box-shadow:inset 0 -1.5px 0 var(--rule-double) !important}', 'booktabs'),
  // The header wrapping onto a second line.
  'wrap-the-header': css('.y-prose th{white-space:normal !important}', 'booktabs'),
  // Every column set flush left, whatever its delimiter row said.
  'left-align-every-cell': css('.y-prose :is(th, td){text-align:left !important}', 'align'),
  // The row under the pointer left unwashed.
  'no-row-wash': css('@media (hover: hover){.y-prose tbody tr:hover{background:none !important}}', 'hover'),
  // A row washed where no pointer can rest.
  'wash-rows-on-touch': css('.y-prose tbody tr:hover{background:var(--wash-strong) !important}', 'hover'),
  // A table that scrolls given no place in the tab order.
  'no-table-stop': { js: { path: '/static/diagrams.js', needle: "document.querySelectorAll('.mat, .y-tablewrap')", replacement: "document.querySelectorAll('.mat')" }, site: 'scroll' },
  // The plate's ring gone, so nothing but tone sets it into the page.
  'flat-plate': css('.y-prose .mat{box-shadow:none !important}', 'plate'),
  // A caption on a plate whose author gave none.
  'caption-every-plate': { page: ['<img src="/raw/Notes/pic.svg" alt="pic.svg" width="120"></figure>', '<img src="/raw/Notes/pic.svg" alt="pic.svg" width="120"><figcaption aria-hidden="true">pic.svg</figcaption></figure>'], site: 'plate' },
  // The caption hidden.
  'hide-the-caption': css('.y-prose figcaption{display:none !important}', 'plate'),
  // A diagram that runs down the page set wide too.
  'widen-every-plate': css('.y-main > .y-article > .y-prose > .mat{margin-inline:calc(-1 * var(--wing-start)) !important}', 'plate'),
  // A diagram's plate in the system's light or dark rather than the page's.
  'diagram-plate-follows-system': css('.y-prose .mat--page{color-scheme:light dark !important}', 'theme'),
  // A diagram drawn in the light theme whatever the page is.
  'diagram-drawn-in-one-theme': { js: { path: '/static/diagrams.js', needle: "theme: preferences.theme() === 'dark' ? 'dark' : 'default',", replacement: "theme: 'default'," }, site: 'theme' },
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`tables-plates: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL tables-plates: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN tables-plates: ${message}`); };
const fire = (site, message) => { throw new LockFired(site, message); };
const runs = (site) => !MUTATE || MODES[MUTATE].site === site;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`tables-plates: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  // Each mode rewrites the one response it names; a needle that is not there
  // exactly once leaves the response as served, and the count below says the
  // mutation never applied.
  const plant = async (page) => {
    if (!mode) return;
    const path = mode.css ? '**/static/app.css{,?*}' : mode.js ? `**${mode.js.path}{,?*}` : `**${PAGE}`;
    await page.route(path, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      requests += 1;
      if (mode.css) {
        planted += 1;
        return route.fulfill({ response, body: `${original}\n${mode.css}\n` });
      }
      const [needle, replacement] = mode.js ? [mode.js.needle, mode.js.replacement] : mode.page;
      const sites = original.split(needle).length - 1;
      if (sites === 1) planted += 1;
      return route.fulfill({ response, body: sites === 1 ? original.replace(needle, replacement) : original });
    });
  };
  const at = async ({ width, theme = 'light', system = 'light', options = {} }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: system, reducedMotion: 'reduce', ...options });
    await context.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
    ]);
    const page = await context.newPage();
    try {
      await plant(page);
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (response?.status() !== 200) broken(`${PAGE} answered ${response?.status()}, want 200`);
      await page.evaluate(() => document.fonts.ready);
      await arrived(page);
      if (!(await page.content()).includes('TABLES-END')) broken('the fixture page did not arrive whole');
      return await fn(page);
    } finally {
      await context.close();
    }
  };
  // A colour as the page resolves it, in the light or dark of element.
  const resolve = (page, selector, value) => page.evaluate(([selector, value]) => {
    const probe = document.createElement('span');
    probe.style.color = value;
    document.querySelector(selector).append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, [selector, value]);

  // Booktabs: the frame's two rules on the wrapper and nothing at its sides;
  // nothing round the table; a header rule lighter than the frame's and
  // heavier than nothing; a faint rule under every row but the last; nothing
  // down the columns; the header on one line. The frame's 1.5px and the
  // header's 0.75px are inset shadows, since a border is drawn in whole
  // pixels; the row rules are borders.
  if (runs('booktabs')) {
    await at({ width: 1280 }, async (page) => {
      const doubled = await resolve(page, '.y-prose', 'var(--rule-double)');
      const strong = await resolve(page, '.y-prose', 'var(--line-strong)');
      const faint = await resolve(page, '.y-prose', 'var(--line-faint)');
      const tables = await page.evaluate(() => [...document.querySelectorAll('.y-prose .y-tablewrap')].map((wrap) => {
        const side = (style, edge) => ({ width: Number.parseFloat(style[`border${edge}Width`]), style: style[`border${edge}Style`], color: style[`border${edge}Color`] });
        const edges = (element) => {
          const style = getComputedStyle(element);
          return { top: side(style, 'Top'), right: side(style, 'Right'), bottom: side(style, 'Bottom'), left: side(style, 'Left') };
        };
        // Each inset shadow a box draws, by its colour and vertical offset.
        const shadows = (element) => {
          const value = getComputedStyle(element).boxShadow;
          if (value === 'none') return [];
          return value.split(/,(?![^()]*\))/).map((part) => {
            const found = part.trim().match(/^(.*\))\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+inset$/);
            return found ? { color: found[1], x: Number(found[2]), y: Number(found[3]), blur: Number(found[4]), spread: Number(found[5]) } : { unread: part };
          });
        };
        const table = wrap.querySelector('table');
        const rows = [...table.rows];
        return {
          wrap: edges(wrap),
          frame: shadows(wrap),
          table: edges(table),
          header: [...rows[0].cells].map((cell) => ({ ...edges(cell), rule: shadows(cell), wrap: getComputedStyle(cell).whiteSpace })),
          body: rows.slice(1).map((row, i, all) => ({ last: i === all.length - 1, cells: [...row.cells].map(edges) })),
        };
      }));
      if (tables.length < 5) broken(`${PAGE} holds ${tables.length} tables`);
      const drawn = (edge) => edge.width > 0 && edge.style !== 'none';
      for (const [i, table] of tables.entries()) {
        for (const [edge, y] of [['top', 1.5], ['bottom', -1.5]]) {
          const rule = table.frame.find((shadow) => shadow.y === y);
          if (!rule || rule.x !== 0 || rule.blur !== 0 || rule.spread !== 0 || rule.color !== doubled) fire('booktabs', `table ${i}'s ${edge} rule is ${JSON.stringify(table.frame)}, want a 1.5px line in ${doubled}`);
        }
        if (table.frame.length !== 2) fire('booktabs', `table ${i}'s wrapper draws ${JSON.stringify(table.frame)}, want its two rules and nothing else`);
        for (const [what, edges] of [['wrapper', table.wrap], ['table', table.table]]) {
          for (const edge of ['top', 'right', 'bottom', 'left']) {
            if (drawn(edges[edge])) fire('booktabs', `table ${i}'s ${what} draws a ${edge} edge ${JSON.stringify(edges[edge])}, want no box round the table`);
          }
        }
        for (const cell of table.header) {
          const [rule, ...more] = cell.rule;
          if (!rule || more.length || rule.y !== -0.75 || rule.x !== 0 || rule.blur !== 0 || rule.color !== strong || drawn(cell.bottom)) fire('booktabs', `table ${i}'s header rule is ${JSON.stringify(cell.rule)} over a ${JSON.stringify(cell.bottom)} border, want only a 0.75px line in ${strong}, lighter than the frame's 1.5px`);
          if (cell.wrap !== 'nowrap') fire('booktabs', `table ${i}'s header is set ${cell.wrap}, want it on one line`);
        }
        for (const row of table.body) {
          for (const cell of row.cells) {
            if (drawn(cell.left) || drawn(cell.right)) fire('booktabs', `table ${i} draws a rule down a column (${JSON.stringify([cell.left, cell.right])})`);
            const ruled = drawn(cell.bottom);
            if (ruled === row.last) fire('booktabs', `table ${i}'s ${row.last ? 'last row is' : 'row is not'} ruled under it`);
            if (ruled && (cell.bottom.width !== 1 || cell.bottom.color !== faint)) fire('booktabs', `table ${i}'s row rule is ${JSON.stringify(cell.bottom)}, want 1px in ${faint}`);
          }
        }
        for (const cell of table.header) {
          if (drawn(cell.left) || drawn(cell.right)) fire('booktabs', `table ${i}'s header draws a rule down a column`);
        }
      }
    });
    console.log('tables-plates: every table is ruled over, under and below its header and between its rows, with nothing down its columns or round it, and its header on one line');
  }

  // Alignment: a column the delimiter row set right ends its figures on one
  // line, and one set centre centres them.
  if (runs('align')) {
    await at({ width: 1280 }, async (page) => {
      const columns = await page.evaluate(() => {
        const table = [...document.querySelectorAll('.y-prose table')].find((t) => t.rows[0].cells[0].textContent.trim() === '隔離等級' && t.rows[0].cells.length === 4);
        return [...table.rows[0].cells].map((_, c) => [...table.rows].map((row) => {
          const cell = row.cells[c];
          const range = document.createRange();
          range.selectNodeContents(cell);
          const text = range.getBoundingClientRect();
          const box = cell.getBoundingClientRect();
          const style = getComputedStyle(cell);
          return { textLeft: text.left, textRight: text.right, left: box.left + Number.parseFloat(style.paddingLeft), right: box.right - Number.parseFloat(style.paddingRight) };
        }));
      });
      const [level, count, aborts, share] = columns;
      for (const [name, column] of [['交易數', count], ['中止', aborts]]) {
        for (const cell of column) {
          if (Math.abs(cell.textRight - cell.right) > 1) fire('align', `the right-aligned column ${name} ends a cell's text at ${cell.textRight}, not at its edge ${cell.right}`);
        }
      }
      for (const cell of share) {
        if (Math.abs((cell.textLeft + cell.textRight) / 2 - (cell.left + cell.right) / 2) > 1) fire('align', `the centred column 比例 is not centred: ${JSON.stringify(cell)}`);
      }
      for (const cell of level) {
        if (Math.abs(cell.textLeft - cell.left) > 1) fire('align', `the left-aligned column starts a cell's text at ${cell.textLeft}, not at its edge ${cell.left}`);
      }
    });
    console.log('tables-plates: each column is aligned as its delimiter row says');
  }

  // The wash under the pointer, where a pointer can rest, on body rows only.
  if (runs('hover')) {
    const washed = async (page, selector) => {
      await page.locator(selector).first().hover();
      return page.evaluate((selector) => getComputedStyle(document.querySelector(selector).closest('tr')).backgroundColor, selector);
    };
    const row = '.y-main > .y-article > .y-prose > .y-tablewrap tbody td';
    const header = '.y-main > .y-article > .y-prose > .y-tablewrap thead th';
    await at({ width: 1280 }, async (page) => {
      const wash = await resolve(page, '.y-prose', 'var(--wash)');
      const got = await washed(page, row);
      if (got !== wash) fire('hover', `with the pointer on a row it is ${got}, want the wash ${wash}`);
      const head = await washed(page, header);
      if (head !== 'rgba(0, 0, 0, 0)') fire('hover', `with the pointer on the header it is ${head}, want no wash`);
    });
    await at({ width: 1280, options: { hasTouch: true } }, async (page) => {
      if (await page.evaluate(() => matchMedia('(hover: hover)').matches)) broken('the touch context reports a pointer that can rest');
      await page.locator(row).first().tap();
      const got = await page.evaluate((selector) => getComputedStyle(document.querySelector(selector).closest('tr')).backgroundColor, row);
      if (got !== 'rgba(0, 0, 0, 0)') fire('hover', `on a touch screen a row a finger landed on is ${got}, want no wash`);
    });
    console.log('tables-plates: a body row is washed under a resting pointer and never on a touch screen');
  }

  // A table wider than its room scrolls inside its wrapper, which is then a
  // stop a key scrolls; one that fits is no stop.
  if (runs('scroll')) {
    for (const width of [390, 1100]) {
      await at({ width }, async (page) => {
        await page.waitForFunction(() => document.documentElement.dataset.js === 'on');
        await page.waitForTimeout(200);
        const wraps = await page.evaluate(() => [...document.querySelectorAll('.y-prose .y-tablewrap')].map((wrap, i) => {
          wrap.dataset.probeWrap = String(i);
          return { i, scrolls: wrap.scrollWidth - wrap.clientWidth > 1, stop: wrap.getAttribute('tabindex') === '0' };
        }));
        if (!wraps.some((wrap) => wrap.scrolls)) broken(`at ${width} no table scrolls`);
        if (width === 1100 && !wraps.some((wrap) => !wrap.scrolls)) broken(`at ${width} every table scrolls`);
        if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)) fire('scroll', `at ${width} the page scrolls sideways`);
        for (const wrap of wraps) {
          if (wrap.stop !== wrap.scrolls) fire('scroll', `at ${width} table ${wrap.i} ${wrap.scrolls ? 'scrolls but is no stop' : 'fits but is a stop'} in the tab order`);
        }
        const target = wraps.find((wrap) => wrap.scrolls);
        await page.focus(`[data-probe-wrap="${target.i}"]`);
        const before = await page.evaluate((i) => document.querySelector(`[data-probe-wrap="${i}"]`).scrollLeft, target.i);
        await page.keyboard.press('ArrowRight');
        await page.waitForTimeout(150);
        const after = await page.evaluate((i) => document.querySelector(`[data-probe-wrap="${i}"]`).scrollLeft, target.i);
        if (after <= before) fire('scroll', `at ${width} a key pressed on table ${target.i} did not scroll it (${before} to ${after})`);
      });
    }
    console.log('tables-plates: at 390 and 1100 a table that does not fit scrolls inside its wrapper, which is a stop a key scrolls, and one that fits is no stop');
  }

  // The plates: ground, ring and radius, a caption exactly where the author
  // gave one, under what it names and on its plate, and the diagram that runs
  // down at the text's width.
  if (runs('plate')) {
    await at({ width: 1920 }, async (page) => {
      await page.waitForFunction(() => [...document.querySelectorAll('.mermaid-diagram')].every((block) => block.querySelector('svg') || block.hasAttribute('data-mermaid-error')));
      const plates = await page.evaluate(() => {
        const prose = document.querySelector('.y-main > .y-article > .y-prose').getBoundingClientRect();
        return [...document.querySelectorAll('.y-main > .y-article > .y-prose > .mat')].map((plate) => {
          const style = getComputedStyle(plate);
          const caption = plate.querySelector(':scope > figcaption');
          const held = plate.firstElementChild.getBoundingClientRect();
          const box = plate.getBoundingClientRect();
          const captionBox = caption?.getBoundingClientRect();
          return {
            ring: style.boxShadow,
            radius: style.borderRadius,
            width: box.width,
            prose: prose.width,
            caption: caption && captionBox.height > 0 ? caption.textContent : null,
            hidden: caption?.getAttribute('aria-hidden') ?? null,
            under: captionBox ? captionBox.top >= held.bottom - 0.5 && captionBox.bottom <= box.bottom + 0.5 && captionBox.left >= box.left && captionBox.right <= box.right : null,
          };
        });
      });
      if (plates.length !== PLATES.length) broken(`${PAGE} holds ${plates.length} plates, want ${PLATES.length}`);
      for (const [i, plate] of plates.entries()) {
        const want = PLATES[i];
        if (!/inset/.test(plate.ring) || plate.radius !== '4px') fire('plate', `plate ${i} has ring ${plate.ring} and radius ${plate.radius}, want a faint inset ring and 4px`);
        if (plate.caption !== want.caption) fire('plate', `plate ${i} is captioned ${JSON.stringify(plate.caption)}, want ${JSON.stringify(want.caption)}`);
        if (want.caption && (plate.hidden !== 'true' || !plate.under)) fire('plate', `plate ${i}'s caption is not under what it names on its plate, hidden from a screen reader (${JSON.stringify(plate)})`);
        if (want.kind === 'diagram' && Math.abs(plate.width - (want.wide ? WIDE : plate.prose)) > 0.5) fire('plate', `the diagram on plate ${i} is ${plate.width}px, want ${want.wide ? WIDE : plate.prose}`);
      }
    });
    console.log('tables-plates: every plate has its ring and radius, a caption exactly where its author gave one, under what it names, and a diagram that runs down keeps the text\'s width');
  }

  // Each plate takes the light or dark of what it holds, and the diagram is
  // drawn in the page's theme, with the system and the page disagreeing both
  // ways.
  if (runs('theme')) {
    const fills = {};
    for (const [system, theme] of [['light', 'dark'], ['dark', 'light']]) {
      await at({ width: 1280, system, theme }, async (page) => {
        await page.waitForFunction(() => [...document.querySelectorAll('.mermaid-diagram')].every((block) => block.querySelector('svg')));
        const seen = await page.evaluate(() => ({
          grounds: [...document.querySelectorAll('.y-main > .y-article > .y-prose > .mat')].map((plate) => getComputedStyle(plate).backgroundColor),
          fill: getComputedStyle(document.querySelector('.mermaid-diagram svg .node rect, .mermaid-diagram svg .node polygon, .mermaid-diagram svg .node path')).fill,
        }));
        for (const [i, ground] of seen.grounds.entries()) {
          const want = MAT_TONE[PLATES[i].scheme === 'system' ? system : theme];
          if (ground !== want) fire('theme', `with a ${system} system and a ${theme} page, plate ${i} (${PLATES[i].kind}) is ${ground}, want the ${PLATES[i].scheme}'s ${want}`);
        }
        fills[theme] = seen.fill;
      });
    }
    if (fills.light === fills.dark) fire('theme', `the diagram's shapes are ${fills.light} on a light page and on a dark one, so the drawing does not follow the page`);
    console.log('tables-plates: a drawing\'s plate takes the system\'s light or dark, a picture\'s and a diagram\'s the page\'s, and the diagram is drawn in the page\'s theme');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED tables-plates: the mutation reached ${planted} of ${requests} requests`);
  console.log('PASS tables-plates: tables are ruled in booktabs, aligned as written, washed only under a resting pointer, and scroll inside a wrapper a key can reach; plates carry their ring, take the light or dark of what they hold, and are captioned only where the author gave words');
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
    if (MUTATE && failure instanceof LockFired && failure.site === MODES[MUTATE].site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
}
