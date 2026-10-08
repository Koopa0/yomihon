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
import { randomUUID } from 'node:crypto';
import { lstat, mkdir, realpath, rm, writeFile } from 'node:fs/promises';
import { basename, dirname, join } from 'node:path';
import { setTimeout as pause } from 'node:timers/promises';
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const PHONE = { width: 390, height: 844 };
const LESSON = 'Writing/lessons/japanese/L01.md';
const CONCEPT = 'Concepts/japanese/は.md';
const SLOT = '.y-slotcard [data-uncertainty-control] button';
const SHEET = '[data-concept-sheet][open] [data-uncertainty-control] button';

const SITES = ['slot-mark-survives-reload', 'concept-mark-names-its-source', 'lost-mark-removes-original-pair'];

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
  await page.route('**/uncertainty.js{,?*}', async (route) => {
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
  'lost-removal-stays-disabled': {
    target: 'lost-mark-removes-original-pair',
    apply: rewriteModule('button.disabled = !available || !keys.has(key);', 'button.disabled = true;', 'the initial stored-pair removal gate'),
  },
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

const owned = [{ path: LESSON }, { path: CONCEPT }];
let folder;
let fixturePath;
let caughtMutation = '';
let mutationProof = () => '';
const publication = async (path, present, marker = '') => {
  const deadline = Date.now() + 12000;
  while (Date.now() < deadline) {
    const response = await fetch(BASE + '/notes/' + path.split('/').map(encodeURIComponent).join('/'));
    const body = await response.text();
    if ((present && response.status === 200 && body.includes(marker)) || (!present && response.status === 404)) return;
    await pause(100);
  }
  throw new ProbeBroken(`BROKEN uncertainty-marks: fixture publication did not reach ${present} for ${path}`);
};

// Cleanup touches only the exact pairs this probe owns, never another reader's mark.
const clearMarks = async () => {
  try {
    const response = await fetch(`${BASE}/uncertainties`);
    if (!response.ok) throw new Error(`read cleanup marks: ${response.status}`);
    const held = await response.json();
    for (const { path, anchor } of held) {
      if (!owned.some((pair) => pair.path === path && pair.anchor === anchor)) continue;
      const removed = await fetch(`${BASE}/uncertainties`, { method: 'POST', body: new URLSearchParams({ path, anchor }) });
      if (!removed.ok || (await removed.json()).marked !== false) throw new Error('owned mark was not removed');
    }
    const verified = await fetch(`${BASE}/uncertainties`);
    if (!verified.ok) throw new Error(`verify cleanup marks: ${verified.status}`);
    const left = await verified.json();
    if (left.some(({ path, anchor }) => owned.some((pair) => pair.path === path && pair.anchor === anchor))) throw new Error('owned mark remains');
    if (folder) {
      await rm(folder, { recursive: true });
      await publication(fixturePath, false);
    }
    return true;
  } catch (err) {
    console.error(`BROKEN uncertainty-marks: cleanup failed: ${err}`);
    process.exitCode = 2;
    return false;
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: PHONE });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : () => '';
  mutationProof = proof;
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
  owned[0].anchor = anchor;
  owned[1].anchor = 'scheduling-details';
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
  // An admitted real place is lost only after a scanner publication. Both
  // widths must arrive at that state before geometry or native input is tested.
  const supplied = process.env.YOMIHON_FIXTURE_ROOT;
  if (!supplied) throw new ProbeBroken('BROKEN uncertainty-marks: no disposable fixture root');
  const root = await realpath(supplied);
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) throw new ProbeBroken('BROKEN uncertainty-marks: unsafe fixture root');
  const notes = join(root, 'Notes');
  const info = await lstat(notes);
  if (!info.isDirectory() || info.isSymbolicLink()) throw new ProbeBroken('BROKEN uncertainty-marks: Notes is not an ordinary directory');
  const id = randomUUID();
  folder = join(notes, `uncertainty-${id}`);
  fixturePath = `Notes/uncertainty-${id}/source.md`;
  await mkdir(folder);
  const file = join(folder, 'source.md');
  await writeFile(file, `---\ntitle: Uncertainty ${id}\n---\n# Uncertainty ${id}\n\n## Original place\n\nBefore ${id}.\n`, { flag: 'wx' });
  await publication(fixturePath, true, `Before ${id}`);
  owned.push({ path: fixturePath, anchor: 'original-place' });
  const admitted = await page.request.post(`${BASE}/uncertainties`, { form: { path: fixturePath, anchor: 'original-place' } });
  if (admitted.status() !== 200 || (await admitted.json()).marked !== true) throw new ProbeBroken('BROKEN uncertainty-marks: real fixture pair was not admitted');
  const saved = await records(page);
  await writeFile(file, `---\ntitle: Uncertainty ${id}\n---\n# Uncertainty ${id}\n\n## Changed place\n\nAfter ${id}.\n`);
  await publication(fixturePath, true, `After ${id}`);
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 844 });
    for (const language of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        await context.addCookies([
          { name: 'yomihon_lang', value: language, url: BASE },
          { name: 'yomihon_theme', value: theme, url: BASE },
        ]);
        for (const address of ['/', '/open-thoughts']) {
          await page.goto(BASE + address, { waitUntil: 'load' });
          await arrived(page);
          applied();
          const state = await page.evaluate(() => ({
            language: document.documentElement.lang,
            theme: document.documentElement.dataset.theme,
          }));
          if (state.language !== language || state.theme !== theme) {
            throw new ProbeBroken(`BROKEN uncertainty-marks: ${address} rendered ${JSON.stringify(state)}, want ${language}/${theme}`);
          }
          const selector = `[data-uncertainty-remove][data-uncertainty-path="${fixturePath}"]`;
          try {
            await page.locator(selector).waitFor({ state: 'attached', timeout: 4000 });
            await ready(page, selector);
          } catch {
            fail('lost-mark-removes-original-pair', `${address}: lost-pair removal never arrived usable`);
          }
          if (JSON.stringify(await records(page)) !== JSON.stringify(saved)) fail('lost-mark-removes-original-pair', 'reading changed stored identities or times');
          const geometry = await page.locator(selector).evaluate((button) => {
            const box = button.getBoundingClientRect();
            return { outside: !button.closest('a'), fits: box.left >= 0 && box.right <= innerWidth, disabled: button.disabled };
          });
          if (!geometry.outside || !geometry.fits || geometry.disabled) fail('lost-mark-removes-original-pair', `${address}: native removal is inside a link, clipped, or disabled`);
        }
      }
    }
  }
  const removal = page.locator(`[data-uncertainty-remove][data-uncertainty-path="${fixturePath}"]`);
  await removal.focus();
  await removal.press('Enter');
  try {
    await page.waitForFunction(() => !document.querySelector('[data-uncertainty-remove]'), {}, { timeout: 4000 });
  } catch {
    fail('lost-mark-removes-original-pair', 'native removal did not refresh the shelf');
  }
  if ((await records(page)).length !== 0) fail('lost-mark-removes-original-pair', 'native removal left the original pair stored');
  await context.close();
  console.log('PASS uncertainty-marks: a phone-width slot mark survives a reload and clears; a concept mark names its own note; lost-pair removal remains usable at both widths, languages, and themes');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired && MUTATE && mutationProof()) {
    console.error(`NOT-APPLIED uncertainty-marks: ${MUTATE}: ${mutationProof()}`);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (err.site === target) caughtMutation = MUTATE;
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
    }
    process.exitCode = 1;
  } else {
    console.error(err instanceof ProbeBroken ? err.message : err);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
  const clean = await clearMarks();
  if (clean && caughtMutation) console.log(`MUTATE-RESULT: caught ${caughtMutation}`);
}
