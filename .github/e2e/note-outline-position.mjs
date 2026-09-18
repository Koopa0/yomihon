// Behavior lock for the outline marking the section the reader is in (#583):
// while scrolling, the on-page contents list marks the heading whose section
// holds the reading line with aria-current="location", one entry at a time,
// in every copy of the list the page carries; scrolling back moves the mark
// again; with no script running, no entry is marked and the plain list of
// links is unchanged; and the reading rail's own current-page mark, sitting
// beside the outline but answering a different question, still announces
// aria-current="page".
//
// None of this is reachable from a Go test: which heading counts as "on
// screen" is a number only a laid-out, scrolled page has, and "no script
// running" is a browser context Go never drives at all.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with headings spaced far enough apart
// to scroll between), and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'exactly-one-entry-marked-at-the-third-section',
  'scrolling-back-moves-the-mark',
  'the-plain-list-survives-with-no-script',
  'the-rails-current-lesson-still-announces-page',
];

// The fixture's headings, in document order, as the contents list itself
// emits them — read off the running server rather than guessed from the
// source file's own heading markers, so a fixture whose H1 does or does not
// enter the list cannot shift what "third" means here.
const FIRST_ID = '第三節-失約的燈';
const THIRD_ID = 'fifth-level-landing';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN note-outline-position: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL note-outline-position: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN note-outline-position: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED note-outline-position: ${message}`);
};

// Rewrites one served page (or module). needle must be found, so a self-test
// that quietly died against rewritten source turns red here rather than
// passing as a regression that walked through.
const rewrite = (match, needle, replacement) => async (page) => {
  let hits = 0;
  await page.route(match, async (route) => {
    const response = await route.fetch();
    const body = await response.text();
    if (!body.includes(needle)) {
      await route.fulfill({ response, body });
      return;
    }
    hits += 1;
    await route.fulfill({ response, body: body.replace(needle, replacement) });
  });
  return async () => (hits === 0 ? `nothing served carried ${JSON.stringify(needle.slice(0, 80))}` : '');
};

const MUTATIONS = {
  // mark() stops clearing every other entry before marking one, so the mark
  // set for the reader's first position never lets go once a second one
  // arrives beside it.
  'mark-two-entries': {
    target: 'exactly-one-entry-marked-at-the-third-section',
    apply: rewrite(
      (url) => url.pathname === '/static/contents.js',
      "      if (active) link.setAttribute('aria-current', 'location');\n      else link.removeAttribute('aria-current');",
      "      if (active) link.setAttribute('aria-current', 'location');",
    ),
  },
  // recompute() freezes the first time the mark actually moves away from the
  // first heading, so the reader's first scroll still lands correctly and
  // every scroll after that — including scrolling back — changes nothing.
  'never-move-the-mark': {
    target: 'scrolling-back-moves-the-mark',
    apply: rewrite(
      (url) => url.pathname === '/static/contents.js',
      `  function recompute() {
    if (locked) return;
    // The document scrolls, so a heading's own viewport coordinate answers
    // directly; measuring against the article box made the comparison move
    // with the page, which is why the mark used to stay on the first entry.
    let current = headings[0].id;
    for (const heading of headings) {
      if (heading.getBoundingClientRect().top <= readingLine) current = heading.id;
      else break;
    }
    mark(current);
  }`,
      `  let frozen = false;
  function recompute() {
    if (locked || frozen) return;
    let current = headings[0].id;
    for (const heading of headings) {
      if (heading.getBoundingClientRect().top <= readingLine) current = heading.id;
      else break;
    }
    if (current !== headings[0].id) frozen = true;
    mark(current);
  }`,
    ),
  },
  // Takes one heading's link out of the page the no-script reader is served,
  // which is the plain list failing to survive rather than merely failing to
  // move.
  'drop-a-link-with-no-script': {
    target: 'the-plain-list-survives-with-no-script',
    at: 'plain',
    apply: rewrite(
      (url) => url.pathname === PAGE,
      '<a href="#sixth-level-landing" data-level="6">Sixth-level landing</a>',
      '',
    ),
  },
  // Takes the reading rail's own aria-current away from the note the reader
  // is already on, so the rail stops saying where they are the moment
  // scripting is not what is being asked about at all.
  'drop-aria-current-on-the-rail': {
    target: 'the-rails-current-lesson-still-announces-page',
    apply: rewrite(
      (url) => url.pathname === PAGE,
      `class="ui-navitem is-active" href="${PAGE}" aria-current="page"`,
      `class="ui-navitem is-active" href="${PAGE}"`,
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`note-outline-position: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`note-outline-position: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`note-outline-position: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What every copy of the on-page contents list says: the links it carries,
// in order, and which of them (if any) is marked as the one being read.
const readOutline = (page) =>
  page.evaluate(() => {
    const lists = [...document.querySelectorAll('.y-toc__list')];
    return lists.map((list) => {
      const links = [...list.querySelectorAll('a[href^="#"]')];
      return {
        hrefs: links.map((a) => a.getAttribute('href')),
        current: links.filter((a) => a.getAttribute('aria-current') === 'location').map((a) => a.getAttribute('href')),
      };
    });
  });

// Jumps straight to where a click on this heading's own entry would land —
// the same clearance recompute() itself measures against — without going
// through a link at all, so the observer's own scroll-tracking is what
// answers rather than the separate, out-of-scope click-travel path.
const scrollToHeading = (page, id) =>
  page.evaluate((headingId) => {
    const heading = document.getElementById(headingId);
    const readingLine = (Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0) + 8;
    const y = heading.getBoundingClientRect().top + window.scrollY - readingLine + 4;
    window.scrollTo(0, Math.max(0, y));
  }, id);

