// Behavior lock for the first frame of a return to a kept place.
//
// A reader who chooses "Continue reading" on the desk follows an address made
// of two parts: the heading they stopped under, as the fragment, and how far
// below it they were, as a query only script can spend. The browser acts on
// the fragment by itself once the document has been parsed, and the module acts
// on the query; whichever of the two moves the window last wins. The page holds
// its column out of sight until the module has placed it, so the lock is on
// what the reader's eye is given: in every frame the column is visible, the
// window is at the place that was kept, never at the heading the browser alone
// would have taken them to.
//
// The measurement is a frame recorder that starts before the document's own
// scripts and samples at every animation frame. It follows the desk's own link
// rather than loading the address, because an address carrying only the query
// has no fragment for the browser to act on, and the defect lives in the
// fragment.
//
// It keeps a place of its own on the disposable fixture, so it is one of the
// probes that leave one behind and has to stand among the last (see probes.sh).
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const NOTE = '/notes/Notes/Reading%20flow.md';
const OTHER_NOTE = 'Notes/alpha.md';

// How far down the note the place is kept, per width. Far enough that the
// anchor above the window is a heading the browser alone would land on a long
// way from it, so a frame at the heading cannot be mistaken for one at the place.
const KEEP_AT = [
  { width: 1280, height: 800, y: 1100 },
  { width: 375, height: 800, y: 1700 },
];
// Each return is made twice over: as the page is served, and with the module
// that places the page arriving after the browser has painted its first frame.
// Whether the old order showed the heading depended on which of the two came
// first, so the second is the one that does not leave it to chance.
const TRIALS = 3;
const LATE_MODULE_MS = 80;
const CONDITIONS = [
  { name: 'as served', delay: 0 },
  { name: 'with the module late', delay: LATE_MODULE_MS },
];
const SLACK = 2;
const APART = 150;

const PLACE_SITE = 'every-visible-frame-is-at-the-kept-place';
const SITES = [PLACE_SITE];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN kept-place-first-frame: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL kept-place-first-frame: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN kept-place-first-frame: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED kept-place-first-frame: ${message}`); };

// rewriteModule plants a regression in the client module that decides the
// landing. The needle has to match exactly once in every copy served: a
// mutation aimed at a line that has been rewritten proves nothing, and saying
// so is the difference between a probe that died and one that passed.
const rewriteModule = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/mark.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the module was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

// rewriteStylesheet plants a regression in the stylesheet, which is where the
// hold on the column is declared. It counts per load for the same reason.
const rewriteStylesheet = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the stylesheet was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

const MUTATIONS = {
  // The column is shown as soon as the module has placed the page, which is
  // before the browser moves the window to the fragment: the order the defect
  // had.
  'reveal-before-the-browser-places': {
    target: PLACE_SITE,
    apply: rewriteModule(
      'afterBrowserPlacement(() => {',
      '((then) => then())(() => {',
      'the wait for the browser to move to the fragment',
    ),
  },
  // The wait is kept and the placing is not repeated after it, so the column is
  // shown wherever the browser left the window.
  'reveal-without-placing-again': {
    target: PLACE_SITE,
    apply: rewriteModule(
      '    apply();\n    reveal();',
      '    reveal();',
      'the placing that follows the wait',
    ),
  },
  // The module places and reveals from inside the event that precedes the
  // browser's move, rather than from a task queued after it.
  'place-inside-the-parsing-event': {
    target: PLACE_SITE,
    apply: rewriteModule(
      "addEventListener('DOMContentLoaded', () => setTimeout(once), { once: true });",
      "addEventListener('DOMContentLoaded', once, { once: true });",
      'the task queued after the parsing event',
    ),
  },
  // Nothing holds the column out of sight, so it is drawn wherever the window
  // is before the module has placed it.
  'drop-the-hold': {
    target: PLACE_SITE,
    apply: rewriteStylesheet(
      "html[data-js][data-arrival='kept'] .y-main {",
      "html[data-js][data-arrival='kept-probe'] .y-main {",
      'the rule that holds the column of a kept place out of sight',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`kept-place-first-frame: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`kept-place-first-frame: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`kept-place-first-frame: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Every document records each frame it paints from inside, starting before any
