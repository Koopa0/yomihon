// Browser lock for the selected Yomihon identity. One passive, multicolour SVG
// is the geometry authority for the favicon and header. The most important
// semantic boundary is rendered, not inferred from coordinates: the
// publication obi may occupy the black cover but must never paint the warm
// page fore-edge.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
const BRAND_PATH = '/static/yomihon-mark.svg';
const APP_ORIGIN = new URL(BASE).origin;
const SITES = [
  'projection-source',
  'decorative-mark',
  'accessible-name',
  'local-only',
  'cover-render',
  'pages-render',
  'obi-render',
  'page-continuity',
  'obi-page-separation',
  'theme-mark',
  'forced-colors-name',
  'header-fit',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN brand-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL brand-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN brand-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED brand-contract: ${message}`); };

const replacePartPath = (source, part, replacement) => {
  const pattern = new RegExp(`(<path\\s+data-brand-part="${part}"\\s+fill="[^"]+"\\s+d=")[^"]+("/>)`, 'g');
  const matches = [...source.matchAll(pattern)];
  if (matches.length !== 1) return { source, count: matches.length };
  return { source: source.replace(pattern, `$1${replacement}$2`), count: 1 };
};

const rewriteSVGPart = (part, replacement, target) => ({
  target,
  before: async (page, state) => {
    await page.route(`**${BRAND_PATH}`, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      const rewritten = replacePartPath(original, part, replacement);
      state.svgRouteCalls += 1;
      state.svgRewriteMatches += rewritten.count;
      state.servedSVG = rewritten.source;
      await route.fulfill({ response, body: rewritten.source });
    });
    return () => {
      if (state.svgRouteCalls === 0) return 'the canonical SVG was never requested';
      if (state.svgRewriteMatches !== state.svgRouteCalls) {
        return `the ${part} path matched ${state.svgRewriteMatches} times across ${state.svgRouteCalls} SVG responses`;
      }
      return '';
    };
  },
});

const changeOne = async (page, selector, change) => {
  const changed = await page.evaluate(({ selector: selected, change: operation }) => {
    const elements = [...document.querySelectorAll(selected)];
    if (elements.length !== 1) return elements.length;
    const element = elements[0];
    if (operation.kind === 'attribute') element.setAttribute(operation.name, operation.value);
    if (operation.kind === 'remove-attribute') element.removeAttribute(operation.name);
    if (operation.kind === 'text') element.textContent = operation.value;
    return 1;
  }, { selector, change });
  if (changed !== 1) notApplied(`${selector} matched ${changed} elements, want exactly 1`);
};

const injectRule = async (page, selector, declarations) => {
  const matches = await page.locator(selector).count();
  if (matches === 0) notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  await page.addStyleTag({ content: `${selector} { ${declarations} }` });
};

const MUTATIONS = {
  'obi-crosses-pages': rewriteSVGPart('obi', 'M5 17 28 8v20L5 22Z', 'obi-page-separation'),
  'break-page-fore-edge': rewriteSVGPart('pages', 'M8.2 3.1 27.5 8.1 25.5 9.6 6.6 4.7Z', 'page-continuity'),
  'collapse-cover': rewriteSVGPart('cover', 'M0 0Z', 'cover-render'),
  'collapse-pages': rewriteSVGPart('pages', 'M0 0Z', 'pages-render'),
  'collapse-obi': rewriteSVGPart('obi', 'M0 0Z', 'obi-render'),
  'drift-header-source': {
    target: 'projection-source',
    after: (page) => changeOne(page, '.y-brand__mark', { kind: 'attribute', name: 'src', value: '/static/app.css' }),
  },
  'drift-favicon-source': {
    target: 'projection-source',
    after: (page) => changeOne(page, 'link[rel="icon"]', { kind: 'attribute', name: 'href', value: '/static/app.css' }),
  },
  'expose-decorative-mark': {
    target: 'decorative-mark',
    after: async (page) => {
      await changeOne(page, '.y-brand__mark', { kind: 'remove-attribute', name: 'aria-hidden' });
      await changeOne(page, '.y-brand__mark', { kind: 'attribute', name: 'alt', value: 'book' });
    },
  },
  'rename-wordmark': {
    target: 'accessible-name',
    after: (page) => changeOne(page, '.y-brand__name > span', { kind: 'text', value: 'Yomihon' }),
  },
  'add-external-brand-request': {
    target: 'local-only',
    after: async (page) => {
      await page.route('https://identity.invalid/**', (route) => route.fulfill({
        status: 200,
        contentType: 'image/png',
        body: '',
      }));
      const requested = page.waitForRequest((request) => request.url() === 'https://identity.invalid/mark.png');
      await page.evaluate(() => {
        const image = document.createElement('img');
        image.src = 'https://identity.invalid/mark.png';
        image.alt = '';
        document.body.append(image);
      });
      await requested;
    },
  },
  'hide-mark-dark': {
    target: 'theme-mark',
    after: async (page) => {
      if (await page.locator('.y-brand__mark').count() !== 1) notApplied('the header mark is not unique');
      await page.addStyleTag({ content: '[data-theme="dark"] .y-brand__mark { display: none !important; }' });
    },
  },
  'hide-wordmark-forced-colors': {
    target: 'forced-colors-name',
    after: async (page) => {
      if (await page.locator('.y-brand__name > span').count() !== 1) {
        notApplied('the visible wordmark span is not unique');
      }
      await page.addStyleTag({ content: '@media (forced-colors: active) { .y-brand__name > span { display: none !important; } }' });
    },
  },
  'overflow-header': {
    target: 'header-fit',
    after: (page) => injectRule(page, '.y-header', 'min-width: 500px !important;'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`brand-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`brand-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`brand-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const canonicalResponse = await fetch(BASE + BRAND_PATH);
