// Behavior lock: a held R inside a focused <select> never starts the seal
// hold — inside a select, letter keys are typeahead, not shortcuts. The
// observable is the seal fill: holdStart stretches every .y-sealfill to
// width 100%, so the guard holds exactly when that never happens.
//
// The probe carries its own can-fail proof: it first holds R with focus on
// the page body and requires the fill to start (positive control). If that
// control fails — no seal form on the page, the fill renamed, the shortcut
// rebound — the probe errors out instead of passing vacuously.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a lesson
// page carrying both a slot-machine <select> and the seal form).
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
// MUTATE=unguard-select self-tests the probe: the served script has the
// select clause stripped from its typing guard, and the probe must go red.
const MUTATE = process.env.MUTATE || '';

const fail = (msg) => { throw new Error(`FAIL seal-select-guard: ${msg}`); };

// The fill starts synchronously inside the keydown handler today, so reading it
// on the next line happens to work. This bound exists so the lock does not
// depend on that: move the fill behind a timer or a frame and the probe must
// still see it. Both sides share it — the positive control waits up to
// FILL_TIMEOUT_MS for the fill to appear before calling itself broken, and the
// guard case watches for the same span a fill that must never appear. One shared
// tolerance is what makes the negative sound: a leaked fill that began later than
// the guard watched but sooner than the control tolerates would otherwise slip
// through green. The span stays under the hold's own completion, so a key held
// through it can never run the seal to a submitted form; the aborted POST is the
// last line of defence against that, never the first.
//
// Every wait carries its own bound, so a fill that never moves stops the probe
// with a sentence naming what it was waiting for, rather than stalling until
// the driver's default expires and reporting nothing anyone can act on.
const FILL_TIMEOUT_MS = 300;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // The seal form writes a real status transition. This probe must never
  // commit one — every POST is aborted at the network layer, and a POST
  // attempted while a select is focused is itself a failure signal.
  let postAttempted = false;
  await page.route('**/*', (route) => {
    if (route.request().method() === 'POST') { postAttempted = true; return route.abort(); }
    return route.continue();
  });
  // Tracks that the unguard mutation really rewrote the guard: a needle
  // that matches nothing serves the script unchanged, and the self-test
  // would go green while proving nothing.
  let unguarded = false;
  if (MUTATE === 'unguard-select') {
    await page.route('**/yomihon.js', async (route) => {
      const res = await route.fetch();
      const original = await res.text();
      const body = original.replace("t.tagName === 'SELECT'", 'false');
      if (body !== original) unguarded = true;
      return route.fulfill({ response: res, body });
    });
  }
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');
  if (MUTATE === 'unguard-select' && !unguarded) {
    fail("unguard-select mutation did not apply: the typing guard's select clause was not found");
  }

  if (!(await page.$('[data-seal]'))) fail('page has no seal form — pick a sealable lesson page');
  if (!(await page.$('select.y-slotselect'))) fail('page has no slot select — pick a slot lesson page');

  // Positive control: R held outside any typing surface starts the fill.
  await page.evaluate(() => document.body.focus());
  await page.keyboard.down('r');
  let controlStarted = true;
  try {
    await page.waitForFunction(
      () => [...document.querySelectorAll('.y-sealfill')].some((f) => f.style.width === '100%'),
      null,
      { timeout: FILL_TIMEOUT_MS },
    );
  } catch {
    controlStarted = false;
  }
  await page.keyboard.up('r');
  if (!controlStarted) {
    fail('positive control broken: held R on the body did not start the seal fill, so this probe cannot see the seal path at all');
  }
  // Release must retract the fill before the real case runs: a fill left at full
  // width would read as a leak the guard case never caused.
  try {
    await page.waitForFunction(
      () => [...document.querySelectorAll('.y-sealfill')].every((f) => f.style.width !== '100%'),
      null,
      { timeout: FILL_TIMEOUT_MS },
    );
  } catch {
    fail('the seal fill never retracted after R was released, so the guard case below would start from a fill already at full width');
  }
  // A completed hold would submit the form and latch the script's sealing
  // state, making the case below pass without testing the guard — so a POST
  // during the control window is a loud stop, not a quiet abort.
  if (postAttempted) {
    fail('positive control ran long enough to submit the seal form; the guard case below would be vacuous');
  }

  // The lock: R held from a focused select must not start the fill. Watching for
  // the fill up to the bound the control tolerates — rather than reading once
  // after a fixed pause — is what makes this a sound negative: a leaked fill is
  // caught the instant it appears, and only a window that stays empty for as long
  // as a real fill was ever allowed to take is read as no leak.
  await page.focus('select.y-slotselect');
  await page.keyboard.down('r');
  let leaked = false;
  try {
    await page.waitForFunction(
      () => [...document.querySelectorAll('.y-sealfill')].some((f) => f.style.width === '100%'),
      null,
      { timeout: FILL_TIMEOUT_MS },
    );
    leaked = true;
  } catch {
    leaked = false;
  }
  await page.keyboard.up('r');
  if (leaked) fail('held R inside a focused select started the seal fill');
  if (postAttempted) fail('a POST fired while the select was focused — the seal path ran to completion');

  console.log('PASS seal-select-guard: control fill started from body; no fill from a focused select');
} catch (err) {
  console.error(err instanceof Error && err.message.startsWith('FAIL') ? err.message : err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
