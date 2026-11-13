// Behavior lock for the choice of order on the shelf of what the reader left
// open. Three marks are left in an order that puts the three orders in three
// different places: made oldest to newest in the order beta, alpha, cutover, so
// newest first reads cutover, alpha, beta; oldest first the reverse; and by
// source, which keeps together what came from one note and runs the notes in
// path order, alpha, beta, cutover.
//
// Claims a rendering test cannot reach, because each is settled by following a
// button in a browser: that each choice is a plain GET that really comes back
// with the rows in the order it names — with and without script — that the
// order in force is the one pressed, and that the shelf opens newest first again
// whatever it was last left in, since nothing remembers the choice.
//
// The marks are the probe's own, made on the disposable fixture and taken off
// again before it ends.
//
// Env: YOMIHON_BASE, PAGE_PATH (the shelf) and MUTATE. MUTATE=list prints every
// self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/open-thoughts';
const MUTATE = process.env.MUTATE || '';

const ORDER_SITE = 'each-choice-comes-back-with-the-rows-in-its-order';
const PRESSED_SITE = 'the-order-in-force-is-the-one-pressed';
const OPENING_SITE = 'the-shelf-opens-newest-first-every-time';
const SITES = [ORDER_SITE, PRESSED_SITE, OPENING_SITE];

// Notes the fixture holds, in the order the marks are made.
const MARKED = ['Notes/beta.md', 'Notes/alpha.md', 'Notes/cutover.md'];
const EXPECTED = {
  newest: ['Notes/cutover.md', 'Notes/alpha.md', 'Notes/beta.md'],
  oldest: ['Notes/beta.md', 'Notes/alpha.md', 'Notes/cutover.md'],
  source: ['Notes/alpha.md', 'Notes/beta.md', 'Notes/cutover.md'],
};

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL shelf-order: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN shelf-order: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED shelf-order: ${message}`); };

// Rewrites the shelf's own document and proves, from what was served, that the
// needle was there to rewrite.
const rewriteShelf = (needle, replacement) => (page) => {
  let requests = 0;
  let rewritten = 0;
  return page.route((url) => url.pathname === '/open-thoughts', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    rewritten += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  }).then(() => () => {
    if (requests < 1) return 'the shelf was never requested, so nothing was rewritten';
    if (rewritten < 1) return `the shelf carried no ${JSON.stringify(needle)} to rewrite`;
    return '';
  });
};

// A shelf that remembers the last choice: every visit to the shelf's own
// address is answered as if it had asked for oldest first.
const rememberTheChoice = (page) => {
  let remembered = 0;
  return page.route((url) => url.pathname === '/open-thoughts' && url.search === '', async (route) => {
    remembered += 1;
    await route.continue({ url: `${route.request().url()}?order=oldest` });
  }).then(() => () => (remembered > 0 ? '' : 'the shelf was never visited by its own address, so nothing was remembered'));
};

const MUTATIONS = {
  // The control submits a post, which the shelf does not read.
  'post-the-order-form': { target: ORDER_SITE, apply: rewriteShelf('<form class="y-order" method="get"', '<form class="y-order" method="post"') },
  // The buttons name the order by a word the page does not read.
  'rename-the-order-field': { target: ORDER_SITE, apply: rewriteShelf('name="order"', 'name="sequence"') },
  // Whatever was chosen, no button is marked.
  'remember-the-choice': { target: OPENING_SITE, apply: rememberTheChoice },
  'press-nothing': { target: PRESSED_SITE, apply: rewriteShelf('aria-pressed="true"', 'aria-pressed="false"') },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`shelf-order: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`shelf-order: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`shelf-order: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// The notes the rows lead to, top to bottom, and the word on the pressed button.
const readShelf = (page) => page.evaluate(() => ({
  notes: [...document.querySelectorAll('.y-openitem__name')].map((link) => decodeURIComponent(new URL(link.href).pathname.replace(/^\/notes\//, ''))),
  pressed: [...document.querySelectorAll('.y-order [aria-pressed="true"]')].map((button) => button.value),
}));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const marked = [];
try {
  const setup = await browser.newContext();
  const api = setup.request;
  for (const path of MARKED) {
    const made = await api.post(`${BASE}/uncertainties`, { form: { path, anchor: '' } });
    if (made.status() !== 200) broken(`marking ${path} answered ${made.status()}, want 200`);
    marked.push(path);
    // The marks are dated to the instant they were made; let each be later.
    await new Promise((resolve) => setTimeout(resolve, 20));
  }

  for (const scripted of [true, false]) {
    const context = await browser.newContext({ javaScriptEnabled: scripted });
    const page = await context.newPage();
    const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await arrived(page);
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const at = scripted ? 'with script' : 'without script';
    const first = await readShelf(page);
    if (first.notes.length !== MARKED.length) broken(`the shelf lists ${first.notes.length} rows, want the ${MARKED.length} marks this probe made`);
    if (JSON.stringify(first.notes) !== JSON.stringify(EXPECTED.newest)) {
      fail(OPENING_SITE, `the shelf opens in ${JSON.stringify(first.notes)} ${at}, want newest first ${JSON.stringify(EXPECTED.newest)}`);
    }
    if (JSON.stringify(first.pressed) !== JSON.stringify(['newest'])) {
      fail(PRESSED_SITE, `the shelf opens with ${JSON.stringify(first.pressed)} pressed ${at}, want newest alone`);
    }

    for (const choice of ['oldest', 'source', 'newest']) {
      const asked = page.url();
      await page.locator(`.y-order button[value="${choice}"]`).click();
      await page.waitForLoadState('domcontentloaded');
      await arrived(page);
      if (choice !== 'newest' || asked !== page.url()) {
        if (new URL(page.url()).searchParams.get('order') !== choice) {
          fail(ORDER_SITE, `pressing ${choice} ${at} landed on ${page.url()}, which does not ask for it`);
        }
      }
      const shelf = await readShelf(page);
      if (JSON.stringify(shelf.notes) !== JSON.stringify(EXPECTED[choice])) {
        fail(ORDER_SITE, `pressing ${choice} ${at} lists ${JSON.stringify(shelf.notes)}, want ${JSON.stringify(EXPECTED[choice])}`);
      }
      if (JSON.stringify(shelf.pressed) !== JSON.stringify([choice])) {
        fail(PRESSED_SITE, `after ${choice} ${at} the pressed choice is ${JSON.stringify(shelf.pressed)}, want ${choice} alone`);
      }
    }

    // Nothing remembers the choice: leave it on oldest and come back by the
    // shelf's own address.
    await page.locator('.y-order button[value="oldest"]').click();
    await page.waitForLoadState('domcontentloaded');
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await arrived(page);
    const again = await readShelf(page);
    if (JSON.stringify(again.notes) !== JSON.stringify(EXPECTED.newest) || JSON.stringify(again.pressed) !== JSON.stringify(['newest'])) {
      fail(OPENING_SITE, `coming back to the shelf ${at} after choosing oldest it lists ${JSON.stringify(again.notes)} with ${JSON.stringify(again.pressed)} pressed, want newest first`);
    }
    await context.close();
  }

  await setup.close();
  console.log(`PASS shelf-order: three marks come back newest first, oldest first and by source from plain GET buttons with and without script, the pressed button names the order in force, and the shelf opens newest first every time`);
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
  // Take off the marks this probe made, so no later probe meets a shelf the
  // fixture alone does not explain.
  const cleaner = await browser.newContext();
  for (const path of marked) {
    await cleaner.request.post(`${BASE}/uncertainties`, { form: { path, anchor: '' } });
  }
  await cleaner.close();
  await browser.close();
}
