// Behavior lock for the page change: a link carries the reader across the seam
// between two documents with a view transition, Back and Forward stay instant,
// a reader who asked for reduced motion gets a plain cut, a transition that
// stalls is ended by the page, and in every one of those the page that arrives
// paints.
//
// Whether the browser runs a transition for a given navigation is its own
// decision, and it declines now and then for reasons the page does not control.
// So a site that needs to see one tries a handful of navigations and passes on
// the first that carries it, and a site that needs to see none requires that
// every one of them carries none. A browser that never ran a transition at all
// would pass the second kind for the wrong reason, which is why each of those
// sites is paired with a mutation that has to make a transition appear.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note carrying the header's link to the
// reading choices), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'link-arrival-transitions',
  'back-arrival-is-instant',
  'reduced-motion-is-a-cut',
  'a-stalled-transition-ends',
  'arrival-paints',
];
const ATTEMPTS = 6;
const WATCHDOG_MS = 600;
const LINK = '.y-prefslink';
const ARRIVAL = '**/preferences**';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN page-crossfade: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL page-crossfade: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN page-crossfade: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED page-crossfade: ${message}`); };

// A rewrite is proven by counting: every response it was meant for went through
// it, and each held the needle exactly once. A needle that matches nothing is a
// mutation that never reached the thing it names, which must not read as a
// caught regression.
const rewriteResponses = (glob, isDocument, needle, replacement, label) => async (context) => {
  let responses = 0;
  let matches = 0;
  await context.route(glob, async (route) => {
    if (isDocument && route.request().resourceType() !== 'document') return route.fallback();
    const response = await route.fetch();
    const original = await response.text();
    responses += 1;
    matches += original.split(needle).length - 1;
    return route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (responses === 0) return `${label} was never requested, so the rewrite never reached it`;
    if (matches !== responses) return `${label} needle matched ${matches} times across ${responses} responses, want exactly once in each`;
    return '';
  };
};
const rewriteStylesheet = (needle, replacement, label) => rewriteResponses('**/static/app.css', false, needle, replacement, label);
const rewriteDocument = (needle, replacement, label) => rewriteResponses('**/*', true, needle, replacement, label);

const MUTATIONS = {
  // The opt-in is what makes a link animate at all.
  'drop-the-opt-in': {
    target: 'link-arrival-transitions',
    apply: rewriteStylesheet('@view-transition{navigation:auto}', '', 'the stylesheet opt-in'),
  },
  // Back and Forward are skipped by the page, from the script in the head.
  'drop-the-traverse-skip': {
    target: 'back-arrival-is-instant',
    apply: rewriteDocument(
      'window.navigation?.activation?.navigationType === "traverse"',
      'false',
      'the traverse guard',
    ),
  },
  // The gate is what keeps a transition from existing at all for a reduced-motion
  // reader. The blanket rule for the generated pseudo-elements only takes the
  // motion out of one that was started anyway, and this site asks for none.
  'drop-the-reduced-motion-gate': {
    target: 'reduced-motion-is-a-cut',
    apply: rewriteStylesheet(
      '@media (prefers-reduced-motion:no-preference){@view-transition',
      '@media all{@view-transition',
      'the motion gate',
    ),
  },
  // Without the watchdog a transition that never ends holds the arrival.
  'drop-the-watchdog': {
    target: 'a-stalled-transition-ends',
    apply: rewriteDocument(
      'setTimeout(() => transition.skipTransition(), 600);',
      '',
      'the watchdog',
    ),
  },
  // The paint oracle asks the arriving document for a frame. A document whose
  // frames never come is the failure it exists for, and this browser does not
  // have the freeze that causes it, so the frames are taken away instead: this
  // shows the oracle can say no, which is all it can show.
  'starve-the-arrival-of-frames': {
    target: 'arrival-paints',
    apply: async (context) => {
      await context.addInitScript(() => {
        if (location.pathname.startsWith('/preferences')) window.requestAnimationFrame = () => 0;
      });
      return () => '';
    },
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`page-crossfade: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`page-crossfade: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`page-crossfade: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Installed in every document before anything else runs, because the arrival is
// announced once to the document that receives it and cannot be asked for
// afterwards. This listener is registered before the page's own, so it sees the
// transition the page is about to skip; a skip shows afterwards as `ready`
// rejecting, and `finished` settles either way.
const recordArrival = () => {
  const arrival = {
    reveal: false,
    transition: false,
    type: null,
    ready: 'pending',
    finished: 'pending',
    revealAt: 0,
    finishedAt: 0,
  };
  window.__arrival = arrival;
  window.addEventListener('pagereveal', (event) => {
    arrival.reveal = true;
    arrival.revealAt = performance.now();
    arrival.type = window.navigation?.activation?.navigationType ?? null;
    const transition = event.viewTransition;
    arrival.transition = Boolean(transition);
    if (!transition) return;
    transition.ready.then(
      () => { arrival.ready = 'resolved'; },
      () => { arrival.ready = 'rejected'; },
    );
    const done = (outcome) => () => {
      arrival.finished = outcome;
      arrival.finishedAt = performance.now();
    };
    transition.finished.then(done('resolved'), done('rejected'));
  });
};

