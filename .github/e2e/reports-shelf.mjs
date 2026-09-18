// Behaviour lock for the reports shelf. A report is dated by nature and the
// row is laid out around that: the day leads, the name follows, the line the
// report opens with runs under it, and the kind sits at the far edge. Four
// claims a rendering test cannot reach, because all four are settled by the
// browser rather than by the bytes — where the day is drawn relative to the
// name, that the opening really is held to one line rather than merely told
// to be, that nothing on the page reaches past a phone's edge at the width
// the rows are longest at, and that the days a reader sees really do run
// newest first down the page.
//
// A day is recognised by its shape, never by the words beside it, so this
// says the same thing whichever language the interface is speaking.
//
// Env: YOMIHON_BASE, PAGE_PATH (the reports shelf), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/reports';
const MUTATE = process.env.MUTATE || '';
const ANSWER_SITE = 'every-row-names-a-day';
const LEAD_SITE = 'the-day-leads-the-row';
const ONELINE_SITE = 'the-opening-stays-one-line';
const WIDTH_SITE = 'no-sideways-scroll';
const ORDER_SITE = 'newest-first';
const SITES = [ANSWER_SITE, LEAD_SITE, ONELINE_SITE, WIDTH_SITE, ORDER_SITE];

// The two widths the shelf is read at: a phone, which is where the row has to
// fold, and a laptop, which is where it has room for its three columns.
const PHONE = { width: 390, height: 844 };
const DESK = { width: 1280, height: 900 };
const WIDTHS = [PHONE, DESK];

