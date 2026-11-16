// Behavior lock for the kept place on a page that is shown again.
//
// The control that keeps a place, and the ribbon beside the section that holds
// it, are drawn by the server from the file the place is kept in. A page the
// browser shows again from its HTTP cache is the page of the visit before, so
// what it draws is what the file said then: a place kept after that visit is
// missing from it, and a place that has since moved to another note is still
// drawn in it. The module has to ask the server where the place is now and draw
// that, as the page does for the uncertainty marks, and has to ask again when a
// page comes back from the back/forward cache, which runs no script of its own.
//
// Going back is real: the previous document is handed back by Chrome's HTTP
// cache, which a page-level route would switch off, so a mutation is planted in
// the module through the protocol's own request interception, scoped to the
// module alone. Each scenario first reads what the cached document drew when
// it was parsed, before the module could have answered, and stops as broken
// when that is not the stale picture the scenario is about.
//
// The back/forward cache is not driven for real: Chrome refuses one under this
// automation, and the branch is the same handler a dispatched
// PageTransitionEvent({ persisted: true }) reaches (see preference-restore.mjs).
//
// It keeps places of its own on the disposable fixture, so it is one of the
// probes that leave one behind and has to stand among the last (see probes.sh).
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const NOTE = '/notes/Notes/Reading%20flow.md';
const OTHER = '/notes/Notes/alpha.md';
const OTHER_REL = 'Notes/alpha.md';

// Where the place is kept, per width: below the second section's heading, so
// the ribbon is drawn beside a section and not above the first one.
const VIEWS = [
  { width: 1280, height: 800, y: 1100 },
  { width: 375, height: 800, y: 1700 },
];
const DEADLINE = 3000;

