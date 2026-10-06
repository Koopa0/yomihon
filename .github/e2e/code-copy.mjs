// The copied value is authored here independently of the DOM. A real browser
// clipboard round trip protects whitespace that a visual comparison cannot.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9934';
const MODE = process.env.MUTATE || '';
const PROGRAM = 'package main\n\nimport "fmt"\n\nfunc main() {\n\tch := make(chan string, 1)\n\tch <- "雪 & <tag>"\n\n\tfmt.Println(<-ch)\n}\n';
const PLAIN = 'plain\t<-\n\nlast line\n';
const EMBEDDED = 'embedded <-\n\tindent\n';
const DOCUMENT = 'document <-\n\tindent\n';
const CONCEPT = 'concept <-\n\tindent\n';
const OPENING = 'opening <-\n\tindent\n';
const HOST = '/notes/Notes/code-copy.md';
const LESSON = '/notes/Writing/lessons/japanese/code-copy-lesson.md';
const READINGS = [
  [HOST, [PROGRAM, PLAIN, EMBEDDED]],
  ['/compare/Notes/code-copy.md?with=Notes%2Fcode-copy-embedded.md', [PROGRAM, PLAIN, EMBEDDED, EMBEDDED]],
  ['/notes/Notes/code-copy-embedded.md', [EMBEDDED]],
  ['/notes/Concepts/japanese/code-copy-concept.md', [CONCEPT]],
  ['/notes/Examples/code-copy.go', [PROGRAM]],
  ['/notes/Examples/code-copy-document.md', [DOCUMENT]],
  ['/syllabus/Maps/code-copy-path.md', [OPENING]],
];
const MUTATIONS = {
  'flatten-newlines': {
    path: '/static/codecopy.js',
    needle: 'await navigator.clipboard.writeText(text.textContent);',
    replacement: "await navigator.clipboard.writeText(text.textContent.replaceAll('\\n', ' '));",
    site: 'clipboard-literal',
  },
  'trim-whitespace': {
    path: '/static/codecopy.js',
    needle: 'await navigator.clipboard.writeText(text.textContent);',
    replacement: 'await navigator.clipboard.writeText(text.textContent.trim());',
    site: 'clipboard-literal',
  },
  'omit-concept-enhancement': {
    path: '/static/lesson.js',
    needle: 'enhanceCodeCopy(body);',
    replacement: 'void body;',
    site: 'concept-controls',
  },
  'duplicate-controls': {
    path: '/static/codecopy.js',
    needle: "block.previousElementSibling?.matches('.y-codecopy')",
    replacement: 'false',
    site: 'idempotent-controls',
  },
  'report-denial-as-success': {
    path: '/static/codecopy.js',
    needle: 'reply.textContent = button.dataset.codecopyFailed;',
    replacement: 'reply.textContent = button.dataset.codecopyCopied;',
    site: 'manual-fallback',
  },
};
if (MODE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}
if (MODE && !Object.hasOwn(MUTATIONS, MODE)) {
  console.error(`not-applied: unknown code-copy mutation ${MODE}`);
  process.exit(2);
}
class Caught extends Error {
  constructor(site, message) { super(message); this.site = site; }
}
class NotApplied extends Error {}
function check(ok, site, message) {
  if (!ok) throw new Caught(site, message);
}
const hits = [];
const writes = [];
let invoked = 0;
function qualify() {
  if (MODE && (hits.length === 0 || hits.some(n => n !== 1))) {
    throw new NotApplied(`mutation matched [${hits}] across ${hits.length} responses; want exactly one site in each`);
  }
}
async function instrument(page) {
  await page.route('**/*', async route => {
    if (route.request().method() !== 'GET') {
      writes.push(route.request().method());
      await route.abort();
      return;
    }
    const mutation = MUTATIONS[MODE];
    const url = new URL(route.request().url());
    if (mutation && url.origin === new URL(BASE).origin && url.pathname === mutation.path) {
      const response = await route.fetch();
      const source = await response.text();
      const count = source.split(mutation.needle).length - 1;
      hits.push(count);
      console.log(`applied: ${MODE} sites=${count}`);
      await route.fulfill({ response, body: count === 1 ? source.replace(mutation.needle, mutation.replacement) : source });
      return;
    }
    await route.continue();
  });
}
async function arrived(page, path) {
  const response = await page.goto(BASE + path, { waitUntil: 'networkidle' });
  check(response?.status() === 200, 'http', `${path}: HTTP ${response?.status()}`);
  qualify();
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(document.getAnimations().filter(a => a.animationName === 'y-come-forward').map(a => a.finished));
  });
}
async function controls(page, expected, site = 'reading-controls') {
  console.log(`invoked: ${site} expected=${expected.length}`);
  const blocks = page.locator('.y-prose pre');
  check(await blocks.count() === expected.length, site, `readable pre count differs from ${expected.length}`);
  const text = await blocks.allTextContents();
  check(JSON.stringify(text) === JSON.stringify(expected), 'rendered-literal', 'rendered text differs from independent authored literals');
  const buttons = page.locator('.y-codecopy [data-codecopy-button]');
  check(await buttons.count() === expected.length, site, `copy controls ${await buttons.count()}, want ${expected.length}`);
  check(await page.locator('pre button').count() === 0, site, 'controls polluted a code block');
  for (let i = 0; i < expected.length; i += 1) {
    const button = buttons.nth(i);
    check(await button.isVisible(), site, 'copy button is invisible');
    await button.focus();
    await button.press(i % 2 ? 'Space' : 'Enter');
    invoked += 1;
    console.log(`invoked: clipboard-literal ${invoked}`);
    const reply = button.locator('..').locator('[data-codecopy-reply]');
    await reply.filter({ hasText: /Copied\.|已複製。/ }).waitFor();
    const value = await page.evaluate(() => navigator.clipboard.readText());
    check(value === expected[i], 'clipboard-literal', `clipboard differs from authored literal at block ${i}`);
    check(await reply.getAttribute('role') === 'status' && await reply.getAttribute('aria-live') === 'polite' && await reply.getAttribute('aria-atomic') === 'true', 'reply', 'reply lost canonical live semantics');
    check(await button.isEnabled(), 'button-restored', 'copy button stayed disabled');
  }
}
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let exit = 0;
try {
  const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
  const page = await context.newPage();
  await instrument(page);
  for (const [path, expected] of READINGS) {
    await arrived(page, path);
    await controls(page, expected);
  }
  await arrived(page, HOST);
  await page.evaluate(async () => {
    const { initCodeCopy } = await import('/static/codecopy.js');
    const enhance = initCodeCopy();
    enhance(document);
    enhance(document);
  });
  invoked += 1;
  console.log('invoked: idempotent-controls');
  check(await page.locator('.y-codecopy [data-codecopy-button]').count() === 3, 'idempotent-controls', 'repeat init or enhancement duplicated controls');
  await page.emulateMedia({ media: 'print' });
  check((await page.locator('.y-codecopy').evaluateAll(nodes => nodes.map(n => getComputedStyle(n).display))).every(display => display === 'none'), 'print', 'copy controls appear in print');
  await page.emulateMedia({ media: 'screen' });
  await arrived(page, LESSON);
  const trigger = page.locator('a[data-concept]').first();
  check(await trigger.count() === 1, 'concept-controls', 'lesson fixture has no concept trigger');
  for (let opening = 0; opening < 2; opening += 1) {
    await trigger.click();
    invoked += 1;
    console.log(`invoked: concept-controls ${opening}`);
    await controls(page, [CONCEPT], 'concept-controls');
    await page.locator('[data-concept-sheet] button[command="close"]').click();
  }
  await context.close();
  for (const failure of ['denied', 'absent']) {
    const fallback = await browser.newContext();
    const p = await fallback.newPage();
    await instrument(p);
    await p.addInitScript(kind => {
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: kind === 'absent' ? undefined : { writeText: () => Promise.reject(new DOMException('denied', 'NotAllowedError')) } });
    }, failure);
    await arrived(p, HOST);
    await p.locator('[data-codecopy-button]').first().click();
    invoked += 1;
    console.log(`invoked: manual-fallback ${failure}`);
    const reply = p.locator('.y-codecopy [data-codecopy-reply]').first();
    await reply.filter({ hasText: /手動複製|copy it manually/ }).waitFor({ timeout: 3000 }).catch(() => {
      throw new Caught('manual-fallback', 'denied or absent Clipboard API reported no manual fallback');
    });
    const selected = await p.evaluate(() => { const s = getSelection(); return s?.rangeCount === 1 ? s.getRangeAt(0).toString() : null; });
    check(selected === PROGRAM, 'manual-fallback', `manual fallback selected different code: ${JSON.stringify(selected)}`);
    check(await p.locator('[data-codecopy-button]').first().isEnabled(), 'manual-fallback', 'fallback left button disabled');
    await fallback.close();
  }
  for (const baseline of ['no-js', 'boot-failed']) {
    const c = await browser.newContext({ javaScriptEnabled: baseline !== 'no-js' });
    const p = await c.newPage();
    if (baseline === 'boot-failed') await p.route('**/yomihon.js{,?*}', route => route.abort());
    await arrived(p, HOST);
    check(await p.locator('.y-codecopy').count() === 0, 'baseline', 'inactive scripting exposed misleading copy controls');
    check(JSON.stringify(await p.locator('.y-prose pre').allTextContents()) === JSON.stringify([PROGRAM, PLAIN, EMBEDDED]), 'baseline', 'manual copy text disappeared without enhancement');
    await c.close();
  }
  check(writes.length === 0, 'no-writes', `unexpected non-GET requests ${writes}`);
  qualify();
  check(invoked > 0, 'invocation', 'no clipboard or enhancement boundary was traversed');
  if (MODE) throw new Error('mutation escaped its designated oracle');
  console.log(`PASS code-copy: ${invoked} invocation boundaries, whole reading set, real headless browser clipboard, no server writes`);
} catch (error) {
  try {
    qualify();
    if (error instanceof NotApplied) throw error;
    if (MODE && (!(error instanceof Caught) || error.site !== MUTATIONS[MODE].site)) throw new NotApplied(`wrong failure boundary: ${error.message}`);
    if (error instanceof Caught) {
      console.error(`caught: code-copy ${error.site}: ${error.message}`);
      if (MODE) console.log(`MUTATE-RESULT: caught ${MODE}`);
      exit = 1;
    } else {
      throw error;
    }
  } catch (failure) {
    console.error(`not-applied: code-copy ${failure.message}`);
    if (MODE) console.log(`MUTATE-RESULT: not-applied ${MODE}`);
    exit = 2;
  }
} finally {
  await browser.close();
}
process.exit(exit);
