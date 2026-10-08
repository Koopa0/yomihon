// A practice card with no marked paragraph still owns speech feedback. Exercise
// a missing local voice and the error callback of a captured real utterance.
// This exercises browser callbacks, not audible output or screen-reader speech.
import { chromium } from 'playwright-core';
import { installSpeechVoices } from './support/speech-voices.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/Practice%20only.md';
const MUTATE = process.env.MUTATE || '';
const MODE = 'move-region-below-paragraph-return';
const LANGUAGE_MODE = 'inherit-authored-language';
const NULL_MODE = 'append-null-status';
const MODES = [MODE, LANGUAGE_MODE, NULL_MODE];
const NULL_GUARD = '    if (speechStatus) toolbar.append(speechStatus);\n';
const LANGUAGE = "      speechStatus.setAttribute('lang', document.documentElement.lang);\n";
const STATUS = '.y-ttsbar__status';
const CARD = '[data-slot-action="speak"]';
const UNAVAILABLE = { 'zh-Hant': '目前無法播放語音', en: 'Speech is unavailable right now' };
const COMPOSITIONS = [
  { name: 'mixed', path: '/notes/Writing/lessons/japanese/L01.md', paragraphs: 1, cards: 1 },
  { name: 'paragraph', path: '/notes/Writing/lessons/japanese/L02.md', paragraphs: 3, cards: 0 },
  { name: 'listen', path: '/listen/Maps/listen.md', paragraphs: 4, cards: 0 },
  { name: 'no-controls', path: '/notes/Notes/alpha.md', paragraphs: 0, cards: 0 },
  { name: 'practice', path: PAGE, paragraphs: 0, cards: 1 },
];
class LockFired extends Error {
  constructor(site, message) { super(`caught: read-aloud-status [${site}]: ${message}`); this.site = site; }
}
class Broken extends Error {}
const check = (condition, site, message) => { if (!condition) throw new LockFired(site, message); };
const setup = (condition, message) => { if (!condition) throw new Broken(message); };
if (MUTATE === 'list') { console.log(MODES.join('\n')); process.exit(0); }
if (MUTATE && !MODES.includes(MUTATE)) { console.error(`unknown MUTATE ${MUTATE}`); process.exit(2); }

const REGION = "    if (column?.querySelector('[data-tts], [data-slot-action=\"speak\"]')) {\n" +
  "      speechStatus = document.createElement('span');\n" +
  "      speechStatus.className = 'y-ttsbar__status';\n" +
  "      speechStatus.setAttribute('aria-live', 'polite');\n" +
  LANGUAGE + "      column.append(speechStatus);\n    }\n";
