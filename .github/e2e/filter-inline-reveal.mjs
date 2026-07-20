// Behavior lock: the sidebar filter box is revealed by the document's own
// inline script and not by the deferred enhancement file, and it stays hidden
// when JavaScript is off. Three cases pin that together:
//   1. normal load        -> visible once the document is parsed
//   2. the yomihon.js entry aborted -> still visible, so the inline script revealed it
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
// with a sidebar, named directly). The mutations route on exactly the URL the
// browser is sent to and answer it with the body they rewrote. Aim PAGE_PATH at
// a path that redirects and the rewrite still lands — the fetch behind the route
// follows the redirect — but the browser is then handed that body at the
// original URL and never performs the redirect, so a mutated run stops loading
// the page the way the plain run does. MUTATE names one of the self-test modes
// below; MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const FILTER = 'input[data-nav-filter]';
const MUTATE = process.env.MUTATE || '';

// The assertions that can fire the lock, one per case, each named for what it
// guards. The three cases run in turn, so "the lock fired" does not say which
// one: a regression in the first case stops the run before the second is ever
// entered. A mutation names the site it aims at, and that name does two jobs —
// it picks the case the mutation is installed on, and it is the only site whose
// firing that mode may report as a catch.
const SITES = ['reveal-on-normal-load', 'reveal-without-the-deferred-script', 'hidden-without-javascript'];

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle matched
// nothing. Only the first is ever reported as a caught mutation — otherwise a
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN filter-inline-reveal: an assertion names the unknown site ${site}`);
  throw new LockFired(site, `FAIL filter-inline-reveal: ${msg}`);
};
const broken = (msg) => { throw new ProbeBroken(`BROKEN filter-inline-reveal: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED filter-inline-reveal: ${msg}`); };

// Serves the document with a needle removed, and hands back the proof it removed
// something. A mutation is installed on one page and answers for that page
// alone: one matching nothing leaves the document whole, and the case would then
// watch the very behavior the mutation meant to prevent and call it a self-test.
//
// Every occurrence goes, not the first. A document that grew a second inline
// script, or a second hidden control, would otherwise be rewritten in part and
// reported as rewritten whole.
const rewriteDocument = (needle, replacement) => async (page) => {
  let applied = false;
  await page.route(BASE + PAGE, async (route) => {
    const res = await route.fetch();
    const original = await res.text();
    const body = original.replaceAll(needle, replacement);
    if (body !== original) applied = true;
    return route.fulfill({ response: res, body });
  });
  return () => applied;
};

// Every mutation this probe can inject lives in this table; the dispatch below
// is a lookup into it, and MUTATE=list prints its keys. A mode that exists but
// is not listed cannot happen. Each names the assertion site it aims at, which
// picks the case it is installed on and is the only site whose firing that mode
// may report as a catch.
//
// Removing the inline scripts leaves the deferred file as the only thing that
// could reveal the filter, and it never does — so the reveal fails on a normal
// load, and fails again when the deferred file is blocked as well. Serving the
// filter without its hidden attribute makes the control show a face no script
// can work, which is what the third case exists to refuse.
const MUTATIONS = {
  'strip-inline-normal': {
    target: 'reveal-on-normal-load',
    apply: rewriteDocument(/<script nonce="[^"]+">[\s\S]*?<\/script>/g, ''),
  },
  'strip-inline-blocked': {
    target: 'reveal-without-the-deferred-script',
    apply: rewriteDocument(/<script nonce="[^"]+">[\s\S]*?<\/script>/g, ''),
  },
  'unhide-filter': {
    target: 'hidden-without-javascript',
    apply: rewriteDocument('data-nav-filter hidden>', 'data-nav-filter>'),
  },
};

// A mutation aiming at a site no assertion carries could never be caught, and
// the run would read as a probe that let the regression walk past it. An
// assertion no mutation aims at is a lock nothing has ever watched fail. Both
// are checked before anything else, so even MUTATE=list refuses to answer for a
// table that has drifted from the assertions it is supposed to cover.
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`filter-inline-reveal: mutation ${name} aims at the unknown assertion site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`filter-inline-reveal: the ${site} assertion is aimed at by no mutation, so nothing shows it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`filter-inline-reveal: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Installs the mutation on this case's page when the mode aims at this case's
// assertion, and hands back the proof it applied. A case the mode does not aim
// at runs untouched: a lock that fires there is a real regression, not a catch.
const arm = async (page, site) => {
  if (!MUTATE || MUTATIONS[MUTATE].target !== site) return null;
  return MUTATIONS[MUTATE].apply(page);
};

// A mutation that was armed on this page has to show it changed something: one
// matching nothing leaves the document whole, and the case would go on to watch
// the very behavior the mutation meant to remove.
const proveApplied = (proof) => {
  if (proof && !proof()) notApplied(`the ${MUTATE} mutation changed nothing in the document this page loaded`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: normal load — revealed by the time the document is parsed.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const applied = await arm(page, 'reveal-on-normal-load');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveApplied(applied);
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (hidden) fail('reveal-on-normal-load', 'case 1 (normal): filter still hidden once the document was parsed');
    await page.close();
  }

  // Case 2: deferred script blocked — the document's own inline script alone
  // must have revealed the filter.
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    // The same reason a mutation proves it applied: a route that matches
    // nothing blocks nothing, and this case would then watch the deferred
    // script run and call it proof that it never did.
    let blocked = false;
    await page.route('**/yomihon.js', (route) => { blocked = true; return route.abort(); });
    const applied = await arm(page, 'reveal-without-the-deferred-script');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (!blocked) {
      broken('case 2 blocked nothing: the deferred enhancement script was never requested, so a visible filter proves nothing about the inline script');
    }
    proveApplied(applied);
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (hidden) fail('reveal-without-the-deferred-script', 'case 2 (deferred blocked): reveal depends on the deferred script');
    await page.close();
  }

  // Case 3: JavaScript off — the control is inert and must show no face.
  {
    const ctx = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    const applied = await arm(page, 'hidden-without-javascript');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveApplied(applied);
    const hidden = await page.$eval(FILTER, (el) => el.hidden);
    if (!hidden) fail('hidden-without-javascript', 'case 3 (JS off): filter visible without the script that makes it work');
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
