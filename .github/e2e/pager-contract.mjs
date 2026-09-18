// Behaviour lock for the strip under a listing that runs past one page, on
// both listings that have one. Claims a rendering test cannot reach, because
// each is settled by a second document the browser actually fetched: following
// "next" really lands on other rows, the query the search page was asked with
// and the ordering the findings table was being read in both survive the step,
// the divisions beside the results keep counting the whole answer rather than
// the rows on screen, and the strip leaves no ink on paper — with no script
// involved in any of it.
//
// The query is the whole of what a facet row writes, so "the constraint
// survives the step" is the same claim as "the query survives it": the fixture
// holds no filtered answer long enough to divide, and a filter that vanished
// from a strip link would vanish exactly as any other part of the query does.
//
// Env: YOMIHON_BASE, PAGE_PATH (a search whose answer runs past one page), and
// MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/search?q=e';
const HEALTH = '/health?sort=severity';
const MUTATE = process.env.MUTATE || '';

const NEXT_SITE = 'next-lands-on-other-rows';
const QUERY_SITE = 'the-strip-keeps-the-query';
const SORT_SITE = 'the-strip-keeps-the-ordering';
const FACET_SITE = 'the-divisions-count-the-answer';
const PRINT_SITE = 'the-strip-leaves-no-ink';
const SITES = [NEXT_SITE, QUERY_SITE, SORT_SITE, FACET_SITE, PRINT_SITE];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL pager-contract: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN pager-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED pager-contract: ${message}`); };

// rewrite serves the pages under one path through a substitution, and reports
// afterwards whether it ever had anything to substitute. A mutation whose
// needle matched nothing proves nothing, which is why the report is separate
// from the route.
const rewrite = (page, pathname, substitute) => {
  let served = 0;
  let changed = 0;
  return page.route(
    (url) => url.pathname === pathname,
    async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      served += 1;
      const body = substitute(original, new URL(route.request().url()));
      if (body !== original) changed += 1;
      await route.fulfill({ response, body });
    },
  ).then(() => () => {
    if (served === 0) return `nothing under ${pathname} was ever requested`;
    if (changed === 0) return `${served} document(s) under ${pathname} carried nothing to rewrite`;
    return '';
  });
};

// appendStyle puts a rule into the product's own stylesheet, past the layer it
// declares, so the injected rule outranks what it stands in for the way a
// regression written into the stylesheet itself would.
const appendStyle = (page, css) => {
  let seen = 0;
  return page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${css}\n` });
  }).then(() => () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page'));
};

