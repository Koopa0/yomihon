// Behavior lock for the appearance choice on the settings page. The server
// marks the option in force from the stored choice, which is all a server can
// see: a system that prefers dark never reaches it. So the option a reader who
// has chosen nothing is shown is "follow the system", whichever way the system
// leans — and the page has to be painted the way the system leans while it
// says so.
//
// The choice is kept the moment it is picked, so picking it has to change the
// page at once. A choice of light or dark holds against the system moving
// under it; following the system follows it.
//
// What this does not cover: the same answer is recomputed after a
// back/forward-cache restore, which preference-restore.mjs drives.
//
// Env: YOMIHON_BASE, PAGE_PATH (the settings page), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/preferences';
const MUTATE = process.env.MUTATE || '';
const FIELD = '[data-pref-field="theme"]';
const SITES = [
  'nothing-stored-shows-the-system',
  'a-choice-paints-at-once',
  'following-the-system-follows-it',
  'a-stored-choice-holds-against-the-system',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN appearance-choice: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL appearance-choice: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN appearance-choice: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED appearance-choice: ${message}`); };

// Rewrites one served resource, counting the needle in every response so a
// mutation that found nothing to change is reported as such.
const rewrite = (glob, needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route(glob, async (route) => {
    if (route.request().resourceType() === 'document' && route.request().method() !== 'GET') return route.continue();
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: nothing was served to rewrite`;
    if (perLoad.some((hits) => hits !== 1)) return `${label} needle matched [${perLoad.join(', ')}], want exactly 1 in each`;
    return '';
  };
};

const MUTATIONS = {
  // The server stops marking following the system as the option in force for
  // a reader who chose nothing, so the page claims no answer at all.
  'mark-nothing-for-nothing-stored': {
    target: 'nothing-stored-shows-the-system',
    apply: rewrite(
      `**${PAGE}`,
      'name="theme" value="system" checked',
      'name="theme" value="system"',
      'the option in force for nothing stored',
    ),
  },
  'keep-the-choice-without-painting-it': {
    target: 'a-choice-paints-at-once',
    apply: rewrite('**/preferences.js{,?*}', '        writeTheme(value, true);\n', '', 'the write of a picked theme'),
  },
  'leave-the-old-choice-on-the-page': {
    target: 'following-the-system-follows-it',
    apply: rewrite('**/preferences.js{,?*}', '      writeTheme(null, false);\n', '', 'the removal of a stored theme'),
  },
  'let-the-system-override-a-choice': {
    target: 'a-stored-choice-holds-against-the-system',
    apply: rewrite(
      '**/app.css{,?*}',
      "@media (prefers-color-scheme: dark) {\n  :root:not([data-theme='light']) {",
      '@media (prefers-color-scheme: dark) {\n  :root {',
      'the dark block a stored light choice is kept out of',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`appearance-choice: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`appearance-choice: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`appearance-choice: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const appearance = (page) =>
  page.evaluate((field) => ({
    shown: document.querySelector(`${field} input:checked`)?.value ?? null,
    painted: getComputedStyle(document.documentElement).colorScheme,
    stamped: document.documentElement.dataset.theme ?? '',
  }), FIELD);

// Opens the settings page in a browser whose system leans one way, with the
// mutation in place before anything is served.
const open = async (scheme, cookies = []) => {
  const context = await browser.newContext({ colorScheme: scheme, viewport: { width: 1280, height: 800 } });
  if (cookies.length > 0) await context.addCookies(cookies);
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : () => '';
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('html[data-js]');
  if (await page.locator(FIELD).count() !== 1) broken(`${PAGE} carries no single appearance choice`);
  return { context, page, proof };
};
const checkProof = (proof) => {
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Nothing stored, on a dark system and on a light one: the option in force
  // is following the system both times, and the page is painted the way the
  // system leans.
  for (const scheme of ['dark', 'light']) {
    const { context, page, proof } = await open(scheme);
    const seen = await appearance(page);
    checkProof(proof);
    if (seen.stamped !== '') broken(`the root already carries a stored theme (${seen.stamped}); this probe needs a reader who has chosen nothing`);
    if (seen.shown !== 'system' || !seen.painted.includes(scheme)) {
      fail('nothing-stored-shows-the-system', `on a ${scheme} system with nothing stored the page shows ${seen.shown} painted ${seen.painted}, want system painted ${scheme}`);
    }
    await context.close();
  }

  // Picked on a light system, dark paints at once; picking the system again
  // gives the page back to the system, and the system moving then moves it.
  {
    const { context, page, proof } = await open('light');
    await page.locator(`${FIELD} input[value="dark"]`).click();
    const dark = await appearance(page);
    checkProof(proof);
    if (dark.stamped !== 'dark' || !dark.painted.includes('dark')) {
      fail('a-choice-paints-at-once', `picking dark left the root ${JSON.stringify(dark.stamped)} painted ${dark.painted}, want dark at once`);
    }
    await page.locator(`${FIELD} input[value="system"]`).click();
    const following = await appearance(page);
    checkProof(proof);
    if (following.stamped !== '' || following.painted.includes('dark')) {
      fail('following-the-system-follows-it', `picking the system on a light one left the root ${JSON.stringify(following.stamped)} painted ${following.painted}, want unstamped and light`);
    }
    await page.emulateMedia({ colorScheme: 'dark' });
    const moved = await appearance(page);
    if (!moved.painted.includes('dark') || moved.shown !== 'system') {
      fail('following-the-system-follows-it', `the system went dark under a reader following it and the page shows ${moved.shown} painted ${moved.painted}, want system painted dark`);
    }
    await context.close();
  }

  // A stored light choice is not following the system, so the system going
  // dark leaves it light — which is what separates following the reader from
  // reacting to any media change at all.
  {
    const { context, page, proof } = await open('light', [{ name: 'yomihon_theme', value: 'light', url: BASE }]);
    const before = await appearance(page);
    if (before.stamped !== 'light' || before.shown !== 'light') {
      broken(`the stored-choice page carries data-theme=${JSON.stringify(before.stamped)} showing ${before.shown}, want light`);
    }
    await page.emulateMedia({ colorScheme: 'dark' });
    const after = await appearance(page);
    checkProof(proof);
    if (after.painted.includes('dark') || after.shown !== 'light') {
      fail('a-stored-choice-holds-against-the-system', `the system went dark under a stored light choice and the page shows ${after.shown} painted ${after.painted}, want light`);
    }
    await context.close();
  }

  console.log('PASS appearance-choice: the settings page shows following the system for a reader who chose nothing and paints the way the system leans, a picked theme paints at once, following the system follows it, and a stored choice holds when the system moves');
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
