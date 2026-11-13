// Browser lock for the selected Yomihon identity. One passive, two-colour SVG
// is the geometry authority for the favicon and header. What matters most is
// checked by rendering it rather than by reading coordinates: the page stays
// one unbroken face, and the ribbon keeps clear of the page's curved cut
// instead of running into it. The header mark keeps its page shape legible on
// whatever ground each theme gives it, and the light theme gives it none.
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
  'page-render',
  'ribbon-render',
  'page-is-one-face',
  'ribbon-clears-the-page',
  'theme-mark',
  'light-mark-bare',
  'mark-ground',
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
    await page.route(`**${BRAND_PATH}{,?*}`, async (route) => {
      if (new URL(route.request().url()).pathname !== BRAND_PATH) return route.continue();
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
  // The ribbon moved up into the page's curved cut, where it runs into the page.
  'ribbon-onto-the-page': rewriteSVGPart('ribbon', 'M26 49C34 32 40 27 49 28V49Z', 'ribbon-clears-the-page'),
  // A cut across the page, which must read as one unbroken face.
  'split-the-page': rewriteSVGPart('page', 'M10 10H51V20H10ZM10 24H51V25C40 24 34 30 26 49H10Z', 'page-is-one-face'),
  'collapse-page': rewriteSVGPart('page', 'M0 0Z', 'page-render'),
  'collapse-ribbon': rewriteSVGPart('ribbon', 'M0 0Z', 'ribbon-render'),
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
  'tile-in-light': {
    target: 'light-mark-bare',
    after: async (page) => {
      if (await page.locator('.y-brand__mark').count() !== 1) notApplied('the header mark is not unique');
      await page.addStyleTag({ content: ':root[data-theme="light"] .y-brand__mark { background: #F5F1E6 !important; }' });
    },
  },
  'drop-dark-tile': {
    target: 'mark-ground',
    after: (page) => injectRule(page, '.y-brand__mark', 'background: transparent !important;'),
  },
  'drop-system-dark-tile': {
    target: 'mark-ground',
    after: async (page) => {
      if (await page.locator('.y-brand__mark').count() !== 1) notApplied('the header mark is not unique');
      await page.addStyleTag({ content: '@media (prefers-color-scheme: dark) { :root:not([data-theme]) .y-brand__mark { background: transparent !important; } }' });
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
  if (paths.length !== 2 || paths.some((element) => element.localName !== 'path')) {
    return { issue: `the served SVG has ${paths.length} direct children, want two paths` };
  }
  const byPart = Object.fromEntries(paths.map((path) => [path.getAttribute('data-brand-part'), path]));
  for (const part of ['page', 'ribbon']) {
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
    page: color('#282A2D'),
    ribbon: color('#536D85'),
  };
  // The two fills sit close enough that a fixed distance would credit page
  // pixels to the ribbon, so each opaque pixel goes to whichever approved
  // colour it is nearest. The source palette is locked exactly by the SVG
  // grammar test; this rendered lock asks whether each part still leaves an
  // attributable pixel after real browser antialiasing.
  const attribute = (pixels) => {
    const counts = Object.fromEntries(Object.keys(palettes).map((part) => [part, 0]));
    for (let i = 0; i < pixels.length; i += 4) {
      if (pixels[i + 3] < 128) continue;
      let nearest = '';
      let best = Infinity;
      for (const [part, wanted] of Object.entries(palettes)) {
        const distance = Math.hypot(pixels[i] - wanted[0], pixels[i + 1] - wanted[1], pixels[i + 2] - wanted[2]);
        if (distance < best) {
          best = distance;
          nearest = part;
        }
      }
      counts[nearest] += 1;
    }
    return counts;
  };

  const rendered = {};
  for (const size of [16, 24, 32, 180]) {
    const result = await loadPixels(source, size);
    if (result.issue) return result;
    rendered[size] = attribute(result.pixels);
  }

  const partSVG = (part) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><path fill="${byPart[part].getAttribute('fill')}" d="${byPart[part].getAttribute('d')}"/></svg>`;
  const pageLayer = await loadPixels(partSVG('page'), 256);
  const ribbonLayer = await loadPixels(partSVG('ribbon'), 256);
  if (pageLayer.issue) return pageLayer;
  if (ribbonLayer.issue) return ribbonLayer;

  const threshold = (pixels, index) => pixels[index * 4 + 3] >= 128;
  const pageMask = new Uint8Array(256 * 256);
  const ribbonIndexes = [];
  for (let index = 0; index < pageMask.length; index += 1) {
    if (threshold(pageLayer.pixels, index)) pageMask[index] = 1;
    if (threshold(ribbonLayer.pixels, index)) ribbonIndexes.push(index);
  }
  // The ribbon lies along the page's curved cut with a channel of ground
  // between them. The narrowest distance from any ribbon pixel to the page,
  // in 256px pixels, is that channel; a ribbon run into the cut closes it.
  const pageEdge = [];
  for (let index = 0; index < pageMask.length; index += 1) {
    if (pageMask[index] !== 1) continue;
    const x = index % 256;
    const y = Math.floor(index / 256);
    const inside = (nx, ny) => nx >= 0 && nx < 256 && ny >= 0 && ny < 256 && pageMask[ny * 256 + nx] === 1;
    if (!inside(x - 1, y) || !inside(x + 1, y) || !inside(x, y - 1) || !inside(x, y + 1)) pageEdge.push([x, y]);
  }
  let channel = Infinity;
  for (const index of ribbonIndexes) {
    const x = index % 256;
    const y = Math.floor(index / 256);
    if (pageMask[index] === 1) {
      channel = 0;
      break;
    }
    for (const [ex, ey] of pageEdge) channel = Math.min(channel, Math.hypot(x - ex, y - ey));
  }

  let components = 0;
  const componentSizes = [];
  let pagePixels = 0;
  let minX = 256;
  let maxX = -1;
  let minY = 256;
  let maxY = -1;
  const queue = new Int32Array(pageMask.length);
  for (let start = 0; start < pageMask.length; start += 1) {
    if (pageMask[start] !== 1) continue;
    components += 1;
    let componentPixels = 0;
    let head = 0;
    let tail = 0;
    queue[tail++] = start;
    pageMask[start] = 2;
    while (head < tail) {
      const index = queue[head++];
      pagePixels += 1;
      componentPixels += 1;
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
    componentSizes.push(componentPixels);
  }
  componentSizes.sort((a, b) => b - a);

  return {
    rendered,
    channel: Number.isFinite(channel) ? Math.round(channel * 10) / 10 : -1,
    page: {
      components,
      sizes: componentSizes,
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
      new URL(projection.iconSource, BASE).pathname !== BRAND_PATH || new URL(projection.markSource, BASE).pathname !== BRAND_PATH ||
      projection.iconSource !== projection.markSource) {
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
  for (const [part, site] of Object.entries({ page: 'page-render', ribbon: 'ribbon-render' })) {
    for (const size of [16, 24, 32, 180]) {
      if (analysis.rendered[size][part] < 1) {
        fail(site, `${part} has no approved-colour pixel at ${size}px; counts = ${JSON.stringify(analysis.rendered[size])}`);
      }
    }
  }
  if (analysis.page.components !== 1 || analysis.page.height < 120 || analysis.page.width < 120) {
    fail('page-is-one-face', `page mask = ${JSON.stringify(analysis.page)}, want one face spanning the mark`);
  }
  // The drawn channel is about thirty pixels at 256px, three in the 24px
  // header. Below sixteen it shrinks under a pixel and a half there and stops
  // reading as ground between the two shapes.
  if (analysis.channel < 16) {
    fail('ribbon-clears-the-page', `the narrowest ground between ribbon and page is ${analysis.channel}px at 256px, want at least 16`);
  }

  await page.emulateMedia({ forcedColors: 'none' });
  // Each way a theme arrives: a stored light, a stored dark, and no stored
  // choice on a dark desk.
  const themes = [
    { name: 'light', theme: 'light', scheme: 'light' },
    { name: 'dark', theme: 'dark', scheme: 'light' },
    { name: 'system dark', theme: null, scheme: 'dark' },
  ];
  const grounds = {};
  for (const { name, theme, scheme } of themes) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.evaluate((selected) => {
      if (selected === null) delete document.documentElement.dataset.theme;
      else document.documentElement.dataset.theme = selected;
    }, theme);
    const mark = await page.locator('.y-brand__mark').evaluate((element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      // The ground the page shape is seen against: the mark's own background
      // over the header's over the document's, composited the way they paint.
      const canvas = document.createElement('canvas');
      canvas.width = 1;
      canvas.height = 1;
      const context = canvas.getContext('2d', { willReadFrequently: true });
      const header = element.closest('.y-header');
      for (const layer of [document.documentElement, document.body, header, element]) {
        if (!layer) continue;
        context.fillStyle = getComputedStyle(layer).backgroundColor;
        context.fillRect(0, 0, 1, 1);
      }
      const [r, g, b] = context.getImageData(0, 0, 1, 1).data;
      context.clearRect(0, 0, 1, 1);
      for (const layer of [document.documentElement, document.body, header]) {
        if (!layer) continue;
        context.fillStyle = getComputedStyle(layer).backgroundColor;
        context.fillRect(0, 0, 1, 1);
      }
      const [hr, hg, hb] = context.getImageData(0, 0, 1, 1).data;
      return {
        visible: style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity) > 0 && rect.width === 24 && rect.height === 24,
        background: style.backgroundColor,
        ground: [r, g, b],
        header: [hr, hg, hb],
      };
    });
    if (!mark.visible) fail('theme-mark', `header mark is not a visible 24px projection in ${name} theme`);
    if (name === 'light' && mark.background !== 'rgba(0, 0, 0, 0)') {
      fail('light-mark-bare', `header mark background in the light theme = ${mark.background}, want none`);
    }
    const luminance = (rgb) => {
      const [r, g, b] = rgb.map((value) => {
        const channel = value / 255;
        return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
      });
      return 0.2126 * r + 0.7152 * g + 0.0722 * b;
    };
    const contrast = (a, b) => {
      const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
      return (hi + 0.05) / (lo + 0.05);
    };
    const pageInk = [0x28, 0x2a, 0x2d];
    const ribbonInk = [0x53, 0x6d, 0x85];
    grounds[name] = {
      page: Math.round(contrast(pageInk, mark.ground) * 100) / 100,
      ribbon: Math.round(contrast(ribbonInk, mark.ground) * 100) / 100,
      header: Math.round(contrast(mark.ground, mark.header) * 100) / 100,
    };
    // The page shape is the mark's silhouette; as a graphic it needs 3:1
    // against what it is seen on.
    if (grounds[name].page < 3) {
      fail('mark-ground', `header mark page shape contrast in ${name} theme = ${grounds[name].page}:1 on rgb(${mark.ground}), want at least 3:1`);
    }
  }
  console.log(`brand-contract channel: ${analysis.channel}px at 256px; grounds: ${JSON.stringify(grounds)}`);
  await page.emulateMedia({ colorScheme: 'light' });

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

  console.log(`PASS brand-contract: one canonical mark keeps its page whole and its ribbon clear of the page, with an accessible projection, a legible ground in every theme, and a 360px fit`);
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
