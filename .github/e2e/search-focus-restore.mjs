// Behavior lock: leaving search returns focus to whoever held it when the
// reader went into it. Escape used to land on the header search control after
// dialog.close(), so a reader who opened from a body link found themselves in
// the chrome; Ctrl+K toggle-close already restored.
//
// The search has two shapes now (D94). Above a phone's width it is a field in
// the header: Ctrl+K puts the reader in it, the first Escape puts away the list
// that drops under it and leaves them typing, and the second Escape — or Ctrl+K
// again — takes them back to where they were. On a phone it is the glass and
// the dialog it opens over the whole screen, whose return is the platform's.
// Both shapes are asked the same questions.
//
// The lock drives real Tab / Enter / Ctrl+K / Escape. Substituting
// element.focus() for those keystrokes is exactly what lets this class of test
// pass while the product stays broken.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with an authored body wikilink), MUTATE.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const DIALOG = '[data-search]';
const SEARCH_OPEN = '[data-search-open]';
const FIELD = '.y-searchfield [data-live-search-input]';
const LIST = '.y-searchfield [data-live-search-panel]';
const BODY_LINK = '.y-prose a.wikilink[href="/notes/Notes/beta.md"]';
const TAB_CAP = 30;
// The widths each shape is drawn at: the field above a phone's, the dialog on one.
const WIDE = 1280;
const PHONE = 390;

