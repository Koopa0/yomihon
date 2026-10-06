// Authored image fields survive rendering; loaded pictures keep their natural
// ratio inside the responsive reading column. Each fault rewrites one served declaration or image field, and the
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
  'restore-authored-height': 'ratio',
  'override-product-height': 'ratio',
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
const cssField = (source, selector, property, replacement, after = '') => {
  const rules = [...source.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter((m) => m[1].replace(/\/\*[\s\S]*?\*\//g, '').trim() === selector);
  if (rules.length !== 1) throw new NotApplied(`stylesheet selector ${selector} matched zero or several sites`);
  const rule = rules[0];
  const declarations = [...rule[2].matchAll(/([\w-]+)\s*:\s*([^;]+);/g)].filter((m) => m[1] === property);
  if (declarations.length !== 1) throw new NotApplied(`stylesheet property ${property} matched zero or several sites`);
  return source.replace(rule[0], rule[0].replace(declarations[0][0], `${property}: ${replacement};`) + after);
};

let browser;
let routeError;
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
        try {
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
          } else if (MUTATE === 'restore-authored-height') {
            if (body.includes('.y-prose img[height]')) throw new NotApplied('authored height rollback already exists');
            body = cssField(body, '.y-prose img', 'height', 'revert-layer', '\n@layer base {\n  .y-prose img[height] {\n    height: revert-layer;\n  }\n}');
            changed = true;
          } else if (MUTATE === 'override-product-height') {
            body = cssField(body, '.y-prose img', 'height', '600px');
            changed = true;
          } else if (MUTATE === 'uncap-the-image') {
            body = cssField(body, '.y-prose img', 'max-width', 'none');
            changed = true;
          }
          if (changed) applied++;
          await route.fulfill({ response, body });
        } catch (error) {
          routeError = error;
          await route.abort();
        }
      });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (response?.status() !== 200) throw new NotApplied('fixture page did not load');
      await arrived(page);
      await page.waitForFunction(() => [...document.querySelectorAll('.y-prose img')].every((i) => i.complete && i.naturalWidth > 0));
      const images = page.locator('.y-prose img');
      const expected = [
        ['120', null, 'pic.svg', 600, 400, 'width'],
        ['50', '20', 'pic.svg', 600, 400, 'height'],
        [null, null, 'some alt', 600, 400, 'alt'],
        ['120', null, 'alt', 600, 400, 'markdown'],
        ['1200', null, 'pic.svg', 600, 400, 'cap'],
        ['800', '600', 'ratio.svg', 800, 600, 'height'],
        ['300', null, 'pic.svg', 600, 400, 'width'],
        ['800', '600', 'pic.svg', 600, 400, 'height'],
      ];
      if (await images.count() !== expected.length) throw new NotApplied('fixture does not contain the complete eight-picture set');
      const column = await page.locator('.y-prose').boundingBox();
      const boxes = [];
      for (const [i, [expectedWidth, expectedHeight, alt, naturalWidth, naturalHeight, site]] of expected.entries()) {
        const image = images.nth(i);
        const natural = await image.evaluate((i) => [i.naturalWidth, i.naturalHeight]);
        if (natural[0] !== naturalWidth || natural[1] !== naturalHeight) throw new NotApplied(`image ${i} did not load the declared fixture resource`);
        assert(site, await image.getAttribute('width') === expectedWidth && await image.getAttribute('height') === expectedHeight && await image.getAttribute('alt') === alt, `image ${i} lost its authored fields`);
        boxes.push(await image.boundingBox());
      }
      // The large picture keeps an independent literal ratio even when the
      // column caps its width. Smaller mismatched hints exercise the browser's
      // loaded natural ratio, while their original HTML fields stay intact.
      assert('ratio', boxes[5] && Math.abs(boxes[5].height - boxes[5].width * 600 / 800) <= 1, `800x600 image lost its loaded ratio: ${boxes[5]?.width}x${boxes[5]?.height}`);
      for (const [i, [expectedWidth, , , naturalWidth, naturalHeight]] of expected.entries()) {
        const box = boxes[i];
        assert('cap', box && column && box.width <= column.width + 1, `image ${i} escaped the column`);
        if (expectedWidth && Number(expectedWidth) < column.width) assert(expected[i][5], Math.abs(box.width - Number(expectedWidth)) <= 1, `image ${i} lost its actual width`);
        assert('ratio', Math.abs(box.height - box.width * naturalHeight / naturalWidth) <= 1, `image ${i} lost its loaded natural ratio: ${box.width}x${box.height}`);
      }
      assert('cap', await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'images overflowed the page');
      if (errors.length) throw new Error(errors.join('\n'));
    } finally {
      await context.close();
    }
  }
  if (MUTATE) throw new NotApplied(applied ? 'mutation was not caught' : 'mutation was never invoked');
  console.log('PASS image-size: authored fields, loaded natural ratios, responsive cap; eight language/theme/width contexts');
} catch (error) {
  const failure = routeError || error;
  if (MUTATE && applied > 0 && failure instanceof LockFired && failure.site === MODES[MUTATE]) {
    console.error(`caught: image-size ${failure.message}`);
    console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    console.error(`${failure instanceof NotApplied ? 'NOT-APPLIED' : failure instanceof LockFired ? 'caught:' : 'FAIL'} image-size: ${failure.message}`);
    process.exitCode = failure instanceof LockFired && !MUTATE ? 1 : 2;
  }
} finally {
  await browser?.close();
}
