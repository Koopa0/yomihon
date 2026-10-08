// Audit the real fixture pages without exclusions. Incomplete results remain
// visible for manual review; every violation fails the ordinary audit.
import { AxeBuilder } from '@axe-core/playwright';
import { chromium } from 'playwright-core';
import { arrived } from './support/arrival.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const MODE = 'drop-task-label';
const NOTE = '/notes/Notes/reading-fidelity.md';
const PAGES = ['/', NOTE, '/syllabus/Maps/branches.md'];
const THEMES = ['light', 'dark'];
const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
const LABEL = '<label class="y-task"><input disabled="" type="checkbox"> Unfinished task</label>';
const UNLABELLED = '<input disabled="" type="checkbox"> Unfinished task';
class NotApplied extends Error {}
class LockFired extends Error {}
if (MUTATE === 'list') { console.log(MODE); process.exit(0); }
if (MUTATE && MUTATE !== MODE) { console.error(`unknown MUTATE ${MUTATE}`); process.exit(2); }

const audit = async (browser, path, theme, canary = false) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  try {
    await context.addCookies([{ name: 'yomihon_theme', value: theme, url: BASE }]);
    const page = await context.newPage();
    let requests = 0;
    let matches = 0;
    if (canary) {
      await page.route(new URL(NOTE, BASE).href, async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        requests += 1;
        matches = original.split(LABEL).length - 1;
        await route.fulfill({ response, body: matches === 1 ? original.replace(LABEL, UNLABELLED) : original });
      });
    }
    const response = await page.goto(new URL(path, BASE).href, { waitUntil: 'networkidle' });
    if (response?.status() !== 200) throw new Error(`${path} HTTP ${response?.status()}`);
    await page.evaluate(() => document.fonts.ready);
    await arrived(page);
    await page.waitForFunction(() => document.documentElement.dataset.js === 'on');
    await page.waitForFunction(() => [...document.querySelectorAll('.mermaid-diagram')].every((block) =>
      block.querySelector('svg') || block.hasAttribute('data-mermaid-error')));
    await page.waitForFunction(() => [...document.querySelectorAll('[data-codecopy-button]')].every((button) => !button.disabled));
    const state = await page.evaluate(() => ({ theme: document.documentElement.dataset.theme, language: document.documentElement.lang }));
    if (state.theme !== theme || state.language !== 'zh-Hant') throw new Error(`wrong audit state ${JSON.stringify(state)}`);
    if (canary) {
      const applied = await page.evaluate(() => {
        const inputs = [...document.querySelectorAll('.y-prose li > input[type="checkbox"]')]
          .filter((input) => input.parentElement.textContent.trim() === 'Unfinished task');
        return inputs.length === 1 && inputs[0].labels.length === 0;
      });
      if (requests !== 1 || matches !== 1 || !applied) throw new NotApplied(`label requests=${requests} matches=${matches} applied=${applied}`);
    }
    console.log(`INVOCATION-HIT a11y-audit page=${path} theme=${theme} canary=${canary}`);
    const result = await new AxeBuilder({ page }).withTags(TAGS).analyze();
    for (const [kind, findings] of [['VIOLATION', result.violations], ['INCOMPLETE', result.incomplete]]) {
      for (const finding of findings) {
        console.log(`${kind} a11y-audit page=${path} theme=${theme} rule=${finding.id} impact=${finding.impact ?? 'none'}`);
        for (const node of finding.nodes) console.log(`TARGET ${kind} a11y-audit ${JSON.stringify(node.target)}`);
      }
    }
    console.log(`AUDIT a11y-audit page=${path} theme=${theme} version=${result.testEngine.version} violations=${result.violations.length} incomplete=${result.incomplete.length}`);
    if (canary) {
      const nodes = result.violations.filter((finding) => finding.id === 'label').flatMap((finding) => finding.nodes);
      const caught = await page.evaluate((targets) => {
        const input = [...document.querySelectorAll('.y-prose li > input[type="checkbox"]')]
          .find((candidate) => candidate.parentElement.textContent.trim() === 'Unfinished task');
        return targets.some((target) => target.length === 1 && typeof target[0] === 'string' && document.querySelector(target[0]) === input);
      }, nodes.map((node) => node.target));
      if (!caught) throw new LockFired('one-site canary did not report label on its actual task input');
      console.log('caught: a11y-audit label');
    }
    return { violations: result.violations.length, incomplete: result.incomplete.length, version: result.testEngine.version };
  } finally {
    await context.close();
  }
};

let browser;
const started = performance.now();
try {
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  if (MUTATE) {
    await audit(browser, NOTE, 'light', true);
    console.log(`MUTATE-RESULT: caught ${MODE}`);
    process.exitCode = 1;
  } else {
    let violations = 0;
    let incomplete = 0;
    const versions = new Set();
    for (const theme of THEMES) {
      for (const path of PAGES) {
        const result = await audit(browser, path, theme);
        violations += result.violations;
        incomplete += result.incomplete;
        versions.add(result.version);
      }
    }
    if (violations) throw new LockFired(`${violations} violations across six page/theme audits`);
    await audit(browser, NOTE, 'light', true);
    console.log(`PASS a11y-audit routes=${JSON.stringify(PAGES)} themes=${JSON.stringify(THEMES)} viewport=1280x900 version=${[...versions].join(',')} runtime-ms=${Math.round(performance.now() - started)} violations=0 incomplete=${incomplete} canary=label`);
  }
} catch (error) {
  console.error(`a11y-audit: ${error.message}`);
  if (MUTATE && error instanceof NotApplied) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
  process.exitCode = error instanceof LockFired ? (MUTATE ? 0 : 1) : 2;
} finally {
  await browser?.close();
}
