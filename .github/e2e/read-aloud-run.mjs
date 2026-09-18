// Behavior lock for a note read through. A reader who presses play through
// hears every marked paragraph in the order the author wrote them, sees which
// one the voice is on, and is told where they are at each move — speech
// synthesis reports no reliable duration, so the paragraph is the only unit
// there is to count in. The ends refuse rather than wrap, and stop reaches the
// voice itself rather than only the page's account of it.
//
// Env: YOMIHON_BASE, PAGE_PATH (the L02 fixture, three marked paragraphs), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L02.md';
const MUTATE = process.env.MUTATE || '';
const PLAY = '.y-ttsbar__play';
const STOP = '.y-ttsbar__stop';
const PREVIOUS = '.y-ttsbar__prev';
const NEXT = '.y-ttsbar__next';
const STATUS = '.y-ttsbar__status';

// The fixture's three marked paragraphs and what the bar says at each move,
// written out rather than read back off the page: an expectation taken from the
// element the code reads would agree with the code however wrong both were.
const PARAGRAPHS = ['一つ目の段落です。', '二つ目の段落です。', '三つ目の段落です。'];
const PROGRESS = ['第 1 段，共 3 段', '第 2 段，共 3 段', '第 3 段，共 3 段'];
const FINISHED = '播放完成';
const STOPPED = '已停止';

