// Behavior lock for the status seal bar at the end of the article. While
// reading, write chrome must not own the viewport bottom: the bar stays in
// document order after the article content, not fixed to the screen edge.
//
// Env: YOMIHON_BASE, PAGE_PATH (the writable L01 fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const ARTICLE = '.y-article';
const SEALBAR = '.y-sealbar';
const STEPS = '.y-steps';

const SITES = [
  'not-fixed-position',
  'inside-article',
  'below-fold-while-reading',
  'follows-article-end',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN sealbar-flow-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL sealbar-flow-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN sealbar-flow-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED sealbar-flow-contract: ${message}`); };

const rewriteStylesheet = (needle, replacement, label) => async (page) => {
  let matches = 0;
  await page.route('**/static/app.css', async (route) => {
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

const rewriteDocument = (replacements, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    let body = await response.text();
    for (const [needle, replacement] of replacements) {
      const count = body.split(needle).length - 1;
      matches += count;
      body = body.replace(needle, replacement);
    }
    await route.fulfill({ response, body });
  });
  return () => {
    if (requests !== 1) return `${label} document was requested ${requests} times, want exactly 1`;
    if (matches !== replacements.length) return `${label} replacement matched ${matches} times, want exactly ${replacements.length}`;
    return '';
  };
};

const MUTATIONS = {
  'restore-fixed-bar': {
    target: 'not-fixed-position',
    apply: rewriteStylesheet(
      '.y-sealbar { display: flex;',
      '.y-sealbar { display: flex; position: fixed; left: 0; right: 0; bottom: 0; z-index: 36;',
      'fixed sealbar',
    ),
  },
  'move-bar-outside-article': {
    target: 'inside-article',
    apply: rewriteDocument([
      ['</nav><section class="y-sealbar"', '</nav></article><section class="y-sealbar"'],
      ['</section></article></main>', '</section></main>'],
    ], 'sealbar outside article'),
  },
};

const measureSealbar = async (page) => page.evaluate(({ article, seal, steps }) => {
  const articleEl = document.querySelector(article);
  const sealEl = document.querySelector(seal);
  const stepsEl = document.querySelector(steps);
  if (!articleEl || !sealEl) {
    return { missing: true };
  }
  const articleRect = articleEl.getBoundingClientRect();
  const sealRect = sealEl.getBoundingClientRect();
  const stepsRect = stepsEl ? stepsEl.getBoundingClientRect() : null;
  const style = getComputedStyle(sealEl);
  return {
    missing: false,
    position: style.position,
    display: style.display,
    insideArticle: articleEl.contains(sealEl),
    sealBottom: sealRect.bottom,
    sealTop: sealRect.top,
    articleBottom: articleRect.bottom,
    stepsBottom: stepsRect ? stepsRect.bottom : null,
    viewportBottom: window.innerHeight,
    scrollY: window.scrollY,
  };
}, { article: ARTICLE, seal: SEALBAR, steps: STEPS });

const assertFlowAtWidth = async (page, width, height, label, { hiddenOK = false } = {}) => {
  await page.setViewportSize({ width, height });
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector(SEALBAR);

  const mid = await measureSealbar(page);
  if (mid.missing) broken(`${label}: the page has no article or seal bar to measure`);
  if (mid.position === 'fixed') fail('not-fixed-position', `${label}: seal bar position is fixed at scrollY ${mid.scrollY}`);
  if (!mid.insideArticle) fail('inside-article', `${label}: seal bar is not inside the article`);
  if (mid.display === 'none') {
    if (!hiddenOK) broken(`${label}: seal bar is display:none at ${width}px, so this width cannot be exercised`);
    return;
  }

  if (mid.sealBottom <= mid.viewportBottom + 1) {
    fail('below-fold-while-reading', `${label}: seal bar bottom ${mid.sealBottom} sits on the viewport bottom ${mid.viewportBottom} at scrollY 0`);
  }

  await page.evaluate(() => {
    window.scrollTo(0, document.documentElement.scrollHeight);
  });
  const end = await measureSealbar(page);
  if (end.stepsBottom !== null && end.sealTop + 1 < end.stepsBottom) {
    fail('follows-article-end', `${label}: seal bar top ${end.sealTop} sits above step nav bottom ${end.stepsBottom} at scroll end`);
  }
  if (Math.abs(end.sealBottom - end.articleBottom) > 2) {
    fail('follows-article-end', `${label}: seal bar bottom ${end.sealBottom} is not at article bottom ${end.articleBottom} at scroll end`);
  }
  if (end.sealBottom > end.viewportBottom + 1) {
    fail('follows-article-end', `${label}: seal bar bottom ${end.sealBottom} is below the viewport ${end.viewportBottom} at scroll end`);
  }
};

const runLocks = async () => {
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    await assertFlowAtWidth(page, 1440, 900, 'wide 1440×900', { hiddenOK: true });
    await assertFlowAtWidth(page, 1280, 900, 'mid 1280×900');
    await assertFlowAtWidth(page, 390, 844, 'narrow 390×844');
    console.log('PASS sealbar-flow-contract: the seal bar is in flow at the article end at wide, mid and narrow widths');
  } finally {
    await browser.close();
  }
};

const runMutation = async (mode) => {
  const mutation = MUTATIONS[mode];
  if (!mutation) broken(`unknown mutation mode ${mode}`);
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    const prove = await mutation.apply(page);
    try {
      await assertFlowAtWidth(page, 1280, 900, `mutated ${mode}`);
      console.log(`MUTATE-RESULT: missed ${mode}`);
      process.exit(0);
    } catch (err) {
      if (!(err instanceof LockFired) || err.site !== mutation.target) throw err;
      console.log(`MUTATE-RESULT: caught ${mode}`);
      process.exit(1);
    } finally {
      const issue = prove();
      if (issue) notApplied(issue);
    }
  } finally {
    await browser.close();
  }
};

if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}

if (MUTATE) {
  await runMutation(MUTATE);
} else {
  await runLocks();
}
