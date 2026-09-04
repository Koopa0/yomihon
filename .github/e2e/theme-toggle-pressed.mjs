// Behavior lock for the theme control's pressed state. The server renders it
// from the stored choice, which is all a server can see: a system that prefers
// dark never reaches it. So a reader who has chosen nothing, on a dark system,
// is shown a dark page by a button claiming it is not pressed — and the first
// press then reads as switching to the thing already on screen.
//
// WAI-ARIA says aria-pressed reports whether the toggle is currently pressed,
// so on that page it has to be true. The client is the only side that can know.
//
// What this does not cover: the same answer is recomputed after a
// back/forward-cache restore, and driving a real bfcache restore from a headless
// browser here did not produce one — a goBack that reloads instead re-runs the
// load path, so an assertion there would have been reading the load-time answer
// and calling it the restore's. Measured: with the restore branch put back to
// reading the stored choice alone, that assertion still passed. It is left out
// rather than shipped as a lock that cannot fail; the restore branch is held
// only by the code being the same expression as the one below.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['pressed-matches-painted-theme', 'not-pressed-on-a-light-page'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN theme-toggle-pressed: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL theme-toggle-pressed: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN theme-toggle-pressed: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED theme-toggle-pressed: ${message}`); };

// Rewrites the served preferences module, which is where the answer is
// computed. Both mutations put back a reading of the stored choice alone.
const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/preferences.js', async (route) => {
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
  'always-pressed': {
    target: 'not-pressed-on-a-light-page',
    apply: rewriteModule(
      "themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));\n\n  textsizeToggle",
      "themeToggle?.setAttribute('aria-pressed', String(true));\n\n  textsizeToggle",
      'first-paint pressed state, hardcoded',
    ),
  },
  'load-reads-stored-choice-only': {
    target: 'pressed-matches-painted-theme',
    apply: rewriteModule(
      "themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));\n\n  textsizeToggle",
      "themeToggle?.setAttribute('aria-pressed', String(root.dataset.theme === 'dark'));\n\n  textsizeToggle",
      'first-paint pressed state',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`theme-toggle-pressed: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`theme-toggle-pressed: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`theme-toggle-pressed: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const pressedState = (page) =>
  page.evaluate(() => {
    const toggle = document.querySelector('[data-theme-toggle]');
    if (!toggle) return null;
    return {
      pressed: toggle.getAttribute('aria-pressed'),
      painted: getComputedStyle(document.documentElement).colorScheme,
      stamped: document.documentElement.dataset.theme ?? '',
    };
  });

// The system prefers dark and nothing is stored, which is the only combination
// where the server's answer and the reader's page can disagree.
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ colorScheme: 'dark', viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const onLoad = await pressedState(page);
  if (onLoad === null) broken('the page carries no theme control to read');
  if (onLoad.stamped !== '') {
    broken(`the root already carries a stored theme (${onLoad.stamped}); this probe needs a reader who has chosen nothing`);
  }
  if (!onLoad.painted.includes('dark')) {
    broken(`the page painted ${onLoad.painted} under a dark system preference, so there is no disagreement to detect`);
  }
  if (onLoad.pressed !== 'true') {
    fail('pressed-matches-painted-theme', `the page is dark and the control reports aria-pressed=${onLoad.pressed}, want true`);
  }

  // The other direction, on its own page: an answer hardcoded to "pressed"
  // satisfies the assertion above and is wrong for every reader whose system
  // asks for light, which is the default this interface is written around.
  const lightContext = await browser.newContext({ colorScheme: 'light', viewport: { width: 1280, height: 800 } });
  const lightPage = await lightContext.newPage();
  if (MUTATE) await MUTATIONS[MUTATE].apply(lightPage);
  await lightPage.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  const onLight = await pressedState(lightPage);
  if (onLight === null) broken('the light page carries no theme control to read');
  if (onLight.painted.includes('dark')) {
    broken(`the page painted ${onLight.painted} under a light system preference, so the two directions are not separated`);
  }
  if (onLight.pressed !== 'false') {
    fail('not-pressed-on-a-light-page', `the page is light and the control reports aria-pressed=${onLight.pressed}, want false`);
  }

  console.log('PASS theme-toggle-pressed: the control reports pressed on a dark page nobody chose, and not pressed on a light one');
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
