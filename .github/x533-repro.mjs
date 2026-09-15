// Throwaway experiment for issue #533. Not a lock, not wired to probes.sh.
//
// It asks one question: with the cross-document navigation transition restored
// exactly as it stood before 444d2a09, does a page reached by following a link
// still arrive complete and never paint?
//
// Every round is a fresh page. The arriving document records its own arrival
// from the inside, because pagereveal has already fired or already been missed
// by the time anything outside can ask. Three readings decide a round:
//
//   declared   the arriving document's own stylesheets carry the opt-in
//   transition the arrival actually came through a transition
//   frames     a frame callback ran on the arriving document within a second
//   pressed    a control on the arriving page accepted a real click
//
// A round where transition is false did not exercise the mechanism and proves
// nothing either way; those rounds are counted separately rather than read as
// good news.
//
// Env: YOMIHON_BASE, PAGE_PATH, ROUNDS, LABEL.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const ROUNDS = Number(process.env.ROUNDS || 5);
const LABEL = process.env.LABEL || 'unnamed';

// Read the arriving document's stylesheets for the opt-in, so a round can say
// the rules reached the browser rather than assuming the build carried them.
const declaredNavigationTransitions = () => {
  const found = [];
  let readable = 0;
  const rulesOf = (owner) => {
    try {
      return owner.cssRules;
    } catch {
      return null;
    }
  };
  const walk = (rules) => {
    for (const rule of rules) {
      if (rule.cssText.startsWith('@view-transition') && /navigation:\s*auto\b/.test(rule.cssText)) {
        found.push(rule.cssText);
      }
      const nested = rulesOf(rule.styleSheet ?? rule);
      if (nested) walk(nested);
    }
  };
  for (const sheet of [...document.styleSheets, ...(document.adoptedStyleSheets ?? [])]) {
    const rules = rulesOf(sheet);
    if (!rules) continue;
    readable += 1;
    walk(rules);
  }
  return { readable, found: found.length };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const rows = [];
try {
  for (let round = 1; round <= ROUNDS; round += 1) {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 800 },
      reducedMotion: 'no-preference',
    });
    await page.addInitScript(() => {
      window.__arrival = { reveal: false, transition: false, finished: 'pending' };
      window.addEventListener('pagereveal', (event) => {
        window.__arrival.reveal = true;
        window.__arrival.transition = Boolean(event.viewTransition);
        event.viewTransition?.finished.then(
          () => { window.__arrival.finished = true; },
          () => { window.__arrival.finished = 'rejected'; },
        );
      }, { once: true });
    });

    const row = { round, label: LABEL };
    try {
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      row.first = response?.status() ?? null;
      await page.waitForSelector('html[data-js]');

      // The walk #474 described: from a note to the reading choices through the
      // one link the chrome offers on every page.
      await Promise.all([
        page.waitForURL('**/preferences**', { timeout: 15_000 }),
        page.locator('.y-prefslink').click(),
      ]);
      row.arrived = true;
    } catch (held) {
      row.arrived = false;
      row.held = String(held.message).split('\n')[0];
      rows.push(row);
      await page.close();
      continue;
    }

    // Frames are the reading that separates "the element moved" from "the page
    // stopped": a stability check waits on two consecutive frames and nothing
    // else on this page needs one. setTimeout still runs on a document whose
    // rendering is suspended, so the count can be collected either way.
    const reading = await page.evaluate(() => new Promise((resolve) => {
      let frames = 0;
      const tick = () => { frames += 1; requestAnimationFrame(tick); };
      requestAnimationFrame(tick);
      setTimeout(() => resolve({
        frames,
        reveal: window.__arrival?.reveal ?? null,
        transition: window.__arrival?.transition ?? null,
        finished: window.__arrival?.finished ?? null,
        ready: document.readyState,
        visibility: document.visibilityState,
        href: location.pathname,
      }), 1000);
    }));
    Object.assign(row, reading);

    await page.waitForLoadState('load');
    const declared = await page.evaluate(declaredNavigationTransitions);
    row.declared = declared.found;
    row.sheets = declared.readable;

    // #474's observed symptom was a refused press, not only a dead frame clock,
    // so the round drives the control a reader would press.
    const form = page.locator('form[action="/preferences"]').filter({
      has: page.locator('input[type=radio][name="lang"]'),
    });
    const other = form.locator('label:has(input[type=radio][name="lang"]:not(:checked))');
    try {
      await other.click({ timeout: 8_000 });
      row.pressed = true;
    } catch (refused) {
      row.pressed = false;
      row.refused = String(refused.message).split('\n')[0];
    }

    rows.push(row);
    await page.close();
  }
} finally {
  await browser.close();
}

for (const row of rows) console.log(`X533 ${JSON.stringify(row)}`);

const exercised = rows.filter((row) => row.transition === true);
const frozen = rows.filter((row) => row.arrived === false || row.frames === 0 || row.pressed === false);
console.log(`X533-SUMMARY ${JSON.stringify({
  label: LABEL,
  rounds: rows.length,
  arrived: rows.filter((row) => row.arrived).length,
  declaredEverywhere: rows.every((row) => row.declared > 0),
  exercised: exercised.length,
  frozen: frozen.length,
})}`);
if (exercised.length === 0) {
  console.log('X533-VERDICT inconclusive: no round arrived through a transition, so the mechanism was never exercised');
} else if (frozen.length > 0) {
  console.log(`X533-VERDICT reproduced: ${frozen.length} of ${rows.length} rounds arrived at a page that did not paint or would not take a press`);
} else {
  console.log(`X533-VERDICT not-reproduced: ${exercised.length} of ${rows.length} rounds arrived through a transition, painted, and took a press`);
}
