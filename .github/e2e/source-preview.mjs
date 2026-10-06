// Behavior lock for the card over a claim's declared sources: a reader checking
// a claim rests on the Methods or Limitations row of its source list and sees
// that passage beside the row, without leaving the claim. It runs where the
// list sits in the rail and where it sits in the disclosure above the text,
// because they are two copies of one list and two different places to be
// reached from.
//
// What must not open a card matters as much as what does: a row whose place the
// source lacks (its address falls back to the top of the file, and the card
// would show that opening as the passage the claim named), and the outline, the
// course navigation and the list of notes citing this one, which are not
// declared sources. Each of those is asked only after a source row on the same
// page has been shown to open a card, so an absent card cannot be a page on
// which nothing opens one.
//
// The mutation modes rewrite bytes as they are served and touch no file on
// disk. Env: YOMIHON_BASE, PAGE_PATH (the claim note), MUTATE.
import { chromium } from 'playwright-core';

const arrived = (page) => page.waitForFunction(
  async () => {
    if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return true;
  },
  null,
  { timeout: 3000 },
);

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/source-locations-claim.md';
const SOURCE = '/notes/Notes/source-locations-source.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'source-opens-on-hover',
  'source-opens-on-focus',
  'source-card-anchored-to-row',
  'source-card-shows-the-cited-passage',
  'compact-row-opens-its-section',
  'the-whole-source-row-opens-the-top-of-the-source',
  'compare-column-source-opens',
  'compare-column-missing-location-opens-nothing',
  'narrow-inline-source-opens',
  'missing-location-opens-nothing',
  'outline-and-navigation-open-nothing',
  'escape-dismisses-a-source-card',
  'a-source-click-follows-it-once',
  'a-failed-excerpt-leaves-the-source-link-working',
  'a-coarse-pointer-opens-no-source-card',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN source-preview: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL source-preview: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN source-preview: ${message}`);
};

// The module rewrite is counted, so a needle that no longer matches the source
// reports itself rather than passing as a mutation nobody noticed.
const rewriteAsset = (route, needle, replacement) => async (context) => {
  let matched = -1;
  await context.route(route, async (handler) => {
    const response = await handler.fetch();
    const original = await response.text();
    matched = original.split(needle).length - 1;
    await handler.fulfill({ response, body: original.split(needle).join(replacement) });
  });
  return () =>
    matched === 1
      ? ''
      : `the needle ${JSON.stringify(needle)} matched ${matched === -1 ? 'nothing, because the asset was never fetched' : `${matched} times, want 1`}`;
};
const rewriteModule = (needle, replacement) => rewriteAsset('**/static/preview.js{,?*}', needle, replacement);
const rewriteStylesheet = (needle, replacement) => rewriteAsset('**/static/app.css{,?*}', needle, replacement);

const SOURCE_SELECTOR = "'.y-basedon:not([data-declared-by]) a.ui-navitem'";

const MUTATIONS = {
  // The list is looked for inside the main element only, which is the barrier
  // that kept the rail out: at the narrow width the inline copy still sits in
  // main, so this is caught by the wide run's first site.
  'find-sources-only-inside-main': {
    target: 'source-opens-on-hover',
    apply: rewriteModule('const sources = [...document.querySelectorAll(', 'const sources = [...root.querySelectorAll('),
  },
  // Only the rail is asked, so the copy inside the disclosure never opens one.
  'find-sources-only-in-the-rail': {
    target: 'narrow-inline-source-opens',
    apply: rewriteModule(SOURCE_SELECTOR, "'.y-rail-right .y-basedon:not([data-declared-by]) a.ui-navitem'"),
  },
  'ignore-the-keyboard': {
    target: 'source-opens-on-focus',
    apply: rewriteModule("link.addEventListener('focus', (event) => schedule(link, openDelay, event.timeStamp));", ''),
  },
  // The card keeps its content and loses its place: no row names the anchor.
  'unanchor-the-source-row': {
    target: 'source-card-anchored-to-row',
    apply: rewriteStylesheet('.y-basedon .ui-navitem[data-preview-open]', '.y-basedon .ui-navitem[data-never-open]'),
  },
  // The fragment is not forwarded, so every row shows the source from the top.
  'drop-the-fragment': {
    target: 'source-card-shows-the-cited-passage',
    apply: rewriteModule('const fragment = decodeURIComponent(link.hash.slice(1));', "const fragment = '';"),
  },
  // A row whose place the source lacks is asked for as well.
  'preview-the-missing-location': {
    target: 'missing-location-opens-nothing',
    apply: rewriteModule(':not(.wikilink-degraded)', ''),
  },
  // A row whose place is missing is previewed in the compare columns only.
  'preview-the-missing-location-in-compare-only': {
    target: 'compare-column-missing-location-opens-nothing',
    apply: rewriteModule(':not(.wikilink-degraded)', ':is(:not(.wikilink-degraded), .y-compare *)'),
  },
  // The compare columns draw the same article, and stop being asked for.
  'exclude-compare-columns': {
    target: 'compare-column-source-opens',
    apply: rewriteModule(SOURCE_SELECTOR, "'.y-basedon:not([data-declared-by]):not(.y-compare .y-basedon) a.ui-navitem'"),
  },
  // A row that is the only place a source is declared at is a compact row, not
  // a location under a group; this stops asking for it.
  'drop-compact-rows': {
    target: 'compact-row-opens-its-section',
    apply: rewriteModule(
      'links.push(...sources);',
      'links.push(...sources.filter((link) => link.closest(\'.y-basedon__locations\') || link.nextElementSibling));',
    ),
  },
  // The source selector is no longer scoped to the declared-sources block.
  'preview-every-navigation-link': {
    target: 'outline-and-navigation-open-nothing',
    apply: rewriteModule(SOURCE_SELECTOR, "'a.ui-navitem'"),
  },
  // The list of notes that declare this one shares the block's class and
  // would be asked for as though it were this note's own sources.
  'preview-the-declared-by-list': {
    target: 'outline-and-navigation-open-nothing',
    apply: rewriteModule(':not([data-declared-by])', ''),
  },
  'preview-on-any-pointer': {
    target: 'a-coarse-pointer-opens-no-source-card',
    apply: rewriteModule("if (!matchMedia('(pointer: fine)').matches) return;", ''),
  },
  'deafen-escape': {
    target: 'escape-dismisses-a-source-card',
    apply: rewriteModule("if (event.key === 'Escape') close();", ''),
  },
  // A handler that swallows the activation, so following the row needs a
  // second press.
  'swallow-the-click': {
    target: 'a-source-click-follows-it-once',
    apply: rewriteModule(
      "link.addEventListener('blur', close);",
      "link.addEventListener('blur', close);\n    link.addEventListener('click', (event) => event.preventDefault());",
    ),
  },
  // A failed excerpt request takes the link down with it.
  'break-the-link-when-the-excerpt-fails': {
    target: 'a-failed-excerpt-leaves-the-source-link-working',
    apply: rewriteModule(
      'if (error.name !== \'AbortError\') close();',
      "if (error.name !== 'AbortError') { close(); link.removeAttribute('href'); }",
    ),
  },
  // The card no longer says the whole-source row is the whole source: the
  // fragment is invented for it.
  'invent-a-fragment-for-the-whole-source': {
    target: 'the-whole-source-row-opens-the-top-of-the-source',
    apply: rewriteModule(
      "if (fragment) url.searchParams.set('section', fragment);",
      "url.searchParams.set('section', fragment || 'methods');",
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`source-preview: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`source-preview: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`source-preview: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
let proof = null;
// Only the site a mode aims at proves that mode applied.
const proveApplied = (site) => {
  if (!mutation || mutation.target !== site || !proof) return;
  const issue = proof();
  if (issue) throw new NotApplied(`NOT-APPLIED source-preview: ${MUTATE}: ${issue}`);
};

