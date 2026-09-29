// Behavior lock for the "not sure yet" mark a reader sets on a phone.
//
// The Go tests own what the route accepts and stores. What only a live browser
// shows is that the two halves agree: the control a reader presses on a slot
// card or in a concept sheet, and the record the server keeps. A mark set at
// 390px has to survive a reload and clear on a second press, and a mark set in
// a concept sheet has to name the concept's own note rather than the lesson it
// was opened from.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson with a slot card and a concept link
// naming a section), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const PHONE = { width: 390, height: 844 };
const LESSON = 'Writing/lessons/japanese/L01.md';
const CONCEPT = 'Concepts/japanese/は.md';
const SLOT = '.y-slotcard [data-uncertainty-control] button';
const SHEET = '[data-concept-sheet][open] [data-uncertainty-control] button';

const SITES = ['slot-mark-survives-reload', 'concept-mark-names-its-source'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN uncertainty-marks: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL uncertainty-marks: ${message}`);
};

// rewriteModule edits the client module as served, never a file on disk. The
// needle has to match exactly once in every load.
const rewriteModule = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/uncertainty.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the module was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) return `${label} needle matched [${perLoad.join(', ')}], want exactly 1 in each load`;
    return '';
  };
};

const MUTATIONS = {
  'post-becomes-a-read': {
    target: 'slot-mark-survives-reload',
    apply: rewriteModule("method: 'POST',", "method: 'GET',", 'the method that stores a mark'),
  },
  'forget-stored-marks-on-load': {
    target: 'slot-mark-survives-reload',
    apply: rewriteModule('for (const mark of marks) keys.add(keyOf(mark.path, mark.anchor));', 'void marks;', 'the read of stored marks'),
  },
  'concept-mark-names-the-lesson': {
    target: 'concept-mark-names-its-source',
    apply: rewriteModule("decodeURIComponent(address.pathname.slice('/notes/'.length))", 'article.dataset.uncertaintyPath', 'the path a concept control marks'),
  },
};

for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`uncertainty-marks: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`uncertainty-marks: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const records = async (page) => {
  const response = await page.request.get(`${BASE}/uncertainties`);
  if (response.status() !== 200) throw new ProbeBroken(`BROKEN uncertainty-marks: GET /uncertainties = ${response.status()}`);
  return response.json();
};

const settle = async (page, site, selector, wanted, what) => {
  try {
    await page.waitForFunction(({ target, value }) => {
      const button = document.querySelector(target);
      return button && !button.disabled && button.getAttribute('aria-pressed') === value;
    }, { target: selector, value: String(wanted) }, { timeout: 4000 });
  } catch {
    fail(site, `${what}: the control never settled at aria-pressed=${wanted}`);
  }
};

const ready = async (page, selector) => {
  try {
    await page.waitForFunction((target) => {
      const button = document.querySelector(target);
      return button && !button.disabled;
    }, selector, { timeout: 4000 });
  } catch {
    throw new ProbeBroken(`BROKEN uncertainty-marks: ${selector} never became usable`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: PHONE });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : () => '';
  const applied = () => {
    const issue = proof();
    if (issue) throw new NotApplied(`NOT-APPLIED uncertainty-marks: ${MUTATE}: ${issue}`);
  };
  if ((await records(page)).length !== 0) throw new ProbeBroken('BROKEN uncertainty-marks: the fixture starts with marks');

  // --- A slot-card mark survives a reload and clears on a second press ---
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await ready(page, SLOT);
  applied();
  const anchor = await page.locator('.y-slotcard__abstract[id]').first().getAttribute('id');
  await page.locator(SLOT).first().click();
  await settle(page, 'slot-mark-survives-reload', SLOT, true, 'after the first press');
  let held = await records(page);
  if (held.length !== 1 || held[0].path !== LESSON || held[0].anchor !== anchor) {
    fail('slot-mark-survives-reload', `stored ${JSON.stringify(held)}, want the one slot location ${LESSON}#${anchor}`);
  }
  await page.reload({ waitUntil: 'load' });
  await settle(page, 'slot-mark-survives-reload', SLOT, true, 'after a reload');
  await page.locator(SLOT).first().click();
  await settle(page, 'slot-mark-survives-reload', SLOT, false, 'after the second press');
  held = await records(page);
  if (held.length !== 0) fail('slot-mark-survives-reload', `a second press left ${JSON.stringify(held)}, want none`);

  // --- A concept-sheet mark names the concept's own note -----------------
  await page.locator('[data-concept][href*="#"]').first().click();
  await ready(page, SHEET);
  await page.locator(SHEET).click();
  await settle(page, 'concept-mark-names-its-source', SHEET, true, 'in the concept sheet');
  held = await records(page);
  if (held.length !== 1 || held[0].path !== CONCEPT || held[0].anchor !== 'scheduling-details') {
    fail('concept-mark-names-its-source', `stored ${JSON.stringify(held)}, want ${CONCEPT}#scheduling-details`);
  }
  await page.locator(SHEET).click();
  await settle(page, 'concept-mark-names-its-source', SHEET, false, 'clearing in the concept sheet');
  if ((await records(page)).length !== 0) fail('concept-mark-names-its-source', 'clearing left the mark stored');
  await context.close();
  console.log('PASS uncertainty-marks: a phone-width slot mark survives a reload and clears; a concept mark names its own note');
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
  } else {
    console.error(err instanceof ProbeBroken ? err.message : err);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
