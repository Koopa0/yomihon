// Inspect the browser-owned clear node: getComputedStyle on the unsupported
// search pseudo-element returns the input's style rather than the clear mark.
import { chromium } from 'playwright-core';
import { arrived } from './support/arrival.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const MODE = 'drop-clear-palette';
if (MUTATE === 'list') { console.log(MODE); process.exit(0); }
if (MUTATE && MUTATE !== MODE) process.exit(2);

const find = (node, predicate) => {
  if (predicate(node)) return node;
  for (const child of [...node.children || [], ...node.shadowRoots || []]) {
    const found = find(child, predicate);
    if (found) return found;
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await context.addCookies([{ name: 'yomihon_theme', value: theme, url: BASE }]);
    const page = await context.newPage();
    let applied = 0;
    if (MUTATE) await page.route('**/static/app.css{,?*}', async route => {
      const response = await route.fetch();
      const original = await response.text();
      const rule = /\.y-homesearch input::-webkit-search-cancel-button,\s*\.y-searchdialog__input::-webkit-search-cancel-button,\s*\.y-searchpage__form input::-webkit-search-cancel-button\s*\{[^}]*\}/g;
      const matches = [...original.matchAll(rule)];
      if (matches.length === 1) applied++;
      await route.fulfill({ response, body: matches.length === 1 ? original.replace(rule, '') : original });
    });
    const cdp = await context.newCDPSession(page);
    await cdp.send('DOM.enable');
    await cdp.send('CSS.enable');
    for (const [path, selector] of [['/', '.y-homesearch input'], ['/', '.y-searchdialog__input'], ['/search?q=alpha', '.y-searchpage__form input']]) {
      await page.goto(BASE + path, { waitUntil: 'load' });
      await arrived(page);
      if (selector === '.y-searchdialog__input') await page.locator('.y-searchbtn').click();
      const input = page.locator(selector);
      await input.fill('alpha');
      if (MUTATE && applied === 0) {
        console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
        process.exitCode = 2;
        throw new Error('NOT-APPLIED search-clear: clear rule did not match exactly once');
      }
      const { root } = await cdp.send('DOM.getDocument', { depth: -1, pierce: true });
      const { nodeId } = await cdp.send('DOM.querySelector', { nodeId: root.nodeId, selector });
      const owner = find(root, node => node.nodeId === nodeId);
      const clear = owner && find(owner, node => node.attributes?.includes('-webkit-search-cancel-button'));
      if (!clear) throw new Error(`BROKEN search-clear: ${selector} has no browser clear node`);
      const { computedStyle } = await cdp.send('CSS.getComputedStyleForNode', { nodeId: clear.nodeId });
      const style = Object.fromEntries(computedStyle.map(property => [property.name, property.value]));
      const ink = await input.evaluate(element => getComputedStyle(element).getPropertyValue('--fg-subtle').trim());
      // Resolve the token through a real style property, as its declaration
      // may use a colour syntax different from the computed browser output.
      const colour = await input.evaluate((element, value) => {
        const sample = document.createElement('span');
        sample.style.color = value;
        element.parentElement.append(sample);
        const resolved = getComputedStyle(sample).color;
        sample.remove();
        return resolved;
      }, ink);
      if (style.appearance !== 'none' || style.width !== '14px' || style.height !== '14px' || (style['background-image'].match(/linear-gradient/g) || []).length !== 2 || !style['background-image'].includes(colour)) {
        if (MUTATE) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
        throw new Error(`FAIL search-clear: caught: ${theme} ${selector} clear appearance=${style.appearance}, background=${style['background-image']}, size=${style.width}x${style.height}; want palette cross in ${colour}`);
      }
      const { model } = await cdp.send('DOM.getBoxModel', { nodeId: clear.nodeId });
      const [x0, y0, x1, , , y1] = model.border;
      await page.mouse.click((x0 + x1) / 2, (y0 + y1) / 2);
      if (await input.inputValue() !== '') throw new Error(`FAIL search-clear: ${selector} no longer clears when pressed`);
    }
    await context.close();
  }
  console.log('PASS search-clear: every native clear mark uses the palette in both themes and still clears its field');
} catch (error) {
  console.error(error.message);
  process.exitCode ||= 1;
} finally { await browser.close(); }
