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

  const anyFillHolding = () => page.$$eval('.y-sealfill', (fills) =>
    fills.some((f) => f.style.width === '100%'));

  if (!(await page.$('[data-seal]'))) fail('page has no seal form — pick a sealable lesson page');
  if (!(await page.$('select.y-slotselect'))) fail('page has no slot select — pick a slot lesson page');

  // Positive control: R held outside any typing surface starts the fill.
  await page.evaluate(() => document.body.focus());
  await page.keyboard.down('r');
  const controlStarted = await anyFillHolding();
  await page.keyboard.up('r');
  if (!controlStarted) {
    fail('positive control broken: held R on the body did not start the seal fill, so this probe cannot see the seal path at all');
  }
  // Release must retract the fill before the real case runs.
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.y-sealfill')].every((f) => f.style.width !== '100%'));
  // A completed hold would submit the form and latch the script's sealing
  // state, making the case below pass without testing the guard — so a POST
  // during the control window is a loud stop, not a quiet abort.
  if (postAttempted) {
    fail('positive control ran long enough to submit the seal form; the guard case below would be vacuous');
  }

  // The lock: R held from a focused select must not start the fill.
  await page.focus('select.y-slotselect');
  await page.keyboard.down('r');
  const leaked = await anyFillHolding();
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
