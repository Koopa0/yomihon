// Browser lock for the control that shows and hides the readings over Japanese
// text. The control is a switch with its name beside it, and the name comes in
// two lengths: the stylesheet picks one by width, so which words a reader
// actually sees is decided by CSS and cannot be read off the server's bytes.
//
// Three widths stand for the bands the stylesheet draws. At each one the
// control must show exactly one of its two lengths, that length must be the
// one the band is for, and it must be contained in the name the browser
// computes for the control — a reader who asks for a control by the words in
// front of them is asking with the visible label, so a name that has drifted
// away from it is a control they cannot reach by name.
//
// Which way the switch is set is not a word. It is `aria-pressed` on the button,
// which is what assistive technology reads, and the track drawn from that same
// attribute, which is what a sighted reader reads; the name stays the same in
// both states, because a control renamed by its own state is announced twice.
// Both are held at each width, with the readings on and then off. The run ends
// by asking the same three widths again in English — the one language the
// recorded pages say nothing about.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

// The literal words, not the constants that render them: a lock that asked the
// page what it says would agree with the page whatever the page said.
const LONG_LABEL = '顯示讀音';
const SHORT_LABEL = '讀音';
// The English side carries the same word at both lengths, with no short form.
// It is held here because the recorded pages are all in Traditional Chinese,
// so nothing else in the tree can say what the English control reads.
const ENGLISH_LABEL = 'Readings';

// One width per band the stylesheet draws, named by what the reader gets there.
// The two bands are split at 720px; 390px is a phone, held apart from 620px so a
// rule that only reaches a tablet cannot stand in for the narrow band.
const BANDS = [
  { width: 1280, label: LONG_LABEL },
  { width: 620, label: SHORT_LABEL },
  { width: 390, label: SHORT_LABEL },
];

const SITES = [
  'one-label',
  'label-for-width',
  'label-in-name',
  'state-announced',
  'state-by-track',
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
const injectRule = async (page, selector, declarations, media, owner = selector) => {
  if (await page.locator(owner).count() === 0) {
    notApplied(`no element matches ${owner}, so the injected rule styles nothing`);
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
  // The pressed state taken off the control however it is set, so what
  // assistive technology reads says nothing about which way the switch is.
  'pressed-state-stripped': {
    target: 'state-announced',
    apply: (page) => page.evaluate(() => {
      const button = document.querySelector('.y-rubybtn');
      const strip = () => button.removeAttribute('aria-pressed');
      strip();
      new MutationObserver(strip).observe(button, { attributes: true, attributeFilter: ['aria-pressed'] });
    }),
    proof: async (page) => (await page.locator('.y-rubybtn').getAttribute('aria-pressed')) === null ? '' : 'the control still carries aria-pressed',
    proofWidth: 1280,
  },
  // The pressed state pinned on, so it stops following the readings.
  'pressed-state-stuck-on': {
    target: 'state-announced',
    apply: (page) => page.evaluate(() => {
      const button = document.querySelector('.y-rubybtn');
      const pin = () => { if (button.getAttribute('aria-pressed') !== 'true') button.setAttribute('aria-pressed', 'true'); };
      pin();
      new MutationObserver(pin).observe(button, { attributes: true, attributeFilter: ['aria-pressed'] });
    }),
    proof: async (page) => {
      // Switched off by the control itself, then read: a pin that had not taken
      // hold would show the attribute following the readings.
      await setReadings(page, 'off');
      const pressed = await page.locator('.y-rubybtn').getAttribute('aria-pressed');
      await setReadings(page, 'on');
      return pressed === 'true' ? '' : `with the readings off the control carries aria-pressed=${pressed}`;
    },
    proofWidth: 1280,
  },
  // The drawn track made the same in both states, so a sighted reader is left
  // with a switch that does not show which way it is set.
  'track-ignores-the-state': {
    target: 'state-by-track',
    apply: (page) => injectRule(page, '.y-rubybtn[aria-pressed]::before', 'background: var(--fg) !important; box-shadow: none !important;', undefined, '.y-rubybtn'),
    proof: async (page) => {
      await setReadings(page, 'off');
      const fill = await trackFill(page);
      await setReadings(page, 'on');
      return fill !== TRANSPARENT ? '' : 'with the readings off the track is still unfilled';
    },
    proofWidth: 1280,
  },
};

const visibleLabels = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-rubybtn__label')]
    .filter((element) => getComputedStyle(element).display !== 'none')
    .map((element) => element.textContent));

// The fill the track is drawn with. An unset switch is a hollow track, which the
// browser reports as a fully transparent background. The track fades between its
// two fills, so it is read once every transition on the control has landed:
// asked mid-fade it reports a colour that is neither state.
const TRANSPARENT = 'rgba(0, 0, 0, 0)';
const trackFill = (page) => page.locator('.y-rubybtn').evaluate(async (element) => {
  await Promise.all(element.getAnimations({ subtree: true }).map((animation) => animation.finished));
  return getComputedStyle(element, '::before').backgroundColor;
});

// The name the browser computes, not the attribute someone wrote: an attribute
// that has been removed leaves a name assembled from the contents, and reading
// the attribute would report nothing where a reader hears something.
const computedName = async (page) => {
  const snapshot = await page.locator('.y-rubybtn').ariaSnapshot();
  const quoted = /^- button "((?:[^"\\]|\\.)*)"/.exec(snapshot.trim());
  if (!quoted) broken(`the control did not read as a named button: ${JSON.stringify(snapshot)}`);
  return quoted[1].replaceAll('\\"', '"');
};

