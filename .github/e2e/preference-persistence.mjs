// Behavior lock for what a reader keeps after leaving the reading-choices
// page: the choices survive a reload, the walk back to what they were reading,
// and the history buttons; a revived page's radios agree with the cookies
// rather than with the state they were left in; the control that applies the
// page is put away exactly where something else is keeping the choices, with
// the way back in its place; and a browser running nothing still applies every
// field from the form.
//
// The last of those is the one lock here that is meant to pass against the
// build before this one: it guards the path that already worked. The rest
// cannot, because before this change nothing was kept until the reader pressed
// the control at the foot of the page.
//
// Env: YOMIHON_BASE, PAGE_PATH (the reading-choices page reached from a note),
// and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/preferences';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'choices-survive-the-journey',
  'restore-settings-radios',
  'apply-is-put-away-when-enhanced',
  'the-way-back-is-there-instead',
  'no-javascript-fallback',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN preference-persistence: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL preference-persistence: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN preference-persistence: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED preference-persistence: ${message}`);
};

// Both rewrites go through the response the page actually loads, and both
// insist on exactly one match, so a needle the source has moved on from is
// reported rather than silently applied to nothing.
const rewriteResource = (glob, needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route(glob, async (route) => {
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
const rewriteModule = (needle, replacement, label) => rewriteResource('**/preferences.js', needle, replacement, label);
const rewriteCSS = (needle, replacement, label) => rewriteResource('**/app.css', needle, replacement, label);

// The line being rewritten is itself a template string, so its placeholders
// are built rather than written: spelled out, they would read as this file's
// own interpolation and never match the source.
const NAME = '$' + '{name}';
const VALUE = '$' + '{value}';
const storedSpan = (age) => `    document.cookie = \`yomihon_${NAME}=${VALUE};path=/;max-age=${age};samesite=lax\`;\n`;

