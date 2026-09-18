// Behavior lock for the interface's tempo: motion exists so a state change can
// be read, and a reader who asks for less of it gets none.
//
// Three things are asserted, and they are separate claims. A reader who asks
// for reduced motion has every transition inside the reading chrome collapsed —
// the reading-position hairline excepted, because that is scroll state rather
// than decoration. The blanket that does the collapsing reaches elements and
// ::before/::after and not ::details-content, so the folds are read through
// that pseudo-element by name; reading the elements alone would pass over
// exactly the surface the blanket cannot see.
//
// With motion allowed, every fold the interface owns opens by growing. The
// oracle is the height of ::details-content, sampled at the click and again
// once it has settled: a declared duration is not the observable, because a
// wrapper whose height cannot interpolate still reports one while snapping
// open. And the concept sheet, which is held against an edge of the window,
// arrives from that edge rather than materialising in place.
//
// Env: YOMIHON_BASE, PAGE_PATH (a lesson carrying a folding metadata row, the
// inline reading aids, a no-return confirm, and a concept term), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SEARCH = '/search?q=a';
const SHEET = '[data-concept-sheet]';
const CONCEPT = '[data-concept]';

// The four folds the interface owns, with the page and width each one is
// actually reachable on. The metadata row and the no-return confirm ride the
// note; the inline reading aids stand in for the right rail at and below
// 1280px; the value column has a summary to press only below that width, since
// the search page holds it open and takes the control away where the results
// have a column beside them.
const FOLDS = [
  { key: 'metadata row', path: null, width: 1280, selector: 'details.y-metarow' },
  { key: 'inline reading aids', path: null, width: 1280, selector: 'details.y-toc-inline' },
  { key: 'no-return confirm', path: null, width: 1280, selector: 'details.y-statusconfirm' },
  { key: 'search value column', path: SEARCH, width: 900, selector: 'details.y-facets' },
];

// Where the reduced-motion walk runs. Three readings rather than one: the
// note carries three of the folds, the value column lives on the search page,
// and the narrow width swaps the rail for a drawer and the reading aids for an
// inline block, so it resolves different rules over the same markup.
const QUIET_STOPS = [
  { name: 'the note at 1280px', path: null, width: 1280, folds: ['details.y-metarow', 'details.y-toc-inline', 'details.y-statusconfirm'] },
  { name: 'the note at 390px', path: null, width: 390, folds: ['details.y-metarow', 'details.y-toc-inline', 'details.y-statusconfirm'] },
  { name: 'the search page at 900px', path: SEARCH, width: 900, folds: ['details.y-facets'] },
];

