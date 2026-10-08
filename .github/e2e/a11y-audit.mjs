// Development-only audit of the real fixture pages. Every violation fails;
// axe's incomplete results remain visible as a separate review obligation.
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname } from 'node:path';
import { createConnection } from 'node:net';

// Playwright initializes its debug sinks when its dependency graph is loaded.
// Sanitize before either dependency can initialize a logger with the endpoint.
delete process.env.DEBUG;
delete process.env.DEBUG_FILE;
const { AxeBuilder } = await import('@axe-core/playwright');
const { chromium } = await import('playwright-core');

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

const readHandEndpoint = () => {
  const args = process.argv.slice(2);
  if (args.length === 0) return '';
  if (MUTATE === 'drop-task-label' && args.length === 2 && args[0] === '--hand-browser-endpoint') {
    try {
      const url = new URL(args[1]);
      if (url.protocol === 'ws:' && url.hostname === '127.0.0.1' && url.port &&
          Number(url.port) > 0 && Number(url.port) <= 65535 && url.pathname.length > 1 &&
          !url.username && !url.password && !url.search && !url.hash) return args[1];
    } catch { /* Refuse without disclosing the supplied capability. */ }
  }
  throw new ProbeBroken('usage: a11y-audit.mjs [internal hand browser endpoint]');
};

let endpoint = '';
const redact = (value) => endpoint ? String(value).split(endpoint).join('[browser-endpoint]') : String(value);
const started = performance.now();
let status = 0;
const unavailable = (message) => {
  status = 2;
  console.error(`UNAVAILABLE a11y-audit: ${redact(message)}`);
};
const delay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

const confirmEndpointClosed = () => new Promise((resolve) => {
  const url = new URL(endpoint);
  const socket = createConnection({ host: '127.0.0.1', port: Number(url.port) });
  let settled = false;
  const finish = (refused) => {
    if (settled) return;
    settled = true;
    socket.destroy();
    resolve(refused);
  };
  socket.setTimeout(250);
  socket.once('connect', () => finish(false));
  socket.once('timeout', () => finish(false));
  socket.once('error', (error) => finish(error.code === 'ECONNREFUSED'));
});

let server;
let browser;
let plainClientAcquired = false;
let chrome;
let chromePID;
let chromeClose;
let chromeClosed = false;
let child;
let childClose;
let childClosed = false;
let childError = false;
let shutdownPromise;
let cancelled = false;
let launchPending = false;
let workTimer;
let childTimer;
let unresolved = false;
const signals = ['SIGINT', 'SIGTERM', 'SIGHUP'];
const onSignal = (signal) => {
  cancelled = true;
  unavailable(`cancelled by ${signal}`);
  console.log('caught: a11y-audit operational cancellation');
  if (!launchPending) void shutdownPlain();
};

