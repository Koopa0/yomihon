// Behavior lock: switching the chrome's language mid-note returns the reader
// to the place they were reading. The form posts a path; without a position
// on that address the reload lands at the top of a long note.
//
// The destination note is long on purpose. On a note that fits in one screen,
// "the reader came back where they were" is already true before anything
// scrolls, so the arrival check could not have failed.
//
// Env: YOMIHON_BASE, PAGE_PATH (Glass Tide), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['position-survives-switch'];
const TARGET_Y = 600;
const SLACK_FLOOR = 700;
const TOLERANCE = 48;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN language-scroll-restore: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL language-scroll-restore: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN language-scroll-restore: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED language-scroll-restore: ${message}`); };

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/langform.js', async (route) => {
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

const MUTATIONS = {
  // The defect itself: next stays the path alone, so the redirect has no
  // position to restore and the reader arrives at the top.
  'leave-next-as-the-path': {
    target: 'position-survives-switch',
    apply: rewriteModule(
      "  next.value = path + '#' + MARK + y;",
      '  next.value = path;',
      'language-form next rewrite',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`language-scroll-restore: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`language-scroll-restore: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`language-scroll-restore: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const waitSettled = (page) => page.evaluate(() => new Promise((resolve) => {
  let settled = false;
  const finish = () => {
    if (settled) return;
    settled = true;
    resolve();
  };
  setTimeout(finish, 600);
  const afterPaint = () => requestAnimationFrame(() => requestAnimationFrame(finish));
  afterPaint();
  window.addEventListener('pagereveal', (event) => {
    if (event.viewTransition?.finished) {
      event.viewTransition.finished.then(afterPaint, afterPaint);
      return;
    }
    afterPaint();
  }, { once: true });
}));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!response || response.status() !== 200) {
    broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  }
  await page.waitForSelector('html[data-js]');
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const buttons = await page.locator('.y-langform .y-langbtn').count();
  if (buttons !== 1) broken(`the page carries ${buttons} language controls, want exactly 1`);

  const slack = await page.evaluate(() =>
    document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight);
  if (slack < SLACK_FLOOR) {
    broken(`the document has ${slack}px of slack, want at least ${SLACK_FLOOR} so a mid-note switch has somewhere to fall from`);
  }

  await page.evaluate((y) => { window.scrollTo(0, y); }, TARGET_Y);
  const before = await page.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
  }));
  if (before.y < TARGET_Y - 8) {
    broken(`scrolled to ${before.y}, want near ${TARGET_Y}; the page did not travel`);
  }

  await page.locator('.y-langform .y-langbtn').click();
  const switched = await page.waitForFunction(
    (from) => document.documentElement.getAttribute('lang') !== from,
    before.lang,
    { timeout: 10_000 },
  ).then(() => true, () => false);
  if (!switched) {
    broken(`the language stayed ${JSON.stringify(before.lang)} after clicking the language control`);
  }
  await page.waitForSelector('html[data-js]');
  await waitSettled(page);

  const after = await page.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
  }));
  if (after.lang === before.lang) {
    broken(`the language stayed ${JSON.stringify(after.lang)} after the switch, so this run never left the page`);
  }
  if (Math.abs(after.y - before.y) > TOLERANCE) {
    fail(
      'position-survives-switch',
      `after switching ${before.lang} → ${after.lang} the page is at scrollY=${after.y}, want near ${before.y} (within ${TOLERANCE}px)`,
    );
  }

  console.log(`PASS language-scroll-restore: a mid-note language switch (${before.lang} → ${after.lang}) returned at scrollY=${after.y} from ${before.y}`);
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
