// Behaviour lock for the whole-folder findings table. Two claims a rendering
// test cannot reach, because both are settled by the browser rather than by the
// bytes: on a phone the rows stack and each cell says which column it is,
// instead of the sheet running off the side where half of it cannot be read;
// and a heading is a link that really comes back with the rows in another
// order, with no script involved in either.
//
// Env: YOMIHON_BASE, PAGE_PATH (the whole-folder page), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/health';
const MUTATE = process.env.MUTATE || '';
const LABEL_SITE = 'stacked-cells-name-their-column';
const DETAIL_SITE = 'evidence-sits-beside-the-finding';
const BADGE_SITE = 'the-weight-is-a-badge-not-a-banner';
const EMPTY_SITE = 'no-label-over-an-empty-cell';
const WIDTH_SITE = 'no-sideways-scroll';
const ORDER_SITE = 'a-heading-reorders-the-rows';
const SITES = [LABEL_SITE, DETAIL_SITE, BADGE_SITE, EMPTY_SITE, WIDTH_SITE, ORDER_SITE];

// The width a phone gives the page. Narrow enough that a four-column table
// laid out as a table cannot hold its columns.
const PHONE = { width: 390, height: 844 };

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL health-table: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN health-table: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED health-table: ${message}`); };

// appendStyle puts a rule into the product's own stylesheet, past the layer it
// declares, so the injected rule outranks what it stands in for without an
// importance flag — the same shape a regression written into the stylesheet
// itself would have.
const appendStyle = (page, css) => {
  let seen = 0;
  return page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${css}\n` });
  }).then(() => () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page'));
};

// restoreTableLayout undoes the narrow-width stacking: the rows go back to
// being table rows, and the column names the cells carry stop being drawn.
const restoreTableLayout = (page) => appendStyle(page, `@media screen and (max-width:${PHONE.width + 1}px){
  .y-findings{display:table}
  .y-findings thead{display:table-header-group}
  .y-findings tbody{display:table-row-group}
  .y-findings tr{display:table-row}
  .y-findings th,.y-findings td{display:table-cell}
  .y-findings tbody td::before{content:none}
}`);

// widenTheCells gives every cell more width than the phone has, which is what
// any un-wrappable content in the table would do.
const widenTheCells = (page) => appendStyle(page, `.y-findings tbody td{min-width:${PHONE.width + 200}px}`);

// unpinTheEvidenceColumn lets a cell's second piece flow where the grid puts
// it, which is back under the label in a fifth of the width it needs.
const unpinTheEvidenceColumn = (page) => appendStyle(page, `@media screen and (max-width:${PHONE.width + 1}px){
  .y-findings tbody td>*{grid-column:auto}
}`);

// stretchTheWeightBadge lets the badge take the whole column, which is what a
// grid cell does when nothing tells it not to.
const stretchTheWeightBadge = (page) => appendStyle(page, `@media screen and (max-width:${PHONE.width + 1}px){
  .y-findings__severity .y-severity{justify-self:stretch}
}`);

// showTheEmptyCells brings back the cell that holds nothing, so a stacked row
// carries a column's name over a blank line.
const showTheEmptyCells = (page) => appendStyle(page, `@media screen and (max-width:${PHONE.width + 1}px){
  .y-findings tbody td:empty{display:grid}
}`);

// dropTheOrdering leaves the headings looking exactly as they do and makes them
// ask for nothing the page reads, which is how a link that has quietly stopped
// being a control behaves.
const dropTheOrdering = (page) => {
  let requests = 0;
  let rewritten = 0;
  return page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const matches = original.match(/\?sort=/g) || [];
    rewritten += matches.length;
    await route.fulfill({ response, body: original.replaceAll('?sort=', '?unsorted=') });
  }).then(() => () => {
    if (requests < 1) return 'the page was never requested, so nothing was rewritten';
    if (rewritten < 2) return `the document carried ${rewritten} ordering links, want at least 2`;
    return '';
  });
};

