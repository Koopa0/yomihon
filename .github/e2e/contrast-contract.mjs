// Browser lock for legibility in both reading themes: the text tokens the
// interface is built from, the syntax colours inside a code block, busy-search
// ink after ancestor opacity is composited, and a mark after its wash is.
//
// The least-prominent text tokens are used at 10-13px, including inside
// elevated and hover surfaces, so they owe the normal-text WCAG AA ratio
// (4.5:1), not the 3:1 large-text threshold. Code owes the same ratio for the
// same reason, and owes it against the surface it is actually painted on:
// the reading page gives a code block the product's own panel, so a palette
// measured against the highlighter's intended background would be measuring a
// colour nobody sees. Text and code readings come from a real span on a real
// page. Busy-search and mark readings plant the production classes on that
// page — idle, hovered, and aria-busy — because those states are brief and
// the regression this exists to catch is a stylesheet that is perfectly valid
// and still paints near-black words on a near-black panel.
//
// Chrome performs the OKLCH conversion; the lock reads the resulting pixels
// from a canvas rather than maintaining a second color-conversion algorithm.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (a note whose
// body carries fenced code). MUTATE names one of the self-test modes below;
// MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
// A file that is not a note runs the same highlighter over its whole contents,
// under a different page shell. It is the second place the palette has to
// land, and the one where a code block is the entire page rather than a block
// inside prose.
const SOURCE_PAGE = '/notes/System/schemas/vault-schema.toml';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'light-text-aa',
  'dark-text-aa',
  'light-code-aa',
  'dark-code-aa',
  'print-code-light',
  'code-forced-colors',
  'dark-code-persists',
  'busy-search-aa',
  'mark-aa',
  'mark-padding',
];

// The colours are named by the literal bytes the two palettes ship, never by a
// style name: what a reader receives is the colour, and a name compared
// against itself would hold while the wrong palette was being served.
const DARK_KEYWORD = '#ff7b72';

// The token kinds a code block has to keep legible, each named by the classes
// the highlighter writes for it. A measurement pass that never saw one of
// these measured a page with no code on it, which is a probe reporting on
// nothing rather than a page that passed.
const REQUIRED_TOKENS = [
  { name: 'keyword', classes: ['k', 'kc', 'kd', 'kn', 'kr', 'kt'] },
  { name: 'identifier', classes: ['nx', 'nf', 'nb', 'na', 'nv', 'no', 'n'] },
  { name: 'string or URL', classes: ['s', 's1', 's2', 'sb', 'sd', 'sx', 'sr'] },
  { name: 'operator or punctuation', classes: ['o', 'ow', 'p'] },
  { name: 'comment', classes: ['c', 'c1', 'cm', 'ch', 'cs', 'cp'] },
  { name: 'diff', classes: ['gd', 'gi'] },
];

const AA = 4.5;

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN contrast-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL contrast-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN contrast-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED contrast-contract: ${message}`); };

// A mutation edits what the browser receives, never what the probe believes.
// Appending to a stylesheet lands the rule outside the layer the product's own
// sheet declares, so it outranks the rules it is standing in for without
// needing an importance flag to do it.
const weakenStylesheet = (asset, rule) => async (context) => {
  let requests = 0;
  await context.route(`**/static/${asset}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (requests >= 1 ? '' : `${asset} was never requested, so the rule reached no page`);
};

// Rewrites the served document, which is how a server that stopped honouring
// the theme cookie would look from the outside. replaceAll, so a body that
// changed is a body wholly changed: a first-occurrence rewrite would leave a
// second copy still saying dark and the probe would blame the page.
const serveLightRoot = () => async (context) => {
  let rewrites = 0;
  await context.route('**/notes/**', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = original.replaceAll('data-theme="dark"', 'data-theme="light"');
    if (body !== original) rewrites += 1;
    await route.fulfill({ response, body });
  });
  return () => (rewrites >= 1 ? '' : 'no served document carried data-theme="dark" to rewrite');
};

