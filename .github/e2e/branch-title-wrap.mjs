// A branch name belongs to the author and must fit the course at reflow width.
// Measure the laid-out words as well as the document: clipping an overflowing
// name would remove the scrollbar while still hiding the author's words.
// Env: YOMIHON_BASE, PAGE_PATH (a course with a local branch), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/syllabus/Maps/branches.md';
const MUTATE = process.env.MUTATE || '';
const SITE = 'branch-heading-reflows-at-320';
const NAMES = [
  'If your notes are not all in English',
  'Averylongauthoredbranchheadingwithoutspacesmustremainentirelyreadableatthereflowwidth',
];
const MUTATIONS = {
  'restore-nonshrinking-name': { property: 'flex', replacement: 'none', computed: 'flexShrink', wanted: '0' },
  'restore-normal-word-wrap': { property: 'overflow-wrap', replacement: 'normal', computed: 'overflowWrap', wanted: 'normal' },
  'clip-name-to-one-pixel': { property: 'flex', replacement: '0 0 1px', computed: 'flexBasis', wanted: '1px', clip: true },
};
class LockFired extends Error {}
class NotApplied extends Error {}

const arrived = (page) => page.waitForFunction(
  async () => {
    if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return true;
  },
  null,
  { timeout: 3000 },
);

const mutate = async (page, mode) => {
  let requests = 0;
  let sites = 0;
  let properties = 0;
  let changedSites = 0;
  await page.route('**/static/app.css*', async (route) => {
    if (new URL(route.request().url()).pathname !== '/static/app.css') {
      await route.fallback();
      return;
    }
    requests += 1;
    const response = await route.fetch();
    if (response.status() !== 200) throw new Error(`stylesheet response=${response.status()}`);
    const css = await response.text();
    const blocks = [...css.matchAll(/\.y-module__name\s*\{[^}]*\}/g)];
    sites += blocks.length;
    let body = css;
    if (blocks.length === 1) {
      const block = blocks[0][0];
      const declaration = new RegExp(`(^|[;{]\\s*)${mode.property}\\s*:[^;]+;`, 'g');
      const hits = [...block.matchAll(declaration)];
      properties += hits.length;
      if (hits.length === 1) {
        let changed = block.replace(declaration, `$1${mode.property}: ${mode.replacement};`);
        if (mode.clip) {
          if (/(^|[;{]\s*)overflow\s*:/.test(block)) {
            await route.fulfill({ response });
            return;
          }
          changed = changed.replace(/}$/, 'overflow: hidden;}');
        }
        if (changed !== block) changedSites += 1;
        body = css.replace(block, changed);
      }
    }
    await route.fulfill({ response, body });
  });
  return async () => {
    if (requests !== 1 || sites !== 1 || properties !== 1 || changedSites !== 1) {
      throw new NotApplied(`NOT-APPLIED branch-title-wrap: requests=${requests}, class sites=${sites}, property sites=${properties}, changed sites=${changedSites}; want exactly one each`);
    }
    const got = await page.locator('.y-module__name').first().evaluate((el, property) => getComputedStyle(el)[property], mode.computed);
    if (got !== mode.wanted) throw new NotApplied(`NOT-APPLIED branch-title-wrap: computed ${mode.computed}=${got}, want ${mode.wanted}`);
    if (mode.clip) {
      const clip = await page.locator('.y-module__name').first().evaluate((el) => ({ width: el.getBoundingClientRect().width, overflow: getComputedStyle(el).overflowX }));
      if (clip.width !== 1 || clip.overflow !== 'hidden') throw new NotApplied(`NOT-APPLIED branch-title-wrap: clipping was not applied: ${JSON.stringify(clip)}`);
    }
  };
};

if (MUTATE === 'list') {
  for (const mode of Object.keys(MUTATIONS)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`NOT-APPLIED branch-title-wrap: unknown mode ${MUTATE}`);
  process.exit(2);
}
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let measured = 0;
try {
  for (const lang of ['en', 'zh-Hant']) {
    for (const theme of ['light', 'dark']) {
      for (const size of ['m', 'xl']) {
        const context = await browser.newContext({ viewport: { width: 320, height: 900 }, colorScheme: theme });
        try {
          await context.addCookies([
            { name: 'yomihon_lang', value: lang, url: BASE },
            { name: 'yomihon_theme', value: theme, url: BASE },
            { name: 'yomihon_textsize', value: size, url: BASE },
          ]);
          const page = await context.newPage();
          const proof = MUTATE ? await mutate(page, MUTATIONS[MUTATE]) : null;
          const response = await page.goto(BASE + PAGE);
          if (!response || response.status() !== 200) throw new Error(`course response=${response?.status()}`);
          await page.evaluate(() => document.fonts.ready);
          await arrived(page);
          const name = page.locator('.y-module__name').first();
          if (await name.count() !== 1) throw new Error('course has no branch name');
          if (proof) await proof();
          for (const authored of NAMES) {
            await name.evaluate((el, text) => { el.textContent = text; }, authored);
            const box = await name.evaluate((el) => {
              const root = document.documentElement;
              const label = el.closest('.y-module__label').getBoundingClientRect();
              const own = el.getBoundingClientRect();
              const range = document.createRange();
              range.selectNodeContents(el);
              const lines = [...range.getClientRects()].map((r) => ({ left: r.left, right: r.right, top: r.top, bottom: r.bottom }));
              return {
                text: el.textContent, lang: root.lang, theme: root.dataset.theme, size: root.dataset.textsize,
                width: root.clientWidth, scroll: root.scrollWidth,
                label: { left: label.left, right: label.right }, own: { left: own.left, right: own.right, top: own.top, bottom: own.bottom }, lines,
              };
            });
            console.log(`invoked: laid-out branch text ${lang}/${theme}/${size}`);
            if (box.text !== authored || box.lang !== lang || box.theme !== theme || box.size !== size || box.width !== 320 || box.lines.length === 0) {
              throw new Error(`unexpected reading context: ${JSON.stringify(box)}`);
            }
            const outside = box.lines.some((line) => line.left < Math.max(box.label.left, box.own.left) - 1 || line.right > Math.min(box.label.right, box.own.right, box.width) + 1 || line.top < box.own.top - 1 || line.bottom > box.own.bottom + 1);
            if (box.scroll > box.width || outside) {
              throw new LockFired(`FAIL branch-title-wrap: caught: ${SITE} ${lang}/${theme}/${size} document=${box.scroll}/${box.width}; author text leaves its visible branch: ${JSON.stringify(box)}`);
            }
            measured += 1;
          }
        } finally {
          await context.close();
        }
      }
    }
  }
  console.log(`PASS branch-title-wrap: ${measured} original/unbroken heading cases fit at 320px`);
} catch (err) {
  console.error(err.message || err);
  if (err instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && err instanceof LockFired) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
