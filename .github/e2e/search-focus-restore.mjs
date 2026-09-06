// Behavior lock: closing search returns focus to whoever held it when the
// dialog opened. Escape used to land on the header search control after
// dialog.close(), so a reader who opened from a body link found themselves in
// the chrome; Ctrl+K toggle-close already restored. The lock drives real
// Tab / Enter / Ctrl+K / Escape. Substituting element.focus() for those
// keystrokes is exactly what lets this class of test pass while the product
// stays broken.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with an authored body wikilink), MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const DIALOG = '[data-search]';
const SEARCH_OPEN = '[data-search-open]';
const BODY_LINK = '.y-prose a.wikilink[href="/notes/Notes/beta.md"]';
const TAB_CAP = 30;

const SITES = [
  'escape-restores-body-opener',
  'tab-continues-from-restored',
  'shortcut-restores-body-opener',
  'header-open-returns-to-header',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN search-focus-restore: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL search-focus-restore: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN search-focus-restore: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED search-focus-restore: ${message}`); };

const targetsOf = (mutation) => (Array.isArray(mutation.target) ? mutation.target : [mutation.target]);

const rewriteSearch = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/search.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({
      response,
      body: count === 1 ? original.replace(needle, replacement) : original,
    });
  });
  return () => {
    if (requests !== 1) return `search.js was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `search.js needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  // The original defect: after dialog.close(), focus is forced onto the
  // header control, overriding the platform's return.
  'focus-header-on-escape': {
    target: 'escape-restores-body-opener',
    apply: rewriteSearch(
      '    dialog.close();\n  }',
      '    dialog.close();\n    document.querySelector(\'[data-search-open]\')?.focus();\n  }',
    ),
  },
  'focus-header-on-shortcut-close': {
    target: 'shortcut-restores-body-opener',
    apply: rewriteSearch(
      '    if (dialog.open) dialog.close();',
      '    if (dialog.open) { dialog.close(); document.querySelector(\'[data-search-open]\')?.focus(); }',
    ),
  },
  'open-as-if-the-body-link-held-focus': {
    target: 'header-open-returns-to-header',
    apply: rewriteSearch(
      '    if (!dialog.open) dialog.showModal();',
      '    document.querySelector(\'.y-prose a.wikilink\')?.focus();\n    if (!dialog.open) dialog.showModal();',
    ),
  },
  // Keeps the Escape landing correct and steals the next Tab into the header,
  // so tab-continues-from-restored is watched red on its own.
  'next-tab-jumps-header': {
    target: 'tab-continues-from-restored',
    apply: rewriteSearch(
      '    dialog.close();\n  }',
      '    dialog.close();\n    document.addEventListener(\'keydown\', (event) => {\n      if (event.key === \'Tab\') {\n        event.preventDefault();\n        document.querySelector(\'[data-search-open]\')?.focus();\n      }\n    }, { once: true });\n  }',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  for (const site of targetsOf(mutation)) {
    if (!SITES.includes(site)) {
      console.error(`search-focus-restore: mutation ${name} aims at unknown site ${site}`);
      process.exit(2);
    }
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => targetsOf(mutation).includes(site))) {
    console.error(`search-focus-restore: site ${site} has no mutation, so nothing proves it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`search-focus-restore: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const arm = async (page, sites) => {
  if (!MUTATE) return null;
  const mutation = MUTATIONS[MUTATE];
  if (!targetsOf(mutation).some((site) => sites.includes(site))) return null;
  return mutation.apply(page);
};

const proveApplied = (proof) => {
  if (!proof) return;
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const frames = (page) => page.evaluate(() => new Promise((resolve) => {
  requestAnimationFrame(() => requestAnimationFrame(resolve));
}));

const focusInfo = (page) => page.evaluate((bodyLink) => {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement)) {
    return {
      tag: active?.tagName ?? 'none',
      searchOpen: false,
      inHeader: false,
      inMain: false,
      inProse: false,
      href: null,
      matchesBody: false,
    };
  }
  return {
    tag: active.tagName,
    searchOpen: active.hasAttribute('data-search-open'),
    inHeader: Boolean(active.closest('.y-header')),
    inMain: Boolean(active.closest('#main-content')),
    inProse: Boolean(active.closest('.y-prose')),
    href: active.getAttribute('href'),
    matchesBody: active.matches(bodyLink),
  };
}, BODY_LINK);

const dialogState = (page) => page.locator(DIALOG).evaluate((dialog) => ({
  open: dialog.open,
  modal: dialog.matches(':modal'),
}));

const matches = (page, selector) => page.evaluate(
  (sel) => document.activeElement instanceof HTMLElement && document.activeElement.matches(sel),
  selector,
);

const tabUntil = async (page, selector, label) => {
  for (let i = 0; i < TAB_CAP; i += 1) {
    await page.keyboard.press('Tab');
    if (await matches(page, selector)) return;
  }
  broken(`Tab never reached ${label} within ${TAB_CAP} presses`);
};

const skipToMainThenBodyLink = async (page) => {
  await page.keyboard.press('Tab');
  if (!(await matches(page, 'a.y-skiplink'))) {
    broken('the first Tab did not land on the skip link, so this is not an ordinary keyboard arrival');
  }
  await page.keyboard.press('Enter');
  await frames(page);
  if (!(await matches(page, '#main-content'))) {
    broken('activating the skip link did not focus main, so later Tabs are not walking the article');
  }
  await tabUntil(page, BODY_LINK, 'the authored body wikilink');
};

const openDialog = async (page, chord) => {
  await page.keyboard.press(chord);
  await page.locator(`${DIALOG}[open]`).waitFor({ state: 'visible', timeout: 3000 });
  const opened = await dialogState(page);
  if (!opened.open || !opened.modal) {
    broken(`the dialog opened as open=${opened.open} modal=${opened.modal}, want a native modal`);
  }
};

const closeDialog = async (page, chord) => {
  await page.keyboard.press(chord);
  // Read on the same turn the close key is handled so a scripted header
  // landing is observed before anything else can move focus.
  const closed = await dialogState(page);
  if (closed.open || closed.modal) {
    broken(`after ${chord} the dialog was still open=${closed.open} modal=${closed.modal}`);
  }
};

const start = async (browser, sites) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  const proof = await arm(page, sites);
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');
  proveApplied(proof);
  if (await page.locator(BODY_LINK).count() !== 1) {
    broken(`the fixture has ${await page.locator(BODY_LINK).count()} body wikilinks to Beta, want 1`);
  }
  if (await page.locator(SEARCH_OPEN).count() !== 1) {
    broken(`the page has ${await page.locator(SEARCH_OPEN).count()} header search controls, want 1`);
  }
  return { context, page };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  {
    const sites = ['escape-restores-body-opener', 'tab-continues-from-restored'];
    const { context, page } = await start(browser, sites);
    try {
      await skipToMainThenBodyLink(page);
      await openDialog(page, 'ControlOrMeta+k');
      await closeDialog(page, 'Escape');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'escape-restores-body-opener',
          `Escape left focus on ${restored.tag} href=${JSON.stringify(restored.href)} searchOpen=${restored.searchOpen} inHeader=${restored.inHeader}, want the authored body wikilink`,
        );
      }

      // The restore claim is already locked. The dialog has to leave the top
      // layer before a Tab can walk the page it covered; that wait is after
      // the Escape assertion so a scripted header landing cannot hide.
      await frames(page);
      await page.keyboard.press('Tab');
      const next = await focusInfo(page);
      if (next.searchOpen || next.inHeader) {
        fail(
          'tab-continues-from-restored',
          `Tab after Escape walked into the header (${next.tag} href=${JSON.stringify(next.href)}), so reading did not continue from the restored body link`,
        );
      }
      if (next.tag === 'BODY' || !next.inMain) {
        fail(
          'tab-continues-from-restored',
          `Tab after Escape left the article (${next.tag} href=${JSON.stringify(next.href)} inMain=${next.inMain}), so reading did not continue from the restored body link`,
        );
      }
      if (next.matchesBody) {
        fail('tab-continues-from-restored', 'Tab after Escape stayed on the body wikilink, so the walk did not continue');
      }
    } finally {
      await context.close();
    }
  }

  {
    const sites = ['shortcut-restores-body-opener'];
    const { context, page } = await start(browser, sites);
    try {
      await skipToMainThenBodyLink(page);
      await openDialog(page, 'ControlOrMeta+k');
      await closeDialog(page, 'ControlOrMeta+k');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'shortcut-restores-body-opener',
          `Ctrl+K close left focus on ${restored.tag} href=${JSON.stringify(restored.href)} searchOpen=${restored.searchOpen} inHeader=${restored.inHeader}, want the authored body wikilink`,
        );
      }
    } finally {
      await context.close();
    }
  }

  {
    const sites = ['header-open-returns-to-header'];
    const { context, page } = await start(browser, sites);
    try {
      await tabUntil(page, SEARCH_OPEN, 'the header search control');
      await openDialog(page, 'Enter');
      await closeDialog(page, 'Escape');
      const restored = await focusInfo(page);
      if (!restored.searchOpen) {
        fail(
          'header-open-returns-to-header',
          `closing a header-opened dialog left focus on ${restored.tag} href=${JSON.stringify(restored.href)} inProse=${restored.inProse}, want [data-search-open]`,
        );
      }
    } finally {
      await context.close();
    }
  }

  console.log('PASS search-focus-restore: Escape and Ctrl+K return to the body opener; header open still returns to the header; Tab continues from the restored element');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const targets = targetsOf(MUTATIONS[MUTATE]);
      if (targets.includes(err.site)) {
        console.error(`caught at ${err.site}`);
        console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      } else {
        console.error(`no catch: ${MUTATE} targets ${targets.join(', ')}, but ${err.site} fired first`);
      }
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
