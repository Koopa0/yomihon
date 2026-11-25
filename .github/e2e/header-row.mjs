// Browser lock for the header row: it carries only what reading needs. At
// every width the button that opens and folds the left column stands first,
// ahead of the wordmark, over the column it acts on. To the right of the wordmark
// there is the furigana switch on a page that has furigana, the search — a
// field above a phone's width, the glass that opens it over the whole screen
// on a phone — the bookmark for keeping a reading place, on a note the control
// that copies its path where the left column is not a drawer, the four grounds
// where the left column is not a drawer, the link to My marks above a phone's
// width, and the way to the settings — in that order, which is the design's,
// and nothing else, at every width. A row that also carries the furigana
// switch is full sooner, so there the grounds wait for a wider window and the
// link to My marks for a column rather than a drawer. Everything else a reader
// sets once and leaves alone lives on the settings page.
//
// What is asked here is what a reader can see and reach, measured rather than
// read off the stylesheet: at each width the row fits inside the window, the
// wordmark keeps all of its letters, the cluster holds exactly its controls in
// their order, and exactly one control for keeping a reading place is drawn —
// the bookmark in the row, never two and never none. The bookmark is then asked
// what a toggle owes a reader: a name, a pressed state that changes when the
// place is kept, a target a thumb can hit, and a place in the keyboard's order.
// The sentence its press earns is asked to hang out of the row: nothing in the
// row moves while it is there, it never covers the title or another control,
// and it stays until the reader's next press or key — not on a timer, which
// takes it from a slow reader before they have read it (D93, D94) — and that
// next action takes it away.
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

// A note in the fixture vault whose body carries furigana, the one kind of
// page the switch for them is drawn on. PAGE is a note that carries none and
// offers to keep a reading place.
const READINGS_PAGE = '/notes/Writing/lessons/japanese/L01.md';

// The width the row's bookmark used to step aside at, when the right rail
// carried the control from there. Both sides of it are still measured, so a
// breakpoint brought back in either direction shows up here.
const RAIL = 1280;
const DRAWER = 1023;
const WIDTHS = [1440, RAIL + 1, RAIL, RAIL - 1, DRAWER + 1, DRAWER, 721, 720, 390, 375, 360];
const PLACE_WIDTHS = [DRAWER, DRAWER + 1, RAIL - 1, RAIL, RAIL + 1, 1440];

// Each control the cluster may hold, named by the hook that survives a
// restyling, in the order the row draws them.
const RUBY = '[data-ruby-toggle]';
const MARK = '.y-headermark [data-mark-button]';
const PATH = '.y-headerpath [data-path-button]';
// The search is the field above a phone's width and the glass on a phone,
// never both and never neither.
const SEARCH_FIELD = '.y-searchfield [data-live-search-input]';
const SEARCH = '[data-search-open]';
const PAPERS = ['stone', 'washi', 'white', 'dark'].map((ground) => `.y-paperswitch [data-paper-option="${ground}"]`);
const MARKS = '.y-headerlink[href="/marks"]';
const SETTINGS = '.y-prefslink';
const SIDEBAR = '.y-sidebtn';
const PHONE = 720;
const CLUSTER = [RUBY, SEARCH_FIELD, SEARCH, MARK, PATH, ...PAPERS, MARKS, SETTINGS];
// What may stand ahead of the wordmark: the sidebar button alone.
const LEAD = [SIDEBAR];