const cardState = (page) =>
  page.evaluate(() => {
    const card = document.querySelector('[data-preview-card]');
    if (!card) return { present: false, open: false };
    const box = card.getBoundingClientRect();
    return {
      present: true,
      open: card.matches(':popover-open'),
      text: card.textContent.replace(/\s+/g, ' ').trim(),
      proseText: (card.querySelector('.y-prose')?.textContent ?? '').replace(/\s+/g, ' ').trim(),
      box: { top: box.top, left: box.left, right: box.right, bottom: box.bottom },
      viewport: { width: window.innerWidth, height: window.innerHeight },
      anchored: [...document.querySelectorAll('[data-preview-open]')].map((el) => el.textContent.replace(/\s+/g, ' ').trim()),
    };
  });

const settles = (page, open, timeout) =>
  page
    .waitForFunction(
      (wanted) => Boolean(document.querySelector('[data-preview-card]')?.matches(':popover-open')) === wanted,
      open,
      { timeout },
    )
    .then(
      () => true,
      () => false,
    );

// A scroll event closes the card, so a link this probe has to travel to would
// answer "no card" whichever way the product behaves. All travelling happens
// here, before the pointer or the keyboard arrives, and the page's resting
// place is read back afterwards.
const QUIET = 150;
const stillness = async (page) => {
  await page.evaluate(() => {
    window.__probeScrolledAt = performance.now();
    if (window.__probeWatchingScroll) return;
    window.__probeWatchingScroll = true;
    document.addEventListener(
      'scroll',
      () => {
        window.__probeScrolledAt = performance.now();
      },
      { capture: true, passive: true },
    );
  });
  await page.waitForFunction((quiet) => performance.now() - window.__probeScrolledAt > quiet, QUIET, { timeout: 5000 });
};

