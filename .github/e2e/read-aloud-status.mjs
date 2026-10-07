// A practice card with no marked paragraph still owns speech feedback. Capture
// the real utterance at the synthesis boundary, then deliver its error event.
// This exercises browser callbacks, not audible output or screen-reader speech.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/Practice%20only.md';
const MUTATE = process.env.MUTATE || '';
const MODE = 'move-region-below-paragraph-return';
const STATUS = '.y-ttsbar__status';
const CARD = '[data-slot-action="speak"]';
const UNAVAILABLE = { 'zh-Hant': '目前無法播放語音', en: 'Speech is unavailable right now' };
class LockFired extends Error {
  constructor(site, message) { super(`FAIL read-aloud-status [${site}]: ${message}`); this.site = site; }
}
class Broken extends Error {}
const check = (condition, site, message) => { if (!condition) throw new LockFired(site, message); };
const setup = (condition, message) => { if (!condition) throw new Broken(message); };
if (MUTATE === 'list') { console.log(MODE); process.exit(0); }
if (MUTATE && MUTATE !== MODE) { console.error(`unknown MUTATE ${MUTATE}`); process.exit(2); }

const REGION = "    if (column?.querySelector('[data-tts], [data-slot-action=\"speak\"]')) {\n" +
  "      speechStatus = document.createElement('span');\n" +
  "      speechStatus.className = 'y-ttsbar__status';\n" +
  "      speechStatus.setAttribute('aria-live', 'polite');\n" +
  "      column.append(speechStatus);\n    }\n";
