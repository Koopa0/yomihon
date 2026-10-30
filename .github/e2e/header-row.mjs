// Browser lock for the header row: it carries only what reading needs. To the
// right of the wordmark there is the furigana switch on a page that has
// furigana, the bookmark for keeping a reading place below the width the
// reading rail is drawn at, the search, and the way to the settings — in that
// order and nothing else. Everything a reader sets once and leaves alone lives
// on the settings page.
//
// What is asked here is what a reader can see and reach, measured rather than
// read off the stylesheet: at each width the row fits inside the window, the
// wordmark keeps all of its letters, the cluster holds exactly its controls in
// their order, and exactly one control for keeping a reading place is drawn —
// the bookmark in the row or the one in the rail, never both and never
// neither. The bookmark is then asked what a toggle owes a reader: a name, a
// pressed state that changes when the place is kept, a target a thumb can hit,
// and a place in the keyboard's order.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

// A note in the fixture vault whose body carries furigana, the one kind of
// page the switch for them is drawn on. PAGE is a note that carries none and
// offers to keep a reading place.
const READINGS_PAGE = '/notes/Writing/lessons/japanese/L01.md';

// The first width the reading rail is drawn at, which is the first width the
// rail carries the reading-place control and the row's bookmark steps aside.
// Both sides of it are measured, so a breakpoint that moved in either
// direction shows up here.
const RAIL = 1280;
const WIDTHS = [1440, RAIL + 1, RAIL, RAIL - 1, 1024, 938, 937, 720, 521, 390, 375];
const PLACE_WIDTHS = [937, 938, 1024, RAIL - 1, RAIL, RAIL + 1, 1440];

// Each control the cluster may hold, named by the hook that survives a
// restyling, in the order the row draws them.
const RUBY = '[data-ruby-toggle]';
const MARK = '.y-headermark [data-mark-button]';
const SEARCH = '[data-search-open]';
const SETTINGS = '.y-prefslink';
const CLUSTER = [RUBY, MARK, SEARCH, SETTINGS];