if (canonicalResponse.status !== 200) {
  console.error(`BROKEN brand-contract: ${BRAND_PATH} returned ${canonicalResponse.status}, want 200`);
  process.exit(1);
}
if (!(canonicalResponse.headers.get('content-type') || '').toLowerCase().startsWith('image/svg+xml')) {
  console.error(`BROKEN brand-contract: ${BRAND_PATH} has content type ${canonicalResponse.headers.get('content-type')}`);
  process.exit(1);
}
const canonicalSVG = await canonicalResponse.text();

const analyseSVG = async (source) => {
  const parser = new DOMParser();
  const svgDocument = parser.parseFromString(source, 'image/svg+xml');
  if (svgDocument.querySelector('parsererror')) return { issue: 'the served SVG cannot be parsed' };
  const root = svgDocument.documentElement;
  const paths = [...root.children];
  if (paths.length !== 3 || paths.some((element) => element.localName !== 'path')) {
    return { issue: `the served SVG has ${paths.length} direct children, want three paths` };
  }
  const byPart = Object.fromEntries(paths.map((path) => [path.getAttribute('data-brand-part'), path]));
  for (const part of ['cover', 'pages', 'obi']) {
    if (!byPart[part]) return { issue: `the served SVG has no ${part} path` };
  }

  const loadPixels = (svg, size) => new Promise((resolve) => {
    const image = new Image();
    image.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const context = canvas.getContext('2d', { willReadFrequently: true });
      if (!context) {
        resolve({ issue: 'a 2D canvas context is unavailable' });
        return;
      }
      context.clearRect(0, 0, size, size);
      context.drawImage(image, 0, 0, size, size);
      resolve({ pixels: context.getImageData(0, 0, size, size).data, size });
    };
    image.onerror = () => resolve({ issue: `an SVG failed to render at ${size}px` });
    image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  });
  const color = (hex) => [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16));
  const palettes = {
    cover: color('#0F0F0F'),
    pages: color('#F5F1E6'),
    obi: color('#D62A0F'),
  };
  const near = (pixels, wanted) => {
    let count = 0;
    for (let i = 0; i < pixels.length; i += 4) {
      if (pixels[i + 3] < 128) continue;
      const distance = Math.hypot(pixels[i] - wanted[0], pixels[i + 1] - wanted[1], pixels[i + 2] - wanted[2]);
      // At 16px the one-pixel diagonal fore-edge is necessarily blended with
      // the surrounding ink. The source palette is locked exactly by the SVG
      // grammar test; this rendered lock asks whether each part still leaves a
      // perceptually attributable pixel after real browser antialiasing.
      if (distance <= 120) count += 1;
    }
    return count;
  };

  const rendered = {};
  for (const size of [16, 24, 32, 180]) {
    const result = await loadPixels(source, size);
    if (result.issue) return result;
    rendered[size] = Object.fromEntries(Object.entries(palettes).map(([part, wanted]) => [part, near(result.pixels, wanted)]));
  }

  const partSVG = (part) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path fill="${byPart[part].getAttribute('fill')}" d="${byPart[part].getAttribute('d')}"/></svg>`;
  const pageLayer = await loadPixels(partSVG('pages'), 256);
  const obiLayer = await loadPixels(partSVG('obi'), 256);
  if (pageLayer.issue) return pageLayer;
  if (obiLayer.issue) return obiLayer;

  const threshold = (pixels, index) => pixels[index * 4 + 3] >= 128;
  const pageMask = new Uint8Array(256 * 256);
  let overlap = 0;
  for (let index = 0; index < pageMask.length; index += 1) {
    if (threshold(pageLayer.pixels, index)) pageMask[index] = 1;
    if (threshold(pageLayer.pixels, index) && threshold(obiLayer.pixels, index)) overlap += 1;
  }

  let components = 0;
  let pagePixels = 0;
  let minX = 256;
  let maxX = -1;
  let minY = 256;
  let maxY = -1;
  const queue = new Int32Array(pageMask.length);
  for (let start = 0; start < pageMask.length; start += 1) {
    if (pageMask[start] !== 1) continue;
    components += 1;
    let head = 0;
    let tail = 0;
    queue[tail++] = start;
    pageMask[start] = 2;
    while (head < tail) {
      const index = queue[head++];
      pagePixels += 1;
      const x = index % 256;
      const y = Math.floor(index / 256);
      minX = Math.min(minX, x);
      maxX = Math.max(maxX, x);
      minY = Math.min(minY, y);
      maxY = Math.max(maxY, y);
      for (let dy = -1; dy <= 1; dy += 1) {
        for (let dx = -1; dx <= 1; dx += 1) {
          if (dx === 0 && dy === 0) continue;
          const nx = x + dx;
          const ny = y + dy;
          if (nx < 0 || nx >= 256 || ny < 0 || ny >= 256) continue;
          const next = ny * 256 + nx;
          if (pageMask[next] !== 1) continue;
          pageMask[next] = 2;
          queue[tail++] = next;
        }
      }
    }
  }

  return {
    rendered,
    overlap,
    page: {
      components,
      pixels: pagePixels,
      width: maxX >= minX ? maxX - minX + 1 : 0,
      height: maxY >= minY ? maxY - minY + 1 : 0,
    },
  };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const state = {
  servedSVG: canonicalSVG,
  svgRouteCalls: 0,
  svgRewriteMatches: 0,
  brandRequests: [],
};
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.origin !== APP_ORIGIN || url.pathname === BRAND_PATH) state.brandRequests.push(url.href);
  });

  let mutationProof = null;
  if (MUTATE && MUTATIONS[MUTATE].before) mutationProof = await MUTATIONS[MUTATE].before(page, state);
  const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);
  if (MUTATE && MUTATIONS[MUTATE].after) await MUTATIONS[MUTATE].after(page, state);
  await page.waitForTimeout(50);
  if (mutationProof) {
    const issue = mutationProof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const projection = await page.evaluate(() => {
    const icons = [...document.querySelectorAll('link[rel="icon"]')];
    const marks = [...document.querySelectorAll('.y-brand__mark')];
    const links = [...document.querySelectorAll('a.y-brand__name')];
    const icon = icons[0];
    const mark = marks[0];
    const link = links[0];
    return {
      iconCount: icons.length,
      markCount: marks.length,
      linkCount: links.length,
      iconSource: icon?.getAttribute('href') || '',
      markSource: mark?.getAttribute('src') || '',
      markAlt: mark?.getAttribute('alt'),
      markHidden: mark?.getAttribute('aria-hidden'),
      linkName: link?.textContent?.trim() || '',
      linkAriaLabel: link?.getAttribute('aria-label'),
    };
  });
  if (projection.iconCount !== 1 || projection.markCount !== 1 || projection.linkCount !== 1 ||
      projection.iconSource !== BRAND_PATH || projection.markSource !== BRAND_PATH) {
    fail('projection-source', `favicon/header projections are ${JSON.stringify(projection)}`);
  }
  if (projection.markAlt !== '' || projection.markHidden !== 'true') {
    fail('decorative-mark', `header mark alt/aria-hidden = ${JSON.stringify([projection.markAlt, projection.markHidden])}`);
  }
  if (projection.linkName !== 'yomihon' || projection.linkAriaLabel !== null) {
    fail('accessible-name', `brand link text/aria-label = ${JSON.stringify([projection.linkName, projection.linkAriaLabel])}`);
  }

  const brandURLs = [...new Set(state.brandRequests)];
  if (brandURLs.length !== 1 || new URL(brandURLs[0]).origin !== new URL(BASE).origin || new URL(brandURLs[0]).pathname !== BRAND_PATH) {
    fail('local-only', `brand requests = ${JSON.stringify(brandURLs)}, want only ${BASE + BRAND_PATH}`);
  }

  const analysis = await page.evaluate(analyseSVG, state.servedSVG);
  if (analysis.issue) broken(analysis.issue);
  for (const [part, site] of Object.entries({ cover: 'cover-render', pages: 'pages-render', obi: 'obi-render' })) {
    for (const size of [16, 24, 32, 180]) {
      if (analysis.rendered[size][part] < 1) {
        fail(site, `${part} has no approved-colour pixel at ${size}px; counts = ${JSON.stringify(analysis.rendered[size])}`);
      }
    }
  }
  if (analysis.page.components !== 1 || analysis.page.pixels < 100 || analysis.page.height < 150 || analysis.page.width < 140) {
    fail('page-continuity', `page mask = ${JSON.stringify(analysis.page)}, want one continuous top-and-fore-edge region`);
  }
  if (analysis.overlap !== 0) {
    fail('obi-page-separation', `page and obi have ${analysis.overlap} shared interior pixels at 256px`);
  }

  await page.emulateMedia({ forcedColors: 'none' });
  for (const theme of ['light', 'dark']) {
    await page.evaluate((selected) => { document.documentElement.dataset.theme = selected; }, theme);
    const visible = await page.locator('.y-brand__mark').evaluate((element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity) > 0 && rect.width === 24 && rect.height === 24;
    });
    if (!visible) fail('theme-mark', `header mark is not a visible 24px projection in ${theme} theme`);
  }

  await page.emulateMedia({ forcedColors: 'active' });
  const forced = await page.evaluate(() => {
    const mark = document.querySelector('.y-brand__mark');
    const name = document.querySelector('.y-brand__name > span');
    const link = document.querySelector('.y-brand__name');
    const nameStyle = name ? getComputedStyle(name) : null;
    const linkRect = link?.getBoundingClientRect();
    return {
      markDisplay: mark ? getComputedStyle(mark).display : '',
      nameDisplay: nameStyle?.display || '',
      nameVisibility: nameStyle?.visibility || '',
      nameText: name?.textContent?.trim() || '',
      linkWidth: linkRect?.width || 0,
      linkHeight: linkRect?.height || 0,
    };
  });
  if (forced.markDisplay !== 'none' || forced.nameDisplay === 'none' || forced.nameVisibility === 'hidden' ||
      forced.nameText !== 'yomihon' || forced.linkWidth <= 0 || forced.linkHeight <= 0) {
    fail('forced-colors-name', `forced-colours identity = ${JSON.stringify(forced)}`);
  }
  await page.emulateMedia({ forcedColors: 'none' });

  await page.setViewportSize({ width: 360, height: 800 });
  const fit = await page.evaluate(() => {
    const header = document.querySelector('.y-header')?.getBoundingClientRect();
    return {
      height: header?.height || 0,
      left: header?.left || 0,
      right: header?.right || 0,
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    };
  });
  // What must not happen is the page reaching past the width it was given. It
  // may fall short of it: where a scrollbar keeps its own column the header
  // stops at that column's edge, which is the whole of what the reader can
  // see. Demanding the two be equal called that correct behaviour a failure on
  // every machine whose scrollbars take room.
  if (Math.abs(fit.height - 56) > 0.5 || fit.left < -0.5 || fit.right > fit.clientWidth + 0.5 || fit.scrollWidth > fit.clientWidth + 0.5) {
    fail('header-fit', `360px header geometry = ${JSON.stringify(fit)}`);
  }

  console.log(`PASS brand-contract: one canonical mark preserves cover/pages/obi, zero page-band overlap, accessible projection, themes, and 360px fit`);
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else if (error instanceof ProbeBroken) {
    console.error(error.message);
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