const SITES = [
  'reading-place-reachable',
  'cluster-holds-only-what-reading-needs',
  'no-switch-in-the-header',
  'row-fits',
  'wordmark-whole',
  'bookmark-says-whether-it-is-set',
  'drawn-marks-are-named',
  'bookmark-takes-a-thumb',
  'keyboard-walks-the-cluster',
  'saved-label-stays-out-of-the-row',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN header-row: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL header-row: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN header-row: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED header-row: ${message}`); };

// A rule injected at a selector that matches nothing styles nothing, and a
// mutation that styled nothing would be reported as caught by whatever the
// page was already doing.
const injectRule = async (page, selector, css) => {
  if (await page.locator(selector).count() === 0) {
    notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  }
  await page.addStyleTag({ content: css });
};

const changeOne = async (page, selector, change) => {
  const changed = await page.evaluate(({ chosen, operation }) => {
    const elements = [...document.querySelectorAll(chosen)];
    if (elements.length !== 1) return elements.length;
    if (operation.remove) elements[0].removeAttribute(operation.name);
    else elements[0].setAttribute(operation.name, operation.value);
    return 1;
  }, { chosen: selector, operation: change });
  if (changed !== 1) notApplied(`${selector} matched ${changed} elements, want exactly 1`);
};

// Rewrites one script the page loads, before the page asks for it, and
// reports whether the needle was there to rewrite.
const rewriteScript = async (page, glob, needle, replacement) => {
  const hits = [];
  await page.route(glob, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    hits.push(original.split(needle).length - 1);
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => hits.length > 0 && hits.every((count) => count === 1);
};

// Each mutation names the page it is applied on: the note without furigana,
// the one with them, the note with the left column folded, or the note on a
// touch screen. A mutation applied on a page whose questions it cannot
// reach would be reported as not caught for the wrong reason.
const MUTATIONS = {
  // The row's bookmark steps aside again from the width the right rail used to
  // carry it at, so a reader there has no way to keep a place at all.
  'hide-the-bookmark-wide': {
    target: 'reading-place-reachable',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', `@media (min-width: ${RAIL}px) { [data-js] .y-headermark { display: none; } }`),
  },
  // The row's bookmark steps aside early, so a reader in between has no way to
  // keep a place at all.
  'hide-the-bookmark-early': {
    target: 'reading-place-reachable',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', `@media (min-width: 1200px) { [data-js] .y-headermark { display: none; } }`),
  },
  // The sentence a press earns is laid in the row's flow again, so it pushes
  // the controls beside it aside for as long as it is there.
  'put-the-saved-label-in-the-row': {
    target: 'saved-label-stays-out-of-the-row',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark__said', '.y-headermark__said { position: static !important; }'),
  },
  // The bookmark stands ahead of the search again, the order the row drew
  // before the design's order was kept.
  'put-the-bookmark-ahead-of-the-search': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', `.y-headermark { order: -1; }`),
  },
  // The cluster's order changes: the search moves to the end of the row,
  // the field and the glass alike.
  'move-search-past-settings': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, '.y-searchfield', `.y-searchfield, ${SEARCH} { order: 1; }`),
  },
  // The grounds move past the settings link, out of the design's order.
  'move-the-grounds-past-settings': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, '.y-paperswitch', '.y-paperswitch { order: 1; }'),
  },
  // The grounds stay in a phone's row, which has no room for them; the
  // settings page holds the same choice there.
  'draw-the-grounds-on-a-phone': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, '.y-paperswitch', '@media (max-width: 1023px) { .y-paperswitch { display: flex !important; } }'),
  },
  // The link to My marks goes from the row where it has room.
  'drop-the-marks-link': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, MARKS, `${MARKS} { display: none; }`),
  },
  // The phone's glass is drawn beside the field, two ways into one search.
  'draw-the-glass-beside-the-field': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, SEARCH, `${SEARCH} { display: inline-flex !important; }`),
  },
  // The sidebar button drawn only where the column is a drawer, as it was
  // before it also folded the column beside the text.
  'draw-the-sidebar-button-only-for-the-drawer': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, SIDEBAR, `@media (min-width: 1321px) { [data-js] ${SIDEBAR} { display: none !important; } }`),
  },
  // The sidebar button goes back to the end of the row, away from the column
  // it opens.
  'put-the-sidebar-button-last': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, SIDEBAR, `${SIDEBAR} { order: 99; }`),
  },
  // A control that belongs on the settings page is drawn in the row again.
  'put-a-setting-back-in-the-row': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: async (page) => {
      const placed = await page.evaluate(() => {
        const search = document.querySelector('.y-searchfield');
        if (!search) return false;
        const theme = document.createElement('button');
        theme.type = 'button';
        theme.className = 'y-iconbtn';
        theme.setAttribute('aria-label', 'theme');
        theme.textContent = '◐';
        search.before(theme);
        return true;
      });
      if (!placed) notApplied('there is no search control to place a setting beside');
    },
  },
  // The furigana switch comes back to the header, which only the page head draws.
  'draw-the-switch-in-the-header': {
    target: 'no-switch-in-the-header',
    on: 'note',
    apply: async (page) => {
      const placed = await page.evaluate(() => {
        const search = document.querySelector('.y-searchfield');
        if (!search) return false;
        const ruby = document.createElement('button');
        ruby.type = 'button';
        ruby.className = 'y-iconbtn';
        ruby.setAttribute('data-ruby-toggle', '');
        ruby.setAttribute('aria-pressed', 'true');
        search.before(ruby);
        return true;
      });
      if (!placed) notApplied('there is no search control to place the switch beside');
    },
  },
  // One control is drawn over the next, which the window's edges alone
  // would never show.
  'lay-the-bookmark-over-the-path': {
    target: 'row-fits',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', '.y-headermark { margin-inline-end: -40px; }'),
  },
  // The row wants more width than the window has.
  'overflow-the-row': {
    target: 'row-fits',
    on: 'note',
    apply: (page) => injectRule(page, '.y-header', '.y-header { min-width: 700px; }'),
  },
  // A control grows and the wordmark pays for it: the wordmark is let shrink,
  // as the folder's name beside it may. A control that grows alone only pushes
  // the brand under the cluster, which the cluster's own question sees first.
  'let-a-control-take-the-name': {
    target: 'wordmark-whole',
    on: 'note',
    apply: (page) => injectRule(page, '.y-searchfield', '.y-searchfield { min-width: 400px; } .y-brand__name { flex: 0 1 auto; min-width: 24px; } .y-brand__word { overflow: hidden; }'),
  },
  // The bookmark loses the attribute that tells a screen reader it is a
  // toggle and whether it is on.
  'drop-the-pressed-state': {
    target: 'bookmark-says-whether-it-is-set',
    on: 'note',
    apply: (page) => changeOne(page, MARK, { name: 'aria-pressed', remove: true }),
  },
  // The sentence goes back on a timer, so a slow reader loses it before they
  // have read it.
  'put-the-saved-label-on-a-timer': {
    target: 'saved-label-stays-out-of-the-row',
    on: 'note',
    before: (page) => rewriteScript(
      page,
      '**/mark.js{,?*}',
      '  setTimeout(() => {\n    if (next.signal.aborted) return;',
      '  setTimeout(() => hush(said), 1000);\n  setTimeout(() => {\n    if (next.signal.aborted) return;',
    ),
  },
  // The sentence is never taken away: the reader's next action leaves it
  // standing over the page.
  'keep-the-saved-label-for-good': {
    target: 'saved-label-stays-out-of-the-row',
    on: 'note',
    before: (page) => rewriteScript(
      page,
      '**/mark.js{,?*}',
      '    if (next.signal.aborted) return;',
      '    return;',
    ),
  },
  // At a phone's width the sentence is laid over the sidebar button at the
  // start of the row rather than over the wordmark beside it.
  'lay-the-saved-label-over-the-sidebar-button': {
    target: 'saved-label-stays-out-of-the-row',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark__said', '@media (max-width: 720px) { .y-headermark__said { inset-inline-start: 0 !important; } }'),
  },
  // A place kept in this note leaves the bookmark saying it is not set.
  'forget-the-press-after-keeping': {
    target: 'bookmark-says-whether-it-is-set',
    on: 'note',
    before: (page) => rewriteScript(
      page,
      '**/mark.js{,?*}',
      "?.setAttribute('aria-pressed', String(kept));",
      ';',
    ),
  },
  // The settings link loses its name, which on a phone is the only one it
  // has: its word is not drawn there and the mark it draws instead is not read.
  'drop-the-settings-name': {
    target: 'drawn-marks-are-named',
    on: 'note',
    apply: (page) => changeOne(page, SETTINGS, { name: 'aria-label', remove: true }),
  },
  // A touch screen gets the pointer's target, too small for a thumb.
  'shrink-the-touch-target': {
    target: 'bookmark-takes-a-thumb',
    on: 'touch',
    apply: (page) => injectRule(page, '.y-headermark__button', '@media (pointer: coarse) { .y-headermark__button { height: 32px; width: 32px; } }'),
  },
  // The bookmark is out of the tab order, so a reader on the keyboard cannot
  // keep a place at all.
  'take-the-bookmark-out-of-tab-order': {
    target: 'keyboard-walks-the-cluster',
    on: 'note',
    apply: (page) => changeOne(page, MARK, { name: 'tabindex', value: '-1' }),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`header-row: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`header-row: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`header-row: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}