// Whether the browser reports the control as pressed in the accessibility tree,
// which is where assistive technology reads it, rather than the attribute that
// was written.
const reportedPressed = async (page) => {
  const snapshot = await page.locator('.y-rubybtn').ariaSnapshot();
  const line = /^- button "(?:[^"\\]|\\.)*"(.*)$/.exec(snapshot.trim());
  if (!line) broken(`the control did not read as a named button: ${JSON.stringify(snapshot)}`);
  return line[1].includes('[pressed]');
};

// A listener does not hear case, so a spoken name that carries the visible
// word under different capitalization — the button label's title case against
// the accessible name's sentence case — still names the control by the words
// in front of the reader. The fold makes no difference for the Chinese labels,
// which carry no case at all.
const nameCarriesLabel = (name, label) => name.toLowerCase().includes(label.toLowerCase());

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

  const namesByBand = new Map();
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
      if (!nameCarriesLabel(name, shown[0])) {
        fail('label-in-name', `${where}: the visible ${JSON.stringify(shown[0])} is not inside the spoken name ${JSON.stringify(name)}`);
      }

      // The state is told by the attribute and by what the browser reports from
      // it, and the name does not move with it: both states are the one name.
      const pressed = await page.locator('.y-rubybtn').getAttribute('aria-pressed');
      const wantPressed = readings === 'on' ? 'true' : 'false';
      if (pressed !== wantPressed) {
        fail('state-announced', `${where}: aria-pressed is ${JSON.stringify(pressed)}, want ${JSON.stringify(wantPressed)}`);
      }
      if ((await reportedPressed(page)) !== (readings === 'on')) {
        fail('state-announced', `${where}: the accessibility tree reports the control as ${readings === 'on' ? 'not pressed' : 'pressed'}`);
      }
      const knownName = namesByBand.get(band.width);
      if (knownName === undefined) namesByBand.set(band.width, name);
      else if (knownName !== name) {
        fail('state-announced', `${where}: the control is named ${JSON.stringify(name)}, but was ${JSON.stringify(knownName)} with the readings the other way; a name that follows the state is announced twice`);
      }
      // The sighted reader's half: the track is hollow when the readings are off
      // and filled when they are on, drawn from the same attribute.
      const fill = await trackFill(page);
      if ((fill !== TRANSPARENT) !== (readings === 'on')) {
        fail('state-by-track', `${where}: the track is ${fill === TRANSPARENT ? 'hollow' : `filled (${fill})`}, want ${readings === 'on' ? 'filled' : 'hollow'}`);
      }
    }
  }

  // The same control in the other language, which the recorded pages do not
  // cover at all. The visible word is required to sit inside the spoken name
  // here too: a control read out as something other than what it visibly says
  // cannot be asked for by name.
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
    const name = await computedName(page);
    if (!nameCarriesLabel(name, shown[0])) {
      fail('label-in-name', `${where}: the visible ${JSON.stringify(shown[0])} is not inside the spoken name ${JSON.stringify(name)}`);
    }
  }

  console.log('PASS reading-switch-label: the readings control shows one label per width band, the band\'s own label, inside the name it is spoken by, with the state carried by aria-pressed and the drawn track rather than by a word or a rename');
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