const runHandChild = async () => {
  const env = { ...process.env, MUTATE: 'drop-task-label' };
  delete env.NODE_OPTIONS;
  delete env.DEBUG;
  delete env.DEBUG_FILE;
  const filename = fileURLToPath(import.meta.url);
  child = spawn(process.execPath, [filename, '--hand-browser-endpoint', endpoint], {
    cwd: dirname(filename), env, shell: false, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let stdout = Buffer.alloc(0);
  let stderr = Buffer.alloc(0);
  child.once('spawn', () => {
    console.log('invoked: a11y-audit hand-mode child spawned mode=drop-task-label');
    console.log('invoked: a11y-audit proof plain-owner child-spawn-boundary');
    process.kill(process.pid, 'SIGTERM');
  });
  child.once('error', (error) => {
    childError = true;
    unavailable(`hand child process error: ${error.message}`);
    void shutdownPlain();
  });
  childClose = new Promise((resolve) => child.once('close', (code, signal) => {
    childClosed = true;
    clearTimeout(childTimer);
    console.log(`invoked: a11y-audit hand-mode child mode=drop-task-label status=${code ?? 'none'} signal=${signal ?? 'none'}`);
    resolve({ code, signal });
  }));
  for (const [pipe, name] of [[child.stdout, 'stdout'], [child.stderr, 'stderr']]) {
    pipe.on('data', (chunk) => {
      const kept = name === 'stdout' ? stdout : stderr;
      const room = 1048576 - kept.length;
      const next = Buffer.concat([kept, chunk.subarray(0, Math.max(0, room))]);
      if (name === 'stdout') stdout = next;
      else stderr = next;
      if (chunk.length > room && !childError) {
        childError = true;
        unavailable(`hand child ${name} exceeded 1048576 bytes`);
        void shutdownPlain();
      }
    });
    pipe.on('error', (error) => {
      childError = true;
      unavailable(`hand child ${name} stream error: ${error.message}`);
      void shutdownPlain();
    });
  }
  childTimer = setTimeout(() => {
    cancelled = true;
    unavailable('hand child deadline exceeded');
    console.log('caught: a11y-audit operational child-deadline');
    void shutdownPlain();
  }, 120000);
  const close = await childClose;
  process.stdout.write(redact(stdout.toString('utf8')));
  process.stderr.write(redact(stderr.toString('utf8')));
  return { ...close, stdout: stdout.toString('utf8') };
};

const assertHandReceipt = (receipt, versions) => {
  if (childError || cancelled || receipt.signal !== null || ![0, 1].includes(receipt.code)) {
    throw new ProbeBroken('hand child did not complete operationally');
  }
  // Only newline-terminated stdout records are proof, never stderr or prefixes.
  const lines = receipt.stdout.split('\n').slice(0, -1);
  const unique = (prefix) => {
    const found = lines.filter((line) => line.startsWith(prefix));
    if (found.length !== 1) throw new LockFired(`hand receipt count for ${prefix} was ${found.length}, want one`);
    return found[0];
  };
  const exact = (line) => {
    if (lines.filter((entry) => entry === line).length !== 1) throw new LockFired('hand exact caught or settled receipt missing or duplicated');
  };
  exact('caught: a11y-audit label');
  exact('MUTATE-RESULT: caught drop-task-label');
  const invocation = unique('invoked: a11y-audit page=');
  if (invocation !== `invoked: a11y-audit page=${NOTE} theme=light canary=true status=200 settled`) throw new LockFired('hand settled invocation identity differs');
  const auditLine = unique('AUDIT a11y-audit ');
  const auditMatch = auditLine.match(/^AUDIT a11y-audit page=(\S+) theme=(\S+) canary=true version=(\S+) violations=(\d+) incomplete=(\d+)$/);
  const canaryLine = unique('CANARY a11y-audit ');
  const canaryMatch = canaryLine.match(/^CANARY a11y-audit page=(\S+) theme=(\S+) rule=label version=(\S+): caught one unlabelled task input$/);
  if (!auditMatch || !canaryMatch || auditMatch[1] !== NOTE || canaryMatch[1] !== NOTE ||
      auditMatch[2] !== 'light' || canaryMatch[2] !== 'light') throw new LockFired('hand audit/canary identity differs');
  if (auditMatch[3] !== canaryMatch[3] || versions.size !== 1 || !versions.has(auditMatch[3])) {
    throw new ProbeBroken('hand and ordinary axe versions are absent or disagree');
  }
  const violation = unique('CANARY-VIOLATION a11y-audit ');
  if (!violation.startsWith(`CANARY-VIOLATION a11y-audit page=${NOTE} theme=light rule=label impact=`)) throw new LockFired('hand label violation identity differs');
  const target = unique('TARGET a11y-audit CANARY-VIOLATION ');
  const targetPrefix = `TARGET a11y-audit CANARY-VIOLATION page=${NOTE} theme=light rule=label target=`;
  if (!target.startsWith(targetPrefix)) throw new LockFired('hand label target identity differs');
  let nodes;
  try { nodes = JSON.parse(target.slice(targetPrefix.length)); } catch { throw new LockFired('hand target is malformed JSON'); }
  if (!Array.isArray(nodes) || nodes.length !== 1 || typeof nodes[0] !== 'string' || !nodes[0]) throw new LockFired('hand target is not one actual selector');
  if (receipt.code !== 1) throw new LockFired(`hand child status ${receipt.code}, want 1`);
  return auditMatch[3];
};

const shutdownPlain = () => {
  if (shutdownPromise) return shutdownPromise;
  shutdownPromise = (async () => {
    const cleanupStarted = performance.now();
    let serverSettled = !server;
    let serverCloseResult = server ? 'pending' : 'not-acquired';
    let chromeObserved = !chrome || chromeClosed;
    let childObserved = !child || childClosed;
    let clientSettled = !browser;
    chromeClose?.then(() => { chromeObserved = true; });
    childClose?.then(() => { childObserved = true; });
    const signalChild = (signal) => {
      if (!child || childObserved) {
        console.log(`invoked: a11y-audit cleanup hand-child signal=${signal} request=${child ? 'already-closed' : 'not-acquired'}`);
        return;
      }
      try {
        const accepted = child.kill(signal);
        console.log(`invoked: a11y-audit cleanup hand-child signal=${signal} request=${accepted ? 'accepted' : 'false'}`);
      } catch (error) {
        console.log(`invoked: a11y-audit cleanup hand-child signal=${signal} request=${error.code === 'ESRCH' ? 'already-absent' : 'error'}`);
        if (error.code !== 'ESRCH') unavailable(`hand child ${signal} failed: ${error.message}`);
      }
    };
    // Start both owned teardown paths before awaiting either.
    if (child && !childObserved) signalChild('SIGTERM');
    if (server) {
      Promise.resolve().then(() => server.close()).then(() => {
        serverSettled = true;
        serverCloseResult = 'fulfilled';
        console.log('invoked: a11y-audit cleanup browser-server close fulfilled');
      }, (error) => {
        serverSettled = true;
        serverCloseResult = 'rejected';
        console.log('invoked: a11y-audit cleanup browser-server close rejected');
        unavailable(`BrowserServer close failed: ${error.message}`);
      });
    }
    if (browser) {
      const client = browser;
      browser = undefined;
      Promise.resolve().then(() => client.close()).then(() => { clientSettled = true; }, (error) => {
        clientSettled = true;
        unavailable(`client disconnect failed: ${error.message}`);
      });
    }
    let escalated = false;
    while (performance.now() - cleanupStarted < 10000 && (!chromeObserved || !childObserved || !serverSettled || !clientSettled)) {
      if (!escalated && performance.now() - cleanupStarted >= 5000) {
        escalated = true;
        unavailable('owned cleanup required escalation');
        console.log('caught: a11y-audit operational cleanup-escalation');
        if (!chromeObserved || !serverSettled) {
          console.log('invoked: a11y-audit cleanup escalation owned-browser-server SIGKILL');
          Promise.resolve().then(() => server.kill()).catch((error) => unavailable(`BrowserServer kill failed: ${error.message}`));
        }
        if (!childObserved) {
          console.log('invoked: a11y-audit cleanup escalation owned-hand-child SIGKILL');
          signalChild('SIGKILL');
        }
      }
      await delay(50);
    }
    let groupAbsent = !chrome;
    let endpointRefused = !server;
    let groupKillSent = false;
    if (chrome && !['linux', 'darwin'].includes(process.platform)) unavailable('owned Chrome process group observation unsupported');
    while (performance.now() - cleanupStarted < 15000 && (!groupAbsent || !endpointRefused)) {
      if (chromeObserved && chrome && ['linux', 'darwin'].includes(process.platform)) {
        try {
          process.kill(-chromePID, 0);
          if (!groupKillSent) {
            groupKillSent = true;
            unavailable('owned Chrome group remained after root close');
            console.log('caught: a11y-audit operational group-escalation');
            console.log('invoked: a11y-audit cleanup escalation owned-chrome-group SIGKILL');
            process.kill(-chromePID, 'SIGKILL');
          }
        } catch (error) {
          if (error.code === 'ESRCH') groupAbsent = true;
          else unavailable(`owned Chrome group observation/kill failed: ${error.message}`);
        }
      }
      if (server && !endpointRefused) endpointRefused = await confirmEndpointClosed();
      if (!groupAbsent || !endpointRefused) await delay(50);
    }
    if (chrome && groupAbsent) console.log('invoked: a11y-audit cleanup owned-chrome-group absent');
    if (server && endpointRefused) console.log('invoked: a11y-audit cleanup browser-endpoint refused');
    if (!chromeObserved || !childObserved || !serverSettled || !clientSettled || !groupAbsent || !endpointRefused) {
      unresolved = true;
      unavailable(`unresolved cleanup chrome-close=${chromeObserved} child-close=${childObserved} server-close=${serverSettled} client-close=${clientSettled} group-absent=${groupAbsent} endpoint-refused=${endpointRefused}`);
    }
    console.log(`invoked: a11y-audit cleanup terminal chrome=${chrome ? 'acquired' : 'not-acquired'} chrome-close=${chrome ? chromeObserved : 'not-acquired'} child=${child ? 'acquired' : 'not-acquired'} child-close=${child ? childObserved : 'not-acquired'} server=${server ? 'acquired' : 'not-acquired'} server-settled=${server ? serverSettled : 'not-acquired'} server-close-result=${serverCloseResult} client=${plainClientAcquired ? 'acquired' : 'not-acquired'} client-settled=${plainClientAcquired ? clientSettled : 'not-acquired'} group-absent=${chrome ? groupAbsent : 'not-acquired'} endpoint-refused=${server ? endpointRefused : 'not-acquired'} cancelled=${cancelled} child-error=${childError} escalated=${escalated} group-kill-sent=${groupKillSent} unresolved=${unresolved} status=${status}`);
    clearTimeout(workTimer);
    clearTimeout(childTimer);
    for (const signal of signals) process.removeListener(signal, onSignal);
    // A missing close can strand the work await and keep owned handles alive.
    // This terminal branch is explicitly unresolved, never successful cleanup.
    if (unresolved) {
      console.log('invoked: a11y-audit native-status mode=plain status=2 unresolved=true');
      process.exit(2);
    }
  })();
  return shutdownPromise;
};

let summary;
try {
  endpoint = readHandEndpoint();
  if (MUTATE) {
    browser = endpoint ? await chromium.connect(endpoint, { timeout: 5000 }) : await chromium.launch({ channel: 'chrome', headless: true });
    await audit(browser, NOTE, 'light', true);
    console.log('caught: a11y-audit label');
    console.log('MUTATE-RESULT: caught drop-task-label');
    process.exitCode = 1;
    if (endpoint) {
      console.log('invoked: a11y-audit proof hand-canary-boundary');
    }
  } else {
    for (const signal of signals) process.on(signal, onSignal);
    workTimer = setTimeout(() => {
      cancelled = true;
      unavailable('managed plain work deadline exceeded');
      console.log('caught: a11y-audit operational work-deadline');
      if (!launchPending) void shutdownPlain();
    }, 300000);
    launchPending = true;
    try {
      server = await chromium.launchServer({ channel: 'chrome', headless: true, host: '127.0.0.1', timeout: 30000,
        handleSIGINT: false, handleSIGTERM: false, handleSIGHUP: false });
    } finally { launchPending = false; }
    endpoint = server.wsEndpoint();
    chrome = server.process();
    chromePID = chrome.pid;
    chrome.once('error', (error) => {
      unavailable(`owned Chrome process error: ${error.message}`);
      void shutdownPlain();
    });
    chromeClose = new Promise((resolve) => chrome.once('close', (code, signal) => {
      chromeClosed = true;
      console.log(`invoked: a11y-audit cleanup owned-chrome close status=${code ?? 'none'} signal=${signal ?? 'none'}`);
      resolve({ code, signal });
    }));
    if (!Number.isInteger(chromePID) || chromePID <= 0) throw new ProbeBroken('acquired Chrome has no valid PID');
    if (cancelled) throw new ProbeBroken('cancelled during browser launch');
    browser = await chromium.connect(endpoint, { timeout: 5000 });
    plainClientAcquired = true;
    let violations = 0;
    let incomplete = 0;
    let unavailableCount = 0;
    const versions = new Set();
    for (const theme of THEMES) {
      for (const path of PAGES) {
        if (cancelled) throw new ProbeBroken('ordinary audits cancelled');
        try {
          const result = await audit(browser, path, theme);
          violations += result.violations;
          incomplete += result.incomplete;
          versions.add(result.version);
        } catch (error) {
          unavailableCount += 1;
          unavailable(`page=${path} theme=${theme}: ${error.message}`);
        }
      }
    }
    console.log(`SUMMARY a11y-audit audits=6 violations=${violations} incomplete=${incomplete} unavailable=${unavailableCount}`);
    await browser.close();
    browser = undefined;
    if (cancelled || shutdownPromise) throw new ProbeBroken('hand child unavailable after cancellation/cleanup');
    const receipt = await runHandChild();
    try { assertHandReceipt(receipt, versions); } catch (error) {
      if (error instanceof LockFired) {
        console.error(`a11y-audit: ${redact(error.message)}`);
        console.log('caught: a11y-audit hand-mode contract');
        if (status !== 2) status = 1;
      } else throw error;
    }
    if (unavailableCount > 0) throw new ProbeBroken(`${unavailableCount} page/theme audits were unavailable`);
    if (violations > 0) throw new LockFired(`${violations} violations across six page/theme audits; incomplete=${incomplete}`);
    summary = { versions, incomplete };
  }
} catch (error) {
  console.error(`a11y-audit: ${redact(error.message)}`);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE || 'plain-canary'}`);
    status = 2;
  } else if (error instanceof LockFired) {
    console.log('caught: a11y-audit contract');
    if (status !== 2) status = 1;
  } else status = 2;
} finally {
  if (!MUTATE) await shutdownPlain();
  else {
    try { await browser?.close(); } catch (error) {
      console.error(`a11y-audit browser cleanup: ${redact(error.message)}`);
      status = 2;
    }
  }
}
if (!MUTATE && status === 0 && summary) {
  console.log(`PASS a11y-audit routes=${JSON.stringify(PAGES)} themes=${JSON.stringify(THEMES)} viewport=1280x900 version=${[...summary.versions].join(',')} runtime-ms=${Math.round(performance.now() - started)} violations=0 incomplete=${summary.incomplete} canary=label`);
}
console.log(`invoked: a11y-audit native-status mode=${MUTATE || 'plain'} status=${status !== 0 || !MUTATE ? status : process.exitCode ?? 0} unresolved=${unresolved}`);
if (status !== 0 || !MUTATE) process.exitCode = status;
if (unresolved) process.exit(2);
