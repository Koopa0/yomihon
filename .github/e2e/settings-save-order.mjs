// Behavior lock for the two ways an immediate save can lie to a reader.
//
// The first is order. Every submission the reading choices page sends carries
// every field, so two of them in the air at once can be answered in either
// order, and the older answer would put a field back where the reader had just
// moved it from. Loopback answers so fast that two sends usually land in the
// order they left, which would make a plain test of this pass over a page that
// has no defence at all — so the first answer is held back here while the
// second is let straight through. Under that delay the two designs part: a
// page that sends both at once ends up holding the older answer's values, and
// one that waits builds its second submission after the first is answered and
// ends up holding both choices.
//
// The second is silence. A submission the server refuses stores nothing, and
// the controls are left showing a choice this browser is not set to. That is
// the one failure a reader has no way of noticing, so the controls go back to
// what is stored and the page says why.
//
// Env: YOMIHON_BASE, PAGE_PATH (the settings page), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/preferences?from=%2Fnotes%2FNotes%2Falpha.md';
const MUTATE = process.env.MUTATE || '';

// Found by the field the choices are made of rather than by the attribute the
// enhancement adds, so this probe can be pointed at a build with no
// enhancement and report the missing behaviour rather than a missing hook.
const CHOICES = 'form:has(input[name="theme"])';
const FORM = CHOICES + ' ';

// Long enough that the second submission is answered, its cookies stored, and
// the first one's answer arrives afterwards — which is the exact hazard.
const HOLD_FIRST_ANSWER_MS = 500;

