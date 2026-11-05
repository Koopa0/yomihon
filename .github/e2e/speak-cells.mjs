// Behavior lock for the speaker beside a table cell that says Japanese. A cell
// is Japanese on its own evidence: written wholly in kana, or Han under the
// author's furigana. A row label, a Chinese gloss, a dash and a note stay
// silent. A speaker hands the cell's words to the same speech owner as a marked
// paragraph, in Japanese whatever language the page declares, stops on a
// second press, and takes no room from the table.
//
// The fixture note declares no language of its own, so the nearest declared
// language above its cells is the interface's. A voice chosen from that would
// be Chinese or English, which is the fault the language site watches for.
//
// Env: YOMIHON_BASE, PAGE_PATH (the read-aloud fixture note, which carries a
// small table), and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';
import { installSpeechVoices } from './support/speech-voices.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/languages/Read%20aloud.md';
const MUTATE = process.env.MUTATE || '';
const CELL = '.y-cellspeak';

const SITES = [
  'cells-classified',
  'accessible-name',
  'cell-language',
  'spoken-text',
  'second-press-stops',
  'no-layout-shift',
];

// The fixture table, cell by cell in document order, and what each says: null
// for a cell that stays silent. Written out rather than read back from the
// page, because an expectation taken from the code under test agrees with it
// however wrong both are.
const CELLS = [
  ['き行', null], ['きゃ', 'きゃ'], ['今天', null], ['濁音+拗音', null],
  ['撥音', null], ['か / かん', 'か、かん'], ['—', null], ['〜さん', 'さん'],
  ['外來語', null], ['バス [片]', 'バス'], ['巴士', null], ['今日きょう', '今日'],
];
const SAID = CELLS.map(([, said]) => said).filter(Boolean);
const NAME = {
  'zh-Hant': (said) => `朗讀「${said}」`,
  en: (said) => `Read “${said}” aloud`,
};
const STOP = { 'zh-Hant': '停止朗讀', en: 'Stop reading aloud' };

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL speak-cells: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}
const fail = (site, message) => {
  if (!SITES.includes(site)) throw new Error(`unknown assertion site ${site}`);
  throw new LockFired(site, message);
};

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
const script = (needle, replacement) => rewrite('**/lesson.js{,?*}', needle, replacement);
const stylesheet = (rule) => async (page) => {
  let requests = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    requests += 1;
    const response = await route.fetch();
    await route.fulfill({ response, body: `${await response.text()}\n${rule}\n` });
  });
  return () => {
    if (requests < 1) throw new NotApplied('app.css was never requested, so the override reached no page');
  };
};

// The script writes its codepoints as escapes, and the needles below must
// reproduce those escapes character for character.
const BACKSLASH = '\\';

