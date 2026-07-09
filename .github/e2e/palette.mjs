// Behavior lock: the command palette opens centered and paints an opaque
// panel over the dimmed backdrop. Regression class: a universal CSS reset
// zeroing dialog margins (pins it to the left edge), or a later rule
// zeroing the panel fill (transparent body over the backdrop).
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (any page).
// Set MUTATE=palette-margins or MUTATE=palette-fill to self-test that the
// probe can fail: it injects the regression and must then exit non-zero.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

const fail = (msg) => { throw new Error(`FAIL palette: ${msg}`); };

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
  const alpha = (() => {
    const m = bg.match(/rgba?\(([^)]+)\)/);
    if (!m) return bg === 'transparent' ? 0 : 1;
    const parts = m[1].split(/[,/]/).map((s) => s.trim());
    return parts.length === 4 ? parseFloat(parts[3]) : 1;
  })();
  if (alpha < 1) fail(`panel not opaque: computed background ${bg}`);

  console.log(`PASS palette: centered (${dialogCenter}px) and opaque (${bg})`);
} catch (err) {
  console.error(err instanceof Error && err.message.startsWith('FAIL') ? err.message : err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
