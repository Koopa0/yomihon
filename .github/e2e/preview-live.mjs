// A reopened hover card asks the live note again, including a target that was
// removed. Only the disposable vault copy owned by serve.sh is changed; the
// original fixture and the reader's vault are never the mutation stimulus.
import { randomUUID } from 'node:crypto';
import { realpath, unlink, writeFile } from 'node:fs/promises';
import { basename, dirname, join } from 'node:path';
import { setTimeout as pause } from 'node:timers/promises';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const MODES = {
  'hold-the-first-body-after-update': 'updated',
  'hold-the-first-body-after-delete': 'deleted',
  'keep-the-read-running-after-escape': 'canceled',
};
const SITES = ['updated', 'deleted', 'canceled'];
if (new Set(Object.values(MODES)).size !== SITES.length || SITES.some((site) => !Object.values(MODES).includes(site))) {
  console.error('preview-live: every assertion needs a mutation');
  process.exit(2);
}
if (MUTATE === 'list') {
  for (const mode of Object.keys(MODES)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MODES, MUTATE)) {
  console.error(`preview-live: unknown mutation ${MUTATE}`);
  process.exit(2);
}

class Broken extends Error {}
class NotApplied extends Error {}
class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}

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

const rewriteHeldBody = async (context) => {
  let matches = -1;
  await context.route('**/static/preview.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const needle = `  async function excerpt(url, signal) {
    const response = await fetch(url, { headers: { Accept: 'text/html' }, signal });
    const parsed = new DOMParser().parseFromString(await response.text(), 'text/html');
    const body = parsed.querySelector('[data-preview-body]');
    if (!body) throw new Error(\`preview response for \${url.pathname} carries no body\`);
    return body;
  }`;
    matches = original.split(needle).length - 1;
    const replacement = `  const excerpts = new Map();
  async function excerpt(url, signal) {
    const held = excerpts.get(url.href);
    if (held !== undefined) return held;
    const response = await fetch(url, { headers: { Accept: 'text/html' }, signal });
    const parsed = new DOMParser().parseFromString(await response.text(), 'text/html');
    const body = parsed.querySelector('[data-preview-body]');
    if (!body) throw new Error(\`preview response for \${url.pathname} carries no body\`);
    excerpts.set(url.href, body);
    return body;
  }`;
    await route.fulfill({ response, body: matches === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (matches !== 1) throw new NotApplied(`preview excerpt mutation matched ${matches} sites, want 1`);
  };
};

const rewriteAbort = async (context) => {
  let matches = -1;
  await context.route('**/static/preview.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const needle = '    controller?.abort();';
    matches = original.split(needle).length - 1;
    await route.fulfill({ response, body: matches === 1 ? original.replace(needle, '') : original });
  });
  return () => {
    if (matches !== 1) throw new NotApplied(`preview abort mutation matched ${matches} sites, want 1`);
  };
};

const published = async (context, path, status, marker) => {
  const deadline = Date.now() + 12000;
  while (Date.now() < deadline) {
    const response = await context.request.get(BASE + path);
    const body = await response.text();
    if (response.status() === status && body.includes(marker)) return;
    await pause(100);
  }
  throw new Broken(`server never published ${status} containing ${marker} at ${path}`);
};

const unpublished = async (path) => {
  const deadline = Date.now() + 12000;
  while (Date.now() < deadline) {
    const response = await fetch(BASE + path);
    await response.text();
    if (response.status === 404) return;
    await pause(100);
  }
  throw new Broken(`server still publishes ${path} after cleanup`);
};

const cardText = (page) => page.locator('[data-preview-card]').innerText();
const open = async (page, link) => {
  await link.hover();
  await page.waitForFunction(() => document.querySelector('[data-preview-card]')?.matches(':popover-open'), null, { timeout: 4000 });
};
const dismiss = async (page) => {
  await page.mouse.move(1, 1);
  await page.keyboard.press('Escape');
  await page.waitForFunction(() => !document.querySelector('[data-preview-card]')?.matches(':popover-open'));
  await page.waitForFunction(() => document.querySelectorAll('[data-preview-open]').length === 0);
};

const cancellation = async (context, page, link, path, proof) => {
  let release;
  const held = new Promise((resolve) => { release = resolve; });
  let started = false;
  let deliveryError;
  let delivered;
  const finished = new Promise((resolve) => { delivered = resolve; });
  const routeHandler = async (route) => {
    try {
      const response = await route.fetch();
      if (response.status() !== 200) throw new Broken(`the delayed excerpt returned ${response.status()}, want 200`);
      started = true;
      await held;
      await route.fulfill({ response });
    } catch (error) {
      deliveryError = error;
    } finally {
      delivered();
    }
  };
  await context.route(BASE + path, routeHandler);
  await page.evaluate((target) => {
    const nativeFetch = window.fetch;
    window.__previewCancel = { started: 0, aborted: 0, settled: 0 };
    window.fetch = (...args) => {
      if (new URL(args[0], location.href).pathname !== target) return nativeFetch.apply(window, args);
      const state = window.__previewCancel;
      state.started += 1;
      args[1].signal.addEventListener('abort', () => { state.aborted += 1; }, { once: true });
      return nativeFetch.apply(window, args).then(
        (response) => { state.settled += 1; return response; },
        (error) => { state.settled += 1; throw error; },
      );
    };
  }, path);
  try {
    await link.hover();
    await page.waitForFunction(() => window.__previewCancel.started === 1, null, { timeout: 4000 });
    const deadline = Date.now() + 4000;
    while (!started && Date.now() < deadline) await pause(20);
    if (!started) throw new Broken('the delayed excerpt never reached its server response');
    console.log('invocation-hit preview-live canceled: real excerpt response held before Escape');
    await page.keyboard.press('Escape');
    proof();
    const aborted = await page.evaluate(() => window.__previewCancel.aborted);
    if (aborted !== 1) throw new LockFired('canceled', `Escape emitted ${aborted} abort signals for an in-flight excerpt, want 1`);
    release();
    await finished;
    if (deliveryError) throw new Broken(`delayed excerpt delivery failed: ${deliveryError.message}`);
    await page.waitForFunction(() => window.__previewCancel.settled === 1);
    if (await page.locator('[data-preview-card]').evaluate((card) => card.matches(':popover-open'))) {
      throw new LockFired('canceled', 'the canceled excerpt reopened the card');
    }
    if (await page.locator('[data-preview-open]').count() !== 0) throw new LockFired('canceled', 'the canceled excerpt left an anchor');
    console.log('PASS preview-live: canceled in-flight read remains dismissed');
  } finally {
    release();
    await context.unroute(BASE + path, routeHandler);
  }
};

let browser;
const created = [];
try {
  const supplied = process.env.YOMIHON_FIXTURE_ROOT;
  if (!supplied) throw new Broken('serve.sh did not expose its disposable fixture root');
  const root = await realpath(supplied);
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) {
    throw new Broken('the fixture root is not a serve.sh disposable vault copy');
  }
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  for (const width of [1280, 390]) {
    for (const lang of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        for (const site of SITES.filter((candidate) => candidate !== 'canceled')) {
          const id = randomUUID();
          const host = `preview-live-host-${id}`;
          const target = `preview-live-target-${id}`;
          const hostFile = join(root, 'Notes', `${host}.md`);
          const targetFile = join(root, 'Notes', `${target}.md`);
          const before = `PREVIEW_BEFORE_${id}`;
          const after = `PREVIEW_AFTER_${id}`;
          const targetPath = `/preview/Notes/${target}.md`;
          const source = (marker) => `# Preview target\n\n${marker}\n`;
          await writeFile(hostFile, `# Preview host\n\n[[${target}|Preview target]]\n`);
          created.push(hostFile);
          await writeFile(targetFile, source(before));
          created.push(targetFile);
          const context = await browser.newContext({ viewport: { width, height: 800 }, colorScheme: theme });
          try {
            await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }, { name: 'yomihon_theme', value: theme, url: BASE }]);
            const proof = MODES[MUTATE] === 'canceled' ? await rewriteAbort(context)
              : MUTATE && MODES[MUTATE] === site ? await rewriteHeldBody(context) : () => {};
            await published(context, targetPath, 200, before);
            const page = await context.newPage();
            const response = await page.goto(`${BASE}/notes/Notes/${host}.md`, { waitUntil: 'networkidle' });
            if (response?.status() !== 200) throw new Broken(`host returned ${response?.status()}`);
            await arrived(page);
            const link = page.locator('.y-prose a.wikilink').filter({ hasText: 'Preview target' });
            if (await link.count() !== 1) throw new Broken('host lacks its unique preview link');
            await open(page, link);
            if (!(await cardText(page)).includes(before)) throw new Broken('first card lacks the initial server marker');
            await dismiss(page);
            const refusal = lang === 'en' ? 'There is no note to preview at that address.' : '這個位置沒有可以預覽的筆記。';
            if (site === 'updated') {
              await writeFile(targetFile, source(after));
              await published(context, targetPath, 200, after);
            } else {
              await unlink(targetFile);
              await published(context, targetPath, 404, refusal);
            }
            console.log(`invocation-hit preview-live ${site} ${width} ${lang} ${theme}: server publication observed`);
            proof();
            await open(page, link);
            const text = await cardText(page);
            const expected = site === 'updated' ? after : refusal;
            if (!text.includes(expected) || text.includes(before)) {
              throw new LockFired(site, `reopened ${site} card still reads ${JSON.stringify(text)}, want ${expected} and no first marker`);
            }
            await dismiss(page);
            if (width === 1280 && lang === 'zh-Hant' && theme === 'light' && site === 'updated' && (!MUTATE || MODES[MUTATE] === 'canceled')) {
              await cancellation(context, page, link, targetPath, proof);
            }
            await link.click();
            await page.waitForURL(`${BASE}/notes/Notes/${target}.md`);
            console.log(`PASS preview-live: ${site} ${width} ${lang} ${theme}`);
          } finally {
            await context.close();
          }
        }
      }
    }
  }
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.log(`caught: preview-live ${error.site}`);
    if (MUTATE && MODES[MUTATE] === error.site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 2;
  }
} finally {
  try {
    await browser?.close();
  } catch (error) {
    console.error(`preview-live browser cleanup: ${error.message}`);
    process.exitCode = 2;
  }
  for (const file of created) {
    try {
      await unlink(file);
    } catch (error) {
      if (error.code !== 'ENOENT') {
        console.error(`preview-live cleanup: ${error.message}`);
        process.exitCode = 2;
      }
    }
  }
  for (const file of created) {
    try {
      await unpublished(`/notes/Notes/${basename(file)}`);
    } catch (error) {
      console.error(`preview-live cleanup: ${error.message}`);
      process.exitCode = 2;
    }
  }
}
