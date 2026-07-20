// Behavior lock: a held R on a typing surface, or anywhere while the search
// dialog is open, never starts the seal hold. The observable is the seal fill:
// holdStart stretches every .y-sealfill to width 100%, so the guard holds
// exactly when that never happens.
//
// A select has two faces. With the picker closed the key
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
// page carrying the sidebar filter, search dialog, slot-machine <select>, and
// seal form). MUTATE names one of the self-test modes below; MUTATE=list prints
// them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
const SELECT = 'select.y-slotselect';
const SIDEBAR_INPUT = '[data-nav-filter]';
const SEARCH_DIALOG = '[data-search]';
const TEST_EDITABLE = '[data-e2e-contenteditable]';
const TEST_TEXTAREA = '[data-e2e-textarea]';

// The assertions that can fire the lock, each named for the clause it guards.
// Select's closed and open faces share one site because both exercise the same
// closest('select') clause; the open face also carries the historical weakening
// mutation that only an option target can catch.
const SITES = [
  'no-fill-from-inside-the-select',
  'no-fill-from-sidebar-input',
  'no-fill-from-textarea',
  'no-fill-from-contenteditable',
  'no-fill-while-search-dialog-open',
];

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle did not match
// exactly once. Only the first is ever reported as a caught mutation — otherwise a
// crash that happens to exit 1 would read as a detection — and only when the
// site that fired is the site the mode aimed at.
class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, msg) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN seal-select-guard: an assertion names the unknown site ${site}`);
  throw new LockFired(site, `FAIL seal-select-guard: ${msg}`);
};
const broken = (msg) => { throw new ProbeBroken(`BROKEN seal-select-guard: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED seal-select-guard: ${msg}`); };

// Serves the script with the guard's clause rewritten, and hands back the proof
// it rewrote something. A mutation is installed on one page and answers for that
// page alone: a proof shared between the cases would let a page served the
// script unchanged inherit the other page's success — and that case would then
// hold R against the guard it believed it had removed, find nothing leaking, and
// call the silence a pass.
//
// Exactly one occurrence is the contract. Zero means the mutation died against
// rewritten source; two means the needle is ambiguous and could rewrite a clause
// the mode never aimed at. Both leave the served script untouched and report
// not-applied rather than pretending to inject a sound regression.
const rewriteExactlyOnce = (original, needle, replacement) => {
  const matches = original.split(needle).length - 1;
  return {
    body: matches === 1 ? original.replace(needle, replacement) : original,
    matches,
  };
};

const requireUniqueNeedle = ({ matches, mode, name, needle }) => {
  if (matches === 1) return;
  const count = matches === null ? 'no fetched script' : `${matches} occurrence${matches === 1 ? '' : 's'}`;
  notApplied(`${name}: the ${mode} needle matched ${count}; want exactly one: ${needle}`);
};

// The ambiguous-needle branch cannot arise against today's script, so carry a
// literal control that proves a second occurrence leaves the source untouched.
// It also drives the same reporter guardCase uses and requires NotApplied, so
// neither half of the ambiguous-source contract can silently disappear.
const duplicateNeedleControl = rewriteExactlyOnce('guard || guard', 'guard', 'false');
let duplicateReportedNotApplied = false;
try {
  requireUniqueNeedle({ matches: duplicateNeedleControl.matches, mode: 'duplicate-control', name: 'duplicate control', needle: 'guard' });
} catch (err) {
  duplicateReportedNotApplied = err instanceof NotApplied;
}
if (duplicateNeedleControl.matches !== 2 || duplicateNeedleControl.body !== 'guard || guard' || !duplicateReportedNotApplied) {
  console.error('seal-select-guard: rewriteGuard did not leave an ambiguous source untouched and report it not-applied');
  process.exit(2);
}

const rewriteGuard = ({ needle, replacement }) => async (page) => {
  let matches = null;
  await page.route('**/shortcuts.js', async (route) => {
    const res = await route.fetch();
    const original = await res.text();
    const rewritten = rewriteExactlyOnce(original, needle, replacement);
    matches = rewritten.matches;
    const { body } = rewritten;
    return route.fulfill({ response: res, body });
  });
  return () => matches;
};

// Every mutation this probe can inject lives in this table; the dispatch below
// is a lookup into it, and MUTATE=list prints its keys. A mode that exists but
// is not listed cannot happen.
//
// Removing the guard makes both faces leak, and the closed face alone would
// report that. Weakening it to a test of the target's own tag name is the
// regression only the open picker can see: with the picker closed the key event
// targets the select, and a tag-name test still holds; with it open the event
// targets an option, whose tag name is not SELECT, and the shortcut fires. That
// weakening is the guard this script actually shipped before the picker was
// branded, so it is the regression the second case exists for — and without a
// mutation of its own, nothing would ever watch that case fail.
const MUTATIONS = {
  'unguard-select': {
    target: 'no-fill-from-inside-the-select',
    needle: "target.closest('select')",
    replacement: 'false',
  },
  'weaken-select-to-tagname': {
    target: 'no-fill-from-inside-the-select',
    needle: "target.closest('select')",
    replacement: "target.tagName === 'SELECT'",
  },
  'unguard-sidebar-input': {
    target: 'no-fill-from-sidebar-input',
    needle: "target.tagName === 'INPUT'",
    replacement: 'false',
  },
  'unguard-textarea': {
    target: 'no-fill-from-textarea',
    needle: "target.tagName === 'TEXTAREA'",
    replacement: 'false',
  },
  'unguard-contenteditable': {
    target: 'no-fill-from-contenteditable',
    needle: 'target.isContentEditable',
    replacement: 'false',
  },
  'unguard-open-search-dialog': {
    target: 'no-fill-while-search-dialog-open',
    needle: 'if (typing || search.isOpen()) return;',
    replacement: 'if (typing) return;',
  },
};

// A mutation aiming at a site no assertion carries could never be caught, and
// the run would read as a probe that let the regression walk past it. An
// assertion no mutation aims at is a lock nothing has ever watched fail. Both
// are checked before anything else, so even MUTATE=list refuses to answer for a
// table that has drifted from the assertions it is supposed to cover.
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`seal-select-guard: mutation ${name} aims at the unknown assertion site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`seal-select-guard: the ${site} assertion is aimed at by no mutation, so nothing shows it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`seal-select-guard: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// The customizable picker can move focus to its option before it is ready for
// the next held key. Waiting after every arrange keeps the hold out of that
// handoff window; the same rule for every case avoids a timing exception hidden
// inside the picker case.
const ARRANGE_SETTLE_MS = 100;

