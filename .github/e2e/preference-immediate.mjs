// Behavior lock for the reading-choices page: a choice is applied and kept the
// moment it is picked, by the page itself, with nothing asked of the server.
//
// Four things are pinned here. A change sends no request, so there is no reply
// that could arrive out of order and undo a later choice — which is why two
// radios changed inside one frame can be required to leave two cookies and
// nothing in flight. A single change reaches both the root attribute the
// stylesheet reads and the cookie the next request is answered from. And a
// write this browser refuses is caught, put back, and said out loud, because a
// radio left sitting on a value nothing stored is the page telling the reader
// something untrue.
//
// The refusal is manufactured by replacing the cookie setter with one that
// drops writes, which is what a browser refusing this address its storage does
// — silently. The same stand-in is switched back on to check that the sentence
// it produced goes away when a later choice is kept.
//
// Env: YOMIHON_BASE, PAGE_PATH (the reading-choices page), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/preferences';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'no-request-on-a-change',
  'rapid-change-keeps-both',
  'applies-and-stores',
  'size-and-typeface-together',
  'refusal-puts-the-choice-back',
  'a-kept-choice-clears-the-sentence',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN preference-immediate: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL preference-immediate: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN preference-immediate: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED preference-immediate: ${message}`);
};

// Rewrite the runtime the page actually loads, rather than a stand-in for it.
// Requiring one exact match is what turns a needle gone stale — the source
// rewritten around it — into a reported failure instead of a mutation that
// quietly applied to nothing.
const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/preferences.js', async (route) => {
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

const MUTATIONS = {
  // The mechanism the ruling asked to be designed around: a save that goes to
  // the server has a reply, and a reply can arrive after a later choice.
  'change-posts-the-form': {
    target: 'no-request-on-a-change',
    apply: rewriteModule('    applyOption(name, option);\n', '    settingsForm.requestSubmit();\n', 'the write in the change handler'),
  },
  // The same hazard wearing a different coat: a save that holds the pick in
  // one slot and carries it out later keeps only the last one, so the field
  // changed first is written with the field changed second.
  'coalesce-the-writes': {
    target: 'rapid-change-keeps-both',
    apply: rewriteModule(
      '    applyOption(name, option);\n',
      '    globalThis.yomihonPending = { name, option };\n    queueMicrotask(() => applyOption(globalThis.yomihonPending.name, globalThis.yomihonPending.option));\n',
      'the write in the change handler',
    ),
  },
  'size-stores-nothing': {
    target: 'applies-and-stores',
    apply: rewriteModule("    setPreference('textsize', size);\n", '    root.dataset.textsize = size;\n', 'the size write'),
  },
  'typeface-stores-nothing': {
    target: 'size-and-typeface-together',
    apply: rewriteModule("      setPreference('font', value);\n", '      root.dataset.font = value;\n', 'the typeface write'),
  },
  // The one the whole refusal path rests on: believing the write instead of
  // reading it back leaves a refused choice looking chosen.
  'skip-the-read-back': {
    target: 'refusal-puts-the-choice-back',
    apply: rewriteModule(
      '    if (storesAValue(option) ? kept !== option.value : kept !== null) {\n',
      '    if (false) {\n',
      'the read-back verdict',
    ),
  },
  'keep-the-refusal-sentence': {
    target: 'a-kept-choice-clears-the-sentence',
    apply: rewriteModule("    if (status) status.textContent = '';\n", '', 'the sentence being cleared'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`preference-immediate: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`preference-immediate: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`preference-immediate: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// A browser that refuses this address its storage drops the write and says
// nothing, so the stand-in does exactly that: the getter stays honest and the
// setter is a no-op while the flag is up.
const REFUSE_COOKIES = `
  (() => {
    const own = Object.getOwnPropertyDescriptor(Document.prototype, 'cookie');
    window.__yomihonRefuse = true;
    Object.defineProperty(document, 'cookie', {
      configurable: true,
      get() { return own.get.call(document); },
      set(value) { if (!window.__yomihonRefuse) own.set.call(document, value); },
    });
  })();
`;

