// Behaviour lock for the journal read as a month. Three claims a rendering
// test cannot reach, because all three are settled by the browser rather than
// by the bytes — that the seven columns really do shrink to a phone instead of
// asking it to scroll sideways, that the days a reader sees run in order
// across each week and down the month, and that the entries which wrote no day
// are drawn where a reader can still reach them.
//
// A day is recognised by the square's own machine-readable date, never by the
// words beside it, so this says the same thing whichever language the
// interface is speaking.
//
// Env: YOMIHON_BASE, PAGE_PATH (the journal at a named month), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/journal?month=2026-07';
const MUTATE = process.env.MUTATE || '';
const ORDER_SITE = 'the-days-run-in-order';
const UNDATED_SITE = 'the-dateless-are-reachable';
const WIDTH_SITE = 'no-sideways-scroll';
const SITES = [ORDER_SITE, UNDATED_SITE, WIDTH_SITE];

// The two widths the month is read at: a phone, which is where seven columns
// have to give, and a laptop, which is where they have room.
const PHONE = { width: 390, height: 844 };
const DESK = { width: 1280, height: 900 };
const WIDTHS = [PHONE, DESK];

const COLUMNS = 7;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => { throw new LockFired(site, `FAIL journal-month: ${message}`); };
const broken = (message) => { throw new ProbeBroken(`BROKEN journal-month: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED journal-month: ${message}`); };

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

// letTheGridScroll takes the shared-width layout off the table, so the columns
// are sized by what is written in them and the month reaches past a phone —
// which is what the calendar does the moment nothing holds it to the page.
const letTheGridScroll = (page) => appendStyle(page, '.y-month{table-layout:auto;width:auto;min-width:760px}.y-month__entry{overflow:visible;white-space:nowrap}');

// hideTheDatelessGroup stops the entries that wrote no day being drawn, which
// is what a page that quietly dropped them looks like from here: they belong to
// no month, so no month would ever show them and nothing else would say so.
const hideTheDatelessGroup = (page) => appendStyle(page, '[data-journal-undated]{display:none}');

// scrambleTheDays serves the same month with its weeks turned around, which is
// the page a calendar that had stopped laying the month out in order returns.
const scrambleTheDays = (page) => {
  let requests = 0;
  let turned = 0;
  const body = /(<tbody>)(.*?)(<\/tbody>)/s;
  const week = /<tr>.*?<\/tr>/gs;
  return page.route((url) => url.href === BASE + PAGE, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const found = original.match(body);
    if (found) {
      const weeks = found[2].match(week) || [];
      turned = weeks.length;
      if (weeks.length > 1) {
        await route.fulfill({ response, body: original.replace(body, `$1${weeks.reverse().join('')}$3`) });
        return;
      }
    }
    await route.fulfill({ response, body: original });
  }).then(() => () => {
    if (requests < 1) return 'the month was never requested, so nothing was reordered';
    if (turned < 2) return `the month drew ${turned} weeks, too few for an order to be turned around`;
    return '';
  });
};

const MUTATIONS = {
  'scramble-the-days': { target: ORDER_SITE, apply: scrambleTheDays },
  'hide-the-dateless-group': { target: UNDATED_SITE, apply: hideTheDatelessGroup },
  'let-the-grid-scroll': { target: WIDTH_SITE, apply: letTheGridScroll },
};

