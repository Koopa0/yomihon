// Behavior lock for the type of a note's body: the words between the head and
// the end of the note, set as the Atelier note sets them.
//
// A section's heading is 22 at the base size, on a whole-pixel line. Every line
// in the column is a whole pixel tall at every text size, because a fractional
// line drifts a restored reading place. Code in a sentence sits on the code
// block's ground with no edge. A link's line stands a quarter em under it, and
// a link at fault keeps that line and changes only its pattern. A quotation and
// an excerpt hang on one strong hairline, in the second ink and the first; a
// callout is the principle: a rule in the second ink with a hairline inside it,
// a warning's rule in the ink, a quotation callout on the quotation's hairline.
// The callout's type word is small, faint and in the interface face, and its
// words are a step under the text. Everything small enough to be furniture (the
// type word, the excerpt's source line, a list's markers, a footnote's mark)
// is still text at 4.5:1 or more in light and in dark, and grows with the
// reader's text size. An excerpt's headings never outrank the section that
// holds them.
//
// Env: YOMIHON_BASE, PAGE_PATH and MUTATE (MUTATE=list prints the modes).
import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';
import { baseUrl as BASE } from './support/base.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/prose-type.md';
const MUTATE = process.env.MUTATE || '';

// The sizes are written out, not read from the stylesheet: a lock that took
// its expected value from the rule under test would agree with any rule.
const SECTION_HEADING_PX = 22;
const CALLOUT_WORDS_PX = 16;

