// Behavior lock for the first-focus skip link. Keyboard users can reveal the
// link before traversing the shared sidebar and move focus to the page's main
// landmark without JavaScript.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['first-focus-visible', 'main-receives-focus'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN skip-link-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL skip-link-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN skip-link-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED skip-link-contract: ${message}`); };

const rewriteDocument = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({
      response,
      body: original.replace(needle, replacement),
    });
  });
  return () => {
    if (requests !== 1) return `${label} document was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

// Match the declared anchor and its attributes, never the render blocker
// that also names #_y-main. The tag scan consumes quoted attribute values
// as a unit, so text inside another attribute cannot masquerade as href.
const skipTarget = (document) => {
  const sites = [];
  for (const tag of document.matchAll(/<a\b(?:[^"'<>]|"[^"]*"|'[^']*')*>/gi)) {
    const attributes = [...tag[0].matchAll(/\s+([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g)];
    const classes = attributes.filter((attribute) => attribute[1].toLowerCase() === 'class');
    const targets = attributes.filter((attribute) => attribute[1].toLowerCase() === 'href');
    if (classes.length !== 1 || targets.length !== 1) continue;
    const value = (attribute) => attribute[2] ?? attribute[3] ?? attribute[4] ?? '';
    if (!value(classes[0]).split(/\s+/).includes('y-skiplink') || value(targets[0]) !== '#_y-main') continue;
    const target = targets[0];
    const changed = tag[0].slice(0, target.index)
      + target[0].replace(/=[\s\S]*$/, '="#missing-content"')
      + tag[0].slice(target.index + target[0].length);
    sites.push({ index: tag.index, original: tag[0], changed });
  }
  if (sites.length !== 1) return { body: document, count: sites.length };
  const site = sites[0];
  return {
    body: document.slice(0, site.index) + site.changed + document.slice(site.index + site.original.length),
    count: sites.length,
  };
};

const rewriteSkipTarget = async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const changed = skipTarget(original);
    matches += changed.count;
    await route.fulfill({ response, body: changed.body });
  });
  return () => {
    if (requests !== 1) return `skip-link target document was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `skip-link target matched ${matches} anchors, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'hide-focused-link': {
    target: 'first-focus-visible',
    apply: rewriteDocument(
      '</head>',
      '<style>.y-skiplink:focus{opacity:0!important;pointer-events:none!important}</style></head>',
      'skip-link focus style',
    ),
  },
  'break-main-target': {
    target: 'main-receives-focus',
    apply: rewriteSkipTarget,
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`skip-link-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`skip-link-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`skip-link-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const skip = page.locator('body > a.y-skiplink');
  const main = page.locator('main#_y-main[tabindex="-1"]');
  if (await skip.count() !== 1) broken(`the page has ${await skip.count()} body-first skip links, want 1`);
  if (await main.count() !== 1) broken(`the page has ${await main.count()} focusable main targets, want 1`);
  const firstElement = await page.locator('body a, body button, body input, body select, body textarea, body [tabindex]:not([tabindex="-1"])').first().getAttribute('class');
  if (firstElement !== 'y-skiplink') broken(`the first sequential focus target has class ${JSON.stringify(firstElement)}, want y-skiplink`);

  await page.keyboard.press('Tab');
  const visible = await skip.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
    const style = getComputedStyle(element);
    return {
      active: document.activeElement === element,
      focusVisible: element.matches(':focus-visible'),
      outline: `${style.outlineStyle}/${style.outlineWidth}`,
      top: rect.top,
      bottom: rect.bottom,
      width: rect.width,
      height: rect.height,
      viewportHeight: innerHeight,
      hit: hit === element || element.contains(hit),
    };
  });
  if (!visible.active || !visible.focusVisible || visible.outline.startsWith('none/') || visible.top < 0 || visible.bottom > visible.viewportHeight || visible.width <= 0 || visible.height <= 0 || !visible.hit) {
    fail('first-focus-visible', `the first Tab did not reveal one painted, focused skip link: ${JSON.stringify(visible)}`);
  }

  await page.keyboard.press('Enter');
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const activation = await main.evaluate((element) => ({
    active: document.activeElement === element,
    hash: location.hash,
  }));
  if (!activation.active || activation.hash !== '#_y-main') {
    fail('main-receives-focus', `activating the skip link produced ${JSON.stringify(activation)}, want focused #_y-main`);
  }

  console.log('PASS skip-link-contract: the first Tab reveals the skip link and activation focuses main content');
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
