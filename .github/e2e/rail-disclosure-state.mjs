// A book rail keeps manual disclosure choices within the tab session, just as
// the library rail does. Current-note ancestors still win over stored choices.
// The inline initializer also works when the deferred enhancement is blocked;
// this checks ownership at DOMContentLoaded, not the timing of the first paint.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = '/notes/Course/C02.md';
const NEXT = '/notes/Course/C03.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['saved-open', 'saved-closed', 'current-chain', 'storage-defaults', 'inline-owner', 'denied-getter', 'denied-write'];
const disclosureWrite = 'sessionStorage.setItem(storageKey, JSON.stringify(stored));';
const disclosureGuard = /try\s*\{\s*(sessionStorage\.setItem\(storageKey,\s*JSON\.stringify\(stored\)\);)\s*\}\s*catch\s*\{\s*\/\/[^\n]*\s*\}/g;
const MUTATIONS = {
  'unguard-denied-getter': { target: 'denied-getter', needle: disclosureGuard, replacement: '$1', asset: true },
  'unguard-denied-write': { target: 'denied-write', needle: disclosureGuard, replacement: '$1', asset: true },
  'defer-restoration': { target: 'inline-owner', needle: /d\.open\s*=\s*want;/g, replacement: 'void want;' },
  'drop-restoration': { target: 'saved-open', needle: /d\.open\s*=\s*want;/g, replacement: 'void want;' },
  'ignore-closed-choice': { target: 'saved-closed', needle: /d\.open\s*=\s*want;/g, replacement: 'd.open = true;' },
  'ignore-current-chain': { target: 'current-chain', needle: /if\s*\(d\.hasAttribute\("data-chain"\)\)/g, replacement: 'if (false)' },
  'replace-defaults': { target: 'storage-defaults', needle: /const want\s*=\s*stored\[d\.dataset\.key\];/g, replacement: 'const want = true;' },
};
class LockFired extends Error {
  constructor(site, message) { super(`FAIL rail-disclosure-state: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const check = (condition, site, message) => { if (!condition) throw new LockFired(site, message); };
// These journey controls preserve existing behavior. They are not independent
// mutation claims: each restoration mutation is caught on arrival first.
const preserve = (condition, message) => { if (!condition) throw new Error(message); };
for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) throw new Error(`unknown site for ${name}`);
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) throw new Error(`no mutation for ${site}`);
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) process.exit(2);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
async function arm(page, site) {
  proof = null;
  if (!MUTATE || MUTATIONS[MUTATE].target !== site) return;
  const mode = MUTATIONS[MUTATE];
  let requests = 0;
  let invalid = false;
  await page.route(`${BASE}/**`, async (route) => {
    if (route.request().resourceType() !== 'document') return route.continue();
    const response = await route.fetch();
    const original = await response.text();
    const matches = [...original.matchAll(mode.needle)].length;
    requests += 1;
    invalid ||= matches !== 1;
    await route.fulfill({ response, body: matches === 1 ? original.replace(mode.needle, mode.replacement) : original });
  });
  proof = () => requests > 0 && !invalid;
}
function prove() {
  if (proof && !proof()) throw new NotApplied('the mutation did not match exactly one initializer in every requested document');
}
function side(page) {
  return page.locator('#nav-rail details[data-key]').filter({ has: page.locator(':scope > summary', { hasText: '支線' }) });
}
async function fixture(page) {
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (await side(page).count() !== 1 || await page.locator('#nav-rail details[data-chain]').count() === 0) {
    throw new Error('fixture needs one side branch and the current lesson ancestry');
  }
  return side(page).getAttribute('data-key');
}
async function isOpen(page) { return side(page).evaluate((element) => element.open); }
async function seed(page, value) {
  await page.evaluate((stored) => sessionStorage.setItem('yomihon.nav', JSON.stringify(stored)), value);
}
// Arrival can scale the page while a click is being aimed at its summary.
const arrived = (page) => page.waitForFunction(async () => {
  if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
  await Promise.all(document.getAnimations().filter((a) => a.animationName === 'y-come-forward').map((a) => a.finished.catch(() => {})));
  return true;
}, null, { timeout: 3000 });

async function denyStorage(page, kind) {
  await page.addInitScript((denial) => {
    window.__deniedStorage = { reads: 0, writes: [], removals: [] };
    if (denial === 'getter') {
      Object.defineProperty(window, 'sessionStorage', { get() {
        window.__deniedStorage.reads += 1;
        throw new DOMException('storage denied', 'SecurityError');
      } });
    } else {
      Storage.prototype.setItem = (key) => {
        window.__deniedStorage.writes.push(key);
        throw new DOMException('storage denied', 'SecurityError');
      };
      Storage.prototype.removeItem = (key) => {
        window.__deniedStorage.removals.push(key);
        throw new DOMException('storage denied', 'SecurityError');
      };
    }
  }, kind);
}

// Instrument the actual write boundary in both states. The counter proves a
// native toggle reached the write, even when the catch prevents a page error.
async function armDisclosureWrite(page, site) {
  const mode = MUTATE && MUTATIONS[MUTATE].target === site ? MUTATIONS[MUTATE] : null;
  let requests = 0;
  let invalid = false;
  await page.route('**/sidebar.js', async (route) => {
    const response = await route.fetch();
    let source = await response.text();
    requests += 1;
    const writes = source.split(disclosureWrite).length - 1;
    const guards = mode ? [...source.matchAll(mode.needle)].length : 1;
    invalid ||= writes !== 1 || guards !== 1;
    if (writes === 1 && guards === 1) {
      if (mode) source = source.replace(mode.needle, mode.replacement);
      source = source.replace(disclosureWrite, 'window.__disclosureWrites = (window.__disclosureWrites || 0) + 1; ' + disclosureWrite);
    }
    await route.fulfill({ response, body: source });
  });
  proof = () => requests > 0 && !invalid;
}

