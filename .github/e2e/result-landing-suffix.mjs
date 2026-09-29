// Behavior lock: clicking a search result written in a script that parts no
// words with spaces puts the match on the screen, whichever way the match sits
// in its words. The note is Japanese, long enough that every match starts
// thousands of pixels down, and each block in it is one way a browser can
// fail to find the directive the row carries.
//
// A text directive is a request to a browser, and the browser asks two things
// of the term it is given. Left to itself it must begin and end at a word
// boundary, and in Han, hiragana and katakana that boundary is a dictionary
// one this repository has no model of. Words named before the term release
// the beginning; words named after it release the end, and a run itself
// beginning or ending at white space, punctuation or the end of its block is a
// boundary every dictionary agrees with. So the row names both, and the
// directive is found wherever the match sits inside a word.
//
// The assertion is geometry, not the href: a plausible address and any nonzero
// scroll both pass while the match stays below the screen. Each case is
// measured at a narrow and a wide width, and the probe refuses to run unless
// the match starts off-screen, because an assertion that it is visible proves
// nothing about a page where it was visible all along.
//
// The cases:
//   control    a word a dictionary knows, reached after a run with no boundary
//   report     れ、, the match that opens inside 晴れ, after a long run of kana
//              and kanji with no boundary in it. The fixture's other 晴れ、 is
//              inside a ruby block, which the page does not reproduce as
//              written and which keeps its bare term, so this sentence is the
//              plain-text stand-in for it
//   suffix     a match that stops inside 紫陽花, so only the run after it lets
//              the end of the term go
//   alnum      a Latin word reached mid-word, which does not depend on either
//   crossing   a phrase from one paragraph into the next, ending inside 錠前
//   hard break a whole word with a long unbroken run beside it across a hard
//              line break, in both forms Markdown writes one (a trailing
//              backslash and two trailing spaces), once for the run after the
//              match and once for the run before it: the page draws the break
//              as a break, so a run reaching over it is text the page does not
//              carry in one piece
//   ruby       a match beside a ruby annotation, with furigana on and off: the
//              annotation is in the rendered text when it is on, so a run
//              spanning it is text the page does not carry in one piece
//
// Env: YOMIHON_BASE, PAGE_PATH (the report's own search), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const REPORT_QUERY = 'れ、';
const PAGE = process.env.PAGE_PATH || `/search?q=${encodeURIComponent(REPORT_QUERY)}`;
const MUTATE = process.env.MUTATE || '';
const NARROW = 390;
const WIDE = 1600;
const NOTE = /Suffix%20landing/;

const enc = encodeURIComponent;