const bring = async (page, link, how) => {
  await link.evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await stillness(page);
  await page.mouse.move(4, 4);
  if (!(await settles(page, false, 2000))) {
    await page.evaluate(() => document.querySelector('[data-preview-card]')?.hidePopover());
    if (!(await settles(page, false, 2000))) broken(`a card stayed open before the ${how} reached its link`);
  }
  const rest = await page.evaluate(() => Math.round(window.scrollY));
  if (how === 'pointer') await link.hover();
  else await link.focus();
  const landed = await page.evaluate(() => Math.round(window.scrollY));
  if (landed !== rest) broken(`the page moved ${Math.abs(landed - rest)}px as the ${how} reached its link`);
};
const pointerOnto = (page, link) => bring(page, link, 'pointer');
const focusOnto = (page, link) => bring(page, link, 'keyboard');

// The one source list a reader can see, opening the narrow layout's disclosure
// when that is where it sits.
const openSources = async (page) => {
  const disclosure = page.locator('details.y-toc-inline').filter({ has: page.locator('.y-basedon') }).first();
  if ((await disclosure.count()) && (await disclosure.isVisible()) && !(await disclosure.evaluate((el) => el.open))) {
    await disclosure.locator('summary').click();
    // The disclosure opens with a short animation, and a list it has not yet
    // opened is not a list a reader can see.
    await page.locator('.y-basedon:not([data-declared-by]):visible').first().waitFor({ timeout: 3000 });
  }
  const list = page.locator('.y-basedon:not([data-declared-by]):visible');
  if ((await list.count()) !== 1) broken(`${await list.count()} source lists are visible at ${page.url()} (${page.viewportSize()?.width}px), want 1`);
  return list;
};

// The first of the matches a reader can see: the aids exist twice on a note
// page, once in the rail and once in the disclosure the narrow layout folds.
const visibleOne = async (locator, what) => {
  for (const link of await locator.all()) {
    if (await link.evaluate((el) => el.offsetParent !== null)) return link;
  }
  return broken(`no ${what} is visible`);
};