const SITES = [
  'the-control-says-the-place-is-kept-after-going-back',
  'the-ribbon-is-drawn-after-going-back',
  'a-place-kept-elsewhere-is-taken-down-after-going-back',
  'a-persisted-show-reads-the-place-again',
  'an-older-answer-does-not-undo-a-press',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN kept-place-after-back: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL kept-place-after-back: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN kept-place-after-back: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED kept-place-after-back: ${message}`); };

// Each mutation removes one decision from the module and names the assertion
// that is there to notice. The needles are the module's own statements, and each
// has to match exactly once in every copy served (see rewriteModule below).
const MUTATIONS = {
  // The module never asks where the place is when a page loads.
  'never-ask-where-the-place-is': {
    target: SITES[0],
    needle: '  land();\n  recall();\n',
    replacement: '  land();\n',
    label: 'the reading of where the place is when a page loads',
  },
  // The control is corrected and the ribbon beside the section is left as the
  // document drew it.
  'leave-the-ribbon-out': {
    target: SITES[1],
    needle: "  showPlace(here ? held.anchor : '');\n",
    replacement: '',
    label: 'the drawing of the ribbon from the answer',
  },
  // A place the server holds in this note is drawn; one it holds elsewhere is
  // left standing.
  'never-take-the-place-down': {
    target: SITES[2],
    needle: '  const here = held !== null && held.path === face.dataset.markPath;\n',
    replacement: '  const here = held !== null && held.path === face.dataset.markPath;\n  if (!here) return;\n',
    label: 'the taking down of a place that is not in this note',
  },
  // Any place the server holds is taken to be this note's.
  'take-any-place-for-this-notes': {
    target: SITES[2],
    needle: 'held !== null && held.path === face.dataset.markPath',
    replacement: 'held !== null',
    label: 'the comparison of the place with the note the page shows',
  },
  // A page restored from the back/forward cache is not asked again.
  'ignore-a-persisted-show': {
    target: SITES[3],
    needle: 'if (event.persisted) recall();',
    replacement: 'if (event.persisted) return;',
    label: 'the reading of where the place is when a page is restored',
  },
  // An answer that began before a press is drawn after it.
  'draw-an-answer-older-than-a-press': {
    target: SITES[4],
    needle: '  if (presses !== asked) return;\n',
    replacement: '',
    label: 'the guard on an answer that began before a press',
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`kept-place-after-back: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`kept-place-after-back: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`kept-place-after-back: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What a document draws of the place kept, read from the page itself: whether
// each face of the control says the place is here, and the section each drawn
// ribbon stands beside. The same reader runs at parse time, before the module
// can have answered, and at any later moment.
const drawn = () => {
  const faces = [...document.querySelectorAll('[data-mark-control]')];
  return {
    faces: faces.length,
    pressed: faces.map((face) => face.querySelector('[data-mark-button]')?.getAttribute('aria-pressed') ?? null),
    kept: faces.map((face) => face.hasAttribute('data-mark-kept')),
    ribbons: [...document.querySelectorAll('.y-article [data-section-place]')]
      .filter((label) => !label.hidden)
      .map((label) => label.closest('.y-sect')?.firstElementChild?.id ?? ''),
  };
};
const recorder = () => {
  if (window.top !== window) return;
  document.addEventListener('DOMContentLoaded', () => {
    window.__parsed = window.__drawn();
  });
};

const says = (state, kept) => state.faces > 0
  && state.pressed.every((value) => value === String(kept))
  && state.kept.every((value) => value === kept);

const browser = await chromium.launch({ channel: 'chrome', headless: true });

// rewriteModule plants a regression in the module through the protocol's request
// interception, which switches nothing else off: a page-level route disables
// the HTTP cache, and with it the restore this probe exists to drive. The
// request is intercepted before the cache is asked, so a module the cache
// would have handed back is rewritten too. The needle has to match exactly once
// in every copy served.
const rewriteModule = ({ needle, replacement, label }) => async (page) => {
  const session = await page.context().newCDPSession(page);
  const perLoad = [];
  await session.send('Fetch.enable', { patterns: [{ urlPattern: '*/static/mark.js*', requestStage: 'Request' }] });
  session.on('Fetch.requestPaused', async ({ requestId, request }) => {
    const source = await (await fetch(request.url)).text();
    perLoad.push(source.split(needle).length - 1);
    await session.send('Fetch.fulfillRequest', {
      requestId,
      responseCode: 200,
      responseHeaders: [{ name: 'Content-Type', value: 'text/javascript; charset=utf-8' }],
      body: Buffer.from(source.replace(needle, replacement)).toString('base64'),
    });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the module was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

const keepHere = async (page, y) => {
  await page.evaluate((to) => window.scrollTo(0, to), y);
  const control = page.locator('[data-mark-control]:visible');
  if (await control.count() !== 1) broken(`${page.url()} shows ${await control.count()} mark controls, want exactly 1`);
  const posted = page.waitForResponse((response) => new URL(response.url()).pathname === '/marks' && response.request().method() === 'POST', { timeout: 4000 });
  await control.locator('[data-mark-button]').click();
  const response = await posted;
  if (response.status() !== 204) broken(`keeping a place in ${page.url()} answered ${response.status()}, want 204`);
  await control.locator('[data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
  return new URLSearchParams(response.request().postData() ?? '');
};

const opened = async (page, path) => {
  await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
};

// until waits for the page to come to a state, and answers whether it did within
// the deadline: a page that never does is the failure the caller reports, in
// words about the reader, and not a timeout from here.
const until = (page, predicate, arg) => page.waitForFunction(predicate, arg, { timeout: DEADLINE }).then(() => true, () => false);

try {
  console.log(`kept-place-after-back: Chrome ${browser.version()}`);
  let checked = 0;
  for (const view of VIEWS) {
    const context = await browser.newContext({ viewport: { width: view.width, height: view.height }, reducedMotion: 'no-preference' });
    await context.addInitScript(`window.__drawn = ${drawn.toString()};`);
    await context.addInitScript(recorder);
    const elsewhere = await context.request.post(`${BASE}/marks`, {
      form: { path: OTHER_REL, anchor: '', offset: '0', identity: '0'.repeat(64) },
    });
    if (elsewhere.status() !== 204) broken(`keeping a place in ${OTHER_REL} answered ${elsewhere.status()}, want 204`);

    const page = await context.newPage();
    const proof = MUTATE ? await rewriteModule(MUTATIONS[MUTATE])(page) : null;
    const checkProof = () => {
      if (!proof) return;
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    };

    // 1. A place kept after the page was fetched. The page is fetched while the
    // place is in another note, so its document draws none; the reader keeps one
    // in it, leaves, and goes back.
    await opened(page, NOTE);
    const before = await page.evaluate(() => window.__drawn());
    if (!says(before, false) || before.ribbons.length !== 0) {
      fail(SITES[2], `at ${view.width}px a page loaded while the place is kept in another note draws ${JSON.stringify(before)}, want no place`);
    }
    const posted = await keepHere(page, view.y);
    const anchor = posted.get('anchor');
    const here = await page.evaluate(() => window.__drawn());
    if (!says(here, true) || here.ribbons.length !== 1) broken(`pressing the control at ${view.width}px left ${JSON.stringify(here)}, want the place kept and one ribbon (anchor ${JSON.stringify(anchor)})`);
    await page.goto(BASE + OTHER, { waitUntil: 'load' });
    await page.goBack({ waitUntil: 'load' });
    const stale = await page.evaluate(() => window.__parsed);
    if (!says(stale, false) || stale.ribbons.length !== 0) {
      broken(`going back to ${NOTE} at ${view.width}px handed over a document that drew ${JSON.stringify(stale)} when parsed, want the picture of the visit before the place was kept`);
    }
    await until(page, () => window.__drawn().kept.every(Boolean), null);
    const back = await page.evaluate(() => window.__drawn());
    checkProof();
    if (!says(back, true)) {
      fail(SITES[0], `at ${view.width}px a page shown again from the cache after the place was kept in it draws ${JSON.stringify(back)}, want every face saying the place is kept`);
    }
    if (back.ribbons.length !== 1 || back.ribbons[0] !== here.ribbons[0]) {
      fail(SITES[1], `at ${view.width}px a page shown again from the cache draws the ribbon beside ${JSON.stringify(back.ribbons)}, want beside ${JSON.stringify(here.ribbons)}, where it was when the place was kept`);
    }
    checked += 1;

    // 2. A place kept elsewhere since the page was fetched. The document is
    // fetched while the place is in it, so it draws the place; the reader keeps
    // one in another note and goes back.
    await opened(page, NOTE);
    const fresh = await page.evaluate(() => window.__drawn());
    if (!says(fresh, true) || fresh.ribbons.length !== 1) broken(`${NOTE} drew ${JSON.stringify(fresh)} with the place kept in it, want the place and one ribbon`);
    await page.goto(BASE + OTHER, { waitUntil: 'load' });
    await arrived(page);
    await keepHere(page, 0);
    await page.goBack({ waitUntil: 'load' });
    const kept = await page.evaluate(() => window.__parsed);
    if (!says(kept, true) || kept.ribbons.length !== 1) {
      broken(`going back to ${NOTE} at ${view.width}px handed over a document that drew ${JSON.stringify(kept)} when parsed, want the picture of the visit before the place moved`);
    }
    await until(page, () => window.__drawn().kept.every((value) => !value), null);
    const moved = await page.evaluate(() => window.__drawn());
    if (!says(moved, false) || moved.ribbons.length !== 0) {
      fail(SITES[2], `at ${view.width}px a page shown again from the cache after the place was kept in another note draws ${JSON.stringify(moved)}, want no place`);
    }
    checked += 1;

    // 3. A page that comes back from the back/forward cache runs nothing of its
    // own, so it is told by the event the browser sends, and the place is read
    // again then. The place moves behind the page's back and returns the same way.
    await opened(page, NOTE);
    const live = await page.evaluate(() => window.__drawn());
    if (!says(live, false) || live.ribbons.length !== 0) {
      fail(SITES[2], `at ${view.width}px a page loaded while the place is kept in another note draws ${JSON.stringify(live)}, want no place`);
    }
    const behind = await context.request.post(`${BASE}/marks`, {
      form: { path: decodeURIComponent(NOTE.slice('/notes/'.length)), anchor, offset: '1', identity: '0'.repeat(64) },
    });
    if (behind.status() !== 204) broken(`keeping a place in ${NOTE} answered ${behind.status()}, want 204`);
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
    const drawnAgain = await until(page, () => window.__drawn().kept.every(Boolean), null);
    const restored = await page.evaluate(() => window.__drawn());
    if (!drawnAgain || !says(restored, true) || restored.ribbons.length !== 1) {
      fail(SITES[3], `at ${view.width}px a page restored from the back/forward cache after the place was kept in it draws ${JSON.stringify(restored)}, want the place`);
    }
    await context.request.post(`${BASE}/marks`, {
      form: { path: OTHER_REL, anchor: '', offset: '0', identity: '0'.repeat(64) },
    });
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
    const takenDown = await until(page, () => window.__drawn().kept.every((value) => !value), null);
    const gone = await page.evaluate(() => window.__drawn());
    if (!takenDown || !says(gone, false) || gone.ribbons.length !== 0) {
      fail(SITES[3], `at ${view.width}px a page restored from the back/forward cache after the place was kept elsewhere draws ${JSON.stringify(gone)}, want no place`);
    }
    checked += 1;

    // 4. An answer that began before a press is older than the press. The answer
    // to a page's first question is held back, the reader keeps a place, and the
    // held answer, which says the place is nowhere, is let through.
    await page.addInitScript(() => {
      if (window.top !== window) return;
      const real = window.fetch.bind(window);
      window.fetch = (input, init) => {
        const url = typeof input === 'string' ? input : input.url;
        if (new URL(url, location.href).pathname === '/marks' && !init?.method) {
          return new Promise((resolve) => {
            window.__letThrough = () => resolve(new Response(null, { status: 204 }));
          });
        }
        return real(input, init);
      };
    });
    await opened(page, NOTE);
    const waiting = await page.evaluate(() => typeof window.__letThrough);
    if (waiting !== 'function') broken(`${NOTE} asked nothing of the server when it loaded, so there is no answer to hold back`);
    await keepHere(page, view.y);
    await page.evaluate(() => window.__letThrough());
    await page.evaluate(() => new Promise((resolve) => { requestAnimationFrame(() => requestAnimationFrame(resolve)); }));
    const after = await page.evaluate(() => window.__drawn());
    if (!says(after, true) || after.ribbons.length !== 1) {
      fail(SITES[4], `at ${view.width}px an answer that began before the place was kept took it down again: the page draws ${JSON.stringify(after)}, want the place`);
    }
    checked += 1;
    checkProof();
    await context.close();
  }
  console.log(`PASS kept-place-after-back: ${checked} scenarios, a page shown again from either cache draws the place kept now`);
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