// A scroll's intersection changes are delivered asynchronously; two frames
// past the scroll is past every layout and paint it caused, and the small
// wait after covers a callback queued just behind them.
const settle = (page) =>
  page.evaluate(
    () => new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(resolve, 80)));
    }),
  );

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // Pass 1: scripting on. Scrolls the fixture to its third contents entry,
  // back toward the top, and reads the reading rail's own mark along the way.
  // The third heading sits close enough to the fixture's own end that a
  // taller viewport leaves less document below it than a scroll needs to
  // carry its top up to the reading line — this height is chosen to leave
  // room for that scroll to land, not to match a reader's own window.
  const context = await browser.newContext({ viewport: { width: 1280, height: 700 } });
  const page = await context.newPage();
  const proof = MUTATE && (MUTATIONS[MUTATE].at ?? 'scripted') === 'scripted' ? await MUTATIONS[MUTATE].apply(page) : null;

  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const lists = await page.locator('.y-toc__list').count();
  if (lists < 1) broken(`${PAGE} draws no .y-toc__list, so there is no outline to mark`);
  const headingCount = await page.locator('.y-toc__list').first().locator('a[href^="#"]').count();
  if (headingCount < 3) broken(`${PAGE}'s outline carries ${headingCount} entries, too few to scroll to a third`);

  // recompute() defaults to the first heading whenever nothing has crossed
  // the reading line yet, so a page that has not scrolled marks it already —
  // not a site of its own (mark-two-entries has nothing to duplicate this
  // early), but a prerequisite: a fixture that did not start here would make
  // "scrolling back reaches the first heading" prove nothing below.
  const baseline = await readOutline(page);
  if (baseline.some((list) => list.current.length !== 1 || list.current[0] !== `#${FIRST_ID}`)) {
    broken(`before any scroll, a contents list marks ${JSON.stringify(baseline.map((list) => list.current))}, want exactly ["#${FIRST_ID}"] in each`);
  }

  // The reading rail's own current-page mark — a different attribute value,
  // answering a different question, checked here because nothing in this
  // repo's browser probes watches it from a live page today.
  // null and "absent" are different answers here — the element missing is a
  // broken fixture, and the element present without the attribute is exactly
  // the regression drop-aria-current-on-the-rail injects — so the element's
  // presence is asked for on its own rather than folded into the same null.
  const rail = await page.evaluate((href) => {
    const el = document.querySelector(`a.ui-navitem.is-active[href="${href}"]`);
    return el ? { found: true, current: el.getAttribute('aria-current') } : { found: false, current: null };
  }, PAGE);
  if (!rail.found) broken(`${PAGE} draws no current nav entry for its own address, so there is nothing here to check the reading rail's mark on`);
  if (rail.current !== 'page') {
    fail(
      'the-rails-current-lesson-still-announces-page',
      `the reading rail's current entry announces aria-current=${JSON.stringify(rail.current)}, want "page"`,
    );
  }

  await scrollToHeading(page, THIRD_ID);
  await settle(page);
  const atThird = await readOutline(page);
  for (const list of atThird) {
    if (list.current.length !== 1 || list.current[0] !== `#${THIRD_ID}`) {
      fail(
        'exactly-one-entry-marked-at-the-third-section',
        `after scrolling to #${THIRD_ID}, a contents list marks ${JSON.stringify(list.current)}, want exactly ["#${THIRD_ID}"]`,
      );
    }
  }

  await page.evaluate(() => window.scrollTo(0, 0));
  await settle(page);
  const atTop = await readOutline(page);
  for (const list of atTop) {
    if (list.current.length !== 1 || list.current[0] !== `#${FIRST_ID}`) {
      fail(
        'scrolling-back-moves-the-mark',
        `after scrolling back to the top, a contents list marks ${JSON.stringify(list.current)}, want exactly ["#${FIRST_ID}"], not still ["#${THIRD_ID}"]`,
      );
    }
  }
  await context.close();

  // Pass 2: no scripting at all. The served markup is identical either way,
  // so what changes is entirely the absence of the module that would have
  // marked anything.
  const plainContext = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 900 } });
  const plainPage = await plainContext.newPage();
  const plainProof = MUTATE && (MUTATIONS[MUTATE].at ?? 'scripted') === 'plain' ? await MUTATIONS[MUTATE].apply(plainPage) : null;

  await plainPage.goto(BASE + PAGE, { waitUntil: 'load' });
  if (plainProof) {
    const issue = await plainProof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const plain = await readOutline(plainPage);
  if (plain.length !== baseline.length) {
    broken(`with no script running the page draws ${plain.length} contents lists and ${baseline.length} with scripting, so the two readings are not of the same page`);
  }
  for (let i = 0; i < plain.length; i += 1) {
    if (plain[i].current.length !== 0) {
      fail(
        'the-plain-list-survives-with-no-script',
        `with no script running, a contents list still marks ${JSON.stringify(plain[i].current)}; nothing should mark an entry with no module running to do it`,
      );
    }
    const want = baseline[i].hrefs;
    const got = plain[i].hrefs;
    if (got.length !== want.length || got.some((href, idx) => href !== want[idx])) {
      fail(
        'the-plain-list-survives-with-no-script',
        `with no script running, a contents list carries ${JSON.stringify(got)}, want the same ${JSON.stringify(want)} the scripted reading carries`,
      );
    }
  }
  await plainContext.close();

  console.log(
    `PASS note-outline-position: ${PAGE}'s outline marks #${THIRD_ID} alone after scrolling there and #${FIRST_ID} alone after scrolling back, across ${atThird.length} contents list(s); the rail's own entry announces aria-current="page"; and with no script running none of it marks anything while the ${plain[0]?.hrefs.length ?? 0}-entry list stays intact`,
  );
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
