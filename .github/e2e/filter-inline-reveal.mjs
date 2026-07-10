// Behavior lock: the sidebar filter box is revealed by the document's own
// inline script and not by the deferred enhancement file, and it stays hidden
// when JavaScript is off. Three cases pin that together:
//   1. normal load        -> visible once the document is parsed
//   2. yomihon.js aborted -> still visible, so the inline script revealed it
//   3. JavaScript off     -> stays hidden (an inert control shows no face)
// Case 2 is the one that catches the regression class: the reveal drifting back
// into the deferred file, which shows a hidden control on every navigation and
// then pops it open once the file has run.
//
// What this probe does not watch is the paint. It reads the hidden attribute
// after the document has been parsed, so a reveal that the inline script
// deferred onto a timer would read as visible here just the same. Which script
// performs the reveal is what these cases establish; when it lands relative to
// the first frame is not, and no assertion below should be read as saying so.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a note page
// with a sidebar, requested directly — the strip-inline route matches the
// exact page URL, so a redirecting path would dodge it). MUTATE names one of
// the self-test modes below; MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const FILTER = 'input[data-nav-filter]';
const MUTATE = process.env.MUTATE || '';

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle matched
// nothing. Only the first is ever reported as a caught mutation — otherwise a
// crash that happens to exit 1 would read as a detection.
class LockFired extends Error {}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (msg) => { throw new LockFired(`FAIL filter-inline-reveal: ${msg}`); };
const broken = (msg) => { throw new ProbeBroken(`BROKEN filter-inline-reveal: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED filter-inline-reveal: ${msg}`); };

// Every mutation this probe can inject lives in this table; the dispatch below
// is a lookup into it, and MUTATE=list prints its keys. A mode that exists but
// is not listed cannot happen. strip-inline serves the document with its inline
// scripts removed, leaving the deferred file as the only thing that could
// reveal the filter — and case 2 blocks that file, so case 2 must go red.
//
// A mutation is installed on one page and answers for that page alone: it hands
// back the proof that it really removed something from what that page loaded. A
// mutation matching nothing leaves the document whole, and the case would then
// watch the reveal it meant to prevent and call the result a self-test.
const MUTATIONS = {
  'strip-inline': async (page) => {
    let applied = false;
    await page.route(BASE + PAGE, async (route) => {
      const res = await route.fetch();
      const original = await res.text();
      const body = original.replace(/<script>[\s\S]*?<\/script>/g, '');
      if (body !== original) applied = true;
      return route.fulfill({ response: res, body });
    });
    return () => applied;
  },
};

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`filter-inline-reveal: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: normal load — revealed by the time the document is parsed.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (hidden) fail('case 1 (normal): filter still hidden once the document was parsed');
    await page.close();
  }

  // Case 2: deferred script blocked — the document's own inline script alone
  // must have revealed the filter. Every mutation this probe carries belongs
  // to this case, because this is the case that carries the lock.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    // The same reason a mutation proves it applied: a route that matches
    // nothing blocks nothing, and this case would then watch the deferred
    // script run and call it proof that it never did.
    let blocked = false;
    await page.route('**/yomihon.js', (route) => { blocked = true; return route.abort(); });
    let mutationApplied = null;
    if (MUTATE) mutationApplied = await MUTATIONS[MUTATE](page);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (!blocked) {
      broken('case 2 blocked nothing: the deferred enhancement script was never requested, so a visible filter proves nothing about the inline script');
    }
    if (MUTATE && !mutationApplied()) {
      notApplied(`the ${MUTATE} mutation changed nothing in the document this page loaded`);
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

  console.log('PASS filter-inline-reveal: revealed without the deferred script (so the inline script reveals it), hidden with JS off');
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