const MUTATIONS = {
  // A cell that mixes Han into its kana, such as a row label, is read as
  // Japanese: a Chinese reader's row heading is sent to the Japanese voice.
  'mixed-cell-counts': {
    target: 'cells-classified',
    apply: script("  if (!WHOLLY_KANA.test(cellText(cell, 'ruby'))) return '';\n", ''),
  },
  // Furigana stops counting as evidence, so a cell of kanji with its reading
  // above it goes silent although the author said how to read it.
  'furigana-ignored': {
    target: 'cells-classified',
    apply: script("  const readings = [...cell.querySelectorAll('rt')];\n", '  const readings = [];\n'),
  },
  // The tag closing a katakana word is kept in the cell's words, so the word
  // fails the kana test and goes silent.
  'note-tag-kept': {
    target: 'cells-classified',
    apply: script(
      "return copy.textContent.normalize('NFC').replace(CELL_NOTE, '').trim();",
      "return copy.textContent.normalize('NFC').trim();",
    ),
  },
  // The sentence reaches the page with its place for the words unfilled.
  'label-place-unfilled': {
    target: 'accessible-name',
    apply: script("cellTemplate.replace('{text}', spoken)", 'cellTemplate'),
  },
  // The speaker's own language becomes Japanese, which is the language of what
  // it speaks and not of the sentence naming it.
  'label-in-japanese': {
    target: 'accessible-name',
    apply: script('      button.lang = document.documentElement.lang;\n', "      button.lang = 'ja';\n"),
  },
  // The voice is asked for through the cell itself, whose nearest declared
  // language is the page's: the interface language, not Japanese.
  'language-from-the-page': {
    target: 'cell-language',
    apply: script('speakText(spoken, button, japanese)', 'speakText(spoken, button)'),
  },
  // A slash between two forms is handed to the voice as a character.
  'slash-is-spoken': {
    target: 'spoken-text',
    apply: script(
      `.replace(/\\s*[/${BACKSLASH}uFF0F${BACKSLASH}u2192]\\s*/gu, '${BACKSLASH}u3001')`,
      `.replace(/\\s*[${BACKSLASH}uFF0F${BACKSLASH}u2192]\\s*/gu, '${BACKSLASH}u3001')`,
    ),
  },
  // The speaker takes the walk's own way in, so a second press starts the
  // sentence again instead of stopping it.
  'second-press-restarts': {
    target: 'second-press-stops',
    apply: script('speakText(spoken, button, japanese)', 'speakText(spoken, button, japanese, true)'),
  },
  // The speaker takes its place in the flow, so every column it sits in gains
  // its width.
  'speaker-takes-room': {
    target: 'no-layout-shift',
    apply: stylesheet(`${CELL} { position: static !important; }`),
  },
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
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`speak-cells: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Every cell's box relative to its table, rounded to whole pixels, once the
// page's fonts are in: a table that took a speaker's width into a column would
// move its neighbours, and one that took none would not.
const geometry = (page) => page.evaluate(async () => {
  await document.fonts.ready;
  return [...document.querySelectorAll('.y-prose table')].map((table) => {
    const frame = table.getBoundingClientRect();
    return [Math.round(frame.width), Math.round(frame.height), ...[...table.querySelectorAll('th, td')].map((cell) => {
      const box = cell.getBoundingClientRect();
      return [box.left - frame.left, box.top - frame.top, box.width, box.height].map(Math.round).join(',');
    })];
  });
});
const handoffs = (page, count) => page.waitForFunction((expected) => window.__speechFixture.utterances.length === expected, count);
const settle = (page) => page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let applied = false;
try {
  for (const lang of ['zh-Hant', 'en']) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 2400 } });
    await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }]);
    const page = await context.newPage();
    await installSpeechVoices(page, { record: true });
    const proof = MUTATE && lang === 'zh-Hant' ? await MUTATIONS[MUTATE].apply(page) : null;
    const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    if (response?.status() !== 200) throw new Error(`GET returned ${response?.status()}`);
    await page.waitForSelector('.y-ttsbar__play');
    if (proof) { proof(); applied = true; }

    const cells = await page.locator('.y-prose td').evaluateAll((elements) => elements.map((element) => {
      const button = element.querySelector('button');
      return {
        text: element.textContent.trim(),
        name: button?.getAttribute('aria-label') ?? null,
        kind: button ? `${button.tagName}/${button.type}/${button.lang}` : null,
        marked: button?.hasAttribute('data-tts') ?? false,
      };
    }));
    if (cells.length !== CELLS.length) throw new Error(`fixture table has ${cells.length} cells, want ${CELLS.length}`);
    const offered = cells.map((cell) => cell.name !== null);
    const wanted = CELLS.map(([, said]) => said !== null);
    if (JSON.stringify(offered) !== JSON.stringify(wanted)) {
      fail('cells-classified', `${lang} speakers on ${JSON.stringify(cells.filter((cell) => cell.name).map((cell) => cell.text))}, want ${JSON.stringify(CELLS.filter(([, said]) => said).map(([text]) => text))}`);
    }
    if (cells.some((cell) => cell.marked)) throw new Error('a cell speaker carries data-tts, which would put it into the walk through the marked paragraphs');
    if (await page.locator('[data-tts]').count() !== 6) throw new Error('the marked paragraphs changed, so the bar counts something else');

    // Each speaker hands its words over once, in Japanese, to a Japanese voice.
    const speakers = page.locator(CELL);
    for (const [index, said] of SAID.entries()) {
      await speakers.nth(index).click();
      await handoffs(page, index + 1);
      const receipt = await page.evaluate((at) => window.__speechFixture.receipts[at], index);
      if (receipt.text !== said) fail('spoken-text', `${lang} speaker ${index} handed over ${JSON.stringify(receipt.text)}, want ${JSON.stringify(said)}`);
      if (receipt.lang !== 'ja' || receipt.voice?.name !== 'Japanese device voice' || !receipt.voiceWasListed) {
        fail('cell-language', `${lang} speaker ${index} was spoken as ${receipt.lang} by ${receipt.voice?.name}, want ja by the Japanese device voice`);
      }
    }

    // What each speaker is called is read after what it says, so a mutation of
    // the words spoken, which the name carries too, is caught where it is said.
    cells.forEach((cell, index) => {
      const said = CELLS[index][1];
      if (said === null) return;
      if (cell.name !== NAME[lang](said)) fail('accessible-name', `${lang} cell ${JSON.stringify(cell.text)} is named ${JSON.stringify(cell.name)}, want ${JSON.stringify(NAME[lang](said))}`);
      if (cell.kind !== `BUTTON/button/${lang}`) fail('accessible-name', `${lang} speaker on ${JSON.stringify(cell.text)} is ${cell.kind}, want a real button declared in the interface language`);
    });

    // A second press on the speaker that is speaking stops it.
    const spoken = SAID.length;
    await speakers.nth(0).click();
    await handoffs(page, spoken + 1);
    let state = await page.evaluate(() => ({
      speaking: document.querySelectorAll('[data-speaking]').length,
      name: document.querySelector('.y-cellspeak[data-speaking]')?.getAttribute('aria-label'),
      cancels: window.__speechFixture.cancels,
    }));
    if (state.speaking !== 1 || state.name !== STOP[lang]) throw new Error(`${lang} a speaking cell shows ${JSON.stringify(state)}, want one speaker named ${STOP[lang]}`);
    await speakers.nth(0).click();
    await settle(page);
    const after = await page.evaluate(() => ({
      speaking: document.querySelectorAll('[data-speaking]').length,
      name: document.querySelector('.y-cellspeak')?.getAttribute('aria-label'),
      cancels: window.__speechFixture.cancels,
      handed: window.__speechFixture.utterances.length,
    }));
    if (after.handed !== spoken + 1 || after.speaking !== 0 || after.cancels <= state.cancels || after.name !== NAME[lang](SAID[0])) {
      fail('second-press-stops', `${lang} after the second press ${JSON.stringify(after)}, want no new utterance, no speaking speaker, a cancel, and the idle name ${NAME[lang](SAID[0])}`);
    }

    // The table is laid out as it was without the speakers. The windows are tall
    // enough that neither page scrolls, so a scrollbar cannot narrow one of them.
    if (lang === 'zh-Hant') {
      const bare = await browser.newContext({ viewport: { width: 1280, height: 2400 } });
      await bare.addInitScript(() => { delete window.speechSynthesis; });
      const plain = await bare.newPage();
      await plain.goto(BASE + PAGE, { waitUntil: 'networkidle' });
      if (await plain.locator(CELL).count() !== 0) throw new Error('a page with no speech engine still offers cell speakers');
      const without = JSON.stringify(await geometry(plain));
      const withSpeakers = JSON.stringify(await geometry(page));
      await bare.close();
      if (without !== withSpeakers) fail('no-layout-shift', `the table is ${withSpeakers} with speakers and ${without} without them`);
    }
    await context.close();
  }
  console.log('PASS speak-cells: kana and furigana cells speak in Japanese through the page\'s own voice choice, other cells stay silent, a second press stops, and the table keeps its layout');
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