const row = async (list, name) => {
  const found = list.getByRole('link', { name });
  if ((await found.count()) !== 1) broken(`the visible source list carries ${await found.count()} rows named ${JSON.stringify(name)}, want 1`);
  return found;
};

const METHODS = 'Only a frozen event handbook was tested.';
const LIMITATIONS = 'Changing team documents were not tested.';
const INTRO = 'Introduction context';

const journey = async (browser, width, placement) => {
  const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
  if (mutation) proof = await mutation.apply(context);
  const requests = [];
  context.on('request', (request) => {
    if (request.url().startsWith(`${BASE}/preview/`)) requests.push(decodeURIComponent(request.url()));
  });
  try {
    const page = await context.newPage();
    const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    await arrived(page);
    if (!response || response.status() !== 200) broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
    await page.waitForSelector('html[data-js]');

    let list = await openSources(page);
    const methods = await row(list, 'Method evidence');
    const limitations = await row(list, 'Study limitations');

    // Hover on a source section row.
    const openSite = placement === 'inline' ? 'narrow-inline-source-opens' : 'source-opens-on-hover';
    await pointerOnto(page, methods);
    proveApplied(openSite);
    if (!(await settles(page, true, 4000))) {
      fail(openSite, `resting the pointer on the Methods row in the ${placement} list opened no card`);
    }
    {
      const state = await cardState(page);
      proveApplied('source-card-anchored-to-row');
      const box = await methods.evaluate((el) => {
        const b = el.getBoundingClientRect();
        return { top: b.top, left: b.left, right: b.right, bottom: b.bottom };
      });
      const gap = Math.min(Math.abs(state.box.top - box.bottom), Math.abs(box.top - state.box.bottom));
      if (gap > 48) {
        fail('source-card-anchored-to-row', `the ${placement} card's nearest edge sits ${Math.round(gap)}px from its row, so it is not beside the row it belongs to`);
      }
      if (state.box.right < 0 || state.box.left > state.viewport.width || state.box.bottom < 0 || state.box.top > state.viewport.height) {
        fail('source-card-anchored-to-row', 'the card is outside the window');
      }
      if (state.anchored.length !== 1) broken(`${state.anchored.length} links claim the card's anchor, want exactly 1`);

      proveApplied('source-card-shows-the-cited-passage');
      if (!state.proseText.includes(METHODS)) {
        fail('source-card-shows-the-cited-passage', `the card does not carry the Methods passage; it reads ${JSON.stringify(state.proseText.slice(0, 160))}`);
      }
      if (state.proseText.includes(INTRO) || state.proseText.includes(LIMITATIONS)) {
        fail('source-card-shows-the-cited-passage', 'the card carries words outside the Methods section, so the row\'s fragment was not asked for');
      }
    }

    // Escape, without moving the pointer.
    await page.keyboard.press('Escape');
    proveApplied('escape-dismisses-a-source-card');
    if (!(await settles(page, false, 2000))) fail('escape-dismisses-a-source-card', 'Escape left the source card open');

    // Keyboard, on the other row, whose passage differs and whose address is a
    // distinct cache entry.
    await focusOnto(page, limitations);
    proveApplied('source-opens-on-focus');
    if (!(await settles(page, true, 4000))) fail('source-opens-on-focus', 'reaching the Limitations row by keyboard opened no card');
    {
      const state = await cardState(page);
      if (!state.proseText.includes(LIMITATIONS)) {
        fail('source-card-shows-the-cited-passage', `the Limitations card does not carry its passage; it reads ${JSON.stringify(state.proseText.slice(0, 160))}`);
      }
      if (state.proseText.includes(METHODS)) {
        fail('source-card-shows-the-cited-passage', 'the Limitations card still carries the Methods passage, so the two rows share a cache entry');
      }
    }
    await page.keyboard.press('Escape');
    await settles(page, false, 2000);

    // A source with one location is one compact row, not a group.
    const compact = await row(list, /Chapter scale/);
    await pointerOnto(page, compact);
    proveApplied('compact-row-opens-its-section');
    if (!(await settles(page, true, 4000))) {
      fail('compact-row-opens-its-section', `resting the pointer on the compact row in the ${placement} list opened no card`);
    }
    {
      const state = await cardState(page);
      if (!state.proseText.includes('一章的正文') || state.proseText.includes('一篇的正文')) {
        fail('compact-row-opens-its-section', `the compact row's card does not carry exactly its section; it reads ${JSON.stringify(state.proseText.slice(0, 120))}`);
      }
    }
    await page.mouse.move(4, 4);
    await settles(page, false, 2000);

    // The whole-source row asks for no section: the top of the source.
    const whole = await row(list, /^source-locations-source$/);
    await pointerOnto(page, whole);
    proveApplied('the-whole-source-row-opens-the-top-of-the-source');
    if (!(await settles(page, true, 4000))) broken('the whole-source row opened no card');
    {
      const state = await cardState(page);
      if (!state.proseText.includes(INTRO)) {
        fail('the-whole-source-row-opens-the-top-of-the-source', `the whole-source card does not open at the top of the source; it reads ${JSON.stringify(state.proseText.slice(0, 160))}`);
      }
    }
    await page.mouse.move(4, 4);
    await settles(page, false, 2000);

    // Rows that must open nothing, each asked with a control on this page.
    list = await openSources(page);
    const seen = requests.length;
    const missing = list.locator('a.wikilink-degraded');
    if ((await missing.count()) !== 1) broken(`the claim shows ${await missing.count()} rows the source lacks, want 1`);
    await pointerOnto(page, missing);
    await page.waitForTimeout(900);
    proveApplied('missing-location-opens-nothing');
    {
      const state = await cardState(page);
      if (state.open) {
        fail('missing-location-opens-nothing', `the missing-location row opened a card reading ${JSON.stringify(state.text.slice(0, 120))}`);
      }
      if (requests.length !== seen) {
        fail('missing-location-opens-nothing', `the missing-location row asked for an excerpt: ${requests[requests.length - 1]}`);
      }
    }
    await page.mouse.move(4, 4);

    // The claim's own navigation: the sidebar's row for the source, and the
    // link to the next note. Wide only: the sidebar is a drawer at phone width.
    if (placement === 'rail') {
      const before = requests.length;
      const outside = [];
      for (const link of await page.locator('a.ui-navitem[href$="source-locations-source.md"], a.y-steps__link').all()) {
        if (await link.evaluate((el) => !el.closest('.y-basedon') && el.offsetParent !== null)) outside.push(link);
      }
      if (outside.length < 2) broken(`${outside.length} navigation links to the next note are visible outside the declared-sources block, want the sidebar row and the step link`);
      for (const link of outside) {
        await pointerOnto(page, link);
        await page.waitForTimeout(700);
        proveApplied('outline-and-navigation-open-nothing');
        if ((await cardState(page)).open || requests.length !== before) {
          fail('outline-and-navigation-open-nothing', `a navigation link (${await link.getAttribute('href')}) opened a card or asked for an excerpt`);
        }
        await page.mouse.move(4, 4);
      }
    }
  } finally {
    await context.close();
  }
};

