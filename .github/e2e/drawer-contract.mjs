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
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
const RAIL = '#nav-rail';
const TOGGLE = '[data-nav-toggle]';
const SCRIM = '[data-nav-close]';
const FILTER = '[data-nav-filter]';

const SITES = [
  'server-nav-state-free',
  'no-js-sidebar-navigation',
  'no-js-hamburger-hidden',
  'closed-rail-inert',
  'open-focus-entry',
  'open-tab-contained',
  'scrim-focus-return',
  'filter-escape-layering',
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

const rewriteRuntime = (needle, replacement) => async (page) => {
  let matches = 0;
  let requests = 0;
  await page.route('**/yomihon.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    const body = original.replaceAll(needle, replacement);
    return route.fulfill({ response, body });
  });
  return () => {
    if (requests !== 1) return `runtime was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `runtime needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
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
    apply: rewriteRuntime('drawerRail.inert = !open;', 'drawerRail.inert = false;'),
  },
  'suppress-open-focus': {
    target: 'open-focus-entry',
    apply: rewriteRuntime('focusDrawer();', 'void 0;'),
  },
  'suppress-tab-containment': {
    target: 'open-tab-contained',
    apply: rewriteRuntime("if (!drawerOpen() || e.key !== 'Tab') return;", 'if (true) return;'),
  },
  'suppress-scrim-focus-return': {
    target: 'scrim-focus-return',
    apply: rewriteRuntime("document.querySelector('[data-nav-close]')?.addEventListener('click', () => {\n      closeDrawer(true);\n    });", "document.querySelector('[data-nav-close]')?.addEventListener('click', () => {\n      closeDrawer(false);\n    });"),
  },
  'suppress-filter-escape-layer': {
    target: 'filter-escape-layering',
    apply: rewriteRuntime('        e.stopPropagation();', '        void 0;'),
  },
  'suppress-escape-focus-return': {
    target: 'escape-focus-return',
    apply: rewriteRuntime("        if (drawerOpen()) closeDrawer(true);", "        if (drawerOpen()) closeDrawer(false);"),
  },
  'suppress-toggle-focus-return': {
    target: 'toggle-focus-return',
    apply: rewriteRuntime("    drawerToggle.addEventListener('click', () => {\n      if (drawerOpen()) { closeDrawer(true); } else { openDrawer(); }\n    });", "    drawerToggle.addEventListener('click', () => {\n      if (drawerOpen()) { closeDrawer(false); } else { openDrawer(); }\n    });"),
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
    for (let i = 0; i < 20; i += 1) {
      await page.keyboard.press('Tab');
      if (await activeInsideRail(page)) fail('closed-rail-inert', `Tab ${i + 1} entered the closed rail`);
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
