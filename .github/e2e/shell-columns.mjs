// A two-column page ends with its content, including its own bottom padding.
// An empty reading-aids rail must not become another window-height grid row.
// The Go catalog covers every template using this shell; this lock measures
// the health page's real CSS layout after its arrival animation has finished.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/health';
const MUTATE = process.env.MUTATE || '';
const SITE = 'content-ends-the-document';
const MODE = 'restore-empty-right-rail';
const MAIN = '.y-shell2 > main.y-main';

class LockFired extends Error {}
class NotApplied extends Error {}

if (MUTATE === 'list') {
  console.log(MODE);
  process.exit(0);
}
if (MUTATE && MUTATE !== MODE) {
  console.error(`NOT-APPLIED shell-columns: unknown mode ${MUTATE}`);
  console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
  process.exit(2);
}


const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  let measured = 0;
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1280, height: 900 }, { width: 390, height: 844 }]) {
    for (const lang of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        const context = await browser.newContext({ viewport });
        try {
          await context.addCookies([
            { name: 'yomihon_lang', value: lang, url: BASE },
            { name: 'yomihon_theme', value: theme, url: BASE },
          ]);
          const page = await context.newPage();
          const response = await page.goto(BASE + PAGE);
          if (response?.status() !== 200) throw new Error(`BROKEN shell-columns: ${PAGE} answered ${response?.status()}, want 200`);
          const count = await page.locator(MAIN).count();
          if (count !== 1) throw new NotApplied(`NOT-APPLIED shell-columns: ${MAIN} matched ${count} nodes, want exactly 1`);
          if (MUTATE) {
            const applied = await page.locator(MAIN).evaluate((main) => {
              const rail = document.createElement('aside');
              rail.className = 'y-rail-right';
              main.after(rail);
              return rail.parentElement === main.parentElement && rail.previousElementSibling === main && main.parentElement.querySelectorAll(':scope > aside.y-rail-right').length === 1;
            });
            if (!applied) throw new NotApplied(`NOT-APPLIED shell-columns: ${MODE} did not insert exactly one empty rail after main`);
            console.log(`MUTATION_APPLIED: ${MODE} at exactly one ${MAIN}`);
          }
          await page.evaluate(() => document.fonts.ready);
          await arrived(page);
          const shape = await page.locator(MAIN).evaluate((main) => {
            const shell = main.parentElement;
            return {
              documentHeight: document.documentElement.scrollHeight,
              mainBottom: main.getBoundingClientRect().bottom + scrollY,
              shellPadding: parseFloat(getComputedStyle(shell).paddingBottom),
              bodyPadding: parseFloat(getComputedStyle(document.body).paddingBottom),
              viewportHeight: innerHeight,
              documentWidth: document.documentElement.scrollWidth,
              viewportWidth: innerWidth,
              language: document.documentElement.lang,
              theme: document.documentElement.dataset.theme,
            };
          });
          console.log(`SHELL_GEOMETRY_INVOKED: ${viewport.width}px/${lang}/${theme} ${JSON.stringify(shape)}`);
          if (shape.language !== lang || shape.theme !== theme) {
            throw new Error(`BROKEN shell-columns: requested ${lang}/${theme}, got ${shape.language}/${shape.theme}`);
          }
          const end = Math.max(shape.viewportHeight, shape.mainBottom + shape.shellPadding + shape.bodyPadding);
          if (Math.abs(shape.documentHeight - end) > 1) {
            throw new LockFired(`caught: ${SITE} at ${viewport.width}px/${lang}/${theme}: document ends at ${shape.documentHeight}px, content with bottom padding ends at ${end}px`);
          }
          measured += 1;
        } finally {
          await context.close();
        }
      }
    }
  }
  console.log(`PASS shell-columns: ${measured} width/language/theme contexts end at their content with bottom padding`);
} catch (error) {
  console.error(error.message || error);
  if (error instanceof NotApplied) {
    if (MUTATE) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