// A page that has not painted before the press has nothing to hold a snapshot
// of, and the browser then declines to run a transition at all.
const openFresh = async (page) => {
  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!response || response.status() !== 200) {
    broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  }
  await page.waitForSelector('html[data-js]');
  await page.evaluate(() => document.fonts.ready.then(
    () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
  ));
};

const followLink = async (page) => {
  if (await page.locator(LINK).count() !== 1) {
    broken(`${PAGE} carries ${await page.locator(LINK).count()} links to the reading choices, want exactly 1`);
  }
  try {
    await Promise.all([page.waitForURL(ARRIVAL, { timeout: 10_000 }), page.locator(LINK).click()]);
  } catch (held) {
    fail('arrival-paints', `the page reached by following a link never finished arriving: ${String(held.message).split('\n')[0]}`);
  }
};

// A transition that is going to run has settled well inside this; one that has
// been skipped settles at once; one that is stalled has not settled yet, which
// is for the caller to judge.
const settled = (page, timeout) => page.waitForFunction(
  () => window.__arrival?.reveal && (!window.__arrival.transition || window.__arrival.finished !== 'pending'),
  null,
  { timeout, polling: 50 },
).then(() => true, () => false);

const readArrival = (page) => page.evaluate(() => ({ ...window.__arrival, href: location.pathname }));

