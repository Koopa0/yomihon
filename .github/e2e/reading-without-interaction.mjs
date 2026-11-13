// Behavior lock for the first of the three situations the reading page is built
// for: a reader meeting an unfamiliar mechanism for the first time reads it from
// the first line to the last without using any of the page's own controls, and
// nothing in the way of that is asked of them. Each section stands beside quiet
// controls and a closed question stands under a line that says a guess is
// welcome; none of it may block, cover, trap, demand a reply or write anything.
//
// What a real browser shows, at the width a laptop opens at and at a phone's:
// scrolling the whole note top to bottom with nothing pressed, the text under
// the middle of the window is always the note's own (no dialog, no overlay, no
// inert page, no sentence a control earned); the controls leave the text where
// it is; a reader who walks the page with Tab reaches the end of it, and every
// stop on the way is a thing they can see; and nothing was written anywhere.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with several sections and a closed
// question, in a vault whose contract names a place to write a thought), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Reading%20flow.md';
const MUTATE = process.env.MUTATE || '';
const WIDTHS = [1280, 375];

const SITES = [
  'nothing-asks-for-an-answer',
  'controls-leave-the-text-alone',
  'nothing-is-written',
  'nothing-blocks-the-way-on',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN reading-without-interaction: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL reading-without-interaction: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN reading-without-interaction: ${message}`); };

// A planted regression rewrites what the page is served, never a file on disk.
const addRule = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'app.css was never requested, so the rule reached no page');
};
const addScript = (source) => async (page) => {
  let seen = 0;
  await page.route('**/static/yomihon.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${source}\n` });
  });
  return () => (seen > 0 ? '' : 'yomihon.js was never requested, so the script reached no page');
};

