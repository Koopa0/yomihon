// Behavior lock for the left column standing beside the text, opened and
// folded. The choice belongs to the reader and is kept on the machine, so what
// is asked here is what a reader can see and do: open, the column is a panel
// 256px wide at the stage's left edge, from the header down; folded, it is
// gone, with nothing left on the edge; the text does not move either way; the
// choice is still made after a reload and before any script has run; the
// header's button, the key and the filter key all agree about it; and none of
// it leaks into the drawer, which the column is below 1321 beside a note and
// below 1481 beside a list, and whose open state is never stored.
//
// A reader who never chose meets the column folded. Every page here but the one
// that asks that of the server says which state it starts in by the cookie, so
// a site about folding the column starts from it open, as a reader who opened
// it would.
//
// Each site is independent: it opens its own page, sets its own state, and is
// the only place its mutation is applied. A mutation is therefore judged by the
// one assertion it aims at rather than by whichever fired first.
//
// Nothing here reads a frame in the middle of the fold. Motion is asked as what
// the stylesheet declares and what the page has settled into.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a long book rail), and MUTATE.
// MUTATE=list prints every watched regression.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/long-rail/Rail%20lesson%2002.md';
const MUTATE = process.env.MUTATE || '';

const RAIL = '#_y-nav-rail';
const BUTTON = '[data-nav-toggle]';
const HEAD = '.y-railhead';
const FILTER = '[data-nav-filter]';

// One page of each shape of rail: a book rail, a folder rail, a note at the
// vault root, the study-path page and the desk, which mount the shared
// sidebar. A choice honoured on some of them would come back on the others.
const SHAPES = [
  ['book rail', '/notes/Course/C02.md'],
  ['folder rail', '/notes/Notes/cutover.md'],
  ['root note', '/notes/README.md'],
  ['library', '/courses/Maps/study.md'],
  ['desk', '/'],
];

// The column's width where it stands beside the text, and the widths it stands
// from: beside a note's text from 1321, beside a list from 1481. Below them it
// is a drawer.
const PANEL = 256;
const WIDE = 1440;
const NOTE_FROM = 1321;
const LIST_FROM = 1481;
const STAGE = 1680;

