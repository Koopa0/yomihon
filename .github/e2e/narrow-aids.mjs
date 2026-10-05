// Behavior lock for the reading aids where the right rail is hidden. Four
// folds stacked between the title and the first sentence put the text a third
// of a screen down on every note that had them, so the aids are one closed row
// on a forty-pixel pitch, the text starts right under it, and the two aids a
// reader reaches for once finished — the note to read beside this one and the
// notes citing it — stand after the text. Where the rail is drawn it carries
// all of them, and neither the row nor the blocks after the text are drawn a
// second time beside it.
//
// Go tests hold the markup; only a browser can say what is drawn at which
// width, how tall the row is, and where the text lands.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with contents, a pair and citations),
// and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const NARROW = 1024;
const WIDE = 1280;
const PITCH = 40;
// The text follows the row by the head's own gap above the prose and nothing
// else; a second fold, or one drawn open, adds a row's worth at least.
const GAP_AFTER_ROW = 40;
const SITES = [
  'one-closed-row',
  'forty-pixel-pitch',
  'text-follows-the-row',
  'notes-beside-follow-the-text',
  'wide-shows-the-rail-instead',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN narrow-aids: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL narrow-aids: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN narrow-aids: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED narrow-aids: ${message}`); };

// Rewrites the note's own document, once per page that asks for it.
const rewriteDocument = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(`${BASE}${PAGE}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (requests < 1) return 'the note was never requested';
    if (matches !== requests) return `the needle matched ${matches} times over ${requests} requests, want exactly 1 each`;
    return '';
  };
};

// Appends a rule to the product's own stylesheet, after everything it would
// otherwise lose to by source order.
const appendStyle = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

const MUTATIONS = {
  // The row drawn open, so the contents stand between the title and the text.
  'draw-the-row-open': {
    target: 'one-closed-row',
    apply: rewriteDocument('<details class="y-toc-inline">', '<details class="y-toc-inline" open>'),
  },
  // The summary given back the room it had when it was one fold of four.
  'pad-the-row': {
    target: 'forty-pixel-pitch',
    apply: appendStyle('.y-toc-inline__summary{padding-block:8px}'),
  },
  // The aids drawn whole above the text with the row still closed.
  'unfold-the-aids-above-the-text': {
    target: 'text-follows-the-row',
    apply: appendStyle('.y-toc-inline::details-content{content-visibility:visible;height:auto}'),
  },
  // The notes beside this one left out where the rail is not there to hold them.
  'leave-the-notes-beside-out': {
    target: 'notes-beside-follow-the-text',
    apply: appendStyle('.y-related{display:none}'),
  },
  // The same blocks drawn after the text beside the rail that already holds them.
  'draw-the-notes-beside-twice': {
    target: 'wide-shows-the-rail-instead',
    apply: appendStyle('.y-related{display:flex}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`narrow-aids: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`narrow-aids: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`narrow-aids: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const shape = (page) => page.evaluate(() => {
  const box = (element) => {
    if (!element) return null;
    const rect = element.getBoundingClientRect();
    return { top: rect.top + window.scrollY, bottom: rect.bottom + window.scrollY, height: rect.height, display: getComputedStyle(element).display };
  };
  const row = document.querySelector('.y-article > .y-inlineaids');
  const prose = document.querySelector('.y-article > .y-prose');
  const related = document.querySelector('.y-article > .y-related');
  return {
    row: box(row),
    folds: row ? row.querySelectorAll('details').length : 0,
    open: row ? row.querySelectorAll('details[open]').length : 0,
    summary: box(row?.querySelector('summary')),
    text: box(prose?.firstElementChild),
    prose: box(prose),
    related: box(related),
    pair: box(related?.querySelector('.y-pair')),
    cited: box(related?.querySelector('.y-citedby')),
    steps: box(document.querySelector('.y-steps')),
    rail: box(document.querySelector('.y-rail-right')),
  };
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const open = async (width) => {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await page.evaluate(() => document.fonts.ready);
    return { page, proof };
  };

  {
    const { page, proof } = await open(NARROW);
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const got = await shape(page);
    if (!got.row || got.row.display === 'none') broken(`at ${NARROW}px the note draws no row of aids, so there is nothing to measure`);
    if (!got.text) broken('the note has no text to land on');
    if (!got.related || !got.steps) broken('the fixture note has no notes beside it or no way onward, so the end of the text proves nothing');

    if (got.folds !== 1 || got.open !== 0) {
      fail('one-closed-row', `at ${NARROW}px the aids above the text are ${got.folds} folds with ${got.open} open, want one closed row`);
    }
    if (Math.round(got.summary.height) !== PITCH) {
      fail('forty-pixel-pitch', `at ${NARROW}px the row's summary is ${got.summary.height}px, want ${PITCH}px on one line`);
    }
    // Measured from the summary, the one line a closed row is: whatever else
    // is drawn between it and the text is what this is here to notice.
    const gap = got.text.top - got.summary.bottom;
    if (Math.abs(gap - GAP_AFTER_ROW) > 1) {
      fail('text-follows-the-row', `at ${NARROW}px the text starts ${gap}px under the row of aids, want ${GAP_AFTER_ROW}`);
    }
    if (got.related.display === 'none' || !got.pair || !got.cited || got.pair.height === 0 || got.cited.height === 0) {
      fail('notes-beside-follow-the-text', `at ${NARROW}px the notes beside this one are not drawn after the text: ${JSON.stringify({ related: got.related, pair: got.pair, cited: got.cited })}`);
    }
    if (!(got.related.top >= got.prose.bottom && got.related.bottom <= got.steps.top)) {
      fail('notes-beside-follow-the-text', `at ${NARROW}px the notes beside this one are not between the text and the way onward: ${JSON.stringify({ prose: got.prose, related: got.related, steps: got.steps })}`);
    }
    await page.close();
  }

  {
    const { page, proof } = await open(WIDE);
    if (proof) {
      const issue = proof();
      if (issue) notApplied(`${MUTATE}: ${issue}`);
    }
    const got = await shape(page);
    if (!got.rail || got.rail.display === 'none') broken(`at ${WIDE}px the right rail is not drawn, so it holds nothing the row would repeat`);
    if (got.row && got.row.display !== 'none') {
      fail('wide-shows-the-rail-instead', `at ${WIDE}px the row of aids is drawn beside the rail that holds the same aids`);
    }
    if (got.related && got.related.display !== 'none') {
      fail('wide-shows-the-rail-instead', `at ${WIDE}px the notes beside this one are drawn after the text and in the rail`);
    }
    await page.close();
  }

  console.log('PASS narrow-aids: below the width of the rail the aids are one closed forty-pixel row, the text starts under it, and the notes beside this one follow the text; with the rail drawn, neither is');
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
