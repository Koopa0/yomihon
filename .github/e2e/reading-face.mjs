// Behavior lock: the Latin letters of the reading body are drawn by Newsreader.
//
// fonts.css leads --font-serif with Newsreader so an English run inside serif
// prose is truly serif. The two files it once named were fontsource's
// `latin-ext` subset, which holds U+0020 and Latin Extended and none of A-Z,
// a-z or 0-9, so the browser loaded 36 KB of font, drew the word spaces from
// it, and drew every letter in whatever serif the system fell back to. Nothing
// failed: the hash matched, the face loaded, the page looked like prose. Only
// the engine's own account of which face drew which glyphs says it, so that is
// what this reads: CSS.getPlatformFontsForNode, the same account devtools
// shows under "Rendered Fonts".
//
// Two things are asserted.
//   - The served page, as a reader meets it: the first English paragraph of the
//     reading body is at least 90% Newsreader by glyph count. A paragraph is
//     allowed a few glyphs from elsewhere (a link's arrow, a dash a face does
//     not carry); it is not allowed to be mostly someone else's serif.
//   - Every style the stylesheet declares a Newsreader face for. The styles are
//     read off the live stylesheet's own @font-face rules, not written down
//     here, so a style added later is measured the day it is added. Each is
//     set in the reading body as a specimen of A-Z, a-z and 0-9 and of Latin
//     Extended letters carrying the combining marks fonts.css claims for
//     latin-ext, and every glyph of it must be Newsreader: a face that
//     carries the lower case and not the digits is as wrong as one that
//     carries neither.
//
// The mutations point a Latin face's declaration back at the latin-ext file,
// which is the state this lock exists to end, once for each style.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note whose first paragraph is English prose;
// the browser fixture vault's reading-fidelity.md is one), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';
const FAMILY = 'Newsreader';
const MIN_SHARE = 0.9;
const MIN_LETTERS = 40;
// The letters, then one Latin Extended letter under each of the five combining
// marks (grave, acute, tilde, hook above, dot below) that fonts.css gives to
// latin-ext. A mark no face claims is drawn, with the letter it sits on, by a
// fallback serif. The bases are chosen so no precomposed Vietnamese letter,
// which neither vendored file holds, stands in for the pair.
const SPECIMEN = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 0123456789 \u0105\u0301 \u0101\u0300 \u0119\u0303 \u0117\u0309 \u012B\u0323';
const SITES = ['served-prose-is-newsreader', 'every-declared-style-is-newsreader'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN reading-face: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL reading-face: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN reading-face: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED reading-face: ${message}`);
};

// Points one Latin face's declaration at the latin-ext file by rewriting the
// product's own stylesheet as it is served. The needle is the file name the
// stylesheet declares, which is a name of the product and not the way a build
// tool happened to spell the rule around it, and it has to occur exactly once:
// none means the rule this stands in for is gone, and more than one means the
// edit would land somewhere it was not aimed.
const repointAtLatinExt = (from, to) => async (page) => {
  const seen = { served: 0, occurrences: 0 };
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen.served += 1;
    seen.occurrences = original.split(from).length - 1;
    await route.fulfill({ response, body: original.split(from).join(to) });
  });
  return async () => {
    if (seen.served === 0) return 'the stylesheet was never requested, so the edit reached no page';
    if (seen.occurrences !== 1) return `${from} occurs ${seen.occurrences} times in the served stylesheet, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  // The roman Latin face names the latin-ext file: the state every reading
  // body was in before the Latin subset was shipped.
  'roman-latin-face-points-at-latin-ext': {
    target: 'served-prose-is-newsreader',
    apply: repointAtLatinExt('Newsreader-Latin-Variable.woff2', 'Newsreader-LatinExt-Variable.woff2'),
  },
  // Only the italic Latin face does. The served paragraph is roman and reads
  // exactly as it should, so what has to catch this is the style specimen.
  'italic-latin-face-points-at-latin-ext': {
    target: 'every-declared-style-is-newsreader',
    apply: repointAtLatinExt('Newsreader-Latin-Italic-Variable.woff2', 'Newsreader-LatinExt-Italic-Variable.woff2'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`reading-face: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`reading-face: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`reading-face: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// The font-style of every @font-face the live stylesheets declare for the
// family, in the order they first appear. A sheet the page may not read is
// skipped rather than fatal: it cannot be where the product declares its own.
const declaredStyles = (page) =>
  page.evaluate((family) => {
    const styles = [];
    const walk = (rules) => {
      for (const rule of rules) {
        if (rule instanceof CSSFontFaceRule) {
          const name = rule.style.getPropertyValue('font-family').replace(/^["']|["']$/g, '').trim();
          const style = rule.style.getPropertyValue('font-style').trim() || 'normal';
          if (name === family && !styles.includes(style)) styles.push(style);
        } else if (rule.cssRules) {
          walk(rule.cssRules);
        }
      }
    };
    for (const sheet of document.styleSheets) {
      try {
        walk(sheet.cssRules);
      } catch {
        // a cross-origin sheet; the product's own are same-origin
      }
    }
    return styles;
  }, FAMILY);

// A face that has finished loading is not yet the face the engine reports: the
// text laid out while it was loading is re-laid out on the next frames, and
// the account of "which face drew this" is read off that layout. Reading it
// the moment the load settles can report the fallback that was drawn a moment
// before, which is a measurement of nothing the reader sees.
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

// What the reader's paragraph says it is: its text, so the premise can be
// checked, and a marker the protocol can find it by.
const markServedParagraph = (page) =>
  page.evaluate(() => {
    const paragraph = document.querySelector('.y-prose p');
    if (!paragraph) return null;
    paragraph.setAttribute('data-reading-face', 'served');
    return paragraph.textContent;
  });

// Sets the specimen in the reading body in the given style and waits for the
// faces that style needs, so the measurement is of the face a reader sees and
// not of the fallback shown while it loads.
const setSpecimen = (page, style) =>
  page.evaluate(
    async ({ style, text, family }) => {
      const prose = document.querySelector('.y-prose');
      if (!prose) return false;
      for (const old of prose.querySelectorAll('[data-reading-face="specimen"]')) old.remove();
      const specimen = document.createElement('p');
      specimen.setAttribute('data-reading-face', 'specimen');
      specimen.style.fontStyle = style;
      specimen.textContent = text;
      prose.append(specimen);
      void specimen.offsetHeight;
      // A face that fails to load rejects the load. What this reads is which
      // face drew the glyphs, so the rejection is not the finding: the
      // fallback it leaves behind is, and the assertion below reports that.
      await document.fonts.load(`${style} 400 16px "${family}"`, text).catch(() => []);
      return true;
    },
    { style, text: SPECIMEN, family: FAMILY },
  );

// The engine's account of the faces that drew the marked element: how many
// glyphs the family's web font drew and how many anything else did.
const measureFaces = async (cdp, marker) => {
  const { root } = await cdp.send('DOM.getDocument', { depth: 0 });
  const { nodeId } = await cdp.send('DOM.querySelector', { nodeId: root.nodeId, selector: `[data-reading-face="${marker}"]` });
  if (!nodeId) return null;
  const { fonts } = await cdp.send('CSS.getPlatformFontsForNode', { nodeId });
  let own = 0;
  let other = 0;
  const drawn = [];
  for (const font of fonts) {
    drawn.push(`${font.familyName}${font.isCustomFont ? ' (web font)' : ''} x${font.glyphCount}`);
    if (font.isCustomFont && font.familyName.includes(FAMILY)) own += font.glyphCount;
    else other += font.glyphCount;
  }
  return { own, other, total: own + other, drawn: drawn.join(', ') };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => document.fonts.ready);
  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const cdp = await context.newCDPSession(page);
  await cdp.send('DOM.enable');
  await cdp.send('CSS.enable');

  const served = await markServedParagraph(page);
  if (served === null) broken(`${PAGE} paints no .y-prose paragraph to read`);
  const letters = (served.match(/[A-Za-z]/g) || []).length;
  if (letters < MIN_LETTERS) {
    broken(`the first .y-prose paragraph of ${PAGE} holds ${letters} Latin letters, want at least ${MIN_LETTERS}; it is not the English prose this lock is about`);
  }
  // Only a face that has loaded can have drawn anything. A page whose Newsreader
  // never loaded is a failure of the lock's subject, said below, not of the page,
  // so a load that rejects is let through to be measured.
  await page.evaluate(
    ({ text, family }) => document.fonts.load(`400 16px "${family}"`, text).catch(() => []),
    { text: served, family: FAMILY },
  );
  await settle(page);
  const prose = await measureFaces(cdp, 'served');
  if (prose === null) broken('the marked paragraph vanished before its fonts were read');
  if (prose.total === 0) broken('the engine reports no glyphs drawn for the first .y-prose paragraph');
  if (prose.own / prose.total < MIN_SHARE) {
    fail(
      'served-prose-is-newsreader',
      `${Math.round((100 * prose.own) / prose.total)}% of the ${prose.total} glyphs in the first English .y-prose paragraph of ${PAGE} are ${FAMILY}, want at least ${100 * MIN_SHARE}%; drawn by: ${prose.drawn}`,
    );
  }

  const styles = await declaredStyles(page);
  if (styles.length === 0) broken(`no @font-face rule declares ${FAMILY}, so there is no style to measure`);
  for (const style of styles) {
    if (!(await setSpecimen(page, style))) broken(`there is no .y-prose to set a ${style} specimen in`);
    await settle(page);
    const specimen = await measureFaces(cdp, 'specimen');
    if (specimen === null) broken(`the ${style} specimen vanished before its fonts were read`);
    if (specimen.total === 0) broken(`the engine reports no glyphs drawn for the ${style} specimen`);
    if (specimen.other > 0) {
      fail(
        'every-declared-style-is-newsreader',
        `${specimen.other} of the ${specimen.total} glyphs of the A-Z, a-z, 0-9 and accented-letter specimen set in ${style} are not ${FAMILY}; drawn by: ${specimen.drawn}`,
      );
    }
  }

  console.log(
    `PASS reading-face: ${Math.round((100 * prose.own) / prose.total)}% of the first English paragraph is ${FAMILY}, and the A-Z, a-z, 0-9 and accented-letter specimen is wholly ${FAMILY} in ${styles.join(' and ')}`,
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
