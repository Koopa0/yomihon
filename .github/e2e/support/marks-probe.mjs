// What the probes of the reader's own marks share: how a mutation is planted and
// reported, a browser context with the page's cookies, the words a probe chooses
// in a page, and the colours the page resolves.
//
// A probe built on this names its sites, each a place an assertion lives, and a
// table of mutations, each with the site that has to fire for it. MUTATE=list
// prints the table; a mutation that fires its own site prints
// `MUTATE-RESULT: caught <mode>`, and one that reached nothing prints
// `MUTATE-RESULT: not-applied <mode>`.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './base.mjs';

export const NOTE = '/notes/Notes/marks-on-page.md';
export { BASE };

// The words a page of marks must never say: where a mark is kept, what keeps it
// or where else it goes. The reader only reads.
export const STORAGE_WORDS = /儲存|保存|存檔|同步|備份|另一台|其他電腦|本機|機器檔|\bgit\b|\bsync|storage|backup|other computer|machine file/i;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class NotApplied extends Error {}

export function defineProbe(name, sites, modes) {
  const MUTATE = process.env.MUTATE || '';
  if (MUTATE === 'list') {
    console.log(Object.keys(modes).join('\n'));
    process.exit(0);
  }
  if (MUTATE && !modes[MUTATE]) {
    console.error(`${name}: unknown MUTATE mode ${MUTATE}`);
    process.exit(2);
  }
  for (const [mode, row] of Object.entries(modes)) {
    if (!sites.includes(row.site)) throw new Error(`BROKEN ${name}: the mutation ${mode} names the unknown site ${row.site}`);
  }
  const mode = MUTATE ? modes[MUTATE] : null;
  const planted = { seen: 0, matched: 0 };

  const runs = (site) => !mode || mode.site === site;
  const fire = (site, message) => {
    if (!sites.includes(site)) throw new Error(`BROKEN ${name}: unknown assertion site ${site}`);
    throw new LockFired(site, `FAIL ${name}: ${message}`);
  };
  const broken = (message) => {
    throw new Error(`BROKEN ${name}: ${message}`);
  };

  // plant installs the mutation on a page before it loads.
  async function plant(page) {
    if (!mode) return;
    if (mode.css) {
      await page.route('**/static/app.css{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        planted.seen += 1;
        planted.matched += 1;
        await route.fulfill({ response, body: `${original}\n${mode.css}\n` });
      });
    }
    if (mode.script) {
      const file = mode.script.file;
      await page.route(new RegExp(`/static/${file.replace('.', '\\.')}(\\?.*)?$`), async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        planted.seen += 1;
        if (original.split(mode.script.needle).length === 2) planted.matched += 1;
        await route.fulfill({ response, body: original.replace(mode.script.needle, mode.script.replacement) });
      });
    }
    if (mode.doc) {
      await page.route((url) => url.pathname === decodeURI(NOTE) || url.pathname === NOTE, async (route) => {
        const response = await route.fetch();
        if (!(response.headers()['content-type'] ?? '').includes('text/html')) {
          await route.fulfill({ response });
          return;
        }
        const original = await response.text();
        planted.seen += 1;
        if (original.includes(mode.doc.needle)) planted.matched += 1;
        await route.fulfill({ response, body: original.replaceAll(mode.doc.needle, mode.doc.replacement) });
      });
    }
    if (mode.routes) {
      await mode.routes(page, planted);
    }
  }

  async function run(body) {
    const browser = await chromium.launch({ channel: 'chrome', headless: true });
    let failure = null;
    try {
      console.log(`${name}: Chrome ${browser.version()}`);
      // open is one page in a context of its own: cookies the page reads, the
      // window it is read in, and the mutation if this run has one.
      const open = async ({ path = NOTE, width = 1512, height = 1000, theme = 'light', script = true, touch = false, cookies = {}, size = '', plantHere = true } = {}) => {
        const context = await browser.newContext({
          viewport: { width, height },
          colorScheme: theme,
          reducedMotion: 'reduce',
          javaScriptEnabled: script,
          ...(touch ? { hasTouch: true, isMobile: true } : {}),
        });
        await context.addCookies(
          Object.entries({ yomihon_rail: 'collapsed', yomihon_theme: theme, ...(size ? { yomihon_textsize: size } : {}), ...cookies }).map(([key, value]) => ({ name: key, value, url: BASE })),
        );
        const page = await context.newPage();
        if (plantHere) await plant(page);
        const response = await page.goto(BASE + path, { waitUntil: 'load' });
        if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
        await page.evaluate(() => document.fonts.ready);
        if (script) await page.waitForFunction(() => document.documentElement.dataset.marksLifted !== undefined, null, { timeout: 5000 }).catch(() => {});
        return { page, context, close: () => context.close() };
      };
      await body({ browser, open });
      if (mode && (planted.seen < 1 || (planted.matched < 1 && !mode.routes))) throw new NotApplied(`NOT-APPLIED ${name}: the mutation reached ${planted.matched} of ${planted.seen} requests`);
      console.log(`PASS ${name}`);
    } catch (error) {
      failure = error;
    } finally {
      await browser.close();
    }
    if (failure) {
      console.error(failure.message);
      if (failure instanceof NotApplied) {
        console.log(`MUTATE-RESULT: not-applied ${process.env.MUTATE}`);
        process.exitCode = 2;
      } else {
        if (mode && failure instanceof LockFired && failure.site === mode.site) console.log(`MUTATE-RESULT: caught ${process.env.MUTATE}`);
        process.exitCode = 1;
      }
    }
  }

  return { runs, fire, broken, run, mode };
}

