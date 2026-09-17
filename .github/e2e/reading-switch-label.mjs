// Browser lock for the control that shows and hides the readings over Japanese
// text. The control names itself in two lengths and the stylesheet picks one by
// width, so which words a reader actually sees is decided by CSS and cannot be
// read off the server's bytes: the recorded pages pin both lengths and the
// spoken name, and nothing in them can say which length is on screen.
//
// Three widths stand for the three bands the stylesheet draws. At each one the
// control must show exactly one of its two lengths, that length must be the
// one the band is for, and it must be contained in the name the browser
// computes for the control — a reader who asks for a control by the words in
// front of them is asking with the visible label, so a name that has drifted
// away from it is a control they cannot reach by name.
//
// Where the row is narrowest the 開/關 word does not fit, and the label carries
// the state by being struck through instead; that substitution is the whole
// reason the narrow band is allowed to drop the word, so both halves are held
// here. The floor is the width of the icon buttons the control sits among: the
// English side is a single character and would otherwise leave a box narrower
// than its neighbours and narrower than a finger, so the run ends by asking the
// same three widths again in English — the one language the recorded pages say
// nothing about, and the only one where that floor carries any weight.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

// The literal words, not the constants that render them: a lock that asked the
// page what it says would agree with the page whatever the page said.
const LONG_LABEL = '顯示讀音';
const SHORT_LABEL = '讀音';
const ON_WORD = '開';
const OFF_WORD = '關';
// The English side carries the mark at both lengths. It is held here because
// the recorded pages are all in Traditional Chinese, so nothing else in the
// tree can say what the English control reads.
const ENGLISH_LABEL = '振';
// The width every icon button in the header holds.
const ICON_WIDTH = 32;

// One width per band the stylesheet draws, named by what the reader gets there.
const BANDS = [
  { width: 1280, label: LONG_LABEL, stateWord: true },
  { width: 620, label: SHORT_LABEL, stateWord: true },
  { width: 390, label: SHORT_LABEL, stateWord: false },
];