const SITES = [
  'the-ends-refuse-rather-than-wrap',
  'paragraphs-speak-in-order',
  'the-speaking-paragraph-is-marked',
  'the-region-counts-the-paragraphs',
  'stop-ends-the-reading',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN read-aloud-run: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL read-aloud-run: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN read-aloud-run: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED read-aloud-run: ${message}`); };

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

const MUTATIONS = {
  // The defect the whole page exists to rule out: each paragraph is its own act
  // again, and the note never plays past its first marked paragraph.
  'stop-advancing-on-end': {
    target: 'paragraphs-speak-in-order',
    apply: rewriteScript(
      '      if (running) {\n        advance();\n        return;\n      }\n',
      '',
      'advance when an utterance ends',
    ),
  },
  // Off by one: the colour sits on the paragraph the voice is about to reach
  // rather than the one it is reading, so a reader following along reads ahead
  // of the voice for the whole note.
  'mark-the-next-paragraph': {
    target: 'the-speaking-paragraph-is-marked',
    apply: rewriteScript(
      "      markReading(trigger.closest('.y-reading'));",
      "      markReading(trigger.closest('.y-reading')?.nextElementSibling ?? null);",
      'the paragraph the voice is on',
    ),
  },
  // The sentence arrives with its two places unfilled, which is the shape a
  // reader meets when the count is dropped from the announcement.
  'announce-without-the-count': {
    target: 'the-region-counts-the-paragraphs',
    apply: rewriteScript(
      "    announce(progressTemplate\n      .replace('{n}', String(index + 1))\n      .replace('{total}', String(readingButtons.length)));",
      '    announce(progressTemplate);',
      'the progress announcement',
    ),
  },
  // Stop reaches the page's own account of the reading but never the voice, so
  // the bar goes quiet and the speaker carries on talking over it.
  'keep-speaking-after-stop': {
    target: 'stop-ends-the-reading',
    apply: rewriteScript(
      '    speechSynthesis.cancel();\n    resetSpeakButton();',
      '    resetSpeakButton();',
      'the cancel inside stop',
    ),
  },
  // Next stays live on the last paragraph. Nothing then happens when it is
  // pressed, which is a control offering a move the note does not have.
  'next-stays-live-at-the-end': {
    target: 'the-ends-refuse-rather-than-wrap',
    apply: rewriteScript(
      '    setStep(nextButton, cursor >= readingButtons.length - 1);',
      '    setStep(nextButton, false);',
      'the end of the note',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`read-aloud-run: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`read-aloud-run: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`read-aloud-run: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// A real voice starts a moment after it is asked to and ends on its own; both
// arrive as events rather than as return values, and the bar has to hold what
// it said across that gap. The end is held here instead of fired on a timer so
// the walk moves exactly when this probe says so.
const settle = (page) => page.evaluate(() => new Promise((resolve) => { setTimeout(resolve, 0); }));

const observe = (page) => page.evaluate((selectors) => ({
  spoken: [...window.__spoken],
  calls: [...window.__calls],
  marked: document.querySelector('[data-reading] [data-tts]')?.getAttribute('data-tts') ?? null,
  markedCount: document.querySelectorAll('[data-reading]').length,
  region: document.querySelector(selectors.status)?.textContent ?? null,
  pressed: document.querySelector(selectors.play)?.getAttribute('aria-pressed') ?? null,
  previousDisabled: document.querySelector(selectors.previous)?.disabled ?? null,
  nextDisabled: document.querySelector(selectors.next)?.disabled ?? null,
}), { status: STATUS, play: PLAY, previous: PREVIOUS, next: NEXT });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // Headless Chrome need not produce audio, and whether a Japanese voice is
  // installed is not this lock's business. What the page decides is: the calls
  // it makes at the speech boundary and what it does with the events a voice
  // sends back, both recorded here. Cancelling ends the utterance that was
  // speaking, the way a browser does, so a page that mistakes that ending for
  // an ordinary one has somewhere to show it.
  await page.addInitScript(() => {
    window.__spoken = [];
    window.__calls = [];
    window.__pending = null;
    speechSynthesis.speak = (utterance) => {
      window.__calls.push('speak');
      window.__spoken.push(utterance.text);
      window.__pending = utterance;
      setTimeout(() => utterance.dispatchEvent(new Event('start')), 0);
    };
    speechSynthesis.cancel = () => {
      window.__calls.push('cancel');
      const cancelled = window.__pending;
      window.__pending = null;
      if (cancelled) setTimeout(() => cancelled.dispatchEvent(new Event('end')), 0);
    };
    window.__end = () => {
      const utterance = window.__pending;
      window.__pending = null;
      if (!utterance) return false;
      utterance.dispatchEvent(new Event('end'));
      return true;
    };
  });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  await page.waitForSelector(PLAY, { state: 'attached', timeout: 2000 });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  const speakers = await page.locator('[data-tts]').count();
  if (speakers !== PARAGRAPHS.length) {
    broken(`the note marks ${speakers} paragraphs for reading aloud, want ${PARAGRAPHS.length}; the fixture and this lock have to agree before anything below means anything`);
  }

  const atRest = await observe(page);
  if (atRest.previousDisabled !== true) {
    fail('the-ends-refuse-rather-than-wrap', `before anything is read, previous is disabled=${atRest.previousDisabled}, want true: there is no paragraph before the first`);
  }
  if (atRest.nextDisabled !== false) {
    fail('the-ends-refuse-rather-than-wrap', `before anything is read, next is disabled=${atRest.nextDisabled}, want false: the first paragraph is ahead of the reader`);
  }

  // Play through, then let each utterance end the way a voice would. The walk
  // is observed after every ending, so the order below is the order a listener
  // would hear rather than a tally taken once at the end.
  await page.click(PLAY);
  await settle(page);
  const steps = [await observe(page)];
  for (let i = 0; i < PARAGRAPHS.length + 2; i += 1) {
    if (!await page.evaluate(() => window.__end())) break;
    await settle(page);
    steps.push(await observe(page));
  }
  const heard = steps[steps.length - 1].spoken;
  if (heard.join('␟') !== PARAGRAPHS.join('␟')) {
    fail('paragraphs-speak-in-order', `the note read through spoke ${JSON.stringify(heard)}, want ${JSON.stringify(PARAGRAPHS)}: every marked paragraph once, in the order the author wrote them`);
  }
  if (steps.length !== PARAGRAPHS.length + 1) {
    broken(`the reading was observed at ${steps.length} moments, want ${PARAGRAPHS.length + 1}: one for each paragraph and one after the last`);
  }

  for (const [index, paragraph] of PARAGRAPHS.entries()) {
    const step = steps[index];
    if (step.markedCount !== 1) {
      fail('the-speaking-paragraph-is-marked', `while paragraph ${index + 1} was being read the page marked ${step.markedCount} paragraphs, want exactly 1`);
    }
    if (step.marked !== paragraph) {
      fail('the-speaking-paragraph-is-marked', `while paragraph ${index + 1} was being read the page marked ${JSON.stringify(step.marked)}, want ${JSON.stringify(paragraph)}`);
    }
  }
  const afterLast = steps[PARAGRAPHS.length];
  if (afterLast.markedCount !== 0) {
    fail('the-speaking-paragraph-is-marked', `${afterLast.markedCount} paragraphs are still marked after the reading ended, want none: the colour stands for a voice that is no longer anywhere`);
  }

  for (const [index, said] of PROGRESS.entries()) {
    if (steps[index].region !== said) {
      fail('the-region-counts-the-paragraphs', `while paragraph ${index + 1} was being read the bar said ${JSON.stringify(steps[index].region)}, want ${JSON.stringify(said)}`);
    }
  }
  if (afterLast.region !== FINISHED) {
    fail('the-region-counts-the-paragraphs', `at the end of the note the bar said ${JSON.stringify(afterLast.region)}, want ${JSON.stringify(FINISHED)}`);
  }

  if (afterLast.nextDisabled !== true) {
    fail('the-ends-refuse-rather-than-wrap', `at the end of the note next is disabled=${afterLast.nextDisabled}, want true: there is no paragraph after the last, and a control that refuses is not the same as one that does nothing`);
  }
  if (afterLast.previousDisabled !== false) {
    fail('the-ends-refuse-rather-than-wrap', `at the end of the note previous is disabled=${afterLast.previousDisabled}, want false: the reader can go back`);
  }

  // Stop, mid-reading. What has to end is the voice, not merely the bar's
  // account of it: a page that stops saying anything while a speaker talks on
  // is the worse of the two failures, because the reader cannot find the
  // control that would silence it.
  await page.click(PLAY);
  await settle(page);
  const beforeStop = await observe(page);
  if (beforeStop.spoken.length !== PARAGRAPHS.length + 1) {
    broken(`pressing play through a second time spoke ${beforeStop.spoken.length - PARAGRAPHS.length} paragraphs, want 1 to interrupt`);
  }
  await page.click(STOP);
  await settle(page);
  const afterStop = await observe(page);
  if (!afterStop.calls.slice(beforeStop.calls.length).includes('cancel')) {
    fail('stop-ends-the-reading', `stop made ${JSON.stringify(afterStop.calls.slice(beforeStop.calls.length))} at the speech boundary, want a cancel among them: the voice has to be told, not just the page`);
  }
  if (afterStop.spoken.length !== beforeStop.spoken.length) {
    fail('stop-ends-the-reading', `${afterStop.spoken.length - beforeStop.spoken.length} more paragraphs were spoken after stop, want none`);
  }
  if (afterStop.region !== STOPPED) {
    fail('stop-ends-the-reading', `after stop the bar said ${JSON.stringify(afterStop.region)}, want ${JSON.stringify(STOPPED)}`);
  }
  if (afterStop.pressed !== 'false') {
    fail('stop-ends-the-reading', `after stop play through still reports aria-pressed=${JSON.stringify(afterStop.pressed)}, want "false": the bar would be claiming to read a note nothing is reading`);
  }
  if (afterStop.markedCount !== 0) {
    fail('stop-ends-the-reading', `${afterStop.markedCount} paragraphs are still marked after stop, want none`);
  }

  // The reader who walks back to the first paragraph from the keyboard is
  // standing on the control that the arrival switches off. Where they are left
  // standing is the whole question: on the document, their next Tab starts
  // again from the top of the page.
  await page.focus(NEXT);
  await page.keyboard.press('Enter');
  await settle(page);
  const steppedOn = await observe(page);
  if (steppedOn.previousDisabled !== false) {
    broken(`after moving forward one paragraph previous is disabled=${steppedOn.previousDisabled}, want false; there is nothing to press back from`);
  }
  await page.focus(PREVIOUS);
  await page.keyboard.press('Enter');
  await settle(page);
  const landed = await page.evaluate((selectors) => ({
    inBar: Boolean(document.activeElement?.closest?.('.y-ttsbar')),
    on: document.activeElement?.className ?? document.activeElement?.tagName ?? null,
    previousDisabled: document.querySelector(selectors.previous)?.disabled ?? null,
  }), { previous: PREVIOUS });
  if (landed.previousDisabled !== true) {
    fail('the-ends-refuse-rather-than-wrap', `back at the first paragraph previous is disabled=${landed.previousDisabled}, want true`);
  }
  if (!landed.inBar) {
    fail('the-ends-refuse-rather-than-wrap', `pressing previous back to the first paragraph left the keyboard on ${JSON.stringify(landed.on)}, want a control still inside the bar: the reader's place in the page goes with the control that switched off under them`);
  }

  console.log('PASS read-aloud-run: the note reads through in the author\'s order, marks the paragraph the voice is on and no other, counts each one aloud and says so at the end, refuses at both ends rather than wrapping, and stops the voice itself');
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