// The arriving page has to paint. This is the guarantee a frozen arrival
// breaks: a document complete in every other respect and never shown.
const requireFrames = async (page, what) => {
  const painted = await page.evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => resolve(true));
    setTimeout(() => resolve(false), 1000);
  }));
  if (!painted) fail('arrival-paints', `${what} delivered no frame within 1s`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const newContext = async (options = {}) => {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, ...options });
    await context.addInitScript(recordArrival);
    return context;
  };
  const checkProof = () => {
    const issue = proof ? proof() : '';
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  };

  // A link carries a transition, and Back does not. Both are driven through the
  // same loop because Back needs a page to go back from, and because the
  // browser's own declining shows up in either of them.
  const context = await newContext();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(context) : null;
  const page = await context.newPage();
  let linkRan = false;
  let backCarried = false;
  for (let attempt = 1; attempt <= ATTEMPTS && !(linkRan && backCarried); attempt += 1) {
    await openFresh(page);
    if (attempt === 1) checkProof();
    await followLink(page);
    await settled(page, 3000);
    const linked = await readArrival(page);
    await requireFrames(page, `the page reached by following a link (${linked.href})`);
    if (!linked.reveal) fail('arrival-paints', `the page reached by following a link at ${linked.href} was never revealed`);
    if (linked.transition && linked.ready === 'resolved') linkRan = true;

    try {
      await page.goBack({ waitUntil: 'load', timeout: 10_000 });
    } catch (held) {
      fail('arrival-paints', `the page reached by going back never finished arriving: ${String(held.message).split('\n')[0]}`);
    }
    await settled(page, 3000);
    const back = await readArrival(page);
    if (back.type !== 'traverse') {
      broken(`going back arrived as ${JSON.stringify(back.type)}, want "traverse"; the browser's own history restore is not the path this site drives`);
    }
    await requireFrames(page, `the page reached by going back (${back.href})`);
    if (back.transition) {
      backCarried = true;
      if (back.ready !== 'rejected') {
        fail(
          'back-arrival-is-instant',
          `going back ran a transition at ${back.href} (ready=${back.ready}, finished=${back.finished}); want it skipped, because a reader going back wants the page they left`,
        );
      }
    }
  }
  if (!linkRan) {
    fail(
      'link-arrival-transitions',
      `none of ${ATTEMPTS} link navigations from ${PAGE} ran a view transition; want the page change to carry one for a reader who allows motion`,
    );
  }
  if (!backCarried) {
    broken(`none of ${ATTEMPTS} Back navigations carried a transition to skip, so whether Back is instant cannot be told from a browser that never offers one`);
  }
  await context.close();

  // A reader who asked for reduced motion gets a cut. The blanket that switches
  // animations off for them does not reach the pseudo-elements a transition
  // creates, so the stylesheet's own gate is the whole defence.
  const reduced = await newContext({ reducedMotion: 'reduce' });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(reduced) : null;
  const reducedPage = await reduced.newPage();
  for (let attempt = 1; attempt <= 4; attempt += 1) {
    await openFresh(reducedPage);
    if (attempt === 1) {
      checkProof();
      if (!await reducedPage.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)) {
        broken('the reduced-motion context does not report prefers-reduced-motion: reduce');
      }
    }
    await followLink(reducedPage);
    await settled(reducedPage, 3000);
    const cut = await readArrival(reducedPage);
    await requireFrames(reducedPage, `the page reached by following a link under reduced motion (${cut.href})`);
    if (cut.transition) {
      fail(
        'reduced-motion-is-a-cut',
        `following a link under prefers-reduced-motion: reduce ran a view transition at ${cut.href} (attempt ${attempt}); want an instant cut`,
      );
    }
  }
  await reduced.close();

  // A transition that stalls is ended by the page. The stall is made rather than
  // waited for: the motion is stretched to a thousand seconds, which is what a
  // transition the browser never finishes looks like from inside the document.
  // One that ends in far less than the watchdog's delay was never stalled, and
  // says nothing about the watchdog.
  const stalled = await newContext();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(stalled) : null;
  await stalled.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const stretch = '\n::view-transition-old(root),::view-transition-new(root){animation-duration:1000s!important;animation-delay:0s!important}';
    return route.fulfill({ response, body: original + stretch });
  });
  const stalledPage = await stalled.newPage();
  let judged = false;
  for (let attempt = 1; attempt <= ATTEMPTS && !judged; attempt += 1) {
    await openFresh(stalledPage);
    if (attempt === 1) checkProof();
    await followLink(stalledPage);
    const ended = await settled(stalledPage, WATCHDOG_MS + 2000);
    const held = await readArrival(stalledPage);
    await requireFrames(stalledPage, `the page reached by following a link through a stalled transition (${held.href})`);
    if (!held.transition || held.ready !== 'resolved') continue;
    if (!ended) {
      fail(
        'a-stalled-transition-ends',
        `a transition stretched to 1000s was still running ${WATCHDOG_MS + 2000}ms after the arrival at ${held.href}; want the page to end it after ${WATCHDOG_MS}ms`,
      );
    }
    const took = held.finishedAt - held.revealAt;
    if (took > WATCHDOG_MS + 1000) {
      fail('a-stalled-transition-ends', `a stalled transition ended ${Math.round(took)}ms after the arrival, want about ${WATCHDOG_MS}ms`);
    }
    if (took < WATCHDOG_MS - 200) continue;
    judged = true;
  }
  if (!judged) {
    broken(`none of ${ATTEMPTS} stalled navigations ran a transition that lasted until the watchdog, so whether the page ends one cannot be told`);
  }
  await stalled.close();

  console.log('PASS page-crossfade: a link change ran a view transition, Back skipped it, reduced motion got a cut, a stalled transition was ended by the page, and every arrival painted');
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
