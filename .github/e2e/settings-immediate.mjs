// Behavior lock for the reading choices page: picking an option applies it and
// stores it there and then, with no second press. Five of the six choices are
// answered on the page the reader is already on; the interface language needs
// the server to write the words again, so it navigates — back to this page, so
// a reader adjusting several things is not thrown out of it by the first one.
//
// What is locked here, in the order it is checked: each of the five taking
// effect on the page itself, all six surviving a reload, a walk away and back,
// and Back/Forward, the language landing on this page with the way back to the
// reading intact, a revived document's controls agreeing with the cookies, and
// the submit control still being there and still working for a browser with no
// scripting at all.
//
// The last of those passes against the page as it was before immediate saving
// existed, which is the point of it: it guards what must not be lost, while
// every other site here describes behaviour the page did not have.
//
// A real back/forward-cache restore is not driven: Chrome refused one under
// this automation, and the restore branch is the same handler a dispatched
// PageTransitionEvent({ persisted: true }) reaches.
//
// Env: YOMIHON_BASE, PAGE_PATH (the settings page, carrying the reading it was
// reached from in ?from=), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/preferences?from=%2Fnotes%2FNotes%2Falpha.md';
const MUTATE = process.env.MUTATE || '';

// The reading this page was reached from, taken out of the address the table
// drives it with rather than named a second time here.
const READING = new URL(BASE + PAGE).searchParams.get('from') || '/';

// The choices are found by the field they are made of rather than by the
// attribute the enhancement adds, so this probe can be pointed at a build that
// has no enhancement at all and report which behaviour is missing instead of
// which hook is.
const CHOICES = 'form:has(input[name="theme"])';
const FORM = CHOICES + ' ';

