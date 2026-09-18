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

// The bar's own opening tag and nothing before it: what closes above it is
// whatever the note happened to earn — a prose column, or the folder steps a
// second note in the same folder gives it — and a needle that spelled one of
// those stops matching the day the note gains the other.
const SEALBAR_OPEN = '<section class="y-sealbar"';
const SEALBAR_CLOSE = '</section></article></main>';

const MUTATIONS = {
  'restore-fixed-bar': {
    target: 'not-fixed-position',
    apply: rewriteStylesheet(
      '.y-sealbar{border-top:1px solid var(--border);background:var(--panel);flex-wrap:wrap;align-items:center;gap:12px;margin-top:26px;padding:10px 16px;display:flex}',
      '.y-sealbar{position:fixed;left:0;right:0;bottom:0;z-index:36;border-top:1px solid var(--border);background:var(--panel);flex-wrap:wrap;align-items:center;gap:12px;margin-top:26px;padding:10px 16px;display:flex}',
      'fixed sealbar',
    ),
  },
  // The bar is met at the end of the reading; it does not ride along with it.
  // Sticky is how that goes wrong while everything still looks right, because
  // the bar returns to its own place once the reader reaches the end -- so the
  // measurement taken before the scroll is the only one that tells them apart.
  'stick-the-bar-to-the-fold': {
    target: 'below-fold-while-reading',
    apply: rewriteStylesheet(
      '.y-sealbar{border-top:1px solid var(--border);background:var(--panel);flex-wrap:wrap;align-items:center;gap:12px;margin-top:26px;padding:10px 16px;display:flex}',
      '.y-sealbar{position:sticky;bottom:0;z-index:36;border-top:1px solid var(--border);background:var(--panel);flex-wrap:wrap;align-items:center;gap:12px;margin-top:26px;padding:10px 16px;display:flex}',
      'sticky sealbar',
    ),
  },
  // The bar sits in the article's foot, a band deep enough to read as the end
  // of the reading rather than as a strip tacked under the last line. Widening
  // that foot strands the bar in the middle of empty paper. It aims here and
  // not at the reading position because the padding is entirely below the bar:
  // it lengthens the document without moving the bar on the first screen.
  'widen-the-article-foot': {
    target: 'follows-article-end',
    apply: rewriteStylesheet(
      'padding:44px var(--article-gutter)120px',
      'padding:44px var(--article-gutter)300px',
      'widened article foot',
    ),
  },
  'move-bar-outside-article': {
    target: 'inside-article',
    apply: rewriteDocument([
      [SEALBAR_OPEN, '</article><section class="y-sealbar"'],
      [SEALBAR_CLOSE, '</section></main>'],
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
  await page.waitForSelector(SEALBAR, { state: 'attached' });

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
  const tailPadding = end.articleBottom - end.sealBottom;
  if (tailPadding < 80 || tailPadding > 160) {
    fail('follows-article-end', `${label}: seal bar sits ${tailPadding}px above the article bottom, want the article foot padding band`);
  }
  if (end.sealBottom > end.viewportBottom + 1) {
    fail('follows-article-end', `${label}: seal bar bottom ${end.sealBottom} is below the viewport ${end.viewportBottom} at scroll end`);
  }
};

const runLocks = async () => {
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
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
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
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

// A mutation aimed at a site that does not exist never runs, and an assertion
// no mutation aims at is a lock nothing has ever watched fail. Both are silent
// while the suite stays green, so they are refused here instead.
for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`sealbar-flow-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`sealbar-flow-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}

if (MUTATE) {
  await runMutation(MUTATE);
} else {
  await runLocks();
}
