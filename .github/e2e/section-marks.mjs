// Behavior lock for the controls beside each section of a note and for what the
// reader finds there on coming back.
//
// "Not sure yet" is one fixed name: pressing it changes whether the section is
// marked and never what the button is called. The mark and its day are in the
// page the server sends, so a reader who returns the next day finds them with no
// script running at all; the sentence a press earns lapses by itself; the
// buttons are drawn and quiet by default and, where the reader chose to see them
// only on hover, wait out a dwell and never blink under a pointer that is
// resting while the page scrolls. And none of this costs the reader a search
// result: the words beside a heading are drawn from attributes, because text
// between a heading and the paragraph after it makes a browser's landing on the
// heading miss.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with several sections in a vault whose
// contract names a place to write a thought), and MUTATE. MUTATE=list prints
// every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Reading%20flow.md';
const MUTATE = process.env.MUTATE || '';
const NOTE = decodeURIComponent(PAGE.slice('/notes/'.length));
const NAME = '還不確定';

const SITES = [
  'the-name-never-changes',
  'a-mark-and-its-day-are-in-the-page',
  'the-status-line-lapses',
  'the-buttons-follow-the-setting',
  'a-landing-on-a-heading-survives',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN section-marks: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL section-marks: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN section-marks: ${message}`); };

const planted = { seen: 0, matched: 0 };
const addRule = (rule) => async (page) => {
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    planted.seen += 1;
    planted.matched += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
};
const rewriteDocument = (needle, replacement) => async (page) => {
  await page.route((url) => url.pathname === decodeURI(PAGE) || url.pathname === PAGE, async (route) => {
    const response = await route.fetch();
    if (!(response.headers()['content-type'] ?? '').includes('text/html')) {
      await route.fulfill({ response });
      return;
    }
    const original = await response.text();
    planted.seen += 1;
    if (original.includes(needle)) planted.matched += 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
};
const rewriteModule = (needle, replacement) => async (page) => {
  await page.route('**/uncertainty.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    planted.seen += 1;
    if (original.split(needle).length === 2) planted.matched += 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
};
const addScript = (source) => async (page) => {
  await page.route('**/static/yomihon.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    planted.seen += 1;
    planted.matched += 1;
    await route.fulfill({ response, body: `${original}\n${source}\n` });
  });
};

const MUTATIONS = {
  // Once marked, the button says something else.
  'rename-the-button-when-marked': {
    target: 'the-name-never-changes',
    apply: addScript(`
      new MutationObserver((records) => {
        for (const record of records) {
          if (record.attributeName === 'aria-pressed') record.target.setAttribute('aria-label', record.target.getAttribute('aria-pressed') === 'true' ? 'Remove mark' : '${NAME}');
        }
      }).observe(document.body, { subtree: true, attributes: true, attributeFilter: ['aria-pressed'] });
    `),
  },
  // The page forgets the mark until a script runs and asks the file.
  'send-the-mark-only-to-a-script': {
    target: 'a-mark-and-its-day-are-in-the-page',
    apply: rewriteDocument('aria-pressed="true"', 'aria-pressed="false"'),
  },
  // The sentence a press earns is left standing.
  'leave-the-status-line-standing': {
    target: 'the-status-line-lapses',
    apply: rewriteModule('      lapse(said);\n', '      void lapse;\n'),
  },
  // Buttons that are meant to be there are gone until the pointer comes.
  'draw-the-buttons-only-on-hover-by-default': {
    target: 'the-buttons-follow-the-setting',
    apply: addRule('.y-article .y-margin__act:not([aria-pressed="true"]){opacity:0 !important;pointer-events:none !important}'),
  },
  // The dwell is dropped, so they blink under a resting pointer.
  'draw-the-hover-buttons-with-no-dwell': {
    target: 'the-buttons-follow-the-setting',
    apply: addRule(':root[data-sectionbuttons="hover"] .y-article .y-sect:hover .y-margin__act{transition-delay:0s !important}'),
  },
  // Words as text between a heading and the paragraph after it.
  'put-the-words-in-text-beside-the-heading': {
    target: 'a-landing-on-a-heading-survives',
    apply: rewriteDocument(`data-text="${NAME}" disabled></button>`, `data-text="${NAME}" disabled>${NAME}</button>`),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`section-marks: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`section-marks: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`section-marks: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const records = async () => (await fetch(`${BASE}/uncertainties`)).json();
const today = () => {
  const now = new Date();
  const two = (n) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const open = async (options = {}) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, ...options });
  const page = await context.newPage();
  if (MUTATE) await MUTATIONS[MUTATE].apply(page);
  return { context, page };
};
const settled = async (page, selector, wanted) => {
  try {
    await page.waitForFunction(({ target, value }) => {
      const button = document.querySelector(target);
      return button && !button.disabled && button.getAttribute('aria-pressed') === value && button.getAttribute('aria-disabled') !== 'true';
    }, { target: selector, value: String(wanted) }, { timeout: 4000 });
  } catch {
    broken(`${selector} never settled at aria-pressed=${wanted}`);
  }
};

