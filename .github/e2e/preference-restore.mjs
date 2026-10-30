// Behavior lock for the cache-restore path in preferences.js. After a
// back/forward-cache restore the cookies are the truth, and every persisted
// preference — theme, text size, typeface, furigana, and single-key shortcuts
// — has to agree with them: the root attributes on a reading page, the
// furigana switch that page draws, and the options the settings page shows in
// force.
//
// A real bfcache restore is not driven here: Chrome refused one under this
// automation, and the restore branch is the same handler a dispatched
// PageTransitionEvent({ persisted: true }) reaches. Theme and text size
// already move on that path; the lock is that furigana and shortcuts move
// with them, and that only the literal cookie "off" is off.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note carrying furigana, so the switch for
// them is drawn), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const SETTINGS = '/preferences';
const CHOICES = ['theme', 'textsize', 'font', 'ruby', 'shortcuts'];
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'restore-theme',
  'restore-textsize',
  'restore-ruby',
  'restore-shortcuts',
  'restore-font',
  'restore-ruby-unknown-is-on',
  'restore-shortcuts-unknown-is-on',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN preference-restore: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL preference-restore: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN preference-restore: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED preference-restore: ${message}`); };

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/preferences.js{,?*}', async (route) => {
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
  'skip-font-resync': {
    target: 'restore-font',
    apply: rewriteModule(
      "    const font = readCookie('yomihon_font');\n",
      "    const font = '';\n",
      'font restore',
    ),
  },
  'skip-theme-resync': {
    target: 'restore-theme',
    apply: rewriteModule(
      "    const theme = readCookie('yomihon_theme');\n    writeTheme(theme === 'dark' || theme === 'light' ? theme : null, false);\n",
      '',
      'theme restore',
    ),
  },
  'skip-textsize-resync': {
    target: 'restore-textsize',
    apply: rewriteModule(
      "    const stored = readCookie('yomihon_textsize');\n    root.dataset.textsize = stored === 'l' || stored === 'xl' ? stored : 'm';\n",
      '',
      'textsize restore',
    ),
  },
  'skip-ruby-resync': {
    target: 'restore-ruby',
    apply: rewriteModule(
      "    const ruby = readCookie('yomihon_ruby') === 'off' ? 'off' : 'on';\n    root.dataset.ruby = ruby;\n    rubyToggle?.setAttribute('aria-pressed', String(ruby === 'on'));\n",
      '',
      'ruby restore',
    ),
  },
  'skip-shortcuts-resync': {
    target: 'restore-shortcuts',
    apply: rewriteModule(
      "    root.dataset.singleKeyShortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';\n",
      '',
      'shortcuts restore',
    ),
  },
  'treat-unknown-ruby-as-off': {
    target: 'restore-ruby-unknown-is-on',
    apply: rewriteModule(
      "    const ruby = readCookie('yomihon_ruby') === 'off' ? 'off' : 'on';\n",
      "    const ruby = readCookie('yomihon_ruby') !== 'on' ? 'off' : 'on';\n",
      'ruby normalisation',
    ),
  },
  'treat-unknown-shortcuts-as-off': {
    target: 'restore-shortcuts-unknown-is-on',
    apply: rewriteModule(
      "    root.dataset.singleKeyShortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';\n",
      "    root.dataset.singleKeyShortcuts = readCookie('yomihon_shortcuts') !== 'on' ? 'off' : 'on';\n",
      'shortcuts normalisation',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`preference-restore: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`preference-restore: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`preference-restore: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What the page in front of the probe says about each choice: the root's
// attributes, the furigana switch where the page draws one, and the option the
// settings page shows in force for each choice where the page is that one.
const preferenceState = (page) =>
  page.evaluate((choices) => {
    const root = document.documentElement;
    const rubyToggle = document.querySelector('[data-ruby-toggle]');
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      ruby: root.dataset.ruby ?? '',
      font: root.dataset.font ?? '',
      shortcuts: root.dataset.singleKeyShortcuts ?? '',
      rubyPressed: rubyToggle ? rubyToggle.getAttribute('aria-pressed') : null,
      shown: Object.fromEntries(choices.map((name) => [
        name,
        document.querySelector(`[data-pref-field="${name}"] input:checked`)?.value ?? null,
      ])),
    };
  }, CHOICES);

// Stamps the revived document with what it said before the reader moved on,
// and stores what they chose since, the way a later page would have.
const stampStale = (page, stale, cookies) =>
  page.evaluate(({ stamp, stored }) => {
    const root = document.documentElement;
    for (const [name, value] of Object.entries(stamp.root)) root.dataset[name] = value;
    document.querySelector('[data-ruby-toggle]')?.setAttribute('aria-pressed', stamp.rubyPressed);
    for (const [name, value] of Object.entries(stamp.shown)) {
      const option = document.querySelector(`[data-pref-field="${name}"] input[value="${value}"]`);
      if (option) option.checked = true;
    }
    for (const [name, value] of Object.entries(stored)) {
      document.cookie = `yomihon_${name}=${value};path=/;max-age=31536000;samesite=lax`;
    }
  }, { stamp: stale, stored: cookies });