const RETURN = '    if (readingButtons.length === 0) return;\n';
let applied = false;
let hit = false;
async function waitForHandoff(page, count) {
  await page.waitForFunction((expected) => window.__speechFixture.utterances.length === expected, count);
  // The fixture schedules the actual start event after the handoff.
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
}
async function pressForHandoff(page, trigger) {
  const count = await page.evaluate(() => window.__speechFixture.utterances.length);
  await (typeof trigger === 'string' ? page.locator(trigger) : trigger).click();
  await waitForHandoff(page, count + 1);
}
async function observePractice(page) {
  await page.addInitScript(() => {
    window.__refusals = 0;
    document.addEventListener('DOMContentLoaded', () => {
      window.__authoredText = () => {
        const article = document.querySelector('article')?.cloneNode(true);
        article?.querySelectorAll('.y-ttsbar__status').forEach((status) => { status.remove(); });
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
}
async function documentWithoutColumn(browser, width, language, theme) {
  const context = await browser.newContext({ viewport: { width, height: 800 } });
  try {
    await context.addCookies([
      { name: 'yomihon_lang', value: language, url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
    ]);
    const page = await context.newPage();
    await installSpeechVoices(page, { record: true });
    let requests = 0;
    let matches = 0;
    if (MUTATE === NULL_MODE) {
      await page.route('**/lesson.js{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        requests += 1;
        matches = original.split(NULL_GUARD).length - 1;
        await route.fulfill({ response, body: matches === 1
          ? original.replace(NULL_GUARD, '    toolbar.append(speechStatus);\n') : original });
      });
    }
    const response = await page.goto(BASE + '/notes/README.md', { waitUntil: 'networkidle' });
    const paragraphs = await page.locator('[data-tts]').count();
    const columns = await page.locator('[data-readaloud-controls]').count();
    setup(response?.status() === 200 && paragraphs === 1 && columns === 0,
      `document fixture status=${response?.status()} paragraphs=${paragraphs} columns=${columns}; want 200/1/0`);
    if (MUTATE === NULL_MODE) {
      setup(requests === 1 && matches === 1, `not-applied requests=${requests} matches=${matches}`);
      applied = true;
      console.log(`MUTATE-APPLIED: ${NULL_MODE} requests=1 matches=1`);
    }
    const text = await page.locator('.y-ttsbar').textContent();
    hit = true;
    check(!text.includes('null'), 'document-status', `${width}/${language}/${theme} document toolbar contains null: ${text}`);
  } finally {
    await context.close();
  }
}
async function noLocalVoice(browser, width, language, theme) {
  const context = await browser.newContext({ viewport: { width, height: 800 } });
  try {
    await context.addCookies([
      { name: 'yomihon_lang', value: language, url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
    ]);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await installSpeechVoices(page, {
      voices: [{ name: 'English device voice', lang: 'en-US', localService: true, default: true }],
      record: true,
    });
    await observePractice(page);
    let requests = 0;
    let matches = [];
    if (MUTATE === MODE) {
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
    setup(response?.status() === 200 && errors.length === 0, `no-local-voice setup: ${response?.status()} ${errors.join('; ')}`);
    if (MUTATE === MODE) {
      setup(requests === 1 && matches.every((count) => count === 1), `not-applied requests=${requests} matches=${JSON.stringify(matches)}`);
      applied = true;
      console.log(`MUTATE-APPLIED: ${MODE} requests=1 matches=[1,1]`);
    }
    setup(await page.locator(CARD).count() === 1 && await page.locator('[data-tts]').count() === 0,
      'no-local-voice fixture must have one card and no paragraph speaker');
    const before = await page.evaluate(() => ({
      text: window.__authoredText(),
      idle: document.querySelector('[data-slot-action="speak"]').getAttribute('aria-label'),
      region: document.querySelector('.y-ttsbar__status')?.textContent ?? null,
    }));
    await page.click(CARD);
    await page.waitForFunction(() => window.__speechFixture.reads > 0 && !document.querySelector('[data-speaking]'));
    hit = true;
    console.log(`INVOCATION-HIT read-aloud-status no-local-voice ${width}/${language}/${theme}: card press and voice lookup completed`);
    const refusal = await page.locator(STATUS).allTextContents();
    check(refusal.length === 1 && refusal[0] === UNAVAILABLE[language], 'practice-no-local-voice',
      `${width}/${language}/${theme} no-local-voice refusal ${JSON.stringify(refusal)}, want ${JSON.stringify(UNAVAILABLE[language])}`);
    const after = await page.evaluate(() => ({
      refusals: window.__refusals,
      receipts: window.__speechFixture.receipts,
      speaking: document.querySelectorAll('[data-speaking], [data-reading]').length,
      label: document.querySelector('[data-slot-action="speak"]').getAttribute('aria-label'),
      language: document.querySelector('.y-ttsbar__status').closest('[lang]')?.lang,
      text: window.__authoredText(),
    }));
    check(before.region === '' && after.refusals === 1 && after.receipts.length === 0 && after.speaking === 0
      && after.label === before.idle && after.language === language && after.text === before.text,
      'no-local-voice-recovery', `no-local-voice state ${JSON.stringify(after)}`);
    check(await page.locator('.y-ttsbar').count() === 0, 'no-local-voice-toolbar', 'practice-only refusal acquired a paragraph toolbar');
    setup(errors.length === 0, `no-local-voice runtime errors: ${errors.join('; ')}`);
  } finally {
    await context.close();
  }
}
async function composition(browser, width, language, theme, fixture, noAPI) {
  const context = await browser.newContext({ viewport: { width, height: 800 } });
  try {
    await context.addCookies([
      { name: 'yomihon_lang', value: language, url: BASE },
      { name: 'yomihon_theme', value: theme, url: BASE },
    ]);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    if (noAPI) {
      await page.addInitScript(() => {
        window.__speechFixture = { utterances: [] };
        if (!Reflect.deleteProperty(window, 'speechSynthesis') || 'speechSynthesis' in window) {
          throw new Error('cannot remove speech API for control');
        }
      });
    } else {
      await installSpeechVoices(page, { record: true });
    }
    let requests = 0;
    let matches = 0;
    if (MUTATE === LANGUAGE_MODE) {
      await page.route('**/lesson.js{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        requests += 1;
        matches += original.split(LANGUAGE).length - 1;
        await route.fulfill({ response, body: matches === 1 ? original.replace(LANGUAGE, '') : original });
      });
    }
    const response = await page.goto(BASE + fixture.path, { waitUntil: 'networkidle' });
    const identity = `${fixture.name}/${width}/${language}/${theme}/${noAPI ? 'no-api' : 'api'}`;
    setup(response?.status() === 200 && errors.length === 0, `${identity} setup: ${response?.status()} ${errors.join('; ')}`);
    if (MUTATE === LANGUAGE_MODE) {
      setup(requests === 1 && matches === 1, `not-applied requests=${requests} matches=${matches}`);
      applied = true;
      console.log(`MUTATE-APPLIED: ${LANGUAGE_MODE} requests=1 matches=1`);
    }
    setup(await page.locator('[data-tts]').count() === fixture.paragraphs && await page.locator(CARD).count() === fixture.cards, `${identity} fixture controls differ`);
    const speakers = fixture.paragraphs + fixture.cards;
    check(await page.locator(STATUS).count() === (speakers ? 1 : 0), 'composition-region', `${identity} shared status cardinality`);
    check(await page.locator('.y-ttsbar').count() === (!noAPI && fixture.paragraphs ? 1 : 0), 'composition-toolbar', `${identity} toolbar cardinality`);
    if (!speakers) return;
    const initial = await page.evaluate(() => {
      window.__sharedStatus = document.querySelector('.y-ttsbar__status');
      return { text: window.__sharedStatus.textContent, language: window.__sharedStatus.closest('[lang]')?.lang };
    });
    hit = true;
    console.log(`INVOCATION-HIT read-aloud-status ${identity}: shared status observed`);
    check(initial.text === '' && initial.language === language, 'composition-initial', `${identity} initial status ${JSON.stringify(initial)}`);
    if (!noAPI && fixture.paragraphs) {
      check(await page.locator('.y-ttsbar .y-ttsbar__status').count() === 1, 'composition-placement', `${identity} status outside toolbar`);
    }
    if (fixture.name === 'listen' && !noAPI) {
      check(await page.locator('[data-readaloud-bar] .y-ttsbar').count() === 1, 'listen-placement', `${identity} listening anchor changed`);
    }
    const triggers = [];
    if (fixture.cards) triggers.push(CARD);
    if (fixture.paragraphs) triggers.push('[data-tts]');
    const shuffle = await page.locator('.y-slotlive').allTextContents();
    for (const selector of triggers) {
      const trigger = page.locator(selector).first();
      const idle = await trigger.getAttribute('aria-label');
      if (noAPI) {
        // Without a speech engine the page withholds these affordances. Check
        // that state through the UI instead of pressing an inaccessible button.
        check(await page.locator(selector).evaluateAll((controls) => controls.every((control) => getComputedStyle(control).display === 'none')), 'unsupported-hidden', `${identity} unsupported speech control is exposed`);
        check(await page.evaluate(() => window.__speechFixture.utterances.length) === 0 && await page.locator(STATUS).textContent() === '' && await trigger.getAttribute('aria-label') === idle && await page.locator('[data-speaking], [data-reading]').count() === 0, 'unsupported-silent', `${identity} unsupported API changed idle behavior`);
        continue;
      }
      await pressForHandoff(page, trigger);
      await page.evaluate(() => window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('error')));
      check(await page.locator(STATUS).textContent() === UNAVAILABLE[language] && await trigger.getAttribute('aria-label') === idle && await page.locator('[data-speaking], [data-reading]').count() === 0, 'composition-error', `${identity} ${selector} refusal failed`);
      check(await page.evaluate(() => window.__sharedStatus === document.querySelector('.y-ttsbar__status')) && await page.locator(STATUS).count() === 1, 'composition-identity', `${identity} replaced or duplicated status`);
    }
    check(JSON.stringify(await page.locator('.y-slotlive').allTextContents()) === JSON.stringify(shuffle), 'shuffle-isolation', `${identity} speech changed shuffle announcements`);
    if (!noAPI && fixture.paragraphs > 1) {
      await pressForHandoff(page, '.y-ttsbar__play');
      const count = await page.evaluate(() => window.__speechFixture.utterances.length);
      await page.evaluate(() => window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('end')));
      await waitForHandoff(page, count + 1);
      check(await page.evaluate(() => window.__speechFixture.utterances.length) === count + 1, 'automatic-advance', `${identity} end did not advance once`);
      await page.evaluate(() => window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('error')));
      check(await page.locator(STATUS).textContent() === UNAVAILABLE[language] && await page.locator('.y-ttsbar__play').getAttribute('aria-pressed') === 'false' && await page.locator('[data-speaking], [data-reading]').count() === 0 && await page.evaluate(() => window.__speechFixture.utterances.length) === count + 1, 'advance-error', `${identity} error did not end the walk`);
      for (const selector of ['.y-ttsbar__prev', '.y-ttsbar__next']) {
        await pressForHandoff(page, selector);
        await page.evaluate(() => window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('error')));
        check(await page.locator(STATUS).textContent() === UNAVAILABLE[language] && await page.locator('[data-speaking], [data-reading]').count() === 0, 'step-error', `${identity} ${selector} refusal failed`);
      }
      await pressForHandoff(page, '.y-ttsbar__play');
      await page.click('.y-ttsbar__stop');
      const stopped = await page.locator(STATUS).textContent();
      await page.evaluate(() => window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('error')));
      check(await page.locator(STATUS).textContent() === stopped && await page.locator('[data-speaking], [data-reading]').count() === 0, 'stopped-error', `${identity} stopped callback changed status`);
      await pressForHandoff(page, '.y-ttsbar__play');
      const beforeHide = await page.locator(STATUS).textContent();
      const beforeHideCount = await page.evaluate(() => window.__speechFixture.utterances.length);
      await page.evaluate(() => {
        window.dispatchEvent(new Event('pagehide'));
        window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('end'));
        window.__speechFixture.utterances.at(-1).dispatchEvent(new Event('error'));
      });
      check(await page.locator(STATUS).textContent() === beforeHide && await page.evaluate(() => window.__speechFixture.utterances.length) === beforeHideCount, 'pagehide-stale', `${identity} pagehide callback changed status or advanced`);
    }
    setup(errors.length === 0, `${identity} runtime errors: ${errors.join('; ')}`);
  } finally {
    await context.close();
  }
}
let browser;
try {
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  for (const width of [1280, 390]) {
    for (const language of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        await documentWithoutColumn(browser, width, language, theme);
        await noLocalVoice(browser, width, language, theme);
        const context = await browser.newContext({ viewport: { width, height: 800 } });
        await context.addCookies([
          { name: 'yomihon_lang', value: language, url: BASE },
          { name: 'yomihon_theme', value: theme, url: BASE },
        ]);
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', (error) => errors.push(error.message));
        await installSpeechVoices(page, { record: true });
        await observePractice(page);
        const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
        setup(response?.status() === 200, `practice fixture returned ${response?.status()}`);
        setup(errors.length === 0, `runtime errors: ${errors.join('; ')}`);
        setup(await page.locator(CARD).count() === 1 && await page.locator('[data-tts]').count() === 0, 'practice fixture must have one card and no paragraph speaker');
        const before = await page.evaluate(() => ({
          text: window.__authoredText(),
          idle: document.querySelector('[data-slot-action="speak"]')?.getAttribute('aria-label'),
          region: document.querySelector('.y-ttsbar__status')?.textContent ?? null,
        }));
        await pressForHandoff(page, CARD);
        const handoff = await page.evaluate(() => window.__speechFixture.utterances.map((utterance) => ({ text: utterance.text, lang: utterance.lang })));
        setup(handoff.length === 1 && handoff[0].text === 'わたし' && handoff[0].lang === 'ja', `wrong handoff ${JSON.stringify(handoff)}`);
        await page.evaluate(() => window.__speechFixture.utterances[0].dispatchEvent(new Event('error')));
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
        await page.evaluate(() => window.__speechFixture.utterances[0].dispatchEvent(new Event('error')));
        check(await page.evaluate(() => window.__refusals) === 1, 'double-error', 'repeated error announced twice');
        await pressForHandoff(page, CARD);
        await page.click(CARD); // Stop the active generation.
        await pressForHandoff(page, CARD); // Replace it with a later generation.
        const current = await page.locator(STATUS).textContent();
        await page.evaluate(() => window.__speechFixture.utterances[1].dispatchEvent(new Event('error')));
        check(await page.locator(STATUS).textContent() === current && await page.locator('[data-speaking]').count() === 1, 'stale-error', 'stale error altered the later generation');
        await page.evaluate(() => window.__speechFixture.utterances[2].dispatchEvent(new Event('error')));
        check(await page.locator(STATUS).textContent() === UNAVAILABLE[language] && await page.evaluate(() => window.__refusals) === 2, 'later-refusal', 'later generation did not announce its refusal once');
        setup(errors.length === 0, `runtime errors after speech: ${errors.join('; ')}`);
        await context.close();
        for (const fixture of COMPOSITIONS) {
          for (const noAPI of [false, true]) {
            await composition(browser, width, language, theme, fixture, noAPI);
          }
        }
      }
    }
  }
  console.log('PASS read-aloud-status: practice refusal, generation recovery and speech compositions in both languages, themes and viewports');
} catch (error) {
  console.error(error.message);
  if (error instanceof LockFired && error.site === 'practice-unavailable' && hit && (!MUTATE || applied)) {
    console.log('caught: read-aloud-status practice-unavailable');
  }
  const intended = MUTATE === MODE ? 'practice-no-local-voice'
    : MUTATE === NULL_MODE ? 'document-status' : 'composition-initial';
  if (MUTATE && error instanceof LockFired && error.site === intended && applied && hit) {
    console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    if (MUTATE) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = error instanceof LockFired && !MUTATE ? 1 : 2;
  }
} finally {
  await browser?.close();
}
