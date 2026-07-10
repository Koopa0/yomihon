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

// Three outcomes a caller has to tell apart: the lock fired, the probe cannot
// see the thing it claims to watch, and a mutation whose needle matched
// nothing. Only the first is ever reported as a caught mutation — otherwise a
// crash that happens to exit 1 would read as a detection.
class LockFired extends Error {}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (msg) => { throw new LockFired(`FAIL palette: ${msg}`); };
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
// is not listed cannot happen.
const MUTATIONS = {
  'palette-margins': (page) => injectRule(page, '.y-searchdialog', 'margin-left: 0 !important; margin-right: auto !important;'),
  'palette-fill': (page) => injectRule(page, 'dialog.y-searchdialog', 'background: transparent !important;'),
  'palette-fill-partial': (page) => injectRule(page, 'dialog.y-searchdialog', 'background: oklch(0.988 0.003 106 / 0.5) !important;'),
};

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

  if (MUTATE) await MUTATIONS[MUTATE](page);

  await page.keyboard.press('ControlOrMeta+k');
  const dialog = page.locator('dialog.y-searchdialog[open]');
  await dialog.waitFor({ state: 'visible', timeout: 3000 });

  const box = await dialog.boundingBox();
  if (!box) broken('the palette dialog has no box, so nothing can be measured on it');
  const viewportCenter = 1280 / 2;
  const dialogCenter = box.x + box.width / 2;
  if (Math.abs(dialogCenter - viewportCenter) > 2) {
    fail(`not centered: dialog center ${dialogCenter}px vs viewport center ${viewportCenter}px`);
  }

  const bg = await dialog.evaluate((el) => getComputedStyle(el).backgroundColor);
  const alpha = alphaOf(bg);
  if (!Number.isFinite(alpha)) broken(`cannot read an alpha from computed background ${bg}`);
  if (alpha < 1) fail(`panel not opaque: computed background ${bg}`);

  console.log(`PASS palette: centered (${dialogCenter}px) and opaque (${bg})`);
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