const MUTATIONS = {
  // A choice kept only for as long as the page is open is the defect this
  // whole file is about: it looks applied and is gone by the next request.
  'store-for-this-page-view-only': {
    target: 'choices-survive-the-journey',
    apply: rewriteModule(storedSpan('31536000'), storedSpan('0'), 'the stored span'),
  },
  'skip-the-settings-radio-resync': {
    target: 'restore-settings-radios',
    apply: rewriteModule('    syncSettingsChoices();\n', '', 'the radio resync on a revived page'),
  },
  'leave-the-submit-on-the-enhanced-page': {
    target: 'apply-is-put-away-when-enhanced',
    apply: rewriteCSS('[data-js] .y-prefs__fallback{display:none}', '', 'the submit put away under a running enhancement'),
  },
  'leave-the-page-with-no-way-back': {
    target: 'the-way-back-is-there-instead',
    apply: rewriteCSS('[data-js] .y-prefs__return{display:inline-block}', '', 'the way back shown under a running enhancement'),
  },
  // The submit is hidden by a rule that asks whether anything is running. Drop
  // the question and a browser running nothing loses the only control that
  // applies anything.
  'hide-the-submit-from-everyone': {
    target: 'no-javascript-fallback',
    apply: rewriteCSS('[data-js] .y-prefs__fallback{display:none}', '.y-prefs__fallback{display:none}', 'the enhancement test on the submit'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`preference-persistence: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`preference-persistence: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`preference-persistence: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const rootState = (page) =>
  page.evaluate(() => {
    const root = document.documentElement;
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      font: root.dataset.font ?? '',
      ruby: root.dataset.ruby ?? '',
    };
  });

const checkedValues = (page) =>
  page.evaluate(() => {
    const out = {};
    for (const fieldset of document.querySelectorAll('[data-pref-field]')) {
      out[fieldset.dataset.prefField] = fieldset.querySelector('input:checked')?.value ?? '';
    }
    return out;
  });

const pick = (page, field, value) => page.click(`[data-pref-field="${field}"] input[value="${value}"]`);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const reading = await page.getAttribute('.y-prefs__return', 'href');
  if (!reading || !reading.startsWith('/notes/')) {
    broken(`the page carries no way back to a note (href ${reading}), so it was not reached from one`);
  }
  const start = await checkedValues(page);
  if (start.textsize !== 'm' || start.font !== 'serif') {
    broken(`this browser arrives with size ${start.textsize} and face ${start.font} already set, so the changes below prove nothing`);
  }

  // ── What is picked here is still true everywhere else ───────────────────
  await pick(page, 'textsize', 'l');
  await pick(page, 'font', 'kai');
  await page.waitForTimeout(200);

  await page.reload({ waitUntil: 'domcontentloaded' });
  const reloaded = await rootState(page);
  const reloadedRadios = await checkedValues(page);
  if (reloaded.textsize !== 'l' || reloaded.font !== 'kai') {
    fail(
      'choices-survive-the-journey',
      `after a reload the page is stamped size=${reloaded.textsize || 'nothing'} face=${reloaded.font || 'nothing'}, want l and kai`,
    );
  }
  if (reloadedRadios.textsize !== 'l' || reloadedRadios.font !== 'kai') {
    fail(
      'choices-survive-the-journey',
      `after a reload the page shows size=${reloadedRadios.textsize} face=${reloadedRadios.font} as chosen, want l and kai`,
    );
  }

  // The reading itself, which is what the two choices above are for.
  await page.goto(BASE + reading, { waitUntil: 'domcontentloaded' });
  const onTheNote = await rootState(page);
  if (onTheNote.textsize !== 'l' || onTheNote.font !== 'kai') {
    fail(
      'choices-survive-the-journey',
      `the note is stamped size=${onTheNote.textsize || 'nothing'} face=${onTheNote.font || 'nothing'}, want the l and kai chosen before it`,
    );
  }

  // Back to the choices, then forward to the note again. The radios are left
  // out of this pair on purpose: a revived page's radios are their own lock
  // below, and asserting them here would answer for that one too.
  await page.goBack({ waitUntil: 'domcontentloaded' });
  const back = await rootState(page);
  if (back.textsize !== 'l' || back.font !== 'kai') {
    fail('choices-survive-the-journey', `stepping back leaves size=${back.textsize || 'nothing'} face=${back.font || 'nothing'}, want l and kai`);
  }
  await page.goForward({ waitUntil: 'domcontentloaded' });
  const forward = await rootState(page);
  if (forward.textsize !== 'l' || forward.font !== 'kai') {
    fail(
      'choices-survive-the-journey',
      `stepping forward leaves size=${forward.textsize || 'nothing'} face=${forward.font || 'nothing'}, want l and kai`,
    );
  }

  // ── A revived page's radios agree with the cookies ──────────────────────
  // A real back/forward-cache restore is not driven here: Chrome refuses one
  // under this automation, and the restore branch is the same handler a
  // dispatched PageTransitionEvent reaches. What is set up is what such a
  // restore looks like — a page carrying the state it was left in, and cookies
  // that moved on elsewhere.
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => {
    document.cookie = 'yomihon_textsize=xl;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_theme=dark;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_ruby=off;path=/;max-age=31536000;samesite=lax';
  });
  const stale = await checkedValues(page);
  if (stale.textsize !== 'l' || stale.theme !== 'system' || stale.ruby !== 'on') {
    broken(`the page to be revived already shows size=${stale.textsize} theme=${stale.theme} ruby=${stale.ruby}, so nothing here would move`);
  }
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });
  const revived = await checkedValues(page);
  if (revived.textsize !== 'xl') {
    fail('restore-settings-radios', `a revived page shows size=${revived.textsize} as chosen while the cookie says xl`);
  }
  if (revived.theme !== 'dark') {
    fail('restore-settings-radios', `a revived page shows appearance=${revived.theme} as chosen while the cookie says dark`);
  }
  if (revived.ruby !== 'off') {
    fail('restore-settings-radios', `a revived page shows furigana=${revived.ruby} as chosen while the cookie says off`);
  }

  // ── One control in that corner of the page, never two ───────────────────
  if (await page.isVisible('.y-prefs__fallback')) {
    fail('apply-is-put-away-when-enhanced', 'the submit is still drawn on a page that is keeping every choice as it is picked');
  }
  if (!(await page.isVisible('.y-prefs__return'))) {
    fail('the-way-back-is-there-instead', 'nothing on the page leads back to what the reader was reading');
  }
  await page.click('.y-prefs__return');
  await page.waitForLoadState('domcontentloaded');
  if (new URL(page.url()).pathname !== reading) {
    fail('the-way-back-is-there-instead', `the way back led to ${page.url()}, want ${reading}`);
  }

  // ── And a browser running nothing still applies the whole form ──────────
  const quiet = await browser.newContext({ viewport: { width: 1280, height: 900 }, javaScriptEnabled: false });
  const plain = await quiet.newPage();
  const quietProof = MUTATE ? await MUTATIONS[MUTATE].apply(plain) : null;
  // Waiting for the whole load rather than the parse: with nothing running,
  // the deferred module no longer holds the document open, and the parse can
  // finish before the stylesheet this page is judged by has even arrived.
  await plain.goto(BASE + PAGE, { waitUntil: 'load' });
  if (quietProof) {
    const issue = quietProof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  if (!(await plain.isVisible('.y-prefs__fallback'))) {
    fail('no-javascript-fallback', 'a browser running nothing is shown no control that applies anything');
  }
  if (await plain.isVisible('.y-prefs__return')) {
    fail('no-javascript-fallback', 'a browser running nothing is offered a way out that would leave every choice unapplied');
  }
  await plain.click('[data-pref-field="textsize"] input[value="xl"]');
  await plain.click('[data-pref-field="font"] input[value="sans"]');
  await plain.click('.y-prefs__fallback');
  await plain.waitForLoadState('domcontentloaded');
  const applied = {};
  for (const cookie of await quiet.cookies()) applied[cookie.name] = cookie.value;
  if (applied.yomihon_textsize !== 'xl' || applied.yomihon_font !== 'sans') {
    fail(
      'no-javascript-fallback',
      `the form stored size=${applied.yomihon_textsize || 'nothing'} face=${applied.yomihon_font || 'nothing'}, want xl and sans together`,
    );
  }
  if (new URL(plain.url()).pathname !== reading) {
    fail('no-javascript-fallback', `applying the form landed on ${plain.url()}, want the reader back on ${reading}`);
  }

  console.log(
    'PASS preference-persistence: choices survive a reload, the note, and both history buttons; a revived page agrees with the cookies; the submit and the way back swap places with the enhancement; and the form still applies every field without one',
  );
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
