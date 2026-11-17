// The interface speaks the language the browser asks for when the reader has
// chosen none, and a language the reader chose beats the browser's. Each case
// opens a context that names its own locale, so the pinned default of the other
// probes does not decide it.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE;
const PAGE = process.env.PAGE_PATH || '/';
if (!BASE) {
  console.error('language-follows-browser: set YOMIHON_BASE to the running server');
  process.exit(2);
}

const cases = [
  { name: 'no choice, the browser asks for English', locale: 'en-US', cookie: null, want: 'en' },
  { name: 'no choice, the browser asks for Chinese', locale: 'zh-TW', cookie: null, want: 'zh-Hant' },
  { name: 'a chosen Chinese beats a browser asking for English', locale: 'en-US', cookie: 'zh-Hant', want: 'zh-Hant' },
  { name: 'a chosen English beats a browser asking for Chinese', locale: 'zh-TW', cookie: 'en', want: 'en' },
];

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const failures = [];
try {
  for (const c of cases) {
    const context = await browser.newContext({ locale: c.locale });
    try {
      if (c.cookie) {
        await context.addCookies([{ name: 'yomihon_lang', value: c.cookie, url: BASE }]);
      }
      const page = await context.newPage();
      await page.goto(BASE + PAGE);
      const got = await page.evaluate(() => document.documentElement.lang);
      if (got !== c.want) failures.push(`${c.name}: <html lang> is "${got}", want "${c.want}"`);
    } finally {
      await context.close();
    }
  }
} finally {
  await browser.close();
}

if (failures.length > 0) {
  console.error(`FAIL language-follows-browser:\n  ${failures.join('\n  ')}`);
  process.exit(1);
}
console.log(`PASS language-follows-browser: ${cases.length} cases, the cookie wins and the browser decides when there is none`);
