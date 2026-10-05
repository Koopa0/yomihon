// Authored image dimensions survive rendering and the responsive reading
// column. Each fault rewrites one served declaration or image field, and the
// oracle reads the real loaded image after page arrival has settled.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/image-sizes.md';
const MUTATE = process.env.MUTATE || '';
const MODES = {
  'drop-wiki-width': 'width',
  'drop-wiki-height': 'height',
  'drop-wiki-alt': 'alt',
  'drop-markdown-width': 'markdown',
  'override-authored-height': 'height',
  'override-product-height': 'height',
  'uncap-the-image': 'cap',
};
if (MUTATE === 'list') {
  for (const mode of Object.keys(MODES)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MODES, MUTATE)) {
  console.error('NOT-APPLIED image-size: unknown mode');
  process.exit(2);
}
class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class NotApplied extends Error {
  constructor(message) {
    super(message);
    console.error(`NOT-APPLIED image-size: ${message}`);
  }
}
const assert = (site, condition, message) => {
  if (!condition) throw new LockFired(site, message);
};
const arrived = async (page) => {
  await page.waitForFunction(
    async () => {
      if (![...document.styleSheets].some((s) => (s.href || '').includes('/static/app.css'))) return false;
      await Promise.all(
        document.getAnimations().filter((a) => a.animationName === 'y-come-forward').map((a) => a.finished.catch(() => {})),
      );
      return true;
    },
    null,
    { timeout: 3000 },
  );
};
const one = (source, needle, replacement) => {
  if (source.split(needle).length !== 2) throw new NotApplied('image field matched zero or several sites');
  return source.replace(needle, replacement);
};
const cssField = (source, selector, property, replacement) => {
  const rules = [...source.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter((m) => m[1].replace(/\/\*[\s\S]*?\*\//g, '').trim() === selector);
  if (rules.length !== 1) throw new NotApplied(`stylesheet selector ${selector} matched zero or several sites`);
  const rule = rules[0];
  const declarations = [...rule[2].matchAll(/([\w-]+)\s*:\s*([^;]+);/g)].filter((m) => m[1] === property);
  if (declarations.length !== 1) throw new NotApplied(`stylesheet property ${property} matched zero or several sites`);
  return source.replace(rule[0], rule[0].replace(declarations[0][0], `${property}: ${replacement};`));
};

let browser;
let applied = 0;
try {
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  for (const width of [1280, 390]) for (const lang of ['zh-Hant', 'en']) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height: 900 } });
    try {
      await context.addCookies([
        { name: 'yomihon_lang', value: lang, url: BASE },
        { name: 'yomihon_theme', value: theme, url: BASE },
      ]);
      if (MUTATE) await context.route('**/*', async (route) => {
        const path = new URL(route.request().url()).pathname;
        if (path !== PAGE && path !== '/static/app.css') return route.continue();
        const response = await route.fetch();
        let body = await response.text();
        let changed = false;
        if (path === PAGE) {
          const edits = {
            'drop-wiki-width': ['alt="pic.svg" width="120"', 'alt="pic.svg"'],
            'drop-wiki-height': ['alt="pic.svg" width="50" height="20"', 'alt="pic.svg" width="50"'],
            'drop-wiki-alt': ['alt="some alt"', 'alt="pic.svg"'],
            'drop-markdown-width': ['alt="alt" width="120"', 'alt="alt|120"'],
          };
          if (Object.hasOwn(edits, MUTATE)) {
            body = one(body, ...edits[MUTATE]);
            changed = true;
          }
        } else if (MUTATE === 'override-authored-height') {
          body = cssField(body, '.y-prose img[height]', 'height', 'auto');
          changed = true;
        } else if (MUTATE === 'override-product-height') {
          body = cssField(body, '.y-prose img', 'height', 'auto');
          changed = true;
        } else if (MUTATE === 'uncap-the-image') {
          body = cssField(body, '.y-prose img', 'max-width', 'none');
          changed = true;
        }
        if (changed) applied++;
        await route.fulfill({ response, body });
      });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (response?.status() !== 200) throw new NotApplied('fixture page did not load');
      await arrived(page);
      await page.waitForFunction(() => [...document.querySelectorAll('.y-prose img')].every((i) => i.complete && i.naturalWidth === 600));
      const images = page.locator('.y-prose img');
      if (await images.count() !== 5) throw new NotApplied('fixture does not contain five loaded pictures');
      const column = await page.locator('.y-prose').boundingBox();
      for (const [i, expectedWidth, expectedHeight, alt, site] of [
        [0, '120', null, 'pic.svg', 'width'],
        [1, '50', '20', 'pic.svg', 'height'],
        [2, null, null, 'some alt', 'alt'],
        [3, '120', null, 'alt', 'markdown'],
        [4, '1200', null, 'pic.svg', 'cap'],
      ]) {
        const image = images.nth(i);
        const box = await image.boundingBox();
        assert(site, await image.getAttribute('width') === expectedWidth && await image.getAttribute('height') === expectedHeight && await image.getAttribute('alt') === alt, `image ${i} lost its authored fields`);
        assert(site, box && column && box.width <= column.width + 1, `image ${i} escaped the column`);
        if (expectedWidth && Number(expectedWidth) < column.width) assert(site, Math.abs(box.width - Number(expectedWidth)) < 1, `image ${i} lost its actual width`);
        if (expectedHeight) assert(site, Math.abs(box.height - Number(expectedHeight)) < 1, `image ${i} lost its actual height: ${box.height}`);
      }
      assert('cap', await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'images overflowed the page');
      if (errors.length) throw new Error(errors.join('\n'));
    } finally {
      await context.close();
    }
  }
  if (MUTATE) throw new NotApplied(applied ? 'mutation was not caught' : 'mutation was never invoked');
  console.log('PASS image-size: dimensions, alt, responsive cap; eight language/theme/width contexts');
} catch (error) {
  if (MUTATE && applied > 0 && error instanceof LockFired && error.site === MODES[MUTATE]) {
    console.error(`FAIL image-size: ${error.message}`);
    console.error(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    console.error(`${error instanceof NotApplied ? 'NOT-APPLIED' : 'FAIL'} image-size: ${error.message}`);
    process.exitCode = error instanceof LockFired && !MUTATE ? 1 : 2;
  }
} finally {
  await browser?.close();
}
