// Behavior lock: Back returns the reader to the place they left, at every place
// in a long note and at both widths the product is read at, with the page
// reloaded from the browser's cache and with it revived from the back/forward
// cache.
//
// The browser puts the position back against the page's first layout, so the
// place is only the right one if the page it is put back against is the page the
// reader left. Two things were drawn after the first layout and moved
// everything below them: a code block's copy control, which script inserted
// beside the block (the browser names a place by the elements above it, counted
// from the top of their parent, and one more element is a different place), and a
// diagram, which is drawn at the height of its source and only then at its own.
// The note this drives holds both, between sections, and the walk leaves from
// just below each and from two places between them. Each visit starts a new
// browser context: the diagram's height is what the tab remembers of the
// visit that drew it, so a context that carried one over from an earlier
// position would be checking a page nobody has just read.
//
// Back lands at the position the reader left, and it does so once: a restore
// that arrives at the right place by way of another is a page that jumps under
// the reader's eyes.
//
// Env: YOMIHON_BASE, PAGE_PATH (the long note) and MUTATE. MUTATE=list prints
// every watched regression.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Back%20restore.md';
const AWAY = '/notes/Notes/Reading%20flow%20detour.md';
const MUTATE = process.env.MUTATE || '';
const WIDTHS = [1280, 375];
// How far below a code block or a diagram the walk leaves from, so that what it
// was drawn after is above the reader.
const BELOW = 200;
// Subpixel layout lets a restore differ from the position it restores by less
// than a pixel without any element having moved.
const TOLERANCE = 1;

const SITES = ['back-restores-the-place-that-was-left', 'the-back-forward-cache-revives-the-place-that-was-left'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN back-scroll-restore: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL back-scroll-restore: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN back-scroll-restore: ${message}`); };

// Every response a mutation touches is counted, so a needle that matched
// nowhere, or in a response where it should have matched once, ends the run as
// not applied instead of passing over an unchanged page.
const reached = [];
const rewrite = (match, needle, replacement, { first = false } = {}) => async (page) => {
  await page.route(match, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const found = original.split(needle).length - 1;
    reached.push({ needle, found, first });
    const body = first ? original.replace(needle, replacement) : original.replaceAll(needle, replacement);
    await route.fulfill({ response, body });
  });
};
const theNote = (url) => url.pathname === decodeURI(PAGE) || url.pathname === PAGE;
const rewriteTheNote = (needle, replacement, options) => rewrite(theNote, needle, replacement, options);
const rewriteModule = (file, needle, replacement) => rewrite(`**/${file}{,?*}`, needle, replacement);

const MUTATIONS = {
  // The first code block is drawn without its slot, so script has to add its
  // control as one more element beside the block, and every element after it
  // is one place further along than the server drew it.
  'leave-the-first-code-block-without-its-slot': {
    target: 'back-restores-the-place-that-was-left',
    apply: rewriteTheNote('<div class="y-codecopy-slot"></div>', '', { first: true }),
  },
  // The diagram's height is never kept, so the next visit is laid out with the
  // diagram at the height of its source.
  'keep-no-diagram-height': {
    target: 'back-restores-the-place-that-was-left',
    apply: rewriteModule('diagrams.js', '    rememberHeights(blocks);\n', ''),
  },
  // The head never reads what was kept.
  'read-no-diagram-height-in-the-head': {
    target: 'back-restores-the-place-that-was-left',
    apply: rewriteTheNote('sessionStorage.getItem("yomihon.diagramHeights")', 'null'),
  },
  // The stylesheet never holds a diagram at the height the head gave it.
  'hold-no-diagram-at-its-kept-height': {
    target: 'back-restores-the-place-that-was-left',
    apply: rewriteModule('app.css', 'block-size: var(--diagram-height, auto);', 'block-size: auto;'),
  },
  // A page revived from the back/forward cache is put at its top.
  'put-the-revived-page-at-its-top': {
    target: 'the-back-forward-cache-revives-the-place-that-was-left',
    apply: async (page) => {
      await page.route('**/yomihon.js{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        reached.push({ needle: 'yomihon.js', found: 1, first: false });
        await route.fulfill({ response, body: `${original}\naddEventListener('pageshow', (event) => { if (event.persisted) scrollTo(0, 0); });\n` });
      });
    },
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`back-scroll-restore: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`back-scroll-restore: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`back-scroll-restore: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const qualify = () => {
  if (!MUTATE) return;
  const wrong = reached.filter((r) => (r.first ? r.found < 1 : r.found !== 1));
  if (reached.length === 0 || wrong.length > 0) {
    throw new NotApplied(`NOT-APPLIED back-scroll-restore: ${MUTATE} matched [${reached.map((r) => r.found)}] across ${reached.length} responses; want one site in each`);
  }
};

// The page is as the reader will see it once its code controls are in place and
// every diagram is drawn, and a short while after.
const settle = (page) => page.evaluate(async () => {
  await document.fonts.ready;
  const blocks = [...document.querySelectorAll('.mermaid-diagram')];
  const until = performance.now() + 8000;
  while (blocks.some((block) => !block.querySelector('svg')) && performance.now() < until) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  await new Promise((resolve) => setTimeout(resolve, 300));
  return blocks.length > 0 && blocks.every((block) => block.querySelector('svg'));
});

// What the reader's screen shows of the scroll position, from the start of the
// document: every distinct value the page was at, in the order it was there.
const watchPosition = (page) => page.addInitScript(() => {
  const seen = [];
  window.__positions = seen;
  window.__persisted = null;
  const note = () => {
    const y = Math.round(window.scrollY);
    if (seen[seen.length - 1] !== y) seen.push(y);
  };
  addEventListener('scroll', note);
  addEventListener('pageshow', (event) => { window.__persisted = event.persisted; });
  const frame = () => { note(); requestAnimationFrame(frame); };
  requestAnimationFrame(frame);
});

const placesToLeaveFrom = (page) => page.evaluate((below) => {
  const room = document.documentElement.scrollHeight - innerHeight;
  const landmarks = [...document.querySelectorAll('.y-prose pre, .y-prose .mermaid-diagram')]
    .map((element) => Math.round(element.getBoundingClientRect().bottom + scrollY + below));
  const places = [...landmarks, Math.round(room / 3), Math.round((2 * room) / 3)]
    .filter((y) => y > 0 && y < room);
  return { places: [...new Set(places)].sort((a, b) => a - b), landmarks: landmarks.length, room };
}, BELOW);

const browsers = [];
const open = async (cache) => {
  const browser = await chromium.launch({
    channel: 'chrome',
    headless: true,
    ...(cache ? { ignoreDefaultArgs: ['--disable-back-forward-cache'] } : {}),
  });
  browsers.push(browser);
  return browser;
};

// A new context with the note open at a width, every diagram drawn.
const arrive = async (browser, width) => {
  const context = await browser.newContext({ viewport: { width, height: 800 } });
  const page = await context.newPage();
  if (MUTATE) await MUTATIONS[MUTATE].apply(page);
  await watchPosition(page);
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!(await settle(page))) broken(`the diagrams of ${PAGE} were not drawn at ${width}px, so the page the reader leaves is not the page this walk means`);
  return { context, page };
};