// The words of a note a probe chooses are looked for in the text of the page that
// is not a mark, a note or the page's own parts, and chosen as a person would, by
// dragging across them.
export async function choose(page, words) {
  const done = await page.evaluate((words) => {
    const prose = document.querySelector('.y-article > .y-prose');
    const walker = document.createTreeWalker(prose, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      const at = node.data.indexOf(words);
      if (at < 0 || node.parentElement.closest('.y-mnote, .y-margin, .y-mfield, .y-mlayer, .y-mbar, .y-mstatus')) continue;
      const range = document.createRange();
      range.setStart(node, at);
      range.setEnd(node, at + words.length);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      return true;
    }
    return false;
  }, words);
  if (!done) throw new Error(`BROKEN: the words ${JSON.stringify(words)} are not in the page as plain text`);
  await page.waitForSelector('[data-marks-toolbar]:not([hidden])', { timeout: 3000 });
}

// The marks drawn in the page, by id: the words of each, its state, and its note.
export const drawn = (page) =>
  page.evaluate(() => {
    const out = new Map();
    for (const piece of document.querySelectorAll('mark[data-mark]')) {
      const id = piece.dataset.mark;
      const entry = out.get(id) ?? { id, kind: piece.dataset.kind, state: piece.dataset.state, text: '', note: null };
      entry.text += piece.textContent;
      out.set(id, entry);
    }
    for (const note of document.querySelectorAll('.y-mnote[data-mnote]')) {
      const id = note.dataset.mnote;
      const entry = out.get(id) ?? { id, kind: note.dataset.kind, state: note.dataset.state, text: '', note: null };
      entry.note = { words: note.querySelector('.y-mnote__words')?.textContent ?? '', label: note.getAttribute('aria-label'), state: note.dataset.state };
      out.set(id, entry);
    }
    return [...out.values()];
  });

// A mark made without the page, as another tab or another run would have made
// it: a body the server keeps under the id.
export async function keepByHand(context, { kind, quote, body = '', before = '', after = '', anchor = '', path = 'Notes/marks-on-page.md' }) {
  const id = [...Array(32)].map(() => Math.floor(Math.random() * 16).toString(16)).join('');
  const response = await context.request.post(`${BASE}/mark`, { data: { id, kind, path, anchor, quote, before, after, body } });
  if (![200, 201].includes(response.status())) throw new Error(`BROKEN: keeping a ${kind} by hand answered ${response.status()}: ${await response.text()}`);
  return response.json();
}

// The colour the page resolves for a token or colour, as the numbers it is drawn in.
export const resolveColour = (page, value) =>
  page.evaluate((value) => {
    const probe = document.createElement('span');
    probe.style.color = value;
    document.body.append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, value);

// rgb(a) text to [r, g, b].
export const channels = (text) => (text.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number);

// The relative luminance and contrast of WCAG 2, over sRGB.
const channel = (c) => {
  const v = c / 255;
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
};
export const luminance = ([r, g, b]) => 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
export const contrast = (a, b) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};
