// Behaviour lock for the whole-folder findings table and the shape line above
// it. Claims a rendering test cannot reach, because each is settled by the
// browser rather than by the bytes: on a phone the rows stack and each cell
// says which column it is, instead of the sheet running off the side where
// half of it cannot be read; a heading is a link that really comes back with
// the rows in another order; the shape line's own numbers hold against a
// second count of the table it sits above, taken from the rendered page
// rather than from what the line claims; and error and warn, which used to
// differ only by a border, paint different inks and draw different marker
// shapes — with no script involved in any of it.
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
const SHAPE_SITE = 'the-shape-line-counts-what-the-table-shows';
const INK_SITE = 'error-and-warn-paint-different-inks';
const MARKER_SITE = 'error-and-warn-draw-different-markers';
const SITES = [LABEL_SITE, DETAIL_SITE, BADGE_SITE, EMPTY_SITE, WIDTH_SITE, ORDER_SITE, SHAPE_SITE, INK_SITE, MARKER_SITE];

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

// revertErrorInk undoes the correction ink error moved onto, painting it back
// with warn's own colour — the shape this replaced, where the two weights
// read as one.
const revertErrorInk = (page) => appendStyle(page, `.y-severity--error{color:var(--warn)}`);

// revertErrorMarker undoes the squared dot, rounding error's marker back to
// warn's shape.
const revertErrorMarker = (page) => appendStyle(page, `.y-severity--error::before{border-radius:50%}`);

