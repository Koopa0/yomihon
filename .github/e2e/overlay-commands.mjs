// Behavior lock for how the page's overlays are opened and closed: by the
// browser, because the markup says so, rather than by this site's script.
//
// A press that names what it opens is performed by the browser itself, so the
// palette and the keyboard explanation answer a reader whose browser is
// running none of our JavaScript. That is the claim, and the only way to ask
// it is with scripting switched off — a Go test reads markup and cannot tell
// an attribute the browser acts on from one it ignores.
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

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

// Each press is found by something the markup carries for another reason — a
// class the stylesheet dresses, a hook the runtime already looks for — never by
// the attribute under test. A locator that named that attribute would stop
// matching the moment a mutation took it away, and a press that could not be
// found reads as a probe that timed out rather than as the lock it is.
const SEARCH_OPEN = '[data-search-open]';
const SEARCH = '[data-search]';
const HELP_OPEN = '.y-helpbtn';
const HELP = '#_y-kbd-help';
const SHEET = '[data-concept-sheet]';
const SHEET_CLOSE = '[data-concept-sheet] .y-conceptsheet__head button';
const CONCEPT = '[data-concept]';

const SITES = [
  'the-palette-opens-with-no-script',
  'search-remains-reachable-with-neither-feature',
  'the-keyboard-help-opens-with-no-script',
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
  await page.route('**/search.js', async (route) => {
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

const MUTATIONS = {
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
    apply: async (page) => {
      const nativeProof = await rewriteMarkup(
        'commandfor="_y-search-dialog"', 'data-unsupported-commandfor="_y-search-dialog"',
        'the disabled native search invoker',
      )(page);
      const perLoad = [];
      await page.route('**/app.css', async (route) => {
        const response = await route.fetch();
        const body = await response.text();
        perLoad.push((body.match(/\.y-searchfallback\s*\{/g) || []).length);
        await route.fulfill({ response, body: body + '\n.y-searchfallback { display: inline-block; min-width: 1500px; }' });
      });
      return () => nativeProof() || (perLoad.length === 0 || perLoad.some((hits) => hits !== 1)
        ? `scriptless link rule matched [${perLoad}]` : '');
    },
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
  // The same for the older half of the idea, on the keyboard explanation.
  'take-the-target-off-the-help-press': {
    target: 'the-keyboard-help-opens-with-no-script',
    apply: rewriteMarkup(
      'popovertarget="_y-kbd-help"',
      'data-retired-popovertarget="_y-kbd-help"',
      'the popover target on the keyboard help press',
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
  // --- The palette, on a page running no script of ours -----------------
  {
    const site = 'the-palette-opens-with-no-script';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 800 } });
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
    const form = await page.$eval('[data-live-search-form]', (element) => ({
      method: element.getAttribute('method'),
      action: new URL(element.action, location.href).pathname,
    }));
    if (form.method !== 'get' || form.action !== '/search') {
      fail(site, `the palette a reader with no script reaches submits ${JSON.stringify(form)}, want a get to /search`);
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
    for (const width of [1280, 938, 937, 720, 521, 520, 390, 375, 360]) {
      await page.setViewportSize({ width, height: 800 });
      const row = await page.evaluate(() => {
        const header = document.querySelector('header');
        const name = document.querySelector('.y-brand__name');
        const link = document.querySelector('.y-searchfallback').getBoundingClientRect();
        return {
          overflow: header.scrollWidth - header.clientWidth,
          nameLost: name.scrollWidth - name.clientWidth,
          linkFits: link.left >= 0 && link.right <= innerWidth && link.width > 0,
        };
      });
      checkProof(proof);
      if (row.overflow > 1 || row.nameLost > 1 || !row.linkFits) {
        fail(site, `${language} at ${width}px: the scriptless header does not fit: ${JSON.stringify(row)}`);
      }
    }
    // Tab to the link from the document; do not supply focus the reader cannot reach.
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

  // --- The keyboard explanation, the same way ---------------------------
  {
    const site = 'the-keyboard-help-opens-with-no-script';
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const proof = await applyMutation(page, site);
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (await page.locator(HELP).count() !== 1) broken(`${PAGE} carries no single keyboard help panel`);
    if (await isOpen(page, HELP)) broken('the keyboard help is already open before anything was pressed');
    await page.locator(HELP_OPEN).click();
    checkProof(proof);
    if (!(await isOpen(page, HELP))) {
      fail(site, 'pressing the keyboard help control on a page with no script left the panel closed');
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
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
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
    'PASS overlay-commands: the palette and the keyboard help open from a press on a page running no script,'
    + ' the concept sheet closes on a press nothing listens for, and an engine without the markup form of a'
    + ' press still has a way into the palette; with neither feature a keyboard link reaches /search',
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
