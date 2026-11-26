// Behavior lock for a note's edges: what the page says about the file rather
// than reads from it, what the note is besides a note, and the page a reader
// meets where there is no note to read.
//
// What the page says about the file stands in the head, under the title and
// above the double rule that parts the head from the text: one sentence each,
// in the interface's small face, led by a hollow triangle in the ink. It is
// never boxed, tinted, ochre or red, it needs no script, and a field the
// rules list the values of folds that list under its sentence, closed. Every
// line is a whole pixel at every text size, since these lines stand above any
// place a reader keeps, and every word reads at 4.5:1 or more in light and
// dark. A phone is never made wider than itself.
//
// The row above the title ends on what the note is: an index says so, and a
// template is never called a draft, because the draft it writes belongs to the
// notes made from it. An address that names nothing, and a file that could not
// be read, are the same column with the same title in the page's ink; the
// fault's one sign is its word above the title in the crimson the design keeps
// for a file that cannot be read, with no box.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note whose type the rules do not know) and
// MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/out-of-contract.md';
const UNKNOWN_KEY = '/notes/Notes/schema-notice-probe.md';
const INDEX = '/notes/Maps/reading.md';
const TEMPLATE = '/notes/System/templates/Template%20lesson.md';
const MISSING = '/notes/Notes/no-such-note.md';
const MUTATE = process.env.MUTATE || '';

const css = (rule, site) => ({ css: rule, site });
const doc = (path, needle, replacement, site) => ({ doc: { path, needle, replacement }, site });
const MODES = {
  // The notes moved out of the head, under the double rule.
  'notes-below-the-rule': doc(PAGE, '<div class="y-headnotes"', '</header><div class="y-headnotes"', 'head'),
  // A note boxed on a tinted ground.
  'note-boxed': css('.y-headnote{background:var(--panel) !important;padding:8px !important}', 'quiet'),
  // The triangle in the colour kept for writes that failed.
  'triangle-red': css('.y-headnote > .y-icon{color:var(--crimson) !important}', 'quiet'),
  // The triangle in the ochre a question keeps.
  'triangle-ochre': css('.y-headnote > .y-icon{color:var(--ochre) !important}', 'quiet'),
  // A note set in the reading face.
  'note-in-the-reading-face': css('.y-headnotes{font-family:var(--font-serif) !important}', 'quiet'),
  // The list of values drawn open.
  'fold-open': doc(PAGE, '<details class="y-headnote__known">', '<details class="y-headnote__known" open>', 'fold'),
  // A note fainter than text may be.
  'note-too-faint': css('.y-headnotes{color:var(--fg-ornament) !important}', 'contrast'),
  // A note on a line that is not rounded.
  'fractional-note-line': css('.y-headnotes{line-height:1.57 !important}', 'lines'),
  // The notes hidden from a reader with no script.
  'hidden-without-script': css('html:not([data-js]) .y-headnotes{display:none !important}', 'script'),
  // A note wider than a phone.
  'note-past-the-phone': css('.y-headnote{min-width:600px !important}', 'phone'),
  // An index that does not say so.
  'kind-dropped': css('.y-notewhere__kind{display:none !important}', 'kind'),
  // A template called a draft.
  'template-called-draft': doc(TEMPLATE, '<p class="y-notewhere" lang="zh-Hant">', '<p class="y-notewhere" lang="zh-Hant"><span class="y-notewhere__draft" data-note-draft>草稿</span>', 'kind'),
  // A file that could not be read boxed on a tinted ground again.
  'fault-boxed': css('.y-recovery > .y-nothing--fault[data-nothing="notfound"]{background:var(--panel) !important;padding:24px !important}', 'error'),
  // Its title in the ochre.
  'fault-title-ochre': css('.y-recovery > .y-nothing--fault[data-nothing="notfound"] .y-nothing__title{color:var(--ochre) !important}', 'error'),
  // Its word in the ink, as if it were any other fault.
  'unreadable-word-in-ink': css('.y-nothing--unreadable .y-nothing__mark{color:var(--fg) !important}', 'error'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`note-edges: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL note-edges: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN note-edges: ${message}`); };
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

