// CI-only composition checks using the same installed Chrome and fixture
// server as the repository's existing browser probes. No external editor is
// launched, and the no-script task proves selectable text, not OS clipboard use.
import assert from 'node:assert/strict';
import { cp, readFile, writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { join } from 'node:path';

const require = createRequire(new URL('../.github/package.json', import.meta.url));
const { chromium } = require('playwright-core');
const sourcePath = '/notes/Notes/Reading%20source.md';
const lessonPath = '/notes/Writing/lessons/japanese/L01.md';
const lessonRel = 'Writing/lessons/japanese/L01.md';
const conceptRel = 'Concepts/japanese/は.md';

async function prepareFixture(destination) {
  assert(destination, 'fixture destination must be explicit');
  await cp(new URL('../.github/e2e/vault', import.meta.url), destination, { recursive: true });
  const contractPath = join(destination, 'System/schemas/vault-schema.toml');
  const contract = await readFile(contractPath, 'utf8');
  assert.equal(contract.split('[navigation]').length, 2, 'one navigation edit site required');
  await writeFile(contractPath, contract.replace('[navigation]', '[navigation]\nanswer_type = "lesson"'));
  await writeFile(join(destination, 'Notes/Reading source.md'), '# Reading source\n\n## Chapter one\n\nDo not copy this prompt.\n\n## Chapter two\n\nA quotation. ^quote\n');
  await writeFile(join(destination, 'Notes/Thought A.md'), `---
based_on:
  - '[[Notes/Reading source#Chapter one]]'
  - '[[Notes/Reading source#Chapter two]]'
  - '[[Notes/Reading source#Chapter one|repeated]]'
  - '[[Notes/Reading source#Missing chapter]]'
---
My first thought.
`);
  await writeFile(join(destination, 'Notes/Thought B.md'), `---
based_on:
  - '[[Notes/Reading source]]'
  - '[[Notes/Reading source#^quote]]'
---
My second thought.
`);
}

async function visit(page, address) {
  const response = await page.goto(address, { waitUntil: 'load' });
  assert.equal(response.status(), 200, `reading route ${address}`);
}

async function records(page, base) {
  const response = await page.request.get(`${base}/uncertainties`);
  assert.equal(response.status(), 200, 'marks are readable');
  return response.json();
}

async function enabled(page, selector) {
  await page.locator(selector).waitFor({ state: 'visible' });
  await page.waitForFunction((target) => {
    const button = document.querySelector(target);
    return button && !button.disabled;
  }, selector);
}

async function pressed(page, selector, wanted) {
  await page.waitForFunction(({ target, value }) => {
    const button = document.querySelector(target);
    return button && !button.disabled && button.getAttribute('aria-pressed') === value;
  }, { target: selector, value: String(wanted) });
}

async function copyableWithoutScript(browser, base, words) {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1600, height: 900 } });
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: words.lang, url: base }]);
    const page = await context.newPage();
    await visit(page, base + sourcePath);
    const door = page.locator('.y-article a[href="/thought/Notes/Reading%20source.md"]');
    assert.equal(await door.textContent(), words.door, 'ordinary document has a translated thought door');
    await door.click();
    const text = page.locator('[data-thought-markdown]');
    assert(await text.isVisible(), 'Markdown remains visible without scripts');
    assert.equal(await text.getAttribute('readonly'), '', 'stub is selectable read-only text');
    const expected = '---\ntype: "lesson"\nstatus: "draft"\nbased_on: "[[Notes/Reading source]]"\n---\n';
    assert.equal(await text.inputValue(), expected, 'no prompt, answer template, or invented metadata');
    await text.focus();
    await text.press('ControlOrMeta+A');
    assert.equal(await text.evaluate((element) => element.selectionEnd - element.selectionStart), expected.length, 'manual copy can select every byte');
    assert.equal(await page.locator('[data-thought-copy]').isVisible(), false, 'script-only copy button is not a dead control');
    const editor = new URL(await page.locator('a[href^="obsidian://new?"]').getAttribute('href'));
    assert.deepEqual([...editor.searchParams.keys()].sort(), ['content', 'path'], 'editor URI carries no overwrite or append flag');
    assert.equal(editor.searchParams.get('content'), expected, 'editor receives the same selectable text');
    await visit(page, `${base}/thought/Notes/Reading%20source.md?section=chapter-one`);
    assert((await text.inputValue()).includes('[[Notes/Reading source#chapter-one]]'), 'selected section stays in the declaration');
    await visit(page, base + lessonPath);
    assert.equal(await page.locator('.y-article a[href="/thought/Writing/lessons/japanese/L01.md"]').textContent(), words.door, 'lesson uses the same door');
    assert.equal(await page.locator('[data-uncertainty-control]').count(), 0, 'no inert mark controls without scripting');
  } finally {
    await context.close();
  }
}

