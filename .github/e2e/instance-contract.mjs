// Behavior lock: files under the contract's non-instance directories remain
// directly readable and browsable, but never leak into instance projections or
// expose a status write face.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const FOLDERS = '/folders';
const MAP_NOTE = '/notes/System/templates/Template%20map.md';
const MAP_RAW = '/raw/System/templates/Template%20map.md';
const LESSON_NOTE = '/notes/System/templates/Template%20lesson.md';
const LESSON_RAW = '/raw/System/templates/Template%20lesson.md';
const MAP_TITLE = 'TEMPLATE MAP LEAK SENTINEL';
const MAP_BODY = 'TEMPLATE MAP BODY SENTINEL';
const LESSON_TITLE = 'TEMPLATE LESSON WRITE SENTINEL';
const LESSON_BODY = 'TEMPLATE LESSON BODY SENTINEL';
const QUIET_REASON = '不屬於生命週期治理範圍';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'map-projection-omitted',
  'recent-omitted',
  'folder-links-present',
  'map-note-readable',
  'map-raw-readable',
  'lesson-note-readable',
  'lesson-raw-readable',
  'noninstance-quiet-marker',
  'noninstance-no-status-form',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN instance-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL instance-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN instance-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED instance-contract: ${message}`); };

const rewritePath = (path, transform) => async (page) => {
  let applied = false;
  await page.route(BASE + path, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = transform(original);
    if (body !== original) applied = true;
    return route.fulfill({ response, body });
  });
  return () => applied;
};

const rewriteDocument = (transform) => rewritePath(PAGE, transform);

const MUTATIONS = {
  'inject-template-map-projection': {
    target: 'map-projection-omitted',
    apply: rewriteDocument((body) => body.replaceAll(
      '地圖</summary>',
      '地圖</summary><details data-map-tree="System/templates/Template map.md"></details>',
    )),
  },
  'inject-template-recent': {
    target: 'recent-omitted',
    apply: rewritePath(FOLDERS, (body) => body.replaceAll(
      '<div class="y-homerecent">',
      `<div class="y-homerecent"><span>${MAP_TITLE}</span>`,
    )),
  },
  'drop-template-folder-link': {
    target: 'folder-links-present',
    apply: rewriteDocument((body) => body.replaceAll(MAP_NOTE, '/notes/System/templates/Missing%20map.md')),
  },
  'redact-template-map-note': {
    target: 'map-note-readable',
    apply: rewritePath(MAP_NOTE, (body) => body.replaceAll(MAP_BODY, 'REDACTED MAP NOTE')),
  },
  'redact-template-map-raw': {
    target: 'map-raw-readable',
    apply: rewritePath(MAP_RAW, (body) => body.replaceAll(MAP_BODY, 'REDACTED MAP RAW')),
  },
  'redact-template-lesson-note': {
    target: 'lesson-note-readable',
    apply: rewritePath(LESSON_NOTE, (body) => body.replaceAll(LESSON_BODY, 'REDACTED LESSON NOTE')),
  },
  'redact-template-lesson-raw': {
    target: 'lesson-raw-readable',
    apply: rewritePath(LESSON_RAW, (body) => body.replaceAll(LESSON_BODY, 'REDACTED LESSON RAW')),
  },
  'drop-noninstance-marker': {
    target: 'noninstance-quiet-marker',
    apply: rewritePath(LESSON_NOTE, (body) => body.replaceAll(QUIET_REASON, 'governance marker removed')),
  },
  'inject-noninstance-status-form': {
    target: 'noninstance-no-status-form',
    apply: rewritePath(LESSON_NOTE, (body) => body.replaceAll(QUIET_REASON, `${QUIET_REASON}<form class="y-statusform" action="/status"></form>`)),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`instance-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`instance-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`instance-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  let response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response) broken(`navigation to ${PAGE} returned no response`);
  if (response.status() !== 200) broken(`${PAGE} status ${response.status()}, want 200`);
  const projections = page.locator('[data-sidebar-group="paths"], [data-sidebar-group="maps"]');
  const projectionText = (await projections.allTextContents()).join('\n');
  if (await projections.locator('[data-map-tree="System/templates/Template map.md"]').count() !== 0 || projectionText.includes(MAP_TITLE)) {
    fail('map-projection-omitted', 'the template map leaked into Paths or Maps');
  }
  const sidebar = page.locator('aside.y-rail-left');
  if (await sidebar.locator(`a[href="${MAP_NOTE}"]`).count() !== 1 || await sidebar.locator(`a[href="${LESSON_NOTE}"]`).count() !== 1) {
    fail('folder-links-present', 'the template folder does not retain links to both non-instance notes');
  }

  // What changed last is the folder mode's answer, not the desk's: it
  // describes how the files are kept rather than what there is to read.
  response = await page.goto(BASE + FOLDERS, { waitUntil: 'domcontentloaded' });
  if (!response) broken('navigation to the folder index returned no response');
  const recent = page.locator('[data-home-block="recent"]');
  if (await recent.count() !== 1) broken('the folder index carries no recent block to read');
  const recentText = await recent.textContent();
  if (recentText.includes(MAP_TITLE) || recentText.includes(LESSON_TITLE)) {
    fail('recent-omitted', 'a non-instance template leaked into the recent list');
  }

  response = await page.goto(BASE + MAP_NOTE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200 || !(await page.locator('main').textContent()).includes(MAP_BODY)) {
    fail('map-note-readable', 'the template map note is not directly readable with its sentinel');
  }
  response = await page.goto(BASE + MAP_RAW, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200 || !(await response.text()).includes(MAP_BODY)) {
    fail('map-raw-readable', 'the template map raw route is not readable with its sentinel');
  }

  response = await page.goto(BASE + LESSON_NOTE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200 || !(await page.locator('main').textContent()).includes(LESSON_BODY)) {
    fail('lesson-note-readable', 'the template lesson note is not directly readable with its sentinel');
  }
  const noninstanceFaces = page.locator('[data-status-state="non-instance"]');
  if (await noninstanceFaces.count() !== 2 || await noninstanceFaces.getByText(QUIET_REASON, { exact: true }).count() !== 2 || await page.locator('.ui-status').count() !== 0) {
    fail('noninstance-quiet-marker', 'the template lesson lacks the quiet non-instance marker or exposes a status decoration');
  }
  if (await page.locator('form.y-statusform, form[action="/status"]').count() !== 0) {
    fail('noninstance-no-status-form', 'the template lesson exposes a status form');
  }
  response = await page.goto(BASE + LESSON_RAW, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200 || !(await response.text()).includes(LESSON_BODY)) {
    fail('lesson-raw-readable', 'the template lesson raw route is not readable with its sentinel');
  }

  if (proof && !proof()) notApplied(`the ${MUTATE} mutation changed nothing in the response it targeted`);
  console.log('PASS instance-contract: template artifacts stay browsable and directly readable while absent from instance projections and write faces');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE && (!proof || !proof())) {
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