const dispatchRestore = (page) =>
  page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  for (const [index, path] of [PAGE, SETTINGS].entries()) {
    const onSettings = path === SETTINGS;
    await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
    if (proof && index === 0) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const where = onSettings ? 'the settings page' : 'the reading page';

    const onLoad = await preferenceState(page);
    if (!onSettings && onLoad.rubyPressed === null) {
      broken(`${PAGE} draws no furigana switch to read; this probe has to be driven against a note that carries furigana`);
    }
    if (onSettings && CHOICES.some((name) => onLoad.shown[name] === null)) {
      broken(`the settings page shows no option in force for some choice: ${JSON.stringify(onLoad.shown)}`);
    }

    // Cookies say what the reader chose on a later page; the revived document
    // still carries the older stamp. Dispatching pageshow is the restore path.
    await stampStale(page, {
      root: { theme: 'light', textsize: 'm', ruby: 'on', singleKeyShortcuts: 'on', font: 'serif' },
      rubyPressed: 'true',
      shown: { theme: 'light', textsize: 'm', font: 'serif', ruby: 'on', shortcuts: 'on' },
    }, { theme: 'dark', textsize: 'xl', ruby: 'off', shortcuts: 'off', font: 'sans' });

    const before = await preferenceState(page);
    if (before.theme !== 'light' || before.textsize !== 'm' || before.ruby !== 'on' || before.shortcuts !== 'on') {
      broken(`stale stamp did not take: theme=${before.theme} textsize=${before.textsize} ruby=${before.ruby} shortcuts=${before.shortcuts}`);
    }

    await dispatchRestore(page);
    const after = await preferenceState(page);
    // Where the settings page is not the one being read, it has no options to
    // show and the question about them is not asked.
    const shows = (name, value) => !onSettings || after.shown[name] === value;

    if (after.theme !== 'dark' || !shows('theme', 'dark')) {
      fail('restore-theme', `${where}: after restore theme=${after.theme} shown=${after.shown.theme}, want dark`);
    }
    if (after.textsize !== 'xl' || !shows('textsize', 'xl')) {
      fail('restore-textsize', `${where}: after restore textsize=${after.textsize} shown=${after.shown.textsize}, want xl`);
    }
    if (after.ruby !== 'off' || (!onSettings && after.rubyPressed !== 'false') || !shows('ruby', 'off')) {
      fail('restore-ruby', `${where}: after restore ruby=${after.ruby} aria-pressed=${after.rubyPressed} shown=${after.shown.ruby}, want off`);
    }
    // The typeface is the reading surface itself, and it was the one stamped
    // choice the restore path walked past: theme and size came back corrected
    // while the words stayed in the face the reader had already left behind.
    if (after.font !== 'sans' || !shows('font', 'sans')) {
      fail('restore-font', `${where}: after restore font=${after.font} shown=${after.shown.font}, want sans`);
    }
    if (after.shortcuts !== 'off' || !shows('shortcuts', 'off')) {
      fail('restore-shortcuts', `${where}: after restore shortcuts=${after.shortcuts} shown=${after.shown.shortcuts}, want off`);
    }

    // A cookie the server would ignore must not turn the aids off. Stamp the
    // page off, store a value that is not the literal "off", and the restore
    // has to put both back on.
    await stampStale(page, {
      root: { ruby: 'off', singleKeyShortcuts: 'off' },
      rubyPressed: 'false',
      shown: { ruby: 'off', shortcuts: 'off' },
    }, { ruby: 'maybe', shortcuts: 'disabled' });
    await dispatchRestore(page);
    const normalised = await preferenceState(page);
    if (normalised.ruby !== 'on' || (!onSettings && normalised.rubyPressed !== 'true') || (onSettings && normalised.shown.ruby !== 'on')) {
      fail('restore-ruby-unknown-is-on', `${where}: unknown ruby cookie left ruby=${normalised.ruby} aria-pressed=${normalised.rubyPressed} shown=${normalised.shown.ruby}, want on`);
    }
    if (normalised.shortcuts !== 'on' || (onSettings && normalised.shown.shortcuts !== 'on')) {
      fail('restore-shortcuts-unknown-is-on', `${where}: unknown shortcuts cookie left shortcuts=${normalised.shortcuts} shown=${normalised.shown.shortcuts}, want on`);
    }
    // Nothing the next page is drawn with may be left over from this one.
    await context.clearCookies();
  }

  console.log('PASS preference-restore: every persisted preference, the furigana switch and the options the settings page shows agree with the cookies after a persisted pageshow, and only the literal off is off');
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
