// Behavior lock for the settings page as the Atelier lays it out: a bench of
// two columns, the choices on the left at their own width and beside them a
// sheet of reading that the choices are drawn on.
//
// The sheet is the point, and what it must never do is say something a note
// would not. So it is read against a note under the same stored choices — the
// face, the size and the line of a paragraph, the same of a section's heading
// and of the code in the paragraph, the ink, and the ground — and the two have
// to agree to the pixel, in light and dark, on each paper, at each size, in
// each face, and in English as well as Chinese. It moves the moment a choice
// is picked, with nothing sent and nowhere gone. Where no script runs it still
// stands, drawn in the choices the page was drawn with. The buttons beside its
// heading come and go as a note's do on the same window. It is hidden from
// assistive technology and nothing in it takes focus, because what it shows
// the choices already say.
//
// The sheet is captioned a preview in the interface's small print, on a faint
// hairline over the reading and readable at 4.5:1 in light and dark, script or
// none. Below the language, which stands first and alone, the choices stand in
// two groups — the paper and type, then how the page behaves while reading —
// each headed as the page heads its sections: a small heading in the
// interface's face over the double rule.
//
// Around it: on a wide window the sheet stands beside the column and stays in
// view while the column scrolls; below 1081 it stands above the choices and
// scrolls away with them, and nothing scrolls sideways on a phone. The paper's
// row goes while the theme picked is the night lamp or the system's dark, with
// or without a script, and comes back by day.
//
// Env: YOMIHON_BASE, PAGE_PATH (the settings page) and MUTATE (MUTATE=list
// prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/preferences';
const MUTATE = process.env.MUTATE || '';
// A note in the interface's own Chinese with a section, a paragraph and code
// in it, and one tagged English whose sections carry the buttons beside them.
const NOTE_CJK = '/notes/Notes/prose-type.md';
const NOTE_LATIN = '/notes/Notes/Reading%20flow.md';