for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) {
    console.error(`journal-month: mutation ${name} aims at unknown site ${mode.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) {
    console.error(`journal-month: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`journal-month: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// readMonth reports what the calendar is, measured against a reference element
// pinned to the viewport rather than against a screenshot or an assumed width.
// An entry's name is the one thing on the page whose own box is deliberately
// narrower than its text — that clipping is what keeps a square a square — so
// it answers for its overflow separately and everything else answers to the
// same plain rule.
const readMonth = (page) => page.evaluate(() => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const viewport = ruler.getBoundingClientRect();
  ruler.remove();

  const weeks = [...document.querySelectorAll('.y-month tbody tr')].map((row) => ({
    columns: row.children.length,
    days: [...row.children].map((cell) => cell.getAttribute('data-journal-day') ?? ''),
  }));
  const spilling = [];
  for (const element of document.querySelectorAll('*')) {
    if (element.closest('.y-month__entry') || element.closest('.y-row__opening')) continue;
    const rect = element.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    if (element.scrollWidth > element.clientWidth + 1) {
      spilling.push({ how: 'scrolls', at: element.className || element.tagName, by: element.scrollWidth - element.clientWidth });
    }
    if (rect.right > viewport.right + 1) {
      spilling.push({ how: 'reaches past the edge', at: element.className || element.tagName, by: rect.right - viewport.right });
    }
  }
  const undated = document.querySelector('[data-journal-undated]');
  return {
    viewportWidth: viewport.width,
    documentWidth: document.scrollingElement.scrollWidth,
    spilling,
    headings: document.querySelectorAll('.y-month__weekday').length,
    weeks,
    entries: [...document.querySelectorAll('[data-journal-entry]')].map((link) => ({
      day: link.closest('[data-journal-day]')?.getAttribute('data-journal-day') ?? '',
      href: link.getAttribute('href'),
    })),
    undated: undated && {
      rows: undated.querySelectorAll('[data-index-row]').length,
      box: (({ width, height }) => ({ width, height }))(undated.getBoundingClientRect()),
    },
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
    const month = await readMonth(page);

    if (month.headings !== COLUMNS) {
      broken(`the month draws ${month.headings} column headings, so it is not a week wide`);
    }
    if (month.weeks.length < 4) {
      broken(`the month draws ${month.weeks.length} weeks, too few to be a month`);
    }
    if (month.entries.length < 3) {
      broken(`the fixture month holds ${month.entries.length} entries, too few to say anything about where they land`);
    }
    if (!month.entries.some((entry, i) => month.entries.findIndex((other) => other.day === entry.day) !== i)) {
      broken('no two entries in the fixture month share a day, so a square holding more than one is never drawn');
    }
    counted = month.entries.length;

    // Every week is a week wide, the days run left to right and on down the
    // page without repeating or going back, and the squares that carry no day
    // are only ever at the two ends of the month.
    let previous = '';
    let started = false;
    let ended = false;
    for (const [w, week] of month.weeks.entries()) {
      if (week.columns !== COLUMNS) {
        fail(ORDER_SITE, `week ${w} has ${week.columns} squares, want ${COLUMNS}, at ${at}`);
      }
      for (const [column, day] of week.days.entries()) {
        if (day === '') {
          if (started) ended = true;
          continue;
        }
        if (ended) {
          fail(ORDER_SITE, `${day} sits after a square with no day, at ${at}`);
        }
        started = true;
        if (previous !== '' && day <= previous) {
          fail(ORDER_SITE, `square ${column} of week ${w} is ${day}, which is not after the ${previous} before it, at ${at}`);
        }
        previous = day;
      }
    }

    // Every entry is drawn inside the square of its own day. A link that had
    // come loose from its square would still be on the page and still lead
    // somewhere, which is exactly why nothing else here would notice.
    for (const entry of month.entries) {
      if (entry.day === '') {
        fail(ORDER_SITE, `the entry at ${entry.href} is drawn in no day's square, at ${at}`);
      }
    }

    // The entries that wrote no day belong to no month, so this group is the
    // only place any month shows them.
    if (!month.undated || month.undated.rows === 0) {
      fail(UNDATED_SITE, `the entries that wrote no day are listed nowhere on this month, at ${at}`);
    }
    if (month.undated.box.width === 0 || month.undated.box.height === 0) {
      fail(UNDATED_SITE, `the entries that wrote no day are in the markup and never drawn, at ${at}`);
    }

    if (viewport.width === PHONE.width) {
      if (month.documentWidth > month.viewportWidth + 1) {
        fail(WIDTH_SITE, `the page is ${month.documentWidth}px wide in a ${month.viewportWidth}px viewport, so it scrolls sideways`);
      }
      if (month.spilling.length > 0) {
        const worst = month.spilling.sort((a, b) => b.by - a.by)[0];
        fail(WIDTH_SITE, `${month.spilling.length} elements spill at ${at}; the worst is ${worst.at}, which ${worst.how} by ${worst.by}px`);
      }
    }
  }

  await page.close();
  console.log(`PASS journal-month: ${counted} entries sit in the squares of their own days, the month runs in order, and seven columns fit ${PHONE.width}px`);
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
