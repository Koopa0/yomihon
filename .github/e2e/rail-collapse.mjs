// Behavior lock for the left column folded away above the drawer width. The
// choice belongs to the reader and is kept on the machine, so what is asked
// here is what a reader can see and do: the column leaves and the text takes
// the room, the choice is still made after a reload and before any script has
// run, the key and the button and the filter key all agree about it, and none
// of it leaks into the drawer below the width where the column is one.
//
// Each site is independent: it opens its own page, sets its own state, and is
// the only place its mutation is applied. A mutation is therefore judged by the
// one assertion it aims at rather than by whichever fired first.
//
// Nothing here reads a frame in the middle of the fold. Motion is asked as what
// the stylesheet declares and what the page has settled into.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a book rail), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';

const RAIL = '#_y-nav-rail';
const BODY = '#_y-nav-rail-body';
const TOGGLE = '[data-rail-toggle]';
const ROW = '.y-railhead';
const NAV_TOGGLE = '[data-nav-toggle]';
const FILTER = '[data-nav-filter]';

// One page of each shape of rail: a book rail, a folder rail, a note with
// nothing in its right rail, the study-path page, and a page that mounts the
// shared sidebar. A choice honoured on some of them would come back on the
// others.
const SHAPES = [
  ['book rail', '/notes/Course/C02.md'],
  ['folder rail', '/notes/Notes/cutover.md'],
  ['rail-empty note', '/notes/README.md'],
  ['study path', '/syllabus/Maps/study.md'],
  ['shared sidebar', '/health'],
];

const STRIP = 40;

// The column's open widths: every shell's at the width where all three columns
// stand, and the note shell's below it, where it gives up the right rail and
// narrows to leave the text its room.
const OPEN = 272;
const NOTE_OPEN_NARROW = 248;
const THREE_COLUMNS = 1280;

// The reading measure of a note that declares no language, and a window in the
// band where the column is open beside text narrower than that measure. From
// the width where all three columns stand, the text is already at its measure
// with the column shown, so folding the column there has nothing to give back.
const MEASURE = 646;
const FOLD_WIDTH = 940;

