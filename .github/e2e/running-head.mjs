// Behavior lock for a note's running head: once the reader is in the note's
// first section, the header names the section being read, and a press on it
// opens the note's contents in place.
//
// What a reader sees and reaches is asked, not what the stylesheet says. With
// no script nothing of it is drawn or reached. With script it is out of sight
// while the note's head is on the screen, and over the words the note says
// before its first section, where there is no section to name; it comes once
// the first section's heading reaches the reading line, with the note's head
// gone by then, and goes again when the reader scrolls back up. It names the
// section the contents in the left column mark, by the heading's own words,
// with the number hung beside that heading where the note's sections are
// counted and with none where they are not, and it follows the reader down
// the note.
//
// Pressed, it opens the contents as a popover that leaves the page as it was:
// no dimming, nothing modal. The list names every section in order, with the
// numbers the margin shows, and marks the one being read; picking one goes
// there and closes the list. Escape closes it with focus back on the press. A
// press outside on something that takes no focus closes it with focus back on
// the press too, and a press on a control or on the text leaves focus where
// it landed.
//
// It fades as the reader moves, and is simply there on a page that arrives
// mid-note, by a reload or a kept place, and never moves for a reader who
// asked for less motion. It is drawn only where the room in the row holds the
// number and a dozen words, which is a question of the room rather than of the
// window, so the answer is asked in both languages at many widths. And only a
// note's own page has one.
//
// Env: YOMIHON_BASE, PAGE_PATH (a long note whose ## sections are counted), and
// MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
import { settled, waitForPlace } from './support/position.mjs';

