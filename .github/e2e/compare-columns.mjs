// Behavior lock for the page that holds two notes at once.
//
// Side by side, the two are beside one another and each moves on its own —
// which is the whole reason a reader opens this page rather than two windows.
// Where two columns no longer fit, one note is in view at a time, the page
// never scrolls sideways, and the strip above them reaches the other note by
// keyboard alone. And whichever the width, a column is the note's own article:
// what a reader sees here is what they would see opening that note, under the
// names that column's places answer to — its contents list included, which
// marks where its own note is being read and never where the other one is.
//
// The measurements are of laid-out boxes rather than of declared rules: a
// grid that stacks, a column that stopped being its own scroller, and a page
// grown sideways by something inside it are all invisible to the stylesheet
// and plain in the geometry.
//
// Env: YOMIHON_BASE, PAGE_PATH (a side-by-side address), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/compare/Notes/cutover.md?with=Notes%2Fcutover-zh-tw.md';
const MUTATE = process.env.MUTATE || '';

// The id space the second column's places answer to, spelled here as the
// literal bytes the page carries. Read out of a constant it would agree with
// whatever the page happened to write, including nothing at all.
const COLUMN_B_PREFIX = 'b-';

const SITES = [
  'columns-side-by-side',
  'columns-scroll-alone',
  'narrow-tabs-switch-columns',
  'narrow-no-sideways-scroll',
  'column-is-the-note',
  'align-levels-the-columns',
  'contents-mark-stays-in-its-column',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN compare-columns: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL compare-columns: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN compare-columns: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED compare-columns: ${message}`); };

// weakenStylesheet appends one rule to the served stylesheet, which is how a
// layout rule is taken away without touching the tree.
const weakenStylesheet = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'app.css was never requested, so the override reached no page');
};

// rewritePage serves the page itself with one substitution made, for the
// regressions that are a change in what the server wrote rather than in how it
// is laid out. replaceAll, so a changed body means a body wholly rewritten and
// a needle that matched nothing still reports itself.
const rewritePage = (needle, replacement, what) => async (page) => {
  let applied = false;
  let served = 0;
  await page.route((url) => url.pathname.startsWith('/compare/'), async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    served += 1;
    const body = original.replaceAll(needle, replacement);
    applied = applied || body !== original;
    await route.fulfill({ response, body });
  });
  return () => {
    if (served === 0) return 'the side-by-side page was never served, so the rewrite reached nothing';
    return applied ? '' : `${what} matched nothing in the page`;
  };
};

const rewriteScript = (needle, replacement, what) => async (page) => {
  let applied = false;
  let served = 0;
  await page.route('**/static/contents.js', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    served += 1;
    const body = original.replaceAll(needle, replacement);
    applied = applied || body !== original;
    await route.fulfill({ response, body });
  });
  return () => {
    if (served === 0) return 'contents.js was never requested, so the rewrite reached no page';
    return applied ? '' : `${what} matched nothing in the script`;
  };
};

const MUTATIONS = {
  // The grid collapsed to one track: the two notes are still both on the page
  // and no longer beside one another, which is the page's whole claim.
  'merge-the-columns': {
    target: 'columns-side-by-side',
    apply: weakenStylesheet('.y-compare{grid-template-columns:1fr !important}'),
  },
  // The columns stop being scrollers of their own, so the document scrolls and
  // moving through one note carries the other away with it.
  'let-the-columns-share-a-scrollbar': {
    target: 'columns-scroll-alone',
    apply: weakenStylesheet('.y-compare__column{overflow:visible !important;max-height:none !important}'),
  },
  // The links to the columns gone, which is the only way to the other note
  // once the columns are stacked and one of them fills the screen. Both of the
  // regressions below are written inside the width they belong to: a rule that
  // also changed the wide layout would be caught by an assertion made earlier,
  // and a mode has to answer for the site it names.
  'drop-the-narrow-tabs': {
    target: 'narrow-tabs-switch-columns',
    apply: weakenStylesheet('@media (max-width:1100px){.y-compare__tab{display:none !important}}'),
  },
  // Something inside a column setting the page's floor, which is the trap the
  // minimum width on the grid children exists to close.
  'let-a-column-set-the-floor': {
    target: 'narrow-no-sideways-scroll',
    apply: weakenStylesheet('@media (max-width:1100px){.y-compare__column{min-width:760px !important}}'),
  },
  // One element taken out of the second column, so what the page shows there
  // stops being the note.
  'break-column-identity': {
    target: 'column-is-the-note',
    apply: rewritePage('<details class="y-toc-inline">', '<details class="y-toc-inline" data-broken>', 'the disclosure marker'),
  },
  // The levelling that answers the press, gone: the control still takes its
  // pressed state and the other column stays where it was.
  'never-level-the-columns': {
    target: 'align-levels-the-columns',
    apply: rewriteScript('follow(1);', ';', 'the levelling the press asks for'),
  },
  // Both contents lists computed against one reading position again, which is
  // what the module did before a page could hold two notes: the mark lands in
  // whichever column the position was read from, and the other note's list is
  // left with none.
  'let-one-mark-answer-for-both': {
    target: 'contents-mark-stays-in-its-column',
    apply: rewriteScript(
      "list.closest('[data-note-column]') ?? document",
      'document',
      'the column a contents list answers for',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`compare-columns: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`compare-columns: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`compare-columns: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// articleOf cuts one note's article out of a served page. Articles do not nest
// here, so the close that ends one is the first one after it opens.
const articleOf = (html, from) => {
  const at = html.indexOf('<article class="y-article"', from);
  if (at < 0) return '';
  const end = html.indexOf('</article>', at);
  if (end < 0) return '';
  return html.slice(at, end + '</article>'.length);
};

// upToTheWriteFace drops the one thing a column does not carry. The note's own
// page ends its article with the status face; a column passes nothing where the
// page passes that in, so the comparison stops where the face begins. The
// closing tag comes off first, or the column — which has no face to cut at —
// would keep a close the note's own article no longer has.
const upToTheWriteFace = (article) => {
  const close = '</article>';
  const body = article.endsWith(close) ? article.slice(0, -close.length) : article;
  const at = body.indexOf('<section class="y-sealbar"');
  return at < 0 ? body : body.slice(0, at);
};

const boxOf = (locator) => locator.evaluate((element) => {
  const rect = element.getBoundingClientRect();
  return { left: rect.left, top: rect.top, width: rect.width, height: rect.height };
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Short enough that a note of the fixture's length overflows a column, so
  // "each column moves on its own" has something to move.
  const context = await browser.newContext({ viewport: { width: 1280, height: 440 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const served = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const columnA = page.locator('#compare-a');
  const columnB = page.locator('#compare-b');
  if ((await columnA.count()) !== 1 || (await columnB.count()) !== 1) {
    broken(`${PAGE} does not draw two columns, so nothing here can be judged`);
  }

  const wide = { a: await boxOf(columnA), b: await boxOf(columnB) };
  if (wide.a.left === wide.b.left) {
    fail('columns-side-by-side', `both columns start at x=${wide.a.left}, so the two notes are stacked where there is room for both`);
  }
  for (const [name, box] of Object.entries(wide)) {
    if (box.width > 1280 * 0.55) {
      fail('columns-side-by-side', `column ${name} is ${Math.round(box.width)}px of a 1280px page, so it is not sharing the width`);
    }
  }

  // The content has to be taller than the window for a scroller to have
  // anywhere to go. That is a fact about the fixture, not about the layout.
  const articleHeight = await columnB.locator('article.y-article').evaluate((el) => el.getBoundingClientRect().height);
  if (articleHeight <= 440) {
    broken(`the second note is ${Math.round(articleHeight)}px tall in a 440px window, so a column has nothing to scroll`);
  }
  const scrolled = await page.evaluate(() => {
    const [a, b] = [document.getElementById('compare-a'), document.getElementById('compare-b')];
    const ownScroller = (el) => el.scrollHeight - el.clientHeight > 20;
    if (!ownScroller(a) || !ownScroller(b)) return { ownScroller: false, moved: 0, other: 0 };
    b.scrollTop = 0;
    a.scrollTop = 0;
    a.scrollTop = 120;
    return { ownScroller: true, moved: a.scrollTop, other: b.scrollTop };
  });
  if (!scrolled.ownScroller) {
    fail('columns-scroll-alone', 'neither column is a scroller of its own, so moving through one note carries the other with it');
  }
  if (scrolled.moved < 100) {
    broken(`column A took only ${scrolled.moved}px of a 120px scroll, so this measurement says nothing`);
  }
  if (scrolled.other !== 0) {
    fail('columns-scroll-alone', `moving column A by ${scrolled.moved}px moved column B to ${scrolled.other}px`);
  }

  // A column is the note itself, under the names that column's places answer
  // to. The rewriting is by literal bytes rather than by a shared constant, so
  // a page that stopped naming its places cannot satisfy this by agreeing with
  // itself.
  // The bytes the server wrote, not the document the enhancement runtime has
  // been editing since: marking a contents entry as the one being read is a
  // change this page makes to itself, and the note's own page fetched beside it
  // has had no script run over it at all.
  const comparedHTML = await served.text();
  const notePath = new URLSearchParams(PAGE.slice(PAGE.indexOf('?') + 1)).get('with');
  const alone = await (await page.request.get(`${BASE}/notes/${notePath.split('/').map(encodeURIComponent).join('/')}`)).text();
  const columnHTML = articleOf(comparedHTML, comparedHTML.indexOf('id="compare-b"'));
  const aloneHTML = articleOf(alone, 0);
  if (columnHTML === '' || aloneHTML === '') {
    broken('one of the two renderings of the second note carries no article at all');
  }
  const renamed = columnHTML
    .replaceAll(` id="${COLUMN_B_PREFIX}`, ' id="')
    .replaceAll(` href="#${COLUMN_B_PREFIX}`, ' href="#');
  if (renamed === columnHTML) {
    fail('column-is-the-note', 'the second column names none of its places, so the two notes on this page answer to one set of them');
  }
  if (upToTheWriteFace(renamed) !== upToTheWriteFace(aloneHTML)) {
    fail('column-is-the-note', 'the second column is not what the note reads as on its own page');
  }

  // Levelling is the one thing here a script does, and it answers the press
  // rather than moving anything by itself.
  const toggle = page.locator('[data-compare-align]');
  if ((await toggle.count()) !== 1) {
    broken('the page draws no levelling control, so its behaviour cannot be judged');
  }
  // The first column is parked at its last heading and the second put back to
  // the top, so the two are demonstrably apart before the control is pressed.
  const apart = await page.evaluate(() => {
    const a = document.getElementById('compare-a');
    const b = document.getElementById('compare-b');
    b.scrollTop = 0;
    const headings = [...a.querySelectorAll('.y-prose [data-level]')];
    if (headings.length === 0) return { headings: 0, above: 0 };
    a.scrollTop += headings[headings.length - 1].getBoundingClientRect().top - 72;
    return {
      headings: headings.length,
      above: headings.filter((heading) => heading.getBoundingClientRect().top <= 80).length,
    };
  });
  if (apart.headings === 0 || apart.above === 0) {
    broken(`the first note has ${apart.headings} headings and ${apart.above} of them above the reading line, so there is no place to level the other column to`);
  }
  await toggle.click();
  const levelled = await page.evaluate(() => ({
    pressed: document.querySelector('[data-compare-align]').getAttribute('aria-pressed'),
    other: document.getElementById('compare-b').scrollTop,
  }));
  if (levelled.pressed !== 'true') {
    fail('align-levels-the-columns', `the levelling control reports aria-pressed=${levelled.pressed} after being pressed`);
  }
  if (levelled.other === 0) {
    fail('align-levels-the-columns', 'pressing the levelling control left the second column where it was, so the two are still apart');
  }

  // Each note's contents list marks where that note is being read. The columns
  // are put back on their own and taken to different places first, because a
  // mark computed across both would still look right while they are level.
  await toggle.click();
  const released = await page.evaluate(() => document.querySelector('[data-compare-align]').getAttribute('aria-pressed'));
  if (released !== 'false') {
    fail('align-levels-the-columns', `the levelling control reports aria-pressed=${released} after a second press, so it cannot be let go of`);
  }
  const reading = await page.evaluate(() => {
    const a = document.getElementById('compare-a');
    const b = document.getElementById('compare-b');
    b.scrollTop = 0;
    const headings = [...a.querySelectorAll('.y-prose [data-level]')];
    a.scrollTop += headings[headings.length - 1].getBoundingClientRect().top - 72;
    return { a: a.scrollTop, b: b.scrollTop };
  });
  if (reading.a === reading.b) {
    broken(`both columns are at ${reading.a}px, so a mark taken from the wrong one would read as right`);
  }
  // The mark is recomputed from an observer, which answers after the scroll
  // rather than during it.
  await page.waitForTimeout(200);
  const marks = await page.evaluate(() => [...document.querySelectorAll('[data-note-column]')].map((column) => {
    const marked = [...column.querySelectorAll('.y-toc__list a[aria-current="true"]')];
    return {
      id: column.id,
      lists: column.querySelectorAll('.y-toc__list').length,
      marked: marked.length,
      strayed: marked.filter((link) => {
        const heading = document.getElementById(decodeURIComponent(link.getAttribute('href').slice(1)));
        return !heading || heading.closest('[data-note-column]') !== column;
      }).length,
    };
  }));
  for (const column of marks) {
    if (column.lists === 0) {
      broken(`${column.id} draws no contents list, so there is no mark to judge`);
    }
    if (column.marked === 0) {
      fail('contents-mark-stays-in-its-column', `nothing is marked as being read in ${column.id}, so the one mark this page holds is answering for the other note`);
    }
    if (column.strayed > 0) {
      fail('contents-mark-stays-in-its-column', `${column.strayed} marked entries in ${column.id} name a heading outside it`);
    }
  }

  // One note in view at a time, the page moving only downwards, and the other
  // note one keyboard press away.
  await page.setViewportSize({ width: 390, height: 780 });
  await page.evaluate(() => { window.scrollTo(0, 0); });
  const sideways = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
  }));
  if (sideways.scrollWidth > sideways.innerWidth) {
    fail('narrow-no-sideways-scroll', `the page is ${sideways.scrollWidth}px wide in a ${sideways.innerWidth}px window, so the reader has to scroll sideways`);
  }

  const reached = await page.evaluate(() => {
    const tabs = [...document.querySelectorAll('.y-compare__tabs a')];
    return tabs.filter((tab) => tab.getBoundingClientRect().width > 0).length;
  });
  if (reached !== 2) {
    fail('narrow-tabs-switch-columns', `${reached} of the two links to the columns are drawn, so the other note cannot be reached`);
  }
  // From the top of the document, so what is measured is how far into the page
  // a reader has to tab rather than wherever the pointer left the focus.
  await page.evaluate(() => { document.activeElement?.blur(); });
  let guard = 0;
  let here = '';
  while (guard < 40 && here !== '#compare-b') {
    await page.keyboard.press('Tab');
    here = await page.evaluate(() => document.activeElement?.getAttribute('href') || '');
    guard += 1;
  }
  if (here !== '#compare-b') {
    fail('narrow-tabs-switch-columns', `${guard} presses of Tab from the top of the page never reached the link to the other note`);
  }
  await page.keyboard.press('Enter');
  const landed = await page.evaluate(() => document.getElementById('compare-b').getBoundingClientRect().top);
  if (landed > 200) {
    fail('narrow-tabs-switch-columns', `pressing the link left the other note ${Math.round(landed)}px below the top of the window`);
  }

  await context.close();
  console.log('PASS compare-columns: two columns beside one another each move alone, the second is the note itself, levelling answers the press, each contents list marks its own note, and at 390px one note is in view with the other a keyboard press away');
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
