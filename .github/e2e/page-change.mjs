// A page change plays no animation, the left sidebar keeps its scroll position,
// and a kept place is painted where it is, once.
//
// Every document records each frame it paints, from inside, starting before the
// document's own scripts: the column's opacity and scale, how many animations
// run inside it, whether it is visible, and where the window is scrolled. What
// is asserted is read off those frames, not off a stylesheet or a timer, so it
// is what the reader's eye was given: a first frame at half opacity and a
// scale of 98.5% was the defect, and a first frame at the top of a note the
// reader is returning into the middle of would be the one this replaces it
// with.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const COURSE_PAGE = '/notes/Course/C02.md';
const LONG_RAIL_PAGE = '/notes/Writing/lessons/long-rail/Rail%20lesson%2030.md';
const KEPT_PAGE = '/notes/Notes/reading-fidelity.md';
const KEPT_OFFSET = 900;

// Each regression: the response it is planted in, and the text it removes or
// adds there. A needle that does not match exactly once per response is not a
// mutation of anything, and the run says so rather than passing.
const MODES = {
  'restore-arrival-animation': {
    glob: '**/static/app.css{,?*}',
    plant: (body) => `${body}\n.y-article{animation:y-probe-come-forward 320ms backwards}@keyframes y-probe-come-forward{from{opacity:.5;scale:.985}}\n`,
    expect: 'animates',
  },
  'drop-kept-hold': {
    glob: '**/static/app.css{,?*}',
    needle: "html[data-js][data-arrival='kept'] .y-main {",
    replacement: "html[data-js][data-arrival='kept-probe'] .y-main {",
    expect: 'first-visible-frame',
  },
  'drop-kept-fallback': {
    glob: '**/static/app.css{,?*}',
    needle: 'animation: y-kept-reveal 0s 400ms forwards;',
    replacement: '',
    expect: 'fallback',
  },
  'keep-the-mark': {
    glob: '**/static/mark.js{,?*}',
    needle: '    delete root.dataset.arrival;\n',
    replacement: '',
    expect: 'module-reveals',
  },
  'drop-rail-restore': {
    glob: `**/notes/**`,
    needle: 'if (same) rail.scrollTop = kept.top;',
    replacement: '',
    expect: 'rail-position',
  },
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`page-change: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL page-change: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const fail = (site, message) => { throw new LockFired(site, message); };
const broken = (message) => { throw new Error(`BROKEN page-change: ${message}`); };

const browser = await chromium.launch({ channel: 'chrome', headless: true });

const recorder = () => {
  window.__frames = [];
  const started = performance.now();
  const sample = () => {
    const main = document.querySelector('.y-main');
    const article = main?.firstElementChild;
    window.__frames.push({
      at: Math.round(performance.now() - started),
      column: Boolean(main),
      visibility: main ? getComputedStyle(main).visibility : null,
      opacity: article ? getComputedStyle(article).opacity : null,
      scale: article ? getComputedStyle(article).scale : null,
      animations: main ? document.getAnimations().filter((a) => a.effect?.target && main.contains(a.effect.target)
        && !(a.effect.target.closest('[data-ripple]'))).length : 0,
      y: Math.round(window.scrollY),
      held: document.documentElement.dataset.arrival ?? null,
    });
    if (performance.now() - started < 900) requestAnimationFrame(sample);
  };
  requestAnimationFrame(sample);
};

const open = async (options = {}) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, reducedMotion: 'no-preference', ...options.context });
  await context.addInitScript(recorder);
  const page = await context.newPage();
  const planted = [];
  const mode = MUTATE ? MODES[MUTATE] : null;
  if (mode && !options.noMutation) {
    const seen = { requests: 0, matches: 0 };
    planted.push(seen);
    await page.route(mode.glob, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      seen.requests += 1;
      if (mode.plant) {
        seen.matches += 1;
        await route.fulfill({ response, body: mode.plant(original) });
        return;
      }
      seen.matches += original.split(mode.needle).length - 1;
      await route.fulfill({ response, body: original.replace(mode.needle, mode.replacement) });
    });
  }
  for (const pattern of options.abort ?? []) await page.route(pattern, (route) => route.abort());
  return {
    page,
    context,
    proof() {
      for (const { requests, matches } of planted) {
        if (requests < 1 || matches !== requests) {
          throw new NotApplied(`NOT-APPLIED page-change: the planted change matched ${matches} times across ${requests} responses`);
        }
      }
    },
  };
};

const frames = (page) => page.evaluate(() => window.__frames ?? []);

const expects = (site) => !MUTATE || MODES[MUTATE].expect === site;

try {
  console.log(`page-change: Chrome ${browser.version()}`);

  // 1. Following a link to another lesson plays nothing: in every frame the
  // new document paints, its column is opaque, unscaled and unanimated.
  {
    const { page, context, proof } = await open();
    await page.goto(BASE + COURSE_PAGE, { waitUntil: 'load' });
    const link = page.locator('.y-rail-left a.ui-navitem', { hasText: 'C03' }).first();
    if (await link.count() === 0) broken(`${COURSE_PAGE} offers no rail row for C03 to follow`);
    await Promise.all([page.waitForURL(/C03/), link.click()]);
    await page.waitForFunction(() => (window.__frames ?? []).length >= 8, null, { timeout: 5000 });
    const seen = (await frames(page)).filter((frame) => frame.column);
    if (seen.length < 4) broken(`the new document painted ${seen.length} frames with its column, which is too few to say anything about them`);
    const moving = seen.filter((f) => f.opacity !== '1' || f.scale !== 'none' || f.animations !== 0 || f.visibility !== 'visible');
    console.log(`page-change: following a link painted ${seen.length} frames, ${moving.length} of them not whole`);
    if (moving.length > 0) {
      fail('animates', `after following a link the new page's column was painted ${JSON.stringify(moving[0])}; a page change plays nothing, so every frame is opaque, unscaled, visible and unanimated`);
    }
    if (MUTATE) proof();
    await context.close();
  }

  // 2. A kept place is painted once: the first frame the reader can see the
  // column in is already at the place, and the hold that made that so is
  // lifted by the module that scrolled there, not by the timer behind it.
  {
    const { page, context, proof } = await open();
    await page.goto(BASE + `${KEPT_PAGE}?at=${KEPT_OFFSET}`, { waitUntil: 'load' });
    await page.waitForFunction(() => (window.__frames ?? []).length >= 20, null, { timeout: 5000 });
    const seen = (await frames(page)).filter((frame) => frame.column);
    const settled = seen.at(-1);
    if (settled.y < KEPT_OFFSET - 2 || settled.y > KEPT_OFFSET + 2) {
      broken(`${KEPT_PAGE} was not at ${KEPT_OFFSET} once settled (${settled.y}), so there is no place to be painted at`);
    }
    const first = seen.find((frame) => frame.visibility === 'visible');
    console.log(`page-change: kept place, first visible frame ${JSON.stringify(first)}, settled at ${settled.y}`);
    if (!first || Math.abs(first.y - settled.y) > 1) {
      fail('first-visible-frame', `the first frame the column was visible in is at ${first?.y ?? 'no frame'}, but the page settled at ${settled.y}: the reader saw the page somewhere else first`);
    }
    if (MUTATE) proof();
    const held = await page.evaluate(() => document.documentElement.dataset.arrival ?? null);
    if (held !== null) fail('module-reveals', `the document is still marked as a return to a kept place (${held}) once it is settled, so only the timer revealed it`);
    await context.close();
  }

  // 3. A page whose module never runs is not left hidden: the stylesheet
  // reveals the column on its own after a short wait, and until then it is held.
  {
    const { page, context, proof } = await open({ abort: ['**/static/yomihon.js{,?*}'] });
    await page.goto(BASE + `${KEPT_PAGE}?at=${KEPT_OFFSET}`, { waitUntil: 'commit' });
    await page.waitForFunction(() => document.querySelector('.y-main') !== null, null, { timeout: 5000 });
    await page.waitForTimeout(120);
    const early = await page.evaluate(() => getComputedStyle(document.querySelector('.y-main')).visibility);
    if (early !== 'hidden') broken(`the column of a kept place was ${early} 120ms in with no script to place it; the hold this lock waits out is not there`);
    await page.waitForTimeout(900);
    const late = await page.evaluate(() => getComputedStyle(document.querySelector('.y-main')).visibility);
    console.log(`page-change: with the module blocked the column is ${early} early and ${late} late`);
    if (late !== 'visible') fail('fallback', `with the module blocked the column was still ${late} a second in, so a script that fails leaves the page hidden`);
    if (MUTATE) proof();
    await context.close();
  }

  // 4. The left sidebar keeps its scroll position from one page of a course to
  // the next, and a row the next page makes current is brought into view.
  {
    const { page, context, proof } = await open();
    await page.goto(BASE + LONG_RAIL_PAGE, { waitUntil: 'load' });
    const state = await page.evaluate(() => {
      const rail = document.querySelector('.y-rail-left');
      const rows = [...rail.querySelectorAll('.ui-navitem[href^="/notes/"]')].filter((row) => row.getClientRects().length > 0);
      return { max: rail.scrollHeight - rail.clientHeight, rows: rows.length };
    });
    if (state.max < 300) broken(`the long course's rail is only ${state.max}px past its fold, which is too short to hold a chosen place`);
    // Park the rail somewhere the reader chose, and press a row that is in view
    // from there: nothing then has reason to move it.
    const chosen = await page.evaluate(() => {
      const rail = document.querySelector('.y-rail-left');
      rail.scrollTop = 200;
      const box = rail.getBoundingClientRect();
      const row = [...rail.querySelectorAll('.ui-navitem[href^="/notes/"]')].find((r) => {
        const rect = r.getBoundingClientRect();
        return rect.top > box.top + 120 && rect.bottom < box.bottom - 60 && r.getAttribute('aria-current') !== 'page';
      });
      return { top: rail.scrollTop, href: row?.getAttribute('href') ?? null };
    });
    if (!chosen.href || chosen.top < 150) broken(`the rail could not be parked with a row in view: ${JSON.stringify(chosen)}`);
    await Promise.all([page.waitForURL((url) => url.pathname === decodeURI(chosen.href) || url.pathname === chosen.href), page.locator(`.y-rail-left a[href="${chosen.href}"]`).click()]);
    await page.waitForFunction(() => (window.__frames ?? []).length >= 2, null, { timeout: 5000 });
    const arrival = await page.evaluate(() => {
      const rail = document.querySelector('.y-rail-left');
      const row = rail.querySelector('.ui-navitem[aria-current="page"]');
      const box = rail.getBoundingClientRect();
      const rect = row?.getBoundingClientRect();
      return { top: rail.scrollTop, inView: Boolean(rect && rect.top >= box.top - 1 && rect.bottom <= box.bottom + 1) };
    });
    console.log(`page-change: the rail was parked at ${chosen.top} and the next page's rail is at ${arrival.top}, current row in view: ${arrival.inView}`);
    if (Math.abs(arrival.top - chosen.top) > 1) {
      fail('rail-position', `the rail was at ${chosen.top} when a row was pressed and the next page's rail starts at ${arrival.top}: the sidebar jumped on a page change`);
    }
    if (!arrival.inView) broken('the next page\'s current row is outside the rail it was parked in front of, so the position cannot be told from a placement');
    if (MUTATE) proof();
    await context.close();
  }

  console.log('PASS page-change: a page change paints whole frames, a kept place is painted once and revealed even without its script, and the sidebar keeps its place');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.site === MODES[MUTATE].expect) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