const css = (rule, site) => ({ css: rule, site });
const html = (pattern, replacement, site) => ({ html: [pattern, replacement], site });
const js = (needle, replacement, site) => ({ js: [needle, replacement], site });
const MODES = {
  // The two columns run together into one on a wide window.
  'one-column-on-a-wide-window': css('.y-prefs__bench{grid-template-columns:minmax(0,1fr) !important}', 'bench'),
  // The sheet scrolling away beside a column the reader is still working down.
  'sheet-scrolls-away-beside': css('.y-prefs__sheet{position:static !important}', 'bench'),
  // The sheet pinned to the top of a narrow window, over the choices.
  'sheet-sticks-when-stacked': css('@media (max-width:1080px){.y-prefs__sheet{position:sticky !important;top:0 !important}}', 'stacked'),
  // The sheet put under the choices it shows.
  'sheet-under-the-choices': css('@media (max-width:1080px){.y-prefs__sheet{order:1 !important}}', 'stacked'),
  // The sheet wider than a phone.
  'sheet-past-the-phone': css('@media (max-width:720px){.y-prefs__sheet{min-width:480px !important}}', 'stacked'),
  // The sheet setting its own size, face, line or ground instead of a note's.
  'sheet-sets-its-own-size': css('.y-prefs__sheet .y-prose{font-size:17px !important}', 'same-reading'),
  'sheet-sets-its-own-face': css('.y-prefs__sheet .y-prose{font-family:var(--font-serif) !important}', 'same-reading'),
  'sheet-on-its-own-ground': css('.y-prefs__sheet{background:var(--elevated) !important}', 'same-reading'),
  // The sheet's Latin line written as the bare factor, a fraction at 大 and 特大.
  'sheet-line-a-fraction': css('.y-prefs__sheet .y-prose{line-height:1.7647 !important}', 'same-reading'),
  'sheet-keeps-the-han-line-in-english': css('.y-prefs__sheet .y-prose{line-height:var(--lh-cjk) !important}', 'same-reading'),
  // A pick that stores the choice but leaves the root, and so the sheet, as
  // it was until the next page.
  'pick-leaves-the-root': js('    root.dataset[name] = value;\n', '', 'instant'),
  // The paper's row standing in the dark, or gone by day.
  'paper-row-in-the-dark': css(".y-prefs form [data-pref-field='paper']{display:grid !important}", 'paper-row'),
  'paper-row-gone-by-day': css("[data-pref-field='paper']{display:none !important}", 'paper-row'),
  // The buttons beside the sheet's heading drawn whatever the choice, or never.
  'sheet-buttons-always-drawn': css('.y-prefs__sheet .y-margin__act{opacity:1 !important}', 'section-buttons'),
  'sheet-buttons-never-drawn': css('.y-prefs__sheet .y-margin__act{opacity:0 !important}', 'section-buttons'),
  // Every paper's chip on one ground, or a ground written out instead of read
  // from the paper.
  'chips-on-one-ground': css('.y-papersample{background:var(--panel) !important}', 'chips'),
  'chip-ground-written-out': css(".y-papersample[data-paper='washi']{background:#fff !important}", 'chips'),
  // The caption gone, or fainter than text may be.
  'caption-gone': css('.y-prefs__caption{display:none !important}', 'caption'),
  'caption-too-faint': css('.y-prefs__caption{color:var(--fg-ornament) !important}', 'caption'),
  // A group's heading with no rule under it, or set in the reading face.
  'group-head-without-rule': css('.y-prefs__group h2::after{content:none !important}', 'groups'),
  'group-head-in-the-reading-face': css('.y-prefs__group h2{font-family:var(--font-serif) !important;font-size:17px !important}', 'groups'),
  // The language moved into the first group.
  'language-in-a-group': html(/(<fieldset class="y-preffield" data-pref-field="lang">[\s\S]*?<\/fieldset>)(<section class="y-prefs__group"[^>]*><h2[^>]*>[^<]*<\/h2>)/, '$2$1', 'groups'),
  // The sheet put away where no script runs.
  'sheet-needs-a-script': css(':root:not([data-js]) .y-prefs__sheet{display:none !important}', 'no-script'),
  // The sheet read out, or a stop on the way through the page.
  'sheet-read-out': html(/(<div class="y-prefs__sheet"[^>]*?) aria-hidden="true"/, '$1', 'quiet'),
  'sheet-takes-focus': html(/<span class="y-margin__act"/, '<span tabindex="0" class="y-margin__act"', 'quiet'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`preference-sheet: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL preference-sheet: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN preference-sheet: ${message}`); };
const fire = (site, message) => { throw new LockFired(site, message); };
const runs = (site) => !MUTATE || MODES[MUTATE].site === site;

// How a piece of reading is set: a paragraph, the section heading above it,
// the code in the paragraph where it has some, and the ground it lies on. In a
// note the paragraph is one of the article's own, one carrying code where
// there is one; in the settings page it is the sheet's. The English notes the
// fixture carries have no code in a paragraph, so code is compared where the
// note has it.
const readSetting = (scope) => {
  const paragraphs = [...document.querySelectorAll(`${scope} .y-prose > p`)];
  const paragraph = paragraphs.find((p) => p.querySelector('code')) || paragraphs[0];
  const heading = document.querySelector(`${scope} .y-prose > .y-sect > [data-level='2']`);
  if (!paragraph || !heading) return null;
  const p = getComputedStyle(paragraph);
  const h = getComputedStyle(heading);
  const code = paragraph.querySelector('code');
  const c = code ? getComputedStyle(code) : { fontFamily: null, fontSize: null };
  const groundOf = scope === '.y-prefs__sheet' ? document.querySelector(scope) : document.body;
  return {
    face: p.fontFamily, size: p.fontSize, line: p.lineHeight, ink: p.color,
    headingFace: h.fontFamily, headingSize: h.fontSize, headingLine: h.lineHeight,
    codeFace: c.fontFamily, codeSize: c.fontSize,
    ground: getComputedStyle(groundOf).backgroundColor,
  };
};