// The source note carries the list of notes that cite it, and its own outline.
// Its own declared source is the control that proves a card opens on the page.
const citedByAndOutline = async (browser) => {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 }, reducedMotion: 'reduce' });
  if (mutation) proof = await mutation.apply(context);
  const requests = [];
  context.on('request', (request) => {
    if (request.url().startsWith(`${BASE}/preview/`)) requests.push(request.url());
  });
  try {
    const page = await context.newPage();
    const response = await page.goto(BASE + SOURCE, { waitUntil: 'networkidle' });
    await arrived(page);
    if (!response || response.status() !== 200) broken(`${SOURCE} returned ${response?.status() ?? 'no response'}, want 200`);
    await page.waitForSelector('html[data-js]');

    const control = page.locator('main .y-prose a.wikilink', { hasText: 'the checked claim' });
    if ((await control.count()) !== 1) broken('the source note carries no prose link back to the claim');
    await pointerOnto(page, control);
    if (!(await settles(page, true, 4000))) broken('a prose link on the source page opened no card, so an absent card below proves nothing');
    await page.mouse.move(4, 4);
    await settles(page, false, 2000);

    const cited = await visibleOne(page.locator('.y-citedby a[href^="/notes/"]'), 'link in the list of notes citing the source');
    const outline = [];
    for (const link of await page.locator('a[href^="#"]').all()) {
      if ((await link.evaluate((el) => el.offsetParent !== null)) && /Methods|Limitations/.test(await link.innerText())) outline.push(link);
    }
    if (outline.length === 0) broken('the source note shows no outline link, so the outline is unasked');
    const before = requests.length;
    const declaredBy = await visibleOne(page.locator('.y-basedon[data-declared-by] a.ui-navitem[href^="/notes/"]'), 'link in the list of notes declaring the source');
    for (const link of [cited, outline[0], declaredBy]) {
      await pointerOnto(page, link);
      await page.waitForTimeout(700);
      proveApplied('outline-and-navigation-open-nothing');
      if ((await cardState(page)).open || requests.length !== before) {
        fail('outline-and-navigation-open-nothing', `${JSON.stringify(await link.getAttribute('href'))} opened a card or asked for an excerpt`);
      }
      await page.mouse.move(4, 4);
    }
  } finally {
    await context.close();
  }
};

