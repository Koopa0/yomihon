// Captures the README's reading-page pictures from the example vault, so a
// screenshot is something the repository can regenerate rather than something
// somebody once took. The three things that make two runs comparable are fixed
// here: the window, the note, and the light ground.
//
// A picture is for a reader of one README, so the note in it has to be written
// in the language that README is in. The interface language alone cannot show
// that: a note stays in its author's language while the frame around it
// changes, so the frame can read Chinese over an English article and look
// right. Both languages are checked below, and they have to agree.
//
// Which note each picture opens is named by the caller and nowhere else. A
// default here would be a second copy of that choice, and the copy that goes
// stale is always the one nobody passes.
//
// Env: YOMIHON_BASE, LANG_CHOICE (zh-Hant | en), OUT, PAGE_PATH.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH;
const LANG = process.env.LANG_CHOICE || 'zh-Hant';
const OUT = process.env.OUT;

if (!OUT) {
  console.error('screenshot: set OUT to the file to write');
  process.exit(2);
}

if (!PAGE) {
  console.error('screenshot: set PAGE_PATH to the note to open');
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
  const authored = await page.evaluate(() =>
    Array.from(document.querySelectorAll('article.y-article'), (article) => article.getAttribute('lang')),
  );
  if (authored.length !== 1 || authored[0] !== LANG) {
    console.error(`screenshot: ${PAGE} carries note languages ${JSON.stringify(authored)}, want exactly ["${LANG}"]: the picture would show a note nobody reading this README can read`);
    process.exit(1);
  }
  await page.screenshot({ path: OUT });
  console.log(`screenshot: wrote ${OUT} (${LANG})`);
} finally {
  await browser.close();
}
