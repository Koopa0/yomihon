// Behavior lock: switching the chrome's language mid-note returns the reader
// to the place they were reading. The form posts a path; without a position
// on that address the reload lands at the top of a long note.
//
// The destination note is long on purpose. On a note that fits in one screen,
// "the reader came back where they were" is already true before anything
// scrolls, so the arrival check could not have failed.
//
// Env: YOMIHON_BASE, PAGE_PATH (Glass Tide), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['position-survives-switch', 'an-arrival-paints', 'position-survives-preferences'];
const TARGET_Y = 600;
const SLACK_FLOOR = 700;
const TOLERANCE = 48;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN language-scroll-restore: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL language-scroll-restore: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN language-scroll-restore: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED language-scroll-restore: ${message}`); };

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/langform.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

// Putting the navigation opt-in back is the regression this watches for. The
// rule is appended rather than found, because the repair removed it and there
// is nothing left to rewrite.
const restoreNavigationTransition = async (page) => {
  let stylesheets = 0;
  await page.route('**/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    stylesheets += 1;
    return route.fulfill({ response, body: `${original}\n@view-transition{navigation:auto}` });
  });
  return () => (stylesheets === 0 ? 'the stylesheet was never requested, so the rule was never put back' : '');
};

const MUTATIONS = {
  'restore-the-navigation-transition': {
    target: 'an-arrival-paints',
    apply: restoreNavigationTransition,
  },
  // The defect itself: next stays the path alone, so the redirect has no
  // position to restore and the reader arrives at the top.
  // The second door: the walk out to the reading choices carries the return
  // address, and the position rides on it. Stripped, the redirect has nowhere
  // to put the reader but the top.
  'leave-the-return-address-bare': {
    target: 'position-survives-preferences',
    apply: rewriteModule(
      "  address.searchParams.set('from', pathOnly(from) + '#' + MARK + y);",
      "  address.searchParams.set('from', pathOnly(from));",
      'preferences link return-address rewrite',
    ),
  },
  'leave-next-as-the-path': {
    target: 'position-survives-switch',
    apply: rewriteModule(
      "  next.value = path + '#' + MARK + y;",
      '  next.value = path;',
      'language-form next rewrite',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`language-scroll-restore: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`language-scroll-restore: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`language-scroll-restore: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const waitSettled = (page) => page.evaluate(() => new Promise((resolve) => {
  let settled = false;
  const finish = () => {
    if (settled) return;
    settled = true;
    resolve();
  };
  setTimeout(finish, 600);
  const afterPaint = () => requestAnimationFrame(() => requestAnimationFrame(finish));
  afterPaint();
  window.addEventListener('pagereveal', () => {
    afterPaint();
  }, { once: true });
}));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // Every document this page receives records its own arrival, from inside,
  // before anything else runs: whether it was revealed at all, and whether it
  // arrived inside a transition. Nothing outside the document can ask this
  // afterwards — the announcement has already been made or already been missed.
  await page.addInitScript(() => {
    window.__arrival = { reveal: false, transition: false };
    window.addEventListener('pagereveal', (event) => {
      window.__arrival.reveal = true;
      window.__arrival.transition = Boolean(event.viewTransition);
    }, { once: true });
  });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!response || response.status() !== 200) {
    broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  }
  await page.waitForSelector('html[data-js]');
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const buttons = await page.locator('.y-langform .y-langbtn').count();
  if (buttons !== 1) broken(`the page carries ${buttons} language controls, want exactly 1`);

  const slack = await page.evaluate(() =>
    document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight);
  if (slack < SLACK_FLOOR) {
    broken(`the document has ${slack}px of slack, want at least ${SLACK_FLOOR} so a mid-note switch has somewhere to fall from`);
  }

  await page.evaluate((y) => { window.scrollTo(0, y); }, TARGET_Y);
  const before = await page.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
  }));
  if (before.y < TARGET_Y - 8) {
    broken(`scrolled to ${before.y}, want near ${TARGET_Y}; the page did not travel`);
  }

  await page.locator('.y-langform .y-langbtn').click();
  const switched = await page.waitForFunction(
    (from) => document.documentElement.getAttribute('lang') !== from,
    before.lang,
    { timeout: 10_000 },
  ).then(() => true, () => false);
  if (!switched) {
    broken(`the language stayed ${JSON.stringify(before.lang)} after clicking the language control`);
  }
  await page.waitForSelector('html[data-js]');
  await waitSettled(page);

  const after = await page.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
  }));
  if (after.lang === before.lang) {
    broken(`the language stayed ${JSON.stringify(after.lang)} after the switch, so this run never left the page`);
  }
  if (Math.abs(after.y - before.y) > TOLERANCE) {
    fail(
      'position-survives-switch',
      `after switching ${before.lang} → ${after.lang} the page is at scrollY=${after.y}, want near ${before.y} (within ${TOLERANCE}px)`,
    );
  }

  // A page reached by following a link has to paint. A navigation transition
  // holds the arriving document until it is revealed, and where that reveal
  // does not come the page is complete, scripted, laid out, and never shown —
  // with nothing left in it able to notice, because the signal that is missing
  // is the one a script would have to wait for. The reading choices are behind
  // the only link the chrome offers on every page, so that is the arrival this
  // walks into.
  // A held arrival has two shapes and this site owns both. The lighter one
  // finishes loading and never paints; the heavier one never finishes loading
  // at all, and reaching the readings below would already be impossible. Left
  // uncaught, that heavier shape kills the run before any assertion speaks,
  // which reads as a broken probe rather than as the page a reader cannot see.
  try {
    await Promise.all([
      page.waitForURL('**/preferences**'),
      page.locator('.y-prefslink').click(),
    ]);
  } catch (held) {
    fail(
      'an-arrival-paints',
      `the page reached by following a link never finished loading: ${String(held.message).split('\n')[0]}`,
    );
  }
  const arrival = await page.evaluate(() => new Promise((resolve) => {
    let painted = false;
    requestAnimationFrame(() => { painted = true; });
    setTimeout(() => resolve({
      painted,
      reveal: window.__arrival?.reveal ?? null,
      transition: window.__arrival?.transition ?? null,
      href: location.pathname,
    }), 1000);
  }));
  if (!arrival.painted || arrival.reveal !== true || arrival.transition !== false) {
    fail(
      'an-arrival-paints',
      `the page reached by following a link reports painted=${arrival.painted} revealed=${arrival.reveal} arrived-in-a-transition=${arrival.transition} at ${arrival.href}; want a page that was revealed, painted a frame, and came through no transition`,
    );
  }

  // The same promise through the other door: out to the reading choices, a
  // language chosen there, and back. It is a link rather than a form and a
  // page in between, so nothing about the first phase proves this one. A page
  // of its own, because the mutation has to be shown to have reached the
  // document that is being measured rather than inherited from the first.
  const prefsPage = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  // Every document this page loads records its own arrival: whether one came
  // with a view transition, and whether that transition ever finished. The
  // listener has to be installed before the document exists, because pagereveal
  // has already fired by the time anything outside can ask.
  await prefsPage.addInitScript(() => {
    window.__arrival = { reveal: false, transition: false, finished: false };
    window.addEventListener('pagereveal', (event) => {
      window.__arrival.reveal = true;
      window.__arrival.transition = Boolean(event.viewTransition);
      event.viewTransition?.finished.then(
        () => { window.__arrival.finished = true; },
        () => { window.__arrival.finished = 'rejected'; },
      );
    }, { once: true });
  });
  const prefsProof = MUTATE ? await MUTATIONS[MUTATE].apply(prefsPage) : null;
  const prefsResponse = await prefsPage.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!prefsResponse || prefsResponse.status() !== 200) {
    broken(`${PAGE} returned ${prefsResponse?.status() ?? 'no response'} for the preferences route, want 200`);
  }
  await prefsPage.waitForSelector('html[data-js]');
  if (prefsProof) {
    const issue = prefsProof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const prefsSlack = await prefsPage.evaluate(() =>
    document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight);
  if (prefsSlack < SLACK_FLOOR) {
    broken(`the document has ${prefsSlack}px of slack on the preferences route, want at least ${SLACK_FLOOR}`);
  }
  await prefsPage.evaluate((y) => { window.scrollTo(0, y); }, TARGET_Y);
  const leaving = await prefsPage.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
  }));
  if (leaving.y < TARGET_Y - 8) {
    broken(`scrolled to ${leaving.y} before leaving for the reading choices, want near ${TARGET_Y}`);
  }

  const prefsLinks = await prefsPage.locator('.y-prefslink').count();
  if (prefsLinks !== 1) broken(`the page carries ${prefsLinks} links to the reading choices, want exactly 1`);
  await Promise.all([
    prefsPage.waitForURL('**/preferences**'),
    prefsPage.locator('.y-prefslink').click(),
  ]);
  // The address changes when the navigation commits, which is before the
  // arrival has finished painting. Everything below presses controls on that
  // page, so it waits for the page to come to rest the same way the return
  // does.
  await prefsPage.waitForSelector('html[data-js]');
  await waitSettled(prefsPage);

  const languageForm = prefsPage.locator('form.y-preffield').filter({
    has: prefsPage.locator('input[type=radio][name="lang"]'),
  });
  // A reader picks a language by pressing its name, so that is what this
  // presses. Driving the radio instead asks a 14px control to satisfy an
  // actionability check no reader has to satisfy, and pressing the label does
  // not depend on it. Driving it that way timed out once on the Linux runner,
  // waiting for the input to be visible, enabled and stable; that timeout was
  // never reproduced — on macOS, or in a container running the same Chrome
  // major as the runner — so the reason it happened is not recorded here.
  const otherLanguage = languageForm.locator('label:has(input[type=radio][name="lang"]:not(:checked))');
  if (await otherLanguage.count() !== 1) {
    broken('the reading choices offer no second language to pick, so this route cannot change one');
  }
  // The press is bounded and, when it does not land, says everything about the
  // page that could explain why. The runner has refused this press with nothing
  // but "waiting for element to be visible, enabled and stable", and the job
  // keeps no screenshots, so stdout is the only way anything about that moment
  // reaches anyone. None of this prints on a press that lands.
  try {
    await otherLanguage.click({ timeout: 10_000 });
  } catch (refused) {
    const boxes = [];
    for (let sample = 0; sample < 3; sample += 1) {
      boxes.push(await otherLanguage.boundingBox().catch(() => null));
      await prefsPage.waitForTimeout(100);
    }
    // A frame callback is what Playwright's own stability check waits on, and
    // nothing else here needs one — the readings below are taken synchronously,
    // so they answer even on a document that has stopped painting. Asking
    // whether a frame ever arrives is therefore the one reading that separates
    // "the element moved" from "the page stopped".
    const frames = await prefsPage.evaluate(() => new Promise((resolve) => {
      let delivered = false;
      requestAnimationFrame(() => { delivered = true; resolve('alive'); });
      setTimeout(() => resolve(delivered ? 'alive' : 'none in 1s'), 1000);
    })).catch((unreadable) => `unreadable: ${unreadable}`);
    const page = await prefsPage.evaluate(() => {
      const label = document.querySelector('form.y-preffield label:has(input[type=radio][name="lang"]:not(:checked))');
      const shape = (element) => {
        if (!element) return null;
        const style = getComputedStyle(element);
        return {
          display: style.display,
          visibility: style.visibility,
          opacity: style.opacity,
          transform: style.transform,
        };
      };
      return {
        label: shape(label),
        form: shape(label?.closest('form')),
        animations: document.getAnimations().map((animation) => ({
          kind: animation.constructor.name,
          state: animation.playState,
          on: animation.effect?.target?.tagName ?? null,
          pseudo: animation.effect?.pseudoElement ?? null,
        })),
        // Absent in some builds, where it reads the same as "no transition":
        // it cannot rule one out, which is why the arrival records its own.
        activeViewTransition: Boolean(document.activeViewTransition),
        arrival: window.__arrival ?? null,
        visibility: document.visibilityState,
        ready: document.readyState,
        href: location.href,
        viewport: [window.innerWidth, window.innerHeight],
        fields: document.querySelectorAll('.y-preffield').length,
      };
    }).catch((unreadable) => ({ unreadable: String(unreadable) }));
    broken(`the language could not be pressed: ${String(refused.message).split('\n')[0]}; frames=${frames}; boxes=${JSON.stringify(boxes)}; page=${JSON.stringify(page)}`);
  }
  // The press has to have chosen it. A click that landed somewhere harmless
  // would otherwise submit the language already in force, and the position
  // would survive a round trip that changed nothing.
  const picked = languageForm.locator('input[type=radio][name="lang"]:checked');
  if (await picked.getAttribute('value') === leaving.lang) {
    broken(`pressing the other language left ${JSON.stringify(leaving.lang)} chosen, so nothing was picked`);
  }
  await Promise.all([
    prefsPage.waitForURL(`**${PAGE}**`),
    languageForm.locator('button[type=submit]').click(),
  ]);
  await prefsPage.waitForSelector('html[data-js]');
  await waitSettled(prefsPage);

  // The address, the place, and the room there was to travel in — a failure
  // here is one of two different illnesses and only these three tell them
  // apart: an address arriving without its position means the return address
  // lost it, while a position that arrived on a document with too little room
  // to hold it means the page was measured before it had grown.
  const returned = await prefsPage.evaluate(() => ({
    y: window.scrollY,
    lang: document.documentElement.getAttribute('lang'),
    href: location.href,
    slack: document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight,
  }));
  if (returned.lang === leaving.lang) {
    broken(`the language stayed ${JSON.stringify(returned.lang)} through the reading choices, so this run changed nothing`);
  }
  if (Math.abs(returned.y - leaving.y) > TOLERANCE) {
    fail(
      'position-survives-preferences',
      `after choosing ${leaving.lang} → ${returned.lang} in the reading choices the page is at scrollY=${returned.y}, want near ${leaving.y} (within ${TOLERANCE}px); arrived at ${returned.href} with ${returned.slack}px of slack`,
    );
  }

  console.log(`PASS language-scroll-restore: a mid-note language switch (${before.lang} → ${after.lang}) returned at scrollY=${after.y} from ${before.y}; a page reached by following a link was revealed and painted; the same through the reading choices (${leaving.lang} → ${returned.lang}) returned at scrollY=${returned.y} from ${leaving.y}`);
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
    }
    process.exitCode = 1;
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