const SITES = [
  'every-transition-collapses-under-reduce',
  'folds-open-by-growing',
  'sheet-enters-from-its-edge',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN motion-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL motion-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN motion-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED motion-contract: ${message}`); };

// A mutation edits what the browser receives. Appending lands the rule after
// everything the product's own sheet declares, which is what a later rule
// undoing an earlier one looks like.
//
// Being served the sheet is not proof the rule took effect, so the proof is a
// positive read of the parsed CSSOM: the injected declaration has to be found
// on a live rule, and every injection below carries an importance flag the
// product's own rules do not, so a rule of the same name cannot answer for it.
const appendStylesheet = (rule, wanted) => async (context) => {
  let requests = 0;
  await context.route('**/static/app.css', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async (page) => {
    if (requests < 1) return 'the stylesheet was never requested, so the rule reached no page';
    const found = await page.evaluate(({ selector, property }) => {
      const hits = [];
      const walk = (rules) => {
        for (const item of rules) {
          if (item.cssRules) walk(item.cssRules);
          if (item.selectorText !== selector || !item.style) continue;
          if (item.style.getPropertyPriority(property) === 'important') hits.push(item.style.getPropertyValue(property));
        }
      };
      for (const sheet of document.styleSheets) {
        try { walk(sheet.cssRules); } catch { /* a sheet the page may not read cannot hold the injection */ }
      }
      return hits;
    }, wanted);
    if (found.length < 1) {
      return `no live rule ${wanted.selector} carries an important ${wanted.property}, so the injected rule never reached the cascade`;
    }
    return '';
  };
};

const MUTATIONS = {
  // The fold that reports a duration and snaps anyway. Only the keyword that
  // lets a height of auto be interpolated is taken away, so the transition is
  // still declared and still the right length — and the fold jumps to its full
  // height at the press. A lock that read the declaration alone would call this
  // a pass, which is why the height is sampled instead.
  'snap-a-fold-open': {
    target: 'folds-open-by-growing',
    apply: appendStylesheet(
      '.y-metarow::details-content{interpolate-size:numeric-only !important}',
      { selector: '.y-metarow::details-content', property: 'interpolate-size' },
    ),
  },
  // One fold left out of the reduced-motion lines. The blanket cannot reach
  // ::details-content, so a fold missing from that list keeps its motion for
  // the reader who asked for none, and every other fold hides the omission.
  'leave-a-fold-out-of-reduced-motion': {
    apply: appendStylesheet(
      '@media (prefers-reduced-motion: reduce){.y-toc-inline::details-content{transition:height var(--dur-base) var(--ease-standard) !important}}',
      { selector: '.y-toc-inline::details-content', property: 'transition' },
    ),
    target: 'every-transition-collapses-under-reduce',
  },
  // The blanket itself weakened for an ordinary element, which is the other
  // half of the same claim: the walk has to read the elements, not only the
  // four pseudo-elements it was written for.
  'restore-a-hover-under-reduced-motion': {
    apply: appendStylesheet(
      '@media (prefers-reduced-motion: reduce){.yomihon .ui-navitem{transition-duration:var(--dur-slow) !important}}',
      { selector: '.yomihon .ui-navitem', property: 'transition-duration' },
    ),
    target: 'every-transition-collapses-under-reduce',
  },
  // The sheet materialising in place: it still fades, so a lock reading only
  // the fade would call this a pass, and the panel would stop saying which
  // edge of the window it came from and will go back to.
  'slide-the-sheet-in-from-nowhere': {
    target: 'sheet-enters-from-its-edge',
    apply: appendStylesheet(
      '.y-conceptsheet{transform:none !important}\n.y-conceptsheet[open]{transform:none !important}\n@starting-style{.y-conceptsheet[open]{transform:none}}',
      { selector: '.y-conceptsheet', property: 'transform' },
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`motion-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`motion-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (FOLDS.length !== 4) {
  console.error(`motion-contract: the fold table names ${FOLDS.length} folds, want the four the interface owns`);
  process.exit(2);
}
{
  const named = new Set(QUIET_STOPS.flatMap((stop) => stop.folds));
  for (const fold of FOLDS) {
    if (!named.has(fold.selector)) {
      console.error(`motion-contract: ${fold.selector} is never read under reduced motion, so one fold's collapse is unasserted`);
      process.exit(2);
    }
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`motion-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Chrome serialises a computed duration in seconds whatever unit it was
// written in, and a list of them is comma-separated, so the longest of the
// list is the number to compare.
const longestSeconds = (text) => String(text)
  .split(',')
  .map((part) => {
    const value = part.trim();
    if (value.endsWith('ms')) return Number.parseFloat(value) / 1000;
    if (value.endsWith('s')) return Number.parseFloat(value);
    return Number.NaN;
  })
  .reduce((most, one) => (Number.isFinite(one) && one > most ? one : most), 0);

// The duration a token stands for, read from the page under test rather than
// from the source, so the comparison is against what the browser resolved. A
// custom property read from a page that has loaded no stylesheet answers with
// the empty string, which parses to zero and would let a measured duration be
// compared against nothing at all; that is a broken run, not a failed lock.
const readToken = async (target, name) => {
  const value = await target.evaluate(
    (property) => getComputedStyle(document.documentElement).getPropertyValue(property),
    name,
  );
  if (!(longestSeconds(value) > 0)) {
    broken(`${name} reads as ${JSON.stringify(value)} on the page under test, so no measured duration could be judged against it`);
  }
  return value;
};

// The collapsed ceiling. The blanket writes 0.001ms and the explicit fold lines
// write none, so anything a reader could perceive is far above this.
const QUIET = 0.001;

const readQuiet = (page, foldSelectors) => page.evaluate(({ selectors, ceiling }) => {
  // The same reading as the driver's, done here so a page of thousands of
  // elements does not cross the boundary one record at a time.
  const longest = (text) => String(text)
    .split(',')
    .map((part) => {
      const value = part.trim();
      if (value.endsWith('ms')) return Number.parseFloat(value) / 1000;
      if (value.endsWith('s')) return Number.parseFloat(value);
      return Number.NaN;
    })
    .reduce((most, one) => (Number.isFinite(one) && one > most ? one : most), 0);

  const all = document.querySelectorAll('*');
  const loud = [];
  for (const element of all) {
    if (element.matches('.y-readline')) continue;
    const duration = getComputedStyle(element).transitionDuration;
    if (longest(duration) <= ceiling) continue;
    const classes = element.getAttribute('class');
    loud.push({ what: element.tagName.toLowerCase() + (classes ? `.${classes.trim().split(/\s+/).join('.')}` : ''), duration });
  }
  const folds = selectors.map((selector) => {
    const element = document.querySelector(selector);
    return { selector, present: !!element, duration: element ? getComputedStyle(element, '::details-content').transitionDuration : '' };
  });
  return { loud, folds, total: all.length };
}, { selectors: foldSelectors, ceiling: QUIET });

// Opening a fold and watching its wrapper grow, frame by frame. The wrapper's
// own transition is not among the animations the page hands out — the page
// offers the chevron's and nothing of the pseudo-element's — so the reading is
// taken from the height itself: a fold that grows stands, on some frame,
// between nothing and its full height, and a fold that snaps is only ever at
// one or the other. The declared duration cannot tell the two apart, because a
// wrapper whose height cannot interpolate reports the same duration while
// jumping; both are read, since a fold can also be slowed or silenced.
const openAndMeasure = (page, selector) => page.evaluate(async ({ sel, frames }) => {
  const folds = [...document.querySelectorAll(sel)].filter((element) => element.checkVisibility());
  if (folds.length === 0) return { missing: true };
  const fold = folds[0];
  const summary = fold.querySelector(':scope > summary');
  if (!summary) return { noSummary: true };
  if (fold.open) {
    summary.click();
    await new Promise((resolve) => setTimeout(resolve, 400));
  }
  const declared = getComputedStyle(fold, '::details-content').transitionDuration;
  const pressed = performance.now();
  summary.click();
  const trace = [];
  for (let i = 0; i < frames; i += 1) {
    await new Promise((resolve) => requestAnimationFrame(resolve));
    trace.push({
      at: Math.round(performance.now() - pressed),
      height: Number.parseFloat(getComputedStyle(fold, '::details-content').height),
    });
  }
  await new Promise((resolve) => setTimeout(resolve, 500));
  return {
    opened: fold.open,
    declared,
    trace,
    settled: Number.parseFloat(getComputedStyle(fold, '::details-content').height),
  };
}, { sel: selector, frames: 14 });

const traceText = (trace) => trace.map((frame) => `${frame.at}ms ${frame.height}px`).join(', ');

// The sheet's arrival, read from the transition itself. Holding the transform
// transition at its own time zero is what makes the travelled distance
// readable: a sampled frame cannot tell 16px already half spent from a sheet
// that never moved.
const openSheetAndMeasure = (page) => page.evaluate(async ({ concept, sheet }) => {
  const link = document.querySelector(concept);
  if (!link) return { noConcept: true };
  link.click();
  const element = document.querySelector(sheet);
  if (!element) return { noSheet: true };
  await new Promise((resolve) => requestAnimationFrame(resolve));
  const running = element.getAnimations().map((animation) => ({
    property: animation.transitionProperty ?? animation.animationName ?? '',
    duration: animation.effect?.getComputedTiming().duration,
  }));
  const travel = element.getAnimations().find((animation) => animation.transitionProperty === 'transform');
  let atStart = null;
  if (travel) {
    const held = travel.currentTime;
    travel.currentTime = 0;
    atStart = getComputedStyle(element).transform;
    travel.currentTime = held;
  }
  return { open: element.open, running, atStart };
}, { concept: CONCEPT, sheet: SHEET });

const translationX = (matrix) => {
  const numbers = (String(matrix).match(/-?\d*\.?\d+(?:e-?\d+)?/g) || []).map(Number);
  if (numbers.length === 6) return numbers[4];
  if (numbers.length === 16) return numbers[12];
  return Number.NaN;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const quiet = await browser.newContext({ viewport: { width: 1280, height: 900 }, reducedMotion: 'reduce' });
  const moving = await browser.newContext({ viewport: { width: 1280, height: 900 }, reducedMotion: 'no-preference' });
  // Both contexts carry the injection: a mutation aimed at the reduced-motion
  // reader and one aimed at the moving reader are the same edit to the same
  // served sheet, and routing only one context would leave the other reading
  // the product's own bytes.
  let proof = null;
  if (MUTATE) {
    const quietProof = await MUTATIONS[MUTATE].apply(quiet);
    const movingProof = await MUTATIONS[MUTATE].apply(moving);
    proof = { quiet: quietProof, moving: movingProof };
  }
  let confirmed = false;
  const confirm = async (page, which) => {
    if (!proof || confirmed) return;
    const issue = await proof[which](page);
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    confirmed = true;
  };

  // 1 — the reader who asked for less motion.
  const quietPage = await quiet.newPage();
  for (const stop of QUIET_STOPS) {
    await quietPage.setViewportSize({ width: stop.width, height: 900 });
    const response = await quietPage.goto(BASE + (stop.path ?? PAGE), { waitUntil: 'load' });
    if (!response || response.status() !== 200) broken(`${stop.name} answered ${response?.status() ?? 'nothing'}, want 200`);
    await confirm(quietPage, 'quiet');
    const reading = await readQuiet(quietPage, stop.folds);
    if (reading.total < 50) broken(`${stop.name} rendered ${reading.total} elements, which is not a page this can judge`);
    for (const fold of reading.folds) {
      if (!fold.present) broken(`${stop.name} carries no ${fold.selector}, so one of the four folds is unread there`);
    }
    for (const element of reading.loud) {
      if (longestSeconds(element.duration) > QUIET) {
        fail('every-transition-collapses-under-reduce', `on ${stop.name} ${element.what} still transitions for ${element.duration} although the reader asked for less motion`);
      }
    }
    for (const fold of reading.folds) {
      if (longestSeconds(fold.duration) > QUIET) {
        fail('every-transition-collapses-under-reduce', `on ${stop.name} ${fold.selector}::details-content still transitions for ${fold.duration}; the blanket does not reach a pseudo-element, so this fold needs its own line`);
      }
    }
  }

  // 2 — every fold opens by growing, for the reader who did not ask for less.
  const page = await moving.newPage();
  for (const fold of FOLDS) {
    await page.setViewportSize({ width: fold.width, height: 900 });
    const response = await page.goto(BASE + (fold.path ?? PAGE), { waitUntil: 'load' });
    if (!response || response.status() !== 200) broken(`the page carrying the ${fold.key} answered ${response?.status() ?? 'nothing'}, want 200`);
    await confirm(page, 'moving');
    const wanted = await readToken(page, '--dur-base');
    const reading = await openAndMeasure(page, fold.selector);
    if (reading.missing) broken(`the ${fold.key} (${fold.selector}) is not on the page at ${fold.width}px, so it cannot be judged`);
    if (reading.noSummary) broken(`the ${fold.key} has no summary of its own to press`);
    if (!reading.opened) broken(`pressing the ${fold.key}'s summary did not open it`);
    if (!(reading.settled > 8)) broken(`the ${fold.key} settled at ${reading.settled}px, so it holds too little to tell a growth from a snap`);
    if (Math.abs(longestSeconds(reading.declared) - longestSeconds(wanted)) > 1e-6) {
      fail('folds-open-by-growing', `the ${fold.key} opens over ${reading.declared}, want the ${wanted.trim()} of --dur-base that every fold shares`);
    }
    // Half a pixel of room at each end, so a wrapper that rounds its way to
    // the top does not read as a height on the way there.
    const partway = reading.trace.filter((frame) => frame.height > 0.5 && frame.height < reading.settled - 0.5);
    if (partway.length === 0) {
      // A frame that arrives after the transition would already have ended
      // cannot say whether there was one, and under a loaded machine that is
      // the reading's own failure rather than the interface's.
      if (reading.trace.length > 0 && reading.trace[0].at > longestSeconds(wanted) * 500) {
        broken(`the first frame after the ${fold.key} was pressed came ${reading.trace[0].at}ms later, past half of the ${wanted.trim()} it opens over, so this run could not see whether it grew`);
      }
      fail('folds-open-by-growing', `the ${fold.key} went straight to its full ${reading.settled}px (${traceText(reading.trace)}), so it jumped open instead of growing`);
    }
  }

  // 3 — the sheet arrives from the edge it is held against.
  await page.setViewportSize({ width: 1280, height: 900 });
  const lesson = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!lesson || lesson.status() !== 200) broken(`${PAGE} answered ${lesson?.status() ?? 'nothing'}, want 200`);
  await page.waitForSelector('html[data-js]');
  await confirm(page, 'moving');
  const slow = await readToken(page, '--dur-slow');
  const arrival = await openSheetAndMeasure(page);
  if (arrival.noConcept) broken(`${PAGE} paints no concept term, so the sheet cannot be opened`);
  if (arrival.noSheet) broken(`${PAGE} renders no concept sheet`);
  if (!arrival.open) broken('pressing the concept term did not open the sheet');
  const travel = arrival.running.find((animation) => animation.property === 'transform');
  if (!travel) {
    fail('sheet-enters-from-its-edge', `the sheet opened running ${JSON.stringify(arrival.running.map((animation) => animation.property))}, with no transform among them, so it materialises in place instead of arriving from the edge it is held against`);
  }
  if (Math.abs(travel.duration / 1000 - longestSeconds(slow)) > 1e-6) {
    fail('sheet-enters-from-its-edge', `the sheet travels for ${travel.duration}ms, want the ${slow.trim()} of --dur-slow that the summoned surfaces share`);
  }
  const startedAt = translationX(arrival.atStart);
  if (!(Math.abs(startedAt) >= 8)) {
    fail('sheet-enters-from-its-edge', `at its own time zero the sheet stands at ${JSON.stringify(arrival.atStart)}, which is ${startedAt}px from where it comes to rest — too little to read as arriving from an edge`);
  }

  if (proof && !confirmed) broken(`${MUTATE} was never confirmed, so this run proves nothing about it`);

  console.log(`PASS motion-contract: ${QUIET_STOPS.length} readings collapse every transition for the reduced-motion reader, ${FOLDS.length} folds open by growing over --dur-base, and the sheet arrives ${Math.round(Math.abs(startedAt))}px from its edge`);
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