// A day, as the vault writes one. Anything else in that column is the
// interface saying the report carries none, in whichever language.
const DAY = /^\d{4}-\d{2}-\d{2}$/;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL reports-shelf: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN reports-shelf: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED reports-shelf: ${message}`); };

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

// hideTheDay stops the day being drawn, which is what a row whose date face
// was dropped from the template looks like from here.
const hideTheDay = (page) => appendStyle(page, '.y-row__when{display:none}');

// dropTheDayBelowTheName puts the day after everything else in the row, which
// is where it lands the moment nothing places it.
const dropTheDayBelowTheName = (page) => appendStyle(page, '.y-row--dated .y-row__when{grid-row:4}');

// letTheOpeningWrap takes the one-line hold off the opening, so a first
// sentence runs down the page and the listing stops being a listing.
const letTheOpeningWrap = (page) => appendStyle(page, '.y-row__opening{white-space:normal}');

// widenTheRows gives each row more width than a phone has, which is what any
// content in it that cannot wrap would do.
const widenTheRows = (page) => appendStyle(page, `.y-row--dated{min-width:${PHONE.width + 210}px}`);

// unsortTheShelf serves the same rows in the opposite order, which is the page
// a shelf that had quietly stopped ordering by date would return.
const unsortTheShelf = (page) => {
  let requests = 0;
  let reversed = 0;
  const list = /(<div class="y-list">)(.*?)(<\/div>)/s;
  const row = /<a class="y-row[^"]*"[^>]*>.*?<\/a>/gs;
  return page.route(BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const found = original.match(list);
    if (found) {
      const rows = found[2].match(row) || [];
      reversed = rows.length;
      if (rows.length > 0) {
        await route.fulfill({ response, body: original.replace(list, `$1${rows.reverse().join('')}$3`) });
        return;
      }
    }
    await route.fulfill({ response, body: original });
  }).then(() => () => {
    if (requests < 1) return 'the shelf was never requested, so nothing was reordered';
    if (reversed < 3) return `the shelf carried ${reversed} rows, too few for an order to be turned around`;
    return '';
  });
};

const MUTATIONS = {
  'hide-the-day': { target: ANSWER_SITE, apply: hideTheDay },
  'drop-the-day-below-the-name': { target: LEAD_SITE, apply: dropTheDayBelowTheName },
  'let-the-opening-wrap': { target: ONELINE_SITE, apply: letTheOpeningWrap },
  'widen-the-rows': { target: WIDTH_SITE, apply: widenTheRows },
  'unsort-the-shelf': { target: ORDER_SITE, apply: unsortTheShelf },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`reports-shelf: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`reports-shelf: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reports-shelf: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// readShelf reports what the shelf is, measured against a reference element
// pinned to the viewport rather than against a screenshot or an assumed width.
// The opening is the one element on the page whose own box is deliberately
// narrower than its text — that clipping is what holds it to a line — so it
// answers for its overflow separately and every other element answers to the
// same plain rule.
const readShelf = (page) => page.evaluate(() => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const viewport = ruler.getBoundingClientRect();
  ruler.remove();

  const box = (element) => {
    const rect = element.getBoundingClientRect();
    return { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height };
  };
  const rows = [...document.querySelectorAll('.y-list .y-row')].map((row) => {
    const when = row.querySelector('.y-row__when');
    const title = row.querySelector('.y-row__title');
    const opening = row.querySelector('.y-row__opening');
    return {
      when: when && { text: when.textContent.trim(), box: box(when) },
      title: title && { text: title.textContent.trim(), box: box(title) },
      opening: opening && {
        box: box(opening),
        line: parseFloat(getComputedStyle(opening).lineHeight) || 0,
      },
      kind: row.querySelector('.y-row__measure')?.textContent.trim() ?? '',
    };
  });
  const spilling = [];
  for (const element of document.querySelectorAll('*')) {
    if (element.closest('.y-row__opening')) continue;
    const rect = element.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    if (element.scrollWidth > element.clientWidth + 1) {
      spilling.push({ how: 'scrolls', at: element.className || element.tagName, by: element.scrollWidth - element.clientWidth });
    }
    if (rect.right > viewport.right + 1) {
      spilling.push({ how: 'reaches past the edge', at: element.className || element.tagName, by: rect.right - viewport.right });
    }
  }
  return {
    viewportWidth: viewport.width,
    documentWidth: document.scrollingElement.scrollWidth,
    rows,
    spilling,
  };
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: PHONE });
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  let counted = 0;
  for (const viewport of WIDTHS) {
    await page.setViewportSize(viewport);
    const at = `${viewport.width}px`;
    const shelf = await readShelf(page);
    if (shelf.rows.length < 3) {
      broken(`the fixture shelf holds ${shelf.rows.length} reports, too few to say anything about how they are laid out or ordered`);
    }
    counted = shelf.rows.length;

    const days = shelf.rows.map((row) => row.when?.text ?? '');
    if (days.filter((day) => DAY.test(day)).length < 2) {
      broken(`the fixture shelf carries ${days.filter((day) => DAY.test(day)).length} reports with a day, too few for an order to be read off it`);
    }
    if (days.every((day) => DAY.test(day))) {
      broken('every report in the fixture shelf carries a day, so nothing here shows what a report without one does');
    }
    if (!shelf.rows.some((row) => row.opening)) {
      broken('no report in the fixture shelf lends an opening line, so where it lands cannot be seen');
    }

    // The column a reader scans has to answer on every row. A row left blank
    // there reads as a day the page failed to look up, which is a different
    // claim from the one the shelf is making.
    for (const [i, row] of shelf.rows.entries()) {
      if (!row.when || row.when.text === '') {
        fail(ANSWER_SITE, `row ${i} says nothing about when it is from, at ${at}`);
      }
      if (row.when.box.width === 0 || row.when.box.height === 0) {
        fail(ANSWER_SITE, `row ${i} carries a day that is never drawn, at ${at}`);
      }
    }

    // The day leads: never on a line below the name it belongs to, never to
    // the right of it. At a phone's width it sits on the line above; with room
    // it sits in the column beside, where the two share a baseline and the
    // smaller of the two boxes starts a few pixels lower — which is why this
    // asks whether the day has fallen past the name's line rather than
    // comparing the tops of two boxes set in different sizes.
    for (const [i, row] of shelf.rows.entries()) {
      if (!row.title) broken(`row ${i} has no name at ${at}`);
      if (row.when.box.top >= row.title.box.bottom) {
        fail(LEAD_SITE, `row ${i}'s day starts at y=${row.when.box.top}, below the name that ends at y=${row.title.box.bottom}, so the row does not lead with the day, at ${at}`);
      }
      if (row.when.box.left > row.title.box.left + 1) {
        fail(LEAD_SITE, `row ${i}'s day starts at x=${row.when.box.left} and its name at x=${row.title.box.left}, so the name comes first, at ${at}`);
      }
    }

    // One line of the report's own opening. Not two, and not the paragraph.
    for (const [i, row] of shelf.rows.entries()) {
      if (!row.opening) continue;
      if (row.opening.line === 0) broken(`row ${i}'s opening reports no line height at ${at}`);
      if (row.opening.box.height > row.opening.line * 1.5) {
        fail(ONELINE_SITE, `row ${i}'s opening is ${row.opening.box.height}px over a ${row.opening.line}px line, so it runs to more than one, at ${at}`);
      }
    }

    if (viewport.width === PHONE.width) {
      if (shelf.documentWidth > shelf.viewportWidth + 1) {
        fail(WIDTH_SITE, `the page is ${shelf.documentWidth}px wide in a ${shelf.viewportWidth}px viewport, so it scrolls sideways`);
      }
      if (shelf.spilling.length > 0) {
        const worst = shelf.spilling.sort((a, b) => b.by - a.by)[0];
        fail(WIDTH_SITE, `${shelf.spilling.length} elements spill at ${at}; the worst is ${worst.at}, which ${worst.how} by ${worst.by}px`);
      }
    }

    // Newest first. The days run down the page without going back up, and no
    // dated report is left below one that carries no day at all.
    let previous = '';
    let undated = false;
    for (const [i, day] of days.entries()) {
      if (!DAY.test(day)) {
        undated = true;
        continue;
      }
      if (undated) {
        fail(ORDER_SITE, `row ${i} is from ${day} and sits below a report with no day at all, at ${at}`);
      }
      if (previous !== '' && day > previous) {
        fail(ORDER_SITE, `row ${i} is from ${day}, which is newer than the ${previous} above it, at ${at}`);
      }
      previous = day;
    }
  }

  await page.close();
  console.log(`PASS reports-shelf: ${counted} reports lead with their day, hold their opening to one line, and run newest first, with no sideways scroll at ${PHONE.width}px`);
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
