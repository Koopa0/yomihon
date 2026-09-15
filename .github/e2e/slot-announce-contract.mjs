// Behavior lock for what the sentence-pattern card says out loud. Shuffling
// rewrites the Japanese sentence and its Chinese gloss in place; a reader who
// cannot see the card has only the button they pressed, and the button reports
// nothing. The card's live region has to carry the recomposed sentence, and has
// to stay quiet on the path that already speaks for itself — picking from a
// select, where the select announces the option it landed on.
//
// Env: YOMIHON_BASE, PAGE_PATH (the L01 fixture), and MUTATE. MUTATE=list
// prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const LIVE = '.y-slotlive';
const SHUFFLE = '[data-slot-action="shuffle"]';
const SPEAK = '[data-slot-action="speak"]';
// The sentence is Japanese however the chrome around it is written, so this
// site runs on its own pages, one per interface language.
const VOICE_SITE = 'the-voice-follows-the-sentence';
const SLOT_A = 'select[data-slot-key="A"]';

// The fixture's second fill in each slot, which is what the stubbed shuffle
// below always picks. Written out rather than recomputed from the card's own
// data: a value derived the way the code derives it would agree with the code
// however wrong both were.
const SHUFFLED_SENTENCE = '田中さんは 先生です';
const SHUFFLED_GLOSS = '田中先生 是 老師';

