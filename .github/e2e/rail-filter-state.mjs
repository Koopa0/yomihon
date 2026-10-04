// The parsed rail owns remembered narrowing even if the deferred entry fails.
// These observations are at DOMContentLoaded; frame/shift evidence is separate.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Course/C02.md';
const NEXT = '/notes/Course/C03.md';
const MUTATE = process.env.MUTATE || '';
const FILTER = '.y-rail-left [data-nav-filter]';
const SITES = ['early-owner', 'remembered-raw', 'restore-hold', 'controller-lifetime', 'clear-state', 'whole-row-set', 'filter-behavior', 'notice-inventory', 'reach-count', 'reach-locale', 'reach-query', 'filter-persistence'];
const MUTATIONS = {
  'omit-inline-call': { target: 'early-owner', needle: /\binitSidebar\(\);/g, replacement: 'void 0;' },
  'omit-remembered-value': { target: 'remembered-raw', needle: /input\.value = remembered;/g, replacement: 'void remembered;' },
  'omit-restoring-hold': { target: 'restore-hold', needle: /rail\.dataset\.railRestoring = '';/g, replacement: 'void 0;' },
  'omit-restoring-flush': { target: 'restore-hold', needle: /void rail\.offsetWidth;/g, replacement: 'void 0;' },
  'bypass-retained-controller': { target: 'controller-lifetime', reentry: true, needle: /if \(rail\.sidebarController\) return rail\.sidebarController;/g, replacement: 'if (false) return rail.sidebarController;' },
  'lose-original-defaults': { target: 'clear-state', needle: /serverOpen\.set\(details, details\.open\);/g, replacement: 'serverOpen.set(details, false);' },
  'drop-current-ancestry': { target: 'clear-state', needle: /if \(details\.hasAttribute\('data-chain'\)\) return true;/g, replacement: 'if (false) return true;' },
  'omit-unresolved-rows': { target: 'whole-row-set', needle: /const rows = \[\.\.\.rail\.querySelectorAll\('a, span\.ui-navitem'\)\];/g, replacement: "const rows = [...rail.querySelectorAll('a')];" },
  'ignore-filter-text': { target: 'filter-behavior', needle: /const query = input\.value\.trim\(\)\.toLowerCase\(\);/g, replacement: "const query = '';" },
  'recapture-derived-notice': { target: 'notice-inventory', needle: /const rows = \[\.\.\.rail\.querySelectorAll\('a, span\.ui-navitem'\)\];/g, replacement: "const rows = { forEach: (fn) => rail.querySelectorAll('a, span.ui-navitem').forEach(fn), some: (fn) => [...rail.querySelectorAll('a, span.ui-navitem')].some(fn) };" },
  'drop-trimmed-producers': { target: 'reach-count', needle: /const trimmedFolders = \[\.\.\.rail\.querySelectorAll\('\[data-rail-trimmed\]'\)\];/g, replacement: "const trimmedFolders = [...rail.querySelectorAll('[data-rail-trimmed]')].slice(0, 1);" },
  'lose-singular-wording': { target: 'reach-locale', needle: /unreached === 1 \? partial\.dataset\.filterPartialOne : partial\.dataset\.filterPartialMany/g, replacement: 'partial.dataset.filterPartialMany' },
  'lose-search-label': { target: 'reach-locale', needle: /link\.textContent = partial\.dataset\.filterSearchall \?\? '';/g, replacement: "link.textContent = '';" },
  'unquote-folder-query': { target: 'reach-query', needle: /`folder:"\$\{dir\}" \$\{query\}`/g, replacement: `\`folder:\${dir} \${query}\`` },
  'forget-input': { target: 'filter-persistence', needle: /if \(input\.value\) sessionStorage\.setItem\(filterKey, input\.value\);/g, replacement: 'if (input.value) void 0;' },
};
class LockFired extends Error {
  constructor(site, message) { super(`FAIL rail-filter-state: ${message}`); this.site = site; }
}
class NotApplied extends Error {}
const check = (condition, site, message) => { if (!condition) throw new LockFired(site, message); };
// Controls establish the case inputs and preserve already-registered behavior;
// they do not claim a mutation catch for an unrelated failure.
const control = (condition, message) => { if (!condition) throw new Error(`rail-filter-state control: ${message}`); };
for (const [name, mode] of Object.entries(MUTATIONS)) {
  control(SITES.includes(mode.target), `unknown site for ${name}`);
}
for (const site of SITES) {
  control(Object.values(MUTATIONS).some((mode) => mode.target === site), `no mutation for ${site}`);
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) process.exit(2);

