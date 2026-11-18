// Behavior lock for how the page's overlays are opened and closed: by the
// browser, because the markup says so, rather than by this site's script.
//
// A press that names what it opens is performed by the browser itself, so the
// search a phone's glass opens over the whole screen answers a reader whose
// browser is running none of our JavaScript. That is the claim, and the only
// way to ask it is with scripting switched off — a Go test reads markup and
// cannot tell an attribute the browser acts on from one it ignores. Above a
// phone's width the search is a field in the header (D94), an ordinary form
// that needs no script at all, and it is asked to search with none. The keys
// themselves answer only to script, and a reader without it finds them
// explained on the settings page.
//
// The concept sheet is the other half. Its opening stays with the script, for
// reasons the module says at the call; its closing does not, and the press
// that shuts it is asked here with the module's old listener gone.
//
// Last, the engine that has not shipped the markup form yet. It cannot be
// visited, so it is built: the attribute is taken off the press and the API
// is taken off the prototype before the page's own scripts run, which is what
// such an engine looks like from inside the module.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson carrying a concept term), and MUTATE.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

// Each press is found by something the markup carries for another reason — a
// class the stylesheet dresses, a hook the runtime already looks for — never by
// the attribute under test. A locator that named that attribute would stop
// matching the moment a mutation took it away, and a press that could not be
// found reads as a probe that timed out rather than as the lock it is.
const SEARCH_OPEN = '[data-search-open]';
const SEARCH = '[data-search]';
const SEARCH_FIELD = '.y-searchfield [data-live-search-input]';
// The search is the glass at this width and below, the field above it.
const PHONE = 720;
const KEYS_EXPLAINED = '.y-prefs__shortcuts .y-kbdhelp__list';
const SHEET = '[data-concept-sheet]';
const SHEET_CLOSE = '[data-concept-sheet] .y-conceptsheet__head button';
const CONCEPT = '[data-concept]';
// The header's one control that does nothing without script: the bookmark,
// which the stylesheet draws only on a page that marks itself as running script.
const SCRIPT_ONLY = ['.y-headermark [data-mark-button]'];
// The bookmark shares its rule with the copy of the path beside it, which
// waits for script for the same reason.
const BOOKMARK_RULE = /^\.y-headermark,\s*\.y-headerpath\s*\{\s*display:\s*none;/gm;

const SITES = [
  'script-only-header-controls-stay-out-of-the-way',
  'scriptless-preferences-remains-a-keyboard-destination',
  'the-palette-opens-with-no-script',
  'the-header-field-searches-with-no-script',
  'search-remains-reachable-with-neither-feature',
  'the-keys-are-explained-with-no-script',
  'the-sheet-closes-with-no-listener',
  'an-engine-without-commands-keeps-a-way-in',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN overlay-commands: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL overlay-commands: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN overlay-commands: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED overlay-commands: ${message}`); };

// rewriteMarkup takes an attribute off the served page. Renaming rather than
// deleting keeps the element's shape and the row's width the same, so what
// changes is only whether the browser acts on it. Counted per load, because a
// needle that has been rewritten since proves nothing about the page that was
// actually served.
const rewriteMarkup = (needle, replacement, label, transform = (body) => body) => async (page) => {
  const perLoad = [];
  await page.route(`${BASE}/**`, async (route) => {
    const request = route.request();
    if (request.resourceType() !== 'document') {
      await route.continue();
      return;
    }
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: transform(original.replaceAll(needle, replacement)) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: no page was served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

// rewriteModule injects a regression into the client module, for the one claim
// this file makes about script rather than about markup.
const rewriteModule = (needle, replacement, label) => async (page) => {
  const perLoad = [];
  await page.route('**/search.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    perLoad.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (perLoad.length === 0) return `${label}: the module was never served, so nothing was rewritten`;
    if (perLoad.some((hits) => hits !== 1)) {
      return `${label} needle matched [${perLoad.join(', ')}] across ${perLoad.length} loads, want exactly 1 in each`;
    }
    return '';
  };
};

// Each CSS regression also removes the native search target, so geometry is
// measured on the same scriptless capability pair as the destination check.
const distortScriptlessHeader = (selector, css) => async (page) => {
  const nativeProof = await rewriteMarkup(
    'commandfor="_y-search-dialog"', 'data-unsupported-commandfor="_y-search-dialog"',
    'the disabled native search invoker',
  )(page);
  const perLoad = [];
  await page.route('**/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    perLoad.push((body.match(selector) || []).length);
    await route.fulfill({ response, body: body + '\n' + css });
  });
  return () => nativeProof() || (perLoad.length === 0 || perLoad.some((hits) => hits !== 1)
    ? `scriptless header rule matched [${perLoad}]` : '');
};

