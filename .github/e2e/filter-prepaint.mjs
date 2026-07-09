// Behavior lock: the sidebar filter box is revealed by the sidebar's
// pre-paint inline script, not by the deferred enhancement file, and it
// stays hidden when JavaScript is off. Three cases pin that together:
//   1. normal load        -> visible at DOMContentLoaded
//   2. yomihon.js aborted -> still visible (only the inline script ran)
//   3. JavaScript off     -> stays hidden (an inert control shows no face)
// Case 2 is the one that catches the regression class (the reveal drifting
// back into the deferred file, which paints a pop-in after every navigation).
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a note page
// with a sidebar, requested directly — the strip-inline route matches the
// exact page URL, so a redirecting path would dodge it).
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const FILTER = 'input[data-nav-filter]';
// MUTATE=strip-inline self-tests the probe: the document is served with its
// inline scripts removed, so the pre-paint reveal never runs and case 2
// (deferred script blocked) must go red.
const MUTATE = process.env.MUTATE || '';

const fail = (msg) => { throw new Error(`FAIL filter-prepaint: ${msg}`); };

// Tracks that the strip-inline mutation really removed something: a
// mutation that matches nothing produces a green self-test that means
// nothing, so case 2 checks this flag before trusting its own result.
let stripped = false;
const stripInline = async (page) => {
  await page.route(BASE + PAGE, async (route) => {
    const res = await route.fetch();
    const original = await res.text();
    const body = original.replace(/<script>[\s\S]*?<\/script>/g, '');
    if (body !== original) stripped = true;
    return route.fulfill({ response: res, body });
  });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: normal load — visible by DOMContentLoaded.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (hidden) fail('case 1 (normal): filter still hidden at DOMContentLoaded');
    await page.close();
  }

  // Case 2: deferred script blocked — the inline pre-paint script alone
  // must have revealed the filter.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    // The same reason the strip-inline flag exists: a route that matches
    // nothing blocks nothing, and this case would then watch the deferred
    // script run and call it proof that it never did.
    let blocked = false;
    await page.route('**/yomihon.js', (route) => { blocked = true; return route.abort(); });
    if (MUTATE === 'strip-inline') await stripInline(page);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (!blocked) {
      fail('case 2 blocked nothing: the deferred enhancement script was never requested, so a visible filter proves nothing about the inline script');
    }
    if (MUTATE === 'strip-inline' && !stripped) {
      fail('strip-inline mutation did not apply: no inline script block was removed');
    }
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (hidden) fail('case 2 (deferred blocked): reveal depends on the deferred script');
    await page.close();
  }

  // Case 3: JavaScript off — the control is inert and must show no face.
  {
    const ctx = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (!hidden) fail('case 3 (JS off): filter visible without the script that makes it work');
    await ctx.close();
  }

  console.log('PASS filter-prepaint: visible pre-paint (normal + blocked), hidden with JS off');
} catch (err) {
  console.error(err instanceof Error && err.message.startsWith('FAIL') ? err.message : err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
