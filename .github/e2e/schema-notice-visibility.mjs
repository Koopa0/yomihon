// Behavior lock: unknown frontmatter field guidance is painted in the reading
// column at every width, and the transition controls still name that block as
// their accessible description. The notice used to live only in the right rail,
// which is hidden at ≤1280 and dropped when a note has no reading aids.
//
// Env: YOMIHON_BASE, PAGE_PATH (the schema-notice probe fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/schema-notice-probe.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['notice-painted-mid', 'notice-painted-narrow', 'notice-describedby'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN schema-notice-visibility: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL schema-notice-visibility: ${message}`);
};
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED schema-notice-visibility: ${message}`); };

const rewritePath = (path, needle, replacement, expected, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + path, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `${label} document was never requested`;
    if (matches !== expected * requests) return `${label} needle matched ${matches} times over ${requests} request(s), want ${expected} each`;
    return '';
  };
};

const hideVia = (path, selector, label) => rewritePath(
  path,
  '</head>',
  `<style>${selector}{display:none!important}</style></head>`,
  1,
  label,
);

const MUTATIONS = {
  'hide-notice': {
    target: 'notice-painted-mid',
    apply: hideVia(PAGE, '#schema-notices', 'schema-notice hide style'),
  },
  'clip-notice-out-of-sight': {
    target: 'notice-painted-narrow',
    apply: rewritePath(
      PAGE,
      '</head>',
      '<style>#schema-notices{position:absolute!important;width:1px!important;height:1px!important;overflow:hidden!important;clip-path:inset(50%)!important}</style></head>',
      1,
      'schema-notice clip style',
    ),
  },
  'drop-describedby': {
    target: 'notice-describedby',
    apply: rewritePath(PAGE, ' aria-describedby="schema-notices"', '', 1, 'schema-notice describedby'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`schema-notice-visibility: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`schema-notice-visibility: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`schema-notice-visibility: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const painted = (locator) => locator.evaluate((element) => {
  element.scrollIntoView({ block: 'center' });
  const rect = element.getBoundingClientRect();
  const style = getComputedStyle(element);
  const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = style.color;
  ctx.fillRect(0, 0, 1, 1);
  return {
    text: element.textContent.replace(/\s+/g, ' ').trim(),
    width: rect.width,
    height: rect.height,
    inViewport: rect.top >= 0 && rect.bottom <= innerHeight,
    display: style.display,
    visibility: style.visibility,
    opacity: style.opacity,
    inkAlpha: ctx.getImageData(0, 0, 1, 1).data[3],
    visible: element.checkVisibility({
      opacityProperty: true,
      visibilityProperty: true,
      contentVisibilityAuto: true,
    }),
    hit: hit === element || element.contains(hit),
  };
});

const onScreen = (seen) => seen.width > 1 && seen.height > 1 && seen.inViewport
  && seen.visible && seen.inkAlpha > 0 && seen.hit;

const assertNoticeVisible = async (page, site) => {
  const block = page.locator('#schema-notices');
  if (await block.count() !== 1) {
    fail(site, `the page carries ${await block.count()} #schema-notices blocks, want exactly 1`);
  }
  const seen = await painted(block);
  if (!onScreen(seen)) {
    fail(site, `the unknown-field notice is not on screen: ${JSON.stringify(seen)}`);
  }
  if (!seen.text.includes('mystery_key') || !seen.text.includes('不是 schema 認得的欄位')) {
    fail(site, `the notice does not name the unknown key in words: ${JSON.stringify(seen.text)}`);
  }
  const rail = page.locator('aside.y-rail-right #schema-notices');
  if (await rail.count() !== 0) {
    fail(site, 'the notice still renders inside the right rail');
  }
};

const assertDescribedBy = async (page) => {
  const submit = page.locator('.y-sealbar button[type="submit"][aria-describedby="schema-notices"]').first();
  if (await submit.count() !== 1) {
    fail('notice-describedby', `the sealbar carries ${await submit.count()} described submits, want exactly 1`);
  }
  const description = await submit.evaluate((button) => {
    const id = button.getAttribute('aria-describedby');
    const target = id ? document.getElementById(id) : null;
    return target?.textContent.replace(/\s+/g, ' ').trim() ?? '';
  });
  if (!description.includes('mystery_key') || !description.includes('不是 schema 認得的欄位')) {
    fail('notice-describedby', `the submit's described-by text is wrong: ${JSON.stringify(description)}`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  for (const [width, site] of [[1280, 'notice-painted-mid'], [390, 'notice-painted-narrow']]) {
    const page = await browser.newPage({ viewport: { width, height: 800 } });
    if (MUTATE && site === MUTATIONS[MUTATE].target) {
      proof = await MUTATIONS[MUTATE].apply(page);
    }
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    await assertNoticeVisible(page, site);
    if (width === 1280) await assertDescribedBy(page);
    await page.close();
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log('PASS schema-notice-visibility: the unknown-field notice is painted in the reading column and still describes the sealbar submit');
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED schema-notice-visibility: ${MUTATE}: ${issue}`);
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
      await browser.close();
      process.exit(2);
    }
  }
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