const survey = async (browser, width) => {
  const { context, page } = await arrive(browser, width);
  try {
    return await placesToLeaveFrom(page);
  } finally {
    await context.close();
  }
};

// One walk: the reader at a place of the note, a page that is not the note, and
// Back.
const walk = async (browser, width, place) => {
  const { context, page } = await arrive(browser, width);
  try {
    await page.evaluate((y) => window.scrollTo(0, y), place);
    await page.waitForTimeout(250);
    const left = await page.evaluate(() => Math.round(window.scrollY));
    if (Math.abs(left - place) > TOLERANCE) broken(`the reader could not be put at ${place}px of ${PAGE} at ${width}px; the page is at ${left}px`);
    await page.evaluate(() => { window.__persisted = null; });
    await page.goto(BASE + AWAY, { waitUntil: 'load' });
    // A page revived from the back/forward cache is not loaded again, so there
    // is no load to wait for; it says it has arrived by being shown.
    await page.goBack({ waitUntil: 'commit' });
    await page.waitForFunction(() => window.__persisted !== null);
    await settle(page);
    return {
      left,
      back: await page.evaluate(() => Math.round(window.scrollY)),
      url: page.url(),
      positions: await page.evaluate(() => window.__positions),
      persisted: await page.evaluate(() => window.__persisted),
    };
  } finally {
    await context.close();
  }
};

try {
  const cached = await open(false);
  let walked = 0;
  for (const width of WIDTHS) {
    const { places, landmarks, room } = await survey(cached, width);
    if (landmarks < 4) broken(`${PAGE} at ${width}px holds ${landmarks} code blocks and diagrams, want at least 4 to leave from below`);
    if (places.length < 6) broken(`${PAGE} at ${width}px offers ${places.length} places to leave from in ${room}px, want at least 6`);
    for (const place of places) {
      const outcome = await walk(cached, width, place);
      walked += 1;
      if (outcome.url !== BASE + PAGE) broken(`Back at ${width}px ended at ${outcome.url}, not at the note`);
      if (outcome.persisted !== false) broken(`Back at ${width}px was not a reload from the browser's cache; the note's pageshow said persisted=${outcome.persisted}`);
      if (Math.abs(outcome.back - outcome.left) > TOLERANCE) {
        fail('back-restores-the-place-that-was-left', `${width}px: the reader left at ${outcome.left}px and Back put them at ${outcome.back}px (${outcome.back - outcome.left > 0 ? '+' : ''}${outcome.back - outcome.left}px)`);
      }
      const visited = outcome.positions.filter((y) => y > 0);
      if (visited.some((y) => Math.abs(y - outcome.left) > TOLERANCE)) {
        fail('back-restores-the-place-that-was-left', `${width}px: the reader left at ${outcome.left}px and the page went by way of ${JSON.stringify(visited)} before it stopped`);
      }
    }
  }

  const revived = await open(true);
  for (const width of WIDTHS) {
    const { places } = await survey(revived, width);
    for (const place of [places[0], places[places.length - 1]]) {
      const outcome = await walk(revived, width, place);
      if (outcome.persisted !== true) broken(`Back at ${width}px did not revive the page from the back/forward cache (persisted=${outcome.persisted}), so this walk is not the one it says`);
      if (Math.abs(outcome.back - outcome.left) > TOLERANCE) {
        fail('the-back-forward-cache-revives-the-place-that-was-left', `${width}px: the reader left at ${outcome.left}px and the revived page stands at ${outcome.back}px`);
      }
    }
  }
  qualify();

  console.log(`PASS back-scroll-restore: Back put the reader at the place they left on all ${walked} walks from below the code blocks and diagrams of ${PAGE}, at ${WIDTHS.join('px and ')}px, going to no other place on the way, and a page revived from the back/forward cache stood where it was left`);
} catch (error) {
  console.error(error.message);
  if (error instanceof ProbeBroken || !MUTATE) {
    process.exitCode = 1;
  } else {
    let applied = true;
    try {
      qualify();
    } catch (notApplied) {
      applied = false;
      console.error(notApplied.message);
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
    }
    if (applied) {
      process.exitCode = 1;
      if (error instanceof LockFired && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
  }
} finally {
  await Promise.all(browsers.map((browser) => browser.close()));
}