// The compare columns draw the same article, aids included, so a source row in
// one of them opens a card the same way and a missing location still opens
// none.
const compare = async (browser) => {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 }, reducedMotion: 'reduce' });
  if (mutation) proof = await mutation.apply(context);
  const requests = [];
  context.on('request', (request) => {
    if (request.url().startsWith(`${BASE}/preview/`)) requests.push(request.url());
  });
  try {
    const page = await context.newPage();
    const response = await page.goto(`${BASE}/compare${PAGE.slice('/notes'.length)}?with=Notes%2Fsource-locations-source.md`, { waitUntil: 'networkidle' });
    await arrived(page);
    if (!response || response.status() !== 200) broken(`the compare page returned ${response?.status() ?? 'no response'}, want 200`);
    await page.waitForSelector('html[data-js]');
    const list = await openSources(page);
    const methods = await row(list, 'Method evidence');
    await pointerOnto(page, methods);
    proveApplied('compare-column-source-opens');
    if (!(await settles(page, true, 4000))) fail('compare-column-source-opens', 'resting the pointer on the Methods row in a compare column opened no card');
    if (!(await cardState(page)).proseText.includes(METHODS)) fail('compare-column-source-opens', 'the compare column card does not carry the Methods passage');
    await page.mouse.move(4, 4);
    await settles(page, false, 2000);
    const seen = requests.length;
    const missing = list.locator('a.wikilink-degraded');
    if ((await missing.count()) !== 1) broken(`the compare column shows ${await missing.count()} rows the source lacks, want 1`);
    await pointerOnto(page, missing);
    await page.waitForTimeout(900);
    proveApplied('compare-column-missing-location-opens-nothing');
    if ((await cardState(page)).open || requests.length !== seen) {
      fail('compare-column-missing-location-opens-nothing', 'the missing-location row in a compare column opened a card or asked for an excerpt');
    }
  } finally {
    await context.close();
  }
};

