// A note's size explanation leads to its own finding, with the file name clear
// of the fixed header after arrival. Probe files live only in serve.sh's owned
// copy and are removed from the published report before the next probe runs.
import { randomUUID } from 'node:crypto';
import { mkdir, realpath, rm, writeFile } from 'node:fs/promises';
import { basename, dirname, join } from 'node:path';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const MODES = ['drop-finding-clearance', 'drop-finding-class'];
const SITE = 'finding-clears-the-header';
if (MUTATE === 'list') {
  for (const mode of MODES) console.log(mode);
  process.exit(0);
}
if (MUTATE && !MODES.includes(MUTATE)) {
  console.error(`source-limit: unknown mutation ${MUTATE}`);
  process.exit(2);
}
class Broken extends Error {}
class NotApplied extends Error {}
class LockFired extends Error {}
const arrived = (page) => page.waitForFunction(async () => {
  if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
  await Promise.all(document.getAnimations()
    .filter((animation) => animation.animationName === 'y-come-forward')
    .map((animation) => animation.finished.catch(() => {})));
  return true;
}, null, { timeout: 5000 });

let browser;
let control;
let folder;
const token = randomUUID();
const rel = `Notes/Source limit ${token}/A huge 字 # %.md`;
const noteURL = `${BASE}/notes/${encodeURIComponent(rel)}`;

// Both barriers read the served report, so the note is not asked for before the
// scanner has published it, and the next probe cannot inherit a finding from a
// removed file that the scanner has not yet published away. The wait is a loop
// here rather than waitForFunction: that settles on the first truthy return,
// and an async predicate returns a pending promise, which would let either
// barrier pass on its first look.
const published = async (present) => {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const settled = await control.page.evaluate(async ({ address, marker, expected }) => {
      const response = await fetch(address, { cache: 'no-store' });
      return response.ok && (await response.text()).includes(marker) === expected;
    }, { address: `${BASE}/health?page=all`, marker: token, expected: present });
    if (settled) return;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Broken(`the health report ${present ? 'never named' : 'still names'} the probe's files after 20s`);
};

try {
  const root = await realpath(process.env.YOMIHON_FIXTURE_ROOT || '');
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) {
    throw new Broken('fixture root is not the private copy owned by serve.sh');
  }
  const notes = join(root, 'Notes');
  if (await realpath(notes) !== notes) throw new Broken('fixture Notes directory is a link outside the owned copy');
  const ownedFolder = join(notes, `Source limit ${token}`);
  await mkdir(ownedFolder);
  folder = ownedFolder;
  await writeFile(join(folder, 'A huge 字 # %.md'), 'x'.repeat((1 << 20) + 1));
  // Later findings give the native fragment room to align at its target. A
  // last row clamped by the end of a short report would hide a lost margin.
  for (let index = 0; index < 30; index += 1) {
    await writeFile(join(folder, `Z${index}.md`), `# Probe ${index}\n\n[[Missing ${token} ${index}]]\n`);
  }
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  control = { context: await browser.newContext() };
  control.page = await control.context.newPage();
  await control.page.goto(BASE, { waitUntil: 'load' });
  await published(true);
  for (const width of [1280, 390]) for (const lang of ['zh-Hant', 'en']) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height: 900 } });
    try {
      await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }, { name: 'yomihon_theme', value: theme, url: BASE }]);
      let matches = -1;
      if (MUTATE === 'drop-finding-clearance') await context.route('**/static/app.css{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        const needle = /\.y-findings__target\s*\{[^}]*\}/gu;
        matches = [...original.matchAll(needle)].length;
        await route.fulfill({ response, body: matches === 1 ? original.replace(needle, '') : original });
      });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const response = await page.goto(noteURL, { waitUntil: 'load' });
      if (response?.status() !== 200) throw new Broken(`note returned ${response?.status()}, want 200`);
      await arrived(page);
      const links = page.locator('.y-fileinfo a[href^="/health?page=all#"]');
      if (await links.count() !== 1) throw new Broken('the size explanation has no single finding link');
      const href = await links.getAttribute('href');
      const id = new URL(href, BASE).hash.slice(1);
      if (MUTATE === 'drop-finding-class') await context.route((url) => url.pathname === '/health', async (route) => {
        const answer = await route.fetch();
        const original = await answer.text();
        const needle = new RegExp(`<tr\\b[^>]*\\bid="${id}"[^>]*>`, 'gu');
        const rows = [...original.matchAll(needle)];
        const row = rows[0]?.[0] || '';
        matches = rows.length === 1 ? row.split('class="y-findings__target"').length - 1 : rows.length;
        await route.fulfill({ response: answer, body: matches === 1 ? original.replace(row, row.replace('class="y-findings__target"', '')) : original });
      });
      await links.click();
      await page.waitForURL(BASE + href);
      await arrived(page);
      await page.evaluate(() => document.fonts.ready);
      if (MUTATE && matches !== 1) throw new NotApplied(`mutation matched ${matches} sites, want exactly 1`);
      const row = page.locator(`[id="${id}"]`);
      if (await row.count() !== 1 || !(await row.innerText()).includes(rel)) throw new Broken('fragment does not name the note’s own finding');
      const box = await row.boundingBox();
      const header = await page.locator('.y-header').boundingBox();
      console.log(`invocation-hit source-limit ${width} ${lang} ${theme}: native finding link arrived, row=${JSON.stringify(box)}, header=${JSON.stringify(header)}`);
      if (!box || !header) throw new Broken('finding or header has no rendered box');
      if (box.y < header.y + header.height || box.y >= 900) throw new LockFired(`finding starts at ${box.y}, header ends at ${header.y + header.height}`);
      if (errors.length) throw new Broken(`page errors: ${errors.join('; ')}`);
      console.log(`PASS source-limit ${width} ${lang} ${theme}: ${SITE}`);
    } finally {
      await context.close();
    }
  }
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.log(`caught: source-limit ${SITE}`);
    if (MUTATE) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 2;
  }
} finally {
  try {
    if (folder) {
      await rm(folder, { recursive: true });
      if (control) await published(false);
    }
  } catch (error) {
    console.error(`source-limit cleanup: ${error.message}`);
    process.exitCode = 2;
  } finally {
    try {
      await browser?.close();
    } catch (error) {
      console.error(`source-limit browser cleanup: ${error.message}`);
      process.exitCode = 2;
    }
  }
}
