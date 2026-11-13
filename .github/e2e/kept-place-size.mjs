// Behavior lock for the desk of a reader who kept a place: the place is the
// largest text on it. The name of the place is set at the page-title size and
// no heading, row or figure on the desk is set as large, measured as the
// browser computed it over every piece of visible text rather than read from
// the stylesheet's source.
//
// It keeps a place of its own on the disposable fixture, so it is one of the
// probes that leave one behind and has to stand among the last (see probes.sh).
// The desk of a reader who kept none is measured by largest-text.mjs, which has
// to run before any place is kept.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every self-test mode.
import { arrived } from './support/arrival.mjs';
import { readSizes } from './support/text-sizes.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const NOTE = 'Notes/alpha.md';

const PLACE_SITE = 'the-kept-place-is-the-largest-text-on-the-desk';
const SITES = [PLACE_SITE];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL kept-place-size: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN kept-place-size: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED kept-place-size: ${message}`); };

// Appends a rule to the product's own stylesheet, past the layer it declares,
// so the injected rule outranks what it stands in for without an importance
// flag: the shape a regression written into the stylesheet itself would have.
const appendStyle = (css) => (page) => {
  let seen = 0;
  return page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${css}\n` });
  }).then(() => () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page'));
};

const MUTATIONS = {
  // The place set at body size: the desk opens on a line like any other.
  'shrink-the-kept-title': { target: PLACE_SITE, apply: appendStyle('.y-continue__title{font-size:1rem}') },
  // A block's name set above the place: the place is outshouted.
  'enlarge-a-block-name': { target: PLACE_SITE, apply: appendStyle('.y-homeblock__head h2{font-size:4rem}') },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`kept-place-size: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`kept-place-size: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`kept-place-size: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const kept = await context.request.post(`${BASE}/marks`, {
    form: { path: NOTE, anchor: '', offset: '0', identity: '0'.repeat(64) },
  });
  if (kept.status() !== 204) broken(`keeping a place in ${NOTE} answered ${kept.status()}, want 204`);

  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  const response = await page.goto(`${BASE}/`, { waitUntil: 'load' });
  if (response?.status() !== 200) broken(`/ answered ${response?.status()}, want 200`);
  await arrived(page);
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  if (await page.locator('[data-home-continue]').count() !== 1) broken('the desk draws no kept place after one was kept');

  const sizes = await readSizes(page, '.y-continue__title');
  const place = sizes.filter((text) => text.inside);
  if (place.length === 0) fail(PLACE_SITE, 'the kept place has no name drawn at all');
  const size = Math.max(...place.map((text) => text.size));
  if (size < 48) fail(PLACE_SITE, `the kept place is set at ${size}px, want the page-title size of at least 48px`);
  const overtaking = sizes.filter((text) => !text.inside && text.size >= size).sort((a, b) => b.size - a.size);
  if (overtaking.length > 0) {
    fail(PLACE_SITE, `${overtaking[0].what} (${JSON.stringify(overtaking[0].words)}) is set at ${overtaking[0].size}px, not smaller than the ${size}px kept place`);
  }

  await context.close();
  console.log(`PASS kept-place-size: the kept place, ${size}px, is the largest text on the desk`);
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