// One activation follows the row, with the card open over it; Back returns to
// the claim; and a preview that fails leaves the row a working link.
const activation = async (browser) => {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 }, reducedMotion: 'reduce' });
  if (mutation) proof = await mutation.apply(context);
  try {
    const page = await context.newPage();
    const claim = BASE + PAGE;
    await page.goto(claim, { waitUntil: 'networkidle' });
    await arrived(page);
    await page.waitForSelector('html[data-js]');
    const list = await openSources(page);
    const methods = await row(list, 'Method evidence');
    await pointerOnto(page, methods);
    if (!(await settles(page, true, 4000))) broken('the Methods row opened no card before it was followed');
    proveApplied('a-source-click-follows-it-once');
    await methods.click();
    try {
      await page.waitForURL((url) => url.pathname === SOURCE && url.hash === '#methods', { timeout: 4000 });
    } catch {
      fail('a-source-click-follows-it-once', `one click on the Methods row ended at ${page.url()}, want ${SOURCE}#methods`);
    }
    await page.goBack();
    try {
      await page.waitForURL(claim, { timeout: 4000 });
    } catch {
      fail('a-source-click-follows-it-once', `Back ended at ${page.url()}, want the claim`);
    }
  } finally {
    await context.close();
  }

  const failing = await browser.newContext({ viewport: { width: 1600, height: 900 }, reducedMotion: 'reduce' });
  if (mutation) proof = await mutation.apply(failing);
  try {
    await failing.route('**/preview/**', (route) => route.abort());
    const page = await failing.newPage();
    const claim = BASE + PAGE;
    await page.goto(claim, { waitUntil: 'networkidle' });
    await arrived(page);
    await page.waitForSelector('html[data-js]');
    const list = await openSources(page);
    const methods = await row(list, 'Method evidence');
    await pointerOnto(page, methods);
    await page.waitForTimeout(900);
    proveApplied('a-failed-excerpt-leaves-the-source-link-working');
    if ((await cardState(page)).open) broken('a card opened although every excerpt request was refused');
    // Asked by its words rather than by role: a link with no address is no
    // longer a link to the role query, and that is the failure being looked for.
    const after = list.locator('a.ui-navitem', { hasText: 'Method evidence' });
    const href = await after.getAttribute('href');
    if (href !== `${SOURCE}#methods`) {
      fail('a-failed-excerpt-leaves-the-source-link-working', `after the excerpt failed the row's address is ${JSON.stringify(href)}, want ${SOURCE}#methods`);
    }
    await after.click();
    try {
      await page.waitForURL((url) => url.pathname === SOURCE && url.hash === '#methods', { timeout: 4000 });
    } catch {
      fail('a-failed-excerpt-leaves-the-source-link-working', `the row did not open the source after a failed preview; the page is at ${page.url()}`);
    }
  } finally {
    await failing.close();
  }
};

// A touch screen, where a tap is a request to open the source.
const touch = async (browser) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  if (mutation) proof = await mutation.apply(context);
  const requests = [];
  context.on('request', (request) => {
    if (request.url().startsWith(`${BASE}/preview/`)) requests.push(request.url());
  });
  try {
    const page = await context.newPage();
    await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    await arrived(page);
    await page.waitForSelector('html[data-js]');
    if (!(await page.evaluate(() => matchMedia('(pointer: coarse)').matches))) {
      broken('the touch context still reports a fine pointer, so this check would pass over an emulation that never happened');
    }
    const list = await openSources(page);
    const methods = await row(list, 'Method evidence');
    await pointerOnto(page, methods);
    await page.waitForTimeout(900);
    proveApplied('a-coarse-pointer-opens-no-source-card');
    if ((await cardState(page)).open || requests.length > 0) {
      fail('a-coarse-pointer-opens-no-source-card', 'a card opened, or an excerpt was asked for, on a touch screen');
    }
  } finally {
    await context.close();
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  await journey(browser, 1600, 'rail');
  await journey(browser, 390, 'inline');
  await citedByAndOutline(browser);
  await compare(browser);
  await activation(browser);
  await touch(browser);
  console.log('PASS source-preview: a declared source opens a card at its cited passage from the rail and from the disclosure, the missing location, navigation and cited-by open none, and following the row is one activation');
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
