// CI-only composed reader tasks. Native shelf navigation is exercised with
// scripting disabled; mark writes use the real controls and private server.
import assert from 'node:assert/strict';
import { cp, mkdir, readFile, writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { join } from 'node:path';

const require = createRequire(new URL('../.github/package.json', import.meta.url));
const { chromium } = require('playwright-core');
const lesson = '/notes/Writing/lessons/japanese/L01.md';
const concept = '/notes/Concepts/japanese/%E3%81%AF.md#scheduling-details';
const words = [
  { lang: 'en', title: 'What you left open', mark: 'Not sure yet' },
  { lang: 'zh-Hant', title: '還沒說清楚的', mark: '還不確定' },
];

async function fixture(destination) {
  assert(destination, 'fixture destination is required');
  await cp(new URL('../.github/e2e/vault', import.meta.url), destination, { recursive: true });
  const path = join(destination, 'System/schemas/vault-schema.toml');
  let contract = await readFile(path, 'utf8');
  for (const [before, after] of [
    ['type = ["concept"', 'type = ["reflection", "concept"'],
    ['known = ["title"', 'known = ["updated", "title"'],
    ['[navigation]', '[navigation]\nanswer_type = "reflection"'],
    ['status = "draft"', 'status = "draft"\ninitial = true'],
    ['status = "ready"', 'status = "ready"\ninitial = false'],
  ]) {
    assert.equal(contract.split(before).length, 2, `one fixture declaration: ${before}`);
    contract = contract.replace(before, after);
  }
  contract += `\n[[lifecycle]]
status = "seedling"
applies_to = ["reflection"]
initial = true
from = []
owner = ["koopa"]
[[lifecycle]]
status = "ready"
applies_to = ["reflection"]
initial = false
from = ["seedling"]
owner = ["koopa"]
`;
  await writeFile(path, contract);
  await mkdir(join(destination, 'Outside'), { recursive: true });
  for (let i = 1; i <= 7; i++) {
    const dir = i === 7 ? 'Outside' : 'Notes';
    await writeFile(join(destination, dir, `Open${i}.md`), `---
title: Thought ${i}
type: reflection
domain: japanese
status: seedling
updated: 2026-01-0${i}
based_on: '[[Notes/Open source#Source section]]'
---
The reader's own sentence.
`);
  }
  await writeFile(join(destination, 'Notes/Open source.md'), '# Source\n\n## Source section\n\nSource words.\n');
}

async function visit(page, address) {
  const response = await page.goto(address, { waitUntil: 'load' });
  assert.equal(response.status(), 200, address);
}

async function ready(page, selector, value) {
  await page.waitForFunction(({ selector, value }) => {
    const button = document.querySelector(selector);
    return button && !button.disabled && button.getAttribute('aria-pressed') === String(value);
  }, { selector, value });
}

async function nativeShelf(browser, base, copy) {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: copy.lang, url: base }]);
    const page = await context.newPage();
    await visit(page, base);
    const block = page.locator('[data-home-block="open-thoughts"]');
    assert.equal(await block.locator('h2').textContent(), copy.title);
    assert.equal(await block.locator('[data-desk-item]').count(), 5, 'Home shows five');
    assert((await block.locator('[data-desk-item]').first().textContent()).includes('Thought 7'), 'outside-knowledge answer is newest');
    assert((await block.textContent()).includes('[[Notes/Open source#Source section]]'), 'authored source section is on the Home row');
    await block.locator('a.y-shelfall').click();
    assert.equal(new URL(page.url()).pathname, '/open-thoughts');
    const rows = page.locator('[data-index-row]');
    assert.equal(await rows.count(), 7, 'All exposes the entire set without JavaScript');
    assert.deepEqual(await rows.evaluateAll((items) => items.map((item) => item.getAttribute('href'))), [
      '/notes/Outside/Open7.md', ...[6, 5, 4, 3, 2, 1].map((i) => `/notes/Notes/Open${i}.md`),
    ], 'whole ordered answer set');
    await rows.last().click();
    assert.equal(new URL(page.url()).pathname, '/notes/Notes/Open1.md', 'oldest thought remains reachable');
    assert(await page.locator('.y-article').isVisible());
  } finally {
    await context.close();
  }
}