const SITES = [
  'an-unchosen-column-is-folded',
  'folded-column-is-gone',
  'open-panel-stands-at-the-stage-edge',
  'text-does-not-move',
  'text-stands-clear-of-the-panel',
  'choice-survives-a-reload',
  'first-paint-follows-the-cookie-without-script',
  'wide-key-folds-only-the-column',
  'narrow-key-opens-only-the-drawer',
  'list-page-turns-at-1481',
  'drawer-state-is-never-stored',
  'slash-opens-the-column-first',
  'focus-leaves-the-hidden-panel',
  'state-is-told-by-the-button',
  'collapsed-panel-leaves-the-tree',
  'fold-is-animated',
  'nothing-animates-on-load',
  'reduced-motion-stops-the-fold',
  'every-rail-page-folds',
  'drawer-only-never-stands',
  'no-script-draws-no-control',
  'head-revealed-before-runtime',
  'filter-head-is-sticky',
  'rail-scroll-survives-a-fold',
  'focus-is-never-hidden-under-the-head',
  'crossing-the-width-leaves-nothing-stale',
  'wide-fold-sets-no-inertness',
  'restore-does-not-animate',
  'restore-moves-focus-out',
  'off-shortcuts-do-not-advertise-the-key',
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
  // The server answering a browser that never chose with the column open, as
  // it did before the fold became the default.
  'stamp-an-unchosen-column-open': {
    target: 'an-unchosen-column-is-folded',
    apply: rewriteDocument('data-rail="collapsed"', 'data-rail="open"'),
  },
  // The folded column left on the edge as a strip, as it was.
  'fold-to-a-strip': {
    target: 'folded-column-is-gone',
    apply: injectStyle("html[data-rail='collapsed'] .y-shell > .y-rail-left{visibility:visible!important;opacity:1!important;translate:none!important;width:40px!important}", '.y-shell > .y-rail-left'),
  },
  // The panel at the column's old width.
  'widen-the-panel': {
    target: 'open-panel-stands-at-the-stage-edge',
    apply: injectStyle('.y-shell > .y-rail-left{width:272px!important}', '.y-shell > .y-rail-left'),
  },
  // The panel at the window's edge on a window wider than the stage.
  'stand-the-panel-at-the-window-edge': {
    target: 'open-panel-stands-at-the-stage-edge',
    apply: injectStyle('.y-shell > .y-rail-left{left:0!important}', '.y-shell > .y-rail-left'),
  },
  // The text making way for the open column, so it moves at every fold.
  'make-room-for-the-open-column': {
    target: 'text-does-not-move',
    apply: injectStyle("html[data-rail='open'] .y-main{padding-left:256px!important}", '.y-main'),
  },
  // A panel wide enough to run under the centred text at the narrowest width
  // it stands at.
  'run-the-panel-under-the-text': {
    target: 'text-stands-clear-of-the-panel',
    apply: injectStyle('.y-shell > .y-rail-left{width:400px!important}', '.y-shell > .y-rail-left'),
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
    target: 'first-paint-follows-the-cookie-without-script',
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
    target: 'wide-key-folds-only-the-column',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '      if (drawer.isNarrow()) {', '      if (true) {'),
  },
  // The column key claiming the drawer's widths too.
  'arm-the-column-key-at-every-width': {
    target: 'narrow-key-opens-only-the-drawer',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '      if (drawer.isNarrow()) {', '      if (false) {'),
  },
  // The script reading a list page's column as a panel from the note's width,
  // where the stylesheet still draws a drawer.
  'turn-a-list-page-at-the-note-width': {
    target: 'list-page-turns-at-1481',
    apply: rewriteAsset('**/drawer.js{,?*}', "'(max-width: 1480px)'", "'(max-width: 1320px)'"),
  },
  // The drawer's state written to the cookie the wide window's choice is
  // kept in.
  'store-the-drawer-state': {
    target: 'drawer-state-is-never-stored',
    apply: rewriteAsset(
      '**/drawer.js{,?*}',
      "    root.dataset.nav = open ? 'open' : 'closed';\n",
      "    root.dataset.nav = open ? 'open' : 'closed';\n    document.cookie = `yomihon_rail=$" + "{open ? 'open' : 'collapsed'};path=/`;\n",
    ),
  },
  // Focus asked of a filter inside a column nobody can see.
  'focus-the-filter-without-opening-the-column': {
    target: 'slash-opens-the-column-first',
    apply: rewriteAsset('**/shortcuts.js{,?*}', '        rail.expand();\n', ''),
  },
  // Focus left in a column that has just left the tree.
  'leave-focus-in-the-panel': {
    target: 'focus-leaves-the-hidden-panel',
    apply: rewriteAsset('**/preferences.js{,?*}', "    document.querySelector('[data-nav-toggle]')?.focus();\n", ''),
  },
  // The button's state left where it was.
  'leave-aria-expanded-stale': {
    target: 'state-is-told-by-the-button',
    apply: rewriteAsset('**/drawer.js{,?*}', "  button.setAttribute('aria-expanded', String(open));\n", ''),
  },
  // The folded column faded out of sight and left in the tree.
  'keep-the-folded-column-in-the-tree': {
    target: 'collapsed-panel-leaves-the-tree',
    apply: injectStyle("html[data-rail='collapsed'] .y-shell > .y-rail-left{visibility:visible!important}", '.y-shell > .y-rail-left'),
  },
  // The fold with no motion declared.
  'declare-no-transition': {
    target: 'fold-is-animated',
    apply: injectStyle('.y-shell > .y-rail-left{transition:none!important}', '.y-shell > .y-rail-left'),
  },
  // The state arriving by script after the first paint, so the page paints
  // open and then folds — which is a transition on every load. It is written
  // a few frames after the load: motion is held still until the page is
  // marked ready, two frames after its first, so a write made before that
  // would be one the page never paints.
  'fold-after-the-first-paint': {
    target: 'nothing-animates-on-load',
    apply: async (page) => {
      await page.addInitScript(() => {
        const late = (frames, then) => (frames ? requestAnimationFrame(() => late(frames - 1, then)) : then());
        window.addEventListener('load', () => late(4, () => {
          document.documentElement.dataset.rail = 'collapsed';
        }), { once: true });
      });
      const rewrite = await rewriteDocument('data-rail="collapsed"', 'data-rail="open"')(page);
      return rewrite;
    },
  },
  // A duration that wins over the reduced-motion blanket.
  'animate-through-reduced-motion': {
    target: 'reduced-motion-stops-the-fold',
    apply: injectStyle('html .y-shell > .y-rail-left{transition-duration:120ms!important}', '.y-shell > .y-rail-left'),
  },
  // Only the note's column folding.
  'fold-only-the-note-shell': {
    target: 'every-rail-page-folds',
    apply: injectStyle("html[data-rail='collapsed'] .y-shell2 > .y-rail-left{visibility:visible!important;opacity:1!important;translate:none!important}", '.y-shell2 > .y-rail-left'),
  },
  // A page whose column is only a drawer standing it beside the text anyway.
  'stand-the-drawer-only-column': {
    target: 'drawer-only-never-stands',
    apply: injectStyle('@media (min-width:1321px){html[data-js] .y-rail-left[data-drawer-only]{transform:none!important}}', '.y-rail-left[data-drawer-only]'),
  },
  // The button drawn where no script can answer it.
  'draw-the-button-without-script': {
    target: 'no-script-draws-no-control',
    apply: async (page) => {
      let requests = 0;
      await page.route('**/app.css{,?*}', async (route) => {
        requests += 1;
        const response = await route.fetch();
        await route.fulfill({ response, body: `${await response.text()}\n.y-sidebtn{display:inline-flex!important}\n` });
      });
      return () => (requests >= 1 ? '' : 'the stylesheet was never requested');
    },
  },
  // The filter's row revealed by the runtime and not before it.
  'reveal-only-with-the-runtime': {
    target: 'head-revealed-before-runtime',
    apply: rewriteDocument('head.hidden = false;', ''),
  },
  // The filter scrolling away with the tree.
  'unstick-the-head': {
    target: 'filter-head-is-sticky',
    apply: injectStyle('.y-railhead{position:static!important}', '.y-railhead'),
  },
  // The column brought back at its top, as a column rebuilt on every opening
  // would be.
  'reopen-the-column-at-its-top': {
    target: 'rail-scroll-survives-a-fold',
    apply: rewriteAsset('**/rail.js{,?*}', "    preferences.writeRail('open');\n", "    preferences.writeRail('open');\n    rail.scrollTop = 0;\n"),
  },
  // Scroll padding gone, so focus scrolls a row under the sticky head.
  'drop-the-scroll-padding': {
    target: 'focus-is-never-hidden-under-the-head',
    apply: injectStyle('.y-rail-left{scroll-padding-top:0!important}', '.y-rail-left'),
  },
  // The drawer's state written when the column is folded.
  'fold-the-column-by-the-drawer-state': {
    target: 'crossing-the-width-leaves-nothing-stale',
    apply: rewriteAsset('**/rail.js{,?*}', "    preferences.writeRail('collapsed');\n", "    preferences.writeRail('collapsed');\n    document.documentElement.dataset.nav = 'open';\n"),
  },
  // The wide fold leaving the column inert, which the drawer's own rule
  // would have to undo on every crossing.
  'inert-the-folded-rail': {
    target: 'wide-fold-sets-no-inertness',
    apply: rewriteAsset('**/rail.js{,?*}', "    preferences.writeRail('collapsed');\n", "    preferences.writeRail('collapsed');\n    rail.inert = true;\n"),
  },
  // The restore's write left to animate.
  'animate-the-restore': {
    target: 'restore-does-not-animate',
    apply: rewriteAsset('**/preferences.js{,?*}', "    root.dataset.railSettling = '';\n", ''),
  },
  // The restore forgetting to move focus out of a column it hides.
  'restore-strands-focus': {
    target: 'restore-moves-focus-out',
    apply: rewriteAsset('**/preferences.js{,?*}', '    keepFocusOutOfFoldedRail(rail);\n', ''),
  },
  // The key advertised whatever the setting.
  'leave-the-key-advertised': {
    target: 'off-shortcuts-do-not-advertise-the-key',
    apply: rewriteAsset(
      '**/drawer.js{,?*}',
      "  if (keys) button.setAttribute('aria-keyshortcuts', '[');\n  else button.removeAttribute('aria-keyshortcuts');\n",
      "  button.setAttribute('aria-keyshortcuts', '[');\n",
    ),
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

// The transitions a page starts, from before its first script runs.
const recordTransitions = () => {
  window.__railTransitions = [];
  document.addEventListener('transitionrun', (event) => {
    if (event.target.id === '_y-nav-rail') window.__railTransitions.push(event.propertyName);
  }, true);
};

// Opens a page in a context of its own. The mutation is applied only when the
// site being run is the one it aims at, so every other page is the real one.
// The column starts open or folded as the cookie says; unchosen sends no
// cookie at all, which is the browser of a reader who never chose.
const open = async (site, path, {
  width = WIDE, height = 800, collapsed = false, unchosen = false, script = true, reducedMotion = 'no-preference', wait = true, mutate = true,
} = {}) => {
  const context = await browser.newContext({ viewport: { width, height }, javaScriptEnabled: script, reducedMotion });
  if (!unchosen) await context.addCookies([{ name: 'yomihon_rail', value: collapsed ? 'collapsed' : 'open', url: BASE }]);
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

const geometry = (page) => page.evaluate(({ rail, button }) => {
  const column = document.querySelector(rail);
  const control = document.querySelector(button);
  const band = document.querySelector('.y-header').getBoundingClientRect();
  const box = column?.getBoundingClientRect();
  const style = column ? getComputedStyle(column) : null;
  const prose = document.querySelector('.y-prose, .y-home, .y-syl')?.getBoundingClientRect();
  return {
    room: band.width,
    header: band.bottom,
    height: window.innerHeight,
    rail: box ? { left: box.left, right: box.right, top: box.top, bottom: box.bottom, width: box.width } : null,
    visibility: style?.visibility,
    opacity: style ? Number(style.opacity) : null,
    prose: prose ? { left: prose.left, right: prose.right, top: prose.top } : null,
    state: document.documentElement.dataset.rail,
    nav: document.documentElement.dataset.nav,
    expanded: control?.getAttribute('aria-expanded'),
    name: control?.textContent.trim(),
    title: control?.title,
    keys: control?.getAttribute('aria-keyshortcuts'),
    controls: control?.getAttribute('aria-controls'),
    inert: column?.inert,
    hidden: column?.getAttribute('aria-hidden'),
    mainInert: document.querySelector('#_y-main')?.inert,
    focusInButton: document.activeElement === control,
    focusInFilter: document.activeElement === document.querySelector('[data-nav-filter]'),
    focusInRail: Boolean(column?.contains(document.activeElement)),
  };
}, { rail: RAIL, button: BUTTON });

// Shown is the panel at full opacity in the tree; gone is out of it.
const shown = (g) => g.visibility === 'visible' && g.opacity > 0.99 && g.rail.right > 0;
const gone = (g) => g.visibility === 'hidden';

const key = async (page, name) => {
  await page.keyboard.press(name);
};

const cookieValue = async (context) => (await context.cookies(BASE)).find((cookie) => cookie.name === 'yomihon_rail')?.value ?? null;

const run = async (site, work) => {
  if (MUTATE && MUTATIONS[MUTATE].target !== site) return;
  await work();
};

// A rail long enough to scroll, made by repeating the rows in it.
const makeRailScroll = (page) => page.evaluate(() => {
  const rail = document.querySelector('#_y-nav-rail');
  const rows = [...rail.children].filter((child) => !child.matches('.y-railhead, script, p[hidden]'));
  for (let i = 0; i < 12; i += 1) for (const row of rows) rail.append(row.cloneNode(true));
  return rail.scrollHeight - rail.clientHeight;
});

try {
  // A reader who never chose reads with the column folded, from the first
  // paint: with no script it is what the server stamped and the stylesheet
  // drew, and with the script running it stays folded.
  await run('an-unchosen-column-is-folded', async () => {
    for (const script of [false, true]) {
      const { page, context, checkProof } = await open('an-unchosen-column-is-folded', PAGE, { unchosen: true, script });
      await checkProof();
      if (await cookieValue(context) !== null) broken('the unchosen browser carries a rail cookie');
      const g = await geometry(page);
      if (g.state !== 'collapsed' || !gone(g)) {
        fail('an-unchosen-column-is-folded', `with no choice stored and script ${script ? 'on' : 'off'} rail=${g.state} and the column is ${g.visibility}, want it folded and hidden`);
      }
      await context.close();
    }
  });

  // Folded, nothing of the column is left on the edge: no strip, no name, no
  // ground of its own where the reader's eye goes for the edge of the window.
  await run('folded-column-is-gone', async () => {
    const { page, context, checkProof } = await open('folded-column-is-gone', PAGE, { collapsed: true });
    await checkProof();
    await settled(page);
    const g = await geometry(page);
    const atEdge = await page.evaluate(() => [20, 200, 600].map((y) => document.elementFromPoint(20, y)?.closest('#_y-nav-rail') !== null));
    if (!gone(g) || atEdge.some(Boolean)) {
      fail('folded-column-is-gone', `the folded column is ${g.visibility} at ${g.rail.left}-${g.rail.right}px and answers a point at the window's left edge ${JSON.stringify(atEdge)}, want nothing of it there`);
    }
    if (await page.locator('.y-railspine').count() !== 0) fail('folded-column-is-gone', 'the folded column still carries a name on its edge');
    await context.close();
  });

  // Open, the column is a panel of its own width at the stage's left edge: the
  // window's edge on a window narrower than the stage, the stage's on one wider,
  // from under the header to the bottom of the window.
  await run('open-panel-stands-at-the-stage-edge', async () => {
    for (const width of [WIDE, 2560]) {
      const { page, context, checkProof } = await open('open-panel-stands-at-the-stage-edge', PAGE, { width });
      await checkProof();
      const g = await geometry(page);
      const edge = Math.max(0, (g.room - STAGE) / 2);
      if (!shown(g) || Math.abs(g.rail.left - edge) > 0.5 || Math.abs(g.rail.width - PANEL) > 0.5
        || Math.abs(g.rail.top - g.header) > 1 || Math.abs(g.rail.bottom - g.height) > 1) {
        fail('open-panel-stands-at-the-stage-edge', `at ${width}px the open column is ${g.visibility} at ${g.rail.left}-${g.rail.right}px and ${g.rail.top}-${g.rail.bottom}px, want ${PANEL}px wide from ${edge}px and from ${g.header}px to ${g.height}px`);
      }
      await context.close();
    }
  });

  // Opening and folding the column moves nothing the reader is reading: the
  // paragraph in the middle of the window keeps its left edge, its width and
  // its place, at every width the column stands at. The page is lengthened by
  // repeating its own prose and read from the middle, so the anchor the
  // browser holds still is that paragraph and nothing straddling the window's
  // top.
  await run('text-does-not-move', async () => {
    for (const width of [NOTE_FROM, WIDE, 1512, 1720, 2560]) {
      const { page, context, checkProof } = await open('text-does-not-move', '/notes/Notes/reading-fidelity.md', { width, collapsed: true });
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
      const read = () => page.evaluate(() => {
        const box = window.__reading.getBoundingClientRect();
        return { top: box.top, left: box.left, width: box.width };
      });
      const folded = await read();
      for (const step of ['opened', 'folded again']) {
        await key(page, '[');
        await settled(page);
        const g = await geometry(page);
        if (g.state !== (step === 'opened' ? 'open' : 'collapsed')) broken(`at ${width}px the key left rail=${g.state} after it ${step} the column`);
        const now = await read();
        if (Math.abs(now.left - folded.left) > 0.5 || Math.abs(now.width - folded.width) > 0.5 || Math.abs(now.top - folded.top) > 1) {
          fail('text-does-not-move', `at ${width}px the paragraph being read was at ${JSON.stringify(folded)} with the column folded and at ${JSON.stringify(now)} once the column was ${step}`);
        }
      }
      await context.close();
    }
  });

  // At the narrowest width it stands at, the open panel and the text do not
  // meet, beside a note from 1321 and beside a list from 1481.
  await run('text-stands-clear-of-the-panel', async () => {
    for (const [path, width] of [[PAGE, NOTE_FROM], ['/', LIST_FROM]]) {
      const { page, context, checkProof } = await open('text-stands-clear-of-the-panel', path, { width });
      await checkProof();
      const words = await page.evaluate(() => document.querySelector('h1.y-title').getBoundingClientRect().left);
      const g = await geometry(page);
      if (!shown(g)) broken(`${path} at ${width}px draws no open panel`);
      if (g.rail.right > words) {
        fail('text-stands-clear-of-the-panel', `${path} at ${width}px: the open panel reaches ${g.rail.right}px and the text starts at ${words}px`);
      }
      await context.close();
    }
  });

  await run('choice-survives-a-reload', async () => {
    const { page, context, checkProof } = await open('choice-survives-a-reload', PAGE);
    await checkProof();
    await key(page, '[');
    await settled(page);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    const after = await geometry(page);
    if (after.state !== 'collapsed' || !gone(after)) {
      fail('choice-survives-a-reload', `after a reload the column is ${after.visibility} with rail=${after.state}, want it folded (cookie=${await cookieValue(context)})`);
    }
    await key(page, '[');
    await settled(page);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    const reopened = await geometry(page);
    if (reopened.state !== 'open' || !shown(reopened)) {
      fail('choice-survives-a-reload', `after opening and reloading the column is ${reopened.visibility} with rail=${reopened.state}, want it open`);
    }
    await context.close();
  });

  // No script at all: the only thing that can have drawn or folded the column
  // is what the server wrote and the stylesheet read.
  await run('first-paint-follows-the-cookie-without-script', async () => {
    for (const collapsed of [true, false]) {
      const { page, context, checkProof } = await open('first-paint-follows-the-cookie-without-script', PAGE, { collapsed, script: false });
      await checkProof();
      const g = await geometry(page);
      if (g.state !== (collapsed ? 'collapsed' : 'open')) broken(`the server rendered rail=${g.state} for a ${collapsed ? 'collapsed' : 'open'} cookie`);
      if (collapsed ? !gone(g) : !shown(g) || Math.abs(g.rail.width - PANEL) > 0.5) {
        fail('first-paint-follows-the-cookie-without-script', `with no script and the cookie ${collapsed ? 'collapsed' : 'open'} the column is ${g.visibility}, ${g.rail.width}px wide`);
      }
      await context.close();
    }
  });

  await run('wide-key-folds-only-the-column', async () => {
    const { page, context, checkProof } = await open('wide-key-folds-only-the-column', PAGE, { width: NOTE_FROM });
    await checkProof();
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (g.state !== 'collapsed' || g.nav === 'open') {
      fail('wide-key-folds-only-the-column', `[ at ${NOTE_FROM} left rail=${g.state} and nav=${g.nav}, want collapsed and the drawer closed`);
    }
    await context.close();
  });

  await run('narrow-key-opens-only-the-drawer', async () => {
    const { page, context, checkProof } = await open('narrow-key-opens-only-the-drawer', PAGE, { width: NOTE_FROM - 1, collapsed: true });
    await checkProof();
    await key(page, '[');
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open', null, { timeout: 2000 }).catch(() => {});
    await settled(page);
    const g = await geometry(page);
    if (g.nav !== 'open' || g.state !== 'collapsed' || g.rail.left !== 0) {
      fail('narrow-key-opens-only-the-drawer', `[ at ${NOTE_FROM - 1} left nav=${g.nav}, rail=${g.state} and the column at ${g.rail.left}px, want the drawer open over the page and the choice untouched`);
    }
    await context.close();
  });

  // A list is wider than a note's text, so its column is a drawer up to 1480
  // and stands beside it from 1481; the button does what the stylesheet draws.
  await run('list-page-turns-at-1481', async () => {
    for (const width of [LIST_FROM - 1, LIST_FROM]) {
      const { page, context, checkProof } = await open('list-page-turns-at-1481', '/', { width });
      await checkProof();
      await page.locator(BUTTON).click();
      await page.waitForTimeout(50);
      await settled(page);
      const g = await geometry(page);
      const drawer = width < LIST_FROM;
      const want = drawer ? g.nav === 'open' && g.state === 'open' && g.rail.left === 0 && g.expanded === 'true'
        : g.nav !== 'open' && g.state === 'collapsed' && gone(g) && g.expanded === 'false';
      if (!want) {
        fail('list-page-turns-at-1481', `the desk at ${width}px: a press left nav=${g.nav}, rail=${g.state}, the column ${g.visibility} at ${g.rail.left}px and aria-expanded=${g.expanded}; want ${drawer ? 'the drawer open and the choice kept' : 'the column folded'}`);
      }
      await context.close();
    }
  });

  // Opening and closing the drawer is never a choice the reader keeps: the
  // cookie says what it said, and so does the root.
  await run('drawer-state-is-never-stored', async () => {
    const { page, context, checkProof } = await open('drawer-state-is-never-stored', PAGE, { width: 1100, collapsed: true });
    await checkProof();
    await page.locator(BUTTON).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    let g = await geometry(page);
    if (await cookieValue(context) !== 'collapsed' || g.state !== 'collapsed') {
      fail('drawer-state-is-never-stored', `opening the drawer left the cookie ${await cookieValue(context)} and rail=${g.state}, want both collapsed as they were`);
    }
    await key(page, 'Escape');
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'closed');
    g = await geometry(page);
    if (await cookieValue(context) !== 'collapsed' || g.state !== 'collapsed') {
      fail('drawer-state-is-never-stored', `closing the drawer left the cookie ${await cookieValue(context)} and rail=${g.state}`);
    }
    await context.close();
  });

  await run('slash-opens-the-column-first', async () => {
    const { page, context, checkProof } = await open('slash-opens-the-column-first', PAGE, { collapsed: true });
    await checkProof();
    if ((await geometry(page)).state !== 'collapsed') broken('the page did not start folded');
    if (await page.locator(FILTER).count() !== 1) broken(`${PAGE} has no filter, so / has nothing to reach`);
    await key(page, '/');
    await settled(page);
    const g = await geometry(page);
    if (g.state !== 'open' || !g.focusInFilter) {
      fail('slash-opens-the-column-first', `/ while folded left rail=${g.state} and filterFocused=${g.focusInFilter}, want the column open with the filter focused`);
    }
    await context.close();
  });

  await run('focus-leaves-the-hidden-panel', async () => {
    const { page, context, checkProof } = await open('focus-leaves-the-hidden-panel', PAGE);
    await checkProof();
    await page.locator(`${RAIL} a[href]`).first().focus();
    if (!(await geometry(page)).focusInRail) broken('focus did not enter the column');
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (!g.focusInButton) {
      const held = await page.evaluate(() => document.activeElement?.tagName);
      fail('focus-leaves-the-hidden-panel', `folding with focus in the column left focus on ${held}, want the header's button`);
    }
    await context.close();
  });

  await run('state-is-told-by-the-button', async () => {
    const { page, context, checkProof } = await open('state-is-told-by-the-button', PAGE);
    await checkProof();
    let g = await geometry(page);
    const name = g.name;
    if (g.expanded !== 'true' || g.controls !== '_y-nav-rail' || !g.title.includes('[')) {
      fail('state-is-told-by-the-button', `open: aria-expanded=${g.expanded} controls=${g.controls} title=${g.title}`);
    }
    const openTitle = g.title;
    await page.locator(BUTTON).click();
    await settled(page);
    g = await geometry(page);
    if (g.expanded !== 'false' || g.name !== name || g.title === openTitle || !g.title.includes('[')) {
      fail('state-is-told-by-the-button', `folded: aria-expanded=${g.expanded} name=${g.name} (was ${name}) title=${g.title} (was ${openTitle})`);
    }
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    g = await geometry(page);
    if (g.expanded !== 'false') fail('state-is-told-by-the-button', `after a reload of a folded page aria-expanded=${g.expanded}`);
    // Where the column is a drawer the same button speaks for the drawer.
    await page.setViewportSize({ width: 1100, height: 800 });
    await page.waitForFunction(() => window.matchMedia('(max-width: 1320px)').matches);
    await page.locator(BUTTON).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    g = await geometry(page);
    if (g.expanded !== 'true') fail('state-is-told-by-the-button', `an open drawer leaves aria-expanded=${g.expanded}`);
    await key(page, 'Escape');
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'closed');
    g = await geometry(page);
    if (g.expanded !== 'false') fail('state-is-told-by-the-button', `a closed drawer leaves aria-expanded=${g.expanded}`);
    await context.close();
  });

  await run('collapsed-panel-leaves-the-tree', async () => {
    const { page, context, checkProof } = await open('collapsed-panel-leaves-the-tree', PAGE, { collapsed: true });
    await checkProof();
    await settled(page);
    const facts = await page.evaluate(() => {
      const rail = document.querySelector('#_y-nav-rail');
      const rows = [...rail.querySelectorAll('a[href], input, button, summary')];
      const takes = rows.filter((row) => {
        row.focus();
        return document.activeElement === row;
      });
      document.activeElement?.blur();
      return {
        landmark: rail.tagName === 'ASIDE' && rail.hasAttribute('aria-label'),
        rows: rows.length,
        focusable: takes.length,
      };
    });
    if (facts.rows === 0) broken('the column has no rows to ask about');
    if (facts.focusable !== 0) fail('collapsed-panel-leaves-the-tree', `${facts.focusable} of the folded column's ${facts.rows} rows still take focus`);
    if (!facts.landmark) fail('collapsed-panel-leaves-the-tree', 'the column is no longer a labelled landmark');
    await context.close();
  });

  await run('fold-is-animated', async () => {
    const { page, context, checkProof } = await open('fold-is-animated', PAGE);
    await checkProof();
    // Every duration is zero until the root is marked is-ready, two frames
    // after the first paint, so the declared transition is read after that.
    await page.waitForSelector('html.is-ready', { state: 'attached' });
    const declared = await page.evaluate(() => {
      const style = getComputedStyle(document.querySelector('#_y-nav-rail'));
      return { property: style.transitionProperty, duration: style.transitionDuration };
    });
    if (!declared.property.includes('opacity') || !declared.duration.split(',').some((value) => Number.parseFloat(value) > 0)) {
      fail('fold-is-animated', `the column declares transition ${declared.property} over ${declared.duration}`);
    }
    await key(page, '[');
    await settled(page);
    const ran = await page.evaluate(() => window.__railTransitions);
    if (!ran.includes('opacity')) fail('fold-is-animated', `folding ran transitions ${JSON.stringify(ran)}, want opacity among them`);
    await context.close();
  });

  await run('nothing-animates-on-load', async () => {
    for (const collapsed of [true, false]) {
      const { page, context, checkProof } = await open('nothing-animates-on-load', PAGE, { collapsed, mutate: collapsed });
      await checkProof();
      await page.waitForTimeout(500);
      const ran = await page.evaluate(() => window.__railTransitions.slice());
      if (ran.length > 0) fail('nothing-animates-on-load', `loading a ${collapsed ? 'folded' : 'shown'} column ran transitions ${JSON.stringify(ran)}, want none`);
      await context.close();
    }
  });

  await run('reduced-motion-stops-the-fold', async () => {
    const { page, context, checkProof } = await open('reduced-motion-stops-the-fold', PAGE, { reducedMotion: 'reduce' });
    await checkProof();
    await page.waitForSelector('html.is-ready', { state: 'attached' });
    const duration = await page.evaluate(() => getComputedStyle(document.querySelector('#_y-nav-rail')).transitionDuration);
    for (const value of duration.split(',')) {
      if (Number.parseFloat(value) > 0.01) fail('reduced-motion-stops-the-fold', `with reduced motion a fold transition lasts ${value}`);
    }
    await context.close();
  });

  await run('every-rail-page-folds', async () => {
    for (const [shape, path] of SHAPES) {
      const { page, context, checkProof } = await open('every-rail-page-folds', path, { width: 1512 });
      // The injected rule is aimed at the shared sidebar's shell, so only a
      // page in it can show it matched anything.
      if (shape === 'library') await checkProof();
      const before = await geometry(page);
      if (!shown(before) || Math.abs(before.rail.width - PANEL) > 0.5) broken(`${shape} (${path}) draws no open panel at 1512 with the column open`);
      await page.locator(BUTTON).click();
      await settled(page);
      const after = await geometry(page);
      if (after.state !== 'collapsed' || !gone(after)) {
        fail('every-rail-page-folds', `${shape} (${path}) folded to rail=${after.state} with the column ${after.visibility}, want it gone`);
      }
      await context.close();
    }
  });

  // A page whose column is only a drawer stands none beside its text at any
  // width, whatever the cookie says, and the button opens the drawer there.
  await run('drawer-only-never-stands', async () => {
    const { page, context, checkProof } = await open('drawer-only-never-stands', '/health', { width: 2560 });
    await checkProof();
    let g = await geometry(page);
    if (g.rail.right > 0 || g.nav === 'open') {
      fail('drawer-only-never-stands', `the findings at 2560 with the cookie open draw the column at ${g.rail.left}-${g.rail.right}px with nav=${g.nav}, want it off the page`);
    }
    await page.locator(BUTTON).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    await settled(page);
    g = await geometry(page);
    if (g.rail.left !== 0 || g.state !== 'open') fail('drawer-only-never-stands', `the button opened the findings' drawer at ${g.rail.left}px with rail=${g.state}`);
    await context.close();
  });

  await run('no-script-draws-no-control', async () => {
    const { page, context, checkProof } = await open('no-script-draws-no-control', PAGE, { script: false });
    await checkProof();
    const drawn = await page.evaluate(() => [document.querySelector('[data-nav-toggle]'), document.querySelector('.y-railhead')]
      .map((element) => Boolean(element && getComputedStyle(element).display !== 'none' && element.getClientRects().length > 0)));
    if (drawn.some(Boolean)) fail('no-script-draws-no-control', `with no script the sidebar button and the filter row are drawn ${JSON.stringify(drawn)}, and pressing them would do nothing`);
    await context.close();
  });

  await run('head-revealed-before-runtime', async () => {
    // The runtime module is held back, so whatever is drawn was drawn by the
    // document alone.
    const context = await browser.newContext({ viewport: { width: WIDE, height: 800 } });
    await context.addCookies([{ name: 'yomihon_rail', value: 'open', url: BASE }]);
    const page = await context.newPage();
    let held = false;
    await page.route('**/yomihon.js{,?*}', (route) => {
      held = true;
      return route.abort();
    });
    const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
    const proof = mutation && mutation.target === 'head-revealed-before-runtime' ? await mutation.apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    if (!held) broken('the runtime was never requested, so nothing shows it was held back');
    const drawn = await page.locator(HEAD).evaluate((row) => getComputedStyle(row).display !== 'none' && row.getClientRects().length > 0);
    if (!drawn) fail('head-revealed-before-runtime', 'before the runtime has run the filter row is not drawn, so it would appear after the first paint');
    await context.close();
  });

  await run('filter-head-is-sticky', async () => {
    const { page, context, checkProof } = await open('filter-head-is-sticky', PAGE, { height: 600 });
    await checkProof();
    const room = await makeRailScroll(page);
    if (room < 200) broken(`the rail could scroll only ${room}px after being lengthened`);
    await page.evaluate(() => { document.querySelector('#_y-nav-rail').scrollTop = 400; });
    const at = await page.evaluate(() => {
      const rail = document.querySelector('#_y-nav-rail');
      const filter = document.querySelector('[data-nav-filter]').getBoundingClientRect();
      return { scrolled: rail.scrollTop, top: filter.top - rail.getBoundingClientRect().top, bottom: filter.bottom - rail.getBoundingClientRect().top };
    });
    if (at.scrolled < 300) broken(`the rail scrolled only ${at.scrolled}px`);
    if (at.top < 0 || at.bottom > 44) {
      fail('filter-head-is-sticky', `with the tree scrolled ${at.scrolled}px the filter is at ${at.top}-${at.bottom}px from the top of the rail, want it held in the sticky row`);
    }
    await context.close();
  });

  await run('rail-scroll-survives-a-fold', async () => {
    const { page, context, checkProof } = await open('rail-scroll-survives-a-fold', PAGE, { height: 600 });
    await checkProof();
    await makeRailScroll(page);
    await page.evaluate(() => { document.querySelector('#_y-nav-rail').scrollTop = 500; });
    const from = await page.evaluate(() => document.querySelector('#_y-nav-rail').scrollTop);
    if (from < 300) broken(`the rail scrolled only ${from}px`);
    for (let i = 0; i < 2; i += 1) {
      await page.locator(BUTTON).click();
      await settled(page);
    }
    const to = await page.evaluate(() => document.querySelector('#_y-nav-rail').scrollTop);
    if (Math.abs(to - from) > 2) fail('rail-scroll-survives-a-fold', `the rail was at ${from}px, is at ${to}px after folding and opening, want it where it was`);
    await context.close();
  });

  // Keyboard focus never lands on a row the sticky head covers.
  await run('focus-is-never-hidden-under-the-head', async () => {
    const { page, context, checkProof } = await open('focus-is-never-hidden-under-the-head', PAGE, { height: 420 });
    await checkProof();
    await makeRailScroll(page);
    await page.evaluate(() => {
      const rail = document.querySelector('#_y-nav-rail');
      for (const details of rail.querySelectorAll('details')) details.open = true;
      const links = [...rail.querySelectorAll('a[href]')].filter((a) => a.getClientRects().length > 0);
      links.at(-1).focus();
    });
    let steps = 0;
    for (; steps < 30; steps += 1) {
      await page.keyboard.press('Shift+Tab');
      const at = await page.evaluate(() => {
        const active = document.activeElement;
        const head = document.querySelector('.y-railhead').getBoundingClientRect();
        if (!active || !active.closest('#_y-nav-rail') || active.closest('.y-railhead')) return { done: true };
        return { done: false, top: active.getBoundingClientRect().top, head: head.bottom, text: active.textContent.trim().slice(0, 30) };
      });
      if (at.done) break;
      if (at.top < at.head - 1) {
        fail('focus-is-never-hidden-under-the-head', `Shift+Tab step ${steps + 1} focused "${at.text}" at ${at.top}px with the sticky head reaching ${at.head}px`);
      }
    }
    if (steps < 10) broken(`only ${steps} Shift+Tab steps stayed in the column, too few to have scrolled anything`);
    await context.close();
  });

  await run('crossing-the-width-leaves-nothing-stale', async () => {
    const { page, context, checkProof } = await open('crossing-the-width-leaves-nothing-stale', PAGE);
    await checkProof();
    await key(page, '[');
    await settled(page);
    let g = await geometry(page);
    if (g.nav === 'open') fail('crossing-the-width-leaves-nothing-stale', `folding the column set nav=${g.nav}`);
    await page.setViewportSize({ width: 1100, height: 800 });
    await page.waitForFunction(() => window.matchMedia('(max-width: 1320px)').matches);
    // The drawer answers the change in a listener of its own, which may run a
    // frame after the query reads true; what is asked is where it settles.
    await page.waitForFunction(() => document.querySelector('#_y-nav-rail').inert === true, null, { timeout: 2000 }).catch(() => {});
    g = await geometry(page);
    if (g.state !== 'collapsed' || g.nav !== 'closed' || g.inert !== true || g.hidden !== 'true') {
      fail('crossing-the-width-leaves-nothing-stale', `below 1321 rail=${g.state} nav=${g.nav} inert=${g.inert} aria-hidden=${g.hidden}, want collapsed, closed, and a closed drawer`);
    }
    await page.locator(BUTTON).click();
    await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
    g = await geometry(page);
    if (g.state !== 'collapsed' || g.expanded !== 'true') fail('crossing-the-width-leaves-nothing-stale', `opening the drawer set rail=${g.state} and aria-expanded=${g.expanded}`);
    await page.setViewportSize({ width: WIDE, height: 800 });
    await page.waitForFunction(() => !window.matchMedia('(max-width: 1320px)').matches);
    await page.waitForFunction(() => document.querySelector('#_y-nav-rail').inert === false, null, { timeout: 2000 }).catch(() => {});
    await settled(page);
    g = await geometry(page);
    if (g.inert !== false || g.hidden !== null || g.nav !== 'closed' || g.state !== 'collapsed' || g.mainInert || !gone(g) || g.expanded !== 'false') {
      fail('crossing-the-width-leaves-nothing-stale', `back above 1320 inert=${g.inert} aria-hidden=${g.hidden} nav=${g.nav} rail=${g.state} main inert=${g.mainInert} column ${g.visibility} aria-expanded=${g.expanded}, want none of the drawer's marks, the choice kept and the button saying so`);
    }
    await key(page, '[');
    await settled(page);
    g = await geometry(page);
    if (g.state !== 'open' || g.nav !== 'closed' || g.inert !== false || !shown(g)) {
      fail('crossing-the-width-leaves-nothing-stale', `after crossing back, [ left rail=${g.state} nav=${g.nav} inert=${g.inert} and the column ${g.visibility}`);
    }
    await context.close();
  });

  await run('wide-fold-sets-no-inertness', async () => {
    const { page, context, checkProof } = await open('wide-fold-sets-no-inertness', PAGE);
    await checkProof();
    await key(page, '[');
    await settled(page);
    const g = await geometry(page);
    if (g.inert !== false || g.hidden !== null || g.mainInert) {
      fail('wide-fold-sets-no-inertness', `a folded column above 1320 has inert=${g.inert} aria-hidden=${g.hidden} and the main inert=${g.mainInert}; being hidden is what takes it out of the tree`);
    }
    await context.close();
  });

  // A page restored from the back/forward cache is put in the state the cookies
  // name, and the fold is not animated to get there. The pageshow event is
  // dispatched by hand: no headless run keeps a page in the cache reliably.
  const restore = async (site, focusIn) => {
    const { page, context, checkProof } = await open(site, PAGE);
    await checkProof();
    await page.waitForSelector('html.is-ready', { state: 'attached' });
    await context.addCookies([{ name: 'yomihon_rail', value: 'collapsed', url: BASE }]);
    if (focusIn) await page.locator(`${RAIL} a[href]`).first().focus();
    await page.evaluate(() => {
      window.__railTransitions.length = 0;
      window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
    });
    const now = await geometry(page);
    await page.waitForTimeout(300);
    const ran = await page.evaluate(() => window.__railTransitions.slice());
    return { context, now, ran };
  };

  await run('restore-does-not-animate', async () => {
    const { context, now, ran } = await restore('restore-does-not-animate', false);
    if (now.state !== 'collapsed' || !gone(now) || now.opacity > 0.01) {
      fail('restore-does-not-animate', `right after a restore the column is ${now.visibility} at opacity ${now.opacity} with rail=${now.state}, want it gone at once`);
    }
    if (ran.length > 0) fail('restore-does-not-animate', `a restore ran transitions ${JSON.stringify(ran)}`);
    await context.close();
  });

  await run('restore-moves-focus-out', async () => {
    const { context, now } = await restore('restore-moves-focus-out', true);
    if (!now.focusInButton) fail('restore-moves-focus-out', 'a restore that folds the column left focus in the column it hid');
    await context.close();
  });

  // While single keys are off nothing on the button promises one, and turning
  // them on and off moves it. They are turned on and off on the settings page,
  // which has no column, so an open page meets the change when the reader
  // comes back to it from there: the cache restore reads the cookie the choice
  // left.
  await run('off-shortcuts-do-not-advertise-the-key', async () => {
    const { page, context, checkProof } = await open('off-shortcuts-do-not-advertise-the-key', PAGE);
    await context.addCookies([{ name: 'yomihon_shortcuts', value: 'off', url: BASE }]);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('html[data-js]');
    await checkProof();
    const flip = (on) => page.evaluate((value) => {
      document.cookie = `yomihon_shortcuts=${value ? 'on' : 'off'};path=/;max-age=31536000;samesite=lax`;
      window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
    }, on);
    let g = await geometry(page);
    if (g.title.includes('[') || g.keys !== null) fail('off-shortcuts-do-not-advertise-the-key', `with single keys off the button says title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await flip(true);
    g = await geometry(page);
    if (!g.title.includes('[') || g.keys !== '[') fail('off-shortcuts-do-not-advertise-the-key', `turning single keys on left title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await flip(false);
    g = await geometry(page);
    if (g.title.includes('[') || g.keys !== null) fail('off-shortcuts-do-not-advertise-the-key', `turning single keys off left title=${JSON.stringify(g.title)} keys=${g.keys}`);
    await context.close();
  });

  console.log('PASS rail-collapse: the column stands open as a 256px panel at the stage\'s edge from 1321 beside a note and 1481 beside a list, folds away to nothing by the header\'s button, the key and the filter key, moves no text either way, keeps the choice and its own scroll, animates only on change, and leaves the drawer and its unstored state whole');
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