const SITES = [
  'reading-place-reachable',
  'cluster-holds-only-what-reading-needs',
  'switch-only-with-furigana',
  'row-fits',
  'wordmark-whole',
  'bookmark-says-whether-it-is-set',
  'bookmark-takes-a-thumb',
  'keyboard-walks-the-cluster',
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
  // The row's bookmark is drawn from the rail's width too, so a reader there
  // is shown two controls for one verb.
  'draw-the-bookmark-beside-the-rail': {
    target: 'reading-place-reachable',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', `@media (min-width: ${RAIL}px) { [data-js] .y-headermark { display: flex; } }`),
  },
  // The row's bookmark steps aside before the rail arrives, so a reader in
  // between has no way to keep a place at all.
  'hide-the-bookmark-early': {
    target: 'reading-place-reachable',
    on: 'note',
    apply: (page) => injectRule(page, '.y-headermark', `@media (min-width: 1200px) { [data-js] .y-headermark { display: none; } }`),
  },
  // Folding the left column takes the reading rail's control with it, so a
  // reader who folded the column has no way to keep a place at the widths
  // the row's bookmark steps aside at.
  'hide-the-rail-mark-under-a-folded-column': {
    target: 'reading-place-reachable',
    on: 'folded',
    apply: (page) => injectRule(page, '.y-rail-right .y-markset', "html[data-rail='collapsed'] .y-rail-right .y-markset { display: none; }"),
  },
  // The cluster's order changes: the search moves to the end of the row.
  'move-search-past-settings': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: (page) => injectRule(page, SEARCH, `${SEARCH} { order: 1; }`),
  },
  // A control that belongs on the settings page is drawn in the row again.
  'put-a-setting-back-in-the-row': {
    target: 'cluster-holds-only-what-reading-needs',
    on: 'note',
    apply: async (page) => {
      const placed = await page.evaluate(() => {
        const search = document.querySelector('[data-search-open]');
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
  // The switch is drawn whether or not the page has anything for it to show.
  'draw-the-switch-on-every-page': {
    target: 'switch-only-with-furigana',
    on: 'note',
    apply: async (page) => {
      const placed = await page.evaluate(() => {
        const search = document.querySelector('[data-search-open]');
        if (!search) return false;
        const ruby = document.createElement('button');
        ruby.type = 'button';
        ruby.className = 'y-iconbtn y-rubybtn';
        ruby.setAttribute('data-ruby-toggle', '');
        ruby.setAttribute('aria-pressed', 'true');
        search.before(ruby);
        return true;
      });
      if (!placed) notApplied('there is no search control to place the switch beside');
    },
  },
  // The switch is in the markup of a page with furigana and drawn nowhere.
  'hide-the-switch-with-furigana': {
    target: 'switch-only-with-furigana',
    on: 'readings',
    apply: (page) => injectRule(page, RUBY, `${RUBY} { display: none; }`),
  },
  // The row wants more width than the window has.
  'overflow-the-row': {
    target: 'row-fits',
    on: 'note',
    apply: (page) => injectRule(page, '.y-header', '.y-header { min-width: 700px; }'),
  },
  // A control grows and the wordmark pays for it.
  'let-a-control-take-the-name': {
    target: 'wordmark-whole',
    on: 'readings',
    apply: (page) => injectRule(page, '.y-rubybtn', '.y-rubybtn { min-width: 200px; }'),
  },
  // The bookmark loses the attribute that tells a screen reader it is a
  // toggle and whether it is on.
  'drop-the-pressed-state': {
    target: 'bookmark-says-whether-it-is-set',
    on: 'note',
    apply: (page) => changeOne(page, MARK, { name: 'aria-pressed', remove: true }),
  },
  // A place kept in this note leaves the bookmark saying it is not set.
  'forget-the-press-after-keeping': {
    target: 'bookmark-says-whether-it-is-set',
    on: 'note',
    before: (page) => rewriteScript(
      page,
      '**/mark.js{,?*}',
      "?.setAttribute('aria-pressed', 'true');",
      ';',
    ),
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
  await page.route('**/marks', async (route) => {
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
const readRow = (page, cluster) => page.evaluate((selectors) => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const windowWidth = ruler.getBoundingClientRect().width;
  ruler.remove();

  const header = document.querySelector('.y-header');
  const name = document.querySelector('.y-brand__name > span');
  const spacer = document.querySelector('.y-header__spacer');
  if (!header || !name || !spacer) return { issue: 'the page carries no header row, no wordmark or no spacer' };
  const box = header.getBoundingClientRect();
  const after = spacer.getBoundingClientRect().right - 0.5;
  // Every control drawn to the right of the spacer, left to right, named by
  // the hook it answers to — or by its own tag, when it answers to none of
  // them, which is what a control that does not belong here looks like.
  const drawn = [...header.querySelectorAll('button, a, input, select')]
    .filter((element) => element.checkVisibility() && element.getBoundingClientRect().left >= after)
    .sort((one, two) => one.getBoundingClientRect().left - two.getBoundingClientRect().left)
    .map((element) => selectors.find((selector) => element.matches(selector)) ?? element.outerHTML.slice(0, 80));
  return {
    windowWidth,
    headerLeft: box.left,
    headerRight: box.right,
    headerOverflow: header.scrollWidth - header.clientWidth,
    wordmarkLost: name.scrollWidth - name.clientWidth,
    drawn,
  };
}, cluster);

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
      const hasSwitch = kind === 'readings';
      const hasMark = await page.locator('.y-headermark').count() === 1;
      for (const width of WIDTHS) {
        await page.setViewportSize({ width, height: 800 });
        const row = await readRow(page, CLUSTER);
        if (row.issue) broken(row.issue);
        const where = `${language} ${kind === 'note' ? PAGE : READINGS_PAGE} at ${width}px`;

        const switchDrawn = row.drawn.includes(RUBY);
        if (switchDrawn !== hasSwitch) {
          fail('switch-only-with-furigana', `${where}: the furigana switch is ${switchDrawn ? 'drawn' : 'not drawn'}, want it ${hasSwitch ? 'drawn' : 'absent'} on a page ${hasSwitch ? 'with' : 'without'} furigana`);
        }
        const want = CLUSTER.filter((selector) => {
          if (selector === RUBY) return hasSwitch;
          if (selector === MARK) return hasMark && width < RAIL;
          return true;
        });
        // The furigana question is its own site, so the order is asked of the
        // rest of the cluster.
        const order = row.drawn.filter((entry) => entry !== RUBY);
        const wantOrder = want.filter((selector) => selector !== RUBY);
        if (JSON.stringify(order) !== JSON.stringify(wantOrder) || (hasSwitch && row.drawn[0] !== RUBY)) {
          fail('cluster-holds-only-what-reading-needs', `${where}: the row draws ${JSON.stringify(row.drawn)} after the wordmark, want ${JSON.stringify(want)}`);
        }

        if (row.headerLeft < -0.5 || row.headerRight > row.windowWidth + 0.5 || row.headerOverflow > 0.5) {
          fail('row-fits', `${where}: the row runs past the window — ${JSON.stringify({ left: row.headerLeft, right: row.headerRight, windowWidth: row.windowWidth, overflow: row.headerOverflow })}`);
        }
        if (row.wordmarkLost > 0) {
          fail('wordmark-whole', `${where}: the wordmark is ${row.wordmarkLost}px short of its own text, so the name of the book is paying for the row`);
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
    await note.locator(MARK).click();
    await note.locator('.y-headermark [data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
    const after = await note.evaluate((selector) => {
      const button = document.querySelector(selector);
      return { pressed: button.getAttribute('aria-pressed'), fill: getComputedStyle(button.querySelector('svg')).fill };
    }, MARK);
    if (after.pressed !== 'true' || after.fill === 'none') {
      fail('bookmark-says-whether-it-is-set', `${language}: after the place was kept the bookmark reads ${JSON.stringify(after)}, want pressed and filled`);
    }
    if (proveRewrite && !proveRewrite()) notApplied('the module rewrite found no single needle to change');

    // A reader on the keyboard reaches the cluster from the wordmark in the
    // order the row draws it.
    await note.reload({ waitUntil: 'load' });
    if (mutation?.on === 'note' && mutation.apply) await mutation.apply(note);
    await note.locator('.y-brand__name').focus();
    const walked = [];
    for (let press = 0; press < 3; press += 1) {
      await note.keyboard.press('Tab');
      walked.push(await note.evaluate((selectors) => selectors.findIndex((selector) => document.activeElement?.matches(selector) ?? false), [MARK, SEARCH, SETTINGS]));
    }
    if (walked.join(',') !== '0,1,2') {
      fail('keyboard-walks-the-cluster', `${language}: tabbing from the wordmark reached ${JSON.stringify(walked)}, want the bookmark, the search and the settings in turn (-1 is somewhere else)`);
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

  console.log('PASS header-row: the row carries the furigana switch only where there is furigana, the bookmark below the rail width, the search and the settings, in that order and nothing else; one reading-place control at every width; the row fits with the wordmark whole; the bookmark is a named toggle that turns on when the place is kept, takes a thumb, and is in the keyboard order');
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
