// Behavior lock: a link written at a section of another note lands the reader
// on that section, and a link written at a block address lands them on the
// marked line, clear of the sticky header.
//
// The regression class is quiet and complete: the words on screen still say
// "note#section", the link still works, and the reader still arrives — at the
// top of a note that runs for pages, with nothing to say which part they were
// promised. Nothing is broken enough to report, which is why this is checked in
// a browser and against the destination's own anchor rather than against a
// string written down twice: the fragment and the heading id are produced by
// two different passes over two different documents, and the only property
// worth locking is that they agree. A block address is the same quiet miss in
// a different place: the URL and the id are right, and the marked line sits
// under the header.
//
// The destination note is long on purpose. On a note that fits in one screen,
// "the reader arrived at the section" is already true before anything scrolls,
// so the arrival check could not have failed. The block fixture wraps to three
// lines or more for the same reason: a one-line paragraph would sit entirely
// below the header once the caret did, and could not tell a landing on the
// marked line from a landing on the paragraph.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (the note that
// carries the links). MUTATE names one of the self-test modes below;
// MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const DESTINATION = '/notes/Notes/Glass%20Tide.md';
const OPENING = '/notes/Notes/seedling-inbox.md';

const MUTATE = process.env.MUTATE || '';

// The two links under test. One is written with no display text, so its label
// still shows the section name, and its section is named in CJK with
// punctuation in it — the case a slug rule is most likely to reduce to
// nothing. The other carries the reader's own words, which must survive
// alongside the address they hide.
// The third names the heading the destination removed as a duplicate of its
// own title. Nothing in that note's body answers to it any more, so the only
// thing that can is the title the reader is looking at.
const LINKS = [
  { label: 'Glass Tide#第三節：失約的燈', heading: '第三節：失約的燈' },
  { label: 'back to the material', heading: 'Sensory material' },
  { label: 'Glass Tide#Fourth-level landing', heading: 'Fourth-level landing' },
  { label: 'Glass Tide#Glass Tide', heading: 'Glass Tide' },
];

const SITES = [
  'composed-ids-unique',
  'title-carries-its-anchor',
  'fragment-names-the-anchor',
  'fragment-reaches-the-heading',
  'back-returns-to-the-source',
  'opening-heading-sits-flush',
  'block-clears-the-header',
];

