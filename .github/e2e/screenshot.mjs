// Captures the README's reading-page pictures from the example vault, so a
// screenshot is something the repository can regenerate rather than something
// somebody once took. The three things that make two runs comparable are fixed
// here: the window, the note, and the light ground.
//
// Env: YOMIHON_BASE, LANG_CHOICE (zh-Hant | en), OUT, PAGE_PATH.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/What%20yomihon%20is.md';
const LANG = process.env.LANG_CHOICE || 'zh-Hant';
const OUT = process.env.OUT;

if (!OUT) {
  console.error('screenshot: set OUT to the file to write');
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    colorScheme: 'light',
  });
  // The reader's own choices, set outright rather than left to whatever the
  // machine taking the picture happens to prefer.
  await context.addCookies([
    { name: 'yomihon_lang', value: LANG, url: BASE },
    { name: 'yomihon_theme', value: 'light', url: BASE },
  ]);
  const page = await context.newPage();
  const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  if (!response || response.status() !== 200) {
    console.error(`screenshot: ${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
    process.exit(1);
  }
  const declared = await page.evaluate(() => document.documentElement.lang);
  if (declared !== LANG) {
    console.error(`screenshot: the page declares ${JSON.stringify(declared)}, want ${JSON.stringify(LANG)}: the picture would show the wrong language`);
    process.exit(1);
  }
  await page.screenshot({ path: OUT });
  console.log(`screenshot: wrote ${OUT} (${LANG})`);
} finally {
  await browser.close();
}
