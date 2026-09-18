// Behavior lock: a note reads inside the phone viewport even when its prose
// carries a run the browser has nowhere to break — an address pasted into a
// sentence, a vault path, the escaped source of a tag the renderer shows as
// words. Such a run is as wide as it is, and until .y-prose said a line may
// break inside one, the reading column took that width and the reader scrolled
// the whole document sideways to read any line of the note, not only the one
// holding the run.
//
// Two things are asserted, because a page can be made to fit by giving the
// reader less. The document fits the viewport, and the run itself is still
// shown whole: every line it lays out on sits inside the reading column, so a
// clipped paragraph or a one-line ellipsis is a failure here rather than a fix.
//
// The served fixture is measured first, so the regression as a reader meets it
// is what goes red. The second measurement pastes an ordinary address into a
// real paragraph, because the defect is not this fixture's: any note with a URL
// in a sentence has the same shape, and a lock that only knew the security
// fixture's escaped markup would go quiet the day that fixture changed.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note whose prose carries an unbreakable run —
// the browser fixture vault's browser-boundary.md does), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/browser-boundary.md';
const MUTATE = process.env.MUTATE || '';
const WIDTHS = [390, 375];
const SITES = ['note-fits-the-phone-viewport', 'the-run-is-shown-whole'];

// The address is grown against the column it has to break inside, rather than
// written down at a length that fits here. The reading face is a token away
// from the reader's own installed fonts, and a run written long enough to need
// two lines on this machine can need one on another — where a paragraph that
// never wrapped would measure exactly like a paragraph that wrapped correctly.
const RUN_HEAD = 'https://example.invalid/';
const RUN_TAIL = ['notes/', 'an-address/', 'with-no-space/', 'anywhere-in-it/'];
const RUN_PREFIX = 'A sentence citing ';
const RUN_SUFFIX = ' and going on afterwards.';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN prose-overflow: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL prose-overflow: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN prose-overflow: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED prose-overflow: ${message}`);
};

// Appending to the product's own stylesheet lands outside the layer it
// declares, so the appended rule outranks the one it stands in for without an
// importance flag. The proof reads the property back off a live element rather
// than trusting that the stylesheet was merely requested: a selector that
// stopped matching would let the route fire and the rule sit unused, which is
// the shape of a self-test that quietly died against a rewritten source.
const overrideProperty = (rule, selector, property, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await page.evaluate(
      ({ selector, property }) => {
        const el = document.querySelector(selector);
        return el ? getComputedStyle(el).getPropertyValue(property) : null;
      },
      { selector, property },
    );
    if (got === null) return `no ${selector} element is on the page to read ${property} from`;
    if (got !== wanted) return `${selector} resolves ${property} to ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