const SITES = [
  'one-label',
  'label-for-width',
  'label-in-name',
  'state-word-placement',
  'state-by-strike',
  'button-floor',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN reading-switch-label: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL reading-switch-label: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN reading-switch-label: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED reading-switch-label: ${message}`); };

// A rule injected at a selector that matches nothing styles nothing, and the
// run would then blame the probe for a regression that was never installed.
const injectRule = async (page, selector, declarations, media) => {
  if (await page.locator(selector).count() === 0) {
    notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  }
  const rule = `${selector} { ${declarations} }`;
  await page.addStyleTag({ content: media ? `@media ${media} { ${rule} }` : rule });
};

const setAttribute = async (page, selector, name, value) => {
  const changed = await page.evaluate(({ selector: chosen, name: attribute, value: text }) => {
    const elements = [...document.querySelectorAll(chosen)];
    if (elements.length !== 1) return elements.length;
    elements[0].setAttribute(attribute, text);
    return 1;
  }, { selector, name, value });
  if (changed !== 1) notApplied(`${selector} matched ${changed} elements, want exactly 1`);
};

// Each mutation carries the proof that it reached the page. A style rule that
// applied is a computed style that moved; without that, a mutation silently
// doing nothing reads downstream as the probe letting a regression through.
const MUTATIONS = {
  // Both lengths on screen at once — the shape an absent hiding rule leaves.
  'show-both-labels': {
    target: 'one-label',
    apply: (page) => injectRule(page, '.y-rubybtn__label--short', 'display: inline !important;'),
    proof: async (page) => {
      const shown = await visibleLabels(page);
      return shown.length === 2 ? '' : `${shown.length} labels are visible at the reading width, want both`;
    },
    proofWidth: 1280,
  },
  // The long label kept where the row has no room for it.
  'long-label-where-the-row-is-narrow': {
    target: 'label-for-width',
    apply: async (page) => {
      await injectRule(page, '.y-rubybtn__label--long', 'display: inline !important;', '(max-width: 720px)');
      await injectRule(page, '.y-rubybtn__label--short', 'display: none !important;', '(max-width: 720px)');
    },
    proof: async (page) => {
      const shown = await visibleLabels(page);
      return shown.length === 1 && shown[0] === LONG_LABEL ? '' : `the narrow band shows ${JSON.stringify(shown)}`;
    },
    proofWidth: 620,
  },
  // The spoken name back to the term, while the visible words say otherwise.
  'spoken-name-drifts-from-the-label': {
    target: 'label-in-name',
    apply: (page) => setAttribute(page, '.y-rubybtn', 'aria-label', '切換振假名'),
    proof: async (page) => (await computedName(page)) === '切換振假名' ? '' : 'the computed name did not move',
    proofWidth: 1280,
  },
  // The state word forced back in where the row cannot hold it.
  'state-word-where-the-row-is-narrow': {
    target: 'state-word-placement',
    apply: (page) => injectRule(page, '.y-rubybtn__on', 'display: inline !important;', '(max-width: 520px)'),
    proof: async (page) => ((await visibleStateWords(page)).length > 0 ? '' : 'no state word became visible'),
    proofWidth: 390,
  },
  // The state word dropped where there is room for it, leaving the reading
  // widths with no word for which way the switch is set.
  'state-word-gone-where-the-row-is-wide': {
    target: 'state-word-placement',
    apply: (page) => injectRule(page, '.y-rubybtn__on, .y-rubybtn__off', 'display: none !important;'),
    proof: async (page) => ((await visibleStateWords(page)).length === 0 ? '' : 'the state word is still visible'),
    proofWidth: 1280,
  },
  // The narrow band's substitute for the word, taken away.
  'narrow-off-state-loses-its-strike': {
    target: 'state-by-strike',
    apply: (page) => injectRule(page, '.y-rubybtn__label', 'text-decoration: none !important;'),
    proof: async (page) => {
      const decoration = await page.locator('.y-rubybtn__label--short').evaluate((element) => getComputedStyle(element).textDecorationLine);
      return decoration === 'none' ? '' : `the label still reads ${decoration}`;
    },
    proofWidth: 390,
  },
  // The floor taken away, which is the whole of the regression: the Chinese
  // label is wider than the floor on its own and never rests on it, so this
  // only shows where the label is one character.
  'button-floor-removed': {
    target: 'button-floor',
    apply: (page) => injectRule(page, '.y-rubybtn', 'min-width: 0 !important;', '(max-width: 720px)'),
    proof: async (page) => {
      const floor = await page.locator('.y-rubybtn').evaluate((element) => getComputedStyle(element).minWidth);
      return floor === '0px' ? '' : `the control still holds a ${floor} floor`;
    },
    proofWidth: 390,
  },
};

const visibleLabels = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-rubybtn__label')]
    .filter((element) => getComputedStyle(element).display !== 'none')
    .map((element) => element.textContent));

const visibleStateWords = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-rubybtn__on, .y-rubybtn__off')]
    .filter((element) => getComputedStyle(element).display !== 'none')
    .map((element) => element.textContent));

const buttonWidth = (page) => page.locator('.y-rubybtn').evaluate((element) => element.getBoundingClientRect().width);

// The name the browser computes, not the attribute someone wrote: an attribute
// that has been removed leaves a name assembled from the contents, and reading
// the attribute would report nothing where a reader hears something.
const computedName = async (page) => {
  const snapshot = await page.locator('.y-rubybtn').ariaSnapshot();
  const quoted = /^- button "((?:[^"\\]|\\.)*)"/.exec(snapshot.trim());
  if (!quoted) broken(`the control did not read as a named button: ${JSON.stringify(snapshot)}`);
  return quoted[1].replaceAll('\\"', '"');
};

const strikesThrough = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-rubybtn__label')]
    .filter((element) => getComputedStyle(element).display !== 'none')
    .every((element) => getComputedStyle(element).textDecorationLine.includes('line-through')));

// Readings are switched by pressing the control, which is the path a reader
// takes; driving the attribute instead would assert against a value this probe
// had just written.
const setReadings = async (page, want) => {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    if (await page.evaluate(() => document.documentElement.dataset.ruby) === want) return;
    await page.locator('.y-rubybtn').click();
    await page.waitForTimeout(60);
  }
  broken(`the control would not switch the readings ${want}`);
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`reading-switch-label: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`reading-switch-label: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reading-switch-label: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);

  // The words below are the Traditional Chinese ones. Driven against a page in
  // the other language every comparison would be against words that are not
  // there, and the run would be red for the wrong reason.
  const lang = await page.evaluate(() => document.documentElement.lang);
  if (lang !== 'zh-Hant') broken(`the page is in ${lang}; this lock reads the Traditional Chinese labels`);
  if (await page.locator('.y-rubybtn').count() !== 1) broken('the readings control is not unique on this page');

  // A navigation drops an injected rule, so installing the mutation is a step
  // that runs again after each one rather than once at the top.
  const installMutation = async () => { if (MUTATE) await MUTATIONS[MUTATE].apply(page); };

  await installMutation();
  if (MUTATE) {
    await page.setViewportSize({ width: MUTATIONS[MUTATE].proofWidth, height: 900 });
    await page.waitForTimeout(80);
    const issue = await MUTATIONS[MUTATE].proof(page);
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  for (const band of BANDS) {
    await page.setViewportSize({ width: band.width, height: 900 });
    await page.waitForTimeout(80);
    for (const readings of ['on', 'off']) {
      await setReadings(page, readings);
      const where = `${band.width}px with readings ${readings}`;

      const shown = await visibleLabels(page);
      if (shown.length !== 1) {
        fail('one-label', `${where}: ${shown.length} labels are on screen (${JSON.stringify(shown)}), want exactly one`);
      }
      if (shown[0] !== band.label) {
        fail('label-for-width', `${where}: the control reads ${JSON.stringify(shown[0])}, want ${JSON.stringify(band.label)}`);
      }
      const name = await computedName(page);
      if (!name.includes(shown[0])) {
        fail('label-in-name', `${where}: the visible ${JSON.stringify(shown[0])} is not inside the spoken name ${JSON.stringify(name)}`);
      }

      const words = await visibleStateWords(page);
      const wantWord = band.stateWord ? [readings === 'on' ? ON_WORD : OFF_WORD] : [];
      if (JSON.stringify(words) !== JSON.stringify(wantWord)) {
        fail('state-word-placement', `${where}: the state word reads ${JSON.stringify(words)}, want ${JSON.stringify(wantWord)}`);
      }
      // Where the word is gone the strike is the only thing left saying which
      // way the switch is set, so it has to follow the state exactly — struck
      // through when the readings are off and plain when they are on.
      if (!band.stateWord) {
        const struck = await strikesThrough(page);
        if (struck !== (readings === 'off')) {
          fail('state-by-strike', `${where}: the label is ${struck ? 'struck through' : 'plain'} with no state word beside it`);
        }
      }

      const width = await buttonWidth(page);
      if (width < ICON_WIDTH - 0.5) {
        fail('button-floor', `${where}: the control is ${width}px wide, under the ${ICON_WIDTH}px its neighbours hold`);
      }
    }
  }

  // The same control in the other language, which the recorded pages do not
  // cover at all. The floor is held here rather than above because the Chinese
  // label is wider than the floor unaided and never rests on it: a floor that
  // had been deleted would leave every Chinese width still passing.
  //
  // The visible mark is deliberately not required to sit inside the spoken name
  // here. In English the name is the term and the mark is not part of it, which
  // is true of this control before and after this lock existed; holding it
  // would be asserting a change nobody has made.
  await page.context().addCookies([{ name: 'yomihon_lang', value: 'en', url: new URL(BASE).origin }]);
  await page.reload({ waitUntil: 'load' });
  const switched = await page.evaluate(() => document.documentElement.lang);
  if (switched !== 'en') broken(`the language cookie did not take: the page came back in ${switched}`);
  await installMutation();
  await setReadings(page, 'on');
  for (const band of BANDS) {
    await page.setViewportSize({ width: band.width, height: 900 });
    await page.waitForTimeout(80);
    const where = `${band.width}px in English`;

    const shown = await visibleLabels(page);
    if (shown.length !== 1) {
      fail('one-label', `${where}: ${shown.length} labels are on screen (${JSON.stringify(shown)}), want exactly one`);
    }
    if (shown[0] !== ENGLISH_LABEL) {
      fail('label-for-width', `${where}: the control reads ${JSON.stringify(shown[0])}, want ${JSON.stringify(ENGLISH_LABEL)}`);
    }
    const width = await buttonWidth(page);
    if (width < ICON_WIDTH - 0.5) {
      fail('button-floor', `${where}: the control is ${width}px wide, under the ${ICON_WIDTH}px its neighbours hold`);
    }
  }

  console.log('PASS reading-switch-label: the readings control shows one label per width band, the band\'s own label, inside the name it is spoken by, with the state carried by a word where there is room and by a strike where there is not');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else if (error instanceof ProbeBroken) {
    console.error(error.message);
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