const RETURN = '    if (readingButtons.length === 0) return;\n';
let applied = false;
let hit = false;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const width of [1280, 390]) {
    for (const language of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        const context = await browser.newContext({ viewport: { width, height: 800 } });
        await context.addCookies([
          { name: 'yomihon_lang', value: language, url: BASE },
          { name: 'yomihon_theme', value: theme, url: BASE },
        ]);
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', (error) => errors.push(error.message));
        await page.addInitScript(() => {
          window.__utterances = [];
          window.__refusals = 0;
          speechSynthesis.speak = (utterance) => {
            window.__utterances.push(utterance);
            utterance.dispatchEvent(new Event('start'));
          };
          speechSynthesis.cancel = () => {};
          document.addEventListener('DOMContentLoaded', () => {
            window.__authoredText = () => {
              const article = document.querySelector('article')?.cloneNode(true);
              article?.querySelectorAll('.y-ttsbar__status').forEach((status) => status.remove());
              return article?.textContent ?? null;
            };
            new MutationObserver((records) => {
              for (const record of records) {
                if (record.target.matches?.('.y-ttsbar__status') &&
                    ['目前無法播放語音', 'Speech is unavailable right now'].includes(record.target.textContent)) {
                  window.__refusals += 1;
                }
              }
            }).observe(document.body, { childList: true, subtree: true });
          });
        });
        let requests = 0;
        let matches = [];
        if (MUTATE) {
          await page.route('**/lesson.js{,?*}', async (route) => {
            const response = await route.fetch();
            const original = await response.text();
            requests += 1;
            matches = [REGION, RETURN].map((needle) => original.split(needle).length - 1);
            const body = matches.every((count) => count === 1)
              ? original.replace(REGION, '').replace(RETURN, RETURN + REGION) : original;
            await route.fulfill({ response, body });
          });
        }
        const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
        setup(response?.status() === 200, `practice fixture returned ${response?.status()}`);
        setup(errors.length === 0, `runtime errors: ${errors.join('; ')}`);
        if (MUTATE) {
          setup(requests === 1 && matches.every((count) => count === 1), `not-applied requests=${requests} matches=${JSON.stringify(matches)}`);
          applied = true;
        }
        setup(await page.locator(CARD).count() === 1 && await page.locator('[data-tts]').count() === 0, 'practice fixture must have one card and no paragraph speaker');
        const before = await page.evaluate(() => ({
          text: window.__authoredText(),
          idle: document.querySelector('[data-slot-action="speak"]')?.getAttribute('aria-label'),
          region: document.querySelector('.y-ttsbar__status')?.textContent ?? null,
        }));
        await page.click(CARD);
        const handoff = await page.evaluate(() => window.__utterances.map((utterance) => ({ text: utterance.text, lang: utterance.lang })));
        setup(handoff.length === 1 && handoff[0].text === 'わたし' && handoff[0].lang === 'ja', `wrong handoff ${JSON.stringify(handoff)}`);
        await page.evaluate(() => window.__utterances[0].dispatchEvent(new Event('error')));
        hit = true;
        console.log(`INVOCATION-HIT read-aloud-status ${width}/${language}/${theme}: captured utterance error delivered`);
        const refusal = await page.locator(STATUS).allTextContents();
        check(refusal.length === 1 && refusal[0] === UNAVAILABLE[language], 'practice-unavailable', `${width}/${language}/${theme} practice refusal ${JSON.stringify(refusal)}, want ${JSON.stringify(UNAVAILABLE[language])}`);
        check(before.region === '', 'initial-region', 'the shared region must exist empty before pressing');
        check(await page.locator('.y-ttsbar').count() === 0, 'practice-toolbar', 'practice-only lesson acquired a paragraph toolbar');
        const first = await page.evaluate(() => ({
          refusals: window.__refusals,
          speaking: document.querySelectorAll('[data-speaking], [data-reading]').length,
          label: document.querySelector('[data-slot-action="speak"]')?.getAttribute('aria-label'),
          language: document.querySelector('.y-ttsbar__status')?.closest('[lang]')?.lang,
          text: window.__authoredText(),
        }));
        check(first.refusals === 1 && first.speaking === 0 && first.label === before.idle, 'refusal-recovery', `first refusal did not recover: ${JSON.stringify(first)}`);
        check(first.language === language, 'region-language', `status inherited ${first.language}, want ${language}`);
        check(first.text === before.text, 'authored-content', 'speech refusal changed authored article text');
        await page.evaluate(() => window.__utterances[0].dispatchEvent(new Event('error')));
        check(await page.evaluate(() => window.__refusals) === 1, 'double-error', 'repeated error announced twice');
        await page.click(CARD);
        await page.click(CARD); // Stop the active generation.
        await page.click(CARD); // Replace it with a later generation.
        const current = await page.locator(STATUS).textContent();
        await page.evaluate(() => window.__utterances[1].dispatchEvent(new Event('error')));
        check(await page.locator(STATUS).textContent() === current && await page.locator('[data-speaking]').count() === 1, 'stale-error', 'stale error altered the later generation');
        await page.evaluate(() => window.__utterances[2].dispatchEvent(new Event('error')));
        check(await page.locator(STATUS).textContent() === UNAVAILABLE[language] && await page.evaluate(() => window.__refusals) === 2, 'later-refusal', 'later generation did not announce its refusal once');
        setup(errors.length === 0, `runtime errors after speech: ${errors.join('; ')}`);
        await context.close();
      }
    }
  }
  console.log('PASS read-aloud-status: practice refusal and generation recovery in both languages, themes and viewports');
} catch (error) {
  console.error(error.message);
  if (error instanceof LockFired && error.site === 'practice-unavailable' && hit && (!MUTATE || applied)) {
    console.log('caught: read-aloud-status practice-unavailable');
  }
  if (MUTATE && error instanceof LockFired && error.site === 'practice-unavailable' && applied && hit) {
    console.log(`MUTATE-RESULT: caught ${MODE}`);
    process.exitCode = 1;
  } else {
    if (MUTATE) console.log(`MUTATE-RESULT: not-applied ${MODE}`);
    process.exitCode = error instanceof LockFired && !MUTATE ? 1 : 2;
  }
} finally {
  await browser.close();
}
