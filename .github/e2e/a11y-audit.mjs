// Development-only audit of the real fixture pages. Every violation fails;
// axe's incomplete results remain visible as a separate review obligation.
import { AxeBuilder } from '@axe-core/playwright';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const NOTE = '/notes/Notes/reading-fidelity.md';
const PAGES = ['/', NOTE, '/syllabus/Maps/branches.md'];
const THEMES = ['light', 'dark'];
const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
const LABEL = '<label class="y-task"><input disabled="" type="checkbox"> Unfinished task</label>';
const UNLABELLED = '<input disabled="" type="checkbox"> Unfinished task';

if (MUTATE === 'list') {
  console.log('drop-task-label');
  process.exit(0);
}
if (MUTATE && MUTATE !== 'drop-task-label') {
  console.error(`a11y-audit: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class ProbeBroken extends Error {}
class NotApplied extends Error {}
class LockFired extends Error {}

// Base's head sets data-js before the module runs. Exercise the shared real
// listener on every route, including pages with no code blocks or diagrams.
// Ruby avoids starting a theme redraw, and its original state is restored.
const exerciseRuntime = async (page, theme) => {
  const toggle = page.locator('[data-ruby-toggle]');
  if (await toggle.count() !== 1) throw new ProbeBroken('runtime readiness needs exactly one ruby toggle');
  const before = await page.evaluate(() => ({
    ruby: document.documentElement.dataset.ruby,
    pressed: document.querySelector('[data-ruby-toggle]').getAttribute('aria-pressed'),
    theme: document.documentElement.dataset.theme,
  }));
  const cookie = (await page.context().cookies(BASE)).find((entry) => entry.name === 'yomihon_ruby');
  const effectiveRuby = cookie?.value === 'off' ? 'off' : 'on';
  if (before.ruby !== effectiveRuby || before.pressed !== String(before.ruby === 'on') || before.theme !== theme) {
    throw new ProbeBroken(`runtime readiness initial state is inconsistent: ${JSON.stringify(before)}`);
  }
  const active = await page.evaluateHandle(() => document.activeElement);
  try {
    for (const value of [before.ruby === 'on' ? 'off' : 'on', before.ruby]) {
      await toggle.click({ timeout: 5000 });
      await page.waitForFunction((expected) => {
        const kept = document.cookie.split(';').map((part) => part.trim()).find((part) => part.startsWith('yomihon_ruby='));
        return document.documentElement.dataset.ruby === expected &&
          document.querySelector('[data-ruby-toggle]').getAttribute('aria-pressed') === String(expected === 'on') &&
          kept === `yomihon_ruby=${expected}`;
      }, value, { timeout: 5000 });
    }
    if (!cookie) await page.context().clearCookies({ name: 'yomihon_ruby' });
    else await page.context().addCookies([cookie]);
    const after = await page.evaluate(() => ({
      ruby: document.documentElement.dataset.ruby,
      pressed: document.querySelector('[data-ruby-toggle]').getAttribute('aria-pressed'),
      theme: document.documentElement.dataset.theme,
    }));
    const restoredCookies = await page.context().cookies(BASE);
    const restored = restoredCookies.find((entry) => entry.name === 'yomihon_ruby');
    const restoredTheme = restoredCookies.find((entry) => entry.name === 'yomihon_theme');
    if (JSON.stringify(after) !== JSON.stringify(before) || (restored?.value ?? null) !== (cookie?.value ?? null) || restoredTheme?.value !== theme) {
      throw new ProbeBroken('runtime readiness did not restore ruby root/control/cookie and the audited theme');
    }
    console.log(`invoked: a11y-audit runtime ruby listener exercised/restored theme=${theme} ruby=${before.ruby} cookie=${cookie?.value ?? 'absent'}`);
  } catch (error) {
    throw new ProbeBroken(`runtime ruby enhancement unavailable before axe: ${error.message}`);
  } finally {
    await toggle.evaluate((element) => element.blur());
    await active.evaluate((element) => element.focus());
    await active.dispose();
  }
};

const settle = async (page, theme) => {
  await page.waitForFunction(() => document.documentElement.dataset.js === 'on', null, { timeout: 5000 });
  await exerciseRuntime(page, theme);
  await page.waitForFunction(() => [...document.styleSheets].some((sheet) => {
    if (!sheet.href || new URL(sheet.href).pathname !== '/static/app.css') return false;
    return sheet.cssRules.length > 0;
  }), null, { timeout: 5000 });
  await page.waitForFunction(() => [...document.querySelectorAll('.mermaid-diagram')].every((block) =>
    block.querySelector('svg') || block.hasAttribute('data-mermaid-error')),
  null, { timeout: 10000 });
  await page.waitForFunction(() => [...document.querySelectorAll('.y-prose pre')].every((block) => {
    if (block.closest('[aria-hidden="true"]')) return true;
    for (let scope = block.parentElement; scope; scope = scope.parentElement) {
      if (scope.querySelector(':scope > template[data-codecopy-template]')) {
        const controls = block.previousElementSibling;
        const button = controls?.querySelector('[data-codecopy-button]');
        return controls?.matches('.y-codecopy') && button && !button.disabled;
      }
    }
    return true;
  }), null, { timeout: 5000 });
  const state = await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return { theme: document.documentElement.dataset.theme, language: document.documentElement.lang, fonts: document.fonts.status };
  });
  if (state.theme !== theme || state.language !== 'zh-Hant' || state.fonts !== 'loaded') throw new ProbeBroken(`page did not settle in ${theme}/zh-Hant: ${JSON.stringify(state)}`);
};

const printResults = (path, theme, kind, results) => {
  for (const result of results) {
    console.log(`${kind} a11y-audit page=${path} theme=${theme} rule=${result.id} impact=${result.impact ?? 'none'}`);
    for (const node of result.nodes) console.log(`TARGET a11y-audit ${kind} page=${path} theme=${theme} rule=${result.id} target=${JSON.stringify(node.target)}`);
  }
};

const audit = async (browser, path, theme, canary = false) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  try {
    await context.addCookies([{ name: 'yomihon_theme', value: theme, url: BASE }]);
    const page = await context.newPage();
    const rewritten = { requests: 0, matches: 0, error: null };
    if (canary) {
      await page.route(new URL(NOTE, BASE).href, async (route) => {
        const response = await route.fetch();
        rewritten.requests += 1;
        if (response.status() !== 200) {
          rewritten.error = new ProbeBroken(`canary source status ${response.status()}, want 200`);
          await route.fulfill({ response });
          return;
        }
        const original = await response.text();
        rewritten.matches = original.split(LABEL).length - 1;
        if (rewritten.matches !== 1) {
          rewritten.error = new NotApplied(`task label needle matched ${rewritten.matches} sites, want exactly one`);
          await route.fulfill({ response });
          return;
        }
        await route.fulfill({ response, body: original.replace(LABEL, UNLABELLED) });
      });
    }
    const stylesheet = page.waitForResponse((response) => new URL(response.url()).pathname === '/static/app.css', { timeout: 10000 });
    // Capture the wait's rejection while navigation is in flight as well.
    const cssStatus = stylesheet.then((response) => response.status(), (error) => error);
    const response = await page.goto(new URL(path, BASE).href, { waitUntil: 'load' });
    if (rewritten.error) throw rewritten.error;
    if (!response || response.status() !== 200) throw new ProbeBroken(`${path} status ${response?.status()}, want 200`);
    const status = await cssStatus;
    if (status !== 200) throw new ProbeBroken(`${path} stylesheet status ${status}, want 200`);
    await settle(page, theme);
    if (canary) {
      if (rewritten.requests !== 1 || rewritten.matches !== 1) throw new NotApplied(`canary source requests=${rewritten.requests} matches=${rewritten.matches}, want one of each`);
      const applied = await page.evaluate(() => {
        const inputs = [...document.querySelectorAll('.y-prose li > input[type="checkbox"]')]
          .filter((input) => input.parentElement.textContent.trim() === 'Unfinished task');
        return inputs.length === 1 && inputs[0].disabled && !inputs[0].checked && inputs[0].labels.length === 0;
      });
      if (!applied) throw new NotApplied('the rewritten task input did not reach the settled page without its label');
    }
    console.log(`invoked: a11y-audit page=${path} theme=${theme} canary=${canary} status=200 settled`);
    // AxeBuilder injects its pinned source with page.evaluate; no script URL or
    // CSP exception is added, and no rules or parts of the page are excluded.
    const result = await new AxeBuilder({ page }).withTags(TAGS).analyze();
    printResults(path, theme, canary ? 'CANARY-VIOLATION' : 'VIOLATION', result.violations);
    printResults(path, theme, canary ? 'CANARY-INCOMPLETE' : 'INCOMPLETE', result.incomplete);
    console.log(`AUDIT a11y-audit page=${path} theme=${theme} canary=${canary} version=${result.testEngine.version} violations=${result.violations.length} incomplete=${result.incomplete.length}`);
    if (canary) {
      const labelNodes = result.violations.filter((violation) => violation.id === 'label').flatMap((violation) => violation.nodes);
      const caught = await page.evaluate((nodes) => {
        const input = [...document.querySelectorAll('.y-prose li > input[type="checkbox"]')]
          .find((candidate) => candidate.parentElement.textContent.trim() === 'Unfinished task');
        return nodes.some((node) => node.target.length === 1 && typeof node.target[0] === 'string' && document.querySelector(node.target[0]) === input);
      }, labelNodes);
      if (!caught) throw new LockFired('the one-site task-label canary was not reported as label on its input; qualify the ruled button-name fallback from this receipt');
      console.log(`CANARY a11y-audit page=${path} theme=${theme} rule=label version=${result.testEngine.version}: caught one unlabelled task input`);
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
    console.log('caught: a11y-audit label');
    console.log('MUTATE-RESULT: caught drop-task-label');
    process.exitCode = 1;
  } else {
    let violations = 0;
    let incomplete = 0;
    let unavailable = 0;
    const versions = new Set();
    for (const theme of THEMES) {
      for (const path of PAGES) {
        try {
          const result = await audit(browser, path, theme);
          violations += result.violations;
          incomplete += result.incomplete;
          versions.add(result.version);
        } catch (error) {
          unavailable += 1;
          console.error(`UNAVAILABLE a11y-audit page=${path} theme=${theme}: ${error.message}`);
        }
      }
    }
    console.log(`SUMMARY a11y-audit audits=6 violations=${violations} incomplete=${incomplete} unavailable=${unavailable}`);
    await audit(browser, NOTE, 'light', true);
    if (unavailable > 0) throw new ProbeBroken(`${unavailable} page/theme audits were unavailable`);
    if (violations > 0) throw new LockFired(`${violations} violations across six page/theme audits; incomplete=${incomplete}`);
    console.log(`PASS a11y-audit routes=${JSON.stringify(PAGES)} themes=${JSON.stringify(THEMES)} viewport=1280x900 version=${[...versions].join(',')} runtime-ms=${Math.round(performance.now() - started)} violations=0 incomplete=${incomplete} canary=label`);
  }
} catch (error) {
  console.error(`a11y-audit: ${error.message}`);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE || 'plain-canary'}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.log('caught: a11y-audit contract');
    process.exitCode = 1;
  } else {
    process.exitCode = 2;
  }
} finally {
  try { await browser?.close(); } catch (error) {
    console.error(`a11y-audit browser cleanup: ${error.message}`);
    process.exitCode = 2;
  }
}