const SITES = [
  'first-escape-keeps-the-field',
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
  await page.route('**/search.js{,?*}', async (route) => {
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

// The line the dialog's close is on, and the lines the field is left on.
const DIALOG_CLOSE = "    if (dialog?.open) dialog.close();\n  }";
const FIELD_RETURN = '    if (back?.isConnected) back.focus();\n    else field.input.blur();';
const STEAL_NEXT_TAB = "\n    document.addEventListener('keydown', (event) => {\n      if (event.key === 'Tab') {\n        event.preventDefault();\n        document.querySelector('[data-search-open], .y-searchfield input')?.focus();\n      }\n    }, { once: true });";

// Each mutation names the shape it breaks, and is armed only where that shape
// is drawn, so a mutation of the dialog is never reported as caught or missed
// by a run that only asked the field.
const MUTATIONS = {
  // The original defect: after dialog.close(), focus is forced onto the
  // header control, overriding the platform's return.
  'focus-header-on-escape': {
    target: 'escape-restores-body-opener',
    shape: 'dialog',
    apply: rewriteSearch(
      DIALOG_CLOSE,
      "    if (dialog?.open) dialog.close();\n    document.querySelector('[data-search-open]')?.focus();\n  }",
    ),
  },
  'focus-header-on-shortcut-close': {
    target: 'shortcut-restores-body-opener',
    shape: 'dialog',
    apply: rewriteSearch(
      '    if (dialog.open) dialog.close();',
      '    if (dialog.open) { dialog.close(); document.querySelector(\'[data-search-open]\')?.focus(); }',
    ),
  },
  'open-as-if-the-body-link-held-focus': {
    target: 'header-open-returns-to-header',
    shape: 'dialog',
    apply: rewriteSearch(
      // The header press is performed by the browser now, so there is no line
      // in this module between the press and the dialog for a regression to
      // sit on. The browser announces the press on the dialog before it acts
      // on it, which is the last moment focus can still be moved — so that is
      // where a page that helpfully moved focus would do it, and where the
      // element the platform remembers to return to gets replaced.
      "  const dialog = document.querySelector('dialog[data-search]');",
      "  const dialog = document.querySelector('dialog[data-search]');\n  dialog?.addEventListener('command', () => { document.querySelector('.y-prose a.wikilink')?.focus(); });",
    ),
  },
  // Keeps the Escape landing correct and steals the next Tab into the header,
  // so tab-continues-from-restored is watched red on its own.
  'next-tab-jumps-header': {
    target: 'tab-continues-from-restored',
    shape: 'dialog',
    apply: rewriteSearch(DIALOG_CLOSE, `    if (dialog?.open) dialog.close();${STEAL_NEXT_TAB}\n  }`),
  },
  // The field forgets where the reader came from and lets focus fall to the
  // page's body, the field's form of the original defect.
  'drop-the-reader-on-the-body': {
    target: 'escape-restores-body-opener',
    shape: 'field',
    apply: rewriteSearch(FIELD_RETURN, '    field.input.blur();'),
  },
  // The field leaves for the header's own search entrance instead.
  'leave-the-field-for-the-header': {
    target: 'escape-restores-body-opener',
    shape: 'field',
    apply: rewriteSearch(FIELD_RETURN, "    document.querySelector('.y-header a')?.focus();"),
  },
  // The chord, pressed again in the field, only blurs it.
  'blur-the-field-on-the-second-chord': {
    target: 'shortcut-restores-body-opener',
    shape: 'field',
    apply: rewriteSearch(
      '      if (document.activeElement === field.input) leaveField(field);',
      '      if (document.activeElement === field.input) field.input.blur();',
    ),
  },
  // The return is right and the next Tab is stolen into the header.
  'next-tab-jumps-into-the-header-from-the-field': {
    target: 'tab-continues-from-restored',
    shape: 'field',
    apply: rewriteSearch(FIELD_RETURN, `${FIELD_RETURN}${STEAL_NEXT_TAB}`),
  },
  // The first Escape takes the reader out of the field along with the list,
  // so one who only wanted the list gone has lost their place in what they
  // were typing.
  'leave-the-field-on-the-first-escape': {
    target: 'first-escape-keeps-the-field',
    shape: 'field',
    apply: rewriteSearch('      field.panel.hidden = true;\n      return;', '      leaveField(field);\n      return;'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  for (const site of targetsOf(mutation)) {
    if (!SITES.includes(site)) {
      console.error(`search-focus-restore: mutation ${name} aims at unknown site ${site}`);
      process.exit(2);
    }
  }
  if (!['field', 'dialog'].includes(mutation.shape)) {
    console.error(`search-focus-restore: mutation ${name} names no shape it breaks`);
    process.exit(2);
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

const arm = async (page, sites, shape) => {
  if (!MUTATE) return null;
  const mutation = MUTATIONS[MUTATE];
  if (mutation.shape !== shape) return null;
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

const focusInfo = (page) => page.evaluate(({ bodyLink, field }) => {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement)) {
    return {
      tag: active?.tagName ?? 'none',
      searchOpen: false,
      inField: false,
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
    inField: active.matches(field),
    inHeader: Boolean(active.closest('.y-header')),
    inMain: Boolean(active.closest('#_y-main')),
    inProse: Boolean(active.closest('.y-prose')),
    href: active.getAttribute('href'),
    matchesBody: active.matches(bodyLink),
  };
}, { bodyLink: BODY_LINK, field: FIELD });

const describe = (info) => `${info.tag} href=${JSON.stringify(info.href)} searchOpen=${info.searchOpen} inField=${info.inField} inHeader=${info.inHeader}`;

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
  if (!(await matches(page, '#_y-main'))) {
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

// The field is entered by the chord and left by whatever the case asks. The
// list is opened by typing, so the first Escape has a list to put away.
const enterField = async (page) => {
  await page.keyboard.press('ControlOrMeta+k');
  if (!(await matches(page, FIELD))) broken('the chord did not put the reader in the header\'s search field, so leaving it asks nothing');
  if (await page.locator(`${DIALOG}[open]`).count() !== 0) broken('the chord opened the dialog at a width that draws the field');
};

const openList = async (page) => {
  await page.keyboard.type('alpha', { delay: 20 });
  await page.locator(LIST).waitFor({ state: 'visible', timeout: 4000 });
};

const start = async (browser, sites, shape) => {
  const width = shape === 'field' ? WIDE : PHONE;
  const context = await browser.newContext({ viewport: { width, height: 800 } });
  const page = await context.newPage();
  const proof = await arm(page, sites, shape);
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');
  proveApplied(proof);
  if (await page.locator(BODY_LINK).count() !== 1) {
    broken(`the fixture has ${await page.locator(BODY_LINK).count()} body wikilinks to Beta, want 1`);
  }
  const entrance = shape === 'field' ? FIELD : SEARCH_OPEN;
  if (await page.locator(entrance).count() !== 1 || !(await page.locator(entrance).isVisible())) {
    broken(`at ${width}px the page draws no single ${entrance}, the search's ${shape} shape`);
  }
  return { context, page };
};

// After the return, the next Tab walks on through the article from where the
// reader was, not into the header.
const askTheNextTab = async (page, how) => {
  await frames(page);
  await page.keyboard.press('Tab');
  const next = await focusInfo(page);
  if (next.searchOpen || next.inHeader) {
    fail(
      'tab-continues-from-restored',
      `${how}, Tab walked into the header (${describe(next)}), so reading did not continue from the restored body link`,
    );
  }
  if (next.tag === 'BODY' || !next.inMain) {
    fail(
      'tab-continues-from-restored',
      `${how}, Tab left the article (${next.tag} href=${JSON.stringify(next.href)} inMain=${next.inMain}), so reading did not continue from the restored body link`,
    );
  }
  if (next.matchesBody) {
    fail('tab-continues-from-restored', `${how}, Tab stayed on the body wikilink, so the walk did not continue`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // --- The field, above a phone's width ---------------------------------
  {
    const sites = ['first-escape-keeps-the-field', 'escape-restores-body-opener', 'tab-continues-from-restored'];
    const { context, page } = await start(browser, sites, 'field');
    try {
      await skipToMainThenBodyLink(page);
      await enterField(page);
      await openList(page);
      // The first Escape puts the list away and nothing else: the reader is
      // still in the field, with what they typed.
      await page.keyboard.press('Escape');
      const first = await focusInfo(page);
      const listHidden = await page.locator(LIST).isHidden();
      if (!listHidden || !first.inField) {
        fail(
          'first-escape-keeps-the-field',
          `the first Escape left the list ${listHidden ? 'put away' : 'open'} and focus on ${describe(first)}, want the list put away and the reader still in the field`,
        );
      }
      await page.keyboard.press('Escape');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'escape-restores-body-opener',
          `the second Escape left the field for ${describe(restored)}, want the authored body wikilink the chord was pressed on`,
        );
      }
      await askTheNextTab(page, 'after leaving the field by Escape');
    } finally {
      await context.close();
    }
  }

  {
    const sites = ['shortcut-restores-body-opener'];
    const { context, page } = await start(browser, sites, 'field');
    try {
      await skipToMainThenBodyLink(page);
      await enterField(page);
      await openList(page);
      await page.keyboard.press('ControlOrMeta+k');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'shortcut-restores-body-opener',
          `Ctrl+K in the field left focus on ${describe(restored)}, want the authored body wikilink`,
        );
      }
      if (!(await page.locator(LIST).isHidden())) {
        fail('shortcut-restores-body-opener', 'Ctrl+K took the reader out of the field and left its list open behind them');
      }
    } finally {
      await context.close();
    }
  }

  // --- The dialog, on a phone -------------------------------------------
  {
    const sites = ['escape-restores-body-opener', 'tab-continues-from-restored'];
    const { context, page } = await start(browser, sites, 'dialog');
    try {
      await skipToMainThenBodyLink(page);
      await openDialog(page, 'ControlOrMeta+k');
      await closeDialog(page, 'Escape');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'escape-restores-body-opener',
          `Escape left focus on ${describe(restored)}, want the authored body wikilink`,
        );
      }

      // The restore claim is already locked. The dialog has to leave the top
      // layer before a Tab can walk the page it covered; that wait is after
      // the Escape assertion so a scripted header landing cannot hide.
      await askTheNextTab(page, 'after the dialog closed on Escape');
    } finally {
      await context.close();
    }
  }

  {
    const sites = ['shortcut-restores-body-opener'];
    const { context, page } = await start(browser, sites, 'dialog');
    try {
      await skipToMainThenBodyLink(page);
      await openDialog(page, 'ControlOrMeta+k');
      await closeDialog(page, 'ControlOrMeta+k');
      const restored = await focusInfo(page);
      if (!restored.matchesBody) {
        fail(
          'shortcut-restores-body-opener',
          `Ctrl+K close left focus on ${describe(restored)}, want the authored body wikilink`,
        );
      }
    } finally {
      await context.close();
    }
  }

  {
    const sites = ['header-open-returns-to-header'];
    const { context, page } = await start(browser, sites, 'dialog');
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

  console.log('PASS search-focus-restore: in the header\'s field the first Escape puts the list away and keeps the reader typing, and the second Escape or Ctrl+K returns them to the body link they came from; on a phone Escape and Ctrl+K close the dialog onto the body opener and a header-opened dialog returns to the header; Tab continues from the restored element in both shapes');
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
