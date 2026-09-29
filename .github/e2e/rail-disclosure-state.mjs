// A book rail keeps manual disclosure choices within the tab session, just as
// the library rail does. Current-note ancestors still win over stored choices.
// The inline initializer also works when the deferred enhancement is blocked;
// this checks ownership at DOMContentLoaded, not the timing of the first paint.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = '/notes/Course/C02.md';
const NEXT = '/notes/Course/C03.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['saved-open', 'saved-closed', 'current-chain', 'storage-defaults', 'inline-owner'];
const MUTATIONS = {
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
try {
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
    // Use the ordinary next-lesson link, preserving the same browser tab.
    await page.locator(`#nav-rail .y-lessonsteps a[href="${NEXT}"]`).click();
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
    await page.route('**/yomihon.js', (route) => { blocked = true; return route.abort(); });
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