const MUTATIONS = {
  // Takes back the one declaration the reading column needs to break a run
  // inside a word, which is the state every note was read in before it.
  'restore-unbreakable-prose': {
    target: 'note-fits-the-phone-viewport',
    apply: overrideProperty('.y-prose{overflow-wrap:normal}', '.y-prose', 'overflow-wrap', 'normal'),
  },
  // Buys the fit by cutting the sentence instead: the paragraph keeps one line
  // and hides the rest of it. The document then measures exactly as it does
  // when the run wraps, and only the run's own lines say the reader was handed
  // less of the note than the author wrote.
  'clip-the-overflowing-run': {
    target: 'the-run-is-shown-whole',
    apply: overrideProperty(
      '.y-prose p{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}',
      '.y-prose p',
      'white-space',
      'nowrap',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`prose-overflow: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`prose-overflow: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`prose-overflow: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What the page is as it was served: the viewport against the document, the
// reading column against itself, and — with the rule under test switched off
// on this element alone — the width the column's own content would take. That
// last number is the fixture's premise. A note that no longer carries a run
// wider than the column would fit whatever this rule said, and a green run on
// it would mean nothing.
const measureServed = (page) =>
  page.evaluate(() => {
    const prose = document.querySelector('.y-prose');
    if (!prose) return null;
    const unwrapped = (() => {
      const before = prose.style.overflowWrap;
      prose.style.overflowWrap = 'normal';
      const width = prose.scrollWidth;
      prose.style.overflowWrap = before;
      return width;
    })();
    return {
      docScrollWidth: document.documentElement.scrollWidth,
      docClientWidth: document.documentElement.clientWidth,
      proseScrollWidth: prose.scrollWidth,
      proseClientWidth: prose.clientWidth,
      unwrappedWidth: unwrapped,
      paragraphs: prose.querySelectorAll('p').length,
    };
  });

// Pastes an address into a real paragraph of the note, grown until it cannot
// fit the column on one line, and reports where the browser put it. The lines
// come from a range over the address alone, so what is measured is the run the
// reader has to be shown and not the sentence around it.
const measurePasted = (page) =>
  page.evaluate(
    ({ head, tail, prefix, suffix }) => {
      const prose = document.querySelector('.y-prose');
      const paragraph = prose ? prose.querySelector('p') : null;
      if (!paragraph) return null;
      const ruler = document.createElement('span');
      ruler.style.whiteSpace = 'nowrap';
      ruler.style.position = 'absolute';
      ruler.style.visibility = 'hidden';
      paragraph.replaceChildren(ruler);
      let address = head;
      ruler.textContent = address;
      for (let step = 0; step < 200 && ruler.getBoundingClientRect().width <= prose.clientWidth * 1.4; step += 1) {
        address += tail[step % tail.length];
        ruler.textContent = address;
      }
      const unwrappedRun = ruler.getBoundingClientRect().width;
      ruler.remove();
      paragraph.textContent = prefix + address + suffix;

      const node = paragraph.firstChild;
      const range = document.createRange();
      range.setStart(node, prefix.length);
      range.setEnd(node, prefix.length + address.length);
      const rects = [...range.getClientRects()];
      const columnRight = prose.getBoundingClientRect().left + prose.clientLeft + prose.clientWidth;
      return {
        address,
        unwrappedRun,
        lines: rects.length,
        overhang: rects.length === 0 ? null : Math.round(Math.max(...rects.map((rect) => rect.right)) - columnRight),
        docScrollWidth: document.documentElement.scrollWidth,
        docClientWidth: document.documentElement.clientWidth,
        proseClientWidth: prose.clientWidth,
      };
    },
    { head: RUN_HEAD, tail: RUN_TAIL, prefix: RUN_PREFIX, suffix: RUN_SUFFIX },
  );

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const width of WIDTHS) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage();
    const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    // Every width below is a measurement of laid-out text, and text measured
    // while the page still holds a fallback face is a measurement of a face no
    // reader will see.
    await page.evaluate(() => document.fonts.ready);
    if (proof) {
      const issue = await proof();
      if (issue) notApplied(`${MUTATE} at ${width}px: ${issue}`);
    }

    const served = await measureServed(page);
    if (served === null) broken(`the page at ${width}px paints no .y-prose to measure`);
    if (served.proseClientWidth <= 0) broken(`the reading column has no width at ${width}px, so nothing below can be judged`);
    if (served.unwrappedWidth <= served.proseClientWidth) {
      broken(
        `this note's prose is ${served.unwrappedWidth}px wide unbroken against a ${served.proseClientWidth}px column at ${width}px, so it carries no run this lock is about and would fit whatever the rule said`,
      );
    }
    if (served.docScrollWidth > served.docClientWidth) {
      fail(
        'note-fits-the-phone-viewport',
        `as served at ${width}px the document is ${served.docScrollWidth}px wide against a ${served.docClientWidth}px viewport`,
      );
    }
    if (served.proseScrollWidth > served.proseClientWidth) {
      fail(
        'note-fits-the-phone-viewport',
        `as served at ${width}px .y-prose scrollWidth=${served.proseScrollWidth} > clientWidth=${served.proseClientWidth}`,
      );
    }

    if (served.paragraphs === 0) broken(`the note at ${width}px has no paragraph to write an address into`);
    const pasted = await measurePasted(page);
    if (pasted === null) broken(`the note at ${width}px lost its paragraphs before the address was written`);
    if (pasted.unwrappedRun <= pasted.proseClientWidth) {
      broken(
        `the address grown for this column is ${Math.round(pasted.unwrappedRun)}px against a ${pasted.proseClientWidth}px column at ${width}px, so a paragraph that never wrapped would measure the same`,
      );
    }
    if (pasted.docScrollWidth > pasted.docClientWidth) {
      fail(
        'note-fits-the-phone-viewport',
        `with an ordinary address in a sentence at ${width}px the document is ${pasted.docScrollWidth}px wide against a ${pasted.docClientWidth}px viewport`,
      );
    }
    // A line of the address outside the column is the reader being handed less
    // of the sentence than the author wrote, whether it was clipped, hidden or
    // scrolled out of reach. It is asked before the line count, because a
    // paragraph cut back to one line answers both and only this says why.
    if (pasted.overhang === null) {
      fail('the-run-is-shown-whole', `the address written into the note at ${width}px lays out on no line at all`);
    }
    if (pasted.overhang > 0) {
      fail(
        'the-run-is-shown-whole',
        `at ${width}px the address runs ${pasted.overhang}px past the right edge of the reading column on ${pasted.lines} line(s)`,
      );
    }
    if (pasted.lines < 2) {
      broken(
        `the address written into the note at ${width}px lays out on ${pasted.lines} line inside the column, so a run that never broke would measure the same`,
      );
    }
    await context.close();
  }

  console.log(
    `PASS prose-overflow: the note fits the viewport at ${WIDTHS.join('px and ')}px, and an unbreakable address is wrapped whole inside the reading column`,
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