// What each case searches for and the copy of the words it must bring on
// screen. Every term is one the article holds once, except the control's,
// which its own note repeats after the copy that matters.
const CASES = [
  { site: 'control-case-in-view', query: '晴れ', term: '晴れ', copies: 2 },
  { site: 'report-case-in-view', query: REPORT_QUERY, term: 'れ、', copies: 1 },
  { site: 'suffix-case-in-view', query: '紫陽', term: '紫陽', copies: 1 },
  { site: 'alnum-case-in-view', query: 'urmalin', term: 'tourmaline', copies: 1 },
  { site: 'crossing-case-in-view', query: '"銀杏並木の奥にある小さな倉庫の錠"', term: '銀杏', copies: 1 },
  { site: 'hardbreak-case-in-view', query: '布団', term: '布団', copies: 1 },
  { site: 'hardbreak-case-in-view', query: '雪柳', term: '雪柳', copies: 1 },
  { site: 'hardbreak-case-in-view', query: '囲炉裏', term: '囲炉裏', copies: 1 },
  { site: 'hardbreak-case-in-view', query: '山桜', term: '山桜', copies: 1 },
  { site: 'ruby-case-in-view', query: '白い花', term: '白い花', copies: 1, furigana: ['on', 'off'] },
];
const SITES = [...new Set(CASES.map((c) => c.site))];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN result-landing-suffix: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL result-landing-suffix: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN result-landing-suffix: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED result-landing-suffix: ${message}`); };

const searchPath = (query) => `/search?q=${enc(query)}`;

// rewriteResults proves it applied by counting, so a needle that rots against
// a rewritten directive turns this run red rather than quietly mutating
// nothing. The rewrite is on the results page, which is where the row's
// address is written.
const rewriteResults = (query, needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route((url) => url.pathname === '/search' && url.searchParams.get('q') === query, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `the results page for ${JSON.stringify(query)} was never requested`;
    if (matches !== requests) return `needle matched ${matches} times over ${requests} request(s), want 1 each`;
    return '';
  };
};

// The directive each case carries, spelled out so a mutation can take one
// piece of it away. The pieces are encoded the way the server encodes them:
// every character outside the unreserved set, "-" included.
const RUN_BEFORE_REPORT = '夜が明けるとすぐに窓辺の椅子に腰をおろして空の色を長いあいだ眺めていた老人はその日もよく晴';
const RUN_BEFORE_ALNUM = 'ledger of to';
const RUN_AFTER_SUFFIX = '花の株で';
const RUN_AFTER_CROSSING = '前に合うと';
const LINE_ONE_BACKSLASH = '冬の朝は窓の外がいつまでも暗くて誰もが布団の中でじっと身をすくめたまま夜が明けるのをただ静かに待ち続けていた';
const LINE_TWO_BACKSLASH = '白木蓮の花が静かに咲きはじめる頃になってようやく人々は外へ出て';
const LINE_TWO_SPACES = '黒松の枝が静かに揺れはじめる頃になってようやく人々は庭へ出て山桜の花を眺めた。';
const RUN_AFTER_RUBY = 'が咲く木蓮の枝のそばを通って';

const MUTATIONS = {
  // The regression the whole change exists for, at the case that needs it: a
  // match stopping inside a word is asked for as a term that must end at a
  // dictionary boundary, and the note opens at the top.
  'drop-the-suffix': {
    target: 'suffix-case-in-view',
    query: '紫陽',
    needle: `,-${enc(RUN_AFTER_SUFFIX)}`,
    replacement: '',
  },
  // The same for the far end of a phrase across two paragraphs.
  'drop-the-crossing-suffix': {
    target: 'crossing-case-in-view',
    query: '"銀杏並木の奥にある小さな倉庫の錠"',
    needle: `,-${enc(RUN_AFTER_CROSSING)}`,
    replacement: '',
  },
  // A run before the term too long for any boundary the budget allows goes
  // back to the start of its block. Without it the term opens inside 晴れ.
  'drop-the-prefix-completion': {
    target: 'report-case-in-view',
    query: REPORT_QUERY,
    needle: `text=${enc(RUN_BEFORE_REPORT)}-,`,
    replacement: 'text=',
  },
  // Latin words are found without any of this, and must stay found: the
  // directive that used to work is the one that still does.
  'drop-the-alnum-prefix': {
    target: 'alnum-case-in-view',
    query: 'urmalin',
    needle: `text=${enc(RUN_BEFORE_ALNUM)}-,`,
    replacement: 'text=',
  },
  // A block the page does not reproduce as written names no run beside the
  // term. Naming one anyway spans a reading the page draws in among the
  // characters it is spoken over, which furigana turns from hidden to shown.
  'ungate-the-ruby-suffix': {
    target: 'ruby-case-in-view',
    query: '白い花',
    needle: 'text=%E7%99%BD%E3%81%84%E8%8A%B1"',
    replacement: `text=%E7%99%BD%E3%81%84%E8%8A%B1,-${enc(RUN_AFTER_RUBY)}"`,
  },
  // The run before a match reaching back over a hard break, as it did when a
  // run with no boundary in it went back to the start of the block.
  'span-the-break-before': {
    target: 'hardbreak-case-in-view',
    query: '雪柳',
    needle: `text=${enc(LINE_TWO_BACKSLASH)}-,`,
    replacement: `text=${enc(`${LINE_ONE_BACKSLASH} ${LINE_TWO_BACKSLASH}`)}-,`,
  },
  // The run after a match reaching over one, as it did when a run with no
  // boundary in it went on to the end of the block.
  'span-the-break-after': {
    target: 'hardbreak-case-in-view',
    query: '囲炉裏',
    needle: `,-${enc('の端でぼんやり足を伸ばしたまま日が暮れるのをただ静かに眺め続けていた')}`,
    replacement: `,-${enc(`の端でぼんやり足を伸ばしたまま日が暮れるのをただ静かに眺め続けていた ${LINE_TWO_SPACES}`)}`,
  },
  // With no directive at all the note opens at the top, which is what every
  // case above measures the absence of. It is aimed at the first case so that
  // a mutation which broke only a later one could not tell a working probe
  // from an assertion that never ran.
  'strip-the-control-directive': {
    target: 'control-case-in-view',
    query: '晴れ',
    needle: 'Suffix%20landing.md#:~:text=',
    replacement: 'Suffix%20landing.md#',
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`result-landing-suffix: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`result-landing-suffix: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`result-landing-suffix: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// wordCopies measures a range around each copy of the words inside the
// article, so the answer is where the reader's evidence sits rather than where
// its paragraph does. The position in the document is reported beside the one
// on screen: a match that started inside the first screen proves nothing.
const wordCopies = (page, word) => page.evaluate((w) => {
  const root = document.querySelector('main article');
  if (!root) return null;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const found = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    for (let i = node.textContent.indexOf(w); i >= 0; i = node.textContent.indexOf(w, i + 1)) {
      const range = document.createRange();
      range.setStart(node, i);
      range.setEnd(node, i + w.length);
      const box = range.getBoundingClientRect();
      found.push({
        top: Math.round(box.top),
        docTop: Math.round(box.top + scrollY),
        inView: box.top >= 0 && box.bottom <= innerHeight && box.height > 0,
      });
    }
  }
  return found;
}, word);

// land drives the flow a reader drives: open the results, click the row, and
// wait for the browser to finish placing the page. Settling is watched rather
// than assumed, because a directive that is never honoured leaves the scroll
// at rest immediately and would otherwise be measured before a working one
// had moved.
const land = async (browser, testCase, width, furigana, apply) => {
  const context = await browser.newContext({ viewport: { width, height: 900 } });
  if (furigana) await context.addCookies([{ name: 'yomihon_ruby', value: furigana, url: BASE }]);
  const page = await context.newPage();
  const proof = apply ? await apply(page) : null;
  await page.goto(BASE + (testCase.query === REPORT_QUERY ? PAGE : searchPath(testCase.query)), { waitUntil: 'domcontentloaded' });

  const link = page.locator('a.y-result[href*="Suffix%20landing"]').first();
  if (await link.count() !== 1) broken(`the results for ${JSON.stringify(testCase.query)} do not offer the note this probe was written around`);
  const href = await link.getAttribute('href');

  await link.click();
  await page.waitForURL(NOTE);
  await page.waitForLoadState('load');
  let scroll = -1;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(150);
    const now = await page.evaluate(() => Math.round(scrollY));
    if (i > 3 && now === scroll) break;
    scroll = now;
  }

  const copies = await wordCopies(page, testCase.term);
  if (!copies) broken('the note page rendered no article to measure');
  if (copies.length !== testCase.copies) {
    broken(`the article holds ${copies.length} copies of ${JSON.stringify(testCase.term)}, want ${testCase.copies}`);
  }
  // The copy that matters is the first one in the document, which is also
  // the first the browser walks past.
  const match = copies[0];
  if (match.docTop <= 900) {
    broken(`${JSON.stringify(testCase.term)} sits ${match.docTop}px down at ${width}px, inside the first screen, so arriving proves nothing`);
  }
  await context.close();
  return { proof, seen: { width, furigana, href, scrollY: scroll, match } };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const proofs = [];
// Every leg that carried the mutation must have applied it, not only the last.
const unapplied = () => proofs.map((p) => p()).find((issue) => issue) || '';
try {
  for (const testCase of CASES) {
    const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
    const apply = mutation && mutation.target === testCase.site && mutation.query === testCase.query
      ? rewriteResults(mutation.query, mutation.needle, mutation.replacement)
      : null;
    for (const furigana of testCase.furigana || [null]) {
      for (const width of [NARROW, WIDE]) {
        const landed = await land(browser, testCase, width, furigana, apply);
        if (landed.proof) proofs.push(landed.proof);
        if (!landed.seen.match.inView) {
          fail(testCase.site, `the result did not bring ${JSON.stringify(testCase.term)} on screen: ${JSON.stringify(landed.seen)}`);
        }
      }
    }
  }

  const issue = unapplied();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
  console.log(`PASS result-landing-suffix: a result click puts the match on screen in ${SITES.length} cases at ${NARROW}px and at ${WIDE}px`);
} catch (err) {
  if (proofs.length > 0 && !(err instanceof NotApplied)) {
    const issue = unapplied();
    if (issue) {
      console.error(`NOT-APPLIED result-landing-suffix: ${MUTATE}: ${issue}`);
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