try {
  if ((await records()).length !== 0) broken('the fixture starts with marks');
  const first = '[data-section-mark][data-section-anchor="the-first-mechanism"]';

  // A reader presses one button.
  const reading = await open();
  const { page } = reading;
  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (response?.status() !== 200) broken(`${PAGE} answered ${response?.status()}, want 200`);
  const applied = () => {
    if (planted.seen === 0 || planted.matched === 0) throw new NotApplied(`NOT-APPLIED section-marks: ${MUTATE}: the planted change reached ${planted.seen} loads and matched in ${planted.matched}`);
  };
  if (MUTATE && MUTATE !== 'send-the-mark-only-to-a-script') applied();
  await settled(page, first, false);
  const buttons = page.getByRole('button', { name: NAME, exact: true });
  const before = await buttons.count();
  if (before < 3) broken(`${PAGE} has ${before} section buttons, want at least three`);
  await page.locator(first).click();
  await settled(page, first, true);
  const after = await page.getByRole('button', { name: NAME, exact: true }).count();
  if (after !== before) fail('the-name-never-changes', `${before} buttons were named ${JSON.stringify(NAME)} before the press and ${after} after, so the name changed with the state`);
  const marked = await records();
  if (marked.length !== 1 || marked[0].path !== NOTE || marked[0].anchor !== 'the-first-mechanism') broken(`the press stored ${JSON.stringify(marked)}`);

  // The status line lapses by itself.
  const said = page.locator('[data-section-said]').first();
  const earned = (await said.textContent())?.trim();
  if (!earned) fail('the-status-line-lapses', 'pressing the button earned no sentence');
  await page.waitForFunction((selector) => document.querySelector(selector).textContent.trim() === '', '[data-section-said]', { timeout: 6500 }).catch(() => {
    fail('the-status-line-lapses', `the sentence ${JSON.stringify(earned)} was still on the page 6.5 seconds after the press`);
  });
  await reading.context.close();

  // The next day: a new visit with no script running at all.
  const next = await open({ javaScriptEnabled: false });
  await next.page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (MUTATE === 'send-the-mark-only-to-a-script') applied();
  const found = await next.page.evaluate((selector) => {
    const button = document.querySelector(selector);
    const date = button?.parentElement.querySelector('[data-section-date]');
    return {
      pressed: button?.getAttribute('aria-pressed'),
      shown: Boolean(button && button.checkVisibility()),
      day: date && !date.hidden ? getComputedStyle(date, '::before').content : null,
      dateTime: date?.getAttribute('datetime') ?? null,
    };
  }, first);
  if (found.pressed !== 'true' || !found.shown) fail('a-mark-and-its-day-are-in-the-page', `with no script running the marked section's button is ${JSON.stringify(found)}`);
  if (found.dateTime !== today() || found.day !== `"${today()}"`) fail('a-mark-and-its-day-are-in-the-page', `the day beside the mark is ${JSON.stringify(found)}, want ${today()}`);
  await next.context.close();

  // Buttons: quiet and drawn by default; on hover only where the reader chose it.
  const resting = async (page) => page.evaluate(() => {
    const button = document.querySelector('[data-section-mark][aria-pressed="false"]');
    return { opacity: Number(getComputedStyle(button).opacity), events: getComputedStyle(button).pointerEvents };
  });
  {
    const always = await open();
    await always.page.goto(BASE + PAGE, { waitUntil: 'load' });
    const state = await resting(always.page);
    if (state.opacity < 1 || state.events === 'none') fail('the-buttons-follow-the-setting', `by default an unmarked button rests at ${JSON.stringify(state)}, want drawn and pressable`);
    await always.context.close();

    const hover = await open();
    await hover.context.addCookies([{ name: 'yomihon_sectionbuttons', value: 'hover', url: BASE }]);
    const hoverPage = hover.page;
    await hoverPage.goto(BASE + PAGE, { waitUntil: 'load' });
    await hoverPage.evaluate(() => document.querySelector('.y-prose .y-sect').scrollIntoView({ block: 'center' }));
    const rest = await resting(hoverPage);
    if (rest.opacity !== 0 || rest.events !== 'none') fail('the-buttons-follow-the-setting', `with the setting on hover an unmarked button rests at ${JSON.stringify(rest)}, want hidden and not pressable`);
    const target = hoverPage.locator('.y-prose .y-sect').first();
    const opacityOf = () => hoverPage.evaluate(() => Number(getComputedStyle(document.querySelector('[data-section-mark][aria-pressed="false"]')).opacity));
    await target.locator('h3').hover();
    await hoverPage.waitForTimeout(150);
    const early = await opacityOf();
    if (early > 0.05) fail('the-buttons-follow-the-setting', `150ms into resting on a heading the buttons are at ${early}, want them still waiting out the dwell`);
    await hoverPage.waitForTimeout(550);
    if ((await opacityOf()) < 0.99) fail('the-buttons-follow-the-setting', 'after the dwell the buttons are not drawn');
    await hoverPage.mouse.move(5, 5);
    await hoverPage.waitForTimeout(300);
    if ((await opacityOf()) > 0.05) fail('the-buttons-follow-the-setting', 'the buttons stay drawn after the pointer leaves');
    // Scrolling under a pointer that rests never draws them: each heading passes
    // under it for far less than the dwell.
    await hoverPage.evaluate(() => window.scrollTo(0, 0));
    await hoverPage.mouse.move(640, 400);
    let drawn = 0;
    for (let step = 0; step < 9; step += 1) {
      await hoverPage.mouse.wheel(0, 140);
      await hoverPage.waitForTimeout(60);
      drawn = Math.max(drawn, await hoverPage.evaluate(() => Math.max(...[...document.querySelectorAll('[data-section-mark][aria-pressed="false"]')].map((button) => Number(getComputedStyle(button).opacity)))));
    }
    if (drawn > 0.05) fail('the-buttons-follow-the-setting', `scrolling under a resting pointer drew the buttons to ${drawn}`);
    // A keyboard reaches one at once, with no dwell.
    await hoverPage.locator('[data-section-mark][aria-pressed="false"]').first().focus();
    await hoverPage.keyboard.press('Shift+Tab');
    await hoverPage.keyboard.press('Tab');
    await hoverPage.waitForTimeout(60);
    if ((await opacityOf()) < 0.99) fail('the-buttons-follow-the-setting', 'a button the keyboard reached is not drawn at once');
    await hover.context.close();
  }

  // A search result lands on a heading by the text that follows it.
  {
    const landing = await open();
    await landing.page.goto(`${BASE}/`, { waitUntil: 'load' });
    const href = `${PAGE}#:~:text=The%20first%20mechanism,-Sentence%200%20of%20the%20first`;
    await landing.page.evaluate((address) => {
      const link = document.createElement('a');
      link.id = 'probe-landing';
      link.href = address;
      link.textContent = 'landing';
      document.body.prepend(link);
    }, href);
    await landing.page.click('#probe-landing');
    await landing.page.waitForLoadState('load');
    await landing.page.waitForTimeout(700);
    const placed = await landing.page.evaluate(() => {
      const heading = document.getElementById('the-first-mechanism').getBoundingClientRect();
      return { scrollY: Math.round(scrollY), top: Math.round(heading.top), height: innerHeight };
    });
    if (placed.scrollY < 100 || placed.top < 0 || placed.top > placed.height) fail('a-landing-on-a-heading-survives', `following a result that names the heading left the page at ${JSON.stringify(placed)}, the heading out of the window`);
    await landing.context.close();
  }
  console.log('PASS section-marks: the name never changes with the mark; the mark and its day are in the page with no script; the status line lapses; the buttons are quiet by default and wait out a dwell on hover; a result still lands on a heading');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    if (MUTATE && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
  try {
    for (const { path, anchor } of await records()) {
      if (path === NOTE) await fetch(`${BASE}/uncertainties`, { method: 'POST', body: new URLSearchParams({ path, anchor }) });
    }
  } catch (error) {
    console.error(`BROKEN section-marks: cleanup failed: ${error}`);
    process.exitCode = 2;
  }
}