const someFillFull = () => [...document.querySelectorAll('.y-sealfill')].some((f) => f.style.width === '100%');
const noFillFull = () => [...document.querySelectorAll('.y-sealfill')].every((f) => f.style.width !== '100%');

// The shortcut starts or refuses the hold in its keydown handler. Read that
// synchronous result after Playwright has delivered the trusted event, then
// release immediately. Polling animation frames here makes scheduler load look
// like a product failure and holds the key closer to the submit deadline.
const holdRAndReadFill = async (page) => {
  await page.keyboard.down('r');
  try {
    return await page.evaluate(someFillFull);
  } finally {
    await page.keyboard.up('r');
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });

// Runs one case on a page of its own. A leaked hold latches the script's sealing
// state, so a second case sharing that page would see no fill and read the
// silence as a guard that worked. Separate pages keep each case's answer its own.
async function guardCase({ name, site, arrange }) {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // The seal form writes a real status transition. This probe never commits one:
  // every POST is aborted at the network layer, and a POST attempted during a
  // guarded negative case is itself a failure signal.
  let postAttempted = false;
  await page.route('**/*', (route) => {
    if (route.request().method() === 'POST') { postAttempted = true; return route.abort(); }
    return route.continue();
  });
  let mutationMatches = null;
  if (MUTATE) mutationMatches = await rewriteGuard(MUTATIONS[MUTATE])(page);
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');
  if (MUTATE) {
    const { needle } = MUTATIONS[MUTATE];
    requireUniqueNeedle({ matches: mutationMatches(), mode: MUTATE, name, needle });
  }
  // A mutation that only one case can catch leaves the others passing, which is
  // correct and says nothing. The verdict is taken across all cases below.

  if (!(await page.$('[data-seal]'))) broken('page has no seal form — pick a sealable lesson page');
  if (!(await page.$(SELECT))) broken('page has no slot select — pick a slot lesson page');
  if (!(await page.$(SIDEBAR_INPUT))) broken('page has no sidebar filter input — pick a reading page with the shared sidebar');
  if (!(await page.$(SEARCH_DIALOG))) broken('page has no search dialog — pick a page with the shared application shell');
  // Positive control: R held outside any typing surface starts the fill.
  await page.evaluate(() => document.body.focus());
  if (!(await holdRAndReadFill(page))) {
    broken(`${name}: held R on the body did not start the seal fill, so this probe cannot see the seal path at all`);
  }
  // Release must retract the fill before the real case runs: a fill left at full
  // width would read as a leak the case never caused.
  if (!(await page.evaluate(noFillFull))) {
    broken(`${name}: the seal fill never retracted after R was released, so the case below would start from a fill already at full width`);
  }
  // A completed hold would submit the form and latch the script's sealing state,
  // making the case below pass without testing the guard — so a POST during the
  // control window is a loud stop, not a quiet abort.
  if (postAttempted) {
    broken(`${name}: the positive control ran long enough to submit the seal form; the case below would be vacuous`);
  }

  await arrange(page);
  await page.waitForTimeout(ARRANGE_SETTLE_MS);

  // The lock: the same trusted keydown that starts the positive control must
  // leave the fill untouched when its target is guarded.
  const leaked = await holdRAndReadFill(page);
  const submitted = postAttempted;
  await page.close();
  return { leaked, site, submitted };
}