const PAGE = process.env.PAGE_PATH || '/notes/Notes/Back%20restore.md';
const MUTATE = process.env.MUTATE || '';
const NOTE = decodeURIComponent(PAGE.replace(/^\/notes\//, ''));
// A note one of whose sections the author numbered, so the page counts none.
const UNNUMBERED = '/notes/Notes/Glass%20Tide.md';
// Pages that are not a note's own: two notes side by side, the desk, a list.
const ELSEWHERE = ['/compare/Notes/cutover.md?with=Notes%2Fcutover-zh-tw.md', '/', '/marks'];
const HEIGHT = 900;
// The least room the row has to leave for the running head to be drawn: the
// number and a dozen words beside it.
const ROOM = 240;
const WIDTHS = [1680, 1512, 1440, 1366, 1321, 1280, 1200, 1100, 1024, 1023, 960, 900, 721, 720, 600, 390];
const SECTION = '.y-prose > [data-level="2"], .y-prose > .y-sect > [data-level="2"]';
const HEAD = '[data-running-head]';
const POP = '#_y-contents';

const SITES = [
  'nothing-without-script',
  'hidden-while-the-head-shows',
  'quiet-over-the-introduction',
  'follows-the-head',
  'names-the-section',
  'contents-in-place',
  'a-pick-goes-there',
  'dismissal-returns-focus',
  'still-on-arrival',
  'fades-as-the-reader-moves',
  'still-for-less-motion',
  'drawn-only-with-room',
  'only-on-a-note',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN running-head: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL running-head: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN running-head: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED running-head: ${message}`); };

// Each regression is planted in what the browser is served, on every context
// the run opens, and counts what it touched, so a needle that matches nothing
// is reported rather than passed over.
const served = (pattern, edit, label) => {
  let loads = 0;
  let misses = 0;
  return {
    install: (context) => context.route(pattern, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      const changed = edit(original);
      loads += 1;
      if (changed === original) misses += 1;
      await route.fulfill({ response, body: changed });
    }),
    proof: () => {
      if (loads === 0) return `${label}: nothing it rewrites was served`;
      if (misses > 0) return `${label}: ${misses} of ${loads} served copies were left as they were`;
      return '';
    },
  };
};
const once = (needle, replacement) => (text) => (text.split(needle).length === 2 ? text.replace(needle, replacement) : text);
const appendStyle = (css) => served('**/static/app.css{,?*}', (text) => `${text}\n${css}\n`, 'the appended rule');
const rewriteStyle = (needle, replacement) => served('**/static/app.css{,?*}', once(needle, replacement), `the stylesheet needle ${JSON.stringify(needle)}`);
const rewriteModule = (needle, replacement) => served('**/runninghead.js{,?*}', once(needle, replacement), `the module needle ${JSON.stringify(needle)}`);
const intoSpacer = (path, html) => served(`${BASE}${path}`, once('<div class="y-header__spacer">', `<div class="y-header__spacer">${html}`), `the header of ${path}`);

const MUTATIONS = {
  // The stylesheet draws the press whether or not a script said it is ready.
  'drawn-without-script': { target: 'nothing-without-script', plant: appendStyle('.y-runhead.y-runhead{display:inline-flex!important;opacity:1!important;visibility:visible!important}') },
  // The head is shown from the top, over the note's own head.
  'shown-from-the-top': { target: 'hidden-while-the-head-shows', plant: rewriteModule('const shown = begun && marked;', 'const shown = marked;') },
  // Once shown, it stays when the reader scrolls back up to the note's head.
  // The first section's heading is watched in a window that ends at the
  // bottom of the screen, so a jump straight past it says nothing.
  'blind-to-a-jump': { target: 'follows-the-head', plant: rewriteModule("const PAST_ANY_NOTE = '100000%';", "const PAST_ANY_NOTE = '0px';") },
  'never-leaves': { target: 'follows-the-head', plant: rewriteModule("    button.removeAttribute('data-running-head-shown');\n", '') },
  // The running head comes once the note's own head has gone, as it first
  // did, and so names the introduction before the first section as the first
  // section, which is what the left column's contents mark there.
  'names-the-intro-as-01': {
    target: 'quiet-over-the-introduction',
    plant: served('**/runninghead.js{,?*}', (text) => once('observer.observe(first);', "observer.observe(document.querySelector('.y-frontis'));")(once('begun = entry.boundingClientRect.top <=', 'begun = !entry.isIntersecting && entry.boundingClientRect.top <=')(text)), 'the module needles for a head that comes with the note\'s head gone'),
  },
  // It keeps the first section's words however far the reader goes.
  'stuck-on-the-first': { target: 'names-the-section', plant: rewriteModule('if (title.textContent !== shownTitle) title.textContent = shownTitle;', 'if (!title.textContent) title.textContent = shownTitle;') },
  // A note the author numbered is numbered a second time.
  'numbers-on-an-unnumbered-note': { target: 'names-the-section', plant: rewriteModule('if (slot && numbered)', 'if (slot)') },
  // The count starts from nought, so no number is the margin's.
  'counted-from-nought': { target: 'names-the-section', plant: rewriteModule('String(ordinal).padStart(2, \'0\')', 'String(ordinal - 1).padStart(2, \'0\')') },
  // The page behind the list is dimmed, as a modal's would be.
  'dims-the-page': { target: 'contents-in-place', plant: appendStyle('.y-contentspop::backdrop{background:rgb(0 0 0 / 0.4)}') },
  // The list leaves out the note's last section.
  'drops-a-section': { target: 'contents-in-place', plant: appendStyle('.y-contentspop .y-toc__row:last-child{display:none}') },
  // The press keeps saying the list is closed while it is open.
  'says-closed-when-open': { target: 'contents-in-place', plant: rewriteModule("button.setAttribute('aria-expanded', String(open));", ';') },
  // Picking a section goes there and leaves the list standing over the text.
  'pick-leaves-it-open': { target: 'a-pick-goes-there', plant: rewriteModule(".closest('a[href^=\"#\"]')) popover.hidePopover();", ".closest('a[href^=\"#\"]')) {}") },
  // Escape closes the list and drops focus to the document.
  'escape-drops-focus': { target: 'dismissal-returns-focus', plant: rewriteModule('    if (open) return;\n', '    if (open) return;\n    document.activeElement?.blur();\n    return;\n') },
  // A press on nothing focusable leaves focus nowhere.
  'outside-press-drops-focus': { target: 'dismissal-returns-focus', plant: rewriteModule("if (button.hasAttribute('data-running-head-shown')) button.focus({ preventScroll: true });", ';') },
  // A press on a control has its focus taken back to the header.
  'takes-focus-from-a-control': { target: 'dismissal-returns-focus', plant: rewriteModule('if (focused && focused !== document.body && !popover.contains(focused)) return;', '') },
  // The head fades in on a page that arrives mid-note.
  'fades-from-the-start': { target: 'still-on-arrival', plant: rewriteModule("button.setAttribute('data-running-head-ready', '');", "button.setAttribute('data-running-head-ready', ''); button.setAttribute('data-running-head-fades', '');") },
  // The head snaps in and out as the reader moves.
  'snaps': { target: 'fades-as-the-reader-moves', plant: appendStyle('.y-runhead.y-runhead.y-runhead{transition:none}') },
  // The fade escapes the reader's request for less motion.
  'fades-under-reduce': { target: 'still-for-less-motion', plant: appendStyle('@media (prefers-reduced-motion: reduce){.y-runhead.y-runhead.y-runhead{transition-duration:200ms!important}}') },
  // It is drawn into a row with no room for it.
  'drawn-without-room': { target: 'drawn-only-with-room', plant: rewriteStyle('@container y-headroom (width >= 15rem)', '@container y-headroom (width >= 4rem)') },
  // It is left out of a row that has room for it.
  'left-out-with-room': { target: 'drawn-only-with-room', plant: rewriteStyle('@container y-headroom (width >= 15rem)', '@container y-headroom (width >= 20rem)') },
  // The page holding two notes draws a running head too.
  'on-the-compare-page': { target: 'only-on-a-note', plant: intoSpacer(ELSEWHERE[0], '<button class="y-runhead" type="button" data-running-head data-running-head-ready data-running-head-shown>x</button>') },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`running-head: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`running-head: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`running-head: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}
const planted = MUTATE ? MUTATIONS[MUTATE].plant : null;

// Every time the head is shown, how it looked at that very moment: a head that
// arrives at once is fully there; one that fades starts from nothing.
const SAMPLER = () => {
  window.__runningHead = [];
  new MutationObserver((records) => {
    for (const record of records) {
      const head = record.target;
      if (!head.hasAttribute('data-running-head-shown')) continue;
      const style = getComputedStyle(head);
      window.__runningHead.push({ opacity: Number(style.opacity), durations: style.transitionDuration });
    }
  }).observe(document, { subtree: true, attributes: true, attributeFilter: ['data-running-head-shown'] });
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });

const open = async ({ width = 1512, script = true, motion = 'no-preference', language = 'zh-Hant' } = {}) => {
  const context = await browser.newContext({ viewport: { width, height: HEIGHT }, javaScriptEnabled: script, reducedMotion: motion });
  await context.addCookies([
    { name: 'yomihon_rail', value: 'collapsed', url: BASE },
    ...(language === 'en' ? [{ name: 'yomihon_lang', value: 'en', url: BASE }] : []),
  ]);
  if (planted) await planted.install(context);
  return context;
};

const visit = async (context, path, { sample = false } = {}) => {
  const page = await context.newPage();
  if (sample) await page.addInitScript(SAMPLER);
  const response = await page.goto(`${BASE}${path}`, { waitUntil: 'load' });
  if (response?.status() !== 200) broken(`${path} answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
  return page;
};

// A page running no script runs no frame callbacks either, so it is given
// time instead.
const frames = async (page, count = 3) => {
  if (!(await page.evaluate(() => document.documentElement.hasAttribute('data-js')))) {
    await page.waitForTimeout(50 * count);
    return;
  }
  await page.evaluate((n) => new Promise((resolve) => {
    const step = (left) => (left === 0 ? resolve() : requestAnimationFrame(() => step(left - 1)));
    step(n);
  }), count);
};

// Scrolling by the script moves the page as a jump does: the reader has not
// moved, so nothing fades.
const scrollTo = async (page, y) => {
  await page.evaluate((top) => window.scrollTo(0, top), y);
  await frames(page);
};

// What the reader is shown: whether the press is drawn and can be seen, what
// it says, and where the note's head and the header's edge are.
const look = (page) => page.evaluate(({ head, pop }) => {
  const button = document.querySelector(head);
  const frontis = document.querySelector('.y-article .y-frontis').getBoundingClientRect();
  const header = document.querySelector('.y-header').getBoundingClientRect();
  const seen = button ? button.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : false;
  const words = button ? [...button.childNodes].filter((node) => node.nodeType === Node.ELEMENT_NODE && node.getAttribute('aria-hidden') !== 'true').map((node) => node.textContent).join('').trim() : '';
  return {
    present: Boolean(button),
    seen,
    number: button?.querySelector('[data-running-head-num]')?.textContent ?? '',
    numberHidden: button?.querySelector('[data-running-head-num]')?.getAttribute('aria-hidden') === 'true',
    words,
    headGone: frontis.bottom <= header.bottom,
    open: document.querySelector(pop)?.matches(':popover-open') ?? false,
  };
}, { head: HEAD, pop: POP });

const shownNow = (page) => page.evaluate((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown') ?? false, HEAD);

const waitSeen = async (page, want, site, what) => {
  await page.waitForFunction(({ head, seen }) => {
    const button = document.querySelector(head);
    const now = button ? button.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : false;
    return now === seen;
  }, { head: HEAD, seen: want }, { timeout: 2000 }).catch(async () => {
    fail(site, `${what}: the running head reads ${JSON.stringify(await look(page))}, want it ${want ? 'shown' : 'out of sight'}`);
  });
};

// The sections the margin counts, in order: each heading's id, its words and
// the number the margin hangs beside it.
const sectionsOf = (page) => page.evaluate((selector) => {
  const numbered = document.querySelector('.y-prose')?.hasAttribute('data-section-numbers');
  return [...document.querySelectorAll(selector)].map((heading, i) => ({
    id: heading.id,
    words: heading.textContent.replace(/\s+/g, ' ').trim(),
    number: numbered ? String(i + 1).padStart(2, '0') : '',
  }));
}, SECTION);

// headFor parks each section's heading where a jump parks it, and asks what the
// running head names there, beside what the left column's contents mark.
const walk = async (page, site, label) => {
  const sections = await sectionsOf(page);
  if (sections.length < 2) broken(`${label} counts ${sections.length} sections, too few to walk`);
  let walked = 0;
  for (const section of sections) {
    const parked = await page.evaluate((id) => {
      const heading = document.getElementById(id);
      heading.scrollIntoView({ block: 'start' });
      const line = (Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0) + 8;
      return heading.getBoundingClientRect().top <= line;
    }, section.id);
    // A short section at the end of the note never reaches the line, so it is
    // never the one being read; the contents mark the one before it there.
    if (!parked) continue;
    await page.waitForFunction(({ head, words }) => document.querySelector(head)?.querySelector('[data-running-head-title]')?.textContent.replace(/\s+/g, ' ').trim() === words, { head: HEAD, words: section.words }, { timeout: 1500 }).catch(() => {});
    await waitSeen(page, true, site, `${label}: parked at ${section.id}`);
    const seen = await look(page);
    const rail = await page.evaluate(() => document.querySelector('.y-railtoc a[aria-current="location"]')?.textContent.replace(/\s+/g, ' ').trim() ?? null);
    if (seen.words !== section.words || seen.number !== section.number) {
      fail(site, `${label}: parked at ${section.id}, the running head names ${JSON.stringify([seen.number, seen.words])}, want ${JSON.stringify([section.number, section.words])}`);
    }
    if (!seen.numberHidden) fail(site, `${label}: the running head's number is read out, where the margin's is not`);
    if (rail !== null && rail !== section.words) fail(site, `${label}: parked at ${section.id}, the left column marks ${JSON.stringify(rail)} and the running head ${JSON.stringify(section.words)}`);
    walked += 1;
  }
  if (walked < 2) broken(`${label}: only ${walked} sections could be parked under the header`);
  return sections;
};

const sectionTop = (page, id) => page.evaluate((target) => {
  const heading = document.getElementById(target);
  heading.scrollIntoView({ block: 'start' });
  return Math.round(window.scrollY);
}, id);

const rowsOf = (page) => page.evaluate((pop) => [...document.querySelectorAll(`${pop} .y-toc__row a`)]
  .filter((link) => link.checkVisibility())
  .map((link) => ({
    id: decodeURIComponent(link.getAttribute('href').slice(1)),
    number: link.querySelector('.y-contentspop__num')?.textContent ?? '',
    words: link.querySelector('.y-contentspop__title')?.textContent.replace(/\s+/g, ' ').trim() ?? '',
    current: link.getAttribute('aria-current') === 'location',
  })), POP);

const closedWith = async (page, site, how) => {
  await page.waitForFunction((pop) => !document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 }).catch(() => {
    fail(site, `${how} left the contents open`);
  });
  await frames(page, 2);
  return page.evaluate(() => {
    const focused = document.activeElement;
    if (!focused || focused === document.body) return 'nothing';
    if (focused.matches('[data-running-head]')) return 'the press';
    if (focused.matches('[data-live-search-input]')) return 'the search field';
    if (focused.id === '_y-main') return 'the text';
    return focused.outerHTML.slice(0, 80);
  });
};

try {
  // --- No script: nothing is drawn or reached --------------------------------
  {
    const context = await open({ script: false });
    const page = await visit(context, PAGE);
    if (await page.evaluate(() => document.documentElement.hasAttribute('data-js'))) broken('script ran in the scriptless context');
    for (const y of [0, 1600, 4000]) {
      await scrollTo(page, y);
      const drawn = await page.evaluate((head) => {
        const button = document.querySelector(head);
        return button ? button.checkVisibility({ visibilityProperty: true }) || button.getClientRects().length > 0 : false;
      }, HEAD);
      if (drawn) fail('nothing-without-script', `with no script, at ${y}px down, the running head is drawn`);
    }
    await page.locator('.y-brand__vault').focus();
    await page.keyboard.press('Tab');
    if (await page.evaluate((head) => document.activeElement?.matches(head), HEAD)) fail('nothing-without-script', 'with no script, Tab reaches the running head');
    await context.close();
  }

  // --- With script: hidden at the top and over the introduction ------------
  {
    const context = await open();
    const page = await visit(context, PAGE);
    const top = await look(page);
    if (!top.present) broken(`${PAGE} carries no running head`);
    if (top.headGone) broken(`${PAGE} opened with its head already gone`);
    if (top.seen) fail('hidden-while-the-head-shows', `at the top of ${PAGE} the running head is shown over the note's own head`);
    await page.locator('.y-brand__vault').focus();
    await page.keyboard.press('Tab');
    if (await page.evaluate((head) => document.activeElement?.matches(head), HEAD)) fail('hidden-while-the-head-shows', 'at the top of the note Tab reaches the running head');

    // Where the note's head goes under the header, and where the first
    // section's heading reaches the reading line: between the two is the
    // note's introduction, which names no section.
    const at = await page.evaluate((selector) => {
      const frontis = document.querySelector('.y-article .y-frontis').getBoundingClientRect();
      const header = document.querySelector('.y-header').getBoundingClientRect();
      const heading = document.querySelector(selector);
      const line = (Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0) + 8;
      return {
        headGone: Math.round(frontis.bottom + window.scrollY - header.bottom),
        sectionBegins: Math.round(heading.getBoundingClientRect().top + window.scrollY - line),
        words: heading.textContent.replace(/\s+/g, ' ').trim(),
      };
    }, SECTION);
    if (at.sectionBegins - at.headGone < 60) broken(`${PAGE} has ${at.sectionBegins - at.headGone}px of introduction between its head and its first section, too little to ask about`);
    await scrollTo(page, at.headGone - 40);
    await page.waitForTimeout(150);
    if ((await look(page)).seen) fail('follows-the-head', 'with the note\'s head 40px from the header\'s edge, the running head is already shown');
    for (const y of [at.headGone + 10, at.sectionBegins - 10]) {
      await scrollTo(page, y);
      await page.waitForTimeout(150);
      const intro = await look(page);
      if (!intro.headGone) broken(`at ${y}px down the note's head is still on the screen`);
      if (intro.seen) fail('quiet-over-the-introduction', `over the introduction, ${at.sectionBegins - y}px before the first section begins, the running head names ${JSON.stringify([intro.number, intro.words])}`);
    }
    await scrollTo(page, at.sectionBegins + 2);
    await waitSeen(page, true, 'follows-the-head', 'with the first section\'s heading at the reading line');
    const begun = await look(page);
    if (!begun.headGone) fail('follows-the-head', 'the running head is shown while the note\'s head is still on the screen');
    if (begun.words !== at.words) fail('names-the-section', `as the first section begins the running head names ${JSON.stringify(begun.words)}, want ${JSON.stringify(at.words)}`);
    await scrollTo(page, 0);
    await waitSeen(page, false, 'follows-the-head', 'scrolled back to the top');
    // A jump from the top straight to a later section, past the first one's
    // heading in one step, as a link or the left column's contents make, and
    // straight back, in a window short enough that the first section's
    // heading is below it at the top, as a long introduction leaves it.
    if (at.sectionBegins < 240) broken(`${PAGE}'s first section begins ${at.sectionBegins}px down, too near the top to leave its heading below a window`);
    await page.setViewportSize({ width: 1512, height: at.sectionBegins });
    await frames(page);
    if (await page.evaluate((selector) => document.querySelector(selector).getBoundingClientRect().top <= innerHeight, SECTION)) broken('the first section\'s heading is on the screen at the top, so a jump does not pass it unseen');
    await page.evaluate((selector) => document.querySelectorAll(selector)[2].scrollIntoView({ block: 'start' }), SECTION);
    await frames(page);
    await waitSeen(page, true, 'follows-the-head', 'after a jump from the top to the third section');
    await scrollTo(page, 0);
    await waitSeen(page, false, 'follows-the-head', 'after a jump back to the top');
    await page.setViewportSize({ width: 1512, height: HEIGHT });

    // Down the note, section by section.
    const sections = await walk(page, 'names-the-section', PAGE);
    if (sections.some((section) => section.number === '')) broken(`${PAGE} is not counted, so it cannot ask for numbers`);

    // --- The contents, opened in place ----------------------------------------
    const third = sections[2];
    await scrollTo(page, await sectionTop(page, third.id));
    await page.waitForFunction((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown'), HEAD, { timeout: 1500 }).catch(() => broken('the running head never came at the third section'));
    await page.locator(HEAD).click();
    await page.waitForFunction((pop) => document.querySelector(pop)?.matches(':popover-open'), POP, { timeout: 1500 }).catch(() => fail('contents-in-place', 'a press on the running head opened nothing'));
    await page.waitForFunction((head) => document.querySelector(head).getAttribute('aria-expanded') === 'true', HEAD, { timeout: 1000 }).catch(() => {});
    const opened = await page.evaluate(({ head, pop }) => {
      const list = document.querySelector(pop);
      const backdrop = getComputedStyle(list, '::backdrop');
      const backdropDrawn = backdrop.backgroundColor !== 'rgba(0, 0, 0, 0)' && backdrop.backgroundColor !== 'transparent';
      return {
        modal: list.matches(':modal'),
        backdropDrawn,
        expanded: document.querySelector(head).getAttribute('aria-expanded'),
        inert: document.querySelector('.y-prose').closest('[inert]') !== null,
      };
    }, { head: HEAD, pop: POP });
    if (opened.modal || opened.backdropDrawn || opened.inert) fail('contents-in-place', `the contents open as a modal would: ${JSON.stringify(opened)}`);
    if (opened.expanded !== 'true') fail('contents-in-place', `with the contents open the press says aria-expanded=${opened.expanded}`);
    const rows = await rowsOf(page);
    const want = sections.map((section) => ({ id: section.id, number: section.number, words: section.words, current: section.id === third.id }));
    if (JSON.stringify(rows) !== JSON.stringify(want)) fail('contents-in-place', `the contents list ${JSON.stringify(rows)}, want ${JSON.stringify(want)}`);

    // A pick goes to the section and closes the list.
    const fourth = sections[3];
    await page.locator(`${POP} a[href="#${fourth.id}"]`).click();
    await page.waitForFunction((pop) => !document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 }).catch(() => fail('a-pick-goes-there', 'picking a section left the contents open'));
    await page.waitForFunction(({ id }) => {
      const heading = document.getElementById(id);
      const line = (Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0) + 8;
      const top = heading.getBoundingClientRect().top;
      return top >= 0 && top <= line;
    }, { id: fourth.id }, { timeout: 2000 }).catch(() => fail('a-pick-goes-there', `picking ${fourth.id} did not bring its heading under the header`));
    await page.waitForFunction(({ head, words }) => document.querySelector(head)?.querySelector('[data-running-head-title]')?.textContent.trim() === words, { head: HEAD, words: fourth.words }, { timeout: 2000 }).catch(() => fail('a-pick-goes-there', `after picking ${fourth.id} the running head does not name it`));

    // Escape, from inside the list, hands focus back to the press.
    await page.locator(HEAD).focus();
    await page.keyboard.press('Enter');
    await page.waitForFunction((pop) => document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 }).catch(() => fail('contents-in-place', 'Enter on the running head opened nothing'));
    await page.keyboard.press('Tab');
    if (!(await page.evaluate((pop) => document.querySelector(pop).contains(document.activeElement), POP))) fail('contents-in-place', 'Tab from the open press does not go into the contents');
    await page.keyboard.press('Escape');
    const afterEscape = await closedWith(page, 'dismissal-returns-focus', 'Escape');
    if (afterEscape !== 'the press') fail('dismissal-returns-focus', `after Escape focus is on ${afterEscape}, want the press`);

    // A press on the row's empty room, which takes no focus.
    await page.keyboard.press('Enter');
    await page.waitForFunction((pop) => document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 });
    await page.keyboard.press('Tab');
    const room = await page.evaluate(() => {
      const search = document.querySelector('.y-searchfield').getBoundingClientRect();
      const header = document.querySelector('.y-header').getBoundingClientRect();
      return { x: search.left - 6, y: header.top + header.height / 2 };
    });
    await page.mouse.click(room.x, room.y);
    const afterRoom = await closedWith(page, 'dismissal-returns-focus', 'a press on the row\'s empty room');
    if (afterRoom !== 'the press') fail('dismissal-returns-focus', `after a press on nothing focusable focus is on ${afterRoom}, want the press`);

    // A press on a control leaves focus on that control.
    await page.keyboard.press('Enter');
    await page.waitForFunction((pop) => document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 });
    await page.locator('.y-searchfield [data-live-search-input]').click();
    const afterControl = await closedWith(page, 'dismissal-returns-focus', 'a press on the search field');
    if (afterControl !== 'the search field') fail('dismissal-returns-focus', `after a press on the search field focus is on ${afterControl}, want the field`);
    await page.keyboard.press('Escape');
    await page.keyboard.press('Escape');

    // A press on the text leaves focus in the text.
    await page.locator(HEAD).click();
    await page.waitForFunction((pop) => document.querySelector(pop).matches(':popover-open'), POP, { timeout: 1500 });
    const paragraph = await page.evaluate(() => {
      const prose = document.querySelector('.y-prose');
      const box = prose.getBoundingClientRect();
      return { x: box.left + box.width - 12, y: 600 };
    });
    await page.mouse.click(paragraph.x, paragraph.y);
    const afterText = await closedWith(page, 'dismissal-returns-focus', 'a press on the text');
    if (afterText === 'nothing') fail('dismissal-returns-focus', 'after a press on the text focus fell to the document');
    await context.close();
  }

  // --- A note the author numbered is named without numbers ------------------
  {
    const context = await open();
    const page = await visit(context, UNNUMBERED);
    const sections = await walk(page, 'names-the-section', UNNUMBERED);
    if (sections.some((section) => section.number !== '')) broken(`${UNNUMBERED} is counted, so it cannot ask for none`);
    await page.locator(HEAD).click();
    const rows = await rowsOf(page);
    if (rows.some((row) => row.number !== '')) fail('names-the-section', `the contents of ${UNNUMBERED}, which the page does not count, carry numbers: ${JSON.stringify(rows)}`);
    await context.close();
  }

  // --- Motion ----------------------------------------------------------------
  {
    // The reader moves down past the head: it fades in.
    const context = await open();
    const page = await visit(context, PAGE, { sample: true });
    await page.mouse.move(700, 500);
    for (let i = 0; i < 24 && !(await shownNow(page)); i += 1) {
      await page.mouse.wheel(0, 200);
      await frames(page, 2);
    }
    await page.waitForFunction((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown'), HEAD, { timeout: 2000 }).catch(() => broken('the running head never came as the reader scrolled'));
    const moving = await page.evaluate(() => window.__runningHead);
    const first = moving[0];
    const longest = Math.max(...first.durations.split(',').map((d) => Number.parseFloat(d)));
    if (!(first.opacity < 1 && longest >= 0.1)) fail('fades-as-the-reader-moves', `as the reader scrolled past the head, the running head arrived as ${JSON.stringify(first)}, want a fade from nothing`);

    // The page is reloaded mid-note, which puts the reader back there.
    await page.reload({ waitUntil: 'load' });
    await arrived(page);
    await page.waitForFunction((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown'), HEAD, { timeout: 2000 }).catch(() => broken('a reload mid-note did not put the reader past the head'));
    const reloaded = (await page.evaluate(() => window.__runningHead))[0];
    if (reloaded.opacity !== 1) fail('still-on-arrival', `on a reload mid-note the running head arrived as ${JSON.stringify(reloaded)}, want it simply there`);

    // The note is opened afresh, and opens where the reader stopped in it.
    if (!(await waitForPlace(page, NOTE))) broken('no reading place was kept to open the note at');
    await page.goto(`${BASE}${PAGE}`, { waitUntil: 'load' });
    await arrived(page);
    await settled(page);
    await page.waitForFunction((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown'), HEAD, { timeout: 2000 }).catch(() => broken('the note did not open past its head where the reader stopped'));
    const kept = (await page.evaluate(() => window.__runningHead))[0];
    if (kept.opacity !== 1) fail('still-on-arrival', `on opening at the kept place the running head arrived as ${JSON.stringify(kept)}, want it simply there`);
    await context.close();
  }
  {
    // A reader who asked for less motion: no fade, even when they move.
    const context = await open({ motion: 'reduce' });
    const page = await visit(context, PAGE, { sample: true });
    await page.mouse.move(700, 500);
    for (let i = 0; i < 24 && !(await shownNow(page)); i += 1) {
      await page.mouse.wheel(0, 200);
      await frames(page, 2);
    }
    await page.waitForFunction((head) => document.querySelector(head)?.hasAttribute('data-running-head-shown'), HEAD, { timeout: 2000 }).catch(() => broken('the running head never came as the reader scrolled'));
    const sample = (await page.evaluate(() => window.__runningHead))[0];
    const longest = Math.max(...sample.durations.split(',').map((d) => Number.parseFloat(d)));
    if (longest > 0.001) fail('still-for-less-motion', `for a reader who asked for less motion the running head fades over ${sample.durations}`);
    await context.close();
  }

  // --- Room ------------------------------------------------------------------
  for (const language of ['zh-Hant', 'en']) {
    const context = await open({ language });
    const page = await visit(context, PAGE);
    const past = await page.evaluate((selector) => document.querySelectorAll(selector)[1].id, SECTION);
    let drawnSomewhere = false;
    let missingSomewhere = false;
    for (const width of WIDTHS) {
      await page.setViewportSize({ width, height: HEIGHT });
      await sectionTop(page, past);
      await frames(page);
      await page.waitForTimeout(100);
      const row = await page.evaluate((head) => {
        const spacer = document.querySelector('.y-header__spacer').getBoundingClientRect();
        const button = document.querySelector(head);
        const box = button.getBoundingClientRect();
        return {
          room: spacer.width,
          drawn: button.checkVisibility({ visibilityProperty: true }) && box.width > 0,
          inside: box.left >= spacer.left - 0.5 && box.right <= spacer.right + 0.5,
        };
      }, HEAD);
      const want = row.room >= ROOM;
      if (row.drawn !== want) {
        fail('drawn-only-with-room', `${language} at ${width}px the row leaves ${Math.round(row.room)}px and the running head is ${row.drawn ? 'drawn' : 'left out'}, want it ${want ? 'drawn' : 'left out'} (drawn from ${ROOM}px)`);
      }
      if (row.drawn && !row.inside) fail('drawn-only-with-room', `${language} at ${width}px the running head runs out of the room the row leaves it`);
      if (row.drawn) drawnSomewhere = true;
      else missingSomewhere = true;
    }
    if (!drawnSomewhere || !missingSomewhere) broken(`${language}: the widths asked never ${drawnSomewhere ? 'leave it out' : 'draw it'}`);
    await context.close();
  }

  // --- Only a note's own page ------------------------------------------------
  {
    const context = await open();
    for (const path of ELSEWHERE) {
      const page = await visit(context, path);
      const found = await page.evaluate(({ head, pop }) => document.querySelectorAll(`${head}, ${pop}`).length, { head: HEAD, pop: POP });
      if (found > 0) fail('only-on-a-note', `${path} carries a running head`);
      await page.close();
    }
    await context.close();
  }

  const missed = planted?.proof();
  if (missed) notApplied(missed);
  console.log('PASS running-head: nothing without script; out of sight over the note\'s head and its introduction, there once the first section begins, both ways; names the section the contents mark, numbered only where the margin counts; opens the contents in place, every section listed and the current one marked, a pick goes there and closes it, Escape and a press on nothing hand focus back to the press while a press on a control or the text keeps it there; fades as the reader moves, still on a reload or a kept place and for less motion; drawn only where the row has room, in both languages; on a note\'s page only');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      const missed = planted?.proof();
      const { target } = MUTATIONS[MUTATE];
      if (missed) {
        console.error(`NOT-APPLIED running-head: ${missed}`);
        console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      } else if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