const css = (rule, site) => ({ css: rule, site });
const MODES = {
  // A section's heading back at the size it was before.
  'section-heading-24': css(".y-prose [data-level='2']{font-size:var(--fs-ed-24) !important}", 'headings'),
  // A section's heading on a line of 1.42 that is not rounded.
  'fractional-heading-line': css(".y-prose [data-level='2']{line-height:1.42 !important}", 'lines'),
  // A callout's words on a line of 1.86 that is not rounded.
  'fractional-callout-line': css('.y-prose .callout-body{line-height:1.86 !important}', 'lines'),
  // Code in a sentence on the hover's wash instead of the code ground.
  'code-on-wash': css('.y-prose :not(pre) > code{background:var(--wash) !important}', 'code'),
  // Code in a sentence drawn with an edge.
  'code-with-edge': css('.y-prose :not(pre) > code{border:1px solid var(--line) !important}', 'code'),
  // A link whose line hugs the words.
  'link-line-tight': css('.y-prose a{text-underline-offset:0.1em !important}', 'links'),
  // A broken link drawn with a border under it rather than the link's line.
  'broken-link-border': css('.y-prose .wikilink-broken{text-decoration:none !important;border-bottom:1px dashed var(--warn) !important}', 'links'),
  // A quotation on a 2px rule.
  'heavy-quote-rule': css('.y-prose blockquote{box-shadow:inset 2px 0 0 var(--line-strong) !important}', 'rules'),
  // A callout without the hairline inside its rule.
  'callout-without-hairline': css('.y-prose .callout::before{content:none !important}', 'rules'),
  // A warning drawn as quietly as a note.
  'warning-as-note': css('.y-prose .callout-warning{border-left-color:var(--fg-muted) !important}', 'rules'),
  // A callout's words at the text's own size.
  'callout-words-full-size': css('.y-prose .callout-body{font-size:var(--fs-ed-17) !important}', 'callout'),
  // A callout's type word in the reading face.
  'type-word-in-reading-face': css('.y-prose .callout-title > .callout-type[lang]{font-family:var(--font-read) !important}', 'callout'),
  // The excerpt's source line fixed at the chrome's size.
  'source-line-fixed': css('.y-prose .embed__source{font-size:var(--fs-12) !important}', 'growth'),
  // A list's markers fainter than text may be.
  'markers-too-faint': css('.y-prose li::marker{color:var(--fg-ornament) !important}', 'contrast'),
  // The type word in the ornament ink.
  'type-word-too-faint': css('.y-prose .callout-type{color:var(--fg-ornament) !important}', 'contrast'),
  // An excerpt's title at the size of the note's own first level.
  'excerpt-outranks-section': css(".y-prose .embed [data-level='1']{font-size:var(--fs-ed-30) !important}", 'excerpt'),
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`prose-type: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL prose-type: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const broken = (message) => { throw new Error(`BROKEN prose-type: ${message}`); };
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
  console.log(`prose-type: Chrome ${browser.version()}`);
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
      return route.fulfill({ response, body: `${original}\n${mode.css}\n` });
    });
  };
  const at = async ({ width = 1280, theme = 'light', size = '' }, fn) => {
    const context = await browser.newContext({ viewport: { width, height: 900 }, deviceScaleFactor: 2, colorScheme: theme, reducedMotion: 'reduce' });
    await context.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
      ...(size ? [{ name: 'yomihon_textsize', value: size, url: BASE }] : []),
    ]);
    const page = await context.newPage();
    try {
      await plant(page);
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (response?.status() !== 200) broken(`${PAGE} answered ${response?.status()}, want 200`);
      await page.evaluate(() => document.fonts.ready);
      await arrived(page);
      if (!(await page.content()).includes('PROSE-END')) broken('the fixture page did not arrive whole');
      if (size && (await page.evaluate(() => document.documentElement.dataset.textsize)) !== size) broken(`the page did not take the text size ${size}`);
      return await fn(page);
    } finally {
      await context.close();
    }
  };
  // A colour as the page resolves it, inside element.
  const resolve = (page, selector, value) => page.evaluate(([selector, value]) => {
    const probe = document.createElement('span');
    probe.style.color = value;
    document.querySelector(selector).append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, [selector, value]);
  const one = async (page, selector, read) => {
    const found = await page.locator(selector).count();
    if (found < 1) broken(`${PAGE} holds no ${selector}`);
    return page.locator(selector).first().evaluate(read);
  };

  // The section's heading: 22 at the base size, on a whole-pixel line.
  if (runs('headings')) {
    await at({}, async (page) => {
      const got = await one(page, ".y-prose > .y-sect > [data-level='2']", (h) => Number.parseFloat(getComputedStyle(h).fontSize));
      if (got !== SECTION_HEADING_PX) fire('headings', `a section's heading is ${got}px, want ${SECTION_HEADING_PX}`);
    });
    console.log(`prose-type: a section's heading is ${SECTION_HEADING_PX}px at the base size`);
  }

  // Every line in the column is a whole pixel tall, at every text size. The