async function deniedJourney(kind) {
  const site = `denied-${kind}`;
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  try {
    await denyStorage(page, kind);
    await armDisclosureWrite(page, site);
    await fixture(page);
    await page.waitForSelector('html[data-js]');
    await arrived(page);
    prove();
    const input = page.locator('[data-nav-filter]');
    // Opening and closing twice exercises repeated refused writes, rather than
    // a listener that silently removed itself after the first refusal.
    let toggles = 0;
    for (const want of [true, false, true, false]) {
      await side(page).locator(':scope > summary').click();
      await page.waitForFunction((count) => window.__disclosureWrites >= count, ++toggles);
      preserve(await isOpen(page) === want, `${kind}: native disclosure lost open=${want}`);
    }
    const hits = await page.evaluate(() => window.__disclosureWrites);
    preserve(hits === 4, `${kind}: wanted four actual disclosure writes, got ${hits}`);
    console.log(`APPLIED-PROOF: ${site} write-boundary=${hits}`);
    check(errors.length === 0, site, `${kind}: manual toggles raised ${JSON.stringify(errors)}`);
    const manual = await page.evaluate(() => window.__deniedStorage);
    if (kind === 'write') preserve(manual.writes.filter((key) => key === 'yomihon.nav').length === hits, 'write-denial stimulus missed a disclosure write');
    await side(page).evaluate((details) => {
      window.__filterToggles = 0;
      details.addEventListener('toggle', () => { window.__filterToggles += 1; });
    });
    await input.fill('S01');
    await page.waitForFunction(() => window.__filterToggles === 1);
    preserve(await page.locator('#nav-rail a.ui-navitem:not([hidden])').count() === 1, `${kind}: typing did not narrow the rail`);
    await input.fill('');
    await page.waitForFunction(() => window.__filterToggles === 2);
    preserve(await page.locator('#nav-rail a.ui-navitem:not([hidden])').count() > 1, `${kind}: clearing did not restore the rail`);
    preserve(!(await isOpen(page)), `${kind}: clearing lost the server disclosure default`);
    const denial = await page.evaluate(() => window.__deniedStorage);
    if (kind === 'getter') preserve(denial.reads > hits, 'getter-denial stimulus reached no storage reads');
    else {
      preserve(denial.writes.includes('yomihon.nav.filter') && denial.removals.includes('yomihon.nav.filter'), 'write-denial stimulus missed filter set or clear');
    }
    preserve(errors.length === 0, `${kind}: filter interaction raised ${JSON.stringify(errors)}`);
  } finally {
    await context.close();
  }
}

