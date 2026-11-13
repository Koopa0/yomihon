// Behavior lock: a fact yomihon already knew is legible where a reader
// actually looks. A link whose section address was split off a name nothing
// answers to says how it was read inside 筆記狀況. It used to be a colour, a
// hover, or nothing at all, and the reader who could act on it is the one
// least likely to open every note.
//
// Markup alone is not the contract here — text present in the DOM but clipped
// out of sight reads to a Go test exactly like text on screen, so the
// assertion below ends at a hit test against real geometry.
//
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';


const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const NOTE_PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['fragment-split-painted'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN fragment-split-visibility: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL fragment-split-visibility: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN fragment-split-visibility: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED fragment-split-visibility: ${message}`); };

// rewritePath proves it applied by counting, so a needle that rots against a
// rewritten template turns this run red rather than quietly mutating nothing.
const rewritePath = (path, needle, replacement, expected, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + path, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `${label} document was never requested`;
    if (matches !== expected * requests) return `${label} needle matched ${matches} times over ${requests} request(s), want ${expected} each`;
    return '';
  };
};

const hideVia = (path, selector, label) => rewritePath(
  path,
  '</head>',
  `<style>${selector}{display:none!important}</style></head>`,
  1,
  label,
);

const MUTATIONS = {
  'fade-the-fragment-split-away': {
    target: 'fragment-split-painted',
    apply: rewritePath(
      NOTE_PAGE,
      '</head>',
      '<style>.y-diag{opacity:0!important}</style></head>',
      1,
      'fragment-split fade style',
    ),
  },
  'hide-fragment-split': {
    target: 'fragment-split-painted',
    apply: hideVia(NOTE_PAGE, '.y-diag__split', 'fragment-split hide style'),
  },
  // The section half is the part the reader cannot otherwise learn; dropping
  // it leaves a sentence that still looks like an explanation.
  'strip-split-section': {
    target: 'fragment-split-painted',
    apply: rewritePath(NOTE_PAGE, '、章節 <code>補足</code>', '', 1, 'fragment-split section'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`fragment-split-visibility: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`fragment-split-visibility: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`fragment-split-visibility: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// painted answers the only question that matters here: is this
// element's own text on the screen, in ink, at a place the pointer lands on it.
//
// Text can be present in the document and unreadable in several unrelated ways,
// and each one has to be asked about separately, because none of them implies
// another. It can be laid out to nothing; it can be scrolled or clipped away;
// it can sit under something else; it can be faded out by an ancestor, which
// the element's own computed opacity still reports as 1 because opacity does
// not inherit; and it can be written in transparent ink, which changes no box
// and no hit test at all. The browser answers the first group through
// checkVisibility, which unlike a hand-rolled display/visibility pair also
// accounts for content-visibility and for opacity anywhere above the element.
// The ink is the one thing it will not answer, so the alpha is read here.
const painted = (locator) => locator.evaluate((element) => {
  element.scrollIntoView({ block: 'center' });
  const rect = element.getBoundingClientRect();
  const style = getComputedStyle(element);
  const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
  // Any colour notation resolves to rgb()/rgba() through a canvas, so the
  // alpha can be read without parsing oklch by hand.
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = style.color;
  ctx.fillRect(0, 0, 1, 1);
  return {
    text: element.textContent.replace(/\s+/g, ' ').trim(),
    width: rect.width,
    height: rect.height,
    inViewport: rect.top >= 0 && rect.bottom <= innerHeight,
    display: style.display,
    visibility: style.visibility,
    opacity: style.opacity,
    inkAlpha: ctx.getImageData(0, 0, 1, 1).data[3],
    visible: element.checkVisibility({
      opacityProperty: true,
      visibilityProperty: true,
      contentVisibilityAuto: true,
    }),
    // Only the element itself or something inside it counts. Accepting an
    // ancestor here would call a clipped element painted, because clipping is
    // exactly what leaves the parent sitting at that point.
    hit: hit === element || element.contains(hit),
  };
});

const onScreen = (seen) => seen.width > 1 && seen.height > 1 && seen.inViewport
  && seen.visible && seen.inkAlpha > 0 && seen.hit;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  // The findings stand in the block that ends the note, at every width, so the
  // reader reaches them by following the sidebar's last contents row or by
  // scrolling to the end; the probe brings them into view the way a reader
  // would rather than reaching past that into markup nobody has revealed.
  await page.setViewportSize({ width: 1024, height: 800 });
  await page.goto(BASE + NOTE_PAGE, { waitUntil: 'domcontentloaded' });
  await arrived(page);

  const block = page.locator('.y-related');
  if (await block.count() !== 1) broken('the note page has no block at its end for the findings to stand in');

  const splits = block.locator('.y-diag__split');
  if (await splits.count() === 0) {
    fail('fragment-split-painted', 'the note states no reading for a link that addressed a section');
  }
  let shown = null;
  for (let i = 0; i < await splits.count(); i += 1) {
    // A copy that is not laid out cannot be scrolled to, and says so by not being painted.
    await splits.nth(i).scrollIntoViewIfNeeded({ timeout: 1000 }).catch(() => {});
    const seen = await painted(splits.nth(i));
    if (onScreen(seen)) { shown = seen; break; }
  }
  if (!shown) {
    fail('fragment-split-painted', `no copy of the link's reading is on screen; first = ${JSON.stringify(await painted(splits.first()))}`);
  }
  for (const half of ['Missing lesson fixture target', '補足']) {
    if (!shown.text.includes(half)) {
      fail('fragment-split-painted', `the reading omits ${JSON.stringify(half)}: ${JSON.stringify(shown.text)}`);
    }
  }
  if (!shown.text.includes('筆記目標') || !shown.text.includes('章節')) {
    fail('fragment-split-painted', `the reading does not name which half is which: ${JSON.stringify(shown.text)}`);
  }

  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log('PASS fragment-split-visibility: the link reading is painted where a reader looks');
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = await proof();
    if (issue) {
      console.error(`NOT-APPLIED fragment-split-visibility: ${MUTATE}: ${issue}`);
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
      await browser.close();
      process.exit(2);
    }
  }
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