const rootState = (page) =>
  page.evaluate(() => {
    const root = document.documentElement;
    return {
      theme: root.dataset.theme ?? '',
      textsize: root.dataset.textsize ?? '',
      font: root.dataset.font ?? '',
      ruby: root.dataset.ruby ?? '',
      shortcuts: root.dataset.singleKeyShortcuts ?? '',
    };
  });

const cookies = async (context) => {
  const jar = {};
  for (const cookie of await context.cookies()) jar[cookie.name] = cookie.value;
  return jar;
};

// One choice, picked the way a reader picks it: a real click on the radio. A
// plain click rather than a checked-state helper, because a refused choice is
// meant to end up not checked and a helper that insisted otherwise would fail
// here instead of at the assertion that says why.
const pick = (page, field, value) => page.click(`[data-pref-field="${field}"] input[value="${value}"]`);

const checkedValue = (page, field) =>
  page.evaluate(
    (name) => document.querySelector(`[data-pref-field="${name}"] input:checked`)?.value ?? '',
    field,
  );

const statusOf = (page, field) =>
  page.evaluate((name) => {
    const line = document.querySelector(`[data-pref-field="${name}"] [data-pref-status]`);
    if (!line) return null;
    return { shown: line.textContent, served: line.dataset.prefRefused ?? '', live: line.getAttribute('aria-live') };
  }, field);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  // Every request this page makes while choices are being picked, so "nothing
  // was sent" is a claim about the wire rather than about the page's mood.
  const posted = [];
  page.on('request', (request) => {
    if (request.method() !== 'POST') return;
    if (new URL(request.url()).pathname !== '/preferences') return;
    posted.push(request.url());
  });

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const landed = page.url();
  const fields = await page.evaluate(() =>
    [...document.querySelectorAll('[data-pref-field]')].map((f) => f.dataset.prefField),
  );
  for (const needed of ['theme', 'textsize', 'font', 'ruby', 'shortcuts']) {
    if (!fields.includes(needed)) broken(`the page carries no ${needed} choice, so nothing below is being tested`);
  }
  const onArrival = await cookies(context);
  for (const name of ['yomihon_theme', 'yomihon_textsize', 'yomihon_font']) {
    if (onArrival[name]) broken(`${name} was already stored on arrival, so a stored value proves nothing`);
  }

  // ── Nothing is asked of the server ──────────────────────────────────────
  await pick(page, 'theme', 'dark');
  await page.waitForTimeout(200);
  if (posted.length !== 0) {
    fail('no-request-on-a-change', `picking an option sent ${posted.length} POST to /preferences (${posted.join(', ')}), want none`);
  }
  if (page.url() !== landed) {
    fail('no-request-on-a-change', `picking an option left the page for ${page.url()}, want to stay on ${landed}`);
  }

  // ── Two changes inside one frame ────────────────────────────────────────
  // Both clicks are dispatched before the page can paint, so neither has had a
  // chance to be answered, deferred, or coalesced away by the time the other
  // is made.
  await page.evaluate(() => {
    document.querySelector('[data-pref-field="ruby"] input[value="off"]').click();
    document.querySelector('[data-pref-field="shortcuts"] input[value="off"]').click();
  });
  await page.waitForTimeout(200);
  const rapid = await cookies(context);
  const afterRapid = await rootState(page);
  if (rapid.yomihon_ruby !== 'off' || rapid.yomihon_shortcuts !== 'off') {
    fail(
      'rapid-change-keeps-both',
      `two choices made in one frame stored ruby=${rapid.yomihon_ruby} shortcuts=${rapid.yomihon_shortcuts}, want both off`,
    );
  }
  if (afterRapid.ruby !== 'off' || afterRapid.shortcuts !== 'off') {
    fail(
      'rapid-change-keeps-both',
      `two choices made in one frame left the page at ruby=${afterRapid.ruby} shortcuts=${afterRapid.shortcuts}, want both off`,
    );
  }
  if (posted.length !== 0) {
    fail('rapid-change-keeps-both', `two choices made in one frame sent ${posted.length} POST to /preferences, want none`);
  }

  // ── One change reaches the page and the storage ─────────────────────────
  await pick(page, 'textsize', 'xl');
  await page.waitForTimeout(200);
  const sized = await cookies(context);
  const afterSize = await rootState(page);
  if (afterSize.textsize !== 'xl' || sized.yomihon_textsize !== 'xl') {
    fail(
      'applies-and-stores',
      `a size picked once left the page at ${afterSize.textsize || 'nothing'} and stored ${sized.yomihon_textsize || 'nothing'}, want xl in both`,
    );
  }
  if (afterSize.theme !== 'dark' || sized.yomihon_theme !== 'dark') {
    fail(
      'applies-and-stores',
      `the appearance picked earlier left the page at ${afterSize.theme || 'nothing'} and stored ${sized.yomihon_theme || 'nothing'}, want dark in both`,
    );
  }

  // ── A size and a face set in one visit ──────────────────────────────────
  await pick(page, 'font', 'sans');
  await page.waitForTimeout(200);
  const both = await cookies(context);
  const afterBoth = await rootState(page);
  if (afterBoth.font !== 'sans' || both.yomihon_font !== 'sans') {
    fail(
      'size-and-typeface-together',
      `the typeface left the page at ${afterBoth.font || 'nothing'} and stored ${both.yomihon_font || 'nothing'}, want sans in both`,
    );
  }
  if (afterBoth.textsize !== 'xl' || both.yomihon_textsize !== 'xl') {
    fail(
      'size-and-typeface-together',
      `picking a typeface put the size back to ${afterBoth.textsize}/${both.yomihon_textsize}, want the xl picked before it`,
    );
  }

  // ── A write this browser will not keep ──────────────────────────────────
  const refusing = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await refusing.addInitScript(REFUSE_COOKIES);
  const refused = await refusing.newPage();
  const refusedProof = MUTATE ? await MUTATIONS[MUTATE].apply(refused) : null;
  await refused.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (refusedProof) {
    const issue = refusedProof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  const before = await checkedValue(refused, 'theme');
  if (before !== 'system') {
    broken(`a browser storing nothing shows the appearance as ${before}, want the option that stores nothing`);
  }

  await pick(refused, 'theme', 'dark');
  await refused.waitForTimeout(200);
  const putBack = await checkedValue(refused, 'theme');
  const refusedRoot = await rootState(refused);
  const line = await statusOf(refused, 'theme');
  if (!line) broken('the appearance choice carries no line to say a refusal in');
  if (!line.served) broken('the refusal line carries no served sentence, so an empty one would pass for silence');
  if (line.live !== 'polite') broken(`the refusal line is aria-live=${line.live}, so it would be drawn and not announced`);
  if (putBack !== before) {
    fail('refusal-puts-the-choice-back', `a refused choice is left showing ${putBack}, want it back on ${before}`);
  }
  if (refusedRoot.theme !== '') {
    fail('refusal-puts-the-choice-back', `a refused choice left the page stamped theme=${refusedRoot.theme}, want the stamp back off`);
  }
  if (line.shown !== line.served) {
    fail('refusal-puts-the-choice-back', `a refused choice says ${JSON.stringify(line.shown)}, want the served ${JSON.stringify(line.served)}`);
  }

  // ── And the sentence is answered by a choice that is kept ───────────────
  await refused.evaluate(() => {
    window.__yomihonRefuse = false;
  });
  await pick(refused, 'theme', 'dark');
  await refused.waitForTimeout(200);
  const kept = await cookies(refusing);
  const keptLine = await statusOf(refused, 'theme');
  if (kept.yomihon_theme !== 'dark') {
    broken(`the stand-in never let go: theme stored ${kept.yomihon_theme || 'nothing'}, want dark`);
  }
  if (keptLine.shown !== '') {
    fail(
      'a-kept-choice-clears-the-sentence',
      `a choice that was kept leaves ${JSON.stringify(keptLine.shown)} standing under it, want the refusal sentence gone`,
    );
  }

  console.log(
    'PASS preference-immediate: choices apply and store as they are picked, two in one frame both land with nothing sent, and a refused write is put back, announced, and answered',
  );
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
