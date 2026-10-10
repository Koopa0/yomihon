// Arrival must find the current lesson without moving the reading surface.
// The control loads the same document with only the target declaration removed,
// so fragment positioning is compared with the browser's own landing.
import { chromium } from 'playwright-core';
import { arrived } from './support/arrival.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/long-rail/Rail%20lesson%2030.md';
const MUTATE = process.env.MUTATE || '';
const MODE = 'remove-initial-target';
const NEEDLE = '  scroll-initial-target: nearest;\n';
const RAIL = '.y-rail-left';
const CURRENT = `${RAIL} .ui-navitem[aria-current="page"]`;

if (MUTATE === 'list') {
  console.log(MODE);
  process.exit(0);
}
if (MUTATE && MUTATE !== MODE) {
  console.error(`rail-initial-target: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {}
class NotApplied extends Error {}
const fail = (message) => { throw new LockFired(`FAIL rail-initial-target: ${message}`); };
const broken = (message) => { throw new Error(`BROKEN rail-initial-target: ${message}`); };

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const open = async (path, options, remove) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 800 }, ...options });
  const page = await context.newPage();
  let requests = 0;
  let matches = 0;
  if (remove) {
    await page.route('**/app.css{,?*}', async (route) => {
      requests += 1;
      const response = await route.fetch();
      const original = await response.text();
      matches += original.split(NEEDLE).length - 1;
      await route.fulfill({ response, body: original.replace(NEEDLE, '') });
    });
  }
  await page.goto(BASE + path, { waitUntil: 'load' });
  await arrived(page);
  return {
    page,
    context,
    proof() {
      if (requests < 1 || matches !== requests) {
        throw new NotApplied(`NOT-APPLIED rail-initial-target: declaration matched ${matches} times across ${requests} stylesheet requests`);
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
      const control = await open(path, options, true);
      const baseline = await geometry(control.page);
      await control.context.close();
      const subject = await open(path, options, Boolean(MUTATE));
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
