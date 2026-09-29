// CI-only composed acceptance for #655. Runs the real compiled reader and
// system Chrome; two rebuilt production mutations must fail their own locks.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { cpSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright-core';

const SCRIPT = fileURLToPath(import.meta.url);
const ROOT = resolve(fileURLToPath(new URL('..', import.meta.url)));
const ARTIFACTS = resolve(process.env.HOME_ARRIVAL_ARTIFACTS || 'home-arrival-artifacts');
const SOURCE = join(ROOT, 'internal/ui/pages/modeindex.go');
const MODES = ['paths', 'maps', 'reports', 'folders'];
const READING = '# Reading club\n\nBring a notebook.\n\nUmbrellaRule: If it rains, meet inside.\n';
const GUIDANCE = {
  en: 'Use your editor to add a .md file to this folder and start reading; no contract is needed.',
  'zh-Hant': '用你的編輯器在這個資料夾裡新增 .md 檔，就能開始閱讀；不需要契約。',
};
const MUTATIONS = {
  'restore-home-order': {
    site: 'home-order', fixture: 'plain',
    needle: 'return []DeskBlock{folderBlock, pathBlock, mapBlock, reportBlock}',
    replacement: 'return []DeskBlock{pathBlock, mapBlock, reportBlock, folderBlock}',
  },
  'restore-contract-prerequisite': {
    site: 'folder-guidance', fixture: 'empty',
    needle: 'empty = wording.JoinGuide(wording.FolderIndexEmpty, wording.FolderIndexUngovernedNext, lang)',
    replacement: 'empty = wording.JoinGuide(wording.IndexUngoverned, wording.IndexUngovernedNext, lang)',
  },
};

class LockFailed extends Error {
  constructor(site, message) { super(message); this.site = site; }
}
function lock(site, condition, message) {
  if (!condition) throw new LockFailed(site, message);
}

function build(binary, label) {
  const result = spawnSync('go', ['build', '-o', binary, './cmd/yomihon'], {
    cwd: ROOT, encoding: 'utf8', timeout: 180_000,
    env: { ...process.env, GOMAXPROCS: '2', GOFLAGS: '-p=2' },
  });
  writeFileSync(join(ARTIFACTS, `${label}-build.log`), `${result.stdout || ''}${result.stderr || ''}`);
  assert.equal(result.status, 0, `${label} must compile before acceptance: ${result.error || result.stderr}`);
}