const SITES = ['no-stale-overwrite', 'refused-save-reverts', 'refused-save-is-said'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN settings-save-order: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL settings-save-order: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN settings-save-order: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED settings-save-order: ${message}`);
};

const rewriteModule = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/preferences.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => (matches === 1 ? '' : `${label} needle matched ${matches} times, want exactly 1`);
};

const MUTATIONS = {
  'send-in-parallel': {
    target: 'no-stale-overwrite',
    on: 'order',
    apply: rewriteModule('    if (sending) {\n      sendAgain = true;\n      return;\n    }\n', '', 'one submission at a time'),
  },
  'ignore-refusal': {
    target: 'refused-save-reverts',
    on: 'refusal',
    apply: rewriteModule(
      "      stored = response.type === 'opaqueredirect' || response.ok;\n",
      '      stored = true;\n',
      'refusal reading',
    ),
  },
  'refuse-in-silence': {
    target: 'refused-save-is-said',
    on: 'refusal',
    apply: rewriteModule('    showSettingsFailure(true);\n', '', 'refusal notice'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`settings-save-order: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`settings-save-order: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`settings-save-order: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const isSettingsAddress = (url) => url.pathname === '/preferences';

const cookies = async (context) => {
  const jar = {};
  for (const cookie of await context.cookies()) jar[cookie.name] = cookie.value;
  return jar;
};

const describe = (jar) =>
  Object.entries(jar)
    .map(([name, value]) => `${name}=${value}`)
    .join(' ') || '(nothing)';

const until = async (read, settled, deadlineMs) => {
  const stop = Date.now() + deadlineMs;
  let last = await read();
  while (!settled(last) && Date.now() < stop) {
    await new Promise((resolve) => setTimeout(resolve, 50));
    last = await read();
  }
  return last;
};

const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  // Two changes inside one frame, with the first answer held back.
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: 'light' });
  const page = await context.newPage();
  if (MUTATE && MUTATIONS[MUTATE].on === 'order') proof = await MUTATIONS[MUTATE].apply(page);

  let sent = 0;
  let answered = 0;
  await page.route(isSettingsAddress, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    sent += 1;
    if (sent === 1) await pause(HOLD_FIRST_ANSWER_MS);
    return route.fallback();
  });
  page.on('response', (response) => {
    if (response.request().method() === 'POST' && new URL(response.url()).pathname === '/preferences') answered += 1;
  });

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    proof = null;
  }
  if (await page.locator(`${FORM}input[name="theme"][value="dark"]`).count() !== 1) {
    broken('the page offers no dark appearance to pick, so there is nothing here to race');
  }

  await page.evaluate(() => {
    const form = document.querySelector('form:has(input[name="theme"])');
    form.querySelector('input[name="theme"][value="dark"]').click();
    form.querySelector('input[name="textsize"][value="xl"]').click();
  });
  await until(async () => answered, (count) => count >= 2, 8000);
  // The last answer's cookies are stored as its headers arrive, which is when
  // the count above moves; the grace is for a third submission that should not
  // exist landing after it.
  await pause(400);
  const raced = await cookies(context);
  if (raced.yomihon_theme !== 'dark' || raced.yomihon_textsize !== 'xl') {
    fail('no-stale-overwrite', `two choices made within one frame, with the first answer held ${HOLD_FIRST_ANSWER_MS}ms, left this browser holding ${describe(raced)}, want yomihon_theme=dark and yomihon_textsize=xl`);
  }
  await context.close();

  // A refused submission. Nothing is stored, so nothing may be left looking
  // chosen, and the page has to say what happened.
  const refused = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: 'light' });
  const second = await refused.newPage();
  if (MUTATE && MUTATIONS[MUTATE].on === 'refusal') proof = await MUTATIONS[MUTATE].apply(second);
  await second.route(isSettingsAddress, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    return route.fulfill({ status: 422, contentType: 'text/plain; charset=utf-8', body: 'refused' });
  });
  await second.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    proof = null;
  }

  const standing = await second.evaluate(() => document.querySelector('form:has(input[name="theme"])').elements.theme.value);
  if (standing !== 'system') {
    broken(`the page opens with the appearance already at ${standing}; this check needs an answer the refused one can be told apart from`);
  }

  await second.click(`${FORM}input[name="theme"][value="dark"]`);
  const after = await until(
    () =>
      second.evaluate(() => {
        const form = document.querySelector('form:has(input[name="theme"])');
        const notice = document.querySelector('[data-preferences-failed]');
        return {
          chosen: form.elements.theme.value,
          stamped: document.documentElement.dataset.theme ?? '',
          pressed: document.querySelector('[data-theme-toggle]')?.getAttribute('aria-pressed') ?? null,
          said: notice ? !notice.hidden : null,
          words: notice ? notice.textContent.trim() : '',
        };
      }),
    (state) => state.chosen === 'system' && state.said === true,
    4000,
  );
  const jar = await cookies(refused);
  if (after.chosen !== 'system' || after.stamped !== '' || after.pressed !== 'false' || jar.yomihon_theme) {
    fail('refused-save-reverts', `a refused save left the page showing ${after.chosen} with the root at theme=${after.stamped || '(none)'}, the header control at aria-pressed=${after.pressed} and cookies ${describe(jar)}, want everything back on the answer this browser holds`);
  }
  if (after.said !== true || after.words === '') {
    fail('refused-save-is-said', `a refused save was reported to the reader as ${after.said === null ? 'nothing at all — the page has no place to say it' : `hidden (${after.words || 'no words'})`}, want a visible sentence saying the choice was not stored`);
  }

  // And the notice is not a stain: a save that goes through afterwards takes
  // it away, or every later visit would be told about a failure long past.
  await second.unroute(isSettingsAddress);
  await second.click(`${FORM}input[name="textsize"][value="l"]`);
  const cleared = await until(
    () => second.evaluate(() => document.querySelector('[data-preferences-failed]').hidden),
    (hidden) => hidden === true,
    4000,
  );
  if (cleared !== true) {
    fail('refused-save-is-said', 'the refusal notice was still on the page after a later choice was stored, so it no longer says anything about this browser');
  }
  await refused.close();

  console.log(`PASS settings-save-order: two choices one frame apart both survive a ${HOLD_FIRST_ANSWER_MS}ms-delayed first answer, and a refused save puts every control back and says so`);
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