const SITES = [
  'region-starts-empty',
  'select-change-stays-quiet',
  'shuffle-announces-the-sentence',
  'announced-gloss-declares-its-language',
  'second-press-stops-the-sentence',
  VOICE_SITE,
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN slot-announce-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL slot-announce-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN slot-announce-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED slot-announce-contract: ${message}`); };

const rewriteScript = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/lesson.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `${label}: the runtime was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label}: the runtime needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const rewriteDocument = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(`**${PAGE}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `${label}: the document was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label}: the document needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'region-arrives-with-text': {
    target: 'region-starts-empty',
    apply: rewriteDocument(
      'aria-atomic="true" lang="ja"></p>',
      'aria-atomic="true" lang="ja">上一句</p>',
      'live region content',
    ),
  },
  'announce-on-select-change': {
    target: 'select-change-stays-quiet',
    apply: rewriteScript(
      '= Number(select.value) || 0;\n        render();',
      '= Number(select.value) || 0;\n        render();\n        announce();',
      'select change handler',
    ),
  },
  'drop-the-shuffle-announcement': {
    target: 'shuffle-announces-the-sentence',
    apply: rewriteScript('      render();\n      announce();\n', '      render();\n', 'shuffle handler'),
  },
  // The defect itself: the card called the shared speech owner without handing
  // over its button, so the owner could not tell a second press from a first.
  'speak-without-the-button': {
    target: 'second-press-stops-the-sentence',
    apply: rewriteScript('        speakButton,\n', '', 'practice speak trigger'),
  },
  // Handing the button over hands the voice over with it: resolved from the
  // button, the nearest declared language is the card's own root, which carries
  // the interface language. The sentence would then be read in a Chinese or
  // English voice on every card.
  'speak-in-the-pages-language': {
    target: VOICE_SITE,
    apply: rewriteScript("        card.querySelector('.y-slotoutput'),\n", '', 'practice speech passage'),
  },
  'drop-the-gloss-language': {
    target: 'announced-gloss-declares-its-language',
    apply: rewriteScript("      gloss.lang = 'zh-Hant';\n", '', 'announced gloss language'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`slot-announce-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`slot-announce-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`slot-announce-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const readRegion = (page) =>
  page.evaluate((selector) => {
    const regions = Array.from(document.querySelectorAll(selector));
    const region = regions[0];
    return {
      count: regions.length,
      text: region ? region.textContent : null,
      language: region ? region.getAttribute('lang') : null,
      spans: region
        ? Array.from(region.querySelectorAll('span'), (span) => ({ text: span.textContent, language: span.getAttribute('lang') }))
        : [],
    };
  }, LIVE);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // Shuffle picks at random, so left alone it cannot be asserted against and,
  // with two fills a slot, would land on the sentence already showing a quarter
  // of the time. Held at the top of every slot's list it picks the second fill
  // every time, which is never what the page arrived with.
  await page.addInitScript(() => {
    Math.random = () => 0.99;
  });
  // Headless Chrome need not produce audio, and whether a voice is installed is
  // not this lock's business. What the page decides is: the calls it makes at
  // the speech boundary, recorded here, and a start event so the button holds
  // the speaking state the way it would while a voice was running.
  await page.addInitScript(() => {
    window.__speechCalls = [];
    speechSynthesis.speak = (utterance) => {
      window.__speechCalls.push('speak');
      setTimeout(() => utterance.dispatchEvent(new Event('start')), 0);
    };
    speechSynthesis.cancel = () => {
      window.__speechCalls.push('cancel');
    };
  });
  proof = MUTATE && MUTATIONS[MUTATE].target !== VOICE_SITE
    ? await MUTATIONS[MUTATE].apply(page)
    : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  await page.waitForSelector(SHUFFLE, { state: 'attached', timeout: 2000 });

  const atRest = await readRegion(page);
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  if (atRest.count !== 1) broken(`the lesson shows ${atRest.count} live regions, want exactly 1 to drive`);
  if (atRest.text !== '') {
    fail('region-starts-empty', `the card's live region arrived holding ${JSON.stringify(atRest.text)}, want it empty so the page announces nothing on arrival`);
  }

  // The select speaks the option it landed on by itself. Talking over that with
  // the whole sentence is the double announcement this stays clear of.
  await page.selectOption(SLOT_A, '1');
  const afterSelect = await readRegion(page);
  if (afterSelect.text !== '') {
    fail('select-change-stays-quiet', `picking from a slot select put ${JSON.stringify(afterSelect.text)} in the live region, want it left empty because the select already announces the option`);
  }

  await page.click(SHUFFLE);
  const afterShuffle = await readRegion(page);
  const wanted = `${SHUFFLED_SENTENCE} ${SHUFFLED_GLOSS}`;
  if (afterShuffle.text !== wanted) {
    fail('shuffle-announces-the-sentence', `shuffling put ${JSON.stringify(afterShuffle.text)} in the live region, want ${JSON.stringify(wanted)}`);
  }
  if (afterShuffle.language !== 'ja') {
    broken(`the live region declares ${JSON.stringify(afterShuffle.language)}, want "ja"; the announcement below cannot be read in the right voice without it`);
  }

  const glossSpan = afterShuffle.spans.find((span) => span.text === SHUFFLED_GLOSS);
  if (!glossSpan) {
    broken(`the announcement holds no span reading ${JSON.stringify(SHUFFLED_GLOSS)}, so its declared language cannot be checked`);
  }
  if (glossSpan.language !== 'zh-Hant') {
    fail('announced-gloss-declares-its-language', `the announced gloss declares ${JSON.stringify(glossSpan.language)}, want "zh-Hant"; the region around it is Japanese, so an undeclared gloss is read in a Japanese voice`);
  }

  // The speaker beside the card is the one the reader already met further up
  // the page, so pressing it while it speaks stops it. Cancelling and starting
  // the same sentence again reads as a button that does nothing.
  const readSpeech = () => page.evaluate((speak) => {
    const button = document.querySelector(speak);
    return {
      calls: [...window.__speechCalls],
      speaking: button.hasAttribute('data-speaking'),
      label: button.getAttribute('aria-label'),
    };
  }, SPEAK);

  if (await page.locator(SPEAK).count() !== 1) {
    broken('the card shows no speaker button, so there is nothing to press twice');
  }
  const stopLabel = await page.evaluate(() => document.querySelector('[data-readaloud-controls]')?.dataset.readaloudStopthis ?? '');
  if (!stopLabel) {
    broken('the page carries no stop-this label, so the speaking button has no words to take');
  }

  await page.click(SPEAK);
  const firstPress = await readSpeech();
  if (!firstPress.speaking || firstPress.label !== stopLabel) {
    fail('second-press-stops-the-sentence', `one press left the speaker speaking=${firstPress.speaking} labelled ${JSON.stringify(firstPress.label)}, want it holding the speaking state and the stop label ${JSON.stringify(stopLabel)} the paragraph speakers take`);
  }

  await page.click(SPEAK);
  const secondPress = await readSpeech();
  const spoken = secondPress.calls.filter((call) => call === 'speak').length;
  if (spoken !== 1) {
    fail('second-press-stops-the-sentence', `two presses asked the voice to speak ${spoken} times (${secondPress.calls.join(', ')}), want 1: the second press stops the sentence rather than starting it again`);
  }
  if (secondPress.speaking) {
    fail('second-press-stops-the-sentence', 'the speaker still holds the speaking state after the press that stopped it');
  }

  // The voice is the sentence's, not the page's. Read in the language of the
  // chrome, a Japanese line comes out in a Chinese or English voice, which is
  // not a smaller version of reading it aloud.
  for (const interfaceLang of ['zh-Hant', 'en']) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await context.addCookies([{ name: 'yomihon_lang', value: interfaceLang, url: BASE }]);
    const voicePage = await context.newPage();
    await voicePage.addInitScript(() => {
      window.__spoken = [];
      speechSynthesis.speak = (utterance) => {
        window.__spoken.push(utterance.lang);
        setTimeout(() => utterance.dispatchEvent(new Event('start')), 0);
      };
      speechSynthesis.cancel = () => {};
    });
    const voiceProof = MUTATE && MUTATIONS[MUTATE].target === VOICE_SITE
      ? await MUTATIONS[MUTATE].apply(voicePage)
      : null;
    await voicePage.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await voicePage.waitForSelector(SPEAK, { state: 'attached', timeout: 2000 });
    if (voiceProof) {
      const issue = voiceProof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
      mutationApplied = true;
    }
    await voicePage.click(SPEAK);
    const spoken = await voicePage.evaluate(() => [...window.__spoken]);
    if (spoken.length !== 1) {
      broken(`pressing the speaker under ${interfaceLang} asked the voice to speak ${spoken.length} times, want exactly 1 to read its language from`);
    }
    // Written out rather than read back off the page: an expectation taken from
    // the element the code reads would agree with the code whatever it says.
    if (spoken[0] !== 'ja') {
      fail(VOICE_SITE, `with the interface in ${interfaceLang} the card asked for a ${JSON.stringify(spoken[0])} voice, want "ja": the sentence is Japanese however the page around it is written`);
    }
    await context.close();
  }

  console.log('PASS slot-announce-contract: the card announces its shuffled sentence, stays quiet where the select already speaks, marks the gloss language, stops on the second press of its speaker, and asks for a Japanese voice in either interface language');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE && !mutationApplied) {
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
    } else {
      if (MUTATE) {
        const { target } = MUTATIONS[MUTATE];
        if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
        else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
      }
      process.exitCode = 1;
    }
  } else if (err instanceof ProbeBroken) {
    console.error(err.message);
    process.exitCode = 1;
  } else {
    console.error(err);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
