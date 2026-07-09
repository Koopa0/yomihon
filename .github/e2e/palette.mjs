// Behavior lock: the command palette opens centered and paints an opaque
// panel over the dimmed backdrop. Regression class: a universal CSS reset
// zeroing dialog margins (pins it to the left edge), or a later rule
// zeroing the panel fill (transparent body over the backdrop).
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (any page).
// Set MUTATE to palette-margins, palette-fill (a fully transparent panel) or
// palette-fill-partial (a half-transparent one) to self-test that the probe can
// fail: each injects the regression it exists to catch and must then exit
// non-zero.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

const fail = (msg) => { throw new Error(`FAIL palette: ${msg}`); };

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

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  await page.waitForSelector('html[data-js]');

  if (MUTATE === 'palette-margins') {
    await page.addStyleTag({ content: '.y-searchdialog { margin-left: 0 !important; margin-right: auto !important; }' });
  }
  if (MUTATE === 'palette-fill') {
    await page.addStyleTag({ content: 'dialog.y-searchdialog { background: transparent !important; }' });
  }
  if (MUTATE === 'palette-fill-partial') {
    await page.addStyleTag({ content: 'dialog.y-searchdialog { background: oklch(0.988 0.003 106 / 0.5) !important; }' });
  }

  await page.keyboard.press('ControlOrMeta+k');
  const dialog = page.locator('dialog.y-searchdialog[open]');
  await dialog.waitFor({ state: 'visible', timeout: 3000 });

  const box = await dialog.boundingBox();
  if (!box) fail('palette dialog has no box');
  const viewportCenter = 1280 / 2;
  const dialogCenter = box.x + box.width / 2;
  if (Math.abs(dialogCenter - viewportCenter) > 2) {
    fail(`not centered: dialog center ${dialogCenter}px vs viewport center ${viewportCenter}px`);
  }

  const bg = await dialog.evaluate((el) => getComputedStyle(el).backgroundColor);
  const alpha = alphaOf(bg);
  if (!Number.isFinite(alpha)) fail(`cannot read an alpha from computed background ${bg}`);
  if (alpha < 1) fail(`panel not opaque: computed background ${bg}`);

  console.log(`PASS palette: centered (${dialogCenter}px) and opaque (${bg})`);
} catch (err) {
  console.error(err instanceof Error && err.message.startsWith('FAIL') ? err.message : err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
