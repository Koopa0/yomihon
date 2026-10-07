// The page and its native modules plus Mermaid facade name the requested bytes.
// This observes native relative-import resolution in Chrome as well as the
// served response bodies; it does not simulate a deployed CDN's cache policy.
// Env: YOMIHON_BASE, PAGE_PATH, MUTATE. MUTATE=list names every watched fault.
import { createHash } from 'node:crypto';
import { readdir } from 'node:fs/promises';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['styles', 'resources', 'css-font-set', 'css-font-identities', 'entry', 'module-set', 'module-identities', 'nonce', 'order', 'served-bytes'];
const mapElement = /<script type="importmap" nonce="[^"]*">([\s\S]*?)<\/script>/g;
const MUTATIONS = {
  'drop-stylesheet-version': { target: 'styles', needle: /href="\/static\/app\.css\?v=[a-f0-9]{12}"/g, replacement: 'href="/static/app.css"' },
  'drop-mark-version': { target: 'resources', needle: /src="\/static\/yomihon-mark\.svg\?v=[a-f0-9]{12}"/g, replacement: 'src="/static/yomihon-mark.svg"' },
  'drop-preload-version': { target: 'resources', needle: /href="\/static\/fonts\/Geist-Variable\.woff2\?v=[a-f0-9]{12}"/g, replacement: 'href="/static/fonts/Geist-Variable.woff2"' },
  'omit-css-font-url': { target: 'css-font-set', asset: '/static/app.css', needle: /url\('\/static\/fonts\/Newsreader-Latin-Italic-Variable\.woff2\?v=[a-f0-9]{12}'\)/g, replacement: "url('')" },
  'drop-font-url-version': { target: 'css-font-identities', asset: '/static/app.css', needle: /url\('\/static\/fonts\/Newsreader-Latin-Italic-Variable\.woff2\?v=[a-f0-9]{12}'\)/g, replacement: "url('/static/fonts/Newsreader-Latin-Italic-Variable.woff2')" },
  'drop-entry-version': { target: 'entry', needle: /src="\/static\/yomihon\.js\?v=[a-f0-9]{12}"/g, replacement: 'src="/static/yomihon.js"' },
  'omit-relative-module': { target: 'module-set', needle: mapElement, map: (data) => { delete data.imports['/static/contents.js']; } },
  'omit-vendored-module': { target: 'module-set', needle: mapElement, map: (data) => { delete data.imports['/static/mermaid.esm.min.mjs']; } },
  'unversioned-relative-module': { target: 'module-identities', needle: mapElement, map: (data) => { data.imports['/static/contents.js'] = '/static/contents.js'; } },
  'wrong-relative-module-version': { target: 'served-bytes', needle: mapElement, map: (data) => { data.imports['/static/contents.js'] = '/static/contents.js?v=000000000000'; } },
  'wrong-map-nonce': { target: 'nonce', needle: /<script type="importmap" nonce="[^"]*">/g, replacement: '<script type="importmap" nonce="wrong-response-nonce">' },
  'move-map-after-entry': { target: 'order', needle: mapElement, move: true },
  'change-served-module-bytes': { target: 'served-bytes', asset: '/static/contents.js', needle: /export function initContents\(\) \{/g, replacement: 'export function initContents() {\n  /* a different served module */' },
};
class LockFired extends Error {
  constructor(site, message) { super(`caught: asset-identity: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const fail = (site, message) => { throw new LockFired(site, message); };
for (const [name, mode] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mode.target)) throw new Error(`unknown assertion for ${name}`);
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mode) => mode.target === site)) throw new Error(`no mutation for ${site}`);
}
if (MUTATE === 'list') { console.log(Object.keys(MUTATIONS).join('\n')); process.exit(0); }
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) { console.error(`unknown MUTATE ${MUTATE}`); process.exit(2); }

const hash = (bytes) => createHash('sha256').update(bytes).digest('hex').slice(0, 12);
const files = await readdir(new URL('../../assets/js/', import.meta.url), { withFileTypes: true });
const ownPaths = files.filter((file) => file.isFile() && file.name.endsWith('.js')).map((file) => `/static/${file.name}`).sort();
const facadePath = '/static/mermaid.esm.min.mjs';
const facadeFiles = await readdir(new URL('../../assets/js/mermaid/', import.meta.url), { withFileTypes: true });
if (!facadeFiles.some((file) => file.isFile() && file.name === 'mermaid.esm.min.mjs')) throw new Error('embedded Mermaid facade is missing');
const modulePaths = [...ownPaths, facadePath].sort();
if (ownPaths.length !== 18 || modulePaths.length !== 19) throw new Error('embedded client module inventory changed');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const responses = [];
  const pending = [];
  const pageErrors = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  page.on('response', (response) => {
    const url = new URL(response.url());
    if (url.origin !== new URL(BASE).origin || (!modulePaths.includes(url.pathname) && !url.pathname.endsWith('.css') && !url.pathname.startsWith('/static/fonts/') && url.pathname !== '/static/yomihon-mark.svg')) return;
    pending.push((async () => responses.push({ url, status: response.status(), bytes: await response.body() }))());
  });
  let seen = 0;
  let issue = '';
  if (MUTATE) {
    const mode = MUTATIONS[MUTATE];
    await page.route((url) => mode.asset ? url.pathname === mode.asset : url.href === BASE + PAGE, async (route) => {
      const response = await route.fetch();
      const original = await response.text();
      seen++;
      const matches = [...original.matchAll(mode.needle)];
      if (matches.length !== 1) {
        issue = `needle matched ${matches.length} sites, want exactly one`;
        await route.fulfill({ response });
        return;
      }
      let body;
      if (mode.map) {
        const data = JSON.parse(matches[0][1]);
        mode.map(data);
        body = original.replace(mode.needle, (element) => element.replace(matches[0][1], JSON.stringify(data)));
      } else if (mode.move) {
        const entry = /<script nonce="[^"]*" type="module" src="\/static\/yomihon\.js\?v=[a-f0-9]{12}"><\/script>/g;
        if ([...original.matchAll(entry)].length !== 1) {
          issue = 'module entry matched other than one site';
          await route.fulfill({ response });
          return;
        }
        body = original.replace(mode.needle, '').replace(entry, `$&${matches[0][0]}`);
      } else body = original.replace(mode.needle, mode.replacement);
      await route.fulfill({ response, body });
    });
  }
  const documentResponse = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  // The fixture has no diagram. Exercise the actual dynamic import boundary
  // explicitly so facade membership protects a real browser GET as well.
  await page.evaluate(async () => { await import('/static/mermaid.esm.min.mjs'); });
  await Promise.all(pending);
  if (MUTATE && (issue || seen !== 1)) throw new NotApplied(issue || `target fetched ${seen} times, want one`);
  if (MUTATE) console.log(`mutation-applied: ${MUTATE} matched one site`);
  if (!documentResponse || documentResponse.status() !== 200) throw new Error('fixture document did not return 200');
  const declarations = await page.evaluate(() => {
    const scripts = [...document.scripts];
    const maps = scripts.filter((script) => script.type === 'importmap');
    const entries = scripts.filter((script) => script.type === 'module');
    return {
      styles: [...document.querySelectorAll('link[rel="stylesheet"]')].map((link) => link.getAttribute('href')),
      entries: entries.map((script) => script.getAttribute('src')),
      maps: maps.map((script) => ({ text: script.textContent, nonce: script.nonce })),
      mapAt: scripts.indexOf(maps[0]), entryAt: scripts.indexOf(entries[0]),
      resources: [...document.querySelectorAll('link[href], img[src], script[src]')].flatMap((element) => {
        const address = element.getAttribute(element.tagName === 'LINK' ? 'href' : 'src');
        return address && new URL(address, location.href).pathname.startsWith('/static/') ? [address] : [];
      }),
    };
  });
  console.log(`invocation-hit: page declarations and ${responses.length} served asset responses`);
  const csp = documentResponse.headers()['content-security-policy'] || '';
  const nonces = [...csp.matchAll(/'nonce-([^']+)'/g)].map((match) => match[1]);
  if (nonces.length !== 1) throw new Error('response policy does not declare one script nonce');
  const checkAddress = (address, path, site) => {
    const url = new URL(address || '', BASE);
    const response = responses.find((candidate) => candidate.url.href === url.href);
    if (url.origin !== new URL(BASE).origin || url.pathname !== path || url.hash || !/^\?v=[a-f0-9]{12}$/.test(url.search)) fail(site, `${path} has no exact byte-version address: ${address}`);
    if (!response || response.status !== 200) fail(site, `${address} has no successful observed browser response`);
    return response;
  };
  if (declarations.styles.length !== 2) fail('styles', 'page does not declare exactly two stylesheets');
  checkAddress(declarations.styles[0], '/static/app.css', 'styles');
  checkAddress(declarations.styles[1], '/static/chroma.css', 'styles');
  if (declarations.entries.length !== 1) fail('entry', 'page does not declare one module entry');
  checkAddress(declarations.entries[0], '/static/yomihon.js', 'entry');
  if (declarations.maps.length !== 1) fail('module-set', 'page does not declare exactly one import map');
  if (declarations.maps[0].nonce !== nonces[0]) fail('nonce', 'import map is not signed by the response nonce');
  if (declarations.mapAt >= declarations.entryAt) fail('order', 'import map follows the module entry');
  const data = JSON.parse(declarations.maps[0].text);
  if (Object.keys(data).join(',') !== 'imports' || JSON.stringify(Object.keys(data.imports).sort()) !== JSON.stringify(modulePaths)) fail('module-set', 'import map differs from the complete embedded native-module and facade set');
  for (const path of modulePaths) {
    checkAddress(data.imports[path], path, 'module-identities');
  }
  const requestedPaths = [...new Set(responses.filter((response) => modulePaths.includes(response.url.pathname)).map((response) => response.url.pathname))].sort();
  if (JSON.stringify(requestedPaths) !== JSON.stringify(modulePaths)) fail('module-identities', 'native imports did not request the complete module graph and facade');
  const resourcePaths = ['/static/app.css', '/static/chroma.css', '/static/yomihon.js', '/static/yomihon-mark.svg', '/static/yomihon-mark.svg', '/static/fonts/Geist-Variable.woff2', '/static/fonts/GeistMono-Variable.woff2', '/static/fonts/Newsreader-Latin-Variable.woff2'].sort();
  if (JSON.stringify(declarations.resources.map((address) => new URL(address, BASE).pathname).sort()) !== JSON.stringify(resourcePaths)) fail('resources', 'page does not declare the complete static link/img/script multiset');
  for (const address of declarations.resources) checkAddress(address, new URL(address, BASE).pathname, 'resources');
  const appCSS = checkAddress(declarations.styles[0], '/static/app.css', 'styles').bytes.toString('utf8');
  const cssFontURLs = [...appCSS.matchAll(/url\(['"]?(\/static\/fonts\/[^)'"\s]+)['"]?\)/g)].map((match) => match[1]);
  const fontFiles = await readdir(new URL('../../assets/fonts/', import.meta.url), { withFileTypes: true });
  const fontPaths = fontFiles.filter((file) => file.isFile() && file.name.endsWith('.woff2')).map((file) => `/static/fonts/${file.name}`).sort();
  if (fontPaths.length !== 6 || JSON.stringify(cssFontURLs.map((address) => new URL(address, BASE).pathname).sort()) !== JSON.stringify(fontPaths)) fail('css-font-set', 'served CSS does not address the complete embedded font set');
  for (const address of cssFontURLs) {
    const url = new URL(address, BASE);
    const response = await context.request.get(url.href);
    responses.push({ url, status: response.status(), bytes: await response.body() });
    checkAddress(address, url.pathname, 'css-font-identities');
  }
  for (const response of responses) {
    if (response.url.search !== `?v=${hash(response.bytes)}`) fail('served-bytes', `${response.url.pathname} returned bytes unlike its requested hash`);
  }
  if (pageErrors.length) throw new Error(`browser reported module errors: ${JSON.stringify(pageErrors)}`);
  if (MUTATE) throw new Error(`mutation ${MUTATE} escaped its lock`);
  console.log(`PASS asset-identity: styles, fonts, brand and all ${modulePaths.length} native modules plus facade carry served-byte identities and a response-bound map`);
  await context.close();
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) { console.log(`MUTATE-RESULT: not-applied ${MUTATE}`); process.exitCode = 2; }
  else if (error instanceof LockFired) {
    if (MUTATE && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    else if (MUTATE) console.error(`no catch: ${error.site} fired before ${MUTATIONS[MUTATE].target}`);
    process.exitCode = 1;
  } else process.exitCode = 1;
} finally { await browser.close(); }