// Each mode names the browsing context it edits, so a mutation can only be
// reached by the flow whose assertion it aims at. Without that, one regression
// injected globally would fire at whichever site the run happened to reach
// first and the marker would certify the wrong lock.
const MUTATIONS = {
  'weaken-light-faint-text': {
    target: 'light-text-aa',
    contexts: ['text'],
    apply: weakenStylesheet('app.css', ':root,[data-theme="light"]{--fg-faint:oklch(0.585 0.018 107)}'),
  },
  'weaken-dark-faint-text': {
    target: 'dark-text-aa',
    contexts: ['text'],
    apply: weakenStylesheet('app.css', '[data-theme="dark"]{--fg-faint:oklch(0.540 0.012 72)}'),
  },
  'weaken-light-code-comment': {
    target: 'light-code-aa',
    contexts: ['code'],
    apply: weakenStylesheet('chroma.css', '.chroma .c,.chroma .c1{color:#e4e4e0}'),
  },
  // The named regression this whole file was extended for, in the two shapes
  // it takes. Wholesale, the dark scope serves the light palette and no dark
  // colour reaches the page at all. Partially — the shape that actually
  // happened — only the tokens the dark palette leaves to the body colour keep
  // the light ink, so keywords and strings look perfectly correct while
  // identifiers and punctuation, most of the characters on a line, are
  // near-black on a near-black panel. One mode aims at each, because a lock
  // that only ever fired on the loud version would pass the quiet one.
  'dark-scope-uses-light-palette': {
    target: 'dark-code-aa',
    contexts: ['code'],
    apply: weakenStylesheet('chroma.css', ':root[data-theme="dark"] .chroma span{color:#1f2328}'),
  },
  'dark-scope-leaks-light-identifiers': {
    target: 'dark-code-aa',
    contexts: ['code'],
    apply: weakenStylesheet('chroma.css', ':root[data-theme="dark"] .chroma .nx,:root[data-theme="dark"] .chroma .p{color:#1f2328}'),
  },
  'print-keeps-dark-palette': {
    target: 'print-code-light',
    contexts: ['code'],
    apply: weakenStylesheet('chroma.css', '@media print{:root[data-theme="dark"] .chroma span{color:#a5d6ff}}'),
  },
  'strip-forced-colors': {
    target: 'code-forced-colors',
    contexts: ['code'],
    apply: weakenStylesheet('chroma.css', '.chroma span{forced-color-adjust:none}'),
  },
  'serve-light-root': {
    target: 'dark-code-persists',
    contexts: ['nojs'],
    apply: serveLightRoot(),
  },
  // Opacity on the path-line ink, inside the busy region, fades the word
  // against the elevated ground. Opacity on the opaque container would
  // fade the ground with the ink and the ratio would not move. The lock
  // composites ancestor opacity before computing the ratio, which is the
  // only way it can see this class: the token pairs above are opaque.
  'fade-busy-search-ink': {
    target: 'busy-search-aa',
    contexts: ['search'],
    apply: weakenStylesheet('app.css', '.y-searchresults[aria-busy="true"] .y-result__meta{opacity:0.58}'),
  },
  // A mark that inherits faint path-line ink through the gold wash lands
  // under 4.5:1 on every ground the row sits on. The lock composites the
  // translucent background first, and visits hover and busy, not only idle.
  'inherit-mark-ink': {
    target: 'mark-aa',
    contexts: ['search'],
    apply: weakenStylesheet('app.css', '.yomihon mark{color:inherit}'),
  },
  // Horizontal padding splits a stemmed word from its highlighted head.
  'pad-mark-inline': {
    target: 'mark-padding',
    contexts: ['search'],
    apply: weakenStylesheet('app.css', '.yomihon mark{padding:0 .15rem}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`contrast-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`contrast-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`contrast-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
const proofs = new Map();

const openContext = async (browser, label, options = {}) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, ...options });
  if (mutation?.contexts.includes(label)) proofs.set(label, await mutation.apply(context));
  return context;
};

// Only the targeted flow proves application. A mode aimed elsewhere has
// nothing to have applied here, and asking would report not-applied for a
// mutation that is working perfectly somewhere else.
const proveApplied = (site, label) => {
  if (mutation?.target !== site) return;
  const proof = proofs.get(label);
  if (!proof) notApplied(`${MUTATE}: no proof was recorded for the ${label} flow`);
  const issue = proof();
  if (issue) notApplied(`${MUTATE}: ${issue}`);
};

