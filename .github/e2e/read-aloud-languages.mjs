// The initialized lesson hands each authored language to the utterance sink.
// Controlled speech events prove page recovery, not an installed audible voice.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9946';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/languages/Read%20aloud.md';
const MUTATE = process.env.MUTATE || '';
const PASSAGES = [['ja', '朝です。'], ['zh-Hant', '早安。'], ['en', 'Good morning.'], ['fr', 'Bonjour.'], ['und', 'Neutral words.']];
const SITES = ['wrapper-language', 'paragraph-language', 'control-language', 'utterance-language', 'explicit-und', 'unavailable-message', 'error-ends-run', 'error-resets-speaker'];
class LockFired extends Error {
  constructor(site, message) { super(`FAIL read-aloud-languages: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const fail = (site, message) => { throw new LockFired(site, message); };
const rewrite = (path, needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(path, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1 || matches !== 1) throw new NotApplied(`needle requested ${requests} times and matched ${matches} times, want 1 each`);
  };
};
const script = (needle, replacement) => rewrite('**/lesson.js', needle, replacement);
const document = (needle, replacement) => rewrite(BASE + PAGE, needle, replacement);
const MUTATIONS = {
  'change-wrapper-language': { target: 'wrapper-language', apply: document('<div class="y-reading" lang="fr">', '<div class="y-reading" lang="en">') },
  'change-paragraph-language': { target: 'paragraph-language', apply: document('<p lang="fr">Bonjour.</p>', '<p lang="en">Bonjour.</p>') },
  'change-control-language': { target: 'control-language', apply: document('data-tts="Bonjour." lang="zh-Hant"', 'data-tts="Bonjour." lang="fr"') },
  'hardcode-utterance-language': { target: 'utterance-language', apply: script('utterance.lang = speechLanguage(passage);', "utterance.lang = 'ja-JP';") },
  'read-trigger-language': { target: 'utterance-language', apply: script('utterance.lang = speechLanguage(passage);', 'utterance.lang = speechLanguage(trigger);') },
  'replace-explicit-und': { target: 'explicit-und', apply: script("return declared || 'ja-JP';", "return declared && declared !== 'und' ? declared : 'ja-JP';") },
  'keep-first-language': { target: 'utterance-language', apply: script('utterance.lang = speechLanguage(passage);', "utterance.lang = speechLanguage(document.querySelector('.y-reading'));") },
  'suppress-unavailable-message': { target: 'unavailable-message', apply: script('      announce(unavailableLabel);', "      announce('');") },
  'keep-reading-after-error': { target: 'error-ends-run', apply: script('      endRun();\n      announce(unavailableLabel);', '      announce(unavailableLabel);') },
  'retain-speaking-after-error': { target: 'error-resets-speaker', apply: script('      announce(unavailableLabel);\n      resetSpeakButton();', '      announce(unavailableLabel);') },
};
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) throw new Error(`unknown target for ${name}`);
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) throw new Error(`unmutated site ${site}`);
}
if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) process.exit(2);

const installSpeech = () => {
  window.__utterances = [];
  window.__cancels = 0;
  speechSynthesis.speak = (utterance) => { window.__utterances.push(utterance); };
  speechSynthesis.cancel = () => { window.__cancels += 1; };
};
const heard = (page) => page.evaluate(() => window.__utterances.map((utterance) => [utterance.lang, utterance.text]));
const state = (page) => page.evaluate(() => ({
  status: document.querySelector('.y-ttsbar__status')?.textContent,
  running: document.querySelector('.y-ttsbar__play')?.getAttribute('aria-pressed'),
  marked: document.querySelectorAll('[data-reading]').length,
  speaking: document.querySelectorAll('[data-speaking]').length,
  idle: document.querySelector('[data-tts]')?.getAttribute('aria-label'),
  spoken: window.__utterances.length,
}));
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let applied = false;
try {
  for (const lang of ['zh-Hant', 'en']) {
    const context = await browser.newContext();
    await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }]);
    const page = await context.newPage();
    await page.route('**/*', (route) => route.request().method() === 'GET' ? route.continue() : route.abort());
    await page.addInitScript(installSpeech);
    const proof = MUTATE && lang === 'zh-Hant' ? await MUTATIONS[MUTATE].apply(page) : null;
    const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (response?.status() !== 200) throw new Error(`GET returned ${response?.status()}`);
    await page.waitForSelector('.y-ttsbar__play');
    if (proof) { proof(); applied = true; }
    const blocks = await page.locator('.y-reading').evaluateAll((elements) => elements.map((element) => ({
      wrapper: element.lang, paragraph: element.querySelector('p')?.lang,
      control: element.querySelector('button')?.lang, label: element.querySelector('button')?.getAttribute('aria-label'),
    })));
    if (blocks.length !== PASSAGES.length) throw new Error(`fixture produced ${blocks.length} blocks, want 5`);
    for (const [index, [tag]] of PASSAGES.entries()) {
      if (blocks[index].wrapper !== tag) fail('wrapper-language', `wrapper ${index} is ${blocks[index].wrapper}, want ${tag}`);
      if (blocks[index].paragraph !== tag) fail('paragraph-language', `paragraph ${index} is ${blocks[index].paragraph}, want ${tag}`);
      if (blocks[index].control !== lang) fail('control-language', `control ${index} is ${blocks[index].control}, want ${lang}`);
    }
    const idle = lang === 'en' ? 'Read this aloud' : '朗讀這段文字';
    if (blocks.some((block) => block.label !== idle)) throw new Error(`wrong neutral accessible name for ${lang}`);
    const text = await page.locator('.y-article').textContent();
    if (!text.includes('Malformed marker prose stays readable.') || !text.includes('Unmarked prose stays readable.') || text.includes('read-aloud: en_US')) throw new Error('authored degraded prose or malformed drop changed');
    await page.locator('.y-ttsbar__play').focus();
    await page.keyboard.press('Enter');
    for (const [index, expected] of PASSAGES.entries()) {
      const utterances = await heard(page);
      const actual = utterances[index];
      const site = expected[0] === 'und' ? 'explicit-und' : 'utterance-language';
      if (!actual || actual[0] !== expected[0] || actual[1] !== expected[1]) fail(site, `actual utterance ${index} = ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
      await page.evaluate(() => window.__utterances.at(-1).dispatchEvent(new Event('end')));
    }
    if ((await state(page)).spoken !== 5) throw new Error('play-through repeated or omitted a paragraph');
    await page.click('.y-ttsbar__play');
    await page.evaluate(() => {
      window.__announcements = [];
      new MutationObserver(() => window.__announcements.push(document.querySelector('.y-ttsbar__status').textContent))
        .observe(document.querySelector('.y-ttsbar__status'), { childList: true });
      const error = new Event('error');
      Object.defineProperty(error, 'error', { value: 'language-unavailable' });
      window.__utterances.at(-1).dispatchEvent(error);
    });
    const afterError = await state(page);
    const unavailable = lang === 'en' ? 'Speech is unavailable right now' : '目前無法播放語音';
    if (afterError.status !== unavailable) fail('unavailable-message', `error says ${JSON.stringify(afterError.status)}, want ${unavailable}`);
    const announcements = await page.evaluate(() => window.__announcements);
    if (announcements.filter((entry) => entry === unavailable).length !== 1) fail('unavailable-message', `error announced ${JSON.stringify(announcements)}, want one unavailable message`);
    if (afterError.running !== 'false' || afterError.spoken !== 6) fail('error-ends-run', `error left run active or advanced: ${JSON.stringify(afterError)}`);
    if (afterError.marked !== 0 || afterError.speaking !== 0 || afterError.idle !== idle) fail('error-resets-speaker', `error left active state: ${JSON.stringify(afterError)}`);
    // An error ends one generation. Replay and stale events use the real owner.
    await page.click('[data-tts]');
    const replay = await heard(page);
    if (JSON.stringify(replay.at(-1)) !== JSON.stringify(PASSAGES[0])) throw new Error('error recovery did not permit replay');
    await page.click('.y-ttsbar__stop');
    const cancels = await page.evaluate(() => window.__cancels);
    await page.click('[data-tts]');
    await page.evaluate(() => {
      window.__utterances.at(-2).dispatchEvent(new Event('end'));
      window.__utterances.at(-2).dispatchEvent(new Event('error'));
    });
    const replayState = await state(page);
    if (replayState.spoken !== 8 || replayState.speaking !== 1 || replayState.marked !== 1) throw new Error('cancelled generation changed replay');
    await page.evaluate(() => window.dispatchEvent(new Event('pagehide')));
    if (await page.evaluate(() => window.__cancels) !== cancels + 2) throw new Error('page disposal did not reach speech cancel');
    await context.close();
  }
  // Speech is an enhancement: no API and no script leave the author's text.
  for (const javaScriptEnabled of [true, false]) {
    const context = await browser.newContext({ javaScriptEnabled });
    if (javaScriptEnabled) await context.addInitScript(() => { delete window.speechSynthesis; });
    const page = await context.newPage();
    await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (!(await page.locator('.y-article').textContent()).includes('Good morning.')) throw new Error('speech unavailable lost authored prose');
    if (await page.locator('.y-ttsbar').count() !== 0) throw new Error('absent speech API/script offered a live bar');
    await context.close();
  }
  console.log('PASS read-aloud-languages: actual utterances preserve mixed authored languages, neutral bilingual controls and error/replay/cancellation boundaries; unavailable enhancement retains prose');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied || (MUTATE && !applied)) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