async function reverseLocations(page, base) {
  await visit(page, base + sourcePath);
  const block = page.locator('.y-rail-right [data-declared-by]');
  const rows = block.locator(':scope > ul > li');
  assert.equal(await rows.count(), 2, 'two declaring notes, not a fragment count');
  assert.equal(await block.locator('.ui-navitem__count').textContent(), '2');
  const first = rows.nth(0);
  assert.equal(await first.locator(':scope > a').getAttribute('href'), '/notes/Notes/Thought%20A.md', 'thought opens separately from its source locations');
  assert.deepEqual(await first.locator('ul a').evaluateAll((links) => links.map((link) => link.getAttribute('href'))), ['#chapter-one', '#chapter-two']);
  assert.equal(await first.locator('ul span').textContent(), '#Missing chapter', 'missing authored location is visible without a false link');
  const second = rows.nth(1);
  assert.equal(await second.locator(':scope > a').getAttribute('href'), '/notes/Notes/Thought%20B.md');
  assert.deepEqual(await second.locator('ul a').evaluateAll((links) => links.map((link) => decodeURIComponent(link.getAttribute('href')))), ['#reading-source', '#^quote']);
  await first.locator('ul a').first().click();
  assert.equal(new URL(page.url()).hash, '#chapter-one', 'source location stays on the source page');
  assert.equal(await page.locator('#chapter-one').count(), 1, 'source fragment names one real rendered anchor');
}

async function uncertaintyRoundTrip(page, base, words) {
  await visit(page, base + lessonPath);
  const slot = '.y-slotcard [data-uncertainty-control] button';
  await enabled(page, slot);
  assert.equal(await page.locator(slot).textContent(), words.mark, 'slot control speaks the chosen interface language');
  const anchor = await page.locator('.y-slotcard__abstract[id]').getAttribute('id');
  await page.locator(slot).click();
  await pressed(page, slot, true);
  let held = await records(page, base);
  assert.equal(held.length, 1);
  assert.equal(held[0].path, lessonRel);
  assert.equal(held[0].anchor, anchor);
  assert.deepEqual(Object.keys(held[0]).sort(), ['anchor', 'path', 'time'], 'only the ruled location fields are saved');
  assert(Number.isFinite(Date.parse(held[0].time)), 'saved mark has a time');
  const savedTime = held[0].time;
  await page.reload({ waitUntil: 'load' });
  await pressed(page, slot, true);
  assert.equal((await records(page, base))[0].time, savedTime, 'reload reads the saved record instead of adding another');
  await page.locator(slot).click();
  await pressed(page, slot, false);
  assert.deepEqual(await records(page, base), [], 'unmark removes the persisted slot location');

  const trigger = page.locator('[data-concept][href*="#"]').first();
  await trigger.click();
  const concept = '[data-concept-sheet][open] [data-uncertainty-control] button';
  await enabled(page, concept);
  assert.equal(await page.locator(concept).textContent(), words.mark, 'concept control is translated');
  await page.locator(concept).click();
  await pressed(page, concept, true);
  held = await records(page, base);
  assert.equal(held.length, 1);
  assert.equal(held[0].path, conceptRel, 'concept mark names the concept source, not the enclosing lesson');
  assert.equal(held[0].anchor, 'scheduling-details', 'concept mark retains the clicked source section');
  await page.reload({ waitUntil: 'load' });
  await page.locator('[data-concept][href*="#"]').first().click();
  await pressed(page, concept, true);
  await page.locator(concept).click();
  await pressed(page, concept, false);
  assert.deepEqual(await records(page, base), [], 'concept unmark survives the next server read');
}

async function probe() {
  const base = process.env.YOMIHON_BASE;
  assert(base, 'server wrapper must supply YOMIHON_BASE');
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    for (const words of [
      { lang: 'en', door: 'Leave a thought', mark: 'Not sure yet' },
      { lang: 'zh-Hant', door: '留下自己的想法', mark: '還不確定' },
    ]) {
      await copyableWithoutScript(browser, base, words);
      const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
      try {
        await context.addCookies([{ name: 'yomihon_lang', value: words.lang, url: base }]);
        const page = await context.newPage();
        page.setDefaultTimeout(10000);
        await reverseLocations(page, base);
        await uncertaintyRoundTrip(page, base, words);
        console.log(`PASS reading-thoughts ${words.lang}: no-script stub, reverse source locations, durable slot and concept toggles`);
      } finally {
        await context.close();
      }
    }
  } finally {
    await browser.close();
  }
}

if (process.env.CI !== 'true') throw new Error('reading-thoughts is restricted to CI; local checks are paused');
if (process.argv[2] === '--fixture') await prepareFixture(process.argv[3]);
else await probe();
