// Behavior lock for the freshness watch's visibility gate. start() and tick()
// used to run at the end of the module unconditionally, so a note opened into
// a tab that was already hidden began polling and never received the
// visibilitychange that would have stopped it. After the gate, a hidden load
// asks nothing; a later visible moment — a tab brought forward, or the
// visibilitychange a back/forward-cache restore fires — starts the watch.
//
// A real hidden tab is not something this headless driver can keep honest
// across a navigation, so the probe stamps document.visibilityState before
// any page script runs and dispatches the same visibilitychange / pagehide
// events the module already listens for.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note that carries a freshness stamp), MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['hidden-at-load-is-quiet', 'restore-resumes'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN freshness-hidden-start: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL freshness-hidden-start: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN freshness-hidden-start: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED freshness-hidden-start: ${message}`); };

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/freshness.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const VISIBLE_START = `if (document.visibilityState === 'visible') {
    start();
    tick();
  }`;
const VISIBILITY_RESUME = `    start();
    tick();
  });`;

const MUTATIONS = {
  'start-while-hidden': {
    target: 'hidden-at-load-is-quiet',
    apply: rewriteModule(
      VISIBLE_START,
      `start();
    tick();`,
      'visible-only start',
    ),
  },
  'silence-visibility-resume': {
    target: 'restore-resumes',
    apply: rewriteModule(
      VISIBILITY_RESUME,
      `  });`,
      'visibilitychange resume',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`freshness-hidden-start: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`freshness-hidden-start: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`freshness-hidden-start: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  const freshness = [];
  page.on('request', (request) => {
    if (new URL(request.url()).pathname.startsWith('/freshness/')) {
      freshness.push(request.url());
    }
  });

  await page.addInitScript(() => {
    window.__yomihonVisibility = 'hidden';
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get() {
        return window.__yomihonVisibility;
      },
    });
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get() {
        return window.__yomihonVisibility !== 'visible';
      },
    });
  });

  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const stamped = await page.locator('[data-freshness-path][data-freshness-identity]').count();
  if (stamped < 1) broken('the note page carries no freshness stamp, so a quiet watch proves nothing');
  const visibility = await page.evaluate(() => document.visibilityState);
  if (visibility !== 'hidden') broken(`visibilityState is ${visibility} after load; the hidden stamp never took`);

  await page.waitForTimeout(400);
  if (freshness.length !== 0) {
    fail(
      'hidden-at-load-is-quiet',
      `a note hidden at load issued ${freshness.length} /freshness/ request(s); want 0`,
    );
  }

  // pagehide is the door a cache restore walks through; visibilitychange to
  // visible is the door the module says starts the watch again. The ask is
  // issued inside the evaluate, so the waiter has to be armed before it.
  const beforeResume = freshness.length;
  const resumed = page.waitForRequest(
    (request) => new URL(request.url()).pathname.startsWith('/freshness/'),
    { timeout: 2000 },
  );
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
    window.__yomihonVisibility = 'visible';
    document.dispatchEvent(new Event('visibilitychange'));
  });
  try {
    await resumed;
  } catch {
    fail(
      'restore-resumes',
      `after pagehide and a visibilitychange to visible, the watch issued ${freshness.length - beforeResume} /freshness/ request(s); want at least 1`,
    );
  }

  console.log('PASS freshness-hidden-start: a hidden load stays quiet and a restore starts the watch');
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