const cookieList = (stored) => Object.entries(stored).map(([name, value]) => ({ name: `yomihon_${name}`, value, url: BASE }));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`preference-sheet: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  const at = async ({ path = PAGE, width = 1512, scheme = 'light', stored = {}, script = true }, fn) => {
    const context = await browser.newContext({
      viewport: { width, height: 900 },
      colorScheme: scheme,
      reducedMotion: 'reduce',
      javaScriptEnabled: script,
    });
    await context.addCookies(cookieList({ rail: 'collapsed', ...stored }));
    const page = await context.newPage();
    try {
      if (mode?.css) {
        await page.route('**/static/app.css{,?*}', async (route) => {
          const response = await route.fetch();
          planted += 1;
          return route.fulfill({ response, body: `${await response.text()}\n${mode.css}\n` });
        });
      }
      if (mode?.html) {
        await page.route((url) => url.pathname === '/preferences', async (route) => {
          if (route.request().method() !== 'GET' || route.request().resourceType() !== 'document') return route.continue();
          const response = await route.fetch();
          const original = await response.text();
          const changed = original.replace(mode.html[0], mode.html[1]);
          if (changed !== original) planted += 1;
          return route.fulfill({ response, body: changed });
        });
      }
      if (mode?.js) {
        await page.route('**/preferences.js{,?*}', async (route) => {
          const response = await route.fetch();
          const original = await response.text();
          if (original.split(mode.js[0]).length === 2) planted += 1;
          return route.fulfill({ response, body: original.replace(mode.js[0], mode.js[1]) });
        });
      }
      const response = await page.goto(BASE + path, { waitUntil: 'load' });
      if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
      await page.evaluate(() => document.fonts.ready);
      await arrived(page);
      return await fn(page, context);
    } finally {
      await context.close();
    }
  };
  const rect = (page, selector) => page.evaluate((s) => {
    const element = document.querySelector(s);
    if (!element) return null;
    const { top, bottom, left, right, width, height } = element.getBoundingClientRect();
    return { top, bottom, left, right, width, height };
  }, selector);

  // Two columns on a wide window: the choices at 500, the sheet 48 beyond
  // them, and the sheet still in view under the header with the column
  // scrolled well past where it started.
  if (runs('bench')) {
    await at({}, async (page) => {
      const choices = await rect(page, '.y-prefs__controls');
      const sheet = await rect(page, '.y-prefs__sheet');
      if (!choices || !sheet) broken('the settings page draws no column of choices or no sheet');
      if (Math.abs(choices.width - 500) > 0.5 || Math.abs(sheet.left - choices.right - 48) > 0.5 || Math.abs(sheet.top - choices.top) > 0.5) {
        fire('bench', `at 1512 the choices are ${choices.width}px wide and the sheet starts ${sheet.left - choices.right}px beyond them, ${sheet.top - choices.top}px lower; want 500, 48 and level`);
      }
      await page.evaluate(() => window.scrollTo(0, 700));
      const header = await rect(page, '.y-header');
      const held = await rect(page, '.y-prefs__sheet');
      if (held.top < header.bottom || held.top > header.bottom + 48) {
        fire('bench', `scrolled 700px down at 1512, the sheet's top is at ${held.top}px under a header ending at ${header.bottom}px; want it held just under the header`);
      }
    });
    console.log('preference-sheet: on a wide window the choices are 500px, the sheet stands 48px beside them and stays in view as the column scrolls');
  }

  // Stacked below 1081: the sheet above the choices and moving with the page,
  // and nothing wider than the window.
  if (runs('stacked')) {
    for (const width of [1000, 390]) {
      await at({ width }, async (page) => {
        const choices = await rect(page, '.y-prefs__controls');
        const sheet = await rect(page, '.y-prefs__sheet');
        if (sheet.bottom > choices.top) fire('stacked', `at ${width}px the sheet ends at ${sheet.bottom}px, below the top of the choices at ${choices.top}px; want it above them`);
        await page.evaluate(() => window.scrollTo(0, 300));
        const moved = await rect(page, '.y-prefs__sheet');
        if (Math.abs(sheet.top - moved.top - 300) > 1) fire('stacked', `at ${width}px a scroll of 300px moved the sheet ${sheet.top - moved.top}px; want it to scroll away with the page`);
        const [scroll, inner] = await page.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
        if (scroll > inner) fire('stacked', `at ${width}px the page is ${scroll}px wide in a ${inner}px window`);
      });
    }
    console.log('preference-sheet: below 1081 the sheet stands above the choices, scrolls with the page, and nothing scrolls sideways');
  }

  // The sheet against a note, under the same stored choices.
  const notes = {};
  const noteReading = async (stored, scheme, path) => {
    const key = JSON.stringify([stored, scheme, path]);
    if (!notes[key]) notes[key] = await at({ path, stored, scheme }, (page) => page.evaluate(readSetting, '.y-main > .y-article'));
    if (!notes[key]) broken(`${path} has no paragraph under a section heading to read`);
    return notes[key];
  };
  const combinations = [
    { stored: {}, scheme: 'light', note: NOTE_CJK },
    { stored: { textsize: 'l', font: 'sans' }, scheme: 'light', note: NOTE_CJK },
    { stored: { textsize: 'xl', font: 'kai' }, scheme: 'light', note: NOTE_CJK },
    { stored: { theme: 'dark' }, scheme: 'light', note: NOTE_CJK },
    { stored: {}, scheme: 'dark', note: NOTE_CJK },
    { stored: { paper: 'washi', font: 'serif' }, scheme: 'light', note: NOTE_CJK },
    { stored: { paper: 'white', textsize: 'xl' }, scheme: 'light', note: NOTE_CJK },
    { stored: { lang: 'en' }, scheme: 'light', note: NOTE_LATIN },
    { stored: { lang: 'en', textsize: 'xl', font: 'kai', theme: 'dark' }, scheme: 'light', note: NOTE_LATIN },
  ];
  if (runs('same-reading')) {
    for (const { stored, scheme, note } of combinations) {
      const want = await noteReading(stored, scheme, note);
      const got = await at({ stored, scheme }, (page) => page.evaluate(readSetting, '.y-prefs__sheet'));
      if (!got?.codeFace) broken('the sheet has no paragraph with code under a section heading');
      // A line that is a fraction of a pixel moves every block under it, in the
      // sheet as in a note, and a note's own probe holds the note to the same.
      if (!/^\d+px$/.test(got.line)) fire('same-reading', `under ${JSON.stringify(stored)} (${scheme} system) the sheet's line is ${got.line}, want a whole pixel`);
      for (const key of Object.keys(want)) {
        if (want[key] !== null && got[key] !== want[key]) {
          fire('same-reading', `under ${JSON.stringify(stored)} (${scheme} system) the sheet's ${key} is ${got[key]}, a note's is ${want[key]}`);
        }
      }
    }
    console.log(`preference-sheet: the sheet is set as a note is under ${combinations.length} sets of stored choices`);
  }

  // A pick moves the sheet at once: no request, no new page.
  if (runs('instant')) {
    const xlKai = await noteReading({ textsize: 'xl', font: 'kai' }, 'light', NOTE_CJK);
    const dark = await noteReading({ theme: 'dark' }, 'light', NOTE_CJK);
    await at({ width: 1280 }, async (page) => {
      const posted = [];
      page.on('request', (request) => { if (request.method() === 'POST') posted.push(request.url()); });
      const landed = page.url();
      await page.click('[data-pref-field="textsize"] input[value="xl"]');
      await page.click('[data-pref-field="font"] input[value="kai"]');
      const picked = await page.evaluate(readSetting, '.y-prefs__sheet');
      if (picked.size !== xlKai.size || picked.face !== xlKai.face) {
        fire('instant', `with 特大 and 楷體 just picked the sheet is ${picked.size} in ${picked.face}; a note under them is ${xlKai.size} in ${xlKai.face}`);
      }
      await page.click('[data-pref-field="theme"] input[value="dark"]');
      const lamp = await page.evaluate(readSetting, '.y-prefs__sheet');
      if (lamp.ground !== dark.ground || lamp.ink !== dark.ink) {
        fire('instant', `with the night lamp just picked the sheet is ${lamp.ink} on ${lamp.ground}; a note under it is ${dark.ink} on ${dark.ground}`);
      }
      if (posted.length > 0 || page.url() !== landed) fire('instant', `picking moved the page to ${page.url()} or sent ${posted.length} POST; want neither`);
    });
    console.log('preference-sheet: a pick redraws the sheet at once, with nothing sent and the page kept');
  }

  // The paper's row by day and by the lamp, with and without a script.
  if (runs('paper-row')) {
    const shown = (page) => page.evaluate(() => {
      const row = document.querySelector('[data-pref-field="paper"]');
      return Boolean(row) && row.getClientRects().length > 0;
    });
    await at({ scheme: 'light' }, async (page) => {
      if (!(await shown(page))) fire('paper-row', 'by day with no theme stored the paper row is gone; want it shown');
      await page.click('[data-pref-field="theme"] input[value="dark"]');
      if (await shown(page)) fire('paper-row', 'with the night lamp just picked the paper row still stands; want it gone');
      await page.click('[data-pref-field="theme"] input[value="light"]');
      if (!(await shown(page))) fire('paper-row', 'with the light theme picked back the paper row is still gone; want it shown');
    });
    await at({ scheme: 'dark' }, async (page) => {
      if (await shown(page)) fire('paper-row', 'following a dark system the paper row stands; want it gone');
    });
    await at({ scheme: 'dark', stored: { theme: 'light' }, script: false }, async (page) => {
      if (!(await shown(page))) fire('paper-row', 'with no script and the light theme stored on a dark system the paper row is gone; want it shown');
    });
    await at({ scheme: 'light', stored: { theme: 'dark' }, script: false }, async (page) => {
      if (await shown(page)) fire('paper-row', 'with no script and the night lamp stored the paper row stands; want it gone');
    });
    console.log('preference-sheet: the paper row goes under the night lamp and the dark system, and stands by day, with or without a script');
  }

  // Each paper's name carries a chip of that paper: its ground is the ground
  // the page takes when that paper is kept, whichever paper the page is on now.
  if (runs('chips')) {
    const papers = ['stone', 'washi', 'white'];
    const chips = (page) => page.evaluate(() => Object.fromEntries([...document.querySelectorAll("[data-pref-field='paper'] .y-papersample")].map((chip) => [chip.dataset.paper, { ground: getComputedStyle(chip).backgroundColor, width: chip.getBoundingClientRect().width, label: chip.parentElement.textContent.trim() }])));
    const grounds = {};
    for (const paper of papers) {
      await at({ stored: { paper } }, async (page) => {
        grounds[paper] = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
        const got = await chips(page);
        if (Object.keys(got).length !== papers.length) broken(`the paper row holds chips for ${Object.keys(got)}, want ${papers}`);
        grounds[`chip-${paper}`] = got;
      });
    }
    if (new Set(papers.map((p) => grounds[p])).size !== papers.length) broken(`the three papers are not three grounds: ${JSON.stringify(papers.map((p) => grounds[p]))}`);
    for (const paper of papers) {
      for (const on of papers) {
        const chip = grounds[`chip-${on}`][paper];
        if (chip.ground !== grounds[paper]) fire('chips', `on the ${on} paper the ${paper} chip is ${chip.ground}, want the ${paper} paper's ground ${grounds[paper]}`);
        if (chip.width < 10 || chip.width > 16) fire('chips', `the ${paper} chip is ${chip.width}px wide, want a small chip`);
        if (!chip.label) fire('chips', `the ${paper} chip stands beside no name`);
      }
    }
    console.log('preference-sheet: each paper\'s name carries a chip of that paper, read from the paper and the same on any paper');
  }

  // The buttons beside the heading, as a note on the same window draws them.
  if (runs('section-buttons')) {
    const opacity = (page, selector) => page.evaluate((s) => getComputedStyle(document.querySelector(s)).opacity, selector);
    for (const choice of ['hover', 'always']) {
      const stored = { sectionbuttons: choice, lang: 'en' };
      const want = await at({ path: NOTE_LATIN, stored }, (page) => opacity(page, '.y-main > .y-article .y-sect a.y-margin__act'));
      await at({ stored }, async (page) => {
        const got = await opacity(page, '.y-prefs__sheet .y-margin__act');
        if (got !== want) fire('section-buttons', `with the buttons beside sections set to ${choice}, the sheet's are at opacity ${got} and a note's at ${want}`);
        if (choice === 'hover') {
          await page.hover('.y-prefs__sheet .y-sect');
          await page.waitForTimeout(800);
          const rested = await opacity(page, '.y-prefs__sheet .y-margin__act');
          if (rested !== '1') fire('section-buttons', `with the pointer resting on the sheet's heading its buttons are at opacity ${rested}; want 1`);
        }
      });
    }
    console.log('preference-sheet: the buttons beside the sheet\'s heading come and go as a note\'s do');
  }

  // With no script the sheet still stands, in the choices the page was drawn
  // with.
  if (runs('no-script')) {
    const stored = { textsize: 'xl', font: 'kai' };
    const want = await noteReading(stored, 'light', NOTE_CJK);
    await at({ stored, script: false }, async (page) => {
      const sheet = await rect(page, '.y-prefs__sheet');
      if (!sheet || sheet.height <= 0) fire('no-script', 'with no script the settings page draws no sheet');
      const got = await page.evaluate(readSetting, '.y-prefs__sheet');
      if (got.size !== want.size || got.face !== want.face) fire('no-script', `with no script the sheet is ${got.size} in ${got.face}; a note under the same choices is ${want.size} in ${want.face}`);
    });
    console.log('preference-sheet: with no script the sheet stands, drawn in the stored choices');
  }

  // The caption: the first thing on the sheet, in the interface's small face,
  // on a hairline over the reading, and readable on the sheet's own ground, in
  // light and dark, with a script or without one.
  if (runs('caption')) {
    for (const scheme of ['light', 'dark']) {
      for (const script of [true, false]) {
        await at({ scheme, script, stored: { theme: scheme } }, async (page) => {
          const got = await page.evaluate(() => {
            const sheet = document.querySelector('.y-prefs__sheet');
            const caption = sheet?.querySelector('.y-prefs__caption');
            if (!caption) return null;
            const canvas = document.createElement('canvas');
            canvas.width = 1;
            canvas.height = 1;
            const context = canvas.getContext('2d', { willReadFrequently: true });
            const rgba = (value) => {
              context.clearRect(0, 0, 1, 1);
              context.fillStyle = value;
              context.fillRect(0, 0, 1, 1);
              return [...context.getImageData(0, 0, 1, 1).data];
            };
            const style = getComputedStyle(caption);
            const prose = sheet.querySelector('.y-prose').getBoundingClientRect();
            const box = caption.getBoundingClientRect();
            return {
              first: sheet.firstElementChild === caption, drawn: caption.checkVisibility() && box.height > 0,
              text: caption.textContent.trim(), size: Number.parseFloat(style.fontSize), family: style.fontFamily,
              rule: style.borderBottomWidth, above: box.bottom <= prose.top,
              ink: rgba(style.color), ground: rgba(getComputedStyle(sheet).backgroundColor),
              uiFamily: getComputedStyle(document.querySelector('.y-preffield legend')).fontFamily,
            };
          });
          if (!got) fire('caption', `in ${scheme}${script ? '' : ' without a script'} the sheet carries no caption`);
          const channel = (c) => { const v = c / 255; return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; };
          const over = (top, under) => [0, 1, 2].map((i) => top[i] * (top[3] / 255) + under[i] * (1 - top[3] / 255));
          const lum = ([r, g, b]) => 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
          const [hi, lo] = [lum(over(got.ink, got.ground)), lum(got.ground)].sort((x, y) => y - x);
          const ratio = (hi + 0.05) / (lo + 0.05);
          if (!got.first || !got.drawn || !got.above || got.text !== '即時預覽') fire('caption', `in ${scheme}${script ? '' : ' without a script'} the caption is ${JSON.stringify({ first: got.first, drawn: got.drawn, above: got.above, text: got.text })}, want 即時預覽 drawn first, over the reading`);
          if (got.family !== got.uiFamily || got.size > 12.5 || got.rule !== '1px') fire('caption', `the caption is set ${got.size}px in ${got.family} over a ${got.rule} rule, want the interface's face (${got.uiFamily}) at the small size over a hairline`);
          if (ratio < 4.5) fire('caption', `in ${scheme} the caption reads at ${ratio.toFixed(2)}:1 on the sheet, want 4.5:1`);
        });
      }
    }
    console.log('preference-sheet: the sheet is captioned a preview in the small print, over the reading, at 4.5:1 in light and dark');
  }

  // The language first and alone, then two groups, each headed by a small
  // heading in the interface's face over the double rule.
  if (runs('groups')) {
    await at({}, async (page) => {
      const got = await page.evaluate(() => {
        const form = document.querySelector('form[data-prefs]');
        const groups = [...form.querySelectorAll(':scope > .y-prefs__group')];
        return {
          lead: [...form.querySelectorAll(':scope > fieldset[data-pref-field]')].map((f) => f.dataset.prefField),
          groups: groups.map((group) => {
            const h2 = group.querySelector(':scope > h2');
            const style = getComputedStyle(h2);
            const after = getComputedStyle(h2, '::after');
            return {
              title: h2.textContent.trim(), labelled: group.getAttribute('aria-labelledby') === h2.id,
              fields: [...group.querySelectorAll('fieldset[data-pref-field]')].map((f) => f.dataset.prefField),
              size: Number.parseFloat(style.fontSize), family: style.fontFamily,
              rule: { content: after.content, height: after.height, top: after.borderTopWidth, bottom: after.borderBottomWidth, topInk: after.borderTopColor, bottomInk: after.borderBottomColor },
            };
          }),
          uiFamily: getComputedStyle(document.querySelector('.y-preffield legend')).fontFamily,
          shortcutsRule: getComputedStyle(document.querySelector('.y-prefs__shortcuts h2'), '::after').height,
        };
      });
      const want = [
        { title: '紙面與字體', fields: ['theme', 'paper', 'textsize', 'font'] },
        { title: '閱讀行為', fields: ['ruby', 'rail', 'sectionbuttons', 'position', 'shortcuts'] },
      ];
      if (JSON.stringify(got.lead) !== JSON.stringify(['lang'])) fire('groups', `ahead of the groups stand ${JSON.stringify(got.lead)}, want the language alone`);
      const shape = got.groups.map(({ title, fields }) => ({ title, fields }));
      if (JSON.stringify(shape) !== JSON.stringify(want)) fire('groups', `the choices stand in ${JSON.stringify(shape)}, want ${JSON.stringify(want)}`);
      for (const group of got.groups) {
        const { rule } = group;
        const doubled = rule.content !== 'none' && rule.height === '4px' && rule.top === '1px' && Number.parseFloat(rule.bottom) > 0 && Number.parseFloat(rule.bottom) <= 1 && rule.topInk !== rule.bottomInk;
        if (!group.labelled || !doubled) fire('groups', `${group.title} is ${group.labelled ? '' : 'not '}named by its heading and closes on ${JSON.stringify(rule)}, want a 1px firm rule over a finer hairline in a lighter ink, 4px apart`);
        if (group.family !== got.uiFamily || group.size > 12.5) fire('groups', `${group.title} is set ${group.size}px in ${group.family}, want the interface's small face (${got.uiFamily})`);
      }
    });
    console.log('preference-sheet: the language stands first and alone, then the two groups, each headed small over the double rule');
  }

  // Hidden from assistive technology, and never a stop for the keyboard.
  if (runs('quiet')) {
    await at({}, async (page) => {
      const state = await page.evaluate(() => {
        const sheet = document.querySelector('.y-prefs__sheet');
        return {
          hidden: sheet.getAttribute('aria-hidden'),
          focusable: sheet.querySelectorAll('a[href], button, input, select, textarea, [tabindex]').length,
        };
      });
      if (state.hidden !== 'true') fire('quiet', `the sheet carries aria-hidden=${state.hidden}; want true`);
      if (state.focusable > 0) fire('quiet', `the sheet holds ${state.focusable} things that take focus; want none`);
      for (let step = 0; step < 60; step += 1) {
        await page.keyboard.press('Tab');
        if (await page.evaluate(() => Boolean(document.activeElement?.closest('.y-prefs__sheet')))) {
          fire('quiet', `the keyboard stopped inside the sheet on Tab ${step + 1}`);
        }
      }
    });
    console.log('preference-sheet: the sheet is hidden from assistive technology and the keyboard passes it by');
  }

  if (mode && planted === 0) throw new NotApplied(`nothing the mutation rewrites was requested, so ${MUTATE} reached no page`);
  if (MUTATE) failure = new Error(`no catch: ${MUTATE} left every lock standing`);
  else console.log('PASS preference-sheet');
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
