// Browser-boundary evidence for local read-aloud. Audio is controlled; the initialized
// lesson still chooses the voice and owns readiness, cancellation and recovery.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';
import { installSpeechVoices } from './support/speech-voices.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/languages/Read%20aloud.md';
const MUTATE = process.env.MUTATE || '';
const PASSAGES = [
  { lang: 'ja', text: '朝です。' },
  { lang: 'zh-Hant', text: '早安。' },
  { lang: 'en', text: 'Good morning.' },
  { lang: 'fr', text: 'Bonjour.' },
  { lang: 'und', text: 'Neutral words.' },
  { lang: 'und-Latn', text: 'Undetermined Latin words.' },
];
const voice = (name, lang, localService = true, isDefault = false) => ({
  name, lang, localService, default: isDefault,
});
const JAPANESE = voice('Device Japanese', 'ja-JP');
const REMOTE = voice('Remote Japanese', 'ja-JP', false, true);
const ENGLISH = voice('Device English', 'en-US');
class LockFired extends Error {
  constructor(site, message) { super(`caught: read-aloud-local-voices: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const fail = (site, message) => { throw new LockFired(site, message); };
const MUTATIONS = {
  'drop-local-service-filter': {
    target: 'local-service', needle: 'voice.localService === true', replacement: 'true',
  },
  'skip-voices-readiness': {
    target: 'voice-readiness', needle: 'await voicesReady();', replacement: 'void voicesReady();',
  },
  'drop-language-script-comparison': {
    target: 'language-match',
    needle: 'candidate.language === requested.language && candidate.script === requested.script',
    replacement: 'true',
  },
  'leave-utterance-voice-unset': {
    target: 'bound-voice', needle: 'utterance.voice = voice;', replacement: 'void voice;',
  },
  'maximize-undetermined-language': {
    target: 'undetermined-language',
    needle: "locale.language === 'und' ? locale : locale.maximize()",
    replacement: 'locale.maximize()',
  },
};
if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) process.exit(2);

const settle = (page) => page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
const observe = (page) => page.evaluate(() => ({
  receipts: window.__speechFixture.receipts,
  reads: window.__speechFixture.reads,
  cancels: window.__speechFixture.cancels,
  status: document.querySelector('.y-ttsbar__status')?.textContent,
  running: document.querySelector('.y-ttsbar__play')?.getAttribute('aria-pressed'),
  speaking: document.querySelectorAll('[data-speaking]').length,
  marked: document.querySelectorAll('[data-reading]').length,
  labels: [...document.querySelectorAll('[data-tts]')].map((button) => button.getAttribute('aria-label')),
}));
const press = async (page, index = 0) => {
  await page.locator('[data-tts]').nth(index).focus();
  await page.keyboard.press('Enter');
  await settle(page);
};
const publish = (page, voices, notify = true) => page.evaluate(({ voices: next, notify: dispatch }) => {
  window.__speechFixture.setVoices(next, dispatch);
}, { voices, notify });
const unavailableText = (chrome) => chrome === 'en' ? 'Speech is unavailable right now' : '目前無法播放語音';
const idleText = (chrome) => chrome === 'en' ? 'Read this aloud' : '朗讀這段文字';
const noSpeech = async (page, site, label) => {
  if ((await observe(page)).receipts.length !== 0) fail(site, `${label} handed text to speak`);
};
const pendingDeadline = async (page) => {
  const registration = await page.evaluate(() => ({
    reads: window.__speechFixture.reads,
    deadlines: window.__speechFixture.readinessDeadlines(),
  }));
  if (registration.reads === 0 || registration.deadlines.length !== 1
    || registration.deadlines[0].delay !== 1000 || registration.deadlines[0].state !== 'pending') {
    fail('voice-readiness', `readiness did not register one controlled one-second deadline: ${JSON.stringify(registration)}`);
  }
};
const assertVoice = async (page, expected, index = 0, site = 'bound-voice', chrome = 'zh-Hant') => {
  await settle(page);
  const state = await observe(page);
  const receipt = state.receipts.at(-1);
  if (state.receipts.length !== 1 || !receipt) fail(site, `got ${JSON.stringify(state.receipts)}, want exactly one speech handoff`);
  if (!receipt.voice) fail('bound-voice', 'speak received no explicit voice');
  if (receipt.voice.localService !== true) fail('local-service', `speak received remote voice ${receipt.voice.name}`);
  if (!receipt.voiceWasListed || receipt.voice.name !== expected.name) fail(site, `chose ${JSON.stringify(receipt.voice)}, want ${expected.name}`);
  if (receipt.lang !== PASSAGES[index].lang || receipt.text !== PASSAGES[index].text) fail(site, `changed authored speech: ${JSON.stringify(receipt)}`);
  if (state.speaking !== 1 || state.marked !== 1) fail(site, 'speech handoff has no unique active control/passage');
  const playing = chrome === 'en' ? 'Playing' : '播放中';
  const stop = chrome === 'en' ? 'Stop reading aloud' : '停止朗讀';
  if (state.status !== playing || state.labels[index] !== stop || state.running !== 'false') {
    fail(site, `paragraph speech did not announce/label its active state: ${JSON.stringify(state)}`);
  }
};
const assertUnavailable = async (page, chrome, site) => {
  try {
    await page.waitForFunction((text) => document.querySelector('.y-ttsbar__status')?.textContent === text,
      unavailableText(chrome), { timeout: 5000 });
  } catch {
    fail(site, `no unavailable message; state=${JSON.stringify(await observe(page))}`);
  }
  const state = await observe(page);
  if (state.receipts.length !== 0 || state.running !== 'false' || state.speaking !== 0 || state.marked !== 0
    || state.labels.some((label) => label !== idleText(chrome))) {
    fail(site, `unavailable did not refuse/recover: ${JSON.stringify(state)}`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let applied = false;
let appliedRequests = 0;
let appliedMatches = 0;
const visit = async (voices, chrome, scenario) => {
  const context = await browser.newContext();
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: chrome, url: BASE }]);
    const page = await context.newPage();
    await page.route('**/*', (route) => route.request().method() === 'GET' ? route.continue() : route.abort());
    await installSpeechVoices(page, {
      voices, record: true, endOnCancel: true, manualReadinessDeadline: true,
    });
    // One response in the first scenario proves the one production edit site.
    // Later contexts receive the same fault but do not inflate that proof.
    let requests = 0;
    let matches = 0;
    if (MUTATE) {
      await page.route('**/lesson.js{,?*}', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const mutation = MUTATIONS[MUTATE];
        const count = original.split(mutation.needle).length - 1;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(mutation.needle, mutation.replacement) : original });
      });
    }
    const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (response?.status() !== 200) throw new Error(`GET returned ${response?.status()}`);
    await page.waitForSelector('.y-ttsbar__play');
    await arrived(page);
    if (MUTATE) {
      if (requests !== 1 || matches !== 1) throw new NotApplied(`production needle requested ${requests}, matched ${matches}; want exactly 1 each`);
      if (!applied) {
        appliedRequests = requests;
        appliedMatches = matches;
        applied = true;
        console.log(`MUTATE-APPLIED: ${MUTATE} requests=1 matches=1`);
      }
    }
    const passages = await page.locator('.y-reading').evaluateAll((elements) => elements.map((element) => ({
      lang: element.getAttribute('lang'), text: element.querySelector('button')?.getAttribute('data-tts'),
    })));
    if (JSON.stringify(passages) !== JSON.stringify(PASSAGES)) throw new Error('authored language fixture drifted');
    if (voices.length === 0) await page.evaluate(() => window.__speechFixture.armReadinessDeadline());
    await scenario(page);
  } finally {
    await context.close();
  }
};

try {
  for (const chrome of ['zh-Hant', 'en']) {
    await visit([REMOTE, JAPANESE], chrome, async (page) => {
      await press(page);
      await assertVoice(page, JAPANESE, 0, 'local-service', chrome);
    });
    await visit([], chrome, async (page) => {
      await press(page);
      await pendingDeadline(page);
      await noSpeech(page, 'voice-readiness', 'pending readiness');
      if ((await observe(page)).status === unavailableText(chrome)) fail('voice-readiness', 'empty voices refused before voiceschanged or timeout');
      await publish(page, [JAPANESE]);
      await assertVoice(page, JAPANESE, 0, 'voice-readiness', chrome);
    });
    await visit([], chrome, async (page) => {
      await press(page);
      await pendingDeadline(page);
      await noSpeech(page, 'voice-readiness', 'empty voices');
      if ((await observe(page)).status === unavailableText(chrome)) fail('voice-readiness', 'empty voices refused without waiting');
      await page.evaluate(() => window.__speechFixture.expireReadinessDeadline());
      await assertUnavailable(page, chrome, 'voice-readiness');
      // A resolved cached readiness promise must not cache the empty list.
      await publish(page, [JAPANESE], false);
      await press(page);
      await assertVoice(page, JAPANESE, 0, 'voice-readiness', chrome);
    });
    await visit([REMOTE], chrome, async (page) => {
      await page.locator('.y-ttsbar__play').focus();
      await page.keyboard.press('Enter');
      await assertUnavailable(page, chrome, 'local-service');
    });
    await visit([ENGLISH], chrome, async (page) => {
      await press(page);
      await assertUnavailable(page, chrome, 'language-match');
    });
  }

  // Cancellation owns the generation even while a voice list is pending.
  for (const action of ['stop', 'repeat', 'replacement']) {
    await visit([], 'zh-Hant', async (page) => {
      await press(page);
      await pendingDeadline(page);
      await noSpeech(page, 'voice-readiness', `pending ${action}`);
      if (action === 'stop') await page.click('.y-ttsbar__stop');
      else await press(page, action === 'repeat' ? 0 : 2);
      await publish(page, [JAPANESE, ENGLISH]);
      await settle(page);
      if (action === 'replacement') {
        await assertVoice(page, ENGLISH, 2, 'voice-readiness');
      } else {
        await noSpeech(page, 'voice-readiness', `${action} during readiness`);
        const state = await observe(page);
        if (state.running !== 'false' || state.speaking !== 0 || state.marked !== 0
          || state.labels.some((label) => label !== idleText('zh-Hant'))) fail('voice-readiness', `${action} retained active controls`);
      }
    });
  }

  // Literal expected identities keep Intl.Locale out of the oracle: TW/HK
  // match authored Hant, while CN does not.
  const TW = voice('Taiwan device', 'zh-TW');
  const HK = voice('Hong Kong device', 'zh-HK');
  const CN = voice('Simplified device', 'zh-CN', true, true);
  for (const traditional of [TW, HK]) {
    await visit([CN, traditional], 'zh-Hant', async (page) => {
      await press(page, 1);
      await assertVoice(page, traditional, 1, 'language-match');
    });
  }
  await visit([CN], 'zh-Hant', async (page) => {
    await press(page, 1);
    await assertUnavailable(page, 'zh-Hant', 'language-match');
  });
  const UNDERSCORE = voice('Underscore Japanese', 'ja_JP');
  await visit([UNDERSCORE], 'zh-Hant', async (page) => {
    await press(page);
    await assertVoice(page, UNDERSCORE, 0, 'language-match');
  });
  const GB_DEFAULT = voice('British default', 'en-GB', true, true);
  const US = voice('US region match', 'en-US');
  const US_DEFAULT = voice('US default match', 'en-US', true, true);
  await visit([GB_DEFAULT, US], 'zh-Hant', async (page) => {
    await press(page, 2);
    await assertVoice(page, US, 2, 'language-match');
  });
  await visit([US, US_DEFAULT], 'zh-Hant', async (page) => {
    await press(page, 2);
    await assertVoice(page, US_DEFAULT, 2, 'language-match');
  });
  await visit([REMOTE, US_DEFAULT], 'zh-Hant', async (page) => {
    await press(page, 4);
    await assertVoice(page, US_DEFAULT, 4, 'local-service');
  });
  await visit([REMOTE, JAPANESE], 'zh-Hant', async (page) => {
    await press(page, 4);
    await assertUnavailable(page, 'zh-Hant', 'local-service');
  });
  const LOCAL_DEFAULT = voice('Local Japanese default', 'ja-JP', true, true);
  for (const chrome of ['zh-Hant', 'en']) {
    await visit([ENGLISH, LOCAL_DEFAULT], chrome, async (page) => {
      await press(page, 5);
      await assertVoice(page, LOCAL_DEFAULT, 5, 'undetermined-language', chrome);
    });
    await visit([ENGLISH], chrome, async (page) => {
      await press(page, 5);
      await assertUnavailable(page, chrome, 'undetermined-language');
    });
  }
  console.log('PASS read-aloud-local-voices: explicit local voice handoffs, authored language, unavailable recovery, readiness, cancellation and locale selection');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied || (MUTATE && !applied)) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && appliedRequests === 1 && appliedMatches === 1
      && error instanceof LockFired && error.site === MUTATIONS[MUTATE].target) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
