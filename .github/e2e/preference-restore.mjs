// Behavior lock for the pageshow restore of every presentation cookie.
// Theme and text size already rewrite the root and their controls after a
// back/forward-cache restore; furigana and single-key shortcuts did not, so a
// reader who turned them off on the next page came back to the old stamp and
// the old control state. Chrome refused a real bfcache restore under this
// driver — the same limit theme-toggle-pressed.mjs recorded — so the probe
// dispatches a persisted pageshow after writing the cookies, which is the
// restore branch the handler actually reads.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['ruby-follows-cookie', 'shortcuts-follow-cookie'];

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

const RUBY_RESTORE = "const ruby = readCookie('yomihon_ruby') === 'off' ? 'off' : 'on';";
const SHORTCUTS_RESTORE = "const shortcuts = readCookie('yomihon_shortcuts') === 'off' ? 'off' : 'on';";

const MUTATIONS = {
  'omit-ruby-restore': {
    target: 'ruby-follows-cookie',
    apply: rewriteModule(RUBY_RESTORE, "const ruby = root.dataset.ruby === 'off' ? 'off' : 'on';", 'ruby restore'),
  },
  'omit-shortcuts-restore': {
    target: 'shortcuts-follow-cookie',
    apply: rewriteModule(
      SHORTCUTS_RESTORE,
      "const shortcuts = root.dataset.singleKeyShortcuts === 'off' ? 'off' : 'on';",
      'shortcuts restore',
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

const snapshot = (page) =>
  page.evaluate(() => {
    const root = document.documentElement;
    const rubyToggle = document.querySelector('[data-ruby-toggle]');
    const shortcutsToggle = document.querySelector('[data-single-key-shortcuts-toggle]');
    const themeToggle = document.querySelector('[data-theme-toggle]');
    const textsizeToggle = document.querySelector('[data-textsize-toggle]');
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      ruby: root.dataset.ruby ?? '',
      shortcuts: root.dataset.singleKeyShortcuts ?? '',
      themePressed: themeToggle?.getAttribute('aria-pressed') ?? null,
      textsizeLabel: textsizeToggle?.getAttribute('aria-label') ?? null,
      rubyPressed: rubyToggle?.getAttribute('aria-pressed') ?? null,
      shortcutsChecked: shortcutsToggle ? shortcutsToggle.checked : null,
    };
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

  const before = await snapshot(page);
  if (before.rubyPressed === null) broken('the page carries no furigana control to read');
  if (before.shortcutsChecked === null) broken('the page carries no shortcut control to read');
  if (before.themePressed === null) broken('the page carries no theme control to read');
  if (before.ruby !== 'on' || before.shortcuts !== 'on') {
    broken(`the load stamp is ruby=${before.ruby} shortcuts=${before.shortcuts}; this probe needs the server defaults`);
  }

  // Cookies move on the way the next page would have written them. The DOM
  // stays at the load stamp, which is the document a cache restore would revive.
  await page.evaluate(() => {
    const expire = 'path=/;max-age=31536000;samesite=lax';
    document.cookie = `yomihon_theme=dark;${expire}`;
    document.cookie = `yomihon_textsize=xl;${expire}`;
    document.cookie = `yomihon_ruby=off;${expire}`;
    document.cookie = `yomihon_shortcuts=off;${expire}`;
    const root = document.documentElement;
    root.dataset.theme = 'light';
    root.dataset.textsize = 'm';
    root.dataset.ruby = 'on';
    root.dataset.singleKeyShortcuts = 'on';
    document.querySelector('[data-theme-toggle]')?.setAttribute('aria-pressed', 'false');
    document.querySelector('[data-ruby-toggle]')?.setAttribute('aria-pressed', 'true');
    const shortcuts = document.querySelector('[data-single-key-shortcuts-toggle]');
    if (shortcuts) shortcuts.checked = true;
  });

  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });

  const after = await snapshot(page);
  if (after.theme !== 'dark' || after.themePressed !== 'true') {
    broken(`theme did not follow the cookie (theme=${after.theme} pressed=${after.themePressed}); the handler never ran`);
  }
  if (after.textsize !== 'xl') {
    broken(`text size did not follow the cookie (textsize=${after.textsize}); the handler never ran`);
  }
  if (after.ruby !== 'off' || after.rubyPressed !== 'false') {
    fail(
      'ruby-follows-cookie',
      `after persisted pageshow, ruby=${after.ruby} aria-pressed=${after.rubyPressed}, want off / false`,
    );
  }
  if (after.shortcuts !== 'off' || after.shortcutsChecked !== false) {
    fail(
      'shortcuts-follow-cookie',
      `after persisted pageshow, shortcuts=${after.shortcuts} checked=${after.shortcutsChecked}, want off / false`,
    );
  }

  console.log('PASS preference-restore: furigana and shortcuts follow the cookies after a persisted pageshow');
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