const SITES = [
  'apply-theme',
  'submit-hidden-when-live',
  'apply-textsize',
  'apply-font',
  'apply-ruby',
  'apply-shortcuts',
  'persisted-across-visits',
  'language-returns-to-settings',
  'restored-after-bfcache',
  'nojs-submit-remains',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN settings-immediate: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL settings-immediate: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN settings-immediate: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED settings-immediate: ${message}`);
};

// Rewrites the enhancement's own source on its way to the browser. The needle
// has to match exactly once: a mutation that matched nothing would let the
// probe pass over an untouched page and report a catch it never made.
const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/preferences.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => (matches === 1 ? '' : `${label} needle matched ${matches} times, want exactly 1`);
};

// Rewrites the served page itself, for the regression a script cannot cause:
// the no-scripting reader's only control going missing from the markup.
const rewriteDocument = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route((url) => url.pathname === '/preferences', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    return route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => (matches === 1 ? '' : `${label} needle matched ${matches} times, want exactly 1`);
};

// Each mutation says which page it is armed against, because arming a module
// rewrite on the page that runs no scripts would report a needle that matched
// nothing rather than the regression it stands for.
const MUTATIONS = {
  'skip-theme-apply': {
    target: 'apply-theme',
    on: 'scripted',
    apply: rewriteModule(
      "      writeTheme(value === 'system' ? null : value, false);\n",
      '      void value;\n',
      'theme apply',
    ),
  },
  'skip-textsize-apply': {
    target: 'apply-textsize',
    on: 'scripted',
    apply: rewriteModule('      root.dataset.textsize = value;\n', '      void value;\n', 'text size apply'),
  },
  'skip-font-apply': {
    target: 'apply-font',
    on: 'scripted',
    apply: rewriteModule('      root.dataset.font = value;\n', '      void value;\n', 'typeface apply'),
  },
  'skip-ruby-apply': {
    target: 'apply-ruby',
    on: 'scripted',
    apply: rewriteModule('      root.dataset.ruby = value;\n', '      void value;\n', 'furigana apply'),
  },
  'skip-shortcuts-apply': {
    target: 'apply-shortcuts',
    on: 'scripted',
    apply: rewriteModule(
      '      root.dataset.singleKeyShortcuts = value;\n',
      '      void value;\n',
      'shortcuts apply',
    ),
  },
  'skip-immediate-save': {
    target: 'persisted-across-visits',
    on: 'scripted',
    apply: rewriteModule('      saveSettings();\n', '', 'immediate save'),
  },
  'language-returns-to-reading': {
    target: 'language-returns-to-settings',
    on: 'scripted',
    apply: rewriteModule(
      '    if (next) next.value = location.pathname + location.search;\n',
      '',
      'language return address',
    ),
  },
  'skip-settings-resync': {
    target: 'restored-after-bfcache',
    on: 'scripted',
    apply: rewriteModule(
      "    syncSettings({\n      theme: root.dataset.theme ?? 'system',\n      textsize: size,\n      font: root.dataset.font ?? 'serif',\n      ruby,\n      shortcuts,\n    });\n",
      '',
      'settings resync',
    ),
  },
  'never-say-live': {
    target: 'submit-hidden-when-live',
    on: 'scripted',
    apply: rewriteModule("    settingsForm.dataset.preferencesLive = 'on';\n", '', 'live marker'),
  },
  'hide-submit-control': {
    target: 'nojs-submit-remains',
    on: 'nojs',
    apply: rewriteDocument(
      '<button class="y-prefs__apply" type="submit" data-preference-apply>',
      '<button class="y-prefs__apply" type="submit" data-preference-apply hidden>',
      'submit control',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`settings-immediate: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`settings-immediate: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`settings-immediate: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const cookies = async (context) => {
  const jar = {};
  for (const cookie of await context.cookies()) jar[cookie.name] = cookie.value;
  return jar;
};

// What the page's own controls say, read as one answer per choice so a report
// names the choice rather than an element.
const chosen = (page) =>
  page.evaluate((selector) => {
    const form = document.querySelector(selector);
    const answers = {};
    for (const input of form.querySelectorAll('input[type="radio"]:checked')) answers[input.name] = input.value;
    return answers;
  }, CHOICES);

const rootState = (page) =>
  page.evaluate(() => {
    const root = document.documentElement;
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      font: root.dataset.font ?? '',
      ruby: root.dataset.ruby ?? '',
      shortcuts: root.dataset.singleKeyShortcuts ?? '',
      themePressed: document.querySelector('[data-theme-toggle]')?.getAttribute('aria-pressed') ?? null,
      textsizeName: document.querySelector('[data-textsize-toggle]')?.getAttribute('aria-label') ?? null,
      extraLargeName: document.querySelector('[data-textsize-toggle]')?.dataset.labelXl ?? null,
      rubyPressed: document.querySelector('[data-ruby-toggle]')?.getAttribute('aria-pressed') ?? null,
      shortcutsChecked: document.querySelector('[data-single-key-shortcuts-toggle]')?.checked ?? null,
    };
  });

// Polls until read() answers something settle() accepts, and hands back the
// last answer either way, so a lock that fires reports what it actually saw
// rather than that it gave up.
const until = async (read, settled, deadlineMs) => {
  const stop = Date.now() + deadlineMs;
  let last = await read();
  while (!settled(last) && Date.now() < stop) {
    await new Promise((resolve) => setTimeout(resolve, 50));
    last = await read();
  }
  return last;
};

