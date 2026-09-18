// Behavior lock for the notice every surface with nothing to show draws, at
// the width of a phone. The notice prints back what the reader asked for, and
// what a reader asks for is not always something a browser can break a line
// inside: a long identifier pasted into the search field comes back inside the
// sentence naming it, and a run with no space, slash or hyphen in it has
// nowhere to wrap. It went off the side of the screen, so a reader whose
// search matched nothing was handed a page that scrolled sideways to say so.
//
// Go tests cannot see this: the markup is the same either way and only a
// browser lays it out. The probe asks the two surfaces this fixture can reach
// for such a run and measures the box against what it holds.
//
// The four surfaces this vault cannot reach — a folder with nothing left to
// report, two shelves no declaration has filled, and the desk's two notices
// about a reading that came up short — draw the same component with the same
// class, and their markup is pinned by the recordings under
// internal/ui/pages/testdata/render.
//
// Env: YOMIHON_BASE, PAGE_PATH (a search that matches nothing), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

// A run a browser has nowhere to break: no space, no hyphen, no slash and no
// full stop, any of which it would take as a chance to start a new line. It is
// asked for rather than written into the fixture, so the vault the other
// probes share is left as it is.
const UNBREAKABLE = 'qqzzxxwwvvuuttssrrppoonnmmllkkjjiihhggffeeddccbbaa0011223344556677889900aabbccddeeffgghhiijjkkll';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || `/search?q=${UNBREAKABLE}`;
const MUTATE = process.env.MUTATE || '';
const SITES = ['the-notice-fits-the-phone', 'the-page-does-not-scroll-sideways'];

// The address a reader who typed one wrong reaches. It is asked for by this
// probe rather than by the table, because the table names one page and the
// notice answers for more than one.
const MISSING = `/notes/${UNBREAKABLE}`;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN nothing-notice-width: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL nothing-notice-width: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN nothing-notice-width: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED nothing-notice-width: ${message}`);
};

const computedNotice = (page, property) =>
  page.evaluate((name) => {
    const el = document.querySelector('.y-nothing');
    return el ? getComputedStyle(el).getPropertyValue(name) : '';
  }, property);

// Appends a rule outside the served stylesheet so it outranks what it stands
// in for, and reads the resolved value back: a rule that was served and then
// outranked would otherwise pass as a regression nobody noticed.
const appendRule = (rule, property, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await computedNotice(page, property);
    if (got !== wanted) return `the notice resolves ${property} to ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

const MUTATIONS = {
  // Takes back the break of last resort, which is the whole reason a path can
  // be printed in a column narrower than itself.
  'let-a-long-run-refuse-to-break': {
    target: 'the-notice-fits-the-phone',
    apply: appendRule('.y-nothing{overflow-wrap:normal}', 'overflow-wrap', 'normal'),
  },
  // Gives the notice a width the phone does not have. The notice itself still
  // holds its own words; the page is what comes back scrolling sideways.
  'give-the-notice-a-desk-width': {
    target: 'the-page-does-not-scroll-sideways',
    apply: appendRule('.y-nothing{min-width:520px}', 'min-width', '520px'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`nothing-notice-width: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`nothing-notice-width: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`nothing-notice-width: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// laidOut hands back what the browser made of every notice on the page.
const laidOut = (page, run) =>
  page.evaluate((text) => {
    const notices = [...document.querySelectorAll('.y-nothing')];
    const doc = document.documentElement;
    return {
      count: notices.length,
      carrying: notices.filter((el) => el.textContent.includes(text)).length,
      docScroll: doc.scrollWidth - doc.clientWidth,
      boxes: notices.map((el) => ({
        kind: el.dataset.nothing,
        overflowX: el.scrollWidth - el.clientWidth,
        width: Math.round(el.getBoundingClientRect().width),
        widestChild: Math.max(0, ...[...el.querySelectorAll('*')].map((c) => Math.round(c.scrollWidth))),
        breaks: getComputedStyle(el).overflowWrap,
      })),
    };
  }, run);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let checked = 0;
let carried = 0;
try {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  for (const target of [PAGE, MISSING]) {
    await page.goto(BASE + target, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => document.fonts.ready);
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }

    const got = await laidOut(page, UNBREAKABLE);
    if (got.count === 0) broken(`${target} draws no .y-nothing, so nothing on it was measured`);
    checked += got.count;
    carried += got.carrying;

    for (const box of got.boxes) {
      if (box.width <= 0) broken(`${target}: .y-nothing[${box.kind}] has no width, so overflow cannot be judged`);
      if (box.overflowX > 0) {
        fail('the-notice-fits-the-phone', `at 390px ${target} .y-nothing[${box.kind}] holds ${box.overflowX}px more than its ${box.width}px box shows`);
      }
      if (box.widestChild > box.width) {
        fail('the-notice-fits-the-phone', `at 390px ${target} .y-nothing[${box.kind}] lays a ${box.widestChild}px line out in a ${box.width}px box`);
      }
    }
    if (got.docScroll > 0) {
      fail('the-page-does-not-scroll-sideways', `at 390px ${target} scrolls ${got.docScroll}px sideways`);
    }
  }
  if (checked < 2) broken(`only ${checked} notices were measured across both addresses, so a page that stopped drawing one would read the same`);
  // The run has to reach a notice for any of this to be about long runs. The
  // search page prints the query back inside its heading; if it stopped, every
  // measurement above would be of two lines of ordinary prose, which fit at any
  // width and would go on fitting however the notice was broken.
  if (carried === 0) broken('no notice carried the long run, so the boxes measured above were never asked to hold one');

  console.log(`PASS nothing-notice-width: ${checked} notices hold a ${UNBREAKABLE.length}-character unbroken run inside a 390px column, and neither page scrolls sideways`);
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
