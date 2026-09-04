// Behavior lock for the narrow navigation drawer's progressive-enhancement
// and focus contracts. At 800px it exercises two different products:
//
//   - JavaScript off: the sidebar remains in normal flow and its links really
//     navigate; the hamburger has no face and cannot receive focus.
//   - JavaScript on: the closed rail is inert and absent from sequential focus;
//     opening moves focus into it; Tab and Shift+Tab stay inside; filter Escape
//     clears before drawer Escape closes; every close path returns focus to the
//     hamburger.
//
// MUTATE names one self-test mode below; MUTATE=list prints them. Each mode is
// installed only on the browser case carrying its target assertion, proves its
// rewrite reached that case, and may print a caught marker only when that exact
// assertion fires.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const RAIL = '#nav-rail';
const TOGGLE = '[data-nav-toggle]';
const SCRIM = '[data-nav-close]';
const FILTER = '[data-nav-filter]';
const SKIP_LINK = '.y-skiplink';
const MAIN = '#main-content';
// The status write bar is a sibling of the shell, not part of the article, and it
// is visible at exactly the widths where the drawer exists. It paints under the
// scrim, so it belongs in the same enumeration as the other covered regions.
const SEALBAR = '.y-sealbar';

const SITES = [
  'server-nav-state-free',
  'no-js-sidebar-navigation',
  'no-js-hamburger-hidden',
  'closed-rail-inert',
  'closed-background-live',
  'open-background-inert',
  'focused-skip-link-clears-toggle',
  'open-focus-entry',
  'open-tab-contained',
  'scrim-focus-return',
  'filter-escape-layering',
  'empty-filter-escape-closes-the-drawer',
  'composing-enter-stays-with-the-input',
  'escape-focus-return',
  'toggle-focus-return',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, msg) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN drawer-contract: an assertion names the unknown site ${site}`);
  throw new LockFired(site, `FAIL drawer-contract: ${msg}`);
};
const broken = (msg) => { throw new ProbeBroken(`BROKEN drawer-contract: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED drawer-contract: ${msg}`); };

const injectDocumentStyle = (css) => async (page) => {
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const needle = '</head>';
    const count = original.split(needle).length - 1;
    matches += count;
    const body = original.replaceAll(needle, `<style data-e2e-drawer-mutation>${css}</style>${needle}`);
    return route.fulfill({ response, body });
  });
  return () => matches === 1 ? '' : `document needle matched ${matches} times, want exactly 1`;
};

const rewriteDocument = (needle, replacement) => async (page) => {
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    const body = original.replaceAll(needle, replacement);
    return route.fulfill({ response, body });
  });
  return () => matches === 1 ? '' : `document needle matched ${matches} times, want exactly 1`;
};

const rewriteRuntime = (moduleName, needle, replacement) => async (page) => {
  let matches = 0;
  let requests = 0;
  await page.route(`**/${moduleName}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    const body = original.replaceAll(needle, replacement);
    return route.fulfill({ response, body });
  });
  return () => {
    if (requests !== 1) return `${moduleName} was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${moduleName} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  // The background is never taken out of the tree.
  'never-inert-background': {
    target: 'open-background-inert',
    apply: rewriteRuntime('drawer.js', 'for (const region of background) region.inert = modal;', 'for (const region of background) region.inert = false;'),
  },
  // It is taken out and never put back.
  'never-restore-background': {
    target: 'closed-background-live',
    apply: rewriteRuntime('drawer.js', 'for (const region of background) region.inert = modal;', 'for (const region of background) region.inert = true;'),
  },
  'stamp-server-nav-state': {
    target: 'server-nav-state-free',
    apply: rewriteDocument('<html ', '<html data-nav="closed" '),
  },
  'hide-no-js-sidebar': {
    target: 'no-js-sidebar-navigation',
    apply: injectDocumentStyle('@media (max-width:900px){.y-rail-left{position:fixed!important;transform:translateX(-102%)!important}}'),
  },
  'show-no-js-hamburger': {
    target: 'no-js-hamburger-hidden',
    apply: injectDocumentStyle('@media (max-width:900px){.y-hamburger{display:inline-flex!important}}'),
  },
  'suppress-closed-inert': {
    target: 'closed-rail-inert',
    apply: rewriteRuntime('drawer.js', '    rail.inert = !open;', '    rail.inert = false;'),
  },
  'overlap-toggle-with-focused-skip-link': {
    target: 'focused-skip-link-clears-toggle',
    apply: injectDocumentStyle('@media (max-width:900px){.y-skiplink{top:8px!important}}'),
  },
  'suppress-open-focus': {
    target: 'open-focus-entry',
    apply: rewriteRuntime('drawer.js', '    focusFirst();', '    void 0;'),
  },
  'suppress-tab-containment': {
    target: 'open-tab-contained',
    apply: rewriteRuntime('drawer.js', "    if (!isOpen() || event.key !== 'Tab') return;", '    if (true) return;'),
  },
  'suppress-scrim-focus-return': {
    target: 'scrim-focus-return',
    apply: rewriteRuntime(
      'drawer.js',
      "  document.querySelector('[data-nav-close]')?.addEventListener('click', closeAndRestoreFocus);",
      "  document.querySelector('[data-nav-close]')?.addEventListener('click', () => setOpen(false));",
    ),
  },
  'suppress-filter-escape-layer': {
    target: 'filter-escape-layering',
    apply: rewriteRuntime('sidebar.js', '      event.stopPropagation();', '      void 0;'),
  },
  // The filter reading its keys while an input method is still composing with
  // them, which is how Enter stopped meaning "commit this word".
  'read-filter-keys-while-composing': {
    target: 'composing-enter-stays-with-the-input',
    apply: rewriteRuntime('sidebar.js', '    if (event.isComposing) return;\n', ''),
  },
  // The filter answering an Escape it has nothing to answer with, which is
  // what made the drawer's own exit key need pressing twice.
  'swallow-escape-from-an-empty-filter': {
    target: 'empty-filter-escape-closes-the-drawer',
    apply: rewriteRuntime('sidebar.js', '      if (!input.value.trim()) return;\n', ''),
  },
  // The same predicate spelled the other way, which is how a box of spaces
  // kept costing a second press after the empty one had stopped.
  'test-emptiness-without-trimming': {
    target: 'empty-filter-escape-closes-the-drawer',
    apply: rewriteRuntime('sidebar.js', '      if (!input.value.trim()) return;', '      if (!input.value) return;'),
  },
  'suppress-escape-focus-return': {
    target: 'escape-focus-return',
    apply: rewriteRuntime(
      'shortcuts.js',
      '      if (drawer.isOpen()) drawer.closeAndRestoreFocus();',
      "      if (drawer.isOpen()) document.documentElement.dataset.nav = 'closed';",
    ),
  },
  'suppress-toggle-focus-return': {
    target: 'toggle-focus-return',
    apply: rewriteRuntime(
      'drawer.js',
      '    if (isOpen()) closeAndRestoreFocus();',
      '    if (isOpen()) setOpen(false);',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`drawer-contract: mutation ${name} aims at the unknown assertion site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`drawer-contract: the ${site} assertion is aimed at by no mutation, so nothing shows it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`drawer-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const arm = async (page, sites) => {
  if (!MUTATE) return null;
  const mutation = MUTATIONS[MUTATE];
  if (!sites.includes(mutation.target)) return null;
  return mutation.apply(page);
};

const proveApplied = (proof) => {
  if (!proof) return;
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const activeInsideRail = (page) => page.$eval(RAIL, (rail) => rail.contains(document.activeElement));
const toggleFocused = (page) => page.$eval(TOGGLE, (toggle) => document.activeElement === toggle);

const waitForNav = (page, state) => page.waitForFunction(
  (want) => document.documentElement.dataset.nav === want,
  state,
);

const openDrawer = async (page) => {
  await page.locator(TOGGLE).click();
  await waitForNav(page, 'open');
};

// The toggle toggles, so opening a drawer that is already open closes it. The
// cases that leave it closed are followed by ones that need it open, and the
// first of those inherits whatever the case before it left behind.
const ensureDrawerOpen = async (page) => {
  if (await page.evaluate(() => document.documentElement.dataset.nav === 'open')) return;
  await openDrawer(page);
};

const closeState = async (page, site, path) => {
  await waitForNav(page, 'closed');
  if (!(await toggleFocused(page))) fail(site, `${path} closed the drawer without returning focus to its trigger`);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Case 1: the server-rendered page remains navigable at 800px without any
  // client script. The click at the end is the observable: an on-screen link
  // actually performs a navigation, rather than merely existing in the DOM.
  {
    const sites = ['server-nav-state-free', 'no-js-sidebar-navigation', 'no-js-hamburger-hidden'];
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 800, height: 800 } });
    const page = await context.newPage();
    const proof = await arm(page, sites);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveApplied(proof);

    if (await page.$eval('html', (root) => root.hasAttribute('data-nav'))) {
      fail('server-nav-state-free', 'server markup stamped data-nav even though only the enhancement runtime owns drawer state');
    }
    if (!(await page.$(RAIL))) broken('the no-JS page has no #nav-rail');
    const link = page.locator(`${RAIL} a[href]:not([aria-current="page"])`).first();
    if (await link.count() !== 1) broken('the no-JS rail has no non-current navigation link to exercise');
    const hit = await link.evaluate((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) return { visible: false, hit: false };
      const x = rect.left + rect.width / 2;
      const y = rect.top + rect.height / 2;
      const atPoint = document.elementFromPoint(x, y);
      return {
        visible: rect.right > 0 && rect.left < innerWidth && rect.bottom > 0 && rect.top < innerHeight,
        hit: Boolean(atPoint && (atPoint === el || el.contains(atPoint))),
      };
    });
    if (!hit.visible || !hit.hit) {
      fail('no-js-sidebar-navigation', `sidebar link is not on-screen and clickable at 800px (visible=${hit.visible}, hit=${hit.hit})`);
    }

    const hamburger = await page.$eval(TOGGLE, (toggle) => {
      toggle.focus();
      return {
        display: getComputedStyle(toggle).display,
        boxes: toggle.getClientRects().length,
        focused: document.activeElement === toggle,
      };
    });
    if (hamburger.display !== 'none' || hamburger.boxes !== 0 || hamburger.focused) {
      fail('no-js-hamburger-hidden', `dead hamburger is exposed (display=${hamburger.display}, boxes=${hamburger.boxes}, focused=${hamburger.focused})`);
    }

    const before = page.url();
    await link.click();
    await page.waitForLoadState('domcontentloaded');
    if (page.url() === before) fail('no-js-sidebar-navigation', 'clicking the visible sidebar link did not navigate');
    await context.close();
  }

  // Case 2: the enhanced drawer owns the full three-state focus lifecycle —
  // closed (inert), open (focus inside and contained), closed again (trigger).
  {
    const sites = SITES.filter((site) => !site.startsWith('no-js-'));
    const page = await browser.newPage({ viewport: { width: 800, height: 800 } });
    const proof = await arm(page, sites);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js][data-nav="closed"]');
    proveApplied(proof);

    const closed = await page.$eval(RAIL, (rail) => ({ inert: rail.inert, ariaHidden: rail.getAttribute('aria-hidden') }));
    if (!closed.inert || closed.ariaHidden !== 'true') {
      fail('closed-rail-inert', `closed rail state is inert=${closed.inert}, aria-hidden=${closed.ariaHidden}, want true/true`);
    }
    // The page behind a closed drawer is the page. Leaving it inert after the
    // drawer shuts would lock the reader out of everything with nothing on
    // screen to say why, so the live case is asserted before the modal one.
    const shell = await page.evaluate(({ main, skip, seal }) => {
      const found = {
        main: document.querySelectorAll(main).length,
        skip: document.querySelectorAll(skip).length,
        seal: document.querySelectorAll(seal).length,
      };
      return { ...found, inert: [...document.querySelectorAll(`${main}, ${skip}, ${seal}`)].map((element) => element.inert) };
    }, { main: MAIN, skip: SKIP_LINK, seal: SEALBAR });
    // An absent region would make every check below vacuously true, so its
    // absence is a broken probe rather than a passing page.
    if (shell.main !== 1 || shell.skip !== 1 || shell.seal !== 1) {
      broken(`the page has ${shell.main} main landmarks, ${shell.skip} skip links and ${shell.seal} status bars, want 1 of each`);
    }
    if (shell.inert.some(Boolean)) {
      fail('closed-background-live', `a closed drawer leaves the page inert: ${JSON.stringify(shell.inert)}`);
    }
    for (let i = 0; i < 20; i += 1) {
      await page.keyboard.press('Tab');
      if (await activeInsideRail(page)) fail('closed-rail-inert', `Tab ${i + 1} entered the closed rail`);
    }

    await page.locator(SKIP_LINK).focus();
    const toggleHit = await page.$eval(TOGGLE, (toggle) => {
      const rect = toggle.getBoundingClientRect();
      const atPoint = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
      return atPoint === toggle || toggle.contains(atPoint);
    });
    if (!toggleHit) {
      fail('focused-skip-link-clears-toggle', 'the focused skip link covers the navigation trigger');
    }

    await openDrawer(page);
    const opened = await page.$eval(RAIL, (rail) => ({
      inert: rail.inert,
      ariaHidden: rail.getAttribute('aria-hidden'),
      focusedInside: rail.contains(document.activeElement),
    }));
    if (opened.inert || opened.ariaHidden !== null || !opened.focusedInside) {
      fail('open-focus-entry', `open rail state is inert=${opened.inert}, aria-hidden=${opened.ariaHidden}, focusedInside=${opened.focusedInside}`);
    }
    // The scrim dims the article; only this takes it out of the tree. Without
    // it a reading cursor walks straight into the page the curtain covers,
    // which is the one thing the curtain is there to prevent.
    const behind = await page.evaluate(({ main, skip, seal }) => (
      [...document.querySelectorAll(`${main}, ${skip}, ${seal}`)].map((element) => element.inert)
    ), { main: MAIN, skip: SKIP_LINK, seal: SEALBAR });
    // The expected count comes from the closed-state tally rather than from
    // behind itself: comparing a length to its own length can never fail, and
    // deriving it also catches a region that disappears between the two states.
    if (behind.length !== shell.main + shell.skip + shell.seal || !behind.every(Boolean)) {
      fail('open-background-inert', `an open drawer leaves the page in the tree: ${JSON.stringify(behind)}`);
    }

    const boundaryCount = await page.$eval(RAIL, (rail) => {
      const focusable = [...rail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
        .filter((el) => !el.hidden && el.getClientRects().length > 0);
      if (focusable.length < 2) return focusable.length;
      focusable.at(-1).focus();
      return focusable.length;
    });
    if (boundaryCount < 2) broken('the open rail has fewer than two visible focus targets, so its Tab boundary cannot be exercised');
    await page.keyboard.press('Tab');
    if (!(await activeInsideRail(page))) fail('open-tab-contained', 'Tab from the last rail control escaped behind the scrim');
    await page.$eval(RAIL, (rail) => {
      const focusable = [...rail.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
        .filter((el) => !el.hidden && el.getClientRects().length > 0);
      focusable[0].focus();
    });
    await page.keyboard.press('Shift+Tab');
    if (!(await activeInsideRail(page))) fail('open-tab-contained', 'Shift+Tab from the first rail control escaped behind the scrim');

    await page.locator(SCRIM).click({ position: { x: 760, y: 20 } });
    await closeState(page, 'scrim-focus-return', 'scrim click');

    await openDrawer(page);
    const filter = page.locator(FILTER);
    await filter.fill('alpha');
    await filter.press('Escape');
    const layered = await page.evaluate(({ filterSelector }) => ({
      nav: document.documentElement.dataset.nav,
      value: document.querySelector(filterSelector)?.value,
    }), { filterSelector: FILTER });
    if (layered.nav !== 'open' || layered.value !== '') {
      fail('filter-escape-layering', `first filter Escape left nav=${layered.nav}, value=${JSON.stringify(layered.value)}, want open/empty`);
    }

    // A composing input method commits its word with Enter. The filter taking
    // that key opened the first row it had narrowed to and the half-typed word
    // went with it. The composition flag is set on the event rather than
    // driven by a real input method, which no headless run has, so what is
    // exercised is the guard and not the platform.
    //
    // What is counted is the row activation, not a navigation: the handler
    // activates a link synchronously, so the count answers exactly what the
    // handler decided, and the listener doing the counting also holds the page
    // still enough to ask the same question twice. The second question is what
    // keeps the first honest — a filter that had stopped answering Enter
    // altogether would satisfy the composing case just as well.
    await filter.fill('alpha');
    const enter = await page.evaluate(({ railSelector, filterSelector }) => {
      const rail = document.querySelector(railSelector);
      const input = document.querySelector(filterSelector);
      let hits = 0;
      const record = (event) => { event.preventDefault(); hits += 1; };
      rail.addEventListener('click', record, true);
      const send = (composing) => input.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'Enter', bubbles: true, cancelable: true, isComposing: composing,
      }));
      send(true);
      const whileComposing = hits;
      send(false);
      const afterComposing = hits - whileComposing;
      rail.removeEventListener('click', record, true);
      return { whileComposing, afterComposing };
    }, { railSelector: RAIL, filterSelector: FILTER });
    if (enter.whileComposing !== 0 || enter.afterComposing !== 1) {
      fail('composing-enter-stays-with-the-input', `filter Enter opened ${enter.whileComposing} rows while composing and ${enter.afterComposing} after, want 0 then 1`);
    }
    // An empty filter has nothing to clear, so its Escape is the drawer's and
    // one press is enough. Answering the key anyway cost the reader a second
    // press for no visible change, and the first was the one they would
    // describe as having done nothing. Only the count of presses is asked
    // here: where focus lands afterwards is the next case's contract, and
    // asking it twice would let this site fire on a focus regression and rob
    // that case of the self-test aimed at it. The state is read straight
    // after the press rather than waited for, because the close happens
    // inside the handler — and because a wait that timed out would end the
    // run without the named failure a self-test needs to report a catch.
    //
    // A box holding only spaces is asked as well, because it is empty to the
    // reader and to the filtering: it hides no row, so an Escape answered
    // there is the same press spent on nothing. Two spellings of one
    // predicate in one file is how that came back.
    for (const [content, described] of [['', 'an empty filter'], ['   ', 'a filter holding only spaces']]) {
      await ensureDrawerOpen(page);
      await filter.fill(content);
      await filter.press('Escape');
      const closed = await page.evaluate(() => document.documentElement.dataset.nav);
      if (closed !== 'closed') {
        fail('empty-filter-escape-closes-the-drawer', `one Escape on ${described} left nav=${closed}, want closed`);
      }
    }

    await openDrawer(page);
    // The declined Escape leaves the spaces where they were, by design; they
    // are cleared here so the cases below start from an unnarrowed rail.
    await filter.fill('');
    await page.$eval(`${RAIL} a[href]`, (link) => link.focus());
    await page.keyboard.press('Escape');
    await closeState(page, 'escape-focus-return', 'Escape');

    await openDrawer(page);
    // HTMLElement.click() activates the real toggle handler without the test
    // driver first focusing the button. That isolates the handler's explicit
    // return-focus contract; a pointer click would make the assertion pass from
    // the browser's own pre-click focus even if closeDrawer stopped restoring.
    await page.$eval(TOGGLE, (toggle) => toggle.click());
    await closeState(page, 'toggle-focus-return', 'toggle click');
    await page.close();
  }

  console.log('PASS drawer-contract: no-JS navigation works; enhanced drawer focus is inert → contained → restored');
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
