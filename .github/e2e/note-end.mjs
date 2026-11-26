// Behavior lock for how a note ends: its footnotes, then what stands beside it,
// then its colophon, as the Atelier note ends.
//
// What stands beside the note is a quiet label beside each list, and a list
// with nothing in it is not drawn: a note nothing links to and nothing stands
// beside goes from its text straight to its colophon. What the check found is
// said there too, each finding on a thin ochre rule and never in a box. The
// colophon stands 72 pixels under whatever is above it, on a hairline, with a
// centred seal of two short strong hairlines either side of the bookmark. Its
// way on is the only previous and next on the page, between two faint rules:
// the note before on the left and the note after on the right, a small word in
// the interface face over a title in the reading face, for a folder's files and
// a course's units alike; a course's title and a branch's line stand centred.
// Its foot prints the file's path in figures that keep their width and the two
// doors out of the page, and repeats nothing the header offers. Every line is a
// whole pixel at every text size, every word reads at 4.5:1 or more in light
// and in dark, and a phone is never made wider than itself.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with footnotes, notes beside it, findings
// and a folder's previous and next) and MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/prose-type.md';
const COURSE_UNIT = '/notes/Course/C02.md';
const LONE_NOTE = '/notes/Notes/schema-notice-probe.md';
const MUTATE = process.env.MUTATE || '';

// Written out, not read from the stylesheet: a lock that took its expected
// value from the rule under test would agree with any rule.
const GAP_ABOVE_PX = 72;
const SEAL_RULE_PX = 32;