// The fault page is the address that names nothing with the fault's class, which
// is how the server draws a file it could not read; the fixture holds no such
// file, so the probe asks the stylesheet about the same markup.
const asFault = (body) => body.replace('class="y-nothing" data-nothing="notfound"', 'class="y-nothing y-nothing--fault y-nothing--unreadable" data-nothing="notfound"');

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`note-edges: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  const plant = async (page, fault) => {
    if (fault) {
      await page.route(BASE + MISSING, async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        if (asFault(original) === original) broken('the page for an address that names nothing drew no notice to mark as a fault');
        return route.fulfill({ response, body: asFault(original) });
      });
    }
    if (!mode) return;
    if (mode.css) {
      await page.route('**/static/app.css{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        requests += 1;
        planted += 1;
        return route.fulfill({ response, body: `${original}\n${mode.css}\n` });
      });
      return;
    }
    if (fault) return;
    await page.route(BASE + mode.doc.path, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      requests += 1;
      if (original.includes(mode.doc.needle)) planted += 1;
      return route.fulfill({ response, body: original.replaceAll(mode.doc.needle, mode.doc.replacement) });
    });
  };
  const at = async ({ path = PAGE, width = 1280, theme = 'light', size = '', script = true, status = 200, fault = false }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: script });
    await context.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
      ...(size ? [{ name: 'yomihon_textsize', value: size, url: BASE }] : []),
    ]);
    const page = await context.newPage();
    try {
      await plant(page, fault);
      const response = await page.goto(BASE + path, { waitUntil: 'load' });
      if (response?.status() !== status) broken(`${path} answered ${response?.status()}, want ${status}`);
      await page.evaluate(() => document.fonts.ready);
      if (script && status === 200) await arrived(page);
      return await fn(page);
    } finally {
      await context.close();
    }
  };
  const resolve = (page, value) => page.evaluate((value) => {
    const probe = document.createElement('span');
    probe.style.color = value;
    document.body.append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, value);

  // The notes stand in the head: after the title, before the double rule, and
  // before the text.
  if (runs('head')) {
    for (const path of [PAGE, UNKNOWN_KEY]) {
      await at({ path }, async (page) => {
        const got = await page.evaluate(() => {
          const head = document.querySelector('.y-frontis');
          const title = head?.querySelector('.y-title')?.getBoundingClientRect();
          const all = [...document.querySelectorAll('.y-headnote')];
          const rule = head ? head.getBoundingClientRect().bottom - Number.parseFloat(getComputedStyle(head, '::after').height) : null;
          const prose = document.querySelector('.y-prose')?.getBoundingClientRect().top ?? null;
          return {
            count: all.length,
            inHead: all.every((note) => head?.contains(note)),
            top: Math.min(...all.map((note) => note.getBoundingClientRect().top)),
            bottom: Math.max(...all.map((note) => note.getBoundingClientRect().bottom)),
            title: title?.bottom ?? null,
            rule,
            prose,
          };
        });
        if (got.count < 1) broken(`${path} draws no note about its file`);
        if (!got.inHead) fire('head', `on ${path} a note about the file stands outside the head`);
        if (!(got.title <= got.top && got.bottom <= got.rule && got.rule <= got.prose)) fire('head', `on ${path} the title ends at ${got.title}, the notes run ${got.top}..${got.bottom}, the rule is at ${got.rule} and the text starts at ${got.prose}; want them in that order`);
      });
    }
    console.log('note-edges: the notes about the file stand in the head, under the title and above the double rule');
  }

  // Quiet: no box, no tint, no rule of their own, the interface's face, and the
  // triangle in the ink.
  if (runs('quiet')) {
    for (const theme of ['light', 'dark']) {
      await at({ theme }, async (page) => {
        const ink = await resolve(page, 'var(--fg)');
        const sans = await page.evaluate(() => {
          const probe = document.createElement('span');
          probe.style.fontFamily = 'var(--font-sans)';
          document.body.append(probe);
          const face = getComputedStyle(probe).fontFamily;
          probe.remove();
          return face;
        });
        const got = await page.evaluate(() => [...document.querySelectorAll('.y-headnotes, .y-headnote, .y-headnote__known')].map((element) => {
          const style = getComputedStyle(element);
          const icon = element.matches('.y-headnote') ? element.querySelector(':scope > .y-icon') : null;
          return {
            what: element.className,
            ground: style.backgroundColor,
            borders: [style.borderTopWidth, style.borderRightWidth, style.borderBottomWidth, style.borderLeftWidth, style.boxShadow],
            face: style.fontFamily,
            triangle: icon ? { colour: getComputedStyle(icon).color, name: icon.dataset.icon } : null,
          };
        }));
        if (got.length < 3) broken(`only ${got.length} parts of the notes were read`);
        for (const part of got) {
          if (part.ground !== 'rgba(0, 0, 0, 0)') fire('quiet', `in ${theme} ${part.what} stands on ${part.ground}, want no ground`);
          if (part.borders.slice(0, 4).some((width) => width !== '0px') || part.borders[4] !== 'none') fire('quiet', `in ${theme} ${part.what} draws an edge: ${part.borders}`);
          if (part.face !== sans) fire('quiet', `in ${theme} ${part.what} is set in ${part.face}, want the interface's ${sans}`);
          if (part.what === 'y-headnote' && (part.triangle?.name !== 'warning' || part.triangle.colour !== ink)) fire('quiet', `in ${theme} a note is led by ${JSON.stringify(part.triangle)}, want the warning triangle in the ink ${ink}`);
        }
      });
    }
    console.log('note-edges: no box, tint or edge; the interface face; the triangle in the ink');
  }

  // The list of values is folded closed under its sentence, and opens on a press.
  if (runs('fold')) {
    await at({}, async (page) => {
      const fold = page.locator('details.y-headnote__known');
      if ((await fold.count()) !== 1) broken(`${PAGE} folds ${await fold.count()} lists of values, want 1`);
      const closed = await fold.evaluate((element) => ({ open: element.open, list: element.querySelector(':scope > p')?.checkVisibility() ?? false, heading: element.querySelector('summary').textContent.trim() }));
      if (closed.open || closed.list) fire('fold', `the list of values arrives open (${closed.open}, drawn ${closed.list})`);
      if (!closed.heading.includes('type')) broken(`the fold's heading ${JSON.stringify(closed.heading)} does not name the field`);
      await fold.locator('summary').click();
      const values = await fold.evaluate((element) => [...element.querySelectorAll(':scope > p code')].map((code) => code.textContent));
      if (!values.includes('concept') || !values.includes('topic-map')) fire('fold', `the open fold lists ${JSON.stringify(values)}, want the rules' types`);
    });
    console.log('note-edges: the rules\' values fold closed under the sentence and open on a press');
  }

  // Every word in the notes and at the row's end reads at 4.5:1 or more.
  if (runs('contrast')) {
    for (const theme of ['light', 'dark']) {
      for (const path of [PAGE, INDEX]) {
        await at({ path, theme }, async (page) => {
          await page.evaluate(() => { for (const fold of document.querySelectorAll('details.y-headnote__known')) fold.open = true; });
          const pairs = await page.evaluate(() => {
            const rgb = (value) => {
              const canvas = document.createElement('canvas');
              canvas.width = 1;
              canvas.height = 1;
              const context = canvas.getContext('2d');
              context.fillStyle = value;
              context.fillRect(0, 0, 1, 1);
              return [...context.getImageData(0, 0, 1, 1).data];
            };
            const ground = rgb(getComputedStyle(document.body).backgroundColor).slice(0, 3);
            const over = (colour) => {
              const [r, g, b, a] = rgb(colour);
              return [r, g, b].map((c, i) => Math.round(ground[i] * (1 - a / 255) + c * (a / 255)));
            };
            const texts = [...document.querySelectorAll('.y-headnotes *, .y-notewhere *')].filter((element) => element.checkVisibility() && [...element.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() !== ''));
            return texts.map((element) => ({ what: `${element.tagName.toLowerCase()}.${element.className}`, ink: over(getComputedStyle(element).color), ground }));
          });
          if (pairs.length < 3) broken(`only ${pairs.length} pieces of text were read on ${path}`);
          for (const { what, ink, ground } of pairs) {
            const ratio = contrast(ink, ground);
            if (ratio < 4.5) fire('contrast', `in ${theme} on ${path} ${what} is ${ratio.toFixed(2)}:1, want 4.5:1 or more`);
          }
        });
      }
    }
    console.log('note-edges: every word in the head\'s notes and row reads at 4.5:1 or more in light and in dark');
  }

  // Every line is a whole pixel at every text size, with the fold open or shut.
  if (runs('lines')) {
    for (const size of ['', 'l', 'xl']) {
      await at({ size }, async (page) => {
        const read = () => page.evaluate(() => [...document.querySelectorAll('.y-headnotes, .y-headnotes *')].filter((element) => element.checkVisibility() && getComputedStyle(element).display !== 'inline' && element.textContent.trim() !== '').map((element) => {
          const box = element.getBoundingClientRect();
          return { what: `${element.tagName.toLowerCase()}.${element.className}`, line: getComputedStyle(element).lineHeight, height: box.height };
        }));
        const shut = await read();
        await page.evaluate(() => { for (const fold of document.querySelectorAll('details.y-headnote__known')) fold.open = true; });
        const open = await read();
        if (shut.length < 4) broken(`only ${shut.length} lines were read`);
        for (const { what, line, height } of [...shut, ...open]) {
          const px = Number.parseFloat(line);
          if (!line.endsWith('px') || Math.abs(px - Math.round(px)) > 0.001) fire('lines', `at size ${size || 'm'} ${what} is set on a ${line} line, want a whole pixel`);
          if (Math.abs(height - Math.round(height)) > 0.001) fire('lines', `at size ${size || 'm'} ${what} is ${height}px tall, want a whole pixel`);
        }
      });
    }
    console.log('note-edges: every line of the notes is a whole pixel at all three text sizes');
  }

  // The notes are the server's, and read with no script.
  if (runs('script')) {
    await at({ script: false }, async (page) => {
      const drawn = await page.evaluate(() => [...document.querySelectorAll('.y-headnote')].map((note) => note.checkVisibility()));
      if (drawn.length < 1) broken('with no script the page draws no note');
      if (drawn.some((shown) => !shown)) fire('script', `with no script the notes are drawn ${JSON.stringify(drawn)}`);
    });
    console.log('note-edges: the notes read with no script running');
  }

  // A phone is never made wider than itself.
  if (runs('phone')) {
    for (const path of [PAGE, UNKNOWN_KEY]) {
      await at({ path, width: 390 }, async (page) => {
        await page.evaluate(() => { for (const fold of document.querySelectorAll('details.y-headnote__known')) fold.open = true; });
        const got = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, window: innerWidth, right: Math.max(...[...document.querySelectorAll('.y-headnotes, .y-headnotes *')].map((element) => element.getBoundingClientRect().right)) }));
        if (got.page > got.window || got.right > got.window + 0.5) fire('phone', `at 390px on ${path} the page is ${got.page}px wide and the notes reach ${got.right}px`);
      });
    }
    console.log('note-edges: at 390px the notes fit the phone');
  }

  // The row above the title ends on what the note is, and a template is not a draft.
  if (runs('kind')) {
    await at({ path: INDEX }, async (page) => {
      const got = await page.evaluate(() => [...document.querySelectorAll('.y-notewhere [data-note-kind]')].map((mark) => ({ drawn: mark.checkVisibility(), word: mark.textContent.trim(), icon: mark.querySelector('.y-icon')?.dataset.icon })));
      if (got.length !== 1 || !got[0].drawn || got[0].word !== '索引' || got[0].icon !== 'index') fire('kind', `an index's row ends on ${JSON.stringify(got)}, want 索引 drawn with its pictogram`);
    });
    await at({ path: TEMPLATE }, async (page) => {
      const drafts = await page.evaluate(() => document.querySelectorAll('.y-frontis [data-note-draft]').length);
      if (drafts !== 0) fire('kind', `a template's head calls it a draft ${drafts} times`);
    });
    console.log('note-edges: an index says so in its row, and a template is not called a draft');
  }

  // An address that names nothing and a file that could not be read: one
  // column, no ground, the title in the page's ink, and the word of a file that
  // cannot be read in the crimson, which reads at 4.5:1.
  if (runs('error')) {
    for (const theme of ['light', 'dark']) {
      for (const fault of [false, true]) {
        await at({ path: MISSING, theme, status: 404, fault }, async (page) => {
          const fg = await resolve(page, 'var(--fg)');
          const crimson = await resolve(page, 'var(--crimson)');
          const got = await page.evaluate(() => {
            const notice = document.querySelector('.y-recovery > .y-nothing');
            const style = getComputedStyle(notice);
            return {
              fault: notice.classList.contains('y-nothing--fault'),
              ground: style.backgroundColor,
              padding: style.paddingLeft,
              left: notice.getBoundingClientRect().left,
              next: document.querySelector('.y-recovery__detail')?.getBoundingClientRect().left ?? null,
              title: getComputedStyle(notice.querySelector('.y-nothing__title')).color,
              mark: getComputedStyle(notice.querySelector('.y-nothing__mark')).color,
              body: getComputedStyle(document.body).backgroundColor,
            };
          });
          if (got.fault !== fault) broken(`the notice is a fault = ${got.fault}, want ${fault}`);
          if (got.ground !== 'rgba(0, 0, 0, 0)' || got.padding !== '0px') fire('error', `in ${theme} the ${fault ? 'fault' : 'absence'} stands on ${got.ground} with ${got.padding} of padding, want neither`);
          if (Math.abs(got.left - got.next) > 0.5) fire('error', `in ${theme} the notice starts at ${got.left} and the address under it at ${got.next}, want one left edge`);
          if (got.title !== fg) fire('error', `in ${theme} the ${fault ? 'fault' : 'absence'}'s title is ${got.title}, want the page's ink ${fg}`);
          if (fault && got.mark !== crimson) fire('error', `in ${theme} the unreadable file's word is ${got.mark}, want the crimson ${crimson}`);
          const parse = (value) => value.match(/\d+(\.\d+)?/g).slice(0, 3).map(Number);
          const ratio = contrast(parse(got.mark), parse(got.body));
          if (ratio < 4.5) fire('error', `in ${theme} the word above the title reads at ${ratio.toFixed(2)}:1, want 4.5:1 or more`);
        });
      }
    }
    console.log('note-edges: an absence and a fault share one column with no ground; a file that cannot be read says so in the crimson');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED note-edges: the mutation reached ${planted} of ${requests} requests`);
  console.log('PASS note-edges: the head says what it knows about the file, quietly, and the edges of a note read in one voice');
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