async function markedLocation(page, base, expected) {
  await visit(page, base);
  const block = page.locator('[data-home-block="open-thoughts"]');
  assert.equal(await block.locator('[data-desk-item]').count(), 5);
  assert.equal(await block.locator('[data-desk-item]').first().getAttribute('href'), expected, 'new mark leads to actual source location');
  await block.locator('a.y-shelfall').click();
  const rows = page.locator('[data-index-row]');
  assert.equal(await rows.count(), 8, 'seven thoughts plus one location');
  await rows.first().click();
  assert.equal(decodeURIComponent(new URL(page.url()).pathname + new URL(page.url()).hash), decodeURIComponent(expected));
  const anchor = decodeURIComponent(new URL(page.url()).hash.slice(1));
  assert.equal(await page.locator(`[id="${anchor}"]`).count(), 1, 'saved anchor is a real rendered location');
}

async function marksFromShelf(browser, base, copy) {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: copy.lang, url: base }]);
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    await visit(page, base + lesson);
    const slot = '.y-slotcard [data-uncertainty-control] button';
    await ready(page, slot, false);
    assert.equal(await page.locator(slot).textContent(), copy.mark);
    const anchor = await page.locator('.y-slotcard__abstract[id]').getAttribute('id');
    await page.locator(slot).click();
    await ready(page, slot, true);
    await markedLocation(page, base, `${lesson}#${anchor}`);
    await ready(page, slot, true);
    await page.locator(slot).click();
    await ready(page, slot, false);
    await visit(page, `${base}/open-thoughts`);
    assert.equal(await page.locator('[data-index-row]').count(), 7, 'clearing the saved slot removes only its row');

    await visit(page, base + lesson);
    await page.locator('[data-concept][href*="#"]').first().click();
    const popup = '[data-concept-sheet][open] [data-uncertainty-control] button';
    await ready(page, popup, false);
    await page.locator(popup).click();
    await ready(page, popup, true);
    await markedLocation(page, base, concept);
    await visit(page, base + lesson);
    await page.locator('[data-concept][href*="#"]').first().click();
    await ready(page, popup, true);
    await page.locator(popup).click();
    await ready(page, popup, false);
    await visit(page, `${base}/open-thoughts`);
    assert.equal(await page.locator('[data-index-row]').count(), 7, 'concept clear removes its source row');
  } finally {
    await context.close();
  }
}

async function statusLeavesShelf(browser, base) {
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    const page = await context.newPage();
    await visit(page, `${base}/notes/Notes/Open6.md`);
    const form = page.locator('form.y-statusform').filter({ has: page.locator('input[name="to"][value="ready"]') }).first();
    assert.equal(await form.count(), 1, 'existing native status form is available');
    const disclosure = form.locator('summary');
    if (await disclosure.count()) await disclosure.click();
    await form.locator('button[type="submit"]').click();
    await page.waitForLoadState('load');
    await visit(page, `${base}/open-thoughts`);
    assert.equal(await page.locator('[data-index-row]').count(), 6, 'real status writer removes a completed thought');
    assert.equal(await page.locator('[data-index-row][href="/notes/Notes/Open6.md"]').count(), 0);
  } finally {
    await context.close();
  }
}

async function probe() {
  const base = process.env.YOMIHON_BASE;
  assert(base, 'canonical server harness required');
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    for (const copy of words) {
      await nativeShelf(browser, base, copy);
      await marksFromShelf(browser, base, copy);
      console.log(`PASS open-thoughts ${copy.lang}: five/all native shelf, exact ordered set, authored section, slot and concept source return/clear`);
    }
    await statusLeavesShelf(browser, base);
    console.log('PASS open-thoughts: native status writer removes the row');
  } finally {
    await browser.close();
  }
}

if (process.env.CI !== 'true') throw new Error('open-thoughts is restricted to CI; local checks are paused');
if (process.argv[2] === '--fixture') await fixture(process.argv[3]);
else await probe();