// Draw the bookmark whatever the page is running, which is the shape the
// stylesheet would take if it forgot the bookmark depends on script.
const drawTheBookmarkAlways = async (page) => {
  const perLoad = [];
  await page.route('**/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    const hits = [...body.matchAll(BOOKMARK_RULE)].length;
    perLoad.push(hits);
    await route.fulfill({ response, body: hits === 1 ? body.replace(BOOKMARK_RULE, '.y-headermark,\n.y-headerpath { display: flex;') : body });
  });
  return () => perLoad.length === 0 || perLoad.some((hits) => hits !== 1)
    ? `bookmark rule matched [${perLoad}], want exactly 1 per load` : '';
};

const MUTATIONS = {
  'draw-the-bookmark-without-script': {
    target: 'script-only-header-controls-stay-out-of-the-way',
    apply: drawTheBookmarkAlways,
  },
  'take-away-the-scriptless-preferences-destination': {
    target: 'scriptless-preferences-remains-a-keyboard-destination',
    apply: rewriteMarkup(
      'class="y-prefslink" href="/preferences',
      'class="y-prefslink" href="/',
      'the native preferences destination',
    ),
  },
  'remove-the-scriptless-search-destination': {
    target: 'search-remains-reachable-with-neither-feature',
    apply: rewriteMarkup(
      'class="y-searchfallback" href="/search"',
      'class="y-searchfallback" href="/"',
      'the scriptless search link',
      (body) => body.replaceAll('commandfor=', 'data-unsupported-commandfor='),
    ),
  },
  'widen-the-scriptless-search-link': {
    target: 'search-remains-reachable-with-neither-feature',
    apply: distortScriptlessHeader(
      /^\.y-searchfallback\s*\{/gm,
      '.y-searchfallback { display: inline-block; min-width: 1500px; }',
    ),
  },
  'truncate-the-scriptless-wordmark': {
    target: 'search-remains-reachable-with-neither-feature',
    apply: distortScriptlessHeader(
      /^\.y-brand__word\s*\{/gm,
      '.y-brand__name > span { max-width: 1px; }',
    ),
  },
  // The press stops naming the palette, so opening it is back to being the
  // script's job — and with scripting off there is no job.
  'take-the-command-off-the-palette-press': {
    target: 'the-palette-opens-with-no-script',
    apply: rewriteMarkup(
      'command="show-modal" commandfor="_y-search-dialog"',
      'data-retired-command="show-modal"',
      'the command on the header press',
    ),
  },
  // The header's field stops naming the search page, so a reader with no
  // script who types in it and presses Enter is taken nowhere they asked.
  'send-the-header-field-elsewhere': {
    target: 'the-header-field-searches-with-no-script',
    apply: rewriteMarkup(
      'class="y-searchfield__form" method="get" action="/search"',
      'class="y-searchfield__form" method="get" action="/"',
      'the header field\'s form',
    ),
  },
  // The field is drawn only once script has run, as the bookmark is, which
  // leaves a reader with no script above a phone's width no way into search
  // in the row: the fallback link is a phone's.
  'draw-the-header-field-only-with-script': {
    target: 'search-remains-reachable-with-neither-feature',
    apply: distortScriptlessHeader(
      /^\.y-searchfield\s*\{/gm,
      'html:not([data-js]) .y-searchfield { display: none; }',
    ),
  },
  // The settings page stops listing the keys, so a reader whose browser runs
  // none of the script that answers them has nowhere to learn they exist.
  'hide-the-keys-on-the-settings-page': {
    target: 'the-keys-are-explained-with-no-script',
    apply: rewriteMarkup(
      '<section class="y-prefs__shortcuts">',
      '<section class="y-prefs__shortcuts" hidden>',
      'the keys on the settings page',
    ),
  },
  // The sheet's close press stops naming the sheet. Nothing listens for it any
  // more, so the press becomes a button that does nothing.
  'take-the-command-off-the-sheet-close': {
    target: 'the-sheet-closes-with-no-listener',
    apply: rewriteMarkup(
      'command="close" commandfor="_y-concept-sheet"',
      'data-retired-command="close"',
      'the command on the sheet close press',
    ),
  },
  // The module stops standing in for an engine that cannot perform the press
  // itself, which leaves such an engine with a control that does nothing.
  'drop-the-fallback-press': {
    target: 'an-engine-without-commands-keeps-a-way-in',
    apply: rewriteModule(
      "if (!('commandForElement' in HTMLButtonElement.prototype)) {",
      'if (false) {',
      'the question the fallback press is behind',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`overlay-commands: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`overlay-commands: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`overlay-commands: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const applyMutation = async (page, site) => {
  if (!MUTATE || MUTATIONS[MUTATE].target !== site) return () => '';
  return MUTATIONS[MUTATE].apply(page);
};
const checkProof = (proof) => {
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const isOpen = (page, selector) => page.$eval(selector, (element) => (
  element.tagName === 'DIALOG' ? element.open : element.matches(':popover-open')
));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Asked either side of the width the reading rail arrives at, where the
  // bookmark steps aside for the rail's own, and on a phone.
  for (const width of [1281, 1280, 390]) for (const language of ['zh-Hant', 'en']) for (const theme of ['light', 'dark']) {
    const site = 'script-only-header-controls-stay-out-of-the-way';
    const destinationSite = 'scriptless-preferences-remains-a-keyboard-destination';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width, height: 800 } });
    await context.addCookies([
      { name: 'yomihon_lang', value: language, url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
    ]);
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    const destinationProof = await applyMutation(page, destinationSite);
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (await page.$eval('html', (root) => root.hasAttribute('data-js'))) broken('script ran in the header capability fixture');
    if (await page.locator('html').getAttribute('lang') !== language) broken(`the capability fixture did not select ${language}`);
    if (await page.locator('html').getAttribute('data-theme') !== theme) broken(`the capability fixture did not select ${theme}`);
    for (const selector of SCRIPT_ONLY) {
      if (await page.locator(`header ${selector}`).count() !== 1) broken(`the header does not carry exactly one ${selector}`);
    }
    const controls = await page.evaluate((selectors) => selectors.map((selector) => {
      const button = document.querySelector(`header ${selector}`);
      button.focus();
      return { selector, drawn: button.getClientRects().length > 0, focused: button === document.activeElement };
    }), SCRIPT_ONLY);
    checkProof(proof);
    if (controls.some((control) => control.drawn || control.focused)) {
      fail(site, `${language}/${theme} at ${width}px exposes script-only controls: ${JSON.stringify(controls)}`);
    }
    const preferences = page.locator('header .y-prefslink');
    checkProof(destinationProof);
    // On a phone the link is drawn as its mark alone, so its name is the
    // label it carries rather than any text on screen.
    if (!(await preferences.isVisible()) || !((await preferences.getAttribute('aria-label')) ?? '').trim()
        || await preferences.evaluate((link) => new URL(link.href).pathname) !== '/preferences') {
      fail(destinationSite, `${language}/${theme} at ${width}px offers no visible, named native preferences destination`);
    }
    let reached = false;
    for (let step = 0; step < 24; step += 1) {
      await page.keyboard.press('Tab');
      const focused = await page.evaluate((selectors) => ({
        control: selectors.find((selector) => document.activeElement?.matches(selector)),
        preferences: document.activeElement?.matches('header .y-prefslink'),
      }), SCRIPT_ONLY);
      if (focused.control) fail(site, `Tab reached ${focused.control} with scripting disabled`);
      if (focused.preferences) { reached = true; break; }
    }
    if (!reached) fail(destinationSite, `${language}/${theme} at ${width}px: Tab never reached the native preferences link`);
    await Promise.all([
      page.waitForURL((url) => url.pathname !== new URL(BASE + PAGE).pathname),
      page.keyboard.press('Enter'),
    ]);
    if (new URL(page.url()).pathname !== '/preferences') fail(destinationSite, 'the native preferences link did not reach /preferences');
    await context.close();
  }

  // --- The palette, on a page running no script of ours -----------------
  //
  // At a phone's width, where the header draws the glass that opens it.
  {
    const site = 'the-palette-opens-with-no-script';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    // The document says whether any of our script ran. A page that had been
    // enhanced after all would answer this whole block for the wrong reason.
    if (await page.$eval('html', (root) => root.hasAttribute('data-js'))) {
      broken('the page reports a running enhancement runtime, so what a press does here says nothing about a browser with no script');
    }
    if (await page.locator(SEARCH).count() !== 1) broken(`${PAGE} carries no single palette`);
    if (await isOpen(page, SEARCH)) broken('the palette is already open before anything was pressed');
    await page.locator(SEARCH_OPEN).click();
    checkProof(proof);
    if (!(await isOpen(page, SEARCH))) {
      fail(site, 'pressing the header control on a page with no script left the palette closed');
    }
    // What the reader can do with it once it is there: the form inside is an
    // ordinary GET, which is the whole of the palette without live rows.
    const form = await page.$eval(`${SEARCH} [data-live-search-form]`, (element) => ({
      method: element.getAttribute('method'),
      action: new URL(element.action, location.href).pathname,
    }));
    if (form.method !== 'get' || form.action !== '/search') {
      fail(site, `the palette a reader with no script reaches submits ${JSON.stringify(form)}, want a get to /search`);
    }
    await context.close();
  }

  // --- The header's field, on a page running no script of ours ---------
  //
  // Above a phone's width the search is the field itself. Typed in and sent
  // with Enter, it is the same ordinary GET to /search the palette's form is.
  for (const width of [1280, 721]) {
    const site = 'the-header-field-searches-with-no-script';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (await page.$eval('html', (root) => root.hasAttribute('data-js'))) broken('script ran in the no-script fixture');
    const field = page.locator(`header ${SEARCH_FIELD}`);
    if (await field.count() !== 1) broken(`${PAGE} carries no single header search field`);
    if (!(await field.isVisible()) || !((await field.getAttribute('aria-label')) ?? '').trim()) {
      fail(site, `at ${width}px with no script the header's search field is not drawn with a name`);
    }
    await field.fill('alpha');
    await Promise.all([
      page.waitForURL((url) => url.pathname !== new URL(BASE + PAGE).pathname, { timeout: 4000 }).catch(() => {}),
      field.press('Enter'),
    ]);
    checkProof(proof);
    const landed = new URL(page.url());
    if (landed.pathname !== '/search' || landed.searchParams.get('q') !== 'alpha') {
      fail(site, `at ${width}px with no script, Enter in the header's field reached ${landed.pathname}${landed.search}, want /search?q=alpha`);
    }
    await context.close();
  }

  // No script and no invokers: the visible link is an ordinary keyboard route.
  // Removing the command attribute models the unsupported native behavior;
  // no injected script is needed or allowed in this context.
  for (const language of ['zh-Hant', 'en']) {
    const site = 'search-remains-reachable-with-neither-feature';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 375, height: 800 } });
    await context.addCookies([{ name: 'yomihon_lang', value: language, url: BASE }]);
    const page = await context.newPage();
    const proof = MUTATE && MUTATIONS[MUTATE].target === site
      ? await applyMutation(page, site)
      : await rewriteMarkup(
        'commandfor="_y-search-dialog"', 'data-unsupported-commandfor="_y-search-dialog"',
        'the disabled native search invoker',
      )(page);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (await page.locator('html').getAttribute('lang') !== language) broken(`the fixture did not select ${language}`);
    if (await page.$eval(SEARCH_OPEN, (element) => element.hasAttribute('commandfor'))) broken('the no-invoker fixture still has an active invoker');
    if (await page.$eval('html', (root) => root.hasAttribute('data-js'))) broken('script ran in the no-script fixture');
    const link = page.locator('header .y-searchfallback');
    if (await link.count() !== 1 || !(await link.isVisible()) || !(await link.innerText()).trim()) {
      checkProof(proof);
      fail(site, 'the page offers no visible, named scriptless search link');
    }
    // On a phone the way in is the link; above a phone's width it is the
    // header's field, an ordinary form, and the link gives way to it. One of
    // them is drawn, whole and inside the window, at every width.
    for (const width of [1281, 1280, 938, 937, 721, 720, 521, 520, 390, 375, 360]) {
      await page.setViewportSize({ width, height: 800 });
      const row = await page.evaluate(({ field, phone }) => {
        const header = document.querySelector('header');
        const name = document.querySelector('.y-brand__name > span');
        const way = innerWidth > phone ? document.querySelector(field) : document.querySelector('.y-searchfallback');
        const box = way.getBoundingClientRect();
        return {
          way: innerWidth > phone ? 'field' : 'link',
          overflow: header.scrollWidth - header.clientWidth,
          nameLost: name.scrollWidth - name.clientWidth,
          wayFits: way.checkVisibility() && box.left >= 0 && box.right <= innerWidth && box.width > 0,
        };
      }, { field: SEARCH_FIELD, phone: PHONE });
      checkProof(proof);
      if (row.overflow > 1 || row.nameLost > 1 || !row.wayFits) {
        fail(site, `${language} at ${width}px: the scriptless header does not fit: ${JSON.stringify(row)}`);
      }
      if (language === 'en' && width === 375 && row.nameLost !== 0) {
        fail(site, `en at 375px: the scriptless wordmark loses ${row.nameLost}px, want 0`);
      }
    }
    // Tab to the link from the document; do not supply focus the reader cannot reach.
    await page.setViewportSize({ width: 375, height: 800 });
    let reached = false;
    for (let step = 0; step < 20; step += 1) {
      await page.keyboard.press('Tab');
      if (await link.evaluate((element) => element === document.activeElement)) { reached = true; break; }
    }
    checkProof(proof);
    if (!reached) fail(site, 'keyboard navigation never reached the scriptless search link');
    await Promise.all([
      page.waitForURL((url) => url.pathname !== new URL(BASE + PAGE).pathname),
      page.keyboard.press('Enter'),
    ]);
    if (new URL(page.url()).pathname !== '/search') fail(site, 'the scriptless search link did not reach /search');
    await context.close();
  }

  // --- The keys, explained where a reader with no script can read them ----
  //
  // The panel the ? key opens answers only to script, as the keys do. The
  // settings page lists the same keys in its own markup, so a reader whose
  // browser runs none of it can still find out what the keyboard would do.
  {
    const site = 'the-keys-are-explained-with-no-script';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.goto(BASE + '/preferences', { waitUntil: 'domcontentloaded' });
    if (await page.$eval('html', (root) => root.hasAttribute('data-js'))) broken('script ran in the no-script fixture');
    if (await page.locator(KEYS_EXPLAINED).count() !== 1) broken('the settings page carries no single list of keys');
    const keys = await page.locator(`${KEYS_EXPLAINED} .ui-kbd`).allInnerTexts();
    checkProof(proof);
    if (!(await page.locator(KEYS_EXPLAINED).isVisible()) || !keys.includes('?')) {
      fail(site, `the settings page with no script shows the keys ${JSON.stringify(keys)}, want a visible list that names the ? key`);
    }
    await context.close();
  }

  // --- The sheet closes on the press, with nothing listening ------------
  {
    const site = 'the-sheet-closes-with-no-listener';
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (await page.locator(CONCEPT).count() === 0) {
      broken(`${PAGE} carries no concept term, so there is no sheet to open`);
    }
    await page.locator(CONCEPT).first().click();
    await page.locator(SHEET).waitFor({ state: 'visible', timeout: 4000 });
    if (!(await isOpen(page, SHEET))) broken('the concept sheet did not open, so what shuts it proves nothing');
    await page.locator(SHEET_CLOSE).click();
    await page.waitForTimeout(400);
    checkProof(proof);
    if (await isOpen(page, SHEET)) {
      fail(site, 'the sheet stayed open after its own close press, and nothing in the module listens for that press any more');
    }
    await context.close();
  }

  // --- An engine that cannot perform the press itself -------------------
  //
  // Both halves are taken away before the page's scripts run: the attribute,
  // so the browser does nothing with the press, and the property the module
  // asks about, so the module knows it is on such an engine. What is left is
  // the listener this module keeps for exactly that case.
  {
    const site = 'an-engine-without-commands-keeps-a-way-in';
    // At a phone's width, where the header draws the press this is about.
    const context = await browser.newContext({ viewport: { width: 390, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.addInitScript(() => {
      delete HTMLButtonElement.prototype.commandForElement;
      delete HTMLButtonElement.prototype.command;
      document.addEventListener('DOMContentLoaded', () => {
        for (const button of document.querySelectorAll('button[commandfor]')) {
          button.removeAttribute('commandfor');
          button.removeAttribute('command');
        }
      });
    });
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (await page.$eval(SEARCH_OPEN, (element) => element.hasAttribute('commandfor'))) {
      broken('the press still names what it opens, so the browser may be answering it and the module is not being asked');
    }
    if (await page.evaluate(() => 'commandForElement' in HTMLButtonElement.prototype)) {
      broken('the engine still reports that it performs a press declared in the markup, so this is not the engine being stood in for');
    }
    await page.locator(SEARCH_OPEN).click();
    await page.waitForTimeout(300);
    checkProof(proof);
    if (!(await isOpen(page, SEARCH))) {
      fail(site, 'on an engine that cannot perform the press itself, the header control opened nothing');
    }
    await context.close();
  }

  console.log(
    'PASS overlay-commands: the palette opens from a phone\'s press on a page running no script, the header\'s field searches with none above a phone, the settings page explains the keys,'
    + ' the concept sheet closes on a press nothing listens for, and an engine without the markup form of a'
    + ' press still has a way into the palette; with neither feature a keyboard link reaches /search'
    + '; script-only header controls stay hidden and unfocusable while native preferences remains keyboard reachable',
  );
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else if (error instanceof ProbeBroken) {
    console.error(error.message);
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