const STORED = { theme: 'dark', textsize: 'xl', font: 'sans', ruby: 'off', shortcuts: 'off' };
const cookieName = { theme: 'yomihon_theme', textsize: 'yomihon_textsize', font: 'yomihon_font', ruby: 'yomihon_ruby', shortcuts: 'yomihon_shortcuts' };
const allStored = (jar) => Object.entries(STORED).every(([name, value]) => jar[cookieName[name]] === value);
const describe = (answers) =>
  Object.entries(answers)
    .map(([name, value]) => `${name}=${value}`)
    .join(' ');

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  if (MUTATE && MUTATIONS[MUTATE].on === 'scripted') proof = await MUTATIONS[MUTATE].apply(page);
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    proof = null;
  }

  const opening = await chosen(page);
  for (const name of ['lang', ...Object.keys(STORED)]) {
    if (!opening[name]) broken(`the page offers no answer for ${name}, so there is nothing here to pick`);
  }

  // Each of the five the page can answer itself, checked the moment it is
  // picked: the root the stylesheet reads, and the header control that reports
  // the same state from the other end of the page.
  await page.click(`${FORM}input[name="theme"][value="dark"]`);
  let root = await rootState(page);
  if (root.theme !== 'dark' || root.themePressed !== 'true') {
    fail('apply-theme', `picking dark left theme=${root.theme} and the header control at aria-pressed=${root.themePressed}, want dark/true with no Apply pressed`);
  }

  // With picking an option doing the saving, the control that used to do it is
  // a control with nothing left to do, and it goes — but only once the code
  // that replaced it has said it is running.
  const submitWhenLive = page.locator(`${FORM}button[type="submit"]`);
  if ((await submitWhenLive.count()) !== 1) {
    broken(`the choices carry ${await submitWhenLive.count()} submit controls, want exactly one to reason about`);
  }
  if (await submitWhenLive.isVisible()) {
    fail('submit-hidden-when-live', 'the submit control is still on the page with the enhancement running, so a reader is shown a press that changes nothing');
  }

  await page.click(`${FORM}input[name="textsize"][value="xl"]`);
  root = await rootState(page);
  if (root.textsize !== 'xl' || root.textsizeName !== root.extraLargeName) {
    fail('apply-textsize', `picking extra large left textsize=${root.textsize} and the header control named ${root.textsizeName}, want xl/${root.extraLargeName}`);
  }

  await page.click(`${FORM}input[name="font"][value="sans"]`);
  root = await rootState(page);
  if (root.font !== 'sans') {
    fail('apply-font', `picking the sans face left font=${root.font}, want sans`);
  }

  await page.click(`${FORM}input[name="ruby"][value="off"]`);
  root = await rootState(page);
  if (root.ruby !== 'off' || root.rubyPressed !== 'false') {
    fail('apply-ruby', `switching furigana off left ruby=${root.ruby} and the header control at aria-pressed=${root.rubyPressed}, want off/false`);
  }

  await page.click(`${FORM}input[name="shortcuts"][value="off"]`);
  root = await rootState(page);
  if (root.shortcuts !== 'off' || root.shortcutsChecked !== false) {
    fail('apply-shortcuts', `switching the single-key shortcuts off left them at ${root.shortcuts} and the help panel's control checked=${root.shortcutsChecked}, want off/false`);
  }

  // Nothing was confirmed. Everything picked has to be in this browser's
  // cookies, and still be there through a reload, a walk away and back, and
  // the Back and Forward the reader reaches for afterwards.
  const jar = await until(() => cookies(context), allStored, 5000);
  if (!allStored(jar)) {
    fail('persisted-across-visits', `no Apply was pressed and the cookies read ${describe(jar)}, want every choice stored`);
  }

  await page.reload({ waitUntil: 'domcontentloaded' });
  let answers = await chosen(page);
  for (const [name, value] of Object.entries(STORED)) {
    if (answers[name] !== value) {
      fail('persisted-across-visits', `after a reload the page shows ${describe(answers)}, want ${name}=${value}`);
    }
  }

  await page.goto(BASE + READING, { waitUntil: 'domcontentloaded' });
  root = await rootState(page);
  if (root.theme !== 'dark' || root.textsize !== 'xl' || root.font !== 'sans' || root.ruby !== 'off') {
    fail('persisted-across-visits', `the reading was drawn with theme=${root.theme} textsize=${root.textsize} font=${root.font} ruby=${root.ruby}, want the stored choices`);
  }

  await page.click('.y-prefslink');
  await page.waitForLoadState('domcontentloaded');
  answers = await chosen(page);
  for (const [name, value] of Object.entries(STORED)) {
    if (answers[name] !== value) {
      fail('persisted-across-visits', `reopening the page from the reading shows ${describe(answers)}, want ${name}=${value}`);
    }
  }

  await page.goBack({ waitUntil: 'domcontentloaded' });
  root = await rootState(page);
  if (root.theme !== 'dark' || root.textsize !== 'xl') {
    fail('persisted-across-visits', `stepping back to the reading drew it with theme=${root.theme} textsize=${root.textsize}, want the stored choices`);
  }
  await page.goForward({ waitUntil: 'domcontentloaded' });
  answers = await chosen(page);
  for (const [name, value] of Object.entries(STORED)) {
    if (answers[name] !== value) {
      fail('persisted-across-visits', `stepping forward to the page shows ${describe(answers)}, want ${name}=${value}`);
    }
  }

  // A revived document carries the controls it was drawn with, and the cookies
  // may have moved on under it — a theme changed from the header two pages
  // later. The cookies are the truth, and this page's controls have to say so.
  await page.evaluate(() => {
    const form = document.querySelector('form:has(input[name="theme"])');
    form.elements.theme.value = 'light';
    form.elements.textsize.value = 'm';
    form.elements.font.value = 'kai';
    form.elements.ruby.value = 'on';
    form.elements.shortcuts.value = 'on';
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });
  answers = await chosen(page);
  for (const [name, value] of Object.entries(STORED)) {
    if (answers[name] !== value) {
      fail('restored-after-bfcache', `a revived page shows ${describe(answers)} against cookies holding ${describe(STORED)}, want ${name}=${value}`);
    }
  }

  // The words are the server's, so the language is the one choice that has to
  // travel. It comes back to this page, still carrying the reading it was
  // reached from, rather than ending the visit at the article.
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    page.click(`${FORM}input[name="lang"][value="en"]`),
  ]);
  const landed = new URL(page.url());
  // Asked before the page is read for anything else: a build that sent the
  // reader to the article has no choices on screen to read, and waiting for
  // one there would report a timeout instead of the address that was wrong.
  if (landed.pathname !== '/preferences') {
    fail('language-returns-to-settings', `picking English landed on ${landed.pathname}${landed.search}, want the settings page so the reader can carry on adjusting`);
  }
  const spoken = await page.evaluate(() => document.documentElement.lang);
  const stored = (await cookies(context)).yomihon_lang;
  if (spoken !== 'en' || stored !== 'en') {
    fail('language-returns-to-settings', `picking English left the page speaking ${spoken} with the language cookie at ${stored}, want en for both`);
  }
  const carried = await page.getAttribute(`${FORM}input[name="next"]`, 'value');
  if (carried !== READING) {
    fail('language-returns-to-settings', `after the language changed the way back reads ${carried}, want ${READING}`);
  }
  await context.close();

  // The same page with no scripting at all. The submit control is that
  // reader's whole way of being heard, so it is on the page, it is visible,
  // and pressing it is still what stores the choices.
  const plain = await browser.newContext({ viewport: { width: 1280, height: 900 }, javaScriptEnabled: false });
  const bare = await plain.newPage();
  if (MUTATE && MUTATIONS[MUTATE].on === 'nojs') proof = await MUTATIONS[MUTATE].apply(bare);
  await bare.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    proof = null;
  }
  const submit = bare.locator(`${FORM}button[type="submit"]`);
  if ((await submit.count()) !== 1 || !(await submit.isVisible())) {
    fail('nojs-submit-remains', `with no scripting the choices carry ${await submit.count()} submit controls and the first is ${(await submit.count()) ? 'not visible' : 'absent'}, want exactly one a reader can press`);
  }
  await bare.check(`${FORM}input[name="theme"][value="dark"]`);
  await bare.check(`${FORM}input[name="textsize"][value="xl"]`);
  await new Promise((resolve) => setTimeout(resolve, 400));
  const untouched = await cookies(plain);
  if (Object.keys(untouched).length !== 0) {
    broken(`picking an option with no scripting stored ${describe(untouched)}; this fallback is meant to wait for the press`);
  }
  await Promise.all([bare.waitForNavigation({ waitUntil: 'domcontentloaded' }), submit.click()]);
  const pressed = await cookies(plain);
  if (pressed.yomihon_theme !== 'dark' || pressed.yomihon_textsize !== 'xl') {
    fail('nojs-submit-remains', `pressing the control with no scripting stored ${describe(pressed)}, want the theme and size that were picked`);
  }
  await plain.close();

  console.log('PASS settings-immediate: every choice applies and is stored the moment it is picked, survives a reload, a revisit, Back and Forward and a revived document, the language comes back to the page carrying the way home, and the no-scripting form still saves on its own button');
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