// of its own scripts: where the window is, whether the column can be seen, and
// whether the hold that keeps it out of sight is still on.
const recorder = () => {
  if (window.top !== window) return;
  window.__frames = [];
  const started = performance.now();
  const sample = () => {
    const column = document.querySelector('.y-main');
    window.__frames.push({
      at: Math.round(performance.now() - started),
      column: Boolean(column),
      visible: column ? getComputedStyle(column).visibility === 'visible' : false,
      held: document.documentElement.dataset.arrival ?? null,
      y: Math.round(window.scrollY),
      target: document.querySelector(':target')?.id ?? null,
    });
    if (performance.now() - started < 1200) requestAnimationFrame(sample);
  };
  requestAnimationFrame(sample);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  console.log(`kept-place-first-frame: Chrome ${browser.version()}`);
  let measured = 0;
  let returns = 0;
  for (const view of KEEP_AT) {
    const context = await browser.newContext({ viewport: { width: view.width, height: view.height }, reducedMotion: 'no-preference' });
    await context.addInitScript(recorder);
    // The place being returned to is first kept somewhere else, so that the
    // desk offers the note being measured only once the reader has kept it.
    const set = await context.request.post(`${BASE}/marks`, {
      form: { path: OTHER_NOTE, anchor: '', offset: '0', identity: '0'.repeat(64) },
    });
    if (set.status() !== 204) broken(`keeping a place in ${OTHER_NOTE} answered ${set.status()}, want 204`);

    const page = await context.newPage();

    await page.goto(BASE + NOTE, { waitUntil: 'domcontentloaded' });
    await arrived(page);
    await page.evaluate(() => document.fonts.ready);
    await page.evaluate((y) => window.scrollTo(0, y), view.y);
    const control = page.locator('[data-mark-control]:visible');
    if (await control.count() !== 1) broken(`${NOTE} shows ${await control.count()} mark controls at ${view.width}px, want exactly 1`);
    const posted = page.waitForResponse((response) => new URL(response.url()).pathname === '/marks' && response.request().method() === 'POST', { timeout: 4000 });
    await control.locator('[data-mark-button]').click();
    const response = await posted;
    if (response.status() !== 204) broken(`keeping the place at ${view.width}px answered ${response.status()}, want 204`);
    const form = new URLSearchParams(response.request().postData() ?? '');
    const anchor = form.get('anchor');
    const offset = Number(form.get('offset'));
    if (!anchor) broken(`the place kept at ${view.width}px has no anchor, so there is no fragment for the browser to act on`);
    await control.locator('[data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
    // Where the place is, from what was kept: the anchor and the distance below
    // it, which is what the return applies. The window may have been moved a
    // little by reaching the control, so it is not read from the window.
    const place = await page.evaluate(({ id, below }) => {
      const element = document.getElementById(id);
      return element ? Math.round(element.getBoundingClientRect().top + window.scrollY) + below : null;
    }, { id: anchor, below: offset });
    if (place === null) broken(`the anchor ${JSON.stringify(anchor)} that was kept at ${view.width}px is not in the note`);

    // Where the browser alone would take the reader: the same heading, with
    // no distance below it. The lock can tell a frame at the place from a frame
    // at the heading only if the two are far apart.
    const bare = await context.newPage();
    await bare.goto(`${BASE}${NOTE}#${encodeURIComponent(anchor)}`, { waitUntil: 'load' });
    await arrived(bare);
    await bare.evaluate(() => document.fonts.ready);
    const heading = await bare.evaluate(() => Math.round(window.scrollY));
    await bare.close();
    if (Math.abs(heading - place) < APART) {
      broken(`at ${view.width}px the heading ${JSON.stringify(anchor)} is at ${heading} and the place at ${place}, ${APART}px apart is needed to tell them apart`);
    }

    for (const condition of CONDITIONS) {
      for (let trial = 1; trial <= TRIALS; trial += 1) {
        const reader = await context.newPage();
        const proof = MUTATE ? await MUTATIONS[MUTATE].apply(reader) : null;
        if (condition.delay > 0) {
          await reader.route('**/static/yomihon.js{,?*}', async (route) => {
            await new Promise((resolve) => { setTimeout(resolve, condition.delay); });
            await route.continue();
          });
        }
        await reader.goto(`${BASE}/`, { waitUntil: 'load' });
        await arrived(reader);
        const link = reader.locator('[data-home-continue] [data-continue-link]');
        if (await link.count() !== 1) broken(`the desk offers ${await link.count()} ways back to the kept place at ${view.width}px, want 1`);
        const href = await link.getAttribute('href');
        if (!href?.includes('at=') || !href.includes('#')) broken(`the desk's way back is ${JSON.stringify(href)}, want an address with a distance and a fragment`);
        await Promise.all([reader.waitForURL((url) => url.pathname === decodeURI(NOTE) || url.pathname === NOTE), link.click()]);
        await reader.waitForFunction(() => (window.__frames ?? []).at(-1)?.at >= 1150, null, { timeout: 8000 });
        const frames = (await reader.evaluate(() => window.__frames ?? [])).filter((frame) => frame.column);
        if (frames.length < 10) broken(`the document painted ${frames.length} frames with its column at ${view.width}px, too few to say anything about them`);
        const held = frames.filter((frame) => frame.held !== null).length;
        if (condition.delay > 0 && held < 2) {
          broken(`with the module ${condition.delay}ms late the column was held in only ${held} frames at ${view.width}px, so the first frame was not painted before the module ran`);
        }
        const settled = frames.at(-1);
        if (Math.abs(settled.y - place) > SLACK) {
          broken(`the return settled at ${settled.y} at ${view.width}px, not at the ${place} the place was kept at, so there is no place to be painted at`);
        }
        const seen = frames.filter((frame) => frame.visible);
        if (seen.length === 0) broken(`the column was never visible in ${frames.length} frames at ${view.width}px`);
        const wrong = seen.filter((frame) => Math.abs(frame.y - place) > SLACK);
        measured += seen.length;
        returns += 1;
        if (proof) {
          const issue = proof();
          if (issue) notApplied(`${MUTATE}: ${issue}`);
        }
        console.log(`kept-place-first-frame: ${view.width}px ${condition.name} trial ${trial}: ${held} frames held, first visible frame ${JSON.stringify(seen[0])}, ${wrong.length} of ${seen.length} visible frames away from ${place}`);
        if (wrong.length > 0) {
          fail(PLACE_SITE, `at ${view.width}px (${condition.name}) a frame showing the column was painted with the window at ${wrong[0].y} (heading at ${heading}), but the place kept is at ${place}: the reader saw the note somewhere else first ${JSON.stringify(wrong[0])}`);
        }
        await reader.close();
      }
    }
    await context.close();
  }
  console.log(`PASS kept-place-first-frame: ${measured} visible frames over ${returns} returns, every one at the kept place`);
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
