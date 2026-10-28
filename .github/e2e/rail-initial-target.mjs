// Arrival must find the current lesson without moving the reading surface.
// Without scripting the stylesheet's initial target does it; with scripting the
// rail module does it once. The control loads the same document with both
// removed, so fragment positioning is compared with the browser's own landing.
import { chromium } from 'playwright-core';
import { arrived } from './support/arrival.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/long-rail/Rail%20lesson%2030.md';
const MUTATE = process.env.MUTATE || '';
const RAIL = '.y-rail-left';
const CURRENT = `${RAIL} .ui-navitem[aria-current="page"]`;

// Each way the row is placed, and the line that removing it takes away.
const PLACERS = {
  stylesheet: { glob: '**/app.css{,?*}', needle: '    scroll-initial-target: nearest;\n' },
  script: { glob: '**/rail.js{,?*}', needle: '  if (rail) revealCurrent(rail);\n' },
};
const MODES = {
  'remove-initial-target': ['stylesheet'],
  'skip-arrival-reveal': ['script'],
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`rail-initial-target: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {}
class NotApplied extends Error {}
const fail = (message) => { throw new LockFired(`FAIL rail-initial-target: ${message}`); };
const broken = (message) => { throw new Error(`BROKEN rail-initial-target: ${message}`); };

const browser = await chromium.launch({ channel: 'chrome', headless: true });
// A script is only fetched where scripting runs, so a placer is removed, and
// its removal proved, only where the page would load it.
const open = async (path, options, remove) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 800 }, ...options });
  const page = await context.newPage();
  const routed = [];
  for (const name of remove) {
    if (name === 'script' && !options.javaScriptEnabled) continue;
    const { glob, needle } = PLACERS[name];
    const seen = { name, requests: 0, matches: 0 };
    routed.push(seen);
    await page.route(glob, async (route) => {
      seen.requests += 1;
      const response = await route.fetch();
      const original = await response.text();
      seen.matches += original.split(needle).length - 1;
      await route.fulfill({ response, body: original.replace(needle, '') });
    });
  }
  await page.goto(BASE + path, { waitUntil: 'load' });
  await arrived(page);
  return {
    page,
    context,
    proof() {
      for (const { name, requests, matches } of routed) {
        if (requests < 1 || matches !== requests) {
          throw new NotApplied(`NOT-APPLIED rail-initial-target: the ${name} placer matched ${matches} times across ${requests} requests`);
        }
      }
    },
  };
};

const geometry = (page) => page.evaluate(({ railSelector, currentSelector }) => {
  const rail = document.querySelector(railSelector);
  const rows = [...document.querySelectorAll(currentSelector)].filter((row) => row.getClientRects().length > 0);
  if (!rail || rows.length !== 1) return { rows: rows.length };
  const row = rows[0].getBoundingClientRect();
  const box = rail.getBoundingClientRect();
  const head = rail.querySelector('.y-railhead');
  const headBottom = head?.getClientRects().length ? head.getBoundingClientRect().bottom : box.top;
  const heading = document.querySelector('#heading');
  return {
    rows: rows.length,
    current: { top: row.top, bottom: row.bottom },
    visible: { top: Math.max(box.top + rail.clientTop, headBottom), bottom: box.top + rail.clientTop + rail.clientHeight },
    railTop: rail.scrollTop,
    railMax: rail.scrollHeight - rail.clientHeight,
    documentTop: document.scrollingElement.scrollTop,
    headingTop: heading?.getBoundingClientRect().top ?? null,
  };
}, { railSelector: RAIL, currentSelector: CURRENT });

const settled = async (page) => {
  await page.waitForFunction(() => !document.documentElement.hasAttribute('data-rail-moving')
    && !document.getAnimations().some((animation) => animation instanceof CSSTransition));
};

try {
  console.log(`rail-initial-target: Chrome ${browser.version()}`);
  for (const options of [
    { javaScriptEnabled: true, reducedMotion: 'no-preference' },
    { javaScriptEnabled: true, reducedMotion: 'reduce' },
    { javaScriptEnabled: false, reducedMotion: 'no-preference' },
  ]) {
    for (const fragment of ['', '#heading']) {
      const path = PAGE + fragment;
      const control = await open(path, options, Object.keys(PLACERS));
      control.proof();
      const baseline = await geometry(control.page);
      await control.context.close();
      const subject = await open(path, options, MUTATE ? MODES[MUTATE] : []);
      if (MUTATE) subject.proof();
      const actual = await geometry(subject.page);
      if (baseline.rows !== 1 || actual.rows !== 1) broken('the fixture must draw exactly one current row');
      if (baseline.railMax < 300 || baseline.current.top <= baseline.visible.bottom) {
        broken(`the control does not put the current row below a long rail's fold: ${JSON.stringify(baseline)}`);
      }
      if (fragment && (baseline.documentTop < 500 || baseline.headingTop === null)) {
        broken(`the fragment control did not land at a heading below the first screen: ${JSON.stringify(baseline)}`);
      }
      console.log(`rail-initial-target: ${JSON.stringify({ path, ...options, baseline, actual })}`);
      if (actual.current.top < actual.visible.top - 1 || actual.current.bottom > actual.visible.bottom + 1) {
        fail('the current row is outside the visible rail');
      }
      if (Math.abs(actual.documentTop - baseline.documentTop) > 1
        || (fragment && Math.abs(actual.headingTop - baseline.headingTop) > 1)) {
        fail('targeting the rail changed the document landing');
      }
      if (options.javaScriptEnabled) {
        // A reader can leave the current row behind; reopening must preserve
        // that chosen place instead of applying the arrival target again.
        const chosen = await subject.page.locator(RAIL).evaluate((rail) => {
          rail.scrollTop = (rail.scrollHeight - rail.clientHeight) / 3;
          return rail.scrollTop;
        });
        if (chosen < 100 || Math.abs(chosen - actual.railTop) < 100) broken('the chosen position is not distinct from arrival');
        // Driver locator clicks scroll a sticky button's natural position
        // into view. Press its visible position as a reader would instead.
        const press = async () => {
          const box = await subject.page.locator('[data-rail-toggle]').boundingBox();
          if (!box) broken('the fold button is not visible');
          await subject.page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
        };
        await press();
        await settled(subject.page);
        if (await subject.page.locator('html').getAttribute('data-rail') !== 'collapsed') broken('the control did not collapse the rail');
        await press();
        await settled(subject.page);
        const reopened = await geometry(subject.page);
        if (Math.abs(reopened.railTop - chosen) > 1) fail(`reopening moved the rail from ${chosen} to ${reopened.railTop}`);
        if (Math.abs(reopened.documentTop - actual.documentTop) > 1) fail('folding the rail moved the document');
      }
      await subject.context.close();
    }
  }
  console.log('PASS rail-initial-target: arrival reveals the current row, keeps plain and fragment landings, and preserves the chosen rail position across folding');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.message.endsWith('the current row is outside the visible rail')) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