const CASES = [
  {
    name: 'a focused select',
    site: 'no-fill-from-inside-the-select',
    arrange: async (page) => {
      await page.focus(SELECT);
      const tag = await page.evaluate(() => document.activeElement.tagName);
      if (tag !== 'SELECT') broken(`focusing the slot select left focus on ${tag}`);
    },
  },
  {
    name: 'the select picker open on a focused option',
    site: 'no-fill-from-inside-the-select',
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
  {
    name: 'the focused sidebar filter input',
    site: 'no-fill-from-sidebar-input',
    arrange: async (page) => {
      await page.focus(SIDEBAR_INPUT);
      const state = await page.evaluate((selector) => {
        const input = document.querySelector(selector);
        return { focused: document.activeElement === input, hidden: input.hidden };
      }, SIDEBAR_INPUT);
      if (state.hidden) broken('the sidebar filter input is still hidden after JavaScript initialized');
      if (!state.focused) broken('focusing the sidebar filter input did not make it the active element');
    },
  },
  {
    name: 'a focused contentEditable surface',
    site: 'no-fill-from-contenteditable',
    arrange: async (page) => {
      const state = await page.evaluate((selector) => {
        const editable = document.createElement('div');
        editable.contentEditable = 'true';
        editable.dataset.e2eContenteditable = '';
        document.body.append(editable);
        editable.focus();
        return {
          editable: editable.isContentEditable,
          focused: document.activeElement === document.querySelector(selector),
        };
      }, TEST_EDITABLE);
      if (!state.editable) broken('the test-local contentEditable surface is not editable');
      if (!state.focused) broken('focusing the test-local contentEditable surface did not make it the active element');
    },
  },
  {
    name: 'a focused textarea',
    site: 'no-fill-from-textarea',
    arrange: async (page) => {
      const state = await page.evaluate((selector) => {
        const textarea = document.createElement('textarea');
        textarea.dataset.e2eTextarea = '';
        document.body.append(textarea);
        textarea.focus();
        return {
          focused: document.activeElement === document.querySelector(selector),
          tag: document.activeElement?.tagName,
        };
      }, TEST_TEXTAREA);
      if (state.tag !== 'TEXTAREA') broken(`the test-local textarea focused ${state.tag} instead of TEXTAREA`);
      if (!state.focused) broken('focusing the test-local textarea did not make it the active element');
    },
  },
  {
    name: 'the open search dialog with a non-typing target focused',
    site: 'no-fill-while-search-dialog-open',
    arrange: async (page) => {
      await page.keyboard.press('Control+k');
      try {
        await page.waitForFunction((selector) => document.querySelector(selector)?.open, SEARCH_DIALOG, { timeout: 1000 });
      } catch {
        broken('Control+K did not open the search dialog');
      }
      // The dialog autofocuses its INPUT, which the typing clause already guards.
      // A test-local negative tabindex lets the modal itself take programmatic
      // focus, isolating the separate "dialog is open" clause under test.
      const state = await page.evaluate((selector) => {
        const dialog = document.querySelector(selector);
        dialog.tabIndex = -1;
        dialog.focus();
        return { focused: document.activeElement === dialog, open: dialog.open };
      }, SEARCH_DIALOG);
      if (!state.open) broken('the search dialog closed before its guard case ran');
      if (!state.focused) broken('the open search dialog did not accept programmatic focus for the non-typing-target case');
    },
  },
];

try {
  // Every case runs before any verdict, so the select mutation is seen to break
  // both faces rather than only the first one tried. If a mutated run also finds
  // an unrelated site broken, that site wins the verdict: calling an unrelated
  // assertion failure a catch would claim a detection nothing proved.
  const failures = [];
  for (const c of CASES) {
    const { leaked, site, submitted } = await guardCase(c);
    if (leaked) failures.push({ site, detail: `with ${c.name}` });
    if (submitted) failures.push({ site, detail: `with ${c.name}, far enough to submit the form` });
  }
  if (failures.length > 0) {
    const target = MUTATE ? MUTATIONS[MUTATE].target : null;
    const first = (MUTATE && failures.find((failure) => failure.site !== target)) || failures[0];
    const details = failures.filter((failure) => failure.site === first.site).map((failure) => failure.detail);
    fail(first.site, `held R started the seal fill ${details.join('; and ')}`);
  }

  console.log('PASS seal-select-guard: every case started the control fill from the body; no fill from select, sidebar input, textarea, contentEditable, or while search was open');
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
      else console.error(`no catch: ${MUTATE} injects a regression the ${target} assertion watches for, but the ${err.site} assertion fired first — something unrelated is broken`);
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