async function startServer(binary, root, label) {
  let log = '';
  const child = spawn(binary, ['serve', root], {
    env: { ...process.env, YOMIHON_PORT: '0', XDG_CONFIG_HOME: `${root}-config` },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const exited = new Promise((done) => child.once('exit', done));
  let startError;
  child.once('error', (error) => { startError = error; });
  child.stdout.on('data', (data) => { log += data; });
  child.stderr.on('data', (data) => { log += data; });
  const stop = async () => {
    if (child.exitCode === null && child.signalCode === null && !startError) {
      child.kill('SIGTERM');
      const timer = setTimeout(() => child.kill('SIGKILL'), 5000);
      try { await exited; } finally { clearTimeout(timer); }
    }
    writeFileSync(join(ARTIFACTS, `${label}-server.log`), log);
  };
  try {
    for (let attempt = 0; attempt < 200; attempt++) {
      if (startError) throw startError;
      assert.equal(child.exitCode, null, `server exited before readiness: ${log}`);
      assert.equal(child.signalCode, null, `server was signaled before readiness: ${log}`);
      // The child's bound ephemeral port proves ownership; a random successful
      // fetch on a fixed port is not a readiness signal for this child.
      const match = log.match(/msg="yomihon serving" addr=(127\.0\.0\.1:\d+) /);
      if (match) {
        const base = `http://${match[1]}`;
        try {
          const response = await fetch(`${base}/`, { signal: AbortSignal.timeout(500) });
          await response.arrayBuffer();
          if (response.status === 200) return { base, stop };
        } catch { /* Wait only for this child's listener. */ }
      }
      await delay(50);
    }
    throw new Error(`server never reported its own bound listener: ${log}`);
  } catch (error) { await stop(); throw error; }
}

async function journey(browser, base, kind, width, language, javaScriptEnabled, label) {
  const context = await browser.newContext({ viewport: { width, height: 900 }, javaScriptEnabled });
  await context.addCookies([{ name: 'yomihon_lang', value: language, url: base }]);
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const writes = [];
  await page.route('**/*', (route) => {
    if (!['GET', 'HEAD'].includes(route.request().method())) {
      writes.push(route.request().url());
      return route.abort();
    }
    return route.continue();
  });
  const name = `${label}-${kind}-${width}-${language}-${javaScriptEnabled ? 'js' : 'no-js'}`;
  try {
    const response = await page.goto(`${base}/`);
    assert.equal(response.status(), 200, 'Home must answer directly');
    await page.evaluate(() => document.fonts.ready);
    const blocks = page.locator('.y-homegrid > [data-home-block]');
    const expected = kind === 'governed' ? MODES : ['folders', 'paths', 'maps', 'reports'];
    const actual = await blocks.evaluateAll((nodes) => nodes.map((node) => node.dataset.homeBlock));
    lock('home-order', JSON.stringify(actual) === JSON.stringify(expected), `Home order ${actual}; expected ${expected}`);
    const positions = await blocks.evaluateAll((nodes) => nodes.map((node) => {
      const box = node.getBoundingClientRect();
      return { mode: node.dataset.homeBlock, top: box.top, left: box.left };
    }));
    const visual = [...positions].sort((a, b) => a.top - b.top || a.left - b.left).map((item) => item.mode);
    assert.deepEqual(visual, expected, 'visual order must follow document order');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, 'Home must not overflow horizontally');
    const form = page.locator('[data-home-block="search"] form');
    assert.equal(await form.getAttribute('method'), 'get');
    assert.equal(await form.getAttribute('action'), '/search');
    assert.equal(await form.locator('input[name="q"]').isVisible(), true);
    await page.screenshot({ path: join(ARTIFACTS, `${name}-home.png`), fullPage: true });
    const folders = page.locator('[data-home-block="folders"]');
    if (kind === 'empty') {
      lock('folder-guidance', (await folders.innerText()).includes(GUIDANCE[language]), 'empty Home must offer Markdown creation without a contract prerequisite');
      await folders.locator('h2 a').click();
      await page.waitForURL(`${base}/folders`);
      lock('folder-guidance', (await page.locator('[data-index-empty]').innerText()).includes(GUIDANCE[language]), 'empty Folders must offer the same usable next step');
    } else {
      const target = kind === 'plain' ? '/notes/Reading%20club.md' : '/notes/Notes/Reading%20club.md';
      if (kind === 'plain') {
        await folders.locator(`a[data-desk-item][href="${target}"]`).click();
      } else {
        assert.ok(await page.locator('[data-home-block="paths"] [data-desk-item]').count(), 'governed Paths must retain populated rows');
        assert.ok(await page.locator('[data-home-block="maps"] [data-desk-item]').count(), 'governed Maps must retain populated rows');
        await folders.locator('h2 a').click();
        await page.locator('a[data-index-row][href="/folders/Notes"]').click();
        await page.locator(`a[data-index-row][href="${target}"]`).click();
      }
      await page.waitForURL(`${base}${target}`);
      assert.ok((await page.locator('main .y-prose').innerText()).includes('If it rains, meet inside.'), 'the selected note must contain the reading task answer');
      await page.goto(`${base}/`);
      await page.locator('[data-home-block="search"] input[name="q"]').fill('UmbrellaRule');
      await page.locator('[data-home-block="search"] button[type="submit"]').click();
      await page.waitForURL((url) => url.pathname === '/search' && url.searchParams.get('q') === 'UmbrellaRule');
      await page.locator(`a.y-result[href^="${target}"]`).click();
      assert.ok((await page.locator('main .y-prose').innerText()).includes('If it rains, meet inside.'), 'native Home search must still open the note');
    }
    // The optional organizations remain discoverable through their actual
    // shelf headings, without requiring schema setup before ordinary reading.
    for (const mode of ['paths', 'maps', 'reports']) {
      await page.goto(`${base}/`);
      await page.locator(`[data-home-block="${mode}"] h2 a`).click();
      await page.waitForURL(`${base}/${mode}`);
      assert.equal(await page.locator(`[data-index="${mode}"]`).count(), 1, `${mode} index remains reachable`);
    }
    assert.deepEqual(writes, [], 'reading acceptance must not write status or reader state');
    return { kind, width, language, javaScriptEnabled, actual, positions, outcome: 'passed' };
  } finally { await context.close(); }
}

async function probe(binary, fixtures, label, mutationName) {
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  const results = [];
  let activeServer;
  const terminate = () => {
    void (async () => {
      try { if (activeServer) await activeServer.stop(); }
      finally { await browser.close(); process.exit(2); }
    })();
  };
  process.once('SIGTERM', terminate);
  try {
    const kinds = mutationName ? [MUTATIONS[mutationName].fixture] : ['empty', 'plain', 'governed'];
    for (const kind of kinds) {
      const server = await startServer(binary, join(fixtures, kind), `${label}-${kind}`);
      activeServer = server;
      try {
        for (const width of mutationName ? [390] : [1600, 390]) {
          for (const language of mutationName ? ['en'] : ['zh-Hant', 'en']) {
            for (const js of mutationName ? [false] : [true, false]) {
              results.push(await journey(browser, server.base, kind, width, language, js, label));
            }
          }
        }
      } finally { await server.stop(); activeServer = undefined; }
    }
  } finally {
    process.removeListener('SIGTERM', terminate);
    writeFileSync(join(ARTIFACTS, `${label}-journeys.json`), JSON.stringify(results, null, 2));
    await browser.close();
  }
}

function runProbe(binary, fixtures, label, mutationName = '') {
  const result = spawnSync(process.execPath, [SCRIPT, '--probe', binary, fixtures, label, mutationName], {
    cwd: ROOT, encoding: 'utf8', timeout: 180_000, env: { ...process.env, HOME_ARRIVAL_ARTIFACTS: ARTIFACTS },
  });
  const output = `${result.stdout || ''}${result.stderr || ''}`;
  writeFileSync(join(ARTIFACTS, `${label}-probe.log`), output);
  process.stdout.write(output);
  if (mutationName) {
    assert.equal(result.status, 1, `${label}: expected a compiled regression caught with exit 1; ${result.error || ''}`);
    assert.ok(output.split('\n').includes(`MUTATE-RESULT: caught ${mutationName}`), `${label}: required exact owned-site marker missing`);
  } else {
    assert.equal(result.status, 0, `${label}: baseline acceptance failed; ${result.error || ''}`);
  }
}

function orchestrate() {
  const temp = mkdtempSync(join(tmpdir(), 'yomihon-home-arrival-'));
  const original = readFileSync(SOURCE, 'utf8');
  try {
    for (const kind of ['empty', 'plain', 'governed']) mkdirSync(join(temp, kind));
    writeFileSync(join(temp, 'plain/Reading club.md'), READING);
    writeFileSync(join(temp, 'plain/Weather.md'), '# Weather\n\nRead the club plan for rain.\n');
    const fixture = join(ROOT, '.github/e2e/vault');
    for (const entry of readdirSync(fixture)) {
      cpSync(join(fixture, entry), join(temp, 'governed', entry), { recursive: true });
    }
    assert.ok(readFileSync(join(temp, 'governed/System/schemas/vault-schema.toml'), 'utf8').includes('[navigation]'));
    writeFileSync(join(temp, 'governed/Notes/Reading club.md'), READING);
    const binary = join(temp, 'yomihon');
    build(binary, 'baseline');
    runProbe(binary, temp, 'baseline');
    for (const [name, mutation] of Object.entries(MUTATIONS)) {
      const hits = original.split(mutation.needle).length - 1;
      assert.equal(hits, 1, `${name}: mutation must match exactly one production site, found ${hits}`);
      try {
        writeFileSync(SOURCE, original.replace(mutation.needle, mutation.replacement));
        build(binary, name);
        runProbe(binary, temp, name, name);
      } finally { writeFileSync(SOURCE, original); }
      assert.equal(readFileSync(SOURCE, 'utf8'), original, `${name}: production bytes must be restored`);
    }
    build(binary, 'restored');
    runProbe(binary, temp, 'restored');
    console.log('PASS home-arrival: baseline and restored journeys; both compiled production mutations caught');
  } finally {
    writeFileSync(SOURCE, original);
    rmSync(temp, { recursive: true, force: true });
  }
}

mkdirSync(ARTIFACTS, { recursive: true });
try {
  assert.equal(process.env.CI, 'true', 'this acceptance driver runs in CI; local execution is paused');
  if (process.argv[2] === '--probe') {
    await probe(...process.argv.slice(3));
  } else {
    orchestrate();
  }
} catch (error) {
  const mutationName = process.argv[6];
  console.error(error.stack || error);
  if (error instanceof LockFailed && MUTATIONS[mutationName]?.site === error.site) {
    console.log(`MUTATE-RESULT: caught ${mutationName}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 2;
  }
}