const BLOCK_LINKS = [
  { label: 'back to the marked line', id: '^tide-wrap-1', host: 'p', minLines: 3 },
  { label: 'back to the marked item', id: '^tide-item-1', host: 'li' },
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN heading-fragment: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL heading-fragment: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN heading-fragment: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED heading-fragment: ${message}`); };

// A document mutation answers for the page it was installed on, never for the
// run as a whole: the flow loads the destination before the source, and a
// rewrite that only reached the first of them would otherwise report itself
// applied to the page whose links the assertions actually read. The rewrite is
// replaceAll, so a document that changed is a document wholly changed —
// rewriting the first link and leaving the second would make a page that is
// half correct look like a probe that caught something.
const rewriteDocuments = (transform) => async (context) => {
  const rewritten = new Set();
  await context.route('**/notes/**', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = transform(original);
    if (body !== original) rewritten.add(new URL(route.request().url()).pathname);
    await route.fulfill({ response, body });
  });
  return (pathname) => (rewritten.has(pathname) ? '' : `nothing was rewritten in ${pathname}`);
};

// Appends to the product's own stylesheet, landing outside the layer it
// declares, so the rule outranks what it stands in for without an importance
// flag.
const weakenStylesheet = (rule) => async (context) => {
  const seen = new Set();
  await context.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen.add('app.css');
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  // A stylesheet reaches every page in the context at once, so unlike a
  // document rewrite it has no per-page answer to give.
  return () => (seen.has('app.css') ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

const MUTATIONS = {
  // The defect itself: the renderer stops writing the fragment and every
  // section link becomes a link to the top of a note.
  'drop-the-fragment': {
    target: 'fragment-names-the-anchor',
    apply: rewriteDocuments((body) => body.replaceAll(/(href="\/notes\/[^"]*?)#[^"]*"/g, '$1"')),
  },
  // A second slug rule, agreeing with the first almost everywhere: the link
  // still names a section, just not one that is on the page.
  'misspell-the-fragment': {
    target: 'fragment-names-the-anchor',
    apply: rewriteDocuments((body) => body.replaceAll(/(href="\/notes\/[^"]*?#)([^"]*)"/g, '$1$2-elsewhere"')),
  },
  // The address is right and the reader still cannot see the section: a
  // scroll offset large enough to leave the jump with nowhere to go.
  // Scroll-margin follows the authored data-level, including a #### that the
  // shell writes as h5. Burying only h2/h3 walks past that heading.
  'bury-the-target': {
    target: 'fragment-reaches-the-heading',
    apply: weakenStylesheet('.y-prose [data-level]{scroll-margin-top:4000px}'),
  },
  // Following the link stops being a step this tab took, so there is nothing
  // for the browser's own back button to undo.
  'open-the-link-elsewhere': {
    target: 'back-returns-to-the-source',
    apply: rewriteDocuments((body) => body.replaceAll('class="wikilink"', 'class="wikilink" target="_blank"')),
  },
  // The size rows share specificity with `.y-prose > :first-child` and sit
  // later, so an opening heading used to keep its top margin. Restoring that
  // margin is the defect; the reset has to win or 179 vault notes drop.
  'give-the-opening-heading-its-margin-back': {
    target: 'opening-heading-sits-flush',
    provePath: () => openingPath,
    apply: weakenStylesheet('.y-prose > [data-level]:first-child{margin-top:48px}'),
  },
  // Every separately rendered body numbers its footnotes from one, and on this
  // page the second such body is the note the source quotes: its footnotes are
  // named under a region of their own. Taking that region's name off them puts
  // the same id on the page twice and a citation lands on whichever came first.
  'collapse-footnote-regions': {
    target: 'composed-ids-unique',
    provePath: () => sourcePath,
    apply: rewriteDocuments((body) => body.replaceAll('y1-', '')),
  },
  // The title stops answering to the section it inherited, which is what the
  // page looked like before the anchor was carried across.
  'strip-title-anchor': {
    target: 'title-carries-its-anchor',
    provePath: () => destinationPath,
    apply: rewriteDocuments((body) => body.replaceAll('<h1 id="', '<h1 data-was-id="')),
  },
  // The address is right and the marked line still sits under the header: the
  // clearance the product owes the caret is taken off, which is the defect
  // this site exists to catch.
  'pin-the-block-to-the-top': {
    target: 'block-clears-the-header',
    apply: weakenStylesheet('.y-prose [id^="^"]{scroll-margin-top:0}'),
  },
  // The margin string stays at the token while the painted bar grows, so
  // the landing assertion — not the property check — is what has to fire.
  'stretch-the-header': {
    target: 'block-clears-the-header',
    apply: weakenStylesheet('.y-header{height:200px}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`heading-fragment: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`heading-fragment: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`heading-fragment: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
const sourcePath = new URL(BASE + PAGE).pathname;
const destinationPath = new URL(BASE + DESTINATION).pathname;
const openingPath = new URL(BASE + OPENING).pathname;

// Only the flow whose assertion a mode aims at proves it applied. Asking on
// any other page would report not-applied for a mutation working perfectly on
// the one it was written for.
const proveApplied = (site, proof) => {
  if (!mutation || mutation.target !== site) return;
  const issue = proof(mutation.provePath ? mutation.provePath() : sourcePath);
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

// Reads every id the page carries and every same-page address it offers, so
// the two can be checked against each other rather than against a list of the
// ids this file expects to find.
const pageAddresses = (page) => page.evaluate(() => ({
  ids: [...document.querySelectorAll('[id]')].map((el) => el.id),
  fragments: [...document.querySelectorAll('a[href^="#"]')].map((a) => decodeURIComponent(a.getAttribute('href').slice(1))),
  footnoteRefs: [...document.querySelectorAll('.footnote-ref')].length,
  footnoteSections: [...document.querySelectorAll('.footnotes')].length,
}));

// A bare double rAF never resolves while a cross-document view transition
// has the arriving document's rendering suspended. Arm before the click so
// pagereveal can hand us the transition's finished promise; race that and
// the frames against a timer so the lock finishes under normal motion.
const armArrival = (page) => page.addInitScript(() => {
  window.__yArrival = new Promise((resolve) => {
    let settled = false;
    const finish = () => {
      if (settled) return;
      settled = true;
      resolve();
    };
    setTimeout(finish, 500);
    const afterPaint = () => requestAnimationFrame(() => requestAnimationFrame(finish));
    window.addEventListener('pagereveal', (event) => {
      if (event.viewTransition && event.viewTransition.finished) {
        event.viewTransition.finished.then(afterPaint, afterPaint);
        return;
      }
      afterPaint();
    }, { once: true });
  });
});

const waitArrival = (page) => page.evaluate(() => {
  if (window.__yArrival) return window.__yArrival;
  return new Promise((resolve) => {
    setTimeout(resolve, 500);
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  });
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const proof = mutation ? await mutation.apply(context) : null;

  // A page assembled out of several bodies has to name every place on it
  // exactly once. The source note carries its own footnotes and quotes a note
  // carrying another, and those two are rendered separately and spliced
  // together. A callout is not one of those bodies: it is the note's own text,
  // read by the note's own parse, and its footnotes join the note's own list.
  {
    const page = await context.newPage();
    const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (!response || response.status() !== 200) broken(`the source note returned ${response?.status() ?? 'no response'}, want 200`);
    proveApplied('composed-ids-unique', proof);

    const { ids, fragments, footnoteRefs, footnoteSections } = await pageAddresses(page);
    // Without this the checks below would hold over a page that assembled
    // nothing and therefore had nothing to collide.
    if (footnoteSections < 2 || footnoteRefs < 3) {
      broken(`the source note rendered ${footnoteSections} footnote sections and ${footnoteRefs} references, want at least 2 and 3 — the composed fixture is not on this page`);
    }
    const count = new Map();
    for (const id of ids) count.set(id, (count.get(id) ?? 0) + 1);
    for (const [id, n] of count) {
      if (n > 1) fail('composed-ids-unique', `the id ${JSON.stringify(id)} is on this page ${n} times, so a fragment naming it reaches whichever came first`);
    }
    for (const fragment of fragments) {
      const n = count.get(fragment) ?? 0;
      if (n !== 1) fail('composed-ids-unique', `the address ${JSON.stringify(`#${fragment}`)} has ${n} destinations on this page, want exactly 1`);
    }
    await page.close();
  }

  // What the destination itself calls each section. Everything below compares
  // against these, never against a slug this file worked out on its own. The
  // title is read alongside the prose headings because a note that opened with
  // its own name has no body heading left to answer for it.
  const reader = await context.newPage();
  const response = await reader.goto(BASE + DESTINATION, { waitUntil: 'networkidle' });
  if (!response || response.status() !== 200) broken(`the destination returned ${response?.status() ?? 'no response'}, want 200`);
  proveApplied('title-carries-its-anchor', proof);

  const anchors = await reader.evaluate(() => Object.fromEntries(
    [...document.querySelectorAll('h1.y-title, .y-prose :is(h2,h3,h4,h5,h6)')].map((h) => [h.textContent.trim(), h.id]),
  ));
  const fourth = await reader.evaluate(() => {
    const heading = [...document.querySelectorAll('.y-prose :is(h2,h3,h4,h5,h6)')]
      .find((h) => h.textContent.trim() === 'Fourth-level landing');
    if (!heading) return null;
    const style = getComputedStyle(heading);
    const headerHeight = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--header-height'));
    return {
      tag: heading.tagName,
      level: heading.getAttribute('data-level'),
      scrollMarginTop: style.scrollMarginTop,
      clearance: headerHeight + 16,
    };
  });
  if (!fourth) {
    broken('the destination page has no heading named "Fourth-level landing"');
  }
  if (fourth.tag !== 'H5' || fourth.level !== '4') {
    fail('fragment-reaches-the-heading', `the #### heading rendered as ${fourth.tag} data-level=${JSON.stringify(fourth.level)}, want H5 data-level="4"`);
  }
  if (parseFloat(fourth.scrollMarginTop) !== fourth.clearance) {
    fail('fragment-reaches-the-heading', `the demoted h5 scroll-margin-top is ${JSON.stringify(fourth.scrollMarginTop)}, want ${fourth.clearance}px from --header-height so the jump clears the sticky header`);
  }
  const titleText = await reader.evaluate(() => document.querySelector('h1.y-title').textContent.trim());
  if (!anchors[titleText]) {
    fail('title-carries-its-anchor', `the destination's visible title ${JSON.stringify(titleText)} carries no id, so the section it absorbed can be named by a link and reached by nobody`);
  }
  for (const link of LINKS) {
    if (!anchors[link.heading]) broken(`the destination page has nothing named ${JSON.stringify(link.heading)}; it offers ${JSON.stringify(Object.keys(anchors))}`);
  }
  await reader.close();

  // A note whose body opens with a heading used to pick up that heading's top
  // margin once look keyed on data-level: the generic first-child reset tied
  // the size rows and lost. The heading-only reset has to win, including in
  // print, which inherits the same margin.
  {
    const page = await context.newPage();
    const response = await page.goto(BASE + OPENING, { waitUntil: 'networkidle' });
    if (!response || response.status() !== 200) broken(`the opening-heading note returned ${response?.status() ?? 'no response'}, want 200`);
    proveApplied('opening-heading-sits-flush', proof);

    const opening = await page.evaluate(() => {
      const first = document.querySelector('.y-prose > :first-child');
      if (!first) return null;
      const style = getComputedStyle(first);
      return {
        tag: first.tagName,
        level: first.getAttribute('data-level'),
        isHeading: /^H[1-6]$/.test(first.tagName),
        marginTop: style.marginTop,
      };
    });
    if (!opening || !opening.isHeading) {
      broken('the opening-heading fixture does not start .y-prose with a heading');
    }
    if (opening.marginTop !== '0px') {
      fail('opening-heading-sits-flush', `the opening ${opening.tag} data-level=${JSON.stringify(opening.level)} has margin-top ${JSON.stringify(opening.marginTop)}, want "0px"`);
    }
    await page.close();
  }

  for (const link of LINKS) {
    const wanted = anchors[link.heading];

    // A page of its own per link. A cross-document view transition leaves the
    // previous page's animation state behind, and driving a second click
    // through it measures the automation rather than the product.
    const page = await context.newPage();
    const source = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (!source || source.status() !== 200) broken(`the source note returned ${source?.status() ?? 'no response'}, want 200`);

    proveApplied('fragment-names-the-anchor', proof);
    proveApplied('fragment-reaches-the-heading', proof);
    proveApplied('back-returns-to-the-source', proof);

    const anchor = page.locator(`main a.wikilink:text-is("${link.label}")`);
    const found = await anchor.count();
    if (found !== 1) broken(`the source note carries ${found} links labelled ${JSON.stringify(link.label)}, want exactly 1`);

    const href = await anchor.getAttribute('href');
    const fragment = decodeURIComponent(new URL(href, BASE).hash.slice(1));
    if (fragment === '') {
      fail('fragment-names-the-anchor', `the link labelled ${JSON.stringify(link.label)} points at ${href} with no fragment, so the reader is sent to the top of the note instead of to ${JSON.stringify(link.heading)}`);
    }
    if (fragment !== wanted) {
      fail('fragment-names-the-anchor', `the link labelled ${JSON.stringify(link.label)} names the fragment ${JSON.stringify(fragment)}, but the destination stamps that heading ${JSON.stringify(wanted)}`);
    }

    await anchor.click();
    const moved = await page.waitForURL((url) => url.pathname.endsWith('Tide.md'), { timeout: 10_000 })
      .then(() => true, () => false);
    if (!moved) {
      fail('back-returns-to-the-source', `following ${JSON.stringify(link.label)} did not move this tab (it is still at ${page.url()}), so the browser has no step to go back from`);
    }

    const arrival = await page.evaluate(() => {
      const target = document.querySelector(':target');
      return {
        hash: decodeURIComponent(location.hash.slice(1)),
        id: target ? target.id : null,
        text: target ? target.textContent.trim() : null,
        // Where the section sits once the jump has settled. Reading :target
        // alone would pass on a page that matched the selector and never moved.
        top: target ? target.getBoundingClientRect().top : null,
        viewport: window.innerHeight,
      };
    });
    if (arrival.hash !== wanted) {
      fail('fragment-reaches-the-heading', `after following ${JSON.stringify(link.label)} the address bar reads ${JSON.stringify(arrival.hash)}, want ${JSON.stringify(wanted)}`);
    }
    if (arrival.id !== wanted || arrival.text !== link.heading) {
      fail('fragment-reaches-the-heading', `after following ${JSON.stringify(link.label)} the targeted element is ${JSON.stringify(arrival.id)} (${JSON.stringify(arrival.text)}), want the heading ${JSON.stringify(wanted)}`);
    }
    if (!(arrival.top >= 0 && arrival.top < arrival.viewport)) {
      fail('fragment-reaches-the-heading', `after following ${JSON.stringify(link.label)} the heading sits ${arrival.top}px down a ${arrival.viewport}px viewport, so the page never travelled to it`);
    }

    await page.goBack({ waitUntil: 'networkidle' });
    const returned = new URL(page.url()).pathname;
    if (returned !== sourcePath) {
      fail('back-returns-to-the-source', `going back from the section landed on ${returned}, want ${sourcePath}`);
    }
    await page.close();
  }

  // A block address is a trailing span, not the paragraph. The jump has to
  // put that marked line below the sticky header; earlier wrapping lines may
  // still sit under the fold. A second address lives on a list item so a
  // selector that only names p cannot answer for every caret. Other mutations
  // rewrite heading links and would fire here first, so this site only runs
  // for its own mode or the plain lock.
  if (!mutation || mutation.target === 'block-clears-the-header') {
    for (const link of BLOCK_LINKS) {
      const page = await context.newPage();
      await armArrival(page);
      const source = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
      if (!source || source.status() !== 200) broken(`the source note returned ${source?.status() ?? 'no response'}, want 200`);

      proveApplied('block-clears-the-header', proof);

      const anchor = page.locator(`main a.wikilink:text-is("${link.label}")`);
      const found = await anchor.count();
      if (found !== 1) broken(`the source note carries ${found} links labelled ${JSON.stringify(link.label)}, want exactly 1`);

      const href = await anchor.getAttribute('href');
      const fragment = decodeURIComponent(new URL(href, BASE).hash.slice(1));
      if (fragment !== link.id) {
        fail('block-clears-the-header', `the link labelled ${JSON.stringify(link.label)} names the fragment ${JSON.stringify(fragment)}, want ${JSON.stringify(link.id)}`);
      }

      await anchor.click();
      const moved = await page.waitForURL((url) => url.pathname.endsWith('Tide.md'), { timeout: 10_000 })
        .then(() => true, () => false);
      if (!moved) {
        broken(`following ${JSON.stringify(link.label)} did not move this tab (it is still at ${page.url()})`);
      }

      await waitArrival(page);

      const arrival = await page.evaluate((host) => {
        const target = document.querySelector(':target');
        const header = document.querySelector('.y-header');
        if (!target) return { missing: 'target' };
        if (!header) return { missing: 'header' };
        const block = target.closest(host);
        if (!block) return { missing: host };
        const lineHeight = parseFloat(getComputedStyle(block).lineHeight);
        const height = block.getBoundingClientRect().height;
        const headerHeight = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--header-height'));
        const paintedHeight = header.getBoundingClientRect().height;
        const clearance = headerHeight + 16;
        return {
          hash: decodeURIComponent(location.hash.slice(1)),
          id: target.id,
          host: block.tagName,
          lines: lineHeight ? height / lineHeight : 0,
          top: target.getBoundingClientRect().top,
          headerBottom: header.getBoundingClientRect().bottom,
          headerHeight,
          paintedHeight,
          viewport: window.innerHeight,
          scrollMarginTop: getComputedStyle(target).scrollMarginTop,
          clearance,
        };
      }, link.host);
      if (arrival.missing) {
        broken(`after following ${JSON.stringify(link.label)} the page has no ${arrival.missing}`);
      }
      if (arrival.hash !== link.id || arrival.id !== link.id) {
        fail('block-clears-the-header', `after following ${JSON.stringify(link.label)} the address is ${JSON.stringify(arrival.hash)} on ${JSON.stringify(arrival.id)}, want ${JSON.stringify(link.id)}`);
      }
      if (link.minLines && !(arrival.lines >= link.minLines)) {
        broken(`the addressed ${link.host} is ${arrival.lines} lines tall, want ${link.minLines} or more so a single-line landing cannot hide a wrap`);
      }
      if (parseFloat(arrival.scrollMarginTop) !== arrival.clearance) {
        fail('block-clears-the-header', `after following ${JSON.stringify(link.label)} the marked line scroll-margin-top is ${JSON.stringify(arrival.scrollMarginTop)}, want ${arrival.clearance}px from --header-height`);
      }
      if (arrival.headerHeight !== arrival.paintedHeight) {
        fail('block-clears-the-header', `after following ${JSON.stringify(link.label)} --header-height is ${arrival.headerHeight}px but the painted bar is ${arrival.paintedHeight}px tall`);
      }
      // Bound the painted bar, not the token: a landing that only ran out of
      // page can sit anywhere in the viewport and still look clear, and a
      // token that no longer matches the bar would still pass a clearance
      // derived from the custom property.
      if (!(arrival.top >= arrival.headerBottom && arrival.top <= arrival.headerBottom + 24)) {
        fail('block-clears-the-header', `after following ${JSON.stringify(link.label)} the marked line sits at ${arrival.top}px; the header occupies 0–${arrival.headerBottom}px, want the line just under it`);
      }
      await page.close();
    }
  }

  await context.close();
  console.log("PASS heading-fragment: every cross-note section link carries the destination's own anchor, travels to that heading, and a block address on a paragraph or a list item lands its marked line below the sticky header");
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