const css = (rule, site) => ({ css: rule, site });
const doc = (path, needle, replacement, site) => ({ doc: { path, needle, replacement }, site });
const MODES = {
  // What stands beside the note drawn under the colophon.
  'beside-after-colophon': css('.y-article{display:flex !important;flex-direction:column !important}.y-article > .y-related{order:9 !important}', 'order'),
  // An empty list drawn at the end, saying nothing.
  'empty-list-drawn': doc(PAGE, '<div class="y-related" id="_y-end" tabindex="-1" lang="zh-Hant">', '<div class="y-related" id="_y-end" tabindex="-1" lang="zh-Hant"><nav class="y-citedby" aria-label="正文連到這篇的筆記"><div class="ui-side__label">正文連到這篇</div></nav>', 'empty'),
  // The colophon closer to the text than the package sets it.
  'colophon-gap-48': css('.y-colophon{margin-top:48px !important}', 'colophon'),
  // The colophon's rule in the strong ink.
  'colophon-rule-strong': css('.y-colophon{border-top-color:var(--line-strong) !important}', 'colophon'),
  // The seal pushed to the start.
  'seal-off-centre': css('.y-colophon__seal{justify-content:flex-start !important}', 'colophon'),
  // The seal's hairlines in the faint line.
  'seal-rules-faint': css('.y-colophon__rule{background:var(--line-faint) !important}', 'colophon'),
  // The way on without its faint rules.
  'nav-unruled': css('.y-steps{border-block-color:transparent !important}', 'colophon'),
  // The next note drawn on the left.
  'next-on-the-left': css('.y-steps__link--next{order:-1 !important}', 'nav'),
  // A title set in the interface face.
  'title-in-sans': css('.y-steps__name{font-family:var(--font-sans) !important}', 'nav'),
  // A course's title pushed to the start.
  'course-title-at-start': css('.y-steps__source{text-align:start !important}', 'nav'),
  // A finding boxed on a tinted ground.
  'finding-boxed': css('.y-diag{background:var(--panel) !important;border-radius:var(--r-lg) !important}', 'quiet'),
  // The count as a badge.
  'count-as-badge': css('.y-related .y-diagcount{background:var(--panel) !important;padding:0 4px !important}', 'quiet'),
  // A finding's rule in the ochre a question keeps.
  'finding-rule-ochre': css('.y-diag{box-shadow:inset 1.5px 0 0 var(--ochre) !important}', 'quiet'),
  // A finding's kind in the ochre a question keeps.
  'finding-kind-ochre': css('.y-diag__label{color:var(--ochre) !important}', 'quiet'),
  // A finding without its rule.
  'finding-unruled': css('.y-diag{box-shadow:none !important}', 'quiet'),
  // A label fainter than text may be.
  'label-too-faint': css('.y-related .ui-side__label{color:var(--fg-ornament) !important}', 'contrast'),
  // The foot fainter than text may be.
  'foot-too-faint': css('.y-colophon__foot{color:var(--fg-ornament) !important}.y-colophon__facts dt{color:var(--fg-ornament) !important}', 'contrast'),
  // A title on a line that is not rounded.
  'fractional-title-line': css('.y-steps__name{line-height:1.47 !important}', 'lines'),
  // A related title on a line that is not rounded.
  'fractional-related-line': css('.y-related .ui-navitem{line-height:1.53 !important}', 'lines'),
  // The path in figures that change width.
  'path-proportional': css('.y-colophon__path{font-variant-numeric:normal !important}', 'foot'),
  // The foot wider than a phone.
  'foot-past-the-phone': css('.y-colophon__foot{min-width:600px !important}', 'phone'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`note-end: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL note-end: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN note-end: ${message}`); };
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

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let failure = null;
try {
  console.log(`note-end: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  const plant = async (page) => {
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
    await page.route(BASE + mode.doc.path, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      requests += 1;
      // Every occurrence is rewritten, so a body that changed is one wholly
      // rewritten, and a needle that matched nothing reports not-applied.
      if (original.includes(mode.doc.needle)) planted += 1;
      return route.fulfill({ response, body: original.replaceAll(mode.doc.needle, mode.doc.replacement) });
    });
  };
  const at = async ({ path = PAGE, width = 1280, theme = 'light', size = '', script = true }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: script });
    await context.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
      ...(size ? [{ name: 'yomihon_textsize', value: size, url: BASE }] : []),
    ]);
    const page = await context.newPage();
    try {
      await plant(page);
      const response = await page.goto(BASE + path, { waitUntil: 'load' });
      if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
      await page.evaluate(() => document.fonts.ready);
      if (script) await arrived(page);
      if ((await page.locator('.y-colophon').count()) !== 1) broken(`${path} does not end on one colophon`);
      return await fn(page);
    } finally {
      await context.close();
    }
  };
  // A colour as the page resolves it, inside element.
  const resolve = (page, value) => page.evaluate((value) => {
    const probe = document.createElement('span');
    probe.style.color = value;
    document.querySelector('.y-colophon').append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, value);
  const family = (page, value) => page.evaluate((value) => {
    const probe = document.createElement('span');
    probe.style.fontFamily = value;
    document.querySelector('.y-colophon').append(probe);
    const face = getComputedStyle(probe).fontFamily;
    probe.remove();
    return face;
  }, value);

  // Footnotes, then what stands beside the note, then the colophon, last in the
  // article and in that order down the page, with the seal, the way on and the
  // foot in theirs.
  if (runs('order')) {
    await at({}, async (page) => {
      const got = await page.evaluate(() => {
        const box = (selector) => document.querySelector(selector)?.getBoundingClientRect() ?? null;
        const article = document.querySelector('.y-article');
        return {
          notes: box('.y-prose .footnotes'),
          beside: box('.y-article > .y-related'),
          colophon: box('.y-article > .y-colophon'),
          last: article.lastElementChild?.className ?? null,
          parts: ['.y-colophon__seal', '.y-colophon .y-steps', '.y-colophon__foot'].map((selector) => box(selector)?.top ?? null),
        };
      });
      if (!got.notes || !got.beside || !got.colophon) broken(`${PAGE} lacks footnotes, notes beside it or a colophon`);
      if (!(got.notes.bottom <= got.beside.top && got.beside.bottom <= got.colophon.top)) fire('order', `the footnotes end at ${got.notes.bottom}, what stands beside runs ${got.beside.top}..${got.beside.bottom} and the colophon starts at ${got.colophon.top}; want them in that order`);
      if (got.last !== 'y-colophon') fire('order', `the article ends on ${got.last}, want the colophon`);
      if (got.parts.includes(null) || got.parts.some((top, i) => i > 0 && top <= got.parts[i - 1])) fire('order', `the seal, the way on and the foot stand at ${got.parts}, want them in that order`);
    });
    console.log('note-end: the footnotes, what stands beside the note, then the colophon: seal, way on, foot');
  }

  // A list with nothing in it is not drawn, and a note with nothing beside it
  // goes from its text to its colophon without a word about the absence.
  if (runs('empty')) {
    await at({}, async (page) => {
      const empty = await page.evaluate(() => [...document.querySelectorAll('.y-related > *')].filter((block) => block.querySelectorAll('.ui-navitem, .y-diag').length === 0).map((block) => block.textContent.trim()));
      if (empty.length > 0) fire('empty', `the end draws blocks with nothing in them: ${JSON.stringify(empty)}`);
    });
    await at({ path: LONE_NOTE }, async (page) => {
      const got = await page.evaluate(() => ({ beside: document.querySelectorAll('.y-related, .y-citedby').length, words: document.querySelector('.y-article').textContent.includes('目前沒有其他筆記') }));
      if (got.beside > 0 || got.words) fire('empty', `a note with nothing beside it still draws ${got.beside} blocks, or says so (${got.words})`);
    });
    console.log('note-end: no list is drawn empty, and a note with nothing beside it goes straight to its colophon');
  }

  // The colophon's frame: the gap and the rule above it, the centred seal, and
  // the way on between two faint rules.
  if (runs('colophon')) {
    await at({}, async (page) => {
      const line = await resolve(page, 'var(--line)');
      const strong = await resolve(page, 'var(--line-strong)');
      const faint = await resolve(page, 'var(--line-faint)');
      const got = await page.evaluate(() => {
        const colophon = document.querySelector('.y-colophon');
        const above = colophon.previousElementSibling.getBoundingClientRect().bottom;
        const box = colophon.getBoundingClientRect();
        const style = getComputedStyle(colophon);
        const seal = document.querySelector('.y-colophon__seal');
        const glyph = seal.querySelector('.y-icon').getBoundingClientRect();
        const rules = [...seal.querySelectorAll('.y-colophon__rule')].map((rule) => ({ width: rule.getBoundingClientRect().width, height: rule.getBoundingClientRect().height, colour: getComputedStyle(rule).backgroundColor }));
        const steps = getComputedStyle(document.querySelector('.y-colophon .y-steps'));
        return {
          gap: box.top - above,
          rule: { width: Number.parseFloat(style.borderTopWidth), colour: style.borderTopColor },
          glyph: glyph.left + glyph.width / 2,
          middle: box.left + box.width / 2,
          rules,
          band: [steps.borderTopWidth, steps.borderTopColor, steps.borderBottomWidth, steps.borderBottomColor],
        };
      });
      if (Math.abs(got.gap - GAP_ABOVE_PX) > 0.5) fire('colophon', `the colophon stands ${got.gap}px under what is above it, want ${GAP_ABOVE_PX}`);
      if (got.rule.width !== 1 || got.rule.colour !== line) fire('colophon', `the colophon's rule is ${JSON.stringify(got.rule)}, want 1px in ${line}`);
      if (Math.abs(got.glyph - got.middle) > 1) fire('colophon', `the seal's bookmark stands at ${got.glyph}, want the colophon's middle ${got.middle}`);
      if (got.rules.length !== 2 || got.rules.some((rule) => rule.width !== SEAL_RULE_PX || rule.height !== 1 || rule.colour !== strong)) fire('colophon', `the seal's rules are ${JSON.stringify(got.rules)}, want two ${SEAL_RULE_PX}px hairlines in ${strong}`);
      if (JSON.stringify(got.band) !== JSON.stringify(['1px', faint, '1px', faint])) fire('colophon', `the way on is ruled ${JSON.stringify(got.band)}, want 1px in ${faint} above and below`);
    });
    console.log(`note-end: the colophon stands ${GAP_ABOVE_PX}px down on a hairline, its seal centred, its way on between faint rules`);
  }

  // The way on: previous on the left and next on the right, a small word in
  // the interface face over a title in the reading face, for a folder and a
  // course alike; a course's title and a branch's line are centred.
  if (runs('nav')) {
    for (const path of [PAGE, COURSE_UNIT]) {
      await at({ path }, async (page) => {
        const sans = await family(page, 'var(--font-sans)');
        const read = await family(page, 'var(--font-read)');
        const got = await page.evaluate(() => {
          const band = document.querySelector('.y-colophon .y-steps').getBoundingClientRect();
          const side = (selector) => {
            const link = document.querySelector(`.y-colophon ${selector}`);
            const role = getComputedStyle(link.querySelector('.y-steps__role'));
            const name = getComputedStyle(link.querySelector('.y-steps__name'));
            const box = link.getBoundingClientRect();
            return { left: box.left, right: box.right, top: box.top, roleFace: role.fontFamily, roleSize: Number.parseFloat(role.fontSize), nameFace: name.fontFamily, nameSize: Number.parseFloat(name.fontSize) };
          };
          const centred = [...document.querySelectorAll('.y-colophon :is(.y-steps__source, .y-steps__aside)')].map((line) => {
            const range = document.createRange();
            range.selectNodeContents(line);
            const words = range.getBoundingClientRect();
            return Math.abs(words.left + words.width / 2 - (band.left + band.width / 2));
          });
          return { band: { left: band.left, right: band.right }, prev: side('.y-steps__link--prev'), next: side('.y-steps__link--next'), centred, onPage: document.querySelectorAll('a[rel="prev"], a[rel="next"]').length };
        });
        const { band, prev, next } = got;
        if (Math.abs(prev.left - band.left) > 1 || Math.abs(next.right - band.right) > 1 || next.left <= prev.right || Math.abs(prev.top - next.top) > 1) fire('nav', `on ${path} the previous runs ${prev.left}..${prev.right} and the next ${next.left}..${next.right} in a band ${band.left}..${band.right}; want the previous on the left and the next on the right of one row`);
        for (const [what, side] of [['previous', prev], ['next', next]]) {
          if (side.roleFace !== sans || side.nameFace !== read || !(side.roleSize < side.nameSize)) fire('nav', `on ${path} the ${what}'s word is ${side.roleSize}px ${side.roleFace} and its title ${side.nameSize}px ${side.nameFace}; want a smaller word in ${sans} over a title in ${read}`);
        }
        if (got.centred.some((off) => off > 1)) fire('nav', `on ${path} a course's title or a branch's line stands ${JSON.stringify(got.centred)}px off the middle`);
        if (got.onPage !== 2) fire('nav', `on ${path} the page draws ${got.onPage} previous and next links, want the colophon's two alone`);
        if (path === COURSE_UNIT && got.centred.length !== 2) broken(`${COURSE_UNIT} shows ${got.centred.length} centred lines, want its course's title and its branch`);
      });
    }
    console.log('note-end: the way on is previous left and next right, a small word over a reading-face title, for a folder and a course; the course and its branch centred');
  }

  // What the check found: each on a thin rule in the secondary ink, its kind
  // and the count in the full ink, nothing boxed and no colour, the count a
  // figure and not a badge. Ochre is a question still open and nothing else.
  if (runs('quiet')) {
    await at({}, async (page) => {
      const secondary = await resolve(page, 'var(--fg-muted)');
      const ink = await resolve(page, 'var(--fg)');
      const got = await page.evaluate(() => ({
        kinds: [...document.querySelectorAll('.y-related .y-diag__label')].map((label) => getComputedStyle(label).color),
        countInk: getComputedStyle(document.querySelector('.y-related .y-diagcount')).color,
        findings: [...document.querySelectorAll('.y-related .y-diag')].map((finding) => {
          const style = getComputedStyle(finding);
          return { ground: style.backgroundColor, radius: style.borderTopLeftRadius, rule: style.boxShadow, border: style.borderTopWidth };
        }),
        count: getComputedStyle(document.querySelector('.y-related .y-diagcount')).backgroundColor,
      }));
      if (got.findings.length < 2) broken(`${PAGE} shows ${got.findings.length} findings, want two or more`);
      for (const finding of got.findings) {
        const rule = finding.rule.match(/^(.*\))\s+([\d.]+)px\s+0px\s+0px\s+0px\s+inset$/);
        if (finding.ground !== 'rgba(0, 0, 0, 0)' || finding.radius !== '0px' || finding.border !== '0px') fire('quiet', `a finding is drawn as a box: ${JSON.stringify(finding)}`);
        if (!rule || rule[1] !== secondary || Number(rule[2]) !== 1.5) fire('quiet', `a finding hangs on ${finding.rule}, want a 1.5px rule in ${secondary}`);
      }
      if (got.count !== 'rgba(0, 0, 0, 0)') fire('quiet', `the findings' count sits on ${got.count}, want no ground`);
      if (got.kinds.length < 1 || got.kinds.some((kind) => kind !== ink) || got.countInk !== ink) fire('quiet', `a finding's kind or the count is drawn in ${JSON.stringify([...got.kinds, got.countInk])}, want the ink ${ink}`);
    });
    console.log('note-end: each finding hangs on a 1.5px rule in the secondary ink, unboxed, its kind and the bare count in the ink');
  }

  // Every word at the end reads at 4.5:1 or more, in light and in dark.
  if (runs('contrast')) {
    for (const path of [PAGE, COURSE_UNIT]) {
      for (const theme of ['light', 'dark']) {
        await at({ path, theme }, async (page) => {
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
            const texts = [...document.querySelectorAll('.y-related *, .y-colophon *')].filter((element) => element.checkVisibility() && [...element.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() !== ''));
            return texts.map((element) => ({ what: `${element.tagName.toLowerCase()}.${element.className}`, ink: over(getComputedStyle(element).color), ground }));
          });
          if (pairs.length < 10) broken(`only ${pairs.length} pieces of text were read on ${path}`);
          for (const { what, ink, ground } of pairs) {
            const ratio = contrast(ink, ground);
            if (ratio < 4.5) fire('contrast', `in ${theme} on ${path} ${what} is ${ratio.toFixed(2)}:1, want 4.5:1 or more`);
          }
        });
      }
    }
    console.log('note-end: every word at the end reads at 4.5:1 or more in light and in dark');
  }

  // Every line at the end is a whole pixel tall at every text size.
  if (runs('lines')) {
    for (const size of ['', 'l', 'xl']) {
      for (const path of [PAGE, COURSE_UNIT]) {
        await at({ path, size }, async (page) => {
          const lines = await page.evaluate(() => [...document.querySelectorAll('.y-related *, .y-colophon *')].filter((element) => element.checkVisibility() && getComputedStyle(element).display !== 'inline' && element.textContent.trim() !== '').map((element) => ({ what: `${element.tagName.toLowerCase()}.${element.className}`, line: getComputedStyle(element).lineHeight })));
          if (lines.length < 8) broken(`only ${lines.length} lines were read on ${path}`);
          for (const { what, line } of lines) {
            const px = Number.parseFloat(line);
            if (!line.endsWith('px') || Math.abs(px - Math.round(px)) > 0.001) fire('lines', `at size ${size || 'm'} on ${path} ${what} is set on a ${line} line, want a whole pixel`);
          }
        });
      }
    }
    console.log('note-end: every line at the end is a whole pixel tall at all three text sizes');
  }

  // The foot: the path in figures that keep their width, both doors, and no
  // link the header already offers. It arrives with no script running.
  if (runs('foot')) {
    await at({ script: false }, async (page) => {
      const got = await page.evaluate(() => {
        const path = document.querySelector('.y-colophon__path');
        const header = new Set([...document.querySelectorAll('.y-header a[href]')].map((link) => link.getAttribute('href')));
        const foot = [...document.querySelectorAll('.y-colophon__foot a[href]')].map((link) => link.getAttribute('href'));
        return {
          figures: path ? getComputedStyle(path).fontVariantNumeric : null,
          drawn: path?.checkVisibility() ?? false,
          doors: foot.filter((href) => href.startsWith('obsidian://') || href.startsWith('/thought/')).length,
          repeated: foot.filter((href) => header.has(href)),
        };
      });
      if (!got.drawn) fire('foot', 'with no script running the foot does not draw the file\'s path');
      if (!got.figures?.includes('tabular-nums')) fire('foot', `the path is set in ${got.figures} figures, want tabular-nums`);
      if (got.doors !== 2) fire('foot', `the foot carries ${got.doors} doors out of the page, want the editor's and the thought's`);
      if (got.repeated.length > 0) fire('foot', `the foot repeats the header's ${JSON.stringify(got.repeated)}`);
    });
    console.log('note-end: the foot prints the path in tabular figures and both doors, repeats nothing of the header, and needs no script');
  }

  // A phone is never made wider than itself.
  if (runs('phone')) {
    for (const path of [PAGE, COURSE_UNIT]) {
      await at({ path, width: 390 }, async (page) => {
        const got = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, window: innerWidth, right: Math.max(...[...document.querySelectorAll('.y-related, .y-related *, .y-colophon, .y-colophon *')].map((element) => element.getBoundingClientRect().right)) }));
        if (got.page > got.window || got.right > got.window + 0.5) fire('phone', `at 390px on ${path} the page is ${got.page}px wide and the end reaches ${got.right}px`);
      });
    }
    console.log('note-end: at 390px the end fits the phone');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED note-end: the mutation reached ${planted} of ${requests} requests`);
  console.log('PASS note-end: the note ends on what stands beside it and its colophon, as the Atelier note ends');
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
