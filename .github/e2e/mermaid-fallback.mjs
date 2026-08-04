// Behavior lock: Mermaid remains a progressive enhancement in both directions.
// When its module loads, a source block becomes an SVG. When that request is
// aborted, the same block remains readable source and the loading shimmer stops.
//
// The fixture vault deliberately carries no Mermaid block. This probe injects
// one into the served note response, before the module entry runs, so the product
// fixture does not acquire test-only content and both cases exercise the real
// renderer, stylesheet, module request, and boot path.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a note page).
// MUTATE names one self-test mode below; MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const DIAGRAM_SOURCE = 'graph TD\n  Alpha --> Beta';
const DIAGRAM_HTML = `<div class="mermaid-diagram" data-mermaid-code="graph+TD%0A++Alpha+--%3E+Beta">${DIAGRAM_SOURCE}</div>`;
const SITES = [
  'module-success-render',
  'module-load-fallback',
  'label-linebreak-render',
  'diagram-failure-is-visible',
];
// A label written across two lines. Drawn as HTML this becomes an unclosed
// break inside a foreign object, which the strict parse the runtime performs
// refuses — so this source is the shape that separates the two label modes.
const WRAPPED_SOURCE = 'graph TD\n  A["first line\\nsecond line"] --> B["plain"]';
const WRAPPED_HTML = `<div class="mermaid-diagram" data-mermaid-code="graph+TD%0A++A%5B%22first+line%5Cnsecond+line%22%5D+--%3E+B%5B%22plain%22%5D">${WRAPPED_SOURCE}</div>`;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN mermaid-fallback: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL mermaid-fallback: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN mermaid-fallback: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED mermaid-fallback: ${message}`); };

const occurrences = (body, needle) => body.split(needle).length - 1;

// Put one real source block inside the note's .y-prose before the deferred
// runtime executes. The returned proof is checked per page: one case cannot
// inherit the other case's successful injection.
const injectDiagram = async (page, diagramHTML = DIAGRAM_HTML) => {
  // Anchored on the prose container's own opening tag rather than on the
  // article's closing run: what follows the prose is page furniture that
  // changes when the reading page gains a way onward, and an anchor that
  // moves with the furniture is an anchor that breaks with it.
  const needle = '<div class="y-prose">';
  let matches = -1;
  await page.route(BASE + PAGE, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches = occurrences(original, needle);
    const body = matches === 1 ? original.replaceAll(needle, `${needle}${diagramHTML}`) : original;
    return route.fulfill({ response, body });
  });
  return () => matches;
};

// Rewrite the runtime response, not a stand-in. Requiring one exact needle
// proves a mutation changed the only production site it claims to exercise.
const rewriteRuntime = (moduleName, needle, replacement) => async (page) => {
  let matches = -1;
  await page.route(`**/${moduleName}`, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    matches = occurrences(original, needle);
    const body = matches === 1 ? original.replaceAll(needle, replacement) : original;
    return route.fulfill({ response, body });
  });
  return () => matches;
};

// Two rewrites in one response, for a lock that only fails when both halves of
// a defence are gone. Each needle must still match exactly once, so a mutation
// whose second half silently found nothing reports itself rather than passing.
const rewriteRuntimeTwice = (moduleName, first, second) => async (page) => {
  let matches = -1;
  await page.route(`**/${moduleName}`, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const [a, b] = [occurrences(original, first[0]), occurrences(original, second[0])];
    matches = a === 1 && b === 1 ? 1 : Math.min(a, b);
    const body = matches === 1
      ? original.replaceAll(first[0], first[1]).replaceAll(second[0], second[1])
      : original;
    return route.fulfill({ response, body });
  });
  return () => matches;
};

const HTML_LABELS_ON = ['    htmlLabels: false,', '    htmlLabels: true,'];
const SWALLOW_PARSE_FAILURE = [
  "        throw new Error('the renderer produced markup that is not an SVG');",
  '        continue;',
];

const MUTATIONS = {
  'restore-html-labels': {
    target: 'label-linebreak-render',
    apply: rewriteRuntime('diagrams.js', HTML_LABELS_ON[0], HTML_LABELS_ON[1]),
  },
  'swallow-parse-failure': {
    target: 'diagram-failure-is-visible',
    apply: rewriteRuntimeTwice('diagrams.js', HTML_LABELS_ON, SWALLOW_PARSE_FAILURE),
  },
  'drop-rendered-svg': {
    target: 'module-success-render',
    apply: rewriteRuntime(
      'diagrams.js',
      '      element.appendChild(document.importNode(svgElement, true));',
      "      element.appendChild(document.createTextNode('render suppressed'));",
    ),
  },
  'suppress-load-error-marker': {
    target: 'module-load-fallback',
    apply: rewriteRuntime(
      'yomihon.js',
      "    root.setAttribute('data-mermaid-error', '');",
      "    root.removeAttribute('data-mermaid-error');",
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`mermaid-fallback: mutation ${name} aims at the unknown assertion site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`mermaid-fallback: the ${site} assertion is aimed at by no mutation, so nothing shows it can fail`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`mermaid-fallback: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const arm = async (page, site) => {
  if (!MUTATE || MUTATIONS[MUTATE].target !== site) return null;
  return MUTATIONS[MUTATE].apply(page);
};

const proveDiagramInjected = (proof, caseName) => {
  const matches = proof();
  if (matches !== 1) broken(`${caseName} document injection matched ${matches}, want exactly 1`);
};
const proveMutationApplied = (proof) => {
  if (!proof) return;
  const matches = proof();
  if (matches !== 1) notApplied(`the ${MUTATE} runtime needle matched ${matches}, want exactly 1`);
};

// Read the alpha from any functional computed-color notation. An absent alpha
// means opaque; a present value that cannot be parsed is refused, not guessed.
const alphaOf = (color) => {
  const value = color.trim();
  if (value === 'transparent') return 0;
  const fn = value.match(/^[a-z-]+\((.*)\)$/i);
  if (!fn) return 1;
  const args = fn[1];
  const slash = args.lastIndexOf('/');
  let raw;
  if (slash >= 0) {
    raw = args.slice(slash + 1);
  } else {
    const commas = args.split(',');
    if (commas.length !== 4) return 1;
    raw = commas[3];
  }
  const token = raw.trim();
  const number = Number.parseFloat(token);
  if (!Number.isFinite(number)) return Number.NaN;
  return token.endsWith('%') ? number / 100 : number;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let fallbackBrowser = null;
try {
  // Case 1: the vendored module and its chunks load, and source becomes SVG.
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const injected = await injectDiagram(page);
    const mutated = await arm(page, 'module-success-render');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveDiagramInjected(injected, 'case 1');
    proveMutationApplied(mutated);
    await page.waitForFunction(() => document.querySelector('.mermaid-diagram > svg'), null, { timeout: 5000 }).catch(() => {});
    const block = page.locator('.mermaid-diagram');
    if (await block.count() !== 1) broken('case 1 has no single injected Mermaid block');
    if (await block.locator(':scope > svg').count() !== 1) {
      fail('module-success-render', 'case 1 (module loaded): diagram source did not become one SVG');
    }
    await context.close();
  }

  // Case 2: abort the exact entry-module URL. Boot must record the rejection
  // on the root, which returns every still-unrendered block to the no-JS face.
  // A separate browser process keeps case 1's successful module load out of
  // this case's HTTP cache; an abort that sees no request proves nothing.
  {
    fallbackBrowser = await chromium.launch({ channel: 'chrome', headless: true });
    const context = await fallbackBrowser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    let blocked = false;
    let recordBlocked;
    const blockedRequest = new Promise((resolve) => { recordBlocked = resolve; });
    await page.route('**/mermaid.esm.min.mjs', (route) => {
      blocked = true;
      recordBlocked();
      return route.abort('failed');
    });
    const injected = await injectDiagram(page);
    const mutated = await arm(page, 'module-load-fallback');
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveDiagramInjected(injected, 'case 2');
    proveMutationApplied(mutated);
    await Promise.race([
      blockedRequest,
      new Promise((resolve) => setTimeout(resolve, 2000)),
    ]);
    if (!blocked) broken('case 2 aborted nothing: the Mermaid entry module was never requested');
    await page.waitForFunction(() => document.documentElement.hasAttribute('data-mermaid-error'), null, { timeout: 2000 }).catch(() => {});

    const block = page.locator('.mermaid-diagram');
    if (await block.count() !== 1) broken('case 2 has no single injected Mermaid block');
    const state = await block.evaluate((element) => {
      const style = getComputedStyle(element);
      return {
        animationName: style.animationName,
        backgroundImage: style.backgroundImage,
        color: style.color,
        text: element.textContent,
        visible: element.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true }),
        svgCount: element.querySelectorAll(':scope > svg').length,
        rootMarked: document.documentElement.hasAttribute('data-mermaid-error'),
      };
    });
    if (!state.rootMarked) fail('module-load-fallback', 'case 2 (module aborted): root has no data-mermaid-error marker');
    if (state.text !== DIAGRAM_SOURCE || !state.visible) {
      fail('module-load-fallback', `case 2 (module aborted): source is not visibly preserved; text=${JSON.stringify(state.text)}, visible=${state.visible}`);
    }
    const colorAlpha = alphaOf(state.color);
    if (!Number.isFinite(colorAlpha)) broken(`case 2 cannot read an alpha from computed source color ${state.color}`);
    if (colorAlpha === 0) fail('module-load-fallback', `case 2 (module aborted): source color remains transparent (${state.color})`);
    if (state.animationName !== 'none' || state.backgroundImage !== 'none') {
      fail('module-load-fallback', `case 2 (module aborted): shimmer remains (animation=${state.animationName}, background=${state.backgroundImage})`);
    }
    if (state.svgCount !== 0) fail('module-load-fallback', 'case 2 (module aborted): an SVG appeared despite the aborted entry module');
    await context.close();
  }

  // Case 3: a label written across two lines. Drawn as HTML it arrives as an
  // unclosed break inside a foreign object, which the strict parse refuses;
  // five such diagrams in the vault sat on their loading shimmer for as long
  // as the page stayed open, because the refusal returned quietly instead of
  // ending where every other failure ends. Both halves are checked here: the
  // diagram arrives, and if it ever does not, it says so instead of pretending
  // to still be coming.
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const injected = await injectDiagram(page, WRAPPED_HTML);
    const mutated = (await arm(page, 'label-linebreak-render'))
      ?? (await arm(page, 'diagram-failure-is-visible'));
    await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
    proveDiagramInjected(injected, 'case 3');
    proveMutationApplied(mutated);
    await page.waitForFunction(
      () => document.querySelector('.mermaid-diagram > svg')
        || document.querySelector('.mermaid-diagram[data-mermaid-error]'),
      null,
      { timeout: 5000 },
    ).catch(() => {});

    const block = page.locator('.mermaid-diagram');
    if (await block.count() !== 1) broken('case 3 has no single injected Mermaid block');
    const settled = await block.evaluate((element) => ({
      svgCount: element.querySelectorAll(':scope > svg').length,
      said: element.hasAttribute('data-mermaid-error'),
      foreignObject: element.querySelectorAll('foreignObject').length,
    }));
    if (settled.svgCount === 0 && !settled.said) {
      fail('diagram-failure-is-visible', 'case 3 (wrapped label): the block neither became an SVG nor said it failed');
    }
    if (settled.svgCount !== 1) {
      fail('label-linebreak-render', `case 3 (wrapped label): became ${settled.svgCount} SVG, want 1`);
    }
    if (settled.foreignObject !== 0) {
      fail('label-linebreak-render', 'case 3 (wrapped label): the label is still drawn as embedded HTML');
    }
    await context.close();
  }

  console.log('PASS mermaid-fallback: module load renders SVG; aborted module restores readable source with no shimmer; a wrapped label renders and a failure says so');
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
  await fallbackBrowser?.close();
  await browser.close();
}