const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
let proveRewrite = null;

// Opens one page for one of the three questions this probe asks, applying the
// mutation when it belongs on this page.
const open = async (context, kind, path) => {
  const page = await context.newPage();
  // No mark is written to the fixture vault: every press here is answered as
  // the server answers a kept place, without reaching it.
  await page.route('**/bookmark', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    await route.fulfill({ status: 204, body: '' });
  });
  if (mutation?.on === kind && mutation.before) proveRewrite = await mutation.before(page);
  const response = await page.goto(BASE + path, { waitUntil: 'load' });
  if (!response || response.status() !== 200) broken(`${path} returned ${response?.status() ?? 'no response'}, want 200`);
  if (mutation?.on === kind && mutation.apply) await mutation.apply(page);
  return page;
};

// What one width looks like. The window's own width comes from an element
// pinned to the viewport rather than from the document, because a document
// narrowed by its own scrollbar reports a width the header was never given.
const readRow = (page, cluster, lead) => page.evaluate(({ selectors, leading }) => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const windowWidth = ruler.getBoundingClientRect().width;
  ruler.remove();

  const header = document.querySelector('.y-header');
  const name = document.querySelector('.y-brand__name > span');
  const brand = document.querySelector('.y-brand');
  const spacer = document.querySelector('.y-header__spacer');
  if (!header || !name || !brand || !spacer) return { issue: 'the page carries no header row, no wordmark or no spacer' };
  const box = header.getBoundingClientRect();
  const after = spacer.getBoundingClientRect().right - 0.5;
  const before = brand.getBoundingClientRect().left + 0.5;
  const controls = [...header.querySelectorAll('button, a, input, select')]
    .filter((element) => element.checkVisibility())
    .sort((one, two) => one.getBoundingClientRect().left - two.getBoundingClientRect().left);
  const named = (element, hooks) => hooks.find((selector) => element.matches(selector)) ?? element.outerHTML.slice(0, 80);
  // Every control drawn to the right of the spacer, left to right, named by
  // the hook it answers to — or by its own tag, when it answers to none of
  // them, which is what a control that does not belong here looks like.
  const drawn = controls
    .filter((element) => element.getBoundingClientRect().left >= after)
    .map((element) => named(element, selectors));
  // And every control drawn ahead of the wordmark, the same way.
  const ahead = controls
    .filter((element) => element.getBoundingClientRect().right <= before)
    .map((element) => named(element, leading));
  // Any control drawn over the one before it, left to right: a row short of
  // room that lets one thing run under the next fits the window and still
  // hides a control.
  const overlaps = [];
  for (let i = 1; i < controls.length; i += 1) {
    const one = controls[i - 1].getBoundingClientRect();
    const two = controls[i].getBoundingClientRect();
    if (two.left < one.right - 0.5) overlaps.push([controls[i - 1].outerHTML.slice(0, 60), controls[i].outerHTML.slice(0, 60), Math.round(one.right - two.left)]);
  }
  return {
    windowWidth,
    headerLeft: box.left,
    headerRight: box.right,
    headerOverflow: header.scrollWidth - header.clientWidth,
    wordmarkLost: name.scrollWidth - name.clientWidth,
    drawn,
    ahead,
    overlaps,
  };
}, { selectors: cluster, leading: lead });

