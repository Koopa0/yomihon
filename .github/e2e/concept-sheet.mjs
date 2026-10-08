// The native concept dialog fills the phone and gives its controls the same
// reading inset and quiet feedback as the explanation. No mark is written:
// the unavailable read exercises visible feedback through the real module.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const ORIGIN = new URL(BASE).origin;
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const MODES = ['phone-keeps-browser-cap', 'control-outside-body', 'feedback-loses-style'];
const TARGETS = ['phone-width', 'control-inset', 'feedback-style'];
if (MUTATE === 'list') {
  for (const mode of MODES) console.log(mode);
  process.exit(0);
}
if (MUTATE && !MODES.includes(MUTATE)) {
  console.error(`concept-sheet: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class NotApplied extends Error {}
class ProbeBroken extends Error {}

// The served response is changed in memory. Every load has to name precisely
// one declaration or call, before a geometry failure can count as a catch.
const mutation = async (page) => {
  const counts = [];
  if (!MUTATE) return () => {};
  const script = MUTATE === 'control-outside-body';
  const assetPath = script ? '/static/uncertainty.js' : '/static/app.css';
  await page.route((url) => url.origin === ORIGIN && url.pathname === assetPath, async (route) => {
    const response = await route.fetch();
    if (response.status() !== 200) throw new ProbeBroken(`asset returned ${response.status()}`);
    let source = await response.text();
    if (script) {
      const needle = /addControl\(body, article,/g;
      counts.push([...source.matchAll(needle)].length);
      source = source.replace(needle, 'addControl(dialog, article,');
    } else if (MUTATE === 'phone-keeps-browser-cap') {
      const block = /@media\s*\(max-width:\s*720px\)\s*\{\s*\.y-conceptsheet\s*\{[^}]*\}/g;
      const matches = [...source.matchAll(block)];
      const cap = /max-width:\s*none\s*;/g;
      counts.push(matches.length === 1 ? [...matches[0][0].matchAll(cap)].length : matches.length);
      source = source.replace(block, (rule) => rule.replace(cap, ''));
    } else {
      const needle = /\.y-fileinfo__note\s*,\s*\.y-uncertainty__said(?=\s*\{)/g;
      counts.push([...source.matchAll(needle)].length);
      source = source.replace(needle, '.y-fileinfo__note');
    }
    await route.fulfill({ response, body: source });
  });
  return () => {
    if (counts.length === 0 || counts.some((count) => count !== 1)) {
      throw new NotApplied(`NOT-APPLIED concept-sheet: ${MUTATE} matched [${counts}], want exactly one on every load`);
    }
    console.log(`APPLIED concept-sheet: ${MUTATE}: [${counts}]`);
  };
};

const arrived = (page) => page.waitForFunction(async () => {
  if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
  await Promise.all(document.getAnimations()
    .filter((animation) => animation.animationName === 'y-come-forward')
    .map((animation) => animation.finished.catch(() => {})));
  return true;
}, null, { timeout: 3000 });

const settled = (page) => page.waitForFunction(async () => {
  const dialog = document.querySelector('[data-concept-sheet][open]');
  if (!dialog) return false;
  await Promise.all(dialog.getAnimations().map((animation) => animation.finished.catch(() => {})));
  return true;
}, null, { timeout: 3000 });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const failures = new Map();
let contexts = 0;
try {
  for (const width of [360, 390, 720, 721, 1280, 1440]) {
    for (const lang of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        const context = await browser.newContext({ viewport: { width, height: 844 } });
        await context.addCookies([
          { name: 'yomihon_lang', value: lang, url: BASE },
          { name: 'yomihon_theme', value: theme, url: BASE },
        ]);
        const page = await context.newPage();
        const applied = await mutation(page);
        let writes = 0;
        let reads = 0;
        await page.route('**/uncertainties', async (route) => {
          if (route.request().method() !== 'GET') writes++;
          else reads++;
          await route.fulfill({ status: 503, contentType: 'application/json', body: '{}' });
        });
        const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
        if (response.status() !== 200) throw new ProbeBroken(`page returned ${response.status()}`);
        await arrived(page);
        const trigger = page.locator('a[data-concept]').first();
        const sourceHref = await trigger.getAttribute('href');
        await trigger.click();
        await page.locator('[data-concept-sheet][open] .y-uncertainty__said').filter({ hasText: /\S/ }).waitFor();
        await settled(page);
        applied();
        const result = await page.evaluate(() => {
          const dialog = document.querySelector('[data-concept-sheet][open]');
          const body = dialog.querySelector('[data-concept-body]');
          const control = dialog.querySelector('[data-uncertainty-control]');
          const button = control.querySelector('button');
          const said = control.querySelector('.y-uncertainty__said');
          const scope = control.querySelector('.y-fileinfo__note');
          const d = dialog.getBoundingClientRect();
          const b = body.getBoundingClientRect();
          const style = getComputedStyle(body);
          const s = getComputedStyle(said);
          const n = getComputedStyle(scope);
          // The sheet docks to the box a fixed element fills, which is what a
          // scrollbar or a reserved gutter shrinks; a window width is not.
          const edge = document.createElement('div');
          edge.style.cssText = 'position:fixed;inset:0;visibility:hidden;pointer-events:none';
          document.body.append(edge);
          const block = edge.getBoundingClientRect();
          edge.remove();
          return {
            viewport: block.right, blockLeft: block.left,
            left: d.left, right: d.right, width: d.width,
            inset: b.left + Number.parseFloat(style.paddingLeft),
            buttonLeft: button.getBoundingClientRect().left,
            inside: body.contains(control),
            feedback: [s.color, s.fontFamily, s.fontSize, s.margin],
            scope: [n.color, n.fontFamily, n.fontSize, n.margin],
            disabled: button.disabled, status: said.getAttribute('role'),
            controls: dialog.querySelectorAll('[data-uncertainty-control]').length,
            theme: document.documentElement.dataset.theme,
            language: control.lang,
          };
        });
        if (result.theme !== theme || result.language !== lang || reads !== 1 || writes !== 0 || !result.disabled || result.status !== 'status' || result.controls !== 1) {
          throw new ProbeBroken(`fixture/feedback state: ${JSON.stringify(result)}, reads=${reads}, writes=${writes}`);
        }
        const label = `${width}/${lang}/${theme}`;
        const check = (site, okay, detail) => {
          if (!okay && !failures.has(site)) failures.set(site, `${label}: ${detail}`);
        };
        check('phone-width', width > 720
          ? Math.abs(result.width - 460) < 0.1 && Math.abs(result.right - result.viewport) < 0.1
          : Math.abs(result.left - result.blockLeft) < 0.1 && Math.abs(result.right - result.viewport) < 0.1,
        `sheet [${result.left}, ${result.right}] width=${result.width}, viewport=${result.viewport}`);
        check('control-inset', result.inside && Math.abs(result.buttonLeft - result.inset) < 0.1,
          `inside=${result.inside}, button left=${result.buttonLeft}, content left=${result.inset}`);
        check('feedback-style', result.feedback[2] === '13px' && JSON.stringify(result.feedback) === JSON.stringify(result.scope),
          `feedback=${JSON.stringify(result.feedback)}, scope=${JSON.stringify(result.scope)}`);
        await page.keyboard.press('Escape');
        await page.locator('[data-concept-sheet]').waitFor({ state: 'hidden' });
        if (await trigger.getAttribute('href') !== sourceHref) throw new ProbeBroken('concept link lost its native destination');
        await trigger.click();
        await settled(page);
        if (await page.locator('[data-concept-sheet][open] [data-uncertainty-control]').count() !== 1) {
          throw new ProbeBroken('reopening duplicated the control');
        }
        contexts++;
        await context.close();
      }
    }
  }
  console.log(`VISITED concept-sheet: ${contexts} contexts`);
  for (const [site, detail] of failures) console.error(`caught: concept-sheet ${site}: ${detail}`);
  if (failures.size > 0) {
    process.exitCode = 1;
    if (MUTATE && failures.size === 1 && failures.has(TARGETS[MODES.indexOf(MUTATE)])) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
  } else {
    console.log(`PASS concept-sheet: ${contexts} width/language/theme contexts, full phone width, inset controls, styled unavailable feedback, reopen without duplication; no writes`);
  }
} catch (error) {
  console.error(error);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else process.exitCode = 1;
} finally {
  await browser.close();
}