const measureTheme = (page, theme) => page.evaluate((selectedTheme) => {
  document.documentElement.dataset.theme = selectedTheme;
  const root = getComputedStyle(document.documentElement);
  const textTokens = ['--fg', '--fg-muted', '--fg-subtle', '--fg-faint'];
  const surfaces = ['--bg', '--panel', '--elevated', '--overlay'];
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) return { issue: 'a 2D canvas context is unavailable' };

  const rgb = (property) => {
    const value = root.getPropertyValue(property).trim();
    if (!value) return { issue: `${property} is empty` };
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = value;
    context.fillRect(0, 0, 1, 1);
    const pixel = context.getImageData(0, 0, 1, 1).data;
    if (pixel[3] !== 255) return { issue: `${property} rasterized with alpha ${pixel[3]}` };
    return { value, channels: [pixel[0], pixel[1], pixel[2]] };
  };
  const luminance = (channels) => {
    const linear = channels.map((channel) => {
      const value = channel / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };

  const measurements = [];
  for (const text of textTokens) {
    const foreground = rgb(text);
    if (foreground.issue) return { issue: foreground.issue };
    for (const surface of surfaces) {
      const background = rgb(surface);
      if (background.issue) return { issue: background.issue };
      const a = luminance(foreground.channels);
      const b = luminance(background.channels);
      const ratio = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
      measurements.push({ text, surface, ratio, foreground: foreground.value, background: background.value });
    }
  }
  return { measurements };
}, theme);

// Reads every highlighted word the page is actually painting, with the colour
// behind it found by walking outward until something opaque is reached — the
// panel a code block sits on, in practice, which is not the colour the palette
// was designed against. A span holding only whitespace is skipped: it has a
// colour and nothing to show in it.
const measureCode = (page) => page.evaluate(() => {
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) return { issue: 'a 2D canvas context is unavailable' };

  // An unparseable colour leaves fillStyle at whatever it already held, so a
  // sentinel is the only way to tell "this colour" from "the previous one".
  const SENTINEL = '#ff00ff';
  const raster = (value) => {
    context.fillStyle = SENTINEL;
    context.fillStyle = value;
    if (context.fillStyle === SENTINEL) return { issue: `the browser could not rasterize the colour ${value}` };
    context.clearRect(0, 0, 1, 1);
    context.fillRect(0, 0, 1, 1);
    const pixel = context.getImageData(0, 0, 1, 1).data;
    const hex = `#${[pixel[0], pixel[1], pixel[2]].map((c) => c.toString(16).padStart(2, '0')).join('')}`;
    return { channels: [pixel[0], pixel[1], pixel[2]], alpha: pixel[3], hex };
  };
  const luminance = (channels) => {
    const linear = channels.map((channel) => {
      const value = channel / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };

  const measurements = [];
  for (const span of document.querySelectorAll('.chroma span')) {
    const own = [...span.childNodes].filter((node) => node.nodeType === Node.TEXT_NODE).map((node) => node.textContent).join('');
    if (!own.trim()) continue;

    const style = getComputedStyle(span);
    const foreground = raster(style.color);
    if (foreground.issue) return { issue: foreground.issue };
    if (foreground.alpha !== 255) return { issue: `a token's colour rasterized with alpha ${foreground.alpha}` };

    let background = null;
    for (let el = span; el; el = el.parentElement) {
      const painted = raster(getComputedStyle(el).backgroundColor);
      if (painted.issue) return { issue: painted.issue };
      if (painted.alpha === 255) {
        background = painted;
        break;
      }
    }
    if (!background) return { issue: `nothing opaque was found behind the token ${JSON.stringify(own.trim().slice(0, 24))}` };

    const a = luminance(foreground.channels);
    const b = luminance(background.channels);
    measurements.push({
      classes: [...span.classList],
      text: own.trim().slice(0, 24),
      color: foreground.hex,
      background: background.hex,
      forcedColorAdjust: style.forcedColorAdjust,
      ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
    });
  }
  return { measurements };
});

// The busy result region used to fade every descendant word. This reading
// multiplies ancestor opacities — only until the first opaque surface, which
// already includes the faded group — and composites the ink through that
// product onto that surface. Walking past the opaque ground would fade the
// ink twice and compare it to an un-faded panel, a false red. That walk is
// the only way a token-pair probe can see
// `.y-searchresults[aria-busy="true"] { opacity: … }`.
const measureBusySearch = (page, theme) => page.evaluate((selectedTheme) => {
  document.documentElement.dataset.theme = selectedTheme;
  const host = document.querySelector('.yomihon');
  if (!host) return { issue: 'the shell .yomihon was not on the page' };
  const fixture = document.createElement('div');
  fixture.className = 'y-searchresults';
  fixture.setAttribute('aria-busy', 'true');
  fixture.innerHTML = '<ol class="y-results" role="list"><li><a class="y-result" href="#"><span class="y-result__title">Alpha</span><span class="y-result__meta"><mark>Goroutine</mark>s.md</span></a></li></ol>';
  host.appendChild(fixture);
  const planted = { fixture };

  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) {
    planted.fixture.remove();
    return { issue: 'a 2D canvas context is unavailable' };
  }
  const SENTINEL = '#ff00ff';
  const raster = (value) => {
    context.fillStyle = SENTINEL;
    context.fillStyle = value;
    if (context.fillStyle === SENTINEL) return { issue: `the browser could not rasterize the colour ${value}` };
    context.clearRect(0, 0, 1, 1);
    context.fillRect(0, 0, 1, 1);
    const pixel = context.getImageData(0, 0, 1, 1).data;
    return { channels: [pixel[0], pixel[1], pixel[2]], alpha: pixel[3] };
  };
  const luminance = (channels) => {
    const linear = channels.map((channel) => {
      const value = channel / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };

  try {
    const ink = planted.fixture.querySelector('.y-result__meta');
    if (!ink) return { issue: 'the busy fixture had no path-line ink to measure' };
    const foreground = raster(getComputedStyle(ink).color);
    if (foreground.issue) return { issue: foreground.issue };
    if (foreground.alpha !== 255) return { issue: `busy-search ink rasterized with alpha ${foreground.alpha}` };

    let opacity = 1;
    let background = null;
    for (let el = ink; el; el = el.parentElement) {
      const style = getComputedStyle(el);
      const painted = raster(style.backgroundColor);
      if (painted.issue) return { issue: painted.issue };
      if (painted.alpha === 255) {
        background = painted;
        break;
      }
      const own = Number.parseFloat(style.opacity);
      if (Number.isNaN(own)) return { issue: `an ancestor opacity was not a number (${style.opacity})` };
      opacity *= own;
    }
    if (!background) return { issue: 'nothing opaque was found behind the busy path-line ink' };

    const faded = foreground.channels.map((channel, index) => (
      Math.round(channel * opacity + background.channels[index] * (1 - opacity))
    ));
    const a = luminance(faded);
    const b = luminance(background.channels);
    return {
      opacity,
      ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
      text: '--fg-faint through ancestor opacity',
      color: `rgb(${faded.join(' ')})`,
      background: `rgb(${background.channels.join(' ')})`,
    };
  } finally {
    planted.fixture.remove();
  }
}, theme);

// A mark's gold wash is translucent. Measuring the token pair without
// compositing that wash over the surface is how a 4.26:1 path-line hit
// shipped as a passing token. The wash lets the row's ground through, so
// the idle page is the most favourable reading; a hovered row (--overlay)
// and the busy container (--elevated) are darker — the grounds that
// stayed under 4.5:1 when the idle page cleared it. 11px normal text
// still owes 4.5:1 on each.
const SEARCH_MARK_HTML = '<ol class="y-results" role="list"><li><a class="y-result" href="#"><span class="y-result__title">Alpha</span><span class="y-result__meta"><mark>Goroutine</mark>s.md</span></a></li></ol>';
const MARK_GROUNDS = [
  { name: 'hover', busy: false, hover: true },
  { name: 'busy', busy: true, hover: false },
  { name: 'idle', busy: false, hover: false },
];

const plantSearchFixture = (page, theme, { busy }) => page.evaluate(({ selectedTheme, busy: isBusy, html }) => {
  document.documentElement.dataset.theme = selectedTheme;
  document.getElementById('contrast-search-fixture')?.remove();
  const host = document.querySelector('.yomihon');
  if (!host) return { issue: 'the shell .yomihon was not on the page' };
  const fixture = document.createElement('div');
  fixture.id = 'contrast-search-fixture';
  fixture.className = 'y-searchresults';
  fixture.setAttribute('aria-busy', isBusy ? 'true' : 'false');
  // Placed above the in-flow seal bar so a real :hover can land on the row.
  // Position does not change the computed wash or the ancestor walk.
  fixture.style.cssText = 'position:fixed;top:24px;left:24px;z-index:2147483647;width:min(480px,90vw)';
  fixture.innerHTML = html;
  host.appendChild(fixture);
  return {};
}, { selectedTheme: theme, busy, html: SEARCH_MARK_HTML });

const removeSearchFixture = (page) => page.evaluate(() => {
  document.getElementById('contrast-search-fixture')?.remove();
});

const measurePlantedMark = (page, groundName) => page.evaluate((selectedGround) => {
  const fixture = document.getElementById('contrast-search-fixture');
  if (!fixture) return { issue: 'the search fixture was not on the page' };

  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) return { issue: 'a 2D canvas context is unavailable' };
  const SENTINEL = '#ff00ff';
  const raster = (value) => {
    context.fillStyle = SENTINEL;
    context.fillStyle = value;
    if (context.fillStyle === SENTINEL) return { issue: `the browser could not rasterize the colour ${value}` };
    context.clearRect(0, 0, 1, 1);
    context.fillRect(0, 0, 1, 1);
    const pixel = context.getImageData(0, 0, 1, 1).data;
    return { channels: [pixel[0], pixel[1], pixel[2]], alpha: pixel[3] };
  };
  const luminance = (channels) => {
    const linear = channels.map((channel) => {
      const value = channel / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };

  const mark = fixture.querySelector('mark');
  if (!mark) return { issue: 'the fixture had no mark to measure' };
  if (selectedGround === 'hover') {
    const row = mark.closest('.y-result');
    if (!row) return { issue: 'the hovered fixture had no result row' };
    const rowBg = raster(getComputedStyle(row).backgroundColor);
    if (rowBg.issue) return { issue: rowBg.issue };
    if (rowBg.alpha !== 255) return { issue: 'the hovered result has no opaque overlay, so this reading is not a hover' };
  }
  if (selectedGround === 'busy') {
    const regionBg = raster(getComputedStyle(fixture).backgroundColor);
    if (regionBg.issue) return { issue: regionBg.issue };
    if (regionBg.alpha !== 255) return { issue: 'the busy container has no opaque ground' };
  }
  const style = getComputedStyle(mark);
  const foreground = raster(style.color);
  if (foreground.issue) return { issue: foreground.issue };
  if (foreground.alpha !== 255) return { issue: `a mark's colour rasterized with alpha ${foreground.alpha}` };
  const wash = raster(style.backgroundColor);
  if (wash.issue) return { issue: wash.issue };

  let surface = null;
  for (let el = mark.parentElement; el; el = el.parentElement) {
    const painted = raster(getComputedStyle(el).backgroundColor);
    if (painted.issue) return { issue: painted.issue };
    if (painted.alpha === 255) {
      surface = painted;
      break;
    }
  }
  if (!surface) return { issue: 'nothing opaque was found behind the mark' };

  const cover = wash.alpha / 255;
  const composited = wash.channels.map((channel, index) => (
    Math.round(channel * cover + surface.channels[index] * (1 - cover))
  ));
  const a = luminance(foreground.channels);
  const b = luminance(composited);
  return {
    ground: selectedGround,
    ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
    text: mark.textContent,
    color: `rgb(${foreground.channels.join(' ')})`,
    background: `rgb(${composited.join(' ')})`,
    paddingInlineStart: style.paddingInlineStart,
    paddingInlineEnd: style.paddingInlineEnd,
  };
}, groundName);

const measureMark = async (page, theme, ground) => {
  const planted = await plantSearchFixture(page, theme, { busy: ground.busy });
  if (planted.issue) return planted;
  try {
    if (ground.hover) {
      await page.locator('#contrast-search-fixture .y-result').hover();
      const hovered = await page.waitForFunction(() => {
        const row = document.querySelector('#contrast-search-fixture .y-result');
        if (!row || !row.matches(':hover')) return false;
        const canvas = document.createElement('canvas');
        canvas.width = 1;
        canvas.height = 1;
        const context = canvas.getContext('2d', { willReadFrequently: true });
        if (!context) return false;
        context.fillStyle = getComputedStyle(row).backgroundColor;
        context.fillRect(0, 0, 1, 1);
        return context.getImageData(0, 0, 1, 1).data[3] === 255;
      }).then(() => true, () => false);
      if (!hovered) return { issue: 'the hovered result never took an opaque overlay' };
    }
    return await measurePlantedMark(page, ground.name);
  } finally {
    await removeSearchFixture(page);
  }
};

// Gathers one theme's readings from every page code appears on, so a palette
// that is right in prose and wrong on a whole-file view cannot pass.
const readCode = async (page, paths) => {
  const measurements = [];
  for (const path of paths) {
    const response = await page.goto(BASE + path, { waitUntil: 'networkidle' });
    if (!response || response.status() !== 200) broken(`${path} returned ${response?.status() ?? 'no response'}, want 200`);
    const result = await measureCode(page);
    if (result.issue) broken(`${path}: ${result.issue}`);
    measurements.push(...result.measurements.map((m) => ({ ...m, path })));
  }
  return measurements;
};

const requireCoverage = (measurements, what) => {
  if (measurements.length === 0) broken(`${what}: no highlighted word was measured at all`);
  for (const kind of REQUIRED_TOKENS) {
    if (!measurements.some((m) => m.classes.some((c) => kind.classes.includes(c)))) {
      broken(`${what}: no ${kind.name} token was on any measured page, so this reading proves nothing about one`);
    }
  }
};

const requireAA = (site, measurements, what) => {
  const weakest = measurements.reduce((left, right) => (left.ratio < right.ratio ? left : right));
  if (weakest.ratio < AA) {
    fail(site, `${what}: ${JSON.stringify(weakest.text)} (.${weakest.classes.join('.')}) on ${weakest.path} is ${weakest.ratio.toFixed(3)}:1 (${weakest.color} on ${weakest.background}), want at least ${AA}:1`);
  }
};

const themeAttribute = (page) => page.evaluate(() => document.documentElement.getAttribute('data-theme'));

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  // The interface's own text tokens, on every surface they are set against.
  {
    const context = await openContext(browser, 'text');
    const page = await context.newPage();
    const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);
    for (const theme of ['light', 'dark']) {
      proveApplied(`${theme}-text-aa`, 'text');
      const result = await measureTheme(page, theme);
      if (result.issue) broken(result.issue);
      const weakest = result.measurements.reduce((left, right) => (left.ratio < right.ratio ? left : right));
      if (weakest.ratio < AA) {
        fail(`${theme}-text-aa`, `${weakest.text} on ${weakest.surface} is ${weakest.ratio.toFixed(3)}:1 (${weakest.foreground} on ${weakest.background}), want at least ${AA}:1`);
      }
    }
    await context.close();
  }

  // Busy search ink through ancestor opacity, and a mark on the path line
  // with its wash composited over the surface it actually sits on — idle,
  // hovered, and aria-busy. Both wear the production classes; the fixture
  // is how the probe sees them without racing a query.
  {
    const context = await openContext(browser, 'search');
    const page = await context.newPage();
    const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (!response || response.status() !== 200) broken(`search fixture navigation returned ${response?.status() ?? 'no response'}, want 200`);

    proveApplied('busy-search-aa', 'search');
    for (const theme of ['light', 'dark']) {
      const result = await measureBusySearch(page, theme);
      if (result.issue) broken(result.issue);
      if (result.ratio < AA) {
        fail('busy-search-aa', `busy search ${theme}: ${result.text} is ${result.ratio.toFixed(3)}:1 at opacity ${result.opacity.toFixed(2)} (${result.color} on ${result.background}), want at least ${AA}:1`);
      }
    }

    proveApplied('mark-aa', 'search');
    proveApplied('mark-padding', 'search');
    for (const theme of ['light', 'dark']) {
      const readings = [];
      for (const ground of MARK_GROUNDS) {
        const result = await measureMark(page, theme, ground);
        if (result.issue) broken(`mark ${theme} ${ground.name}: ${result.issue}`);
        readings.push(result);
      }
      const weakest = readings.reduce((left, right) => (left.ratio < right.ratio ? left : right));
      if (weakest.ratio < AA) {
        fail('mark-aa', `mark ${theme} ${weakest.ground}: ${JSON.stringify(weakest.text)} is ${weakest.ratio.toFixed(3)}:1 (${weakest.color} on ${weakest.background}), want at least ${AA}:1`);
      }
      for (const result of readings) {
        const start = Number.parseFloat(result.paddingInlineStart);
        const end = Number.parseFloat(result.paddingInlineEnd);
        if (start !== 0 || end !== 0) {
          fail('mark-padding', `mark ${theme} ${result.ground} has padding-inline ${result.paddingInlineStart} ${result.paddingInlineEnd}, want 0`);
        }
      }
    }
    await context.close();
  }

  // Code, in both themes, reached the way a reader reaches them.
  {
    const context = await openContext(browser, 'code');
    const page = await context.newPage();

    const light = await readCode(page, [PAGE, SOURCE_PAGE]);
    proveApplied('light-code-aa', 'code');
    requireCoverage(light, 'light mode');
    requireAA('light-code-aa', light, 'light mode');

    // The reader's own control, not a scripted attribute: what is under test
    // includes the button reaching the stylesheet at all.
    await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    await page.click('[data-theme-toggle]');
    await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark');

    const dark = await readCode(page, [PAGE, SOURCE_PAGE]);
    proveApplied('dark-code-aa', 'code');
    // The cookie the button wrote has to survive two navigations, or the
    // readings below are of a light page and say nothing about dark.
    if (await themeAttribute(page) !== 'dark') {
      fail('dark-code-aa', 'the theme the toggle chose did not survive navigation, so no dark reading was taken');
    }
    if (!dark.some((m) => m.color === DARK_KEYWORD)) {
      fail('dark-code-aa', `no measured token carried the dark palette's keyword colour ${DARK_KEYWORD}; the light palette is still painting this page`);
    }
    requireCoverage(dark, 'dark mode');
    requireAA('dark-code-aa', dark, 'dark mode');

    // Paper, from a dark screen. The light rules are what is left once the
    // dark ones are held back, so the printed page carries syntax colour
    // rather than either bright ink or nothing at all.
    await page.emulateMedia({ media: 'print' });
    const printed = await readCode(page, [PAGE]);
    proveApplied('print-code-light', 'code');
    requireCoverage(printed, 'printing from dark mode');
    if (printed.some((m) => m.color === DARK_KEYWORD)) {
      fail('print-code-light', `the dark palette's keyword colour ${DARK_KEYWORD} reached paper; printing from dark mode must fall back to the light rules`);
    }
    requireAA('print-code-light', printed, 'printing from dark mode');
    await page.emulateMedia({ media: 'screen' });

    // Forced colours belong to the reader. Code is text, and a sheet that
    // opted it out would hand back the palette they switched away from.
    await page.emulateMedia({ forcedColors: 'active' });
    const forced = await readCode(page, [PAGE]);
    proveApplied('code-forced-colors', 'code');
    requireCoverage(forced, 'forced colours');
    const optedOut = forced.find((m) => m.forcedColorAdjust === 'none');
    if (optedOut) {
      fail('code-forced-colors', `the token .${optedOut.classes.join('.')} sets forced-color-adjust: none, so the browser cannot take its colour over`);
    }
    await page.emulateMedia({ forcedColors: 'none' });

    const cookies = await context.cookies();
    await context.close();

    // The same choice, with no script to make it: the server stamps the theme
    // from the cookie, so the first paint is already dark and stays readable.
    const quiet = await openContext(browser, 'nojs', { javaScriptEnabled: false });
    await quiet.addCookies(cookies);
    const quietPage = await quiet.newPage();
    const served = await readCode(quietPage, [PAGE]);
    proveApplied('dark-code-persists', 'nojs');
    const stamped = await themeAttribute(quietPage);
    if (stamped !== 'dark') {
      fail('dark-code-persists', `with no script running the served page carried data-theme=${JSON.stringify(stamped)}, want "dark": the theme has to come from the server or the first paint is the wrong one`);
    }
    requireCoverage(served, 'dark mode without JavaScript');
    if (!served.some((m) => m.color === DARK_KEYWORD)) {
      fail('dark-code-persists', `with no script running no token carried the dark palette's keyword colour ${DARK_KEYWORD}`);
    }
    requireAA('dark-code-persists', served, 'dark mode without JavaScript');
    await quiet.close();
  }

  console.log('PASS contrast-contract: every text token, every highlighted word, busy-search ink through ancestor opacity, and every mark on the path line — idle, hovered, and busy — clears 4.5:1 in both themes, on screen, on paper, and with no script running');
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