// dropTheOrdering leaves the headings looking exactly as they do and makes them
// ask for nothing the page reads, which is how a link that has quietly stopped
// being a control behaves.
const dropTheOrdering = (page) => {
  let requests = 0;
  let rewritten = 0;
  return page.route((url) => url.pathname === new URL(BASE + PAGE).pathname, async (route) => {
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

// corruptShapeCount rewrites the number the shape line shows beside its
// "error" weight, one higher than what the page actually served — the kind of
// drift a hand-written second tally invites and a line derived from the same
// rows the table draws cannot produce.
const corruptShapeCount = (page) => {
  let requests = 0;
  let rewritten = 0;
  return page.route((url) => url.pathname === new URL(BASE + PAGE).pathname, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const body = original.replace(
      /(y-severity y-severity--error" lang="en">error<\/span> )(\d+)/,
      (_whole, prefix, digits) => {
        rewritten += 1;
        return `${prefix}${Number(digits) + 1}`;
      },
    );
    await route.fulfill({ response, body });
  }).then(() => () => {
    if (requests < 1) return 'the page was never requested, so nothing was rewritten';
    if (rewritten < 1) return 'the shape line carries no "error" weight to corrupt';
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
  'corrupt-shape-count': { target: SHAPE_SITE, apply: corruptShapeCount },
  'revert-error-ink': { target: INK_SITE, apply: revertErrorInk },
  'revert-error-marker': { target: MARKER_SITE, apply: revertErrorMarker },
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

// readShape reports the shape line as it stands: each weight it names, with
// the word and the count beside it, and the plain text the line closes with.
// null where the line draws nothing at all.
const readShape = (page) => page.evaluate(() => {
  const shape = document.querySelector('.y-healthshape');
  if (!shape) return null;
  const weights = [...shape.querySelectorAll('.y-healthshape__weight')].map((link) => ({
    href: link.getAttribute('href'),
    word: link.querySelector('.y-severity')?.textContent.trim() ?? '',
    count: Number(link.textContent.replace(/\D+/g, '')),
  }));
  const files = shape.querySelector(':scope > span:last-child');
  return { weights, filesText: files ? files.textContent.trim() : '' };
});

// readSeverityTally counts the table's own rows a second time, summing the
// counted column under whatever weight each row's own cell carries — the
// figure the shape line is answerable to.
const readSeverityTally = (page) => page.evaluate(() => {
  const tally = {};
  document.querySelectorAll('.y-findings tbody tr').forEach((row) => {
    const badge = row.querySelector('.y-findings__severity .y-severity');
    if (!badge) return;
    const word = badge.textContent.trim();
    const count = Number(row.querySelector('.y-findings__count')?.textContent.trim() ?? '0');
    tally[word] = (tally[word] ?? 0) + count;
  });
  return tally;
});

// readFileIdentities is what each row of the table names as its file, read as
// an identity rather than as display text: a note's own link carries its
// address, and a path with no note keeps the path.
const readFileIdentities = (page) => page.evaluate(() => [...document.querySelectorAll('.y-findings__file')].map((cell) => {
  const link = cell.querySelector('a.y-healthlink');
  if (link) return `note:${link.getAttribute('href')}`;
  const path = cell.querySelector('.y-findings__path');
  return `path:${(path ?? cell).textContent.trim()}`;
}));

// badgeStyle reads one severity badge's computed ink and its marker's
// computed shape — the two channels error and warn are asked to disagree on,
// read the same way a reader's own browser resolves them rather than from the
// stylesheet's source text.
const badgeStyle = (page, selector) => page.evaluate((sel) => {
  const badge = document.querySelector(sel);
  if (!badge) return null;
  const style = getComputedStyle(badge);
  const marker = getComputedStyle(badge, '::before');
  return { color: style.color, markerRadius: marker.borderRadius };
}, selector);

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

  // The shape line's own numbers, held against a second count of the very
  // rows it sits above — the table's cells, not the line's own claim about
  // them.
  //
  // The line counts the whole report while the table under it holds one page
  // of it, so the two can only be added up against each other where every row
  // is on the page. That rendering is reached by the strip's own last link,
  // which makes this also the proof that the link leads to the whole of it.
  const undivided = await page.locator('.y-pager__whole').first();
  if (await undivided.count() === 1) {
    await page.goto(new URL(await undivided.getAttribute('href'), page.url()).toString(), { waitUntil: 'domcontentloaded' });
    if (await page.locator('.y-pager').count() !== 0) {
      broken('the undivided report still draws a strip, so it is not the whole of it');
    }
  }
  const shape = await readShape(page);
  if (!shape) broken('the fixture holds findings, so the shape line must be on the page');
  if (shape.weights.length === 0) broken('the fixture carries weighed findings, so the shape line must name at least one weight');
  const severityTally = await readSeverityTally(page);
  for (const weight of shape.weights) {
    if (severityTally[weight.word] !== weight.count) {
      fail(SHAPE_SITE, `the shape line counts ${weight.word} as ${weight.count}; the table's own cells add up to ${severityTally[weight.word]}`);
    }
  }
  const tableFiles = new Set(await readFileIdentities(page));
  if (shape.filesText !== String(tableFiles.size) && !shape.filesText.startsWith(`${tableFiles.size} `)) {
    fail(SHAPE_SITE, `the shape line reads ${JSON.stringify(shape.filesText)}; the table's own file cells name ${tableFiles.size} distinct files`);
  }
  // Back to the page this probe is driven at, which everything below measures.
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  // Error and warn used to share one colour and one marker shape, told apart
  // only by a border a glance can miss. The fixture has to carry one of each
  // or neither channel is exercised at all.
  const errorBadge = await badgeStyle(page, '.y-severity--error');
  const warnBadge = await badgeStyle(page, '.y-severity--warn');
  if (!errorBadge || !warnBadge) broken('the fixture carries no error/warn severity pair to compare ink and marker shape on');
  if (errorBadge.color === warnBadge.color) {
    fail(INK_SITE, `error and warn severity badges both paint ${errorBadge.color}, so the two weights read as the same ink`);
  }
  if (errorBadge.markerRadius === warnBadge.markerRadius) {
    fail(MARKER_SITE, `error and warn severity badges both draw a ${errorBadge.markerRadius} marker, so the two weights read as the same shape`);
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

  // A weight in the shape line is a live link to that same reordering, proven
  // by following it — from a fresh, unsorted visit, so the click has an order
  // to actually change.
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  const beforeShape = await readRows(page);
  const shapeLink = page.locator('.y-healthshape__weight').first();
  if (await shapeLink.count() === 0) broken('the fixture carries a weight, so the shape line must offer a weight link to follow');
  const askedShape = page.url();
  await shapeLink.click();
  await page.waitForURL((url) => url.toString() !== askedShape, { timeout: 15000 });
  await page.waitForLoadState('domcontentloaded');
  const afterShape = await readRows(page);
  if (afterShape.length !== beforeShape.length) broken(`the reordered page holds ${afterShape.length} findings, was ${beforeShape.length}`);
  if (afterShape.every((row, i) => row.subject === beforeShape[i].subject)) {
    fail(SHAPE_SITE, 'following a weight link from the shape line returned the rows in exactly the order they were already in');
  }
  for (let i = 1; i < afterShape.length; i += 1) {
    const heavier = WEIGHTS[afterShape[i - 1].weight];
    const lighter = WEIGHTS[afterShape[i].weight];
    if (heavier === undefined || lighter === undefined) broken(`a row weighs ${JSON.stringify(afterShape[i])}, which is no weight this page uses`);
    if (heavier < lighter) {
      fail(SHAPE_SITE, `following the shape line's weight link, row ${i} (${afterShape[i].weight}) is heavier than the row above it (${afterShape[i - 1].weight})`);
    }
  }

  await page.close();
  console.log(`PASS health-table: ${table.rows} findings stack and name their columns at ${PHONE.width}px with no sideways scroll, a heading really reorders them, the shape line's own numbers hold, and error reads heavier than warn`);
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