// sendNextBack points the way on at the page the reader is already on, which
// is how a strip that has quietly stopped stepping anywhere behaves.
const sendNextBack = (page) => rewrite(page, '/search', (body) =>
  body.replace(/(<a class="y-pager__step" href="\/search\?page=)2(&amp;[^"]*">)/, '$11$2'));

// dropTheQueryOnNext leaves the strip looking exactly as it does and takes the
// reader's own words off the step, which is what a link built from the route
// rather than from the request does — and what a facet constraint would
// disappear with.
const dropTheQueryOnNext = (page) => rewrite(page, '/search', (body) =>
  body.replaceAll(/(<a class="y-pager__[a-z]+" href="\/search\?page=[^"&]+)&amp;q=[^"]*"/g, '$1"'));

// dropTheSortOnNext takes the ordering off the strip's links, so stepping
// through a report being read heaviest-first quietly puts it back in the order
// the reader did not ask for.
const dropTheSortOnNext = (page) => rewrite(page, '/health', (body) =>
  body.replaceAll(/(<a class="y-pager__[a-z]+" href="\?page=[^"&]+)&amp;sort=[^"]*"/g, '$1"'));

// countThePageNotTheAnswer rewrites the divisions on the second page to the
// number of rows that page draws — a column tallied from the results in hand
// instead of from every hit, which is the shape the tally would take if the
// count moved after the list was cut.
const countThePageNotTheAnswer = (page) => rewrite(page, '/search', (body, url) => {
  if (url.searchParams.get('page') !== '2') return body;
  const rows = (body.match(/<a class="y-result" /g) || []).length;
  if (rows === 0) return body;
  return body.replaceAll(/data-facet-count="\d+"/g, `data-facet-count="${rows}"`);
});

// showTheStripOnPaper undoes the one print rule the strip has: a sheet of
// paper carrying page numbers nobody can follow, printed instead of the whole
// listing the last of them leads to.
const showTheStripOnPaper = (page) => appendStyle(page, '@media print{.y-pager{display:flex !important}}');

const MUTATIONS = {
  'repeat-a-row-across-pages': { target: NEXT_SITE, apply: sendNextBack },
  'drop-the-query-on-next': { target: QUERY_SITE, apply: dropTheQueryOnNext },
  'drop-the-sort-on-next': { target: SORT_SITE, apply: dropTheSortOnNext },
  'count-the-page-not-the-answer': { target: FACET_SITE, apply: countThePageNotTheAnswer },
  'strip-prints': { target: PRINT_SITE, apply: showTheStripOnPaper },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`pager-contract: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`pager-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`pager-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// readStrip reports the strip as it stands: every link on it, which number is
// marked as the one being read, and whether the strip is drawn at all.
const readStrip = (page) => page.evaluate(() => {
  const strip = document.querySelector('.y-pager');
  if (!strip) return null;
  const link = (element) => ({
    href: element.getAttribute('href'),
    text: element.textContent.trim(),
    current: element.getAttribute('aria-current') === 'page',
  });
  return {
    name: strip.getAttribute('aria-label') || '',
    display: getComputedStyle(strip).display,
    links: [...strip.querySelectorAll('a')].map(link),
    numbers: [...strip.querySelectorAll('.y-pager__page')].map(link),
    whole: strip.querySelector('.y-pager__whole')?.getAttribute('href') ?? '',
    right: strip.getBoundingClientRect().right,
    documentWidth: document.scrollingElement.scrollWidth,
    viewportWidth: document.documentElement.clientWidth,
  };
});

// readResults is the address of every hit on screen, which is the one thing
// that tells two rows apart across two documents.
const readResults = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-searchpage a.y-result')].map((hit) => hit.getAttribute('href')));

// readFindings identifies each row of the table: the file it is about, read as
// an address rather than as display text, and the kind of finding it reports.
// One file can draw several rows — a note with a broken link and a status its
// type never declared is two of them — so the file alone would read as a
// repeat the moment two of its rows fell either side of a page break.
const readFindings = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-findings tbody tr')].map((row) => {
    const cell = row.querySelector('.y-findings__file');
    const link = cell?.querySelector('a.y-healthlink');
    const file = link ? `note:${link.getAttribute('href')}` : `path:${cell?.textContent.trim() ?? ''}`;
    return `${file}|${row.querySelector('.y-findings__kind')?.textContent.trim() ?? ''}`;
  }));

// readDivisions is every division of the answer as the column states it, keyed
// so two documents can be held against each other.
const readDivisions = (page) => page.evaluate(() => {
  const out = {};
  document.querySelectorAll('.y-searchpage [data-facet-key]').forEach((group) => {
    const key = group.dataset.facetKey;
    group.querySelectorAll('[data-facet-row]').forEach((row) => {
      const label = row.querySelector('.y-facetrow__label')?.textContent.trim() ?? '';
      out[`${key}/${label}`] = Number(row.querySelector('[data-facet-count]')?.dataset.facetCount);
    });
  });
  return out;
});

const answerTotal = (page) => page.evaluate(() =>
  Number(document.querySelector('.y-searchpage [data-live-search-results]')?.dataset.resultCount));

const orderedColumn = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.y-findings thead th[aria-sort]')]
    .map((head) => head.querySelector('a')?.getAttribute('href') ?? ''));

const queryInBox = (page) => page.locator('.y-searchpage [data-live-search-input]').inputValue();

// stepOn follows the last step on the strip, which is the way on, and waits
// for the document that answers it.
const stepOn = async (page) => {
  const next = page.locator('.y-pager__step').last();
  if (await next.count() !== 1) broken('the strip carries no step to follow');
  const asked = page.url();
  await next.click();
  await page.waitForURL((url) => url.toString() !== asked, { timeout: 15000 });
  await page.waitForLoadState('domcontentloaded');
};

const overlap = (before, after) => before.filter((value) => after.includes(value));

// proof reports whether the mutation ever reached a document. It is read both
// when every assertion held — where a mutation that changed nothing is the only
// explanation — and when one fired, so a catch is a catch and not a lock firing
// over a page nothing was injected into.
let proof = null;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  // --- the search page ------------------------------------------------------
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  const firstStrip = await readStrip(page);
  if (!firstStrip) broken(`${PAGE} answers with one page, so there is no strip to drive`);
  if (!firstStrip.name) fail(QUERY_SITE, 'the strip has no accessible name, so a run of bare numbers stands for nothing');
  if (firstStrip.numbers.filter((number) => number.current).length !== 1) {
    fail(NEXT_SITE, `the strip marks ${firstStrip.numbers.filter((n) => n.current).length} numbers as the page being read, want 1`);
  }
  if (!firstStrip.whole) fail(QUERY_SITE, 'the strip offers no undivided listing, so nothing prints whole');

  const asked = await queryInBox(page);
  if (!asked) broken('the search box is empty, so there is no query for a strip link to keep');
  for (const link of firstStrip.links) {
    const carried = new URL(link.href, BASE).searchParams.get('q');
    if (carried !== asked) {
      fail(QUERY_SITE, `the strip link ${JSON.stringify(link.href)} carries q=${JSON.stringify(carried)}, and the box holds ${JSON.stringify(asked)}`);
    }
  }

  const firstHits = await readResults(page);
  if (firstHits.length === 0) broken('the first page draws no hits at all');
  const firstDivisions = await readDivisions(page);
  if (Object.keys(firstDivisions).length === 0) broken('the answer divides along nothing, so the column cannot be checked');
  const total = await answerTotal(page);
  if (!Number.isInteger(total) || total <= firstHits.length) {
    broken(`the answer holds ${total} hits and the page draws ${firstHits.length}, so nothing here is a divided listing`);
  }

  await stepOn(page);
  const secondHits = await readResults(page);
  if (secondHits.length === 0) broken(`following the way on landed on ${page.url()}, which draws no hits`);
  const repeated = overlap(firstHits, secondHits);
  if (repeated.length > 0) {
    fail(NEXT_SITE, `following the way on, ${repeated.length} hit(s) are on both pages, starting with ${repeated[0]}`);
  }

  if (await answerTotal(page) !== total) {
    fail(FACET_SITE, `the answer holds ${await answerTotal(page)} hits on the second page and ${total} on the first`);
  }
  const secondDivisions = await readDivisions(page);
  for (const [division, count] of Object.entries(firstDivisions)) {
    if (secondDivisions[division] !== count) {
      fail(FACET_SITE, `the division ${division} counts ${secondDivisions[division]} on the second page and ${count} on the first, so it counts the rows on screen`);
    }
  }
  for (const [division, count] of Object.entries(secondDivisions)) {
    if (!(count > 0) || count > total) {
      fail(FACET_SITE, `the division ${division} counts ${count} of an answer of ${total}`);
    }
  }
  const secondStrip = await readStrip(page);
  if (!secondStrip) broken('the second page draws no strip, so there is no way back');
  if (!secondStrip.links.some((link) => link.text && new URL(link.href, BASE).searchParams.get('page') === '1')) {
    fail(NEXT_SITE, 'the second page offers no way back to the first');
  }

  // --- the strip leaves no ink ---------------------------------------------
  await page.emulateMedia({ media: 'print' });
  const printed = await readStrip(page);
  if (printed && printed.display !== 'none') {
    fail(PRINT_SITE, `the strip prints as ${printed.display}: a sheet of page numbers nobody can follow, instead of the listing the last of them leads to`);
  }
  await page.emulateMedia({ media: 'screen' });

  // --- the findings table ---------------------------------------------------
  await page.goto(BASE + HEALTH, { waitUntil: 'domcontentloaded' });
  const ordered = await orderedColumn(page);
  if (ordered.length !== 1) broken(`the table marks ${ordered.length} columns as the ordering in force, want 1`);
  const tableStrip = await readStrip(page);
  if (!tableStrip) broken(`${HEALTH} holds one page of findings, so there is no strip to drive`);
  for (const link of tableStrip.links) {
    if (new URL(link.href, BASE + HEALTH).searchParams.get('sort') !== 'severity') {
      fail(SORT_SITE, `the strip link ${JSON.stringify(link.href)} drops the ordering the table is being read in`);
    }
  }

  const firstRows = await readFindings(page);
  if (firstRows.length === 0) broken('the first page of the report draws no rows');
  await stepOn(page);
  const secondRows = await readFindings(page);
  if (secondRows.length === 0) broken(`following the way on landed on ${page.url()}, which draws no findings`);
  const repeatedRows = overlap(firstRows, secondRows);
  if (repeatedRows.length > 0) {
    fail(NEXT_SITE, `${repeatedRows.length} finding(s) are on both pages of the report, starting with ${repeatedRows[0]}`);
  }
  const stillOrdered = await orderedColumn(page);
  if (stillOrdered.length !== 1 || !stillOrdered[0].endsWith('=severity')) {
    fail(SORT_SITE, `after the step the table marks ${JSON.stringify(stillOrdered)} as the ordering, want the weight column alone`);
  }

  // --- a phone's width ------------------------------------------------------
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(BASE + HEALTH, { waitUntil: 'domcontentloaded' });
  const narrow = await readStrip(page);
  if (!narrow) broken('the report draws no strip at a phone width');
  if (narrow.documentWidth > narrow.viewportWidth + 1) {
    fail(SORT_SITE, `the report is ${narrow.documentWidth}px wide in a ${narrow.viewportWidth}px viewport, so it scrolls sideways`);
  }
  if (narrow.right > narrow.viewportWidth + 1) {
    fail(SORT_SITE, `the strip reaches ${narrow.right}px past the ${narrow.viewportWidth}px edge of the viewport`);
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  await page.close();
  console.log(`PASS pager-contract: ${total} hits and a report both step on to other rows, keeping the query and the ordering, with the divisions still counting the answer and no ink on paper`);
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE) {
      const issue = proof ? proof() : 'the mutation was never applied';
      if (issue) {
        console.error(`NOT-APPLIED pager-contract: ${MUTATE}: ${issue}`);
        console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
        process.exitCode = 2;
        throw err;
      }
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