try {
  for (const kind of ['getter', 'write']) await deniedJourney(kind);
  for (const want of [true, false]) {
    const site = want ? 'saved-open' : 'saved-closed';
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    const key = await fixture(page);
    // Start in the opposite state so the test must exercise a real toggle.
    await seed(page, { [key]: !want });
    await page.reload({ waitUntil: 'domcontentloaded' });
    if (await isOpen(page) === want) throw new Error(`fixture did not start opposite to ${site}`);
    await side(page).locator(':scope > summary').click();
    await page.waitForFunction(({ key: k, value }) => JSON.parse(sessionStorage.getItem('yomihon.nav') || '{}')[k] === value, { key, value: want });
    await arm(page, site);
    await page.reload({ waitUntil: 'domcontentloaded' });
    prove();
    check(await isOpen(page) === want, site, `reload lost saved open=${want}`);
    // Follow the next lesson's row in the chapter tree, preserving the same
    // browser tab.
    await page.locator(`#nav-rail a.ui-navitem[href="${NEXT}"]`).click();
    await page.waitForURL(BASE + NEXT);
    prove();
    preserve(await isOpen(page) === want, `next lesson lost saved open=${want}`);
    const input = page.locator('[data-nav-filter]');
    await input.fill('S01');
    await input.fill('');
    preserve(await isOpen(page) === want, `clearing a filter lost saved open=${want}`);
    preserve(await page.evaluate((k) => JSON.parse(sessionStorage.getItem('yomihon.nav') || '{}')[k], key) === want, 'filtering overwrote the manual choice');
    await context.close();
  }
  {
    const context = await browser.newContext();
    const page = await context.newPage();
    await fixture(page);
    const keys = await page.locator('#nav-rail details[data-chain]').evaluateAll((elements) => elements.map((e) => e.dataset.key));
    await seed(page, Object.fromEntries(keys.map((key) => [key, false])));
    await arm(page, 'current-chain');
    await page.reload({ waitUntil: 'domcontentloaded' });
    prove();
    check(await page.locator('#nav-rail details[data-chain]').evaluateAll((elements) => elements.every((e) => e.open)), 'current-chain', 'a stored closed choice hid the current lesson ancestry');
    await context.close();
  }
  // Invalid or inaccessible storage falls back to the server's native state.
  for (const stored of [null, '{bad', 'null', 'false', 'wrong-type', 'unavailable']) {
    const context = await browser.newContext();
    const page = await context.newPage();
    const key = await fixture(page);
    if (stored === 'unavailable') {
      await page.addInitScript(() => { Object.defineProperty(window, 'sessionStorage', { get() { throw new DOMException('blocked', 'SecurityError'); } }); });
    } else {
      await page.evaluate(({ raw, k }) => {
        if (raw === null) sessionStorage.removeItem('yomihon.nav');
        else sessionStorage.setItem('yomihon.nav', raw === 'wrong-type' ? JSON.stringify({ [k]: 'true' }) : raw);
      }, { raw: stored, k: key });
    }
    await arm(page, 'storage-defaults');
    await page.reload({ waitUntil: 'domcontentloaded' });
    prove();
    check(!(await isOpen(page)), 'storage-defaults', `${stored}: non-current branch lost its server default`);
    await context.close();
  }
  {
    const context = await browser.newContext();
    const page = await context.newPage();
    const key = await fixture(page);
    await seed(page, { [key]: true });
    await arm(page, 'inline-owner');
    let blocked = false;
    await page.route('**/yomihon.js{,?*}', (route) => { blocked = true; return route.abort(); });
    await page.reload({ waitUntil: 'domcontentloaded' });
    if (!blocked) throw new Error('deferred-script control blocked nothing');
    prove();
    check(await isOpen(page), 'inline-owner', 'restoration depends on the deferred enhancement');
    await context.close();
  }
  {
    const context = await browser.newContext({ javaScriptEnabled: false });
    const page = await context.newPage();
    await fixture(page);
    preserve(!(await isOpen(page)), 'no-JavaScript default changed');
    await side(page).locator(':scope > summary').click();
    preserve(await isOpen(page), 'native disclosure no longer opens without JavaScript');
    await context.close();
  }
  console.log('PASS rail-disclosure-state: tab-session choices survive reload, navigation and filtering; ancestry and native defaults remain');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    console.error(error);
    if (error instanceof LockFired) console.log(`caught: ${error.site}`);
    if (error instanceof LockFired && MUTATE && proof) {
      if (!proof()) {
        console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
        process.exitCode = 2;
      } else if (error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode ||= 1;
  }
} finally {
  await browser.close();
}
