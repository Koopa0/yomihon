// The three in-place action replies share the flip receipt's presentation.
// The real flip receipt is covered by flip-receipt-contract.mjs. Here marks
// receive mocked HTTP outcomes and preferences use the existing cookie-refusal
// stand-in: no mark or status write reaches the fixture server.
// These declarations are stable after arrival; no assertion samples a frame
// during the entrance, or claims to measure a screen reader announcement.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const NOTE = '/notes/Notes/reading-fidelity.md';
const SURFACES = [
  { name: 'rail', path: NOTE, width: 1600, control: '.y-markset', reply: '.y-markset__said' },
  { name: 'header', path: NOTE, width: 390, control: '.y-headermark', reply: '.y-headermark__said' },
  { name: 'preference', path: '/preferences', width: 1280, control: '[data-pref-field="theme"]', reply: '.y-preffield__status' },
];
const SITES = SURFACES.flatMap(({ name }) => ['empty', 'entrance', 'refused', 'reduce'].map((claim) => `${name}-${claim}`));
SITES.push('rail-kept', 'header-kept', 'mark-scoped');

class LockFired extends Error {
  constructor(site, message) { super(`FAIL reply-contract: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const check = (condition, site, message) => {
  if (!SITES.includes(site)) throw new Error(`unknown assertion site ${site}`);
  if (!condition) throw new LockFired(site, message);
};

// Product selector names are the needles, with optional minifier whitespace
// and quotes. Every response must contain exactly one intended edit site.
const EMPTY_RULE = /\.y-reply:empty\s*\{/g;
const ENTRANCE_RULE = /\.y-reply:not\(:empty\)\s*\{/g;
const REFUSED_RULE = /\.y-reply\[data-reply-tone\s*=\s*["']?refused["']?\]\s*\{/g;
const MUTATIONS = {};
for (const { name } of SURFACES) {
  MUTATIONS[`hide-empty-${name}`] = { target: `${name}-empty`, asset: '**/static/app.css', needle: EMPTY_RULE, replacement: '$&display:none!important;' };
  MUTATIONS[`drop-entrance-${name}`] = { target: `${name}-entrance`, asset: '**/static/app.css', needle: ENTRANCE_RULE, replacement: '$&animation:none!important;' };
  MUTATIONS[`mute-refusal-${name}`] = { target: `${name}-refused`, asset: '**/static/app.css', needle: REFUSED_RULE, replacement: '$&color:var(--fg-muted)!important;' };
  // Outrank the later .yomihon *:not(.y-readline) reduced-motion blanket;
  // equal specificity would let that guard repair the injected regression.
  MUTATIONS[`keep-motion-${name}`] = { target: `${name}-reduce`, asset: '**/static/app.css', needle: ENTRANCE_RULE, replacement: '.y-reply.y-reply:not(:empty){animation-duration:1s!important;' };
  if (name !== 'preference') {
    MUTATIONS[`refuse-kept-${name}`] = {
      target: `${name}-kept`, asset: '**/mark.js',
      needle: /said\.dataset\.replyTone = kept \? 'kept' : 'refused';/g,
      replacement: "said.dataset.replyTone = 'refused';",
    };
  }
}
MUTATIONS['broadcast-mark-reply'] = {
  target: 'mark-scoped', asset: '**/mark.js',
  needle: /said\.textContent = words \?\? '';/g,
  replacement: "document.querySelectorAll('[data-mark-said]').forEach((line) => { line.textContent = words ?? ''; });",
};
for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) throw new Error(`unknown site for ${name}`);
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) throw new Error(`no mutation for ${site}`);
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) process.exit(2);

let proof = null;
async function arm(page, surface) {
  proof = null;
  if (!MUTATE) return;
  const mode = MUTATIONS[MUTATE];
  if (!mode.target.startsWith(`${surface}-`) && !(surface === 'rail' && mode.target === 'mark-scoped')) return;
  const counts = [];
  await page.route(mode.asset, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const count = [...original.matchAll(mode.needle)].length;
    counts.push(count);
    await route.fulfill({ response, body: count === 1 ? original.replace(mode.needle, mode.replacement) : original });
  });
  proof = () => counts.length > 0 && counts.every((count) => count === 1);
}
function prove() {
  if (proof && !proof()) throw new NotApplied('mutation must match exactly one product declaration in every response');
}

const read = (reply) => reply.evaluate((element) => {
  const style = getComputedStyle(element);
  // Resolve the current theme's own ink tokens, independent of their color
  // syntax, so this checks tone ownership without freezing a palette value.
  const swatch = document.createElement('span');
  document.body.append(swatch);
  swatch.style.color = 'var(--fg-muted)';
  const muted = getComputedStyle(swatch).color;
  swatch.style.color = 'var(--warn)';
  const warning = getComputedStyle(swatch).color;
  swatch.remove();
  return {
    shared: element.classList.contains('y-reply'), role: element.getAttribute('role'),
    live: element.getAttribute('aria-live'), atomic: element.getAttribute('aria-atomic'),
    text: element.textContent, tone: element.dataset.replyTone,
    color: style.color, muted, warning, display: style.display,
    margin: style.margin, height: element.getBoundingClientRect().height,
    animation: style.animationName, duration: Number.parseFloat(style.animationDuration),
  };
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const surface of SURFACES) {
    const { name } = surface;
    const context = await browser.newContext({ viewport: { width: surface.width, height: 900 }, reducedMotion: 'no-preference' });
    if (name === 'preference') {
      // The same silent-refusal model used by preference-immediate.mjs.
      await context.addInitScript(() => {
        const own = Object.getOwnPropertyDescriptor(Document.prototype, 'cookie');
        Object.defineProperty(document, 'cookie', {
          configurable: true,
          get() { return own.get.call(document); },
          set() { /* This stand-in silently refuses the write. */ },
        });
      });
    }
    const page = await context.newPage();
    await arm(page, name);
    let kept = true;
    let posts = 0;
    if (name !== 'preference') {
      await page.route('**/marks', async (route) => {
        if (route.request().method() !== 'POST') return route.continue();
        posts += 1;
        await route.fulfill({ status: kept ? 204 : 503, body: '' });
      });
    }
    const response = await page.goto(BASE + surface.path, { waitUntil: 'load' });
    if (response?.status() !== 200) throw new Error(`${surface.path} did not return 200`);
    prove();
    if (name === 'header') await page.locator('[popovertarget="_y-header-fold"]').click();
    const control = page.locator(surface.control);
    const reply = control.locator(surface.reply);
    if (await control.count() !== 1 || await reply.count() !== 1) throw new Error(`${name}: fixture needs one control and one reply`);
    const empty = await read(reply);
    check(empty.shared && empty.role === 'status' && empty.live === 'polite' && empty.atomic === 'true'
      && empty.text === '' && empty.display !== 'none' && empty.margin === '0px' && empty.height === 0,
    `${name}-empty`, `${name}: empty reply lost its live region or takes space: ${JSON.stringify(empty)}`);

    if (name === 'preference') {
      await control.locator('input[value="dark"]').click();
      await page.waitForFunction(() => {
        const line = document.querySelector('[data-pref-field="theme"] [data-pref-status]');
        return line?.textContent === line?.dataset.prefRefused && !!line?.textContent;
      });
    } else {
      await control.locator('[data-mark-button]').click();
      await page.waitForFunction((selector) => {
        const owner = document.querySelector(selector);
        return owner?.querySelector('[data-mark-said]')?.textContent === owner?.dataset.markSaved;
      }, surface.control);
      if (posts !== 1) throw new Error(`${name}: expected one intercepted mark request, got ${posts}`);
      const saved = await read(reply);
      check(saved.tone === 'kept' && saved.color === saved.muted, `${name}-kept`, `${name}: kept mark has tone=${saved.tone}, color=${saved.color}`);
      if (name === 'rail') {
        const untouched = await page.locator('.y-headermark__said').textContent();
        check(untouched === '', 'mark-scoped', 'the rail action also filled the untouched header reply');
      }
    }
    const arrival = await read(reply);
    check(arrival.animation === 'y-reply-in' && arrival.duration > 0,
      `${name}-entrance`, `${name}: entrance is ${arrival.animation} (${arrival.duration}s)`);

    if (name !== 'preference') {
      kept = false;
      await control.locator('[data-mark-button]').click();
      await page.waitForFunction((selector) => {
        const owner = document.querySelector(selector);
        return owner?.querySelector('[data-mark-said]')?.textContent === owner?.dataset.markFailed;
      }, surface.control);
      if (posts !== 2) throw new Error(`${name}: expected two intercepted mark requests, got ${posts}`);
    }
    const refused = await read(reply);
    check(refused.tone === 'refused' && refused.color === refused.warning,
      `${name}-refused`, `${name}: refused action has tone=${refused.tone}, color=${refused.color}`);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    const reduced = await read(reply);
    check(reduced.duration <= 0.001, `${name}-reduce`, `${name}: entrance lasts ${reduced.duration}s under reduced motion`);
    await context.close();
  }
  console.log('PASS reply-contract: rail, header and preference replies share entrance, tone and reduced motion; empty regions remain and mark updates stay local');
} catch (error) {
  console.error(error);
  if (error instanceof NotApplied || (error instanceof LockFired && proof && !proof())) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (error instanceof LockFired && MUTATE && proof && proof() && error.site === MUTATIONS[MUTATE].target) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