const MUTATIONS = {
  'restore-table-layout': { target: LABEL_SITE, apply: restoreTableLayout },
  'unpin-the-evidence-column': { target: DETAIL_SITE, apply: unpinTheEvidenceColumn },
  'stretch-the-weight-badge': { target: BADGE_SITE, apply: stretchTheWeightBadge },
  'show-the-empty-cells': { target: EMPTY_SITE, apply: showTheEmptyCells },
  'widen-the-cells': { target: WIDTH_SITE, apply: widenTheCells },
  'drop-the-ordering': { target: ORDER_SITE, apply: dropTheOrdering },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`health-table: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`health-table: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`health-table: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// readTable reports what the table is, measured against a reference element
// pinned to the viewport rather than against a screenshot or an assumed width.
const readTable = (page) => page.evaluate(() => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const viewport = ruler.getBoundingClientRect();
  ruler.remove();

  const box = (element) => {
    const rect = element.getBoundingClientRect();
    return { left: rect.left, right: rect.right, width: rect.width };
  };
  const cells = [...document.querySelectorAll('.y-findings tbody td')].map((cell) => {
    const before = getComputedStyle(cell, '::before');
    const kind = cell.querySelector('.y-findings__kind');
    const detail = cell.querySelector('.y-findings__detail');
    const badge = cell.querySelector('.y-severity');
    return {
      column: cell.dataset.col || '',
      hidden: getComputedStyle(cell).display === 'none',
      labelDrawn: !['none', 'normal', '""'].includes(before.content),
      filled: cell.textContent.trim() !== '' || cell.children.length > 0,
      box: box(cell),
      kind: kind && box(kind),
      detail: detail && box(detail),
      badge: badge && box(badge),
    };
  });
  return {
    viewportWidth: viewport.width,
    viewportRight: viewport.right,
    documentWidth: document.scrollingElement.scrollWidth,
    rows: document.querySelectorAll('.y-findings tbody tr').length,
    headings: [...document.querySelectorAll('.y-findings thead a')].map((link) => ({
      text: link.textContent.trim(),
      href: link.getAttribute('href'),
      sorted: link.closest('th').getAttribute('aria-sort') || '',
      visible: link.getBoundingClientRect().width > 0,
    })),
    cells,
  };
});

// readRows reports each row's subject and weight, top to bottom, which is what
// an ordering has to be judged on.
const readRows = (page) => page.evaluate(() => [...document.querySelectorAll('.y-findings tbody tr')].map((row) => ({
  subject: row.querySelector('.y-findings__file')?.textContent.trim() ?? '',
  weight: row.querySelector('.y-findings__severity')?.textContent.trim() ?? '',
})));

const WEIGHTS = { error: 3, warn: 2, info: 1, '': 0 };

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: PHONE });
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const table = await readTable(page);
  if (table.rows < 4) broken(`the fixture has ${table.rows} findings, which is too few to say anything about how they lay out`);
  if (table.headings.length !== 4) broken(`the table offers ${table.headings.length} orderings, want the four columns`);
  if (table.cells.length !== table.rows * 4) broken(`the table has ${table.cells.length} cells over ${table.rows} rows, want four each`);

  const shown = table.cells.filter((cell) => !cell.hidden);
  for (const cell of shown) {
    if (!cell.column) fail(LABEL_SITE, `a cell at ${PHONE.width}px carries no column name to be led by`);
    if (!cell.labelDrawn) fail(LABEL_SITE, `the cell for ${cell.column} draws no column name at ${PHONE.width}px, so a stacked row does not say what it holds`);
  }
  for (const heading of table.headings) {
    if (!heading.visible) fail(LABEL_SITE, `the ${heading.text} ordering is not reachable at ${PHONE.width}px, which is the width the table is longest at`);
  }

  // A finding's evidence belongs under the finding, in the column the label
  // points at. Laid out beside the label instead it reads as a second label
  // and wraps into a fraction of the width it needs.
  const withEvidence = shown.filter((cell) => cell.kind && cell.detail);
  if (withEvidence.length === 0) broken('no finding in the fixture carries evidence, so where the evidence lands cannot be seen');
  for (const cell of withEvidence) {
    if (Math.abs(cell.detail.left - cell.kind.left) > 1) {
      fail(DETAIL_SITE, `evidence starts at ${cell.detail.left}px and the finding it belongs to at ${cell.kind.left}px, so it sits in the label's column`);
    }
  }

  // A badge is as wide as the word in it, whatever column it is laid out in.
  const withBadge = shown.filter((cell) => cell.badge);
  if (withBadge.length === 0) broken('no finding in the fixture carries a weight, so the badge cannot be measured');
  for (const cell of withBadge) {
    const column = cell.box.right - cell.badge.left;
    if (cell.badge.width > column - 2) {
      fail(BADGE_SITE, `the weight badge is ${cell.badge.width}px across a ${column}px column, so it is painted as a banner rather than a mark`);
    }
  }

  // A stacked row lists what the finding has. A column's name over nothing
  // says a value went missing, which is a different claim from the one the
  // page is making — that no rule weighs this kind at all.
  for (const cell of shown) {
    if (cell.labelDrawn && !cell.filled) {
      fail(EMPTY_SITE, `the ${cell.column} cell is named over nothing at ${PHONE.width}px`);
    }
  }
  if (table.cells.length === shown.length) {
    broken('every cell in the fixture holds something, so nothing here exercises the empty one');
  }

  if (table.documentWidth > table.viewportWidth + 1) {
    fail(WIDTH_SITE, `the page is ${table.documentWidth}px wide in a ${table.viewportWidth}px viewport, so it scrolls sideways`);
  }
  const widest = Math.max(...shown.map((cell) => cell.box.right));
  if (widest > table.viewportRight + 1) {
    fail(WIDTH_SITE, `a cell reaches ${widest}px past the ${table.viewportRight}px edge of the viewport`);
  }

  const before = await readRows(page);
  const weights = new Set(before.map((row) => row.weight));
  if (weights.size < 2) broken(`every finding in the fixture weighs the same (${[...weights]}), so reordering by weight could not be seen`);

  const severity = page.locator('.y-findings thead a[href$="=severity"]');
  if (await severity.count() !== 1) broken(`the table offers ${await severity.count()} ways to order by weight, want 1`);
  // The ordering is a whole new document, so the rows are read after that
  // document has arrived rather than after whatever was still on screen the
  // moment the click returned.
  const asked = page.url();
  await severity.click();
  await page.waitForURL((url) => url.toString() !== asked, { timeout: 15000 });
  await page.waitForLoadState('domcontentloaded');
  if (await page.locator('.y-findings tbody tr').count() === 0) {
    broken(`following the weight heading landed on ${page.url()}, which carries no findings table at all`);
  }

  const after = await readRows(page);
  if (after.length !== before.length) broken(`the reordered page holds ${after.length} findings, was ${before.length}`);
  if (after.every((row, i) => row.subject === before[i].subject)) {
    fail(ORDER_SITE, 'following the weight heading returned the rows in exactly the order they were already in');
  }
  for (let i = 1; i < after.length; i += 1) {
    const heavier = WEIGHTS[after[i - 1].weight];
    const lighter = WEIGHTS[after[i].weight];
    if (heavier === undefined || lighter === undefined) broken(`a row weighs ${JSON.stringify(after[i])}, which is no weight this page uses`);
    if (heavier < lighter) {
      fail(ORDER_SITE, `ordered by weight, row ${i} (${after[i].weight}) is heavier than the row above it (${after[i - 1].weight})`);
    }
  }
  const sorted = (await readTable(page)).headings.filter((heading) => heading.sorted);
  if (sorted.length !== 1 || !sorted[0].href.endsWith('=severity')) {
    fail(ORDER_SITE, `the reordered page marks ${JSON.stringify(sorted)} as the ordering in force, want the weight heading alone`);
  }

  await page.close();
  console.log(`PASS health-table: ${table.rows} findings stack and name their columns at ${PHONE.width}px with no sideways scroll, and a heading really reorders them`);
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
