import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import test from 'node:test';

const root = resolve(import.meta.dirname, '..');
const config = JSON.parse(readFileSync(resolve(root, 'biome.json'), 'utf8'));
const plugins = (config.plugins || []).map((plugin) => resolve(root, typeof plugin === 'string' ? plugin : plugin.path));
const diagnostic = 'support/arrival.mjs';
const rejected = [
  ['async arrow', 'page.waitForFunction(async () => false);'],
  ['bare async parameter', 'page.waitForFunction(async value => value);'],
  ['parenthesized async', 'page.waitForFunction((async () => false));'],
  ['computed method', 'page["waitForFunction"](async () => false);'],
  ['single-quoted method', "page['waitForFunction'](async () => false);"],
  ['optional receiver', 'page?.waitForFunction(async () => false);'],
  ['optional call', 'page.waitForFunction?.(async () => false);'],
  ['optional receiver and call', 'page?.waitForFunction?.(async () => false);'],
  ['computed optional receiver', 'page?.["waitForFunction"](async () => false);'],
  ['computed optional call', "page['waitForFunction']?.(async () => false);"],
  ['computed optional receiver and call', 'page?.["waitForFunction"]?.(async () => false);'],
  ['parenthesized method', '(page.waitForFunction)(async () => false);'],
  ['async function', 'page.waitForFunction(async function () { return false; }, null, { timeout: 3000 });'],
  ['named async function', 'page.waitForFunction(async function ready() { return false; });'],
  ['commented async arrow', 'page.waitForFunction(/* readiness */ async (value) => value);'],
  ['Promise expression', 'page.waitForFunction(() => new Promise(resolve => resolve(false)));'],
  ['Promise return', 'page.waitForFunction(() => { return new Promise(resolve => resolve(false)); });'],
  ['resolved Promise', 'page.waitForFunction(() => Promise.resolve(false));'],
  ['Promise function', 'page.waitForFunction(function () { return Promise.resolve(false); });'],
];
const accepted = [
  ['sync arrow', 'page.waitForFunction(() => false, null, { timeout: 3000 });'],
  ['sync function', 'page.waitForFunction(function () { return false; });'],
  ['computed sync predicate', 'page["waitForFunction"](() => false);'],
  ['optional computed sync predicate', "page?.['waitForFunction']?.(() => false);"],
  ['parenthesized method sync predicate', '(page.waitForFunction)(() => false);'],
  ['comment and string', 'page.waitForFunction(() => { /* async */ return "new Promise(resolve)"; });'],
  ['other argument', 'page.waitForFunction(value => value, Promise.resolve(false));'],
  ['async evaluation', 'page.evaluate(async () => false);'],
];

for (const [name, source, status] of [
  ...rejected.map(([name, source]) => [name, source, 1]),
  ...accepted.map(([name, source]) => [name, source, 0]),
]) {
  test(name, () => {
    const dir = mkdtempSync(resolve(tmpdir(), 'yomihon-browser-waits-'));
    try {
      writeFileSync(resolve(dir, 'biome.json'), JSON.stringify({ plugins, linter: { rules: { recommended: false } } }));
      const fixture = resolve(dir, 'probe.mjs');
      writeFileSync(fixture, source);
      const result = spawnSync(resolve(root, '.github/node_modules/.bin/biome'), ['lint', `--config-path=${dir}`, fixture], { encoding: 'utf8' });
      assert.ifError(result.error);
      const output = result.stdout + result.stderr;
      assert.equal(result.status, status, `${name}: Biome exit ${result.status}, want ${status}\n${output}`);
      if (status === 1) assert.ok(output.includes(diagnostic), `missing arrival guidance\n${output}`);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
}