// Where each control of the row stands, so a sentence that pushed one aside
// shows as a difference between two readings.
const rowRects = (page) => page.evaluate(() => ['.y-sidebtn', '.y-brand', '.y-searchfield', '[data-search-open]', '.y-headermark__button', '.y-headerpath__button', '.y-paperswitch', '.y-headerlink', '.y-prefslink'].map((selector) => {
  const box = document.querySelector(selector).getBoundingClientRect();
  return [selector, Math.round(box.left * 10) / 10, Math.round(box.top * 10) / 10, Math.round(box.width * 10) / 10];
}));

const reachablePlaces = (page) => page.locator('[data-mark-button]').evaluateAll((buttons) => buttons.filter((button) => {
  const box = button.getBoundingClientRect();
  return button.checkVisibility() && box.width > 0 && box.height > 0 && !button.disabled;
}).length);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const language of ['zh-Hant', 'en']) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 800 } });
    if (language === 'en') await context.addCookies([{ name: 'yomihon_lang', value: 'en', url: BASE }]);

    const note = await open(context, 'note', PAGE);
    if (await note.locator('.y-headermark').count() !== 1) {
      broken(`${PAGE} carries no single header bookmark; this probe has to be driven against a note that offers to keep a reading place`);
    }
    const readings = await open(context, 'readings', READINGS_PAGE);
    if (await readings.locator('main rt').count() === 0) broken(`${READINGS_PAGE} carries no furigana, so it cannot ask where the switch is drawn`);

    // One control for keeping a reading place at every width either side of
    // the rail's: the bookmark in the row below it, the rail's own from it.
    for (const width of PLACE_WIDTHS) {
      await note.setViewportSize({ width, height: 800 });
      const reachable = await reachablePlaces(note);
      if (reachable !== 1) fail('reading-place-reachable', `${language} at ${width}px offers ${reachable} reading-place controls, want exactly one`);
    }
    // The same at the rail's widths with the left column folded: folding the
    // navigation is not a reason to lose the reading rail's control, and the
    // row's bookmark does not come back to stand in for it.
    const foldedContext = await browser.newContext({ viewport: { width: 1440, height: 800 } });
    await foldedContext.addCookies([
      { name: 'yomihon_rail', value: 'collapsed', url: BASE },
      ...(language === 'en' ? [{ name: 'yomihon_lang', value: 'en', url: BASE }] : []),
    ]);
    const folded = await open(foldedContext, 'folded', PAGE);
    if (await folded.evaluate(() => document.documentElement.dataset.rail) !== 'collapsed') {
      broken('the folded-column context did not fold the left column, so it asks nothing about it');
    }
    for (const width of [RAIL, 1440]) {
      await folded.setViewportSize({ width, height: 800 });
      const reachable = await reachablePlaces(folded);
      if (reachable !== 1) fail('reading-place-reachable', `${language} at ${width}px with the left column folded offers ${reachable} reading-place controls, want exactly one`);
    }
    await foldedContext.close();

    for (const [kind, page] of [['note', note], ['readings', readings]]) {
      const hasMark = await page.locator('.y-headermark').count() === 1;
      const hasPath = await page.locator('.y-headerpath').count() === 1;
      for (const width of WIDTHS) {
        await page.setViewportSize({ width, height: 800 });
        const row = await readRow(page, CLUSTER, LEAD);
        if (row.issue) broken(row.issue);
        const where = `${language} ${kind === 'note' ? PAGE : READINGS_PAGE} at ${width}px`;

        const switchDrawn = row.drawn.includes(RUBY);
        if (switchDrawn) {
          fail('no-switch-in-the-header', `${where}: the furigana switch is drawn in the header, which belongs to the page head's toolbar`);
        }
        const want = CLUSTER.filter((selector) => {
          if (selector === RUBY) return false;
          if (selector === SEARCH_FIELD) return width > PHONE;
          if (selector === SEARCH) return width <= PHONE;
          if (selector === MARK) return hasMark;
          if (selector === PATH) return hasPath && width > DRAWER;
          if (PAPERS.includes(selector)) return width > DRAWER;
          if (selector === MARKS) return width > PHONE;
          return true;
        });
        // The furigana question is its own site, so the order is asked of the
        // rest of the cluster.
        const order = row.drawn.filter((entry) => entry !== RUBY);
        const wantOrder = want.filter((selector) => selector !== RUBY);
        if (JSON.stringify(order) !== JSON.stringify(wantOrder)) {
          fail('cluster-holds-only-what-reading-needs', `${where}: the row draws ${JSON.stringify(row.drawn)} after the wordmark, want ${JSON.stringify(want)}`);
        }
        // Ahead of the wordmark, the sidebar button where the column is a
        // drawer, and nothing at all where it is a column.
        const wantAhead = [SIDEBAR];
        if (JSON.stringify(row.ahead) !== JSON.stringify(wantAhead)) {
          fail('cluster-holds-only-what-reading-needs', `${where}: the row draws ${JSON.stringify(row.ahead)} ahead of the wordmark, want ${JSON.stringify(wantAhead)}`);
        }

        if (row.headerLeft < -0.5 || row.headerRight > row.windowWidth + 0.5 || row.headerOverflow > 0.5) {
          fail('row-fits', `${where}: the row runs past the window — ${JSON.stringify({ left: row.headerLeft, right: row.headerRight, windowWidth: row.windowWidth, overflow: row.headerOverflow })}`);
        }
        if (row.wordmarkLost > 0) {
          fail('wordmark-whole', `${where}: the wordmark is ${row.wordmarkLost}px short of its own text, so the name of the book is paying for the row`);
        }
        if (row.overlaps.length > 0) {
          fail('row-fits', `${where}: controls in the row are drawn over each other: ${JSON.stringify(row.overlaps)}`);
        }

        // The folder's name is drawn at least four letters wide or not at all,
        // and on a phone it is not drawn beside the furigana switch, which
        // keeps its word.
        const vault = await page.evaluate(() => {
          const link = document.querySelector('.y-brand__vault');
          if (!link) return null;
          const style = getComputedStyle(link);
          const box = link.getBoundingClientRect();
          return { drawn: style.display !== 'none' && box.width > 0, width: box.width, em: parseFloat(style.fontSize) };
        });
        if (vault) {
          if (!vault.drawn) {
            fail('row-fits', `${where}: the folder's name is not drawn, nothing beside it takes its room`);
          }
          if (vault.width < 4 * vault.em - 0.5) {
            fail('row-fits', `${where}: the folder's name is ${vault.width}px wide, less than the four letters it is drawn at least as wide as`);
          }
        }
      }
    }

    // The bookmark is a toggle: a name, and a pressed state that is on once
    // the place kept is in this note.
    await note.setViewportSize({ width: 390, height: 800 });
    const before = await note.evaluate((selector) => {
      const buttons = [...document.querySelectorAll(selector)];
      if (buttons.length !== 1) return { count: buttons.length };
      const button = buttons[0];
      return {
        count: 1,
        tag: button.tagName,
        name: (button.getAttribute('aria-label') || '').trim(),
        pressed: button.getAttribute('aria-pressed'),
      };
    }, MARK);
    if (before.count !== 1 || before.tag !== 'BUTTON' || !before.name || !['true', 'false'].includes(before.pressed)) {
      fail('bookmark-says-whether-it-is-set', `${language}: the row's bookmark reads ${JSON.stringify(before)}, want one named button with a pressed state`);
    }
    // At this width the search and the settings are drawn as marks alone, so
    // the label each carries is the only name a screen reader can say. The
    // field that stands for the search on a wider row shows only a
    // placeholder, which is not a name, so it carries one of its own too.
    const names = await note.evaluate((selectors) => selectors.map((selector) => (
      (document.querySelector(`header ${selector}`)?.getAttribute('aria-label') || '').trim()
    )), [SEARCH, SETTINGS, SEARCH_FIELD]);
    if (names.some((name) => !name)) {
      fail('drawn-marks-are-named', `${language}: the search glass, the settings and the search field are named ${JSON.stringify(names)}, want a name on each`);
    }
    const rowBefore = await rowRects(note);
    await note.locator(MARK).click();
    await note.locator('.y-headermark [data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
    // The sentence is a small label placed against the row, so nothing in the
    // row moved for it; at this width it covers the wordmark, clear of the
    // sidebar button at the start of the row and short of the search beside it.
    const saved = await note.evaluate(() => {
      const label = document.querySelector('.y-headermark__said');
      const box = label.getBoundingClientRect();
      const search = document.querySelector('[data-search-open]').getBoundingClientRect();
      const sidebar = document.querySelector('.y-sidebtn').getBoundingClientRect();
      return { position: getComputedStyle(label).position, left: box.left, right: box.right, searchLeft: search.left, sidebarRight: sidebar.right, sidebarDrawn: sidebar.width > 0, text: label.textContent };
    });
    const rowAfter = await rowRects(note);
    if (JSON.stringify(rowBefore) !== JSON.stringify(rowAfter)) {
      fail('saved-label-stays-out-of-the-row', `${language}: the row moved when the place was kept: ${JSON.stringify(rowBefore)} then ${JSON.stringify(rowAfter)}`);
    }
    if (saved.position !== 'absolute' || saved.right > saved.searchLeft + 0.5) {
      fail('saved-label-stays-out-of-the-row', `${language}: at 390px the saved label is ${saved.position} and ends at ${saved.right}px, past the search at ${saved.searchLeft}px`);
    }
    if (!saved.sidebarDrawn) broken('at 390px the note draws no sidebar button, so the label cannot be asked to stay clear of it');
    if (saved.left < saved.sidebarRight - 0.5) {
      fail('saved-label-stays-out-of-the-row', `${language}: at 390px the saved label starts at ${saved.left}px, over the sidebar button that ends at ${saved.sidebarRight}px`);
    }
    const after = await note.evaluate((selector) => {
      const button = document.querySelector(selector);
      return { pressed: button.getAttribute('aria-pressed'), fill: getComputedStyle(button.querySelector('svg')).fill };
    }, MARK);
    if (after.pressed !== 'true' || after.fill === 'none') {
      fail('bookmark-says-whether-it-is-set', `${language}: after the place was kept the bookmark reads ${JSON.stringify(after)}, want pressed and filled`);
    }
    // Wide, the label hangs under the row at its right end and never covers the
    // title, and nothing in the row — the field, the grounds, the link to My
    // marks among it — moves for it there either.
    await note.setViewportSize({ width: 1440, height: 800 });
    const wideBefore = await rowRects(note);
    await note.locator(MARK).click();
    await note.locator('.y-headermark [data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
    const wide = await note.evaluate(() => {
      const label = document.querySelector('.y-headermark__said').getBoundingClientRect();
      const header = document.querySelector('.y-header').getBoundingClientRect();
      const title = document.querySelector('.y-article .y-title').getBoundingClientRect();
      const overlap = label.left < title.right && label.right > title.left && label.top < title.bottom && label.bottom > title.top;
      return { top: label.top, headerBottom: header.bottom, overlap };
    });
    if (wide.top < wide.headerBottom - 0.5 || wide.overlap) {
      fail('saved-label-stays-out-of-the-row', `${language}: at 1440px the saved label sits at ${wide.top}px against a header ending at ${wide.headerBottom}px${wide.overlap ? ' and covers the title' : ''}, want it under the row and clear of the title`);
    }
    const wideAfter = await rowRects(note);
    if (JSON.stringify(wideBefore) !== JSON.stringify(wideAfter)) {
      fail('saved-label-stays-out-of-the-row', `${language}: at 1440px the row moved when the place was kept: ${JSON.stringify(wideBefore)} then ${JSON.stringify(wideAfter)}`);
    }
    // The sentence is not on a timer. It is still there well past the four
    // seconds and the fade it used to be given, with the reader doing nothing.
    const said = () => note.evaluate(() => document.querySelector('.y-headermark__said').textContent);
    await note.waitForTimeout(5500);
    if (await said() === '') {
      fail('saved-label-stays-out-of-the-row', `${language}: the saved label had gone five and a half seconds after the place was kept, with the reader doing nothing, want it there until their next press or key`);
    }
    // The reader's next key takes it away: one that does nothing else.
    await note.keyboard.press('Shift');
    await note.waitForFunction(() => document.querySelector('.y-headermark__said').textContent === '', null, { timeout: 2000 }).catch(() => {
      fail('saved-label-stays-out-of-the-row', `${language}: the saved label was still on the page after the reader's next key`);
    });
    // And so does their next press, on something that does nothing else.
    await note.locator(MARK).click();
    await note.locator('.y-headermark [data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
    const title = await note.locator('.y-article .y-title').boundingBox();
    if (!title) broken('the note draws no title to press beside the label');
    await note.mouse.click(title.x + 2, title.y + title.height / 2);
    await note.waitForFunction(() => document.querySelector('.y-headermark__said').textContent === '', null, { timeout: 2000 }).catch(() => {
      fail('saved-label-stays-out-of-the-row', `${language}: the saved label was still on the page after the reader's next press`);
    });
    await note.setViewportSize({ width: 390, height: 800 });
    if (proveRewrite && !proveRewrite()) notApplied('the module rewrite found no single needle to change');

    // A reader on the keyboard reaches the cluster from the folder's name, the
    // last stop of the brand, in the order the row draws it: on a phone, and
    // where the row also carries the copy of the path, the grounds and the link
    // to My marks.
    for (const [width, stops] of [[390, [SEARCH, MARK, SETTINGS]], [1440, [SEARCH_FIELD, MARK, PATH, ...PAPERS, MARKS, SETTINGS]]]) {
      await note.setViewportSize({ width, height: 800 });
      await note.reload({ waitUntil: 'load' });
      if (mutation?.on === 'note' && mutation.apply) await mutation.apply(note);
      await note.locator('.y-brand__vault').focus();
      const walked = [];
      for (let press = 0; press < stops.length; press += 1) {
        await note.keyboard.press('Tab');
        walked.push(await note.evaluate((selectors) => selectors.findIndex((selector) => document.activeElement?.matches(selector) ?? false), stops));
      }
      const want = stops.map((_, i) => i).join(',');
      if (walked.join(',') !== want) {
        fail('keyboard-walks-the-cluster', `${language} at ${width}px: tabbing from the folder's name reached ${JSON.stringify(walked)}, want ${JSON.stringify(stops)} in turn (-1 is somewhere else)`);
      }
    }
    // On a phone the sidebar button is the stop just ahead of the wordmark,
    // where it is drawn.
    await note.setViewportSize({ width: 390, height: 800 });
    await note.locator('.y-brand__name').focus();
    await note.keyboard.press('Shift+Tab');
    if (!(await note.evaluate((selector) => document.activeElement?.matches(selector) ?? false, SIDEBAR))) {
      fail('keyboard-walks-the-cluster', `${language} at 390px: tabbing back from the wordmark did not reach the sidebar button ahead of it`);
    }
    await context.close();

    // A thumb is wider than a pointer.
    const touch = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true });
    if (language === 'en') await touch.addCookies([{ name: 'yomihon_lang', value: 'en', url: BASE }]);
    const tapping = await open(touch, 'touch', PAGE);
    if (!(await tapping.evaluate(() => matchMedia('(pointer: coarse)').matches))) broken('the touch context does not report a coarse pointer');
    const target = await tapping.locator(MARK).boundingBox();
    if (!target || target.width < 44 || target.height < 44) {
      fail('bookmark-takes-a-thumb', `${language}: the bookmark's touch target is ${JSON.stringify(target)}, want at least 44 by 44`);
    }
    await touch.close();
  }

  console.log('PASS header-row: the sidebar button stands ahead of the wordmark at every width; after it the row carries no furigana switch, the search field (the glass on a phone), the bookmark, the copy of the path and the grounds where the left column is not a drawer, My marks above a phone and the settings, in that order and nothing else; one reading-place control at every width; the row fits with the wordmark whole; the bookmark is a named toggle that turns on when the place is kept, and its sentence moves nothing, covers nothing and stays until the next press or key; the search and the settings keep their names; the bookmark takes a thumb and is in the keyboard order');
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
