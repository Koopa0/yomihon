// Behavior lock for the cache-restore path in preferences.js. After a
// back/forward-cache restore the cookies are the truth, and every persisted
// preference — theme, text size, furigana, and single-key shortcuts — has to
// agree with them, including both root attributes and both controls.
//
// A real bfcache restore is not driven here: Chrome refused one under this
// automation, and the restore branch is the same handler a dispatched
// PageTransitionEvent({ persisted: true }) reaches. Theme and text size
// already move on that path; the lock is that furigana and shortcuts move
// with them, and that only the literal cookie "off" is off.
//
// Env: YOMIHON_BASE, PAGE_PATH (any chrome page), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'restore-theme',
  'restore-textsize',
  'restore-ruby',
  'restore-shortcuts',
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
  'skip-theme-resync': {
    target: 'restore-theme',
    apply: rewriteModule(
      "    const theme = readCookie('yomihon_theme');\n    if (theme === 'dark' || theme === 'light') {\n      root.dataset.theme = theme;\n    } else {\n      delete root.dataset.theme;\n    }\n    themeToggle?.setAttribute('aria-pressed', String(effectiveTheme() === 'dark'));\n",
      '',
      'theme restore',
    ),
  },
  'skip-textsize-resync': {
    target: 'restore-textsize',
    apply: rewriteModule(
      "    const stored = readCookie('yomihon_textsize');\n    const size = stored === 'l' || stored === 'xl' ? stored : 'm';\n    root.dataset.textsize = size;\n    textsizeToggle?.setAttribute('aria-label', textsizeLabel(size));\n",
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
      "    const shortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';\n    root.dataset.singleKeyShortcuts = shortcuts;\n    if (shortcutsToggle) shortcutsToggle.checked = shortcuts === 'on';\n",
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
      "    const shortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';\n",
      "    const shortcuts = readCookie('yomihon_shortcuts') !== 'on' ? 'off' : 'on';\n",
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

const preferenceState = (page) =>
  page.evaluate(() => {
    const root = document.documentElement;
    const themeToggle = document.querySelector('[data-theme-toggle]');
    const textsizeToggle = document.querySelector('[data-textsize-toggle]');
    const rubyToggle = document.querySelector('[data-ruby-toggle]');
    const shortcutsToggle = document.querySelector('[data-single-key-shortcuts-toggle]');
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      ruby: root.dataset.ruby ?? '',
      shortcuts: root.dataset.singleKeyShortcuts ?? '',
      themePressed: themeToggle ? themeToggle.getAttribute('aria-pressed') : null,
      textsizeLabel: textsizeToggle ? textsizeToggle.getAttribute('aria-label') : null,
      extraLargeLabel: textsizeToggle ? textsizeToggle.dataset.labelXl : null,
      rubyPressed: rubyToggle ? rubyToggle.getAttribute('aria-pressed') : null,
      shortcutsChecked: shortcutsToggle ? shortcutsToggle.checked : null,
    };
  });

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
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const onLoad = await preferenceState(page);
  if (onLoad.themePressed === null || onLoad.textsizeLabel === null) {
    broken('the page carries no theme or text-size control to read');
  }
  if (onLoad.rubyPressed === null || onLoad.shortcutsChecked === null) {
    broken('the page carries no furigana or shortcuts control to read');
  }
  if (!onLoad.extraLargeLabel) {
    broken('the text-size control does not carry the extra-large name the restore path rewrites from');
  }

  // Cookies say what the reader chose on a later page; the revived document
  // still carries the older stamp. Dispatching pageshow is the restore path.
  await page.evaluate(() => {
    const root = document.documentElement;
    root.dataset.theme = 'light';
    root.dataset.textsize = 'm';
    root.dataset.ruby = 'on';
    root.dataset.singleKeyShortcuts = 'on';
    document.querySelector('[data-theme-toggle]')?.setAttribute('aria-pressed', 'false');
    document.querySelector('[data-ruby-toggle]')?.setAttribute('aria-pressed', 'true');
    const shortcutsToggle = document.querySelector('[data-single-key-shortcuts-toggle]');
    if (shortcutsToggle) shortcutsToggle.checked = true;
    document.cookie = 'yomihon_theme=dark;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_textsize=xl;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_ruby=off;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_shortcuts=off;path=/;max-age=31536000;samesite=lax';
  });

  const before = await preferenceState(page);
  if (before.theme !== 'light' || before.textsize !== 'm' || before.ruby !== 'on' || before.shortcuts !== 'on') {
    broken(`stale stamp did not take: theme=${before.theme} textsize=${before.textsize} ruby=${before.ruby} shortcuts=${before.shortcuts}`);
  }

  await dispatchRestore(page);
  const after = await preferenceState(page);

  if (after.theme !== 'dark' || after.themePressed !== 'true') {
    fail('restore-theme', `after restore theme=${after.theme} aria-pressed=${after.themePressed}, want dark/true`);
  }
  if (after.textsize !== 'xl' || after.textsizeLabel !== after.extraLargeLabel) {
    fail('restore-textsize', `after restore textsize=${after.textsize} aria-label=${after.textsizeLabel}, want xl/${after.extraLargeLabel}`);
  }
  if (after.ruby !== 'off' || after.rubyPressed !== 'false') {
    fail('restore-ruby', `after restore ruby=${after.ruby} aria-pressed=${after.rubyPressed}, want off/false`);
  }
  if (after.shortcuts !== 'off' || after.shortcutsChecked !== false) {
    fail('restore-shortcuts', `after restore shortcuts=${after.shortcuts} checked=${after.shortcutsChecked}, want off/false`);
  }

  // A cookie the server would ignore must not turn the aids off. Stamp the
  // page off, store a value that is not the literal "off", and the restore
  // has to put both back on.
  await page.evaluate(() => {
    const root = document.documentElement;
    root.dataset.ruby = 'off';
    root.dataset.singleKeyShortcuts = 'off';
    document.querySelector('[data-ruby-toggle]')?.setAttribute('aria-pressed', 'false');
    const shortcutsToggle = document.querySelector('[data-single-key-shortcuts-toggle]');
    if (shortcutsToggle) shortcutsToggle.checked = false;
    document.cookie = 'yomihon_ruby=maybe;path=/;max-age=31536000;samesite=lax';
    document.cookie = 'yomihon_shortcuts=disabled;path=/;max-age=31536000;samesite=lax';
  });
  await dispatchRestore(page);
  const normalised = await preferenceState(page);
  if (normalised.ruby !== 'on' || normalised.rubyPressed !== 'true') {
    fail('restore-ruby-unknown-is-on', `unknown ruby cookie left ruby=${normalised.ruby} aria-pressed=${normalised.rubyPressed}, want on/true`);
  }
  if (normalised.shortcuts !== 'on' || normalised.shortcutsChecked !== true) {
    fail('restore-shortcuts-unknown-is-on', `unknown shortcuts cookie left shortcuts=${normalised.shortcuts} checked=${normalised.shortcutsChecked}, want on/true`);
  }

  console.log('PASS preference-restore: all four preferences and both controls agree with the cookies after a persisted pageshow, and only the literal off is off');
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
