// Behavior lock: a held R inside a focused <select> never starts the seal
// hold — inside a select, letter keys are typeahead, not shortcuts. The
// observable is the seal fill: holdStart stretches every .y-sealfill to
// width 100%, so the guard holds exactly when that never happens.
//
// Two cases, because a select has two faces. With the picker closed the key
// event targets the select itself. With the branded picker open it targets the
// focused <option>, and the guard reaches the select only by walking up from
// there — a walk an earlier guard, which tested the target's own tag name, did
// not make.
//
// The probe carries its own can-fail proof: each case first holds R with focus
// on the page body and requires the fill to start (positive control). If that
// control fails — no seal form on the page, the fill renamed, the shortcut
// rebound — the probe says so instead of passing vacuously.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a lesson
// page carrying both a slot-machine <select> and the seal form). MUTATE names
// one of the self-test modes below; MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
const SELECT = 'select.y-slotselect';

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle matched
// nothing. Only the first is ever reported as a caught mutation — otherwise a
// crash that happens to exit 1 would read as a detection.
class LockFired extends Error {}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (msg) => { throw new LockFired(`FAIL seal-select-guard: ${msg}`); };
const broken = (msg) => { throw new ProbeBroken(`BROKEN seal-select-guard: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED seal-select-guard: ${msg}`); };

// The clause of the served script that keeps the single-key shortcuts out of a
// focused select's typeahead. Replacing it with false removes the guard and
// nothing else, which is the regression both cases must catch. Tracking that it
// really rewrote something is the point of `mutated`: a needle matching nothing
// serves the script unchanged, and the self-test would go green proving nothing.
const GUARD_NEEDLE = "t.closest('select')";
let mutated = false;

// Every mutation this probe can inject lives in this table; the dispatch below
// is a lookup into it, and MUTATE=list prints its keys. A mode that exists but
// is not listed cannot happen.
const MUTATIONS = {
  'unguard-select': async (page) => {
    await page.route('**/yomihon.js', async (route) => {
      const res = await route.fetch();
      const original = await res.text();
      const body = original.replace(GUARD_NEEDLE, 'false');
      if (body !== original) mutated = true;
      return route.fulfill({ response: res, body });
    });
  },
};

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`seal-select-guard: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

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

const someFillFull = () => [...document.querySelectorAll('.y-sealfill')].some((f) => f.style.width === '100%');
const noFillFull = () => [...document.querySelectorAll('.y-sealfill')].every((f) => f.style.width !== '100%');

// Holds R for as long as the control is allowed to take, and reports whether any
// seal fill reached full width inside that window.
const holdRAndWatchFill = async (page) => {
  await page.keyboard.down('r');
  let started = false;
  try {
    await page.waitForFunction(someFillFull, null, { timeout: FILL_TIMEOUT_MS });
    started = true;
  } catch {
    started = false;
  }
  await page.keyboard.up('r');
  return started;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });

// Runs one case on a page of its own. A leaked hold latches the script's sealing
// state, so a second case sharing that page would see no fill and read the
// silence as a guard that worked. Separate pages keep each case's answer its own.
async function guardCase({ name, arrange }) {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // The seal form writes a real status transition. This probe never commits one:
  // every POST is aborted at the network layer, and a POST attempted while the
  // select holds focus is itself a failure signal.
  let postAttempted = false;
  await page.route('**/*', (route) => {
    if (route.request().method() === 'POST') { postAttempted = true; return route.abort(); }
    return route.continue();
  });
  if (MUTATE) await MUTATIONS[MUTATE](page);
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');
  if (MUTATE && !mutated) {
    notApplied(`the ${MUTATE} needle matched nothing in the served script: ${GUARD_NEEDLE}`);
  }

  if (!(await page.$('[data-seal]'))) broken('page has no seal form — pick a sealable lesson page');
  if (!(await page.$(SELECT))) broken('page has no slot select — pick a slot lesson page');

  // Positive control: R held outside any typing surface starts the fill.
  await page.evaluate(() => document.body.focus());
  if (!(await holdRAndWatchFill(page))) {
    broken(`${name}: held R on the body did not start the seal fill, so this probe cannot see the seal path at all`);
  }
  // Release must retract the fill before the real case runs: a fill left at full
  // width would read as a leak the case never caused.
  try {
    await page.waitForFunction(noFillFull, null, { timeout: FILL_TIMEOUT_MS });
  } catch {
    broken(`${name}: the seal fill never retracted after R was released, so the case below would start from a fill already at full width`);
  }
  // A completed hold would submit the form and latch the script's sealing state,
  // making the case below pass without testing the guard — so a POST during the
  // control window is a loud stop, not a quiet abort.
  if (postAttempted) {
    broken(`${name}: the positive control ran long enough to submit the seal form; the case below would be vacuous`);
  }

  await arrange(page);

  // The lock: R held from inside the select must not start the fill. Watching for
  // the fill up to the bound the control tolerates — rather than reading once
  // after a fixed pause — is what makes this a sound negative: a leaked fill is
  // caught the instant it appears, and only a window that stays empty for as long
  // as a real fill was ever allowed to take is read as no leak.
  const leaked = await holdRAndWatchFill(page);
  const submitted = postAttempted;
  await page.close();
  return { leaked, submitted };
}

const CASES = [
  {
    name: 'a focused select',
    arrange: async (page) => {
      await page.focus(SELECT);
      const tag = await page.evaluate(() => document.activeElement.tagName);
      if (tag !== 'SELECT') broken(`focusing the slot select left focus on ${tag}`);
    },
  },
  {
    name: 'the select picker open on a focused option',
    arrange: async (page) => {
      await page.focus(SELECT);
      await page.keyboard.press('Space'); // opens the branded picker
      // Focus landing anywhere but an option inside the select means the picker
      // never opened, and holding R here would only retest the closed face.
      try {
        await page.waitForFunction(
          () => {
            const a = document.activeElement;
            return !!a && a.tagName === 'OPTION' && !!a.closest('select');
          },
          null,
          { timeout: 1000 },
        );
      } catch {
        const tag = await page.evaluate(() => document.activeElement.tagName);
        broken(`the picker did not open, or focus stayed on ${tag} rather than an option, so this case never reaches the open picker`);
      }
    },
  },
];

try {
  // Both cases run before any verdict, so a mutation that unguards the select is
  // seen to break both faces rather than only the first one tried.
  const leaks = [];
  for (const c of CASES) {
    const { leaked, submitted } = await guardCase(c);
    if (leaked) leaks.push(`with ${c.name}`);
    if (submitted) leaks.push(`with ${c.name}, far enough to submit the form`);
  }
  if (leaks.length > 0) fail(`held R started the seal fill ${leaks.join('; and ')}`);

  console.log('PASS seal-select-guard: control fill started from the body; no fill from a focused select, closed or with its picker open');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