// section's margin of actions is the capture work's, not the text's.
  if (runs('lines')) {
    for (const size of ['', 'l', 'xl']) {
      await at({ size }, async (page) => {
        const lines = await page.evaluate(() => [...document.querySelectorAll('.y-prose :is(p, li, [data-level], .callout-title, .callout-body, .footnotes, .embed__source)')].filter((element) => !element.closest('.y-margin')).map((element) => ({
          what: `${element.tagName.toLowerCase()}.${element.className}`,
          line: getComputedStyle(element).lineHeight,
        })));
        if (lines.length < 20) broken(`only ${lines.length} lines were read`);
        for (const { what, line } of lines) {
          const px = Number.parseFloat(line);
          if (!line.endsWith('px') || Math.abs(px - Math.round(px)) > 0.001) fire('lines', `at size ${size || 'm'} ${what} is set on a ${line} line, want a whole pixel`);
        }
      });
    }
    console.log('prose-type: every line in the column is a whole pixel tall at all three text sizes');
  }

  // Code in a sentence: the code ground, no edge, no joined glyphs.
  if (runs('code')) {
    await at({}, async (page) => {
      const ground = await resolve(page, '.y-prose', 'var(--code-ground)');
      const code = await one(page, '.y-prose p > code', (c) => {
        const style = getComputedStyle(c);
        return { ground: style.backgroundColor, edge: style.borderTopWidth, ring: style.boxShadow, ligatures: style.fontVariantLigatures, ratio: Number.parseFloat(style.fontSize) / Number.parseFloat(getComputedStyle(c.parentElement).fontSize) };
      });
      if (code.ground !== ground) fire('code', `code in a sentence sits on ${code.ground}, want the code ground ${ground}`);
      if (code.edge !== '0px' || code.ring !== 'none') fire('code', `code in a sentence has an edge (${code.edge}, ${code.ring}), want the tone alone`);
      if (code.ligatures !== 'none') fire('code', `code in a sentence joins glyphs (${code.ligatures})`);
      if (Math.abs(code.ratio - 0.82) > 0.01) fire('code', `code in a sentence is ${code.ratio} of its sentence, want 0.82`);
    });
    console.log('prose-type: code in a sentence sits on the code ground, with no edge, at 0.82 of its sentence');
  }

  // A link's line, and a link at fault keeping that line.
  if (runs('links')) {
    await at({}, async (page) => {
      const warn = await resolve(page, '.y-prose', 'var(--warn)');
      const read = (a) => {
        const style = getComputedStyle(a);
        return { line: style.textDecorationLine, pattern: style.textDecorationStyle, colour: style.textDecorationColor, offset: Number.parseFloat(style.textUnderlineOffset) / Number.parseFloat(style.fontSize), border: Number.parseFloat(style.borderBottomWidth) > 0 && style.borderBottomStyle !== 'none' ? style.borderBottomStyle : 'none' };
      };
      const link = await one(page, '.y-prose p > a.wikilink:not(.wikilink-degraded)', read);
      if (link.line !== 'underline' || Math.abs(link.offset - 0.26) > 0.01) fire('links', `a link's line is ${JSON.stringify(link)}, want an underline 0.26em under it`);
      const fault = await one(page, '.y-prose .wikilink-broken', read);
      if (fault.line !== 'underline' || fault.pattern !== 'dashed' || fault.colour !== warn || Math.abs(fault.offset - link.offset) > 0.01 || fault.border !== 'none') fire('links', `a broken link is drawn ${JSON.stringify(fault)}, want the link's own line, dashed, in ${warn}`);
      const degraded = await one(page, '.y-prose .wikilink-degraded', read);
      if (degraded.line !== 'underline' || degraded.pattern !== 'dotted' || Math.abs(degraded.offset - link.offset) > 0.01 || degraded.border !== 'none') fire('links', `a link that lost its place is drawn ${JSON.stringify(degraded)}, want the link's own line, dotted`);
    });
    console.log('prose-type: a link\'s line stands 0.26em under it, and a link at fault keeps that line and changes its pattern');
  }

  // The rules down a quotation, an excerpt and the callouts.
  if (runs('rules')) {
    await at({}, async (page) => {
      const strong = await resolve(page, '.y-prose', 'var(--line-strong)');
      const muted = await resolve(page, '.y-prose', 'var(--fg-muted)');
      const ink = await resolve(page, '.y-prose', 'var(--fg)');
      const rule = (element) => {
        const style = getComputedStyle(element);
        const inner = getComputedStyle(element, '::before');
        // A 1.5px line is an inset shadow, since a border is drawn in whole
        // pixels; a whole-pixel rule is a border.
        const shadow = style.boxShadow.match(/^(.*\))\s+([\d.]+)px\s+0px\s+0px\s+0px\s+inset$/);
        const border = Number.parseFloat(style.borderLeftWidth) > 0 && style.borderLeftStyle !== 'none';
        const drawn = border ? { width: Number.parseFloat(style.borderLeftWidth), colour: style.borderLeftColor } : shadow ? { width: Number(shadow[2]), colour: shadow[1] } : { width: 0, colour: null };
        return { ...drawn, both: border && Boolean(shadow), ink: style.color, hairline: inner.content !== 'none' && inner.content !== 'normal' ? { width: Number.parseFloat(inner.width), colour: inner.backgroundColor } : null };
      };
      const quote = await one(page, '.y-prose > blockquote', rule);
      if (quote.width !== 1.5 || quote.colour !== strong || quote.ink !== muted || quote.hairline || quote.both) fire('rules', `a quotation hangs on ${JSON.stringify(quote)}, want one 1.5px hairline in ${strong} and its words in ${muted}`);
      const excerpt = await one(page, '.y-prose > .embed', rule);
      if (excerpt.width !== 1.5 || excerpt.colour !== strong || excerpt.hairline || excerpt.both) fire('rules', `an excerpt hangs on ${JSON.stringify(excerpt)}, want one 1.5px hairline in ${strong}`);
      const note = await one(page, '.y-prose > .callout-note', rule);
      if (note.width !== 2 || note.colour !== muted || !note.hairline || note.hairline.colour !== strong) fire('rules', `a callout is ruled ${JSON.stringify(note)}, want 2px in ${muted} with a hairline in ${strong} inside it`);
      const warning = await one(page, '.y-prose > .callout-warning', rule);
      if (warning.width !== 2 || warning.colour !== ink || !warning.hairline) fire('rules', `a warning is ruled ${JSON.stringify(warning)}, want 2px in the ink ${ink} with the hairline inside it`);
      const cited = await one(page, '.y-prose > .callout-quote', rule);
      if (cited.width !== 1.5 || cited.colour !== strong || cited.hairline || cited.both) fire('rules', `a quotation callout is ruled ${JSON.stringify(cited)}, want the quotation's one hairline`);
    });
    console.log('prose-type: a quotation and an excerpt hang on one strong hairline; a callout on a rule with a hairline inside it, a warning\'s in the ink');
  }

  // The callout's title row and words.
  if (runs('callout')) {
    await at({}, async (page) => {
      const sans = await page.evaluate(() => {
        const probe = document.createElement('span');
        probe.style.fontFamily = 'var(--font-sans)';
        document.querySelector('.y-prose').append(probe);
        const family = getComputedStyle(probe).fontFamily;
        probe.remove();
        return family;
      });
      const got = await one(page, '.y-prose > .callout-note', (callout) => {
        const type = callout.querySelector('.callout-type');
        const body = callout.querySelector('.callout-body');
        const text = document.querySelector('.y-prose > p');
        return {
          typeFace: getComputedStyle(type).fontFamily,
          typeSize: Number.parseFloat(getComputedStyle(type).fontSize),
          titleSize: Number.parseFloat(getComputedStyle(type.parentElement).fontSize),
          bodySize: Number.parseFloat(getComputedStyle(body).fontSize),
          textSize: Number.parseFloat(getComputedStyle(text).fontSize),
        };
      });
      if (got.typeFace !== sans) fire('callout', `a callout's type word is set in ${got.typeFace}, want the interface face ${sans}`);
      if (!(got.typeSize < got.titleSize)) fire('callout', `a callout's type word is ${got.typeSize}px beside a ${got.titleSize}px title, want it smaller`);
      if (got.bodySize !== CALLOUT_WORDS_PX || !(got.bodySize < got.textSize)) fire('callout', `a callout's words are ${got.bodySize}px beside ${got.textSize}px text, want ${CALLOUT_WORDS_PX}, a step under it`);
    });
    console.log('prose-type: a callout\'s type word is small and in the interface face, and its words are a step under the text');
  }

  // The small things grow with the reader's text size.
  if (runs('growth')) {
    const sizes = {};
    for (const size of ['', 'xl']) {
      await at({ size }, async (page) => {
        sizes[size || 'm'] = await page.evaluate(() => Object.fromEntries(['.y-prose .embed__source', '.y-prose .callout-type', '.y-prose .callout-title', '.y-prose .callout-body', '.y-prose .footnotes', '.y-prose .footnote-ref'].map((selector) => [selector, Number.parseFloat(getComputedStyle(document.querySelector(selector)).fontSize)])));
      });
    }
    for (const [selector, base] of Object.entries(sizes.m)) {
      if (!(sizes.xl[selector] > base)) fire('growth', `${selector} is ${base}px at the base size and ${sizes.xl[selector]}px at 特大, want it to grow with the text`);
    }
    console.log('prose-type: the type word, the title, the callout\'s words, the excerpt\'s source line and the footnotes grow with the reader\'s text size');
  }

  // Text that is furniture is still text: 4.5:1 or more in light and in dark.
  if (runs('contrast')) {
    for (const theme of ['light', 'dark']) {
      await at({ theme }, async (page) => {
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
          // The paint under an element: the first ancestor with a ground of
          // its own, composited over the page.
          const under = (element) => {
            const page = rgb(getComputedStyle(document.body).backgroundColor);
            for (let at = element; at; at = at.parentElement) {
              const ground = getComputedStyle(at).backgroundColor;
              if (ground !== 'rgba(0, 0, 0, 0)' && at !== document.body && at !== document.documentElement) {
                const [r, g, b, a] = rgb(ground);
                return page.slice(0, 3).map((c, i) => Math.round(c * (1 - a / 255) + [r, g, b][i] * (a / 255)));
              }
            }
            return page.slice(0, 3);
          };
          const over = (colour, ground) => {
            const [r, g, b, a] = rgb(colour);
            return [r, g, b].map((c, i) => Math.round(ground[i] * (1 - a / 255) + c * (a / 255)));
          };
          const pick = (selector, pseudo = null) => [...document.querySelectorAll(selector)].map((element) => {
            const ground = under(element);
            return { what: `${selector}${pseudo || ''}`, ink: over(getComputedStyle(element, pseudo).color, ground), ground };
          });
          return [
            ...pick('.y-prose .callout-type'),
            ...pick('.y-prose .callout-title'),
            ...pick('.y-prose .callout-body'),
            ...pick('.y-prose blockquote'),
            ...pick('.y-prose .embed__source'),
            ...pick('.y-prose .footnote-ref'),
            ...pick('.y-prose .footnotes'),
            ...pick('.y-prose :not(pre) > code'),
            ...pick('.y-prose kbd'),
            ...pick('.y-prose .wikilink-broken'),
            ...pick('.y-prose li', '::marker'),
          ];
        });
        if (pairs.length < 15) broken(`only ${pairs.length} pieces of text were read`);
        for (const { what, ink, ground } of pairs) {
          const ratio = contrast(ink, ground);
          if (ratio < 4.5) fire('contrast', `in ${theme} ${what} is ${ratio.toFixed(2)}:1 (${ink} on ${ground}), want 4.5:1 or more`);
        }
      });
    }
    console.log('prose-type: every small piece of the column\'s text reads at 4.5:1 or more in light and in dark');
  }

  // An excerpt's headings never outrank the section that holds them.
  if (runs('excerpt')) {
    await at({}, async (page) => {
      const got = await page.evaluate(() => ({
        section: Number.parseFloat(getComputedStyle(document.querySelector(".y-prose > .y-sect > [data-level='2']")).fontSize),
        excerpt: [...document.querySelectorAll('.y-prose .embed [data-level]')].map((h) => Number.parseFloat(getComputedStyle(h).fontSize)),
      }));
      if (got.excerpt.length < 1) broken('the excerpt holds no heading');
      for (const size of got.excerpt) {
        if (size >= got.section) fire('excerpt', `an excerpt's heading is ${size}px inside a ${got.section}px section, want it smaller`);
      }
    });
    console.log('prose-type: an excerpt\'s headings stay under the section that holds them');
  }

  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED prose-type: the mutation reached ${planted} of ${requests} requests`);
  console.log('PASS prose-type: the body is set as the Atelier note sets it, on whole-pixel lines, with its furniture legible and growing with the text');
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
