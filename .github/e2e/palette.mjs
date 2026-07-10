// Behavior lock: the command palette opens centered and paints an opaque
// panel over the dimmed backdrop. Regression class: a universal CSS reset
// zeroing dialog margins (pins it to the left edge), or a later rule
// zeroing the panel fill (transparent body over the backdrop).
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (any page).
// MUTATE names one of the self-test modes below; MUTATE=list prints them. Each
// injects the regression this probe exists to catch, so a mutated run that
// stayed green would mean the lock is worth nothing.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

// The assertions that can fire the lock, each named for what it guards. This
// flow reaches two of them in turn, so "the lock fired" does not say which. A
// mutation declares the site it aims at, and only that site's firing is a
// detection: break the centering and run the mutation that makes the panel
// transparent, and the centering assertion stops the run before the panel is
// ever measured. Reported as a catch, it would say the opacity lock still works
// on the day it stopped being reached at all.
const SITES = ['centered', 'opaque'];

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle matched
// nothing. Only the first is ever reported as a caught mutation — otherwise a
// crash that happens to exit 1 would read as a detection — and only when the
// site that fired is the site the mode aimed at.
class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, msg) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN palette: an assertion names the unknown site ${site}`);
  throw new LockFired(site, `FAIL palette: ${msg}`);
};
const broken = (msg) => { throw new ProbeBroken(`BROKEN palette: ${msg}`); };
const notApplied = (msg) => { throw new NotApplied(`NOT-APPLIED palette: ${msg}`); };

// The alpha of a computed CSS color, whatever notation the browser chose.
//
// A computed color is serialized in its own colour space, so the panel's fill
// comes back as oklch(...) here and would come back as rgb(...) elsewhere; a
// probe that only reads rgb() cannot see a translucent panel painted in any
// other space. Every functional notation carries its alpha the same way — after
// a slash, as a number or a percentage — and only the legacy comma forms put it
// fourth. A colour whose alpha is exactly 1 is serialized without an alpha
// component at all, which is why the absence of one means opaque rather than
// unknown. An alpha that is present but unreadable returns NaN, so the caller
// can refuse to guess.
const alphaOf = (color) => {
  const c = color.trim();
  if (c === 'transparent') return 0;
  const fn = c.match(/^[a-z-]+\((.*)\)$/i);
  if (!fn) return 1;
  const args = fn[1];
  const slash = args.lastIndexOf('/');
  let raw;
  if (slash >= 0) {
    raw = args.slice(slash + 1);
  } else {
    const commas = args.split(',');
    if (commas.length !== 4) return 1;
    raw = commas[3];
  }
  const t = raw.trim();
  const n = parseFloat(t);
  if (!Number.isFinite(n)) return NaN;
  return t.endsWith('%') ? n / 100 : n;
};

// Injects one rule, first proving its selector matches something. A rule that
// styles no element — because the panel was renamed, say — leaves the page
// exactly as it was, and the probe would then pass the self-test while showing
// nothing at all about its ability to fail.
const injectRule = async (page, selector, declarations) => {
  const matched = await page.evaluate((s) => document.querySelectorAll(s).length, selector);
  if (matched === 0) notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  await page.addStyleTag({ content: `${selector} { ${declarations} }` });
};

// Every mutation this probe can inject lives in this table; the dispatch below
// is a lookup into it, and MUTATE=list prints its keys. A mode that exists but
// is not listed cannot happen. Each names the assertion site it aims at, which
// is the only site whose firing that mode may report as a catch.
const MUTATIONS = {
  'palette-margins': {
    target: 'centered',
    apply: (page) => injectRule(page, '.y-searchdialog', 'margin-left: 0 !important; margin-right: auto !important;'),
  },
  'palette-fill': {
    target: 'opaque',
    apply: (page) => injectRule(page, 'dialog.y-searchdialog', 'background: transparent !important;'),
  },
  'palette-fill-partial': {
    target: 'opaque',
    apply: (page) => injectRule(page, 'dialog.y-searchdialog', 'background: oklch(0.988 0.003 106 / 0.5) !important;'),
  },
};

// A mutation aiming at a site no assertion carries could never be caught, and
// the run would read as a probe that let the regression walk past it. An
// assertion no mutation aims at is a lock nothing has ever watched fail. Both
// are checked before anything else, so even MUTATE=list refuses to answer for a
// table that has drifted from the assertions it is supposed to cover.
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`palette: mutation ${name} aims at the unknown assertion site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`palette: the ${site} assertion is aimed at by no mutation, so nothing shows it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`palette: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');

  if (MUTATE) await MUTATIONS[MUTATE].apply(page);

  await page.keyboard.press('ControlOrMeta+k');
  const dialog = page.locator('dialog.y-searchdialog[open]');
  await dialog.waitFor({ state: 'visible', timeout: 3000 });

  const box = await dialog.boundingBox();
  if (!box) broken('the palette dialog has no box, so nothing can be measured on it');
  const viewportCenter = 1280 / 2;
  const dialogCenter = box.x + box.width / 2;
  if (Math.abs(dialogCenter - viewportCenter) > 2) {
    fail('centered', `not centered: dialog center ${dialogCenter}px vs viewport center ${viewportCenter}px`);
  }

  const bg = await dialog.evaluate((el) => getComputedStyle(el).backgroundColor);
  const alpha = alphaOf(bg);
  if (!Number.isFinite(alpha)) broken(`cannot read an alpha from computed background ${bg}`);
  if (alpha < 1) fail('opaque', `panel not opaque: computed background ${bg}`);

  console.log(`PASS palette: centered (${dialogCenter}px) and opaque (${bg})`);
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
      else console.error(`no catch: ${MUTATE} injects a regression the ${target} assertion watches for, but the ${err.site} assertion fired first — something unrelated is broken`);
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
