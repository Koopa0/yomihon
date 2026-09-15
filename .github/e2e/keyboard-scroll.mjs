// Behavior lock: a keyboard alone moves the prose. A reader arrives with focus
// on the body, and the browser scrolls the focused element's scrolling box — so
// while the reading column owned its own overflow, the viewport had nothing to
// move and space, page-down and the arrows did nothing at all. The reader had
// to know to press Tab first.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH, and MUTATE.
// MUTATE=list prints every self-test mode.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

const SITES = ['document-has-somewhere-to-scroll', 'space-moves-the-prose'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN keyboard-scroll: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL keyboard-scroll: ${message}`);
};

// Restoring the reading column's own overflow is the regression this locks: the
// document loses its slack, and a key pressed with focus on the body has
// nothing to move.
const MUTATIONS = {
  // A handler that takes the Space key and forgets to let the page have it:
  // the shortcut reads the key, calls preventDefault, and the reader who moves
  // by keyboard alone is stranded. It aims at the key rather than at the slack,
  // because the document is left exactly as long as it was -- only the key
  // stops working, which is the half a scroll-slack mutation cannot reach.
  'swallow-the-space-key': {
    target: 'space-moves-the-prose',
    before: async (page) => {
      await page.evaluate(() => {
        addEventListener('keydown', (event) => {
          if (event.code === 'Space') event.preventDefault();
        }, true);
      });
      // Dispatching one proves the handler is live, rather than proving only
      // that the script which installs it ran.
      const intercepted = await page.evaluate(() => {
        const probe = new KeyboardEvent('keydown', { code: 'Space', cancelable: true, bubbles: true });
        document.body.dispatchEvent(probe);
        return probe.defaultPrevented;
      });
      return () => (intercepted ? '' : 'the swallowing handler never intercepted a Space key');
    },
  },
  'reading-column-owns-the-scroll': {
    target: 'document-has-somewhere-to-scroll',
    before: async (page) => {
      await page.addStyleTag({
        content: '.y-main { overflow-y: auto; max-height: calc(100vh - 56px); } html, body { overflow: hidden; }',
      });
      const applied = await page.evaluate(() =>
        getComputedStyle(document.querySelector('.y-main')).overflowY === 'auto');
      return () => (applied ? '' : 'the reading column did not take back its overflow');
    },
  },
};

// A mutation aimed at a site that does not exist never runs, and an assertion
// no mutation aims at is a lock nothing has ever watched fail. Both are silent
// while the suite stays green, so they are refused here instead.
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`keyboard-scroll: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`keyboard-scroll: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}
if (MUTATE && !MUTATIONS[MUTATE]) {
  console.error(`BROKEN keyboard-scroll: unknown mutation mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let fired = null;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  await page.goto(BASE + PAGE, { waitUntil: 'load' });

  let verify = () => '';
  if (MUTATE) verify = await MUTATIONS[MUTATE].before(page);
  const issue = verify();
  if (issue) throw new NotApplied(`NOT-APPLIED keyboard-scroll: ${issue}`);

  // Smooth scrolling animates, so a reading taken immediately after the key
  // would be zero however well this works — the confound that silently emptied
  // four earlier attempts at measuring this.
  await page.evaluate(() => { document.documentElement.style.scrollBehavior = 'auto'; });

  const slack = await page.evaluate(() =>
    document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight);
  if (slack <= 0) {
    fail('document-has-somewhere-to-scroll',
      `the document has ${slack} of slack, so no key could move it; the page under test must be longer than the viewport`);
  }

  const active = await page.evaluate(() => document.activeElement?.tagName ?? 'none');
  if (active !== 'BODY') {
    throw new ProbeBroken(`BROKEN keyboard-scroll: focus starts on ${active}, so this does not test an ordinary arrival`);
  }

  const before = await page.evaluate(() => document.scrollingElement.scrollTop);
  await page.keyboard.press('Space');
  // A key scroll is animated and lands whenever the compositor gets to it, so
  // the probe waits for the movement itself rather than a fixed slice of time;
  // a slow runner is not a page that failed to move, but a page that never
  // moves still fails at the deadline below.
  await page.waitForFunction((from) => document.scrollingElement.scrollTop > from, before, { timeout: 2000 }).catch(() => {});
  const after = await page.evaluate(() => document.scrollingElement.scrollTop);
  if (after <= before) {
    fail('space-moves-the-prose',
      `the document stayed at ${after} after Space; a reader with only a keyboard cannot move the page`);
  }

  console.log('PASS keyboard-scroll: the document scrolls and Space moves it on arrival');
  await context.close();
} catch (error) {
  if (error instanceof LockFired) { fired = error; console.error(error.message); }
  else if (error instanceof NotApplied) { console.error(error.message); process.exitCode = 2; }
  else if (error instanceof ProbeBroken) { console.error(error.message); process.exitCode = 2; }
  else throw error;
} finally {
  await browser.close();
}

if (fired) {
  if (MUTATE && MUTATIONS[MUTATE].target === fired.site) {
    console.log(`MUTATE-RESULT: caught ${MUTATE}`);
  }
  process.exitCode = 1;
} else if (MUTATE) {
  console.error(`FAIL keyboard-scroll: mutation ${MUTATE} left every lock green`);
  process.exitCode = 1;
}