const MUTATIONS = {
  // The line above a question stops being a line and becomes a sheet over the page.
  'cover-the-page-with-the-prompt': {
    target: 'nothing-asks-for-an-answer',
    apply: addRule('.y-predict{position:fixed !important;inset:0 !important;z-index:99;background:var(--bg);display:grid;place-items:center}'),
  },
  // A modal opens when the reader reaches the second section: the rest of the
  // page goes inert until it is answered.
  'open-a-modal-at-the-second-section': {
    target: 'nothing-asks-for-an-answer',
    apply: addScript(`
      addEventListener('scroll', () => {
        if (document.querySelector('dialog[data-nag]')) return;
        const heading = document.querySelectorAll('.y-prose .y-sect')[1];
        if (!heading || heading.getBoundingClientRect().top > innerHeight) return;
        const nag = document.createElement('dialog');
        nag.dataset.nag = '';
        nag.textContent = 'How sure are you?';
        document.body.append(nag);
        nag.showModal();
      }, { passive: true });
    `),
  },
  // The margin is laid across the column instead of beside it.
  'lay-the-margin-over-the-text': {
    target: 'controls-leave-the-text-alone',
    apply: addRule('.y-article .y-margin{position:absolute !important;inset-inline-start:0 !important;inset-block-start:0 !important;width:100% !important}.y-article .y-sect{position:relative}'),
  },
  // Arriving at the end of the note keeps a mark nobody asked for.
  'keep-a-mark-on-reaching-the-end': {
    target: 'nothing-is-written',
    apply: addScript(`
      let sent = false;
      addEventListener('scroll', () => {
        if (sent || innerHeight + scrollY < document.documentElement.scrollHeight - 40) return;
        sent = true;
        const article = document.querySelector('[data-uncertainty-path]');
        fetch('/uncertainties', { method: 'POST', body: new URLSearchParams({ path: article.dataset.uncertaintyPath, anchor: 'the-first-mechanism' }) });
      }, { passive: true });
    `),
  },
  // Focus meets the margin's controls and stays on them.
  'trap-focus-in-the-margin': {
    target: 'nothing-blocks-the-way-on',
    apply: addScript(`
      document.addEventListener('keydown', (event) => {
        const held = document.activeElement?.closest?.('[data-section-margin]');
        if (event.key === 'Tab' && held && !event.shiftKey) {
          event.preventDefault();
          document.activeElement.focus();
        }
      }, true);
    `),
  },
  // The controls are still in the tab order, and out of sight.
  'move-the-controls-out-of-the-window': {
    target: 'nothing-blocks-the-way-on',
    apply: addRule('.y-margin__act{position:fixed !important;inset-inline-start:-600px !important}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`reading-without-interaction: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`reading-without-interaction: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reading-without-interaction: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What is in the way of the text under the middle of the window, read from the
// page: a dialog or popover that is open, a page made inert, a box fixed over
// the column, a sentence a control earned. The header is fixed by design.
const inTheWay = () => {
  const reasons = [];
  if (document.querySelector('dialog[open], [popover]:popover-open, :modal')) reasons.push('a dialog or popover is open');
  if (document.querySelector('main[inert], .y-article[inert], body[inert]')) reasons.push('the page is inert');
  for (const x of [innerWidth * 0.3, innerWidth * 0.5, innerWidth * 0.7]) {
    for (const y of [innerHeight * 0.4, innerHeight * 0.6]) {
      for (let element = document.elementFromPoint(x, y); element && element !== document.body; element = element.parentElement) {
        if (getComputedStyle(element).position === 'fixed' && !element.closest('.y-header, .y-rail-left')) {
          reasons.push(`a fixed ${element.tagName.toLowerCase()}.${element.className} covers the window at ${Math.round(x)},${Math.round(y)}`);
          break;
        }
      }
    }
  }
  for (const reply of document.querySelectorAll('.y-article [role="status"]')) {
    if (reply.textContent.trim()) reasons.push(`a control said "${reply.textContent.trim()}" unasked`);
  }
  return [...new Set(reasons)];
};

// Whether any control of the margin or any prompt overlaps the note's own text,
// or the page grew sideways.
const crowding = () => {
  const texts = [...document.querySelectorAll('.y-prose > p, .y-prose > ul, .y-prose > ol, .y-prose pre, .y-prose .y-sect > :first-child, .y-title')]
    .filter((element) => !element.closest('.y-margin'));
  const found = [];
  for (const control of document.querySelectorAll('.y-margin__act, .y-margin__mine, .y-predict a')) {
    const box = control.getBoundingClientRect();
    if (box.width === 0 || box.height === 0) continue;
    for (const text of texts) {
      const other = text.getBoundingClientRect();
      const overlap = Math.min(box.right, other.right) - Math.max(box.left, other.left) > 2 && Math.min(box.bottom, other.bottom) - Math.max(box.top, other.top) > 2;
      if (overlap && !text.contains(control) && !control.contains(text)) found.push(`${control.className || control.tagName} over ${text.tagName.toLowerCase()}`);
    }
  }
  if (document.documentElement.scrollWidth > innerWidth + 1) found.push(`the page is ${document.documentElement.scrollWidth}px wide in a ${innerWidth}px window`);
  return [...new Set(found)];
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const writes = [];
  let proof = () => '';
  for (const width of WIDTHS) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage();
    page.on('request', (request) => {
      if (!['GET', 'HEAD'].includes(request.method())) writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
    });
    if (MUTATE) proof = await MUTATIONS[MUTATE].apply(page);
    const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (response?.status() !== 200) broken(`${PAGE} answered ${response?.status()}, want 200`);
    await page.evaluate(() => document.fonts.ready);
    const applied = proof();
    if (applied) throw new NotApplied(`NOT-APPLIED reading-without-interaction: ${MUTATE}: ${applied}`);

    // What the page has to offer must be there, or what follows proves nothing.
    const offer = await page.evaluate(() => ({
      sections: document.querySelectorAll('.y-prose .y-sect [data-section-margin]').length,
      marks: document.querySelectorAll('[data-section-mark]').length,
      doors: document.querySelectorAll('.y-margin a.y-margin__act').length,
      prompts: document.querySelectorAll('.y-predict + details[data-callout="question"]:not([open])').length,
    }));
    if (offer.sections < 3 || offer.marks < 3 || offer.doors < 3 || offer.prompts < 1) {
      broken(`${PAGE} at ${width}px offers ${JSON.stringify(offer)}, want three sections with a mark and a door each and one closed question under its line`);
    }

    // Read it through: top to bottom, in windows' worth, nothing pressed.
    let top = 0;
    for (let step = 0; step < 60; step += 1) {
      await page.evaluate((y) => window.scrollTo(0, y), top);
      await page.waitForTimeout(40);
      const reasons = await page.evaluate(inTheWay);
      if (reasons.length > 0) fail('nothing-asks-for-an-answer', `at ${width}px, ${top}px down: ${reasons.join('; ')}`);
      const reach = await page.evaluate(() => innerHeight + scrollY >= document.documentElement.scrollHeight - 2);
      if (reach) break;
      top += 600;
    }
    const closed = await page.evaluate(() => document.querySelector('details[data-callout="question"]').open);
    if (closed) fail('nothing-asks-for-an-answer', `at ${width}px reading through opened the question, which only the reader opens`);

    await page.evaluate(() => window.scrollTo(0, 0));
    const crowded = await page.evaluate(crowding);
    if (crowded.length > 0) fail('controls-leave-the-text-alone', `at ${width}px: ${crowded.join('; ')}`);

    if (writes.length > 0) fail('nothing-is-written', `reading through at ${width}px sent ${writes.join(', ')}`);
    const stored = await (await fetch(`${BASE}/uncertainties`)).json();
    if (stored.length !== 0) fail('nothing-is-written', `reading through at ${width}px left ${JSON.stringify(stored)} stored`);

    // Walk it with the keyboard: every stop is a thing the reader can see, and the
    // walk reaches the note's end.
    await page.evaluate(() => document.querySelector('#_y-main').focus());
    let previous = null;
    let reachedTheEnd = false;
    let stops = 0;
    for (let press = 0; press < 200 && !reachedTheEnd; press += 1) {
      await page.keyboard.press('Tab');
      const stop = await page.evaluate(() => {
        const element = document.activeElement;
        if (!element || element === document.body) return null;
        const box = element.getBoundingClientRect();
        const style = getComputedStyle(element);
        return {
          id: `${element.tagName}.${element.className}|${element.getAttribute('href') ?? element.getAttribute('data-section-anchor') ?? element.textContent.slice(0, 20)}`,
          inArticle: Boolean(element.closest('.y-article')),
          seen: box.width > 0 && box.height > 0 && box.right > 0 && box.left < innerWidth && style.visibility !== 'hidden' && Number(style.opacity) > 0.05,
          end: Boolean(element.closest('.y-related, .y-steps')),
        };
      });
      if (stop === null) break;
      if (stop.id === previous) fail('nothing-blocks-the-way-on', `at ${width}px, Tab ${press + 1} stayed on ${stop.id}: the keyboard cannot go on`);
      previous = stop.id;
      if (stop.inArticle) {
        stops += 1;
        if (!stop.seen) fail('nothing-blocks-the-way-on', `at ${width}px, Tab ${press + 1} stopped on ${stop.id}, which cannot be seen`);
      }
      reachedTheEnd = stop.end;
    }
    if (!reachedTheEnd) fail('nothing-blocks-the-way-on', `at ${width}px, ${stops} Tab stops in the note never reached its end`);
    await context.close();
  }
  console.log(`PASS reading-without-interaction: at ${WIDTHS.join(' and ')}px the note reads top to bottom with nothing pressed and nothing in the way, the controls leave the text alone, the keyboard reaches the end over stops it can see, and nothing is written`);
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    if (MUTATE && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    else if (MUTATE) console.error(`no catch: ${MUTATE} targets ${MUTATIONS[MUTATE].target}, but ${error.site} fired first`);
    process.exitCode = 1;
  } else {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
  // A planted regression may keep a mark; none of the later probes starts with one.
  const owned = decodeURIComponent(PAGE.slice('/notes/'.length));
  try {
    for (const { path, anchor } of await (await fetch(`${BASE}/uncertainties`)).json()) {
      if (path === owned) await fetch(`${BASE}/uncertainties`, { method: 'POST', body: new URLSearchParams({ path, anchor }) });
    }
  } catch (error) {
    console.error(`BROKEN reading-without-interaction: cleanup failed: ${error}`);
    process.exitCode = 2;
  }
}