const browser = await chromium.launch({ channel: 'chrome', headless: true, ignoreDefaultArgs: ['--disable-back-forward-cache'] });
function inputRows({ defaults = false, reach = [] } = {}) {
  let result = '';
  if (defaults) {
    result += '<details open data-key="probe-open"><summary>Open default</summary><a href="/notes/Course/C01.md">Kept Open</a></details>';
    result += '<details data-key="probe-closed"><summary>Closed default</summary><a href="/notes/Course/S01.md">Needle Closed</a></details>';
    result += '<div class="y-here"><a href="/folders">Header Token</a><span class="ui-navitem">\u661f\u7a7a \u00c4ther</span></div>';
  }
  for (const { count, dir } of reach) {
    control(Number.isInteger(count) && count >= 0 && !/["<>]/.test(dir), 'invalid bounded trimmed input');
    result += `<div class="y-here"><a href="/folders" data-rail-trimmed="${count}" data-rail-dir="${dir}">More Token</a></div>`;
  }
  return result;
}
async function open(site, options = {}) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  if (options.lang) await context.addCookies([{ name: 'yomihon_lang', value: options.lang, url: BASE }]);
  await context.addInitScript(({ remembered, disclosure, refuse }) => {
    if (remembered !== undefined) sessionStorage.setItem('yomihon.nav.filter', remembered);
    if (disclosure !== undefined) sessionStorage.setItem('yomihon.nav', disclosure);
    if (refuse === 'read') Object.defineProperty(window, 'sessionStorage', { get() { throw new DOMException('blocked', 'SecurityError'); } });
    if (refuse === 'write') Storage.prototype.setItem = function setItem() { throw new DOMException('blocked', 'SecurityError'); };
    window.restoreSamples = [];
    const value = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
    Object.defineProperty(HTMLInputElement.prototype, 'value', { ...value, set(next) {
      if (this.hasAttribute('data-nav-filter')) window.restoreSamples.push({ type: 'value', held: this.closest('.y-rail-left').hasAttribute('data-rail-restoring'), value: next });
      value.set.call(this, next);
    } });
    const width = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth');
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { ...width, get() {
      const result = width.get.call(this);
      if (this.matches('.y-rail-left')) window.restoreSamples.push({ type: 'flush', held: this.hasAttribute('data-rail-restoring'), transitions: [...this.querySelectorAll('details')].map((element) => getComputedStyle(element, '::details-content').transitionProperty) });
      return result;
    } });
  }, { remembered: options.remembered, disclosure: options.disclosure, refuse: options.refuse });
  const page = await context.newPage();
  let blocked = 0;
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  let documents = 0;
  const matchesByConsumption = { document: [], reentry: [] };
  const reported = new Set();
  const mode = MUTATE && MUTATIONS[MUTATE].target === site ? MUTATIONS[MUTATE] : null;
  if (options.setup || mode) {
    await page.route(`${BASE}/**`, async (route) => {
      const document = route.request().resourceType() === 'document';
      const reentry = mode?.reentry && new URL(route.request().url()).pathname === '/static/sidebar.js';
      if (!document && !reentry) return route.continue();
      const response = await route.fetch();
      let body = await response.text();
      if (document) documents += 1;
      if (options.setup && document) {
        const end = body.indexOf('</aside>');
        control(end >= 0 && body.includes('data-filter-partial-one='), 'bounded inputs need the real server rail and locale templates');
        body = body.slice(0, end) + inputRows(options.setup) + body.slice(end);
      }
      if (mode) {
        const matches = [...body.matchAll(mode.needle)].length;
        matchesByConsumption[document ? 'document' : 'reentry'].push(matches);
        if (matches === 1) body = body.replace(mode.needle, mode.replacement);
      }
      await route.fulfill({ response, body });
    });
  }
  if (options.blocked !== false) await page.route('**/yomihon.js', (route) => { blocked += 1; return route.abort(); });
  await page.goto(BASE + (options.path || PAGE), { waitUntil: 'domcontentloaded' });
  function prove(reentry = false) {
    if (mode) {
      for (const consumption of reentry && mode.reentry ? ['document', 'reentry'] : ['document']) {
        const matches = matchesByConsumption[consumption];
        if (matches.length === 0 || matches.some((count) => count !== 1)) throw new NotApplied(`${MUTATE}: ${consumption} matched ${JSON.stringify(matches)}, want exactly one site per consumed response`);
        if (!reported.has(consumption)) {
          console.log(`MUTATE-PROOF: ${MUTATE} ${consumption} responses=${matches.length} matches=${matches.join(',')}`);
          reported.add(consumption);
        }
      }
    }
    control(errors.length === 0, `unexpected page exceptions: ${errors.join('; ')}`);
    if (options.blocked !== false) control(blocked > 0, 'the actual deferred yomihon.js request was not intercepted');
    if (options.setup) control(documents > 0, 'bounded document inputs were never applied');
  }
  prove();
  return { page, context, prove };
}
const fill = (page, value) => page.locator(FILTER).fill(value);
const snapshot = (page) => page.locator('.y-rail-left').evaluate((rail) => ({
  rows: [...rail.querySelectorAll('a, span.ui-navitem')].filter((row) => !row.closest('[data-filter-partial]')).map((row) => ({ text: row.textContent.trim(), href: row.getAttribute('href'), hidden: row.hidden })),
  groups: [...rail.querySelectorAll('details, .y-here')].map((group) => ({ key: group.dataset.key || null, hidden: group.hidden, open: group.tagName === 'DETAILS' ? group.open : null })),
  empty: rail.querySelector('[data-filter-empty]')?.hidden,
}));
async function secondInit(page) {
  return page.evaluate(async () => {
    const { initSidebar } = await import('/static/sidebar.js');
    const first = initSidebar();
    const second = initSidebar();
    return first === second && first === document.querySelector('.y-rail-left').sidebarController;
  });
}
async function reachState(page) {
  return page.locator('[data-filter-partial]').evaluate((element) => ({ hidden: element.hidden, text: element.textContent.trim(), links: element.querySelectorAll('a').length, label: element.querySelector('a')?.textContent, query: element.querySelector('a') ? new URL(element.querySelector('a').href).searchParams.get('q') : null }));
}
try {
  {
    const { page, context } = await open('early-owner', { remembered: 'C02' });
    const value = await page.locator(FILTER).inputValue();
    check(value === 'C02' && await page.locator('#nav-rail a[href="/notes/Course/C01.md"]').evaluate((row) => row.hidden), 'early-owner', `remembered narrowing depends on the deferred entry: input=${JSON.stringify(value)}, want "C02" with the other lesson hidden`);
    await context.close();
  }
  {
    const raw = '  c02  ';
    const { page, context } = await open('remembered-raw', { remembered: raw });
    check(await page.locator(FILTER).inputValue() === raw, 'remembered-raw', 'restoration changed or lost the remembered raw query');
    await context.close();
  }
  {
    const { page, context } = await open('restore-hold', { remembered: 'C02' });
    const samples = await page.evaluate(() => window.restoreSamples);
    check(samples.some((sample) => sample.type === 'value' && sample.value === 'C02' && sample.held) && samples.some((sample) => sample.type === 'flush' && sample.held && sample.transitions.length > 0 && sample.transitions.every((property) => property === 'none')), 'restore-hold', `remembered restoration did not apply and flush under the declared motion hold: ${JSON.stringify(samples)}`);
    control(!(await page.locator('#nav-rail').evaluate((rail) => rail.hasAttribute('data-rail-restoring'))), 'restoring hold leaked after initialization');
    await context.close();
  }
  {
    const { page, context, prove } = await open('controller-lifetime', { remembered: 'Needle', setup: { defaults: true, reach: [{ count: 1, dir: 'Module 2' }] } });
    const retained = await secondInit(page);
    prove(true);
    check(retained, 'controller-lifetime', 'second native-module initialization replaced the retained document controller');
    await fill(page, '');
    control(await page.locator('details[data-key="probe-open"]').evaluate((element) => element.open), 'second init recaptured a filtered closed state as the original open default');
    control(!(await page.locator('details[data-key="probe-closed"]').evaluate((element) => element.open)), 'second init recaptured a filtered open state as the original closed default');
    await context.close();
  }
  {
    const { page, context } = await open('clear-state', { remembered: 'Needle', disclosure: '{"probe-closed":false}', setup: { defaults: true } });
    await secondInit(page);
    await page.evaluate(() => {
      const choices = JSON.parse(sessionStorage.getItem('yomihon.nav'));
      document.querySelectorAll('#nav-rail details[data-chain]').forEach((group) => { choices[group.dataset.key] = false; });
      sessionStorage.setItem('yomihon.nav', JSON.stringify(choices));
    });
    await fill(page, '');
    const state = await snapshot(page);
    check(state.rows.every((row) => !row.hidden) && state.groups.every((group) => !group.hidden) && state.groups.find((group) => group.key === 'probe-open')?.open === true && state.groups.find((group) => group.key === 'probe-closed')?.open === false && await page.locator('details[data-chain]').evaluateAll((groups) => groups.every((group) => group.open)), 'clear-state', 'clear lost original open/closed defaults, stored false, current ancestry, or the original row/group set');
    control(await page.evaluate(() => JSON.parse(sessionStorage.getItem('yomihon.nav'))['probe-closed']) === false, 'filtering overwrote a manual disclosure choice');
    await context.close();
  }
  {
    const { page, context } = await open('whole-row-set');
    await fill(page, 'C02');
    check(await page.locator('#nav-rail span.ui-navitem[data-resolution="unresolved"]').evaluate((row) => row.hidden), 'whole-row-set', 'unresolved span rows escaped the original filtering inventory');
    await fill(page, 'C04 Unwritten');
    control(await page.locator('#nav-rail span.ui-navitem[data-resolution="unresolved"]').isVisible(), 'unresolved-only match lost its current ancestry');
    await context.close();
  }
  {
    const { page, context } = await open('filter-behavior', { setup: { defaults: true } });
    const original = await snapshot(page);
    for (const query of [' c02 ', 'C02', 'c0', '\u661f\u7a7a', '\u00e4THER', 'missing-token', '   ', '']) {
      await fill(page, query);
      const state = await snapshot(page);
      const visible = state.rows.filter((row) => !row.hidden).map((row) => row.text);
      const expected = {
        ' c02 ': ['C02 draft'], C02: ['C02 draft'], c0: ['C01 draft', 'C02 draft', 'C03 draft', '! C04 Unwritten \u672a\u89e3\u6790'],
        '\u661f\u7a7a': ['\u661f\u7a7a \u00c4ther'], '\u00e4THER': ['\u661f\u7a7a \u00c4ther'], 'missing-token': [],
      }[query];
      check(expected ? JSON.stringify(visible) === JSON.stringify(expected) && state.empty === (expected.length > 0) : JSON.stringify(state.rows) === JSON.stringify(original.rows) && state.groups.every((group) => !group.hidden), 'filter-behavior', `${JSON.stringify(query)} rendered unexpected original rows: ${JSON.stringify(visible)}`);
    }
    await context.close();
  }
  {
    const { page, context } = await open('notice-inventory', { setup: { reach: [{ count: 1, dir: 'Module 2' }] } });
    await fill(page, 'missing-token');
    await secondInit(page);
    await fill(page, '\u641c\u5c0b\u5168\u90e8');
    check(!(await page.locator('[data-filter-empty]').evaluate((element) => element.hidden)) && (await reachState(page)).links === 1, 'notice-inventory', 'a derived search notice became an original row or duplicated its exit after repeated initialization/input');
    await fill(page, '');
    control((await reachState(page)).hidden, 'empty filter kept the reach notice');
    await context.close();
  }
  {
    const { page, context } = await open('reach-count', { setup: { reach: [{ count: 1, dir: 'Module 2' }, { count: 2, dir: 'Another' }] } });
    await fill(page, 'C02');
    const state = await reachState(page);
    check(!state.hidden && state.text === '\u53ea\u7be9\u4e86\u5074\u6b04\u5217\u51fa\u7684\u9805\u76ee\uff0c\u53e6\u5916 3 \u9805\u6c92\u641c\u5230\u3002 \u641c\u5c0b\u5168\u90e8 \u2192' && state.query === 'c02', 'reach-count', `reach did not cover the entire trimmed producer set: ${JSON.stringify(state)}`);
    await context.close();
  }
  for (const lang of ['zh-Hant', 'en']) {
    for (const count of [0, 1, 2]) {
      const { page, context } = await open('reach-locale', { lang, setup: { reach: [{ count, dir: 'Module 2' }] } });
      control(await page.locator('html').getAttribute('lang') === lang, `locale cookie did not select ${lang}`);
      await fill(page, 'missing-token');
      const state = await reachState(page);
      const sentence = lang === 'en' ? (count === 1 ? 'Only what the rail listed was filtered; 1 more was not reached.' : `Only what the rail listed was filtered; ${count} more were not reached.`) : `\u53ea\u7be9\u4e86\u5074\u6b04\u5217\u51fa\u7684\u9805\u76ee\uff0c\u53e6\u5916 ${count} \u9805\u6c92\u641c\u5230\u3002`;
      const label = lang === 'en' ? 'Search everything \u2192' : '\u641c\u5c0b\u5168\u90e8 \u2192';
      check(count === 0 ? state.hidden : !state.hidden && state.links === 1 && state.label === label && state.text === `${sentence} ${label}`, 'reach-locale', `${lang}, ${count}: lost server-owned reach wording or label: ${JSON.stringify(state)}`);
      await context.close();
    }
  }
  {
    const { page, context } = await open('reach-query', { setup: { reach: [{ count: 1, dir: 'Module 2' }] } });
    await fill(page, '  \u661f\u7a7a  ');
    const state = await reachState(page);
    check(state.query === 'folder:"Module 2" \u661f\u7a7a', 'reach-query', `space-bearing folder query lost quoting or normalized text: ${JSON.stringify(state.query)}`);
    await context.close();
  }
  {
    const { page, context } = await open('filter-persistence');
    await fill(page, ' c02 ');
    check(await page.evaluate(() => sessionStorage.getItem('yomihon.nav.filter')) === ' c02 ', 'filter-persistence', 'input lost its raw tab-session persistence');
    await fill(page, '');
    control(await page.evaluate(() => sessionStorage.getItem('yomihon.nav.filter')) === null, 'clearing kept a stored filter');
    await context.close();
  }
  // Invalid/missing/refused choices fall back to native server disclosures.
  for (const disclosure of [undefined, '{bad', 'null', 'false', '{"probe-closed":"true"}']) {
    const { page, context } = await open('', { disclosure, setup: { defaults: true } });
    await fill(page, 'Needle');
    await secondInit(page);
    await fill(page, '');
    control(await page.locator('details[data-key="probe-open"]').evaluate((element) => element.open) && !(await page.locator('details[data-key="probe-closed"]').evaluate((element) => element.open)), `${String(disclosure)} did not retain the original disclosure defaults`);
    await context.close();
  }
  {
    const { page, context } = await open('', { remembered: 'Kept', disclosure: '{"probe-closed":true}', setup: { defaults: true } });
    await secondInit(page);
    await fill(page, 'Needle');
    await fill(page, '');
    control(await page.locator('details[data-key="probe-closed"]').evaluate((element) => element.open), 'second initialization/input/clear lost a stored open choice');
    control(await page.evaluate(() => JSON.parse(sessionStorage.getItem('yomihon.nav'))['probe-closed']) === true, 'filtering overwrote a stored open choice');
    await context.close();
  }
  for (const refuse of ['read', 'write']) {
    const { page, context } = await open('', { refuse });
    await fill(page, 'C02');
    control(await page.locator('#nav-rail a[href="/notes/Course/C01.md"]').evaluate((row) => row.hidden), `refused ${refuse} prevented current filtering`);
    await context.close();
  }
  // Normal next navigation uses the production footer, with no document input
  // rewriting and no preseeded storage; typing is the preference's only source.
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();
    await page.addInitScript(() => {
      window.addEventListener('pageshow', (event) => { window.fromBFCache = event.persisted; });
    });
    await page.goto(BASE + PAGE);
    await page.waitForSelector('html[data-js]');
    await fill(page, 'C0');
    await page.locator(`.y-steps a[rel="next"][href="${NEXT}"]`).click();
    await page.waitForURL(BASE + NEXT);
    control(await page.locator(FILTER).inputValue() === 'C0', 'real footer next lost remembered narrowing');
    await page.evaluate(() => sessionStorage.setItem('yomihon.nav.filter', 'changed-in-other-document'));
    await page.goBack({ waitUntil: 'commit' });
    await page.waitForFunction(() => window.fromBFCache === true);
    control(await page.evaluate(() => window.fromBFCache === true), 'back navigation did not positively restore a BFCache document');
    control(await page.locator(FILTER).inputValue() === 'C0', 'BFCache restoration resynchronized retained DOM from newer tab-session storage');
    await context.close();
  }
  for (const path of ['/notes/Notes/alpha.md', '/reports/browser-boundary.html', '/search']) {
    const { page, context } = await open('', { path, remembered: 'missing-token' });
    control(await page.locator(FILTER).inputValue() === 'missing-token' && !(await page.locator('[data-filter-empty]').evaluate((element) => element.hidden)), `${path} entry did not use the same parsed filter owner`);
    await fill(page, '');
    control((await snapshot(page)).rows.every((row) => !row.hidden), `${path} clearing did not restore the whole original row set`);
    await context.close();
  }
  {
    const context = await browser.newContext({ javaScriptEnabled: false });
    const page = await context.newPage();
    await page.goto(BASE + PAGE);
    control(await page.locator(FILTER).evaluate((element) => element.hidden), 'no-JavaScript filter showed an inert control');
    const branch = page.locator('#nav-rail details[data-key]:not([data-chain])');
    control(await branch.count() === 1, 'native disclosure control needs one closed side branch');
    await branch.locator(':scope > summary').click();
    control(await branch.evaluate((element) => element.open), 'native disclosure failed without JavaScript');
    await page.locator('#nav-rail a[href="/notes/Course/C03.md"]').press('Enter');
    await page.waitForURL(BASE + NEXT);
    await context.close();
  }
  for (const path of ['/syllabus/Maps/branches.md', '/preferences']) {
    const { page, context } = await open('', { path, remembered: 'missing-token', blocked: false });
    control(await page.locator(FILTER).count() === 0, `${path} control unexpectedly has a filter`);
    const inert = await page.evaluate(async () => { const { initSidebar } = await import('/static/sidebar.js'); return initSidebar().canFocusFilter(); });
    control(inert === false, `${path} did not return the inert filter controller`);
    await context.close();
  }
  console.log('PASS rail-filter-state: parsed-owner restoration, retained original state/inventory, complete row and reach behavior, and public navigation controls');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      if (error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${MUTATIONS[MUTATE].target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
