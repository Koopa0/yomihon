// A reader can search a word's middle or overlapping tokens and still arrive
// at the first matching article passage. Later repeated text must not decide
// the destination, and the probe measures that passage rather than scrollY.
import assert from 'node:assert/strict';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const cases = [
  { name: 'prefixed-tail', query: 'nthanu', term: 'nthanum' },
  { name: 'merged-mark', query: 'nthanu nthanum', term: 'nthanum' },
  { name: 'merged-phrase', query: 'nthanu "nthanum here"', term: 'nthanum%20here' },
];
const mutations = {
  'cut-prefixed-tail': { site: 'prefixed-tail', replacement: 'closes%20with%20la-,nthanu' },
  'infer-merged-mark': { site: 'merged-mark', replacement: 'nthanum' },
  'lose-selected-phrase': { site: 'merged-phrase', replacement: 'closes%20with%20la-,nthanum' },
};
class LockFired extends Error {}
class NotApplied extends Error {}
for (const test of cases) {
  assert.ok(Object.values(mutations).some(m => m.site === test.name), `no mutation for ${test.name}`);
}
for (const mutation of Object.values(mutations)) {
  assert.ok(cases.some(test => test.name === mutation.site), `unknown site ${mutation.site}`);
}
if (MUTATE === 'list') {
  for (const name of Object.keys(mutations)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(mutations, MUTATE)) process.exit(2);
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let site = '';
let applied = 0;
try {
  for (const width of [390, 1600]) {
    for (const test of cases) {
      site = test.name;
      const context = await browser.newContext({ viewport: { width, height: 900 } });
      try {
        const page = await context.newPage();
        const path = '/search?q=' + encodeURIComponent(test.query);
        let matches = 0;
        let requests = 0;
        if (MUTATE && mutations[MUTATE].site === site) {
          await page.route(BASE + path, async (route) => {
            const response = await route.fetch();
            const original = await response.text();
            const needle = '#:~:text=closes%20with%20la-,' + test.term;
            const count = original.split(needle).length - 1;
            requests += 1;
            matches += count;
            await route.fulfill({ response, body: original.replace(needle, '#:~:text=' + mutations[MUTATE].replacement) });
          });
        }
        await page.goto(BASE + '/notes/Notes/Audit%20cross%20block.md');
        const target = page.locator('main article p').filter({ hasText: 'The ledger entry closes with lanthanum here.' });
        assert.equal(await target.count(), 1, 'fixture must identify one intended passage');
        assert.ok(await target.evaluate(e => e.getBoundingClientRect().top > innerHeight), 'fixture target must begin below the screen');
        await page.goto(BASE + path);
        if (MUTATE && mutations[MUTATE].site === site) {
          if (requests < 1 || matches !== requests) throw new NotApplied(`mutation matched ${matches} results over ${requests} requests, want 1 each`);
          applied += matches;
        }
        const row = page.locator('a.y-result[href*="Audit%20cross%20block"]').first();
        const href = await row.getAttribute('href');
        await row.click();
        await page.waitForLoadState('load');
        await page.waitForFunction(() => {
          const p = [...document.querySelectorAll('main article p')].find(e => e.textContent.includes('The ledger entry closes with lanthanum here.'));
          const box = p.getBoundingClientRect();
          return box.height > 0 && box.top >= 0 && box.bottom <= innerHeight;
        }, null, { timeout: 3000 }).catch(error => {
          if (error.name !== 'TimeoutError') throw error;
        });
        const seen = await target.evaluate(e => {
          const box = e.getBoundingClientRect();
          return { top: box.top, bottom: box.bottom, height: box.height, viewport: innerHeight };
        });
        if (seen.height <= 0 || seen.top < 0 || seen.bottom > seen.viewport) {
          throw new LockFired(`${site} at ${width}px did not reach the intended passage: ${JSON.stringify(seen)}`);
        }
        if (!href.endsWith('#:~:text=closes%20with%20la-,' + test.term)) {
          throw new LockFired(`${site}: selected mark lost its source interval: ${href}`);
        }
        console.log(`PASS directive-edges: ${site} at ${width}px reaches the first article passage`);
      } finally {
        await context.close();
      }
    }
  }
  if (MUTATE) throw new Error('mutation escaped the lock');
} catch (error) {
  console.error(`FAIL directive-edges: ${site}: ${error.message}`);
  if (error instanceof NotApplied || (MUTATE && applied === 0)) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (error instanceof LockFired && MUTATE && mutations[MUTATE].site === site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
