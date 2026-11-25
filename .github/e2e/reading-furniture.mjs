// Behavior lock for the furniture of a note's page: the bookmark's ribbon, the
// numbers hung beside the sections, and the line along the header that says
// how far through the note the reader is.
//
// The ribbon is a picture of the header's control. It lies on the page while
// this note holds the bookmark and is drawn up out of sight while it does
// not, from the first byte with or without a script, and it follows the
// control when the reader keeps a place or the place moves to another note.
// It falls at its own tempo once the page has painted, not at all for a
// reader who asked for less motion, and never on arrival. It takes no
// pointer, moves nothing on the page, hangs from the header's edge, and covers
// no control and no line of the note at any width.
//
// The numbers count a note's sections beside their headings, and only a note
// whose author did not number them already: the lesson numbered 第一節 and the
// note with one 第三節 among its headings get none. A number is no part of
// its heading's name. It hangs in the margin where the margin holds it and
// stands inline before the heading where it does not, never off the window
// and never under the left column.
//
// The line grows from nothing at the top of the note to the full width at its
// end, along the header's lower edge and without changing the header, on a
// note's page only, and it keeps tracking for a reader who asked for less
// motion, because it moves only as they move the page.
//
// It keeps a place of its own on the disposable fixture, so it is one of the
// probes that leave one behind and has to stand among the last (see probes.sh).
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with unnumbered ## sections), and
// MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Reading%20flow.md';
const MUTATE = process.env.MUTATE || '';
const NOTE = decodeURIComponent(PAGE.replace(/^\/notes\//, ''));
const ELSEWHERE = 'Notes/alpha.md';
// A note whose author numbered every section, and one that numbered one of
// them: neither is counted again.
const NUMBERED = ['/notes/Writing/lessons/japanese/L01.md', '/notes/Notes/Glass%20Tide.md'];
// A note in Han with no language of its own, so it keeps the full measure,
// which at the largest text size leaves the margin too narrow for a number
// beside the open left column at 1321.
const WIDE_HAN = '/notes/Notes/%E7%81%B0%E5%B8%83%E5%B8%B3%E5%86%8A.md';

const RIBBON_WIDTHS = [
  { width: 390, rail: 'collapsed' },
  { width: 1100, rail: 'collapsed' },
  { width: 1321, rail: 'open' },
  { width: 1512, rail: 'open' },
  { width: 2560, rail: 'open' },
];
const NUMBER_CASES = [
  { path: PAGE, width: 390, rail: 'collapsed', size: 'm', hang: false },
  { path: PAGE, width: 820, rail: 'collapsed', size: 'm', hang: false },
  { path: PAGE, width: 821, rail: 'collapsed', size: 'm', hang: true },
  { path: PAGE, width: 1100, rail: 'collapsed', size: 'm', hang: true },
  { path: PAGE, width: 1321, rail: 'open', size: 'm', hang: true },
  { path: PAGE, width: 1512, rail: 'open', size: 'm', hang: true },
  { path: PAGE, width: 2560, rail: 'open', size: 'm', hang: true },
  { path: WIDE_HAN, width: 821, rail: 'collapsed', size: 'xl', hang: false },
  { path: WIDE_HAN, width: 1321, rail: 'open', size: 'xl', hang: false },
  { path: WIDE_HAN, width: 1512, rail: 'open', size: 'xl', hang: true },
];
const HEIGHT = 900;
// The number's distance left of the text, and the least room it keeps from
// the window's edge or the open left column.
const HANG = 46;
const SLACK = 0.5;

const SITES = [
  'ribbon-shows-the-bookmark',
  'ribbon-lies-still-on-arrival',
  'ribbon-without-script',
  'ribbon-follows-the-control',
  'ribbon-falls-at-its-tempo',
  'ribbon-is-a-picture',
  'ribbon-moves-nothing',
  'ribbon-hangs-from-the-header',
  'ribbon-covers-nothing',
  'sections-are-counted',
  'counted-once',
  'number-outside-the-name',
  'numbers-hang-where-there-is-room',
  'numbers-stay-clear',
  'progress-tracks-the-reading',
  'progress-on-the-header-edge',
  'progress-only-on-a-note',
  'progress-under-reduced-motion',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN reading-furniture: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL reading-furniture: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN reading-furniture: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED reading-furniture: ${message}`); };

// Each regression is planted in what the browser is served — the stylesheet,
// the module or the page itself — on every context the run opens, and counts
// what it touched, so a needle that matches nothing is reported rather than
// passed over.
const served = (pattern, edit, label) => {
  let loads = 0;
  let misses = 0;
  return {
    install: (context) => context.route(pattern, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      const changed = edit(original);
      loads += 1;
      if (changed === original) misses += 1;
      await route.fulfill({ response, body: changed });
    }),
    proof: () => {
      if (loads === 0) return `${label}: nothing it rewrites was served`;
      if (misses > 0) return `${label}: ${misses} of ${loads} served copies were left as they were`;
      return '';
    },
  };
};
const appendStyle = (css) => served('**/static/app.css{,?*}', (text) => `${text}\n${css}\n`, 'the appended rule');
const once = (needle, replacement) => (text) => (text.split(needle).length === 2 ? text.replace(needle, replacement) : text);
const rewriteStyle = (needle, replacement) => served('**/static/app.css{,?*}', once(needle, replacement), `the stylesheet needle ${JSON.stringify(needle)}`);
const rewriteModule = (needle, replacement) => served('**/mark.js{,?*}', once(needle, replacement), `the module needle ${JSON.stringify(needle)}`);
const rewritePage = (path, needle, replacement) => served(`${BASE}${path}`, once(needle, replacement), `the page needle ${JSON.stringify(needle)} on ${path}`);
// The header's end is the first </header> a page carries; a page may close
// more than one header after it.
const intoHeader = (path, html) => served(`${BASE}${path}`, (text) => text.replace('</header>', `${html}</header>`), `the header of ${path}`);

const KEPT_RULE = ':root:has([data-mark-control][data-mark-kept]) .y-ribbon__cloth {';
const MUTATIONS = {
  // The ribbon is lowered by a class the script adds after the first paint,
  // so a note opened on its bookmark shows the ribbon falling.
  'ribbon-falls-on-load': { target: 'ribbon-lies-still-on-arrival', plant: rewriteStyle(KEPT_RULE, ':root.is-ready:has([data-mark-control][data-mark-kept]) .y-ribbon__cloth {') },
  'ribbon-always-down': { target: 'ribbon-shows-the-bookmark', plant: appendStyle('.y-ribbon__cloth.y-ribbon__cloth{translate:none}') },
  'ribbon-never-down': { target: 'ribbon-shows-the-bookmark', plant: rewriteStyle(KEPT_RULE, ':root:has([data-mark-control][data-never]) .y-ribbon__cloth {') },
  // The picture waits for the script to say it may be drawn.
  'ribbon-needs-script': { target: 'ribbon-without-script', plant: rewriteStyle(KEPT_RULE, ":root[data-js='on']:has([data-mark-control][data-mark-kept]) .y-ribbon__cloth {") },
  // The control stops writing its state where the picture reads it.
  'ribbon-deaf-to-the-control': { target: 'ribbon-follows-the-control', plant: rewriteModule("    face.toggleAttribute('data-mark-kept', kept);\n", '') },
  'ribbon-snaps': { target: 'ribbon-falls-at-its-tempo', plant: appendStyle('.y-ribbon__cloth.y-ribbon__cloth{transition:none}') },
  'ribbon-moves-under-reduce': { target: 'ribbon-falls-at-its-tempo', plant: appendStyle('@media (prefers-reduced-motion: reduce){.y-ribbon__cloth.y-ribbon__cloth{transition-duration:420ms!important}}') },
  'ribbon-takes-the-pointer': { target: 'ribbon-is-a-picture', plant: appendStyle('.y-ribbon.y-ribbon{pointer-events:auto;cursor:pointer}') },
  'ribbon-in-flow': { target: 'ribbon-moves-nothing', plant: appendStyle('.y-ribbon.y-ribbon{display:block;position:relative;inset:auto}') },
  'ribbon-holds-room-when-kept': { target: 'ribbon-moves-nothing', plant: appendStyle(':root:has([data-mark-control][data-mark-kept]) .y-ribbon.y-ribbon{display:block;position:relative;inset:auto}') },
  'ribbon-floats-off-the-header': { target: 'ribbon-hangs-from-the-header', plant: appendStyle('.y-ribbon.y-ribbon{inset-block-start:0}') },
  // The room above the head tightened under the ribbon, which then lies over
  // the row naming where the note is.
  'ribbon-over-the-text': { target: 'ribbon-covers-nothing', plant: appendStyle('.y-article.y-article{--read-top:24px}') },
  'ribbon-inside-the-column-on-a-phone': { target: 'ribbon-covers-nothing', plant: appendStyle('@media (max-width: 900px){.y-ribbon.y-ribbon{inset-inline-end:6px}}') },
  'numbers-dropped': { target: 'sections-are-counted', plant: rewritePage(PAGE, '<div class="y-prose" data-section-numbers>', '<div class="y-prose">') },
  'numbers-doubled': { target: 'counted-once', plant: rewritePage(NUMBERED[0], '<div class="y-prose">', '<div class="y-prose" data-section-numbers>') },
  // The number written into the heading as text, as a design's own markup
  // draws it, rather than generated beside it.
  'number-in-the-name': { target: 'number-outside-the-name', plant: rewritePage(PAGE, 'id="the-first-mechanism" data-level="2">', 'id="the-first-mechanism" data-level="2"><span>01 </span>') },
  'numbers-hang-at-820': { target: 'numbers-hang-where-there-is-room', plant: rewriteStyle('@media (min-width: 821px) {\n  @container y-main (width >= 48rem)', '@media (min-width: 800px) {\n  @container y-main (width >= 48rem)') },
  'numbers-never-hang': { target: 'numbers-hang-where-there-is-room', plant: appendStyle('.y-prose.y-prose.y-prose.y-prose.y-prose{--section-hang:initial}') },
  'numbers-off-the-window': { target: 'numbers-stay-clear', plant: appendStyle('@media (max-width: 720px){.y-prose.y-prose.y-prose.y-prose.y-prose{--section-hang:46px}}') },
  'numbers-under-the-rail': { target: 'numbers-stay-clear', plant: rewriteStyle("    :root:not([data-rail='collapsed'])[data-textsize='xl'] .y-prose {\n      --section-hang: initial;", "    :root:not([data-rail='collapsed'])[data-textsize='xl'] .y-prose {\n      --section-hang: 46px;") },
  // The timeline the line read before it was named as the root's: a scope
  // lifted to the page's wrapper hides the root's timeline from the header.
  'progress-line-stuck': { target: 'progress-tracks-the-reading', plant: appendStyle('.yomihon{timeline-scope:--y-read}html{scroll-timeline:--y-read block}@supports (animation-timeline: scroll()){.y-readline.y-readline{animation-timeline:--y-read}}') },
  'progress-line-at-the-top': { target: 'progress-on-the-header-edge', plant: appendStyle('.y-readline.y-readline{top:0;bottom:auto}') },
  'progress-on-the-desk': { target: 'progress-only-on-a-note', plant: intoHeader('/', '<span class="y-readline" aria-hidden="true"></span>') },
  'progress-dropped-under-reduce': { target: 'progress-under-reduced-motion', plant: appendStyle('@media (prefers-reduced-motion: reduce){.y-readline.y-readline{animation:none}}') },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`reading-furniture: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`reading-furniture: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reading-furniture: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}
const planted = MUTATE ? MUTATIONS[MUTATE].plant : null;

const browser = await chromium.launch({ channel: 'chrome', headless: true });

// open is a fresh reader: a window of the given width, the column as the
// cookie leaves it, the text size chosen, and whatever regression this run
// plants.
const open = async ({ width, rail = 'collapsed', size = 'm', script = true, motion = 'no-preference' }) => {
  const context = await browser.newContext({
    viewport: { width, height: HEIGHT },
    javaScriptEnabled: script,
    reducedMotion: motion,
  });
  await context.addCookies([
    { name: 'yomihon_rail', value: rail, url: BASE },
    { name: 'yomihon_textsize', value: size, url: BASE },
  ]);
  if (planted) await planted.install(context);
  return context;
};

const visit = async (context, path, { initScript } = {}) => {
  const page = await context.newPage();
  if (initScript) await page.addInitScript(initScript);
  const response = await page.goto(`${BASE}${path}`, { waitUntil: 'load' });
  if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
  return page;
};

// keepIn keeps the reader's one place in a note, as the header's control does,
// so the page drawn next is drawn by a server that holds it.
const keepIn = async (path) => {
  const response = await fetch(`${BASE}/bookmark`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded', Origin: BASE },
    body: new URLSearchParams({ path, anchor: '', offset: '0', identity: '0'.repeat(64) }),
  });
  if (response.status !== 204) broken(`keeping a place in ${path} answered ${response.status}, want 204`);
};

// settled waits until the ribbon has come to rest where the header's control
// says it should: the page's own reading of the place may still be on its way
// when the page has loaded, and the ribbon falls or is drawn up once it lands.
// A ribbon that never comes to rest there is read as it stands after a while,
// for the assertion to judge.
const settled = async (page) => {
  await page.waitForFunction(() => {
    const box = document.querySelector('.y-ribbon');
    const cloth = document.querySelector('.y-ribbon__cloth');
    if (!box || !cloth) return true;
    if (cloth.getAnimations().length > 0) return false;
    const kept = document.querySelector('[data-mark-control]')?.hasAttribute('data-mark-kept');
    const offset = cloth.getBoundingClientRect().top - box.getBoundingClientRect().top;
    return kept ? Math.abs(offset) <= 0.5 : cloth.getBoundingClientRect().bottom <= box.getBoundingClientRect().top + 0.5;
  }, null, { timeout: 2000 }).catch(() => {});
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.evaluate(() => Promise.all((document.querySelector('.y-ribbon__cloth')?.getAnimations() ?? [])
    .map((animation) => animation.finished.catch(() => {}))));
};

// ribbon reads where the cloth is drawn and where the header's edge is.
const ribbon = (page) => page.evaluate(() => {
  const box = document.querySelector('.y-ribbon');
  const cloth = document.querySelector('.y-ribbon__cloth');
  if (!box || !cloth) return null;
  const rect = (element) => {
    const r = element.getBoundingClientRect();
    return { left: r.left, top: r.top, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
  };
  return {
    box: rect(box),
    cloth: rect(cloth),
    header: rect(document.querySelector('.y-header')),
    kept: document.querySelector('[data-mark-control]')?.hasAttribute('data-mark-kept') ?? null,
  };
});
// A ribbon is down when its cloth fills its box, and up when no part of the
// cloth is left below the edge its box clips at.
const isDown = (r) => Math.abs(r.cloth.top - r.box.top) <= SLACK && Math.abs(r.cloth.height - 42) <= SLACK;
const isUp = (r) => r.cloth.bottom <= r.box.top + SLACK;

try {
  // --- The ribbon shows the bookmark, with a script and without ---
  //
  // Read once the page has settled. With a script that is after the page has
  // painted and the motion has arrived; the arrival itself is read below.
  const showsTheKeptPlace = async (script) => {
    const site = script ? 'ribbon-shows-the-bookmark' : 'ribbon-without-script';
    const how = script ? 'with the script' : 'with no script';
    for (const [where, here] of [[NOTE, true], [ELSEWHERE, false]]) {
      await keepIn(where);
      const context = await open({ width: 1512, rail: 'open', script });
      const page = await visit(context, PAGE);
      if (script) await page.waitForFunction(() => document.documentElement.classList.contains('is-ready'));
      await settled(page);
      const r = await ribbon(page);
      if (!r) broken(`${PAGE} draws no ribbon`);
      if (r.kept !== here) broken(`the header's control says kept=${r.kept} with the place in ${where}`);
      if (here && !isDown(r)) fail(site, `${how}, the note holding the bookmark draws its ribbon ${(r.cloth.top - r.box.top).toFixed(1)}px off the header's edge, want it lying there`);
      if (!here && !isUp(r)) fail(site, `${how}, a note not holding the bookmark still shows ${(r.cloth.bottom - r.box.top).toFixed(1)}px of ribbon below the header's edge`);
      await context.close();
    }
  };
  await showsTheKeptPlace(true);

  // --- The ribbon on arrival ---
  //
  // Every frame from the first paint of a note opened on its bookmark shows
  // the ribbon already lying there.
  {
    await keepIn(NOTE);
    const context = await open({ width: 1512, rail: 'open' });
    const page = await visit(context, PAGE, {
      initScript: () => {
        const frames = [];
        window.__ribbonFrames = frames;
        const sample = () => {
          const box = document.querySelector('.y-ribbon');
          const cloth = document.querySelector('.y-ribbon__cloth');
          if (box && cloth) frames.push(cloth.getBoundingClientRect().top - box.getBoundingClientRect().top);
          if (frames.length < 60) requestAnimationFrame(sample);
        };
        requestAnimationFrame(sample);
      },
    });
    await page.waitForFunction(() => window.__ribbonFrames.length >= 40, null, { timeout: 5000 });
    const frames = await page.evaluate(() => window.__ribbonFrames);
    const moving = frames.filter((offset) => Math.abs(offset) > SLACK);
    if (moving.length > 0) {
      fail('ribbon-lies-still-on-arrival', `a note opened on its bookmark drew the ribbon ${moving.length} of ${frames.length} frames off its resting place (first ${moving[0].toFixed(1)}px); it has to be lying there from the first frame`);
    }
    await context.close();
  }
  await showsTheKeptPlace(false);

  // --- The ribbon follows the control ---
  //
  // A press keeps the place here and the ribbon falls. A page drawn while the
  // place was here and shown after it moved elsewhere — a copy from the
  // browser's cache — is put right by the script's reading of the place, and
  // the ribbon is drawn up.
  {
    await keepIn(ELSEWHERE);
    const context = await open({ width: 1100 });
    const page = await visit(context, PAGE);
    const before = await ribbon(page);
    if (!isUp(before)) broken('the ribbon was down before the press, on a note not holding the place');
    await page.locator('.y-headermark [data-mark-button]').click();
    await page.waitForFunction(() => document.querySelector('[data-mark-said]')?.textContent !== '', null, { timeout: 5000 });
    await page.waitForTimeout(100);
    await settled(page);
    const after = await ribbon(page);
    if (!isDown(after)) fail('ribbon-follows-the-control', `pressing the header's bookmark kept the place here and left the ribbon ${(after.cloth.top - after.box.top).toFixed(1)}px off its resting place`);

    // The page as it was drawn while the place was here, shown after the place
    // moved elsewhere.
    const drawnKept = await (await fetch(`${BASE}${PAGE}`)).text();
    if (!drawnKept.includes('data-mark-kept')) broken('the page drawn while the place was here does not say so');
    await keepIn(ELSEWHERE);
    const stale = await context.newPage();
    await stale.route(`${BASE}${PAGE}`, (route) => route.fulfill({ status: 200, contentType: 'text/html; charset=utf-8', body: drawnKept }));
    await stale.goto(`${BASE}${PAGE}`, { waitUntil: 'load' });
    await arrived(stale);
    await stale.waitForTimeout(400);
    await settled(stale);
    const restored = await ribbon(stale);
    if (!isUp(restored)) fail('ribbon-follows-the-control', `a page drawn while the place was here, shown after it moved to ${ELSEWHERE}, still shows ${(restored.cloth.bottom - restored.box.top).toFixed(1)}px of ribbon`);
    await context.close();
  }

  // --- Its tempo ---
  //
  // Read frame by frame from the press: with motion, frames part way down
  // stand between the two resting places; for a reader who asked for less,
  // none does.
  for (const motion of ['no-preference', 'reduce']) {
    await keepIn(ELSEWHERE);
    const context = await open({ width: 1512, rail: 'open', motion });
    const page = await visit(context, PAGE);
    await page.waitForFunction(() => document.documentElement.classList.contains('is-ready'));
    await page.evaluate(() => {
      const frames = [];
      window.__fall = frames;
      const box = document.querySelector('.y-ribbon');
      const cloth = document.querySelector('.y-ribbon__cloth');
      const start = performance.now();
      const sample = () => {
        frames.push(cloth.getBoundingClientRect().top - box.getBoundingClientRect().top);
        if (performance.now() - start < 900) requestAnimationFrame(sample);
      };
      new MutationObserver((_, observer) => {
        observer.disconnect();
        requestAnimationFrame(sample);
      }).observe(document.querySelector('[data-mark-control]'), { attributes: true, attributeFilter: ['data-mark-kept'] });
    });
    await page.locator('.y-headermark [data-mark-button]').click();
    await page.waitForFunction(() => window.__fall.length > 0 && Math.abs(window.__fall.at(-1)) <= 0.5, null, { timeout: 5000 })
      .catch(() => broken(`${motion}: the ribbon did not come to rest after the press`));
    const frames = await page.evaluate(() => window.__fall);
    // Strictly between drawn up (its whole length and the shadow's room above
    // the edge) and lying down.
    const between = frames.filter((offset) => offset < -SLACK && offset > frames[0] + SLACK);
    if (motion === 'no-preference' && between.length < 3) {
      fail('ribbon-falls-at-its-tempo', `the ribbon was drawn part way down in ${between.length} frames after the press, want it to fall over its tempo`);
    }
    if (motion === 'reduce' && between.length > 0) {
      fail('ribbon-falls-at-its-tempo', `for a reader who asked for less motion the ribbon was drawn part way down in ${between.length} frames, want it at rest in the first`);
    }
    await context.close();
  }

  // --- A picture, which moves nothing and covers nothing ---
  await keepIn(NOTE);
  for (const view of RIBBON_WIDTHS) {
    const at = `at ${view.width}px with the column ${view.rail}`;
    const measure = (page) => page.evaluate(() => {
      const top = (element) => (element ? element.getBoundingClientRect().top + window.scrollY : null);
      const left = (element) => (element ? element.getBoundingClientRect().left : null);
      const title = document.querySelector('.y-frontis .y-title');
      const first = document.querySelector('.y-prose > p');
      const heading = document.querySelector('.y-prose [data-level]');
      return JSON.stringify([top(title), left(title), top(first), left(first), top(heading), document.documentElement.scrollHeight]);
    });

    const keptContext = await open(view);
    const page = await visit(keptContext, PAGE);
    await settled(page);
    const r = await ribbon(page);
    if (!isDown(r)) broken(`${at} the ribbon is not down on the note holding the place`);
    const kept = await measure(page);

    // What the page shows with the ribbon down is read first; every judgement
    // waits until the page has also been read without it.
    const picture = await page.evaluate(() => {
      const box = document.querySelector('.y-ribbon');
      const r = box.getBoundingClientRect();
      const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      return {
        hidden: box.closest('[aria-hidden="true"]') !== null,
        hit: hit ? (hit.closest('.y-ribbon') ? 'the ribbon' : null) : null,
        cursor: getComputedStyle(document.querySelector('.y-ribbon__cloth')).cursor,
        focusable: box.querySelectorAll('a, button, [tabindex]').length,
      };
    });

    const cover = await page.evaluate(({ cloth }) => {
      const meets = (a) => a.width > 0 && a.height > 0 && a.left < cloth.right - 0.5 && a.right > cloth.left + 0.5 && a.top < cloth.bottom - 0.5 && a.bottom > cloth.top + 0.5;
      const name = (element) => `${element.tagName.toLowerCase()}${element.className && typeof element.className === 'string' ? `.${element.className.trim().split(/\s+/).join('.')}` : ''}`;
      const hits = [];
      for (const element of document.querySelectorAll('.y-header :is(a, button, input, form, search, [role="group"])')) {
        if (meets(element.getBoundingClientRect())) hits.push(`the header's ${name(element)}`);
      }
      for (const element of document.querySelectorAll('.y-frontis > *:not(.y-ribbon), .y-article > *:not(.y-frontis, template), .y-prose > *, .y-prose > .y-sect > *')) {
        if (meets(element.getBoundingClientRect())) hits.push(`the note's ${name(element)}`);
      }
      const rail = document.querySelector('.y-rail-left');
      const railBox = rail.getBoundingClientRect();
      const railStyle = getComputedStyle(rail);
      const railShown = railStyle.visibility !== 'hidden' && Number(railStyle.opacity) > 0 && railBox.right > 0 && railBox.width > 0;
      return { hits, railRight: railShown ? railBox.right : 0, windowRight: document.documentElement.clientWidth };
    }, { cloth: r.cloth });

    // Moves nothing: the same page with the ribbon taken out, and the page of
    // a note not holding the place, are laid out the same.
    await page.evaluate(() => document.querySelector('.y-ribbon').remove());
    const without = await measure(page);
    await keptContext.close();

    await keepIn(ELSEWHERE);
    const otherContext = await open(view);
    const other = await visit(otherContext, PAGE);
    await settled(other);
    const unkept = await measure(other);
    await otherContext.close();
    await keepIn(NOTE);

    if (without !== kept) fail('ribbon-moves-nothing', `${at} taking the ribbon out moved the page: ${kept} with it, ${without} without`);
    if (unkept !== kept) fail('ribbon-moves-nothing', `${at} the note lies differently with and without its bookmark: ${kept} kept, ${unkept} not`);

    // A picture: hidden from assistive technology, out of the way of the
    // pointer, and out of the order a keyboard walks.
    if (!picture.hidden) fail('ribbon-is-a-picture', `${at} the ribbon is exposed to assistive technology; the header's control is what says the place is kept`);
    if (picture.hit || picture.cursor === 'pointer') fail('ribbon-is-a-picture', `${at} a pointer over the ribbon lands on it (cursor ${picture.cursor}); it is a picture, and the header's control is the one thing a press acts on`);
    if (picture.focusable > 0) fail('ribbon-is-a-picture', `${at} the ribbon holds ${picture.focusable} focusable element(s)`);

    // Hung from the header's lower edge.
    if (Math.abs(r.cloth.top - r.header.bottom) > SLACK) {
      fail('ribbon-hangs-from-the-header', `${at} the ribbon starts ${(r.cloth.top - r.header.bottom).toFixed(1)}px from the header's lower edge, want it hanging from that edge`);
    }

    // Over no control in the header, no row of the note, and inside the window
    // clear of the open column.
    if (cover.hits.length > 0) fail('ribbon-covers-nothing', `${at} the ribbon lies over ${cover.hits.join(', ')}`);
    if (r.cloth.left < cover.railRight - SLACK) fail('ribbon-covers-nothing', `${at} the ribbon starts at ${r.cloth.left.toFixed(1)}, under the open column that ends at ${cover.railRight.toFixed(1)}`);
    if (r.cloth.left < 0 || r.cloth.right > cover.windowRight + SLACK) fail('ribbon-covers-nothing', `${at} the ribbon runs from ${r.cloth.left.toFixed(1)} to ${r.cloth.right.toFixed(1)}, outside the window's 0 to ${cover.windowRight}`);
  }

  // --- The section numbers ---
  const numbers = (page) => page.evaluate(() => {
    const prose = document.querySelector('.y-article > .y-prose');
    const proseBox = prose.getBoundingClientRect();
    const headings = [...prose.querySelectorAll(':scope > [data-level="2"], :scope > .y-sect > [data-level="2"]')];
    const textStart = (heading) => {
      const walker = document.createTreeWalker(heading, NodeFilter.SHOW_TEXT, { acceptNode: (node) => (node.data.trim() ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP) });
      const node = walker.nextNode();
      if (!node) return null;
      const range = document.createRange();
      range.selectNodeContents(node);
      return range.getClientRects()[0]?.left ?? null;
    };
    const rail = document.querySelector('.y-rail-left');
    const railBox = rail?.getBoundingClientRect();
    const railStyle = rail ? getComputedStyle(rail) : null;
    const railShown = rail && railStyle.visibility !== 'hidden' && Number(railStyle.opacity) > 0 && railBox.right > 0 && railBox.width > 0;
    return {
      flagged: prose.hasAttribute('data-section-numbers'),
      reset: getComputedStyle(prose).counterReset,
      proseLeft: proseBox.left,
      railRight: railShown ? railBox.right : 0,
      overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
      headings: headings.map((heading) => {
        const number = getComputedStyle(heading, '::before');
        return {
          id: heading.id,
          text: heading.textContent.trim(),
          drawn: number.content.includes('counter(y-section'),
          counted: getComputedStyle(heading).counterIncrement,
          start: textStart(heading),
          hang: parseFloat(number.minInlineSize) || 0,
          pull: parseFloat(number.marginInlineStart) || 0,
          left: heading.getBoundingClientRect().left,
          spills: heading.scrollWidth - heading.clientWidth,
        };
      }),
    };
  });

  for (const view of NUMBER_CASES) {
    const at = `${decodeURIComponent(view.path)} at ${view.width}px, text ${view.size}, column ${view.rail}`;
    const context = await open(view);
    const page = await visit(context, view.path);
    const n = await numbers(page);
    if (n.headings.length < 2) broken(`${at} has ${n.headings.length} top-level ## headings, want at least 2 to count`);
    if (!n.flagged || !n.reset.includes('y-section')) fail('sections-are-counted', `${at} the note's sections are not counted (flagged ${n.flagged}, counter-reset ${n.reset})`);
    for (const h of n.headings) {
      if (!h.drawn || h.counted !== 'y-section 1') fail('sections-are-counted', `${at} the heading "${h.text}" draws no number of its own (counter-increment ${h.counted})`);
    }

    for (const h of n.headings) {
      const hung = h.hang > 0 && Math.abs(h.pull + h.hang) <= SLACK;
      // Whichever way it stands, the number stays in the window and clear of
      // the open column, and the heading and the page keep their width.
      const numberLeft = hung ? h.start - h.hang : h.left;
      if (numberLeft < 0) fail('numbers-stay-clear', `${at} the number of "${h.text}" starts at ${numberLeft.toFixed(1)}, off the window`);
      if (numberLeft < n.railRight) fail('numbers-stay-clear', `${at} the number of "${h.text}" starts at ${numberLeft.toFixed(1)}, under the open column that ends at ${n.railRight.toFixed(1)}`);
      if (h.spills > 0) fail('numbers-stay-clear', `${at} the heading "${h.text}" runs ${h.spills}px past its own width`);
    }
    if (n.overflow > 0) fail('numbers-stay-clear', `${at} the page scrolls sideways by ${n.overflow}px`);

    for (const h of n.headings) {
      const hung = h.hang > 0 && Math.abs(h.pull + h.hang) <= SLACK;
      const textOnEdge = Math.abs(h.start - n.proseLeft) <= SLACK;
      if (view.hang && !(hung && textOnEdge && Math.abs(h.hang - HANG) <= SLACK)) {
        fail('numbers-hang-where-there-is-room', `${at} the number of "${h.text}" stands inline (hang ${h.hang}, text starts ${(h.start - n.proseLeft).toFixed(1)}px in), want it hung ${HANG}px into the margin with the heading's first word on the text's edge`);
      }
      if (!view.hang && (hung || h.start - n.proseLeft < 10)) {
        fail('numbers-hang-where-there-is-room', `${at} the number of "${h.text}" hangs in a margin with no room for it (hang ${h.hang}), want it inline before the heading`);
      }
    }

    // The name a screen reader gives each heading is its words alone.
    const cdp = await context.newCDPSession(page);
    const { root } = await cdp.send('DOM.getDocument', { depth: 0 });
    for (const h of n.headings) {
      const { nodeId } = await cdp.send('DOM.querySelector', { nodeId: root.nodeId, selector: `#${CSS_ESCAPE(h.id)}` });
      if (!nodeId) broken(`${at} cannot find the heading #${h.id}`);
      const { nodes } = await cdp.send('Accessibility.getPartialAXTree', { nodeId, fetchRelatives: false });
      const accessible = String(nodes[0]?.name?.value ?? '').trim();
      // Neither drawn into the name from the stylesheet nor written into the
      // heading as text: none of these headings opens on a number of its own.
      if (accessible !== h.text || /^\d/.test(accessible)) fail('number-outside-the-name', `${at} the heading "${h.text}" is named "${accessible}" to a screen reader, want its words alone`);
    }
    await context.close();
  }

  for (const path of NUMBERED) {
    const context = await open({ width: 1512, rail: 'open' });
    const page = await visit(context, path);
    const n = await numbers(page);
    if (n.headings.length < 2) broken(`${path} has ${n.headings.length} top-level ## headings, want at least 2`);
    const drawn = n.headings.filter((h) => h.drawn);
    if (n.flagged || drawn.length > 0) fail('counted-once', `${decodeURIComponent(path)} numbers its own sections and the page counts ${drawn.length} of them again`);
    await context.close();
  }

  // --- The reading line ---
  for (const motion of ['no-preference', 'reduce']) {
    const site = motion === 'reduce' ? 'progress-under-reduced-motion' : 'progress-tracks-the-reading';
    for (const view of [{ width: 1512, rail: 'open' }, { width: 390 }]) {
      const at = `at ${view.width}px${motion === 'reduce' ? ' for a reader who asked for less motion' : ''}`;
      const context = await open({ ...view, motion });
      const page = await visit(context, PAGE);
      const line = async (where) => {
        await page.evaluate((to) => {
          const furthest = document.documentElement.scrollHeight - window.innerHeight;
          window.scrollTo(0, to === 'top' ? 0 : to === 'end' ? furthest : Math.round(furthest / 2));
        }, where);
        await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        return page.evaluate(() => {
          const element = document.querySelector('.y-readline');
          const r = element.getBoundingClientRect();
          const header = document.querySelector('.y-header').getBoundingClientRect();
          return { width: r.width, bottom: r.bottom, height: r.height, header: { width: header.width, height: header.height, bottom: header.bottom } };
        });
      };
      const top = await line('top');
      const middle = await line('middle');
      const end = await line('end');
      if (top.width > 1) fail(site, `${at} the line is ${top.width.toFixed(1)}px long at the top of the note, want nothing`);
      if (middle.width < top.header.width * 0.3 || middle.width > top.header.width * 0.7) fail(site, `${at} the line is ${middle.width.toFixed(1)}px of ${top.header.width}px half way through the note`);
      if (Math.abs(end.width - end.header.width) > 1) fail(site, `${at} the line is ${end.width.toFixed(1)}px at the end of the note, want the header's ${end.header.width}px`);

      if (motion === 'no-preference') {
        // Along the header's lower edge, without changing the header.
        const expected = view.width <= 720 ? 48 : 44;
        if (Math.abs(end.bottom - end.header.bottom) > 1 || Math.abs(end.height - 1.5) > 0.1) {
          fail('progress-on-the-header-edge', `${at} the line is ${end.height}px tall and ends ${(end.bottom - end.header.bottom).toFixed(1)}px from the header's lower edge, want 1.5px along that edge`);
        }
        if (end.header.height !== expected) fail('progress-on-the-header-edge', `${at} the header is ${end.header.height}px tall, want ${expected}px`);
        await page.evaluate(() => document.querySelector('.y-readline').remove());
        const bare = await page.evaluate(() => document.querySelector('.y-header').getBoundingClientRect().height);
        if (bare !== end.header.height) fail('progress-on-the-header-edge', `${at} the header is ${end.header.height}px with the line and ${bare}px without it`);
      }
      await context.close();
    }
  }
  {
    const context = await open({ width: 1512, rail: 'open' });
    for (const path of ['/', '/search?q=alpha', '/marks']) {
      const page = await visit(context, path);
      if (await page.locator('.y-readline').count() > 0) fail('progress-only-on-a-note', `${path} draws the reading line, which only a note's page has a start and end for`);
      await page.close();
    }
    await context.close();
  }

  if (planted) {
    const issue = planted.proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log(
    'PASS reading-furniture: the ribbon lies still on arrival, shows the bookmark with and without a script,'
    + ' follows the control, falls at its tempo and not for reduced motion, takes no pointer, moves nothing,'
    + ' hangs from the header and covers nothing at every width; sections are counted once, outside their'
    + ' names, hung where there is room and inline where not, clear of the window and the column; the reading'
    + ' line tracks the note along the header edge, on a note only, and under reduced motion too',
  );
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const issue = planted.proof();
      const { target } = MUTATIONS[MUTATE];
      if (issue) {
        console.error(`NOT-APPLIED reading-furniture: ${MUTATE}: ${issue}`);
        console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      } else if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
    }
    process.exitCode = 1;
  } else if (err instanceof ProbeBroken) {
    console.error(err.message);
    process.exitCode = 1;
  } else {
    console.error(err);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

// CSS_ESCAPE escapes an id for a selector the way CSS.escape does in a page.
function CSS_ESCAPE(id) {
  return id.replace(/([^\w-])/g, '\\$1').replace(/^(\d)/, '\\3$1 ');
}