const SITES = [
  'text-returns-to-the-measure',
  'choice-survives-a-reload',
  'first-paint-is-collapsed-without-script',
  'wide-key-leaves-the-drawer-alone',
  'drawer-key-still-opens-at-900',
  'slash-opens-the-column-first',
  'focus-leaves-the-hidden-panel',
  'control-is-a-sticky-sibling',
  'rail-scroll-is-restored',
  'collapsed-rule-stops-at-901',
  'crossing-900-leaves-nothing-stale',
  'wide-fold-sets-no-inertness',
  'state-is-told-by-the-control',
  'collapsed-panel-leaves-the-tree',
  'fold-is-animated',
  'nothing-animates-on-load',
  'reduced-motion-stops-the-fold',
  'reading-position-survives',
  'every-rail-page-folds',
  'control-revealed-before-runtime',
  'no-script-draws-no-control',
  'strip-is-one-button',
  'every-shell-keeps-its-open-width',
  'open-panel-keeps-its-padding-beside-a-scrollbar',
  'focus-is-never-hidden-under-the-head',
  'restore-does-not-animate',
  'restore-moves-focus-out',
  'off-shortcuts-do-not-advertise-the-key',
  'strip-ring-is-whole',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN rail-collapse: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL rail-collapse: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN rail-collapse: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED rail-collapse: ${message}`); };

// A rule injected at a selector that matches nothing styles nothing, and a
// mutation that styled nothing would be reported as caught by whatever the
// page was already doing.
const injectStyle = (css, mustMatch) => async (page) => {
  await page.addInitScript(({ rule, selector }) => {
    window.addEventListener('DOMContentLoaded', () => {
      const style = document.createElement('style');
      style.dataset.railCollapseMutation = String(document.querySelectorAll(selector).length);
      style.textContent = rule;
      document.head.append(style);
    }, { once: true });
  }, { rule: css, selector: mustMatch });
  return async () => {
    const seen = await page.evaluate(() => document.querySelector('style[data-rail-collapse-mutation]')?.dataset.railCollapseMutation ?? null);
    if (seen === null) return 'the injected style was never added';
    if (Number(seen) < 1) return `${mustMatch} matched nothing, so the injected rule styles nothing`;
    return '';
  };
};

const rewriteAsset = (glob, needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(glob, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests < 1) return `${glob} was never requested`;
    if (matches !== requests) return `needle matched ${matches} times over ${requests} requests, want exactly 1 each`;
    return '';
  };
};

const rewriteDocument = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/*', async (route) => {
    if (route.request().resourceType() !== 'document') {
      await route.fallback();
      return;
    }
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests < 1) return 'no document was requested';
    if (matches !== requests) return `document needle matched ${matches} times over ${requests} requests, want exactly 1 each`;
    return '';
  };
};

const MUTATIONS = {
  // The collapsed grid rule gone: the column empties and the track stays.
  'drop-the-collapsed-grid-rule': {
    target: 'text-returns-to-the-measure',
    apply: injectStyle(
      "html[data-rail='collapsed'] .y-shell:has(> .y-rail-left){grid-template-columns:var(--rail-open) minmax(0,1fr) 316px!important}"
      + "@media(max-width:1280px){html[data-rail='collapsed'] .y-shell:has(> .y-rail-left){grid-template-columns:var(--rail-open) minmax(0,1fr)!important}}",
      '.y-shell:has(> .y-rail-left)',
    ),
  },
  // The choice held in the root and never in the cookie.
  'drop-the-cookie-write': {
    target: 'choice-survives-a-reload',
    apply: rewriteAsset(
      '**/preferences.js{,?*}',
      // Built in two pieces so the source names no template placeholder.
      '    document.cookie = `yomihon_$' + '{name}=$' + '{value};path=/;max-age=31536000;samesite=lax`;\n',
      '',
    ),
  },
  // The fold waiting on the script that arrives after the first paint.
  'gate-the-fold-on-script': {
    target: 'first-paint-is-collapsed-without-script',
    apply: async (page) => {
      let requests = 0;
      let matches = 0;
      await page.route('**/app.css{,?*}', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const count = original.split("html[data-rail='collapsed']").length - 1;
        matches += count;
        await route.fulfill({ response, body: original.replaceAll("html[data-rail='collapsed']", "html[data-js][data-rail='collapsed']") });
      });
      return () => (requests >= 1 && matches >= 1 ? '' : `stylesheet requested ${requests} times, needle matched ${matches}`);
    },
  },
  // The key reaching the drawer at every width, the column never.
  'arm-the-drawer-key-at-every-width': {
    target: 'wide-key-leaves-the-drawer-alone',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '      if (drawer.isNarrow()) {', '      if (true) {'),
  },
  // The column key claiming the narrow widths too.
  'arm-the-column-key-at-every-width': {
    target: 'drawer-key-still-opens-at-900',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '      if (drawer.isNarrow()) {', '      if (false) {'),
  },
  // Focus asked of a filter inside a panel nobody can see.
  'focus-the-filter-without-opening-the-column': {
    target: 'slash-opens-the-column-first',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '        rail.expand();\n', ''),
  },
  // Focus left in a panel that has just left the tree.
  'leave-focus-in-the-panel': {
    target: 'focus-leaves-the-hidden-panel',
    apply: rewriteAsset('**/preferences.js{,?*}', '      railToggle?.focus();\n', ''),
  },
  // The panel free to reflow while the column moves, so the position the rail
  // was saved at means something else by the time it is put back.
  'let-the-panel-reflow-while-moving': {
    target: 'rail-scroll-is-restored',
    apply: rewriteAsset('**/rail.js{,?*}', "    root.dataset.railMoving = '';\n", ''),
  },
  // The control scrolling away with the tree.
  'unstick-the-control': {
    target: 'control-is-a-sticky-sibling',
    apply: injectStyle('.y-railhead{position:static!important}', '.y-railhead'),
  },
  // The rail coming back at the top.
  'forget-the-rail-scroll': {
    target: 'rail-scroll-is-restored',
    apply: rewriteAsset('**/rail.js{,?*}', '    rail.scrollTop = scrollTop;\n', ''),
  },
  // The collapsed rule reaching below 901px, where it would empty the drawer.
  'collapsed-rule-leaks-below-901': {
    target: 'collapsed-rule-stops-at-901',
    apply: injectStyle("html[data-rail='collapsed'] .y-railbody{display:none!important}", '.y-railbody'),
  },
  // The desktop fold leaving the rail inert, which the drawer's own rule
  // would have to undo on every crossing.
  'inert-the-folded-rail': {
    target: 'wide-fold-sets-no-inertness',
    apply: rewriteAsset('**/rail.js{,?*}', "    preferences.writeRail('collapsed');\n", "    preferences.writeRail('collapsed');\n    rail.inert = true;\n"),
  },
  // The drawer's state written when the column is folded.
  'fold-the-column-by-the-drawer-state': {
    target: 'crossing-900-leaves-nothing-stale',
    apply: rewriteAsset('**/rail.js{,?*}', "    preferences.writeRail('collapsed');\n", "    preferences.writeRail('collapsed');\n    document.documentElement.dataset.nav = 'open';\n"),
  },
  // The button's state left where it was.
  'leave-aria-expanded-stale': {
    target: 'state-is-told-by-the-control',
    apply: rewriteAsset('**/preferences.js{,?*}', "    railToggle.setAttribute('aria-expanded', String(!collapsed));\n", ''),
  },
  // The panel left standing in a column that is folded.
  'keep-the-panel-when-folded': {
    target: 'collapsed-panel-leaves-the-tree',
    apply: injectStyle("html[data-rail='collapsed'] .y-railbody{display:block!important;opacity:1!important}", '.y-railbody'),
  },
  // The fold with no motion declared.
  'declare-no-transition': {
    target: 'fold-is-animated',
    apply: injectStyle('.y-shell:has(> .y-rail-left),.y-shell2:has(> .y-rail-left){transition:none!important}', '.y-shell:has(> .y-rail-left)'),
  },
  // The state arriving by script after the first paint, so the page paints
  // open and then folds — which is a transition on every load.
  'fold-after-the-first-paint': {
    target: 'nothing-animates-on-load',
    apply: async (page) => {
      await page.addInitScript(() => {
        window.addEventListener('DOMContentLoaded', () => {
          document.documentElement.dataset.rail = 'collapsed';
        }, { once: true });
      });
      const rewrite = await rewriteDocument('data-rail="collapsed"', 'data-rail="open"')(page);
      return rewrite;
    },
  },
  // A duration that wins over the reduced-motion blanket.
  'animate-through-reduced-motion': {
    target: 'reduced-motion-stops-the-fold',
    apply: injectStyle('html .y-shell:has(> .y-rail-left){transition-duration:120ms!important}', '.y-shell:has(> .y-rail-left)'),
  },
  // Anchoring off, so a reflow moves the reader.
  'turn-off-scroll-anchoring': {
    target: 'reading-position-survives',
    apply: injectStyle('html,body,.y-main{overflow-anchor:none!important}', 'html'),
  },
  // Only the note shell folding.
  'fold-only-the-note-shell': {
    target: 'every-rail-page-folds',
    apply: injectStyle(
      "html[data-rail='collapsed'] .y-shell2:has(> .y-rail-left){grid-template-columns:var(--rail-open) minmax(0,1fr)!important}",
      '.y-shell2',
    ),
  },
  // The control revealed by the runtime and not before it.
  'reveal-only-with-the-runtime': {
    target: 'control-revealed-before-runtime',
    apply: rewriteDocument('head.hidden = false;', ''),
  },
  // The control drawn where no script can answer it.
  'draw-the-control-without-script': {
    target: 'no-script-draws-no-control',
    apply: rewriteDocument('<div class="y-railhead" hidden>', '<div class="y-railhead">'),
  },
  // The note shell's narrow rail reaching the shells that keep the wide one.
  'give-the-study-shell-the-narrow-rail': {
    target: 'every-shell-keeps-its-open-width',
    apply: injectStyle('.y-shell2{--rail-open:248px!important}', '.y-shell2'),
  },
  // The panel floored at the column's width whether or not it is folding.
  'floor-the-open-panel-at-the-column-width': {
    target: 'open-panel-keeps-its-padding-beside-a-scrollbar',
    apply: injectStyle('.y-railbody{min-width:calc(var(--rail-open) - 21px)!important}', '.y-railbody'),
  },
  // Scroll padding gone, so focus scrolls a row under the sticky head.
  'drop-the-scroll-padding': {
    target: 'focus-is-never-hidden-under-the-head',
    apply: injectStyle('.y-rail-left{scroll-padding-top:0!important}', '.y-rail-left'),
  },
  // The restore's write left to animate.
  'animate-the-restore': {
    target: 'restore-does-not-animate',
    apply: rewriteAsset('**/preferences.js{,?*}', "    root.dataset.railSettling = '';\n", ''),
  },
  // The restore forgetting to move focus out of a panel it hides.
  'restore-strands-focus': {
    target: 'restore-moves-focus-out',
    apply: rewriteAsset('**/preferences.js{,?*}', '    keepFocusOutOfFoldedRail(rail);\n', ''),
  },
  // The key advertised whatever the setting.
  'leave-the-key-advertised': {
    target: 'off-shortcuts-do-not-advertise-the-key',
    apply: rewriteAsset(
      '**/preferences.js{,?*}',
      "    if (keys) railToggle.setAttribute('aria-keyshortcuts', '[');\n    else railToggle.removeAttribute('aria-keyshortcuts');\n",
      "    railToggle.setAttribute('aria-keyshortcuts', '[');\n",
    ),
  },
  // The usual ring offset, which runs a pixel past the strip.
  'restore-the-ring-offset': {
    target: 'strip-ring-is-whole',
    apply: injectStyle("html[data-rail='collapsed'] .y-railtoggle__button:focus-visible{outline-offset:2px!important}", '.y-railtoggle__button'),
  },
  // The whole strip answering a click.
  'make-the-strip-a-target': {
    target: 'strip-is-one-button',
    apply: async (page) => {
      await page.addInitScript(() => {
        window.addEventListener('DOMContentLoaded', () => {
          document.querySelector('#_y-nav-rail')?.addEventListener('click', () => document.querySelector('[data-rail-toggle]').click());
        }, { once: true });
      });
      return async () => '';
    },
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`rail-collapse: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`rail-collapse: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`rail-collapse: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
// A second browser that keeps its scrollbars. Headless Chrome hides them by
// default, so every other page here is one where a scrolling rail loses no room
// to a scrollbar, which is the case that hides how the panel fits beside one.
let classicBrowser = null;
const classic = async () => {
  classicBrowser ??= await chromium.launch({ channel: 'chrome', headless: true, ignoreDefaultArgs: ['--hide-scrollbars'] });
  return classicBrowser;
};

// The transitions a page starts, from before its first script runs.
const recordTransitions = () => {
  window.__railTransitions = [];
  document.addEventListener('transitionrun', (event) => {
    window.__railTransitions.push(event.propertyName);
  }, true);
};

// The page has arrived: the block of content under the header has finished
// coming forward. That arrival scales the block while it plays, so a box read
// from inside it is a box the page is passing through and not the one it
// settles at, and a baseline taken then differs from a measurement taken after
// it by the scale at that moment. Nothing is waited for on a page the arrival
// is gated off, which has no such animation, and only once the stylesheet is
// applied can the answer be that there is none. The shimmer and the spinner
// never finish, which is why the wait names the arrival and not every
// animation.
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

// Opens a page in a context of its own. The mutation is applied only when the
// site being run is the one it aims at, so every other page is the real one.
const open = async (site, path, {
  width = 1281, height = 800, collapsed = false, script = true, reducedMotion = 'no-preference', wait = true, mutate = true, scrollbars = false,
} = {}) => {
  const context = await (scrollbars ? await classic() : browser).newContext({ viewport: { width, height }, javaScriptEnabled: script, reducedMotion });
  if (collapsed) await context.addCookies([{ name: 'yomihon_rail', value: 'collapsed', url: BASE }]);
  const page = await context.newPage();
  await page.addInitScript(recordTransitions);
  const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
  let proof = null;
  if (mutate && mutation && mutation.target === site) proof = await mutation.apply(page);
  // With no script nothing else says the page has finished, and a stylesheet
  // still in flight would be measured as an unstyled page.
  await page.goto(BASE + path, { waitUntil: script ? 'domcontentloaded' : 'load' });
  if (script && wait) {
    await page.waitForSelector('html[data-js]');
    await arrived(page);
  }
  return {
    page,
    context,
    async checkProof() {
      if (!proof) return;
      const issue = await proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    },
  };
};

// The page settled: no transition is still running, so a measurement is of the
// state the page is in and not of a frame on the way to it.
const settled = (page) => page.waitForFunction(
  () => document.getAnimations().filter((animation) => animation instanceof CSSTransition).length === 0,
  null,
  { timeout: 3000 },
);

const geometry = (page) => page.evaluate(({ rail, body, toggle }) => {
  const box = (selector) => {
    const element = document.querySelector(selector);
    if (!element) return null;
    const rect = element.getBoundingClientRect();
    return { left: rect.left, top: rect.top, width: rect.width, height: rect.height, display: getComputedStyle(element).display };
  };
  const main = document.querySelector('main');
  return {
    rail: box(rail),
    body: box(body),
    toggle: box(toggle),
    main: main ? { left: main.getBoundingClientRect().left, width: main.getBoundingClientRect().width } : null,
    prose: box('.y-prose'),
    state: document.documentElement.dataset.rail,
    nav: document.documentElement.dataset.nav,
    expanded: document.querySelector(toggle)?.getAttribute('aria-expanded'),
    label: document.querySelector(toggle)?.getAttribute('aria-label'),
    title: document.querySelector(toggle)?.title,
    controls: document.querySelector(toggle)?.getAttribute('aria-controls'),
    inert: document.querySelector(rail)?.inert,
    hidden: document.querySelector(rail)?.getAttribute('aria-hidden'),
    focusInToggle: document.activeElement === document.querySelector(toggle),
    focusInFilter: document.activeElement === document.querySelector('[data-nav-filter]'),
  };
}, { rail: RAIL, body: BODY, toggle: TOGGLE });

const key = async (page, name) => {
  await page.keyboard.press(name);
};

const cookieValue = async (context) => (await context.cookies(BASE)).find((cookie) => cookie.name === 'yomihon_rail')?.value ?? null;

const run = async (site, work) => {
  if (MUTATE && MUTATIONS[MUTATE].target !== site) return;
  await work();
};

try {
  await run('text-returns-to-the-measure', async () => {
    const { page, context, checkProof } = await open('text-returns-to-the-measure', PAGE, { width: FOLD_WIDTH });
    await checkProof();
    const before = await geometry(page);
    if (!before.prose || before.prose.width >= MEASURE) broken(`the fixture page's text is ${before.prose?.width}px wide with the column shown at ${FOLD_WIDTH}, so collapsing it could not widen the text`);
    if (Math.round(before.rail.width) !== NOTE_OPEN_NARROW) broken(`the column is ${before.rail.width}px at ${FOLD_WIDTH}, want ${NOTE_OPEN_NARROW}`);
    await key(page, '[');
    await settled(page);
    const after = await geometry(page);
    if (Math.round(after.prose.width) !== MEASURE) fail('text-returns-to-the-measure', `collapsing left the text ${after.prose.width}px wide, want ${MEASURE}`);
    if (Math.round(after.rail.width) !== STRIP) fail('text-returns-to-the-measure', `the folded column is ${after.rail.width}px wide, want the ${STRIP}px strip`);
    await key(page, '[');
    await settled(page);
    const back = await geometry(page);
    if (Math.round(back.rail.width) !== NOTE_OPEN_NARROW || Math.round(back.prose.width) !== Math.round(before.prose.width)) {
      fail('text-returns-to-the-measure', `expanding again left the column ${back.rail.width}px and the text ${back.prose.width}px, want ${NOTE_OPEN_NARROW} and ${before.prose.width}`);
    }
    await context.close();
  });

  await run('choice-survives-a-reload', async () => {
    const { page, context, checkProof } = await open('choice-survives-a-reload', PAGE);
    await checkProof();
    await key(page, '[');
    await settled(page);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    const after = await geometry(page);
    if (after.state !== 'collapsed' || Math.round(after.rail.width) !== STRIP) {
      fail('choice-survives-a-reload', `after a reload the column is ${after.rail.width}px with rail=${after.state}, want the strip and collapsed (cookie=${await cookieValue(context)})`);
    }
    await key(page, '[');
    await settled(page);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    const reopened = await geometry(page);
    if (reopened.state !== 'open' || Math.round(reopened.rail.width) !== OPEN) {
      fail('choice-survives-a-reload', `after expanding and reloading the column is ${reopened.rail.width}px with rail=${reopened.state}, want ${OPEN} and open`);
    }
    await context.close();
  });

  await run('first-paint-is-collapsed-without-script', async () => {
    // No script at all: the only thing that can have folded the column is what
    // the server wrote and the stylesheet read.
    const { page, context, checkProof } = await open('first-paint-is-collapsed-without-script', PAGE, { collapsed: true, script: false });
    await checkProof();
    const g = await geometry(page);
    if (g.state !== 'collapsed') broken(`the server rendered rail=${g.state} for a collapsed cookie`);
    if (Math.round(g.rail.width) !== STRIP || g.body.display !== 'none') {
      fail('first-paint-is-collapsed-without-script', `with no script the column is ${g.rail.width}px and its panel display=${g.body.display}, want the strip and none`);
    }
    await context.close();
  });

  await run('wide-key-leaves-the-drawer-alone', async () => {
    const { page, context, checkProof } = await open('wide-key-leaves-the-drawer-alone', PAGE, { width: 1024 });
    await checkProof();
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (g.state !== 'collapsed' || g.nav === 'open') {
      fail('wide-key-leaves-the-drawer-alone', `[ at 1024 left rail=${g.state} and nav=${g.nav}, want collapsed and the drawer closed`);
    }
    await context.close();
  });

  await run('drawer-key-still-opens-at-900', async () => {
    const { page, context, checkProof } = await open('drawer-key-still-opens-at-900', PAGE, { width: 900 });
    await checkProof();
    await key(page, '[');
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open', null, { timeout: 2000 }).catch(() => {});
    const g = await geometry(page);
    if (g.nav !== 'open' || g.state === 'collapsed') {
      fail('drawer-key-still-opens-at-900', `[ at 900 left nav=${g.nav} and rail=${g.state}, want the drawer open and the column untouched`);
    }
    await context.close();
  });

  await run('slash-opens-the-column-first', async () => {
    const { page, context, checkProof } = await open('slash-opens-the-column-first', PAGE, { width: 1024, collapsed: true });
    await checkProof();
    if ((await geometry(page)).state !== 'collapsed') broken('the page did not start folded');
    await key(page, '/');
    await settled(page);
    const g = await geometry(page);
    if (g.state !== 'open' || !g.focusInFilter) {
      fail('slash-opens-the-column-first', `/ while folded left rail=${g.state} and filterFocused=${g.focusInFilter}, want the column open with the filter focused`);
    }
    await context.close();
  });

  await run('focus-leaves-the-hidden-panel', async () => {
    const { page, context, checkProof } = await open('focus-leaves-the-hidden-panel', PAGE, { width: 1024 });
    await checkProof();
    await page.locator(`${BODY} a[href]`).first().focus();
    if (await page.evaluate(() => document.activeElement?.closest('#_y-nav-rail-body') === null)) broken('focus did not enter the panel');
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (!g.focusInToggle) {
      const held = await page.evaluate(() => document.activeElement?.tagName);
      fail('focus-leaves-the-hidden-panel', `folding with focus in the panel left focus on ${held}, want the control`);
    }
    // The filter shares the head with the button and goes with the panel, so it
    // is a place focus can be when the fold happens too.
    await key(page, '[');
    await settled(page);
    await page.locator(FILTER).focus();
    await page.evaluate(() => document.querySelector('[data-rail-toggle]').click());
    await settled(page);
    const fromFilter = await geometry(page);
    if (!fromFilter.focusInToggle) fail('focus-leaves-the-hidden-panel', 'folding with focus in the filter did not move it to the control');
    await context.close();
  });

  // A rail long enough to scroll, made by repeating what is in it.
  const makeRailScroll = (page) => page.evaluate((selector) => {
    const body = document.querySelector(selector);
    const rows = [...body.children];
    for (let i = 0; i < 12; i += 1) for (const row of rows) body.append(row.cloneNode(true));
    const rail = document.querySelector('#_y-nav-rail');
    return rail.scrollHeight - rail.clientHeight;
  }, BODY);

  await run('control-is-a-sticky-sibling', async () => {
    const { page, context, checkProof } = await open('control-is-a-sticky-sibling', PAGE, { width: 1024, height: 600 });
    await checkProof();
    const structure = await page.evaluate(({ body, toggle }) => {
      const button = document.querySelector(toggle);
      const panel = document.querySelector(body);
      return {
        inside: panel.contains(button),
        sibling: button.closest('.y-railhead')?.parentElement === panel.parentElement,
        controlsResolves: document.getElementById(button.getAttribute('aria-controls')) === panel,
      };
    }, { body: BODY, toggle: TOGGLE });
    if (structure.inside || !structure.sibling || !structure.controlsResolves) {
      fail('control-is-a-sticky-sibling', `control structure ${JSON.stringify(structure)}, want a sibling of the panel whose aria-controls names it`);
    }
    const room = await makeRailScroll(page);
    if (room < 200) broken(`the rail could scroll only ${room}px after being lengthened`);
    await page.evaluate(() => { document.querySelector('#_y-nav-rail').scrollTop = 400; });
    const offsets = await page.evaluate(({ row: rowSelector }) => {
      const rail = document.querySelector('#_y-nav-rail').getBoundingClientRect();
      const row = document.querySelector(rowSelector).getBoundingClientRect();
      return { row: row.top, rail: rail.top, scrolled: document.querySelector('#_y-nav-rail').scrollTop };
    }, { row: ROW });
    if (offsets.scrolled < 300) broken(`the rail scrolled only ${offsets.scrolled}px`);
    if (Math.abs(offsets.row - offsets.rail) > 1) {
      fail('control-is-a-sticky-sibling', `with the tree scrolled ${offsets.scrolled}px the control is ${offsets.row - offsets.rail}px from the top of the rail, want it held there`);
    }
    const filterAt = await page.evaluate(() => {
      const rail = document.querySelector('#_y-nav-rail').getBoundingClientRect();
      const filter = document.querySelector('[data-nav-filter]').getBoundingClientRect();
      return { top: filter.top - rail.top, bottom: filter.bottom - rail.top };
    });
    if (filterAt.top < 0 || filterAt.bottom > 44) {
      fail('control-is-a-sticky-sibling', `with the tree scrolled the filter is at ${filterAt.top}-${filterAt.bottom}px from the top of the rail, want it inside the sticky row`);
    }
    await context.close();
  });

  await run('rail-scroll-is-restored', async () => {
    const { page, context, checkProof } = await open('rail-scroll-is-restored', PAGE, { width: 1024, height: 600 });
    await checkProof();
    await makeRailScroll(page);
    await page.evaluate(() => { document.querySelector('#_y-nav-rail').scrollTop = 500; });
    const from = await page.evaluate(() => document.querySelector('#_y-nav-rail').scrollTop);
    if (from < 300) broken(`the rail scrolled only ${from}px`);
    // Pressed from the page rather than by the driver, which scrolls a sticky
    // button's natural place into view before it clicks; a reader presses the
    // one they can see, and nothing scrolls.
    const press = () => page.evaluate(() => document.querySelector('[data-rail-toggle]').click());
    await press();
    await settled(page);
    await press();
    await settled(page);
    const to = await page.evaluate(() => document.querySelector('#_y-nav-rail').scrollTop);
    if (Math.abs(to - from) > 2) fail('rail-scroll-is-restored', `the rail was at ${from}px, is at ${to}px after folding and opening, want it put back`);
    await context.close();
  });

  await run('collapsed-rule-stops-at-901', async () => {
    const { page, context, checkProof } = await open('collapsed-rule-stops-at-901', PAGE, { width: 900, collapsed: true });
    await checkProof();
    const g = await geometry(page);
    if (g.state !== 'collapsed') broken('the page did not carry the collapsed choice');
    if (g.body.display === 'none') fail('collapsed-rule-stops-at-901', 'at 900 a collapsed choice has emptied the drawer: the panel is not drawn');
    if ((await page.locator(TOGGLE).evaluate((button) => getComputedStyle(button).display)) !== 'none') {
      fail('collapsed-rule-stops-at-901', 'the fold control is drawn at 900, where the header owns the act');
    }
    await page.locator(NAV_TOGGLE).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    await page.waitForTimeout(350);
    const drawer = await geometry(page);
    if (drawer.rail.width < 200 || drawer.body.display === 'none') {
      fail('collapsed-rule-stops-at-901', `the opened drawer is ${drawer.rail.width}px wide and its panel display=${drawer.body.display}`);
    }
    await context.close();
  });

  await run('crossing-900-leaves-nothing-stale', async () => {
    const { page, context, checkProof } = await open('crossing-900-leaves-nothing-stale', PAGE, { width: 1281 });
    await checkProof();
    await key(page, '[');
    await settled(page);
    let g = await geometry(page);
    if (g.nav === 'open') fail('crossing-900-leaves-nothing-stale', `folding the column set nav=${g.nav}`);
    await page.setViewportSize({ width: 800, height: 800 });
    await page.waitForFunction(() => window.matchMedia('(max-width: 900px)').matches);
    // The drawer answers the change in a listener of its own, which may run a
    // frame after the query reads true; what is asked is where it settles.
    await page.waitForFunction(() => document.querySelector('#_y-nav-rail').inert === true, null, { timeout: 2000 }).catch(() => {});
    g = await geometry(page);
    if (g.state !== 'collapsed' || g.nav !== 'closed' || g.inert !== true || g.hidden !== 'true') {
      fail('crossing-900-leaves-nothing-stale', `below 900 rail=${g.state} nav=${g.nav} inert=${g.inert} aria-hidden=${g.hidden}, want collapsed, closed, and a closed drawer`);
    }
    await page.locator(NAV_TOGGLE).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    g = await geometry(page);
    if (g.state !== 'collapsed') fail('crossing-900-leaves-nothing-stale', `opening the drawer set rail=${g.state}`);
    await page.setViewportSize({ width: 1281, height: 800 });
    await page.waitForFunction(() => !window.matchMedia('(max-width: 900px)').matches);
    await page.waitForFunction(() => document.querySelector('#_y-nav-rail').inert === false, null, { timeout: 2000 }).catch(() => {});
    await settled(page);
    g = await geometry(page);
    if (g.inert !== false || g.hidden !== null || g.nav !== 'closed' || g.state !== 'collapsed') {
      fail('crossing-900-leaves-nothing-stale', `back above 900 inert=${g.inert} aria-hidden=${g.hidden} nav=${g.nav} rail=${g.state}, want none of the drawer's marks and the choice kept`);
    }
    await key(page, '[');
    await settled(page);
    g = await geometry(page);
    if (g.state !== 'open' || g.nav !== 'closed' || g.inert !== false) {
      fail('crossing-900-leaves-nothing-stale', `after crossing back, [ left rail=${g.state} nav=${g.nav} inert=${g.inert}`);
    }
    await context.close();
  });

  await run('wide-fold-sets-no-inertness', async () => {
    const { page, context, checkProof } = await open('wide-fold-sets-no-inertness', PAGE, { width: 1281 });
    await checkProof();
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (g.inert !== false || g.hidden !== null) {
      fail('wide-fold-sets-no-inertness', `a folded column above 900 has inert=${g.inert} aria-hidden=${g.hidden}; display:none is what removes it from the tree`);
    }
    await context.close();
  });

  await run('state-is-told-by-the-control', async () => {
    const { page, context, checkProof } = await open('state-is-told-by-the-control', PAGE, { width: 1281 });
    await checkProof();
    let g = await geometry(page);
    const name = g.label;
    if (g.expanded !== 'true' || g.controls !== '_y-nav-rail-body' || !g.title.includes('[')) {
      fail('state-is-told-by-the-control', `shown: aria-expanded=${g.expanded} controls=${g.controls} title=${g.title}`);
    }
    const shownTitle = g.title;
    await page.locator(TOGGLE).click();
    await settled(page);
    g = await geometry(page);
    if (g.expanded !== 'false' || g.label !== name || g.title === shownTitle || !g.title.includes('[')) {
      fail('state-is-told-by-the-control', `folded: aria-expanded=${g.expanded} label=${g.label} (was ${name}) title=${g.title} (was ${shownTitle})`);
    }
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    g = await geometry(page);
    if (g.expanded !== 'false') fail('state-is-told-by-the-control', `after a reload of a folded page aria-expanded=${g.expanded}`);
    await context.close();
  });

  await run('collapsed-panel-leaves-the-tree', async () => {
    const { page, context, checkProof } = await open('collapsed-panel-leaves-the-tree', PAGE, { width: 1281, collapsed: true });
    await checkProof();
    const facts = await page.evaluate((body) => {
      const panel = document.querySelector(body);
      const rail = document.querySelector('#_y-nav-rail');
      return {
        display: getComputedStyle(panel).display,
        boxes: panel.getClientRects().length,
        landmark: rail.tagName === 'ASIDE' && rail.hasAttribute('aria-label'),
        reachable: [...rail.querySelectorAll('a[href], input, button, summary')].filter((el) => el.getClientRects().length > 0 && !el.matches('[data-rail-toggle]')).length,
      };
    }, BODY);
    if (facts.display !== 'none' || facts.boxes !== 0 || facts.reachable !== 0) {
      fail('collapsed-panel-leaves-the-tree', `folded panel display=${facts.display}, boxes=${facts.boxes}, reachable controls=${facts.reachable}`);
    }
    if (!facts.landmark) fail('collapsed-panel-leaves-the-tree', 'the folded column is no longer a labelled landmark');
    await context.close();
  });

  await run('fold-is-animated', async () => {
    const { page, context, checkProof } = await open('fold-is-animated', PAGE, { width: 1281 });
    await checkProof();
    const declared = await page.evaluate(() => {
      const shell = document.querySelector('.y-shell:has(> .y-rail-left), .y-shell2:has(> .y-rail-left)');
      const style = getComputedStyle(shell);
      return { property: style.transitionProperty, duration: style.transitionDuration };
    });
    if (!declared.property.includes('grid-template-columns') || Number.parseFloat(declared.duration) <= 0) {
      fail('fold-is-animated', `the shell declares transition ${declared.property} over ${declared.duration}`);
    }
    await key(page, '[');
    await settled(page);
    const ran = await page.evaluate(() => window.__railTransitions);
    if (!ran.includes('grid-template-columns')) fail('fold-is-animated', `folding ran transitions ${JSON.stringify(ran)}, want grid-template-columns among them`);
    const g = await geometry(page);
    if (Math.round(g.rail.width) !== STRIP) fail('fold-is-animated', `the fold settled at ${g.rail.width}px, want ${STRIP}`);
    await context.close();
  });

  await run('nothing-animates-on-load', async () => {
    for (const collapsed of [true, false]) {
      const { page, context, checkProof } = await open('nothing-animates-on-load', PAGE, { width: 1281, collapsed, mutate: collapsed });
      await checkProof();
      await page.waitForTimeout(500);
      const ran = await page.evaluate(() => window.__railTransitions.filter((name) => name === 'grid-template-columns' || name === 'opacity' || name === 'display'));
      if (ran.length > 0) fail('nothing-animates-on-load', `loading a ${collapsed ? 'folded' : 'shown'} column ran transitions ${JSON.stringify(ran)}, want none`);
      await context.close();
    }
  });

  await run('reduced-motion-stops-the-fold', async () => {
    const { page, context, checkProof } = await open('reduced-motion-stops-the-fold', PAGE, { width: 1281, reducedMotion: 'reduce' });
    await checkProof();
    const durations = await page.evaluate(() => {
      const shell = document.querySelector('.y-shell:has(> .y-rail-left), .y-shell2:has(> .y-rail-left)');
      return [getComputedStyle(shell).transitionDuration, getComputedStyle(document.querySelector('#_y-nav-rail-body')).transitionDuration];
    });
    for (const duration of durations.flatMap((value) => value.split(','))) {
      if (Number.parseFloat(duration) > 0.01) fail('reduced-motion-stops-the-fold', `with reduced motion a fold transition lasts ${duration}`);
    }
    await context.close();
  });

  // The reader's place in the text survives the fold. Between the two widths
  // where the text is narrower than its measure, folding widens it and every
  // paragraph above the reader reflows; the browser's own anchoring has to keep
  // the paragraph they are reading where it was. The page is lengthened by
  // repeating its own prose, since no fixture note is long enough to scroll to
  // the middle of.
  //
  // The paragraph read is one from the middle of the page, brought to the top
  // edge of the window. Anchoring holds still the first thing the browser can
  // see, so a block left straddling that edge would be held instead, and its
  // own reflow would move the paragraph under it by a line whichever way the
  // fold went: an outcome of where the window happened to stop, which a change
  // to any block's height anywhere in the fixture could turn either way.
  await run('reading-position-survives', async () => {
    for (const width of [901, 1000, 1024]) {
      const { page, context, checkProof } = await open('reading-position-survives', '/notes/Notes/reading-fidelity.md', { width });
      await checkProof();
      await page.evaluate(() => {
        const prose = document.querySelector('.y-prose');
        const kids = [...prose.children];
        for (let i = 0; i < 30; i += 1) for (const kid of kids) prose.append(kid.cloneNode(true));
        const paragraphs = [...prose.querySelectorAll(':scope > p')];
        window.__reading = paragraphs[Math.floor(paragraphs.length / 2)];
        window.scrollTo(0, window.__reading.getBoundingClientRect().top + window.scrollY);
      });
      await page.waitForTimeout(150);
      const before = await page.evaluate(() => ({ top: window.__reading.getBoundingClientRect().top, width: window.__reading.getBoundingClientRect().width }));
      await key(page, '[');
      await settled(page);
      const after = await page.evaluate(() => ({ top: window.__reading.getBoundingClientRect().top, width: window.__reading.getBoundingClientRect().width }));
      // Where the text is narrower than its measure the fold widens it and the
      // paragraphs above the reader reflow; the narrowest width has to be such a
      // case, or the assertion below would pass for lack of anything to move. The
      // widths above it are asked whichever way they fall, because the measure's
      // edge sits within a few pixels of one of them and moves with the platform.
      if (width === 901 && !(before.width < MEASURE && after.width > before.width)) {
        broken(`at 901px the text was ${before.width}px and became ${after.width}px, so this case would not test a reflow`);
      }
      if (Math.abs(after.top - before.top) > 2) {
        fail('reading-position-survives', `at ${width}px the paragraph being read moved ${after.top - before.top}px when the column folded`);
      }
      await context.close();
    }
  });

  await run('every-rail-page-folds', async () => {
    for (const [shape, path] of SHAPES) {
      const { page, context, checkProof } = await open('every-rail-page-folds', path, { width: 1281 });
      // The injected rule is aimed at the study-path shell, so only that page
      // can show it matched anything.
      if (shape === 'study path') await checkProof();
      const before = await geometry(page);
      if (!before.toggle || before.toggle.display === 'none') broken(`${shape} (${path}) draws no control`);
      await page.locator(TOGGLE).click();
      await settled(page);
      const after = await geometry(page);
      // Measured from the column's own edge, so the answer is the strip's
      // width wherever the column stands.
      const mainFromRail = after.main.left - after.rail.left;
      if (Math.round(after.rail.width) !== STRIP || Math.round(mainFromRail) !== STRIP || after.body.display !== 'none') {
        fail('every-rail-page-folds', `${shape} (${path}) folded to rail ${after.rail.width}px, main ${mainFromRail}px from the column's edge, panel ${after.body.display}; want the strip, main ${STRIP}px in, no panel`);
      }
      await context.close();
    }
  });

  await run('control-revealed-before-runtime', async () => {
    // The runtime module is held back, so whatever is drawn was drawn by the
    // document alone.
    const context = await browser.newContext({ viewport: { width: 1281, height: 800 } });
    const page = await context.newPage();
    // Whether the runtime ran is told by its request having been refused. The
    // root's script mark cannot tell it any more: the document sets that mark
    // itself, before the first style, so it is there either way.
    let held = false;
    await page.route('**/yomihon.js{,?*}', (route) => {
      held = true;
      return route.abort();
    });
    const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
    const proof = mutation && mutation.target === 'control-revealed-before-runtime' ? await mutation.apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    if (!held) broken('the runtime was never requested, so nothing shows it was held back');
    const shown = await page.locator(ROW).evaluate((row) => getComputedStyle(row).display !== 'none' && row.getClientRects().length > 0);
    if (!shown) fail('control-revealed-before-runtime', 'before the runtime has run the control is not drawn, so it would appear after the first paint');
    await context.close();
  });

  await run('no-script-draws-no-control', async () => {
    const { page, context, checkProof } = await open('no-script-draws-no-control', PAGE, { width: 1281, script: false });
    await checkProof();
    const drawn = await page.locator(ROW).evaluate((row) => getComputedStyle(row).display !== 'none' && row.getClientRects().length > 0);
    if (drawn) fail('no-script-draws-no-control', 'with no script the fold control is drawn, and pressing it would do nothing');
    await context.close();
  });

  await run('strip-is-one-button', async () => {
    const { page, context, checkProof } = await open('strip-is-one-button', PAGE, { width: 1281, collapsed: true });
    await checkProof();
    await settled(page);
    // Below the button, inside the strip, where an edge-of-window click lands.
    await page.mouse.click(STRIP / 2, 400);
    await page.waitForTimeout(200);
    let g = await geometry(page);
    if (g.state !== 'collapsed') fail('strip-is-one-button', `a click on the empty strip left rail=${g.state}, want the strip to answer only through its button`);
    await page.locator(TOGGLE).click();
    await settled(page);
    g = await geometry(page);
    if (g.state !== 'open') fail('strip-is-one-button', `the button itself left rail=${g.state}, want open`);
    await context.close();
  });

  // The open width of every shell is what it was before the fold existed: the
  // note shell narrows below the width where all three columns stand, and the
  // study-path and shared-sidebar shells keep the wide column at every width.
  await run('every-shell-keeps-its-open-width', async () => {
    for (const width of [901, 1024, THREE_COLUMNS - 1, THREE_COLUMNS, 1281]) {
      for (const [shape, path] of SHAPES) {
        const { page, context, checkProof } = await open('every-shell-keeps-its-open-width', path, { width });
        if (shape === 'study path' && width === 901) await checkProof();
        const facts = await page.evaluate(() => {
          const shell = document.querySelector('.y-shell, .y-shell2');
          return { note: shell.classList.contains('y-shell'), rail: document.querySelector('#_y-nav-rail').getBoundingClientRect().width };
        });
        const want = facts.note && width < THREE_COLUMNS ? NOTE_OPEN_NARROW : OPEN;
        if (Math.round(facts.rail) !== want) {
          fail('every-shell-keeps-its-open-width', `${shape} (${path}) at ${width}px has an open rail ${facts.rail}px wide, want ${want}`);
        }
        await context.close();
      }
    }
  });

  // With a scrollbar that takes room, an open rail that scrolls still has its
  // padding on the right: the panel is as wide as the room it has, not as wide
  // as the column.
  await run('open-panel-keeps-its-padding-beside-a-scrollbar', async () => {
    const { page, context, checkProof } = await open('open-panel-keeps-its-padding-beside-a-scrollbar', PAGE, { width: 1024, height: 500, scrollbars: true });
    await checkProof();
    await makeRailScroll(page);
    const facts = await page.evaluate(() => {
      const rail = document.querySelector('#_y-nav-rail');
      return {
        gutter: rail.offsetWidth - rail.clientWidth - 1,
        room: rail.clientWidth,
        body: document.querySelector('#_y-nav-rail-body').getBoundingClientRect().right,
        filter: document.querySelector('[data-nav-filter]').getBoundingClientRect().right,
      };
    });
    if (facts.gutter < 5) broken(`the rail has a ${facts.gutter}px scrollbar, so this case would not test the room it takes`);
    for (const [name, right] of [['panel', facts.body], ['filter', facts.filter]]) {
      if (right > facts.room - 9) {
        fail('open-panel-keeps-its-padding-beside-a-scrollbar', `the ${name} ends at ${right}px in a rail ${facts.room}px wide beside a ${facts.gutter}px scrollbar, want the rail's 10px padding kept`);
      }
    }
    await context.close();
  });

  // Keyboard focus never lands on a row the sticky head covers.
  await run('focus-is-never-hidden-under-the-head', async () => {
    const { page, context, checkProof } = await open('focus-is-never-hidden-under-the-head', PAGE, { width: 1024, height: 420 });
    await checkProof();
    await makeRailScroll(page);
    await page.evaluate((body) => {
      for (const details of document.querySelectorAll(`${body} details`)) details.open = true;
      const links = [...document.querySelectorAll(`${body} a[href]`)].filter((a) => a.getClientRects().length > 0);
      links.at(-1).focus();
    }, BODY);
    let steps = 0;
    for (; steps < 30; steps += 1) {
      await page.keyboard.press('Shift+Tab');
      const at = await page.evaluate(() => {
        const active = document.activeElement;
        const head = document.querySelector('.y-railhead').getBoundingClientRect();
        if (!active || !active.closest('#_y-nav-rail-body')) return { done: true };
        return { done: false, top: active.getBoundingClientRect().top, head: head.bottom, text: active.textContent.trim().slice(0, 30) };
      });
      if (at.done) break;
      if (at.top < at.head - 1) {
        fail('focus-is-never-hidden-under-the-head', `Shift+Tab step ${steps + 1} focused "${at.text}" at ${at.top}px with the sticky head reaching ${at.head}px`);
      }
    }
    if (steps < 10) broken(`only ${steps} Shift+Tab steps stayed in the panel, too few to have scrolled anything`);
    await context.close();
  });

  // A page restored from the back/forward cache is put in the state the cookies
  // name, and the fold is not animated to get there. The pageshow event is
  // dispatched by hand: no headless run keeps a page in the cache reliably.
  const restore = async (site, focusIn) => {
    const { page, context, checkProof } = await open(site, PAGE, { width: 1281 });
    await checkProof();
    await context.addCookies([{ name: 'yomihon_rail', value: 'collapsed', url: BASE }]);
    if (focusIn) await page.locator(`${BODY} a[href]`).first().focus();
    await page.evaluate(() => {
      window.__railTransitions.length = 0;
      window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
    });
    const now = await geometry(page);
    await page.waitForTimeout(300);
    const ran = await page.evaluate(() => window.__railTransitions.slice());
    return { page, context, now, ran };
  };

  await run('restore-does-not-animate', async () => {
    const { context, now, ran } = await restore('restore-does-not-animate', false);
    if (now.state !== 'collapsed' || Math.round(now.rail.width) !== STRIP) {
      fail('restore-does-not-animate', `right after a restore the column is ${now.rail.width}px with rail=${now.state}, want the strip at once`);
    }
    if (ran.includes('grid-template-columns')) fail('restore-does-not-animate', `a restore ran transitions ${JSON.stringify(ran)}`);
    await context.close();
  });

  await run('restore-moves-focus-out', async () => {
    const { context, now } = await restore('restore-moves-focus-out', true);
    if (!now.focusInToggle) fail('restore-moves-focus-out', 'a restore that folds the column left focus in the panel it hid');
    await context.close();
  });

  // While single keys are off nothing on the button promises one, and turning
  // them on and off moves it.
  await run('off-shortcuts-do-not-advertise-the-key', async () => {
    const { page, context, checkProof } = await open('off-shortcuts-do-not-advertise-the-key', PAGE, { width: 1281 });
    await context.addCookies([{ name: 'yomihon_shortcuts', value: 'off', url: BASE }]);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    await checkProof();
    const read = () => page.evaluate(() => {
      const button = document.querySelector('[data-rail-toggle]');
      return { title: button.title, keys: button.getAttribute('aria-keyshortcuts') };
    });
    const flip = (on) => page.evaluate((value) => {
      const box = document.querySelector('[data-single-key-shortcuts-toggle]');
      box.checked = value;
      box.dispatchEvent(new Event('change', { bubbles: true }));
    }, on);
    let g = await read();
    if (g.title.includes('[') || g.keys !== null) fail('off-shortcuts-do-not-advertise-the-key', `with single keys off the button says title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await flip(true);
    g = await read();
    if (!g.title.includes('[') || g.keys !== '[') fail('off-shortcuts-do-not-advertise-the-key', `turning single keys on left title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await flip(false);
    g = await read();
    if (g.title.includes('[') || g.keys !== null) fail('off-shortcuts-do-not-advertise-the-key', `turning single keys off left title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await context.close();
  });

  // The focus ring on the strip's button is whole on both sides: the strip is
  // forty pixels with a one-pixel edge, and the button sits four in.
  await run('strip-ring-is-whole', async () => {
    const { page, context, checkProof } = await open('strip-ring-is-whole', PAGE, { width: 1281, collapsed: true });
    await checkProof();
    await page.keyboard.press('Tab');
    await page.locator(TOGGLE).focus();
    const ring = await page.evaluate(() => {
      const button = document.querySelector('[data-rail-toggle]');
      const style = getComputedStyle(button);
      const box = button.getBoundingClientRect();
      const reach = Number.parseFloat(style.outlineOffset) + Number.parseFloat(style.outlineWidth);
      return { focused: button.matches(':focus-visible'), left: box.left - reach, right: box.right + reach, room: document.querySelector('#_y-nav-rail').clientWidth };
    });
    if (!ring.focused) broken('the button is not showing a focus ring');
    if (ring.left < 0 || ring.right > ring.room) fail('strip-ring-is-whole', `the ring spans ${ring.left}-${ring.right}px in a strip ${ring.room}px wide`);
    await context.close();
  });

  console.log('PASS rail-collapse: the left column folds to a strip above 900px by button, key and filter key, keeps the reader\'s place and the choice, animates only on change, and leaves the drawer whole');
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
  await classicBrowser?.close();
  await browser.close();
}
