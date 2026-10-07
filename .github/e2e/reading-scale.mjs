// Code and tables follow the reader's type choice, including table readings.
// The measured elements come from authored Markdown through the real renderer.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const ORIGIN = new URL(BASE).origin;
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-scale.md';
const MUTATE = process.env.MUTATE || '';
const SIZE_CHOICES = ['m', 'l', 'xl'];
const SITES = ['code-scale', 'table-scale', 'table-reading', 'size-catalog'];
const MODES = {
  'add-unchecked-size': { site: 'size-catalog', catalog: 'add' },
  'drop-extra-large-size': { site: 'size-catalog', catalog: 'drop' },
  'pin-code': { site: 'code-scale', selector: '.y-prose pre', from: 'var(--fs-ed-13)', to: 'var(--fs-13)' },
  'pin-table': { site: 'table-scale', selector: '.y-prose table', from: 'var(--fs-ed-15)', to: 'var(--fs-15)' },
  'pin-header': { site: 'table-scale', selector: '.y-prose th', from: 'inherit', to: 'var(--fs-14)' },
  'pin-cell': { site: 'table-scale', selector: '.y-prose td', from: 'inherit', to: 'var(--fs-15)' },
  'shrink-table-reading': { site: 'table-reading', selector: '.y-prose td rt', from: '0.7em', to: '0.55em' },
  'pin-table-reading': { site: 'table-scale', selector: '.y-prose td rt', from: '0.7em', to: '10.5px' },
  'pin-large-code-step': { site: 'code-scale', selector: ":root[data-textsize='l']", property: '--fs-ed-13', from: '0.9375rem', to: '0.8125rem' },
};
if (MUTATE === 'list') {
  for (const mode of Object.keys(MODES)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MODES, MUTATE)) {
  console.error(`UNKNOWN reading-scale mode: ${MUTATE}`);
  process.exit(2);
}
for (const site of SITES) {
  if (!Object.values(MODES).some((mode) => mode.site === site)) throw new Error(`unwatched site ${site}`);
}

class NotApplied extends Error {}
class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const mutation = async (page) => {
  const counts = [];
  if (MUTATE && !MODES[MUTATE].catalog) {
    const mode = MODES[MUTATE];
    const declaration = new RegExp(`(${escaped(mode.property || 'font-size')}\\s*:\\s*)${escaped(mode.from)}(?=\\s*;)`, 'g');
    const rule = new RegExp(`(^|[{}])(\\s*${escaped(mode.selector)}\\s*\\{)([^{}]*)\\}`, 'g');
    await page.route((url) => url.origin === ORIGIN && url.pathname === '/static/app.css', async (route) => {
      const response = await route.fetch();
      if (response.status() !== 200) throw new Error(`asset HTTP ${response.status()}`);
      const source = await response.text();
      // Comments carry no selector or declaration identity. Mask them at the
      // same source positions, then edit only values in the original response.
      const masked = source.replace(/\/\*[\s\S]*?\*\//g, (comment) => ' '.repeat(comment.length));
      const edits = [];
      for (const match of masked.matchAll(rule)) {
        const offset = match.index + match[1].length + match[2].length;
        for (const property of match[3].matchAll(declaration)) {
          const start = offset + property.index + property[1].length;
          edits.push({ start, end: start + mode.from.length });
        }
      }
      let body = source;
      for (const edit of edits.reverse()) body = body.slice(0, edit.start) + mode.to + body.slice(edit.end);
      counts.push(edits.length);
      await route.fulfill({ response, body });
    });
  }
  return () => {
    if (MUTATE && !MODES[MUTATE].catalog) {
      console.log(`APPLIED reading-scale: ${MUTATE}: ${JSON.stringify(counts)}`);
      if (counts.length !== 1 || counts[0] !== 1) throw new NotApplied(`NOT-APPLIED reading-scale ${MUTATE}: ${JSON.stringify(counts)}`);
    }
  };
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const failures = new Map();
let visited = 0;
const fail = (site, message) => {
  if (!SITES.includes(site)) throw new Error(`unknown site ${site}`);
  if (!failures.has(site)) failures.set(site, message);
};
// Read every offered size from the real settings page before driving it.
// A new or removed choice must change the lock rather than escape its matrix.
const offeredSizes = async (lang) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }]);
    const page = await context.newPage();
    let catalogWrites = 0;
    page.on('request', (request) => { if (request.method() !== 'GET') catalogWrites += 1; });
    const proof = { requests: 0, fields: 0, changes: 0, errors: [] };
    if (MODES[MUTATE]?.catalog) {
      await page.route((url) => url.origin === ORIGIN && url.pathname === '/preferences', async (route) => {
        proof.requests += 1;
        try {
          const response = await route.fetch();
          if (response.status() !== 200) throw new Error(`preferences HTTP ${response.status()}`);
          const html = await response.text();
          const fields = [...html.matchAll(/<fieldset\b(?=[^>]*\bdata-pref-field="textsize")[^>]*>[\s\S]*?<\/fieldset>/g)];
          proof.fields += fields.length;
          let body = html;
          if (fields.length === 1) {
            const field = fields[0][0];
            let changed;
            if (MODES[MUTATE].catalog === 'add') {
              const extra = '<label><input type="radio" name="textsize" value="unchecked-size"><span>Unchecked size</span></label>';
              changed = field.replace('</fieldset>', `${extra}</fieldset>`);
              if (changed !== field) proof.changes += 1;
            } else {
              changed = field.replace(/<input\b(?=[^>]*\bname="textsize")(?=[^>]*\bvalue="xl")[^>]*>/g, () => {
                proof.changes += 1;
                return '';
              });
            }
            body = html.replace(field, changed);
          }
          await route.fulfill({ response, body });
        } catch (error) { proof.errors.push(error.message); await route.abort(); }
      });
    }
    const response = await page.goto(`${BASE}/preferences`);
    if (!response || response.status() !== 200) throw new Error(`preferences HTTP ${response?.status()}`);
    if (MODES[MUTATE]?.catalog) {
      if (proof.errors.length) throw new Error(proof.errors.join('; '));
      console.log(`APPLIED reading-scale: ${MUTATE}/${lang}: ${JSON.stringify(proof)}`);
      if (proof.requests !== 1 || proof.fields !== 1 || proof.changes !== 1) throw new NotApplied(`NOT-APPLIED reading-scale ${MUTATE}: ${JSON.stringify(proof)}`);
    }
    const fields = page.locator('[data-pref-field="textsize"]');
    if (await fields.count() !== 1) throw new Error('preferences have no unique text-size field');
    const values = await fields.locator('input[type="radio"][name="textsize"]').evaluateAll((nodes) => nodes.map((node) => node.value));
    if (catalogWrites !== 0) throw new Error('preferences catalog made a write request');
    if (MODES[MUTATE]?.catalog) {
      const extraCount = values.filter((value) => value === 'unchecked-size').length;
      if ((MODES[MUTATE].catalog === 'add' && extraCount !== 1) || (MODES[MUTATE].catalog === 'drop' && values.includes('xl'))) throw new NotApplied('catalog mutation did not reach the actual offered controls');
    }
    console.log(`OFFERED reading-scale ${lang}: ${JSON.stringify(values)}`);
    if (JSON.stringify(values) !== JSON.stringify(SIZE_CHOICES)) fail('size-catalog', `${lang}: actual offered sizes ${JSON.stringify(values)}, expected every driven size ${JSON.stringify(SIZE_CHOICES)}`);
    return values;
  } finally { await context.close(); }
};
try {
  const sizes = await offeredSizes('en');
  const translatedSizes = await offeredSizes('zh-Hant');
  if (JSON.stringify(sizes) !== JSON.stringify(translatedSizes)) fail('size-catalog', 'interface languages offer different sizes');
  if (failures.has('size-catalog')) throw new LockFired('size-catalog', failures.get('size-catalog'));
  for (const width of [390, 1280]) {
    for (const lang of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        for (const javaScriptEnabled of [true, false]) {
          const rows = [];
          for (const size of sizes) {
            const context = await browser.newContext({ viewport: { width, height: 900 }, javaScriptEnabled });
            try {
              await context.addCookies([
                { name: 'yomihon_textsize', value: size, url: BASE },
                { name: 'yomihon_lang', value: lang, url: BASE },
                { name: 'yomihon_theme', value: theme, url: BASE },
                { name: 'yomihon_ruby', value: 'on', url: BASE },
              ]);
              const page = await context.newPage();
              let writes = 0;
              const assetStatuses = [];
              page.on('request', (request) => { if (request.method() !== 'GET') writes += 1; });
              page.on('response', (response) => {
                if (new URL(response.url()).pathname === '/static/app.css') assetStatuses.push(response.status());
              });
              const applied = await mutation(page);
              const response = await page.goto(BASE + PAGE);
              if (response.status() !== 200) throw new Error(`page HTTP ${response.status()}`);
              await page.evaluate(async () => {
                await document.fonts.ready;
              });
              try {
                await arrived(page);
              } catch (error) {
                const where = `${width}/${lang}/${theme}/js=${javaScriptEnabled}/size=${size}`;
                try {
                  const state = await page.evaluate(() => ({
                    url: location.href,
                    readyState: document.readyState,
                    fonts: document.fonts.status,
                    preferences: { ...document.documentElement.dataset },
                    sheets: [...document.styleSheets].map((sheet) => ({ href: sheet.href, disabled: sheet.disabled })),
                    links: [...document.querySelectorAll('link[rel="stylesheet"]')].map((link) => ({ href: link.href, sheet: Boolean(link.sheet), disabled: link.disabled })),
                    animations: document.getAnimations().map((animation) => ({
                      name: animation.animationName,
                      pending: animation.pending,
                      playState: animation.playState,
                      currentTime: animation.currentTime === null || typeof animation.currentTime === 'number'
                        ? animation.currentTime
                        : String(animation.currentTime),
                    })),
                  }));
                  console.error(`ARRIVAL-FAIL reading-scale ${MUTATE || 'plain'} ${where}: ${JSON.stringify({ assetStatuses, writes, state })}`);
                } catch (diagnosticError) {
                  console.error(`ARRIVAL-FAIL reading-scale ${MUTATE || 'plain'} ${where}: diagnostic unavailable: ${diagnosticError.message}`);
                }
                throw error;
              }
              applied();
              const row = await page.evaluate(() => {
                const groups = {
                  code: [...document.querySelectorAll('.y-prose pre code')],
                  headers: [...document.querySelectorAll('.y-prose th')],
                  cells: [...document.querySelectorAll('.y-prose td')],
                  readings: [...document.querySelectorAll('.y-prose td rt')],
                };
                return {
                  size: document.documentElement.dataset.textsize,
                  lang: document.documentElement.lang,
                  theme: document.documentElement.dataset.theme,
                  ruby: document.documentElement.dataset.ruby,
                  groups: Object.fromEntries(Object.entries(groups).map(([key, elements]) => [key, elements.map((element) => ({
                    text: element.textContent.trim(),
                    px: parseFloat(getComputedStyle(element).fontSize),
                    visible: getComputedStyle(element).visibility,
                  }))])),
                  prose: parseFloat(getComputedStyle(document.querySelector('.y-prose p')).fontSize),
                  room: document.documentElement.clientWidth,
                  pageWidth: document.documentElement.scrollWidth,
                };
              });
              if (row.size !== size || row.lang !== lang || row.theme !== theme || row.ruby !== 'on') throw new Error(`wrong emitted preferences ${JSON.stringify(row)}`);
              const texts = Object.fromEntries(Object.entries(row.groups).map(([key, elements]) => [key, elements.map((element) => element.text)]));
              const authored = { code: ['value := <-results'], headers: ['Word', 'Meaning'], cells: ['辞書じしょ', 'dictionary'], readings: ['じしょ'] };
              if (JSON.stringify(texts) !== JSON.stringify(authored)) throw new Error(`fixture members differ: ${JSON.stringify(texts)}`);
              if (assetStatuses.length !== 1 || assetStatuses[0] !== 200 || writes !== 0) throw new Error(`unqualified assets/writes ${JSON.stringify({ assetStatuses, writes })}`);
              if (row.pageWidth > row.room + 1) throw new Error(`article overflows: ${row.pageWidth}/${row.room}`);
              if (Object.values(row.groups).flat().some((element) => element.visible !== 'visible' || !Number.isFinite(element.px))) throw new Error('hidden or nonnumeric fixture member');
              rows.push(row);
              visited += 1;
            } finally {
              await context.close();
            }
          }
          const where = `${width}/${lang}/${theme}/js=${javaScriptEnabled}`;
          const code = rows.map((row) => row.groups.code[0].px);
          if (!(code[0] < code[1] && code[1] < code[2])) fail('code-scale', `${where}: code ${JSON.stringify(code)} does not grow m<l<xl`);
          for (const group of ['headers', 'cells', 'readings']) {
            for (let i = 0; i < rows[0].groups[group].length; i += 1) {
              const sizes = rows.map((row) => row.groups[group][i].px);
              if (!(sizes[0] < sizes[1] && sizes[1] < sizes[2])) fail('table-scale', `${where}: ${group}[${i}] ${JSON.stringify(sizes)} does not grow m<l<xl`);
            }
          }
          const readings = rows.map((row) => row.groups.readings[0].px);
          if (readings.some((px) => px < 10)) fail('table-reading', `${where}: table readings ${JSON.stringify(readings)} fall below 10px`);
          if (JSON.stringify(rows.map((row) => row.prose)) !== '[17,19,21]') throw new Error(`prose control changed: ${where}`);
        }
      }
    }
  }
  console.log(`VISITED reading-scale: ${visited} contexts`);
  for (const [site, message] of failures) console.error(`caught: reading-scale ${site}: ${message}`);
  if (failures.size) {
    process.exitCode = 1;
    if (MUTATE && failures.size === 1 && failures.has(MODES[MUTATE].site)) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
  } else if (MUTATE) {
    throw new Error(`mutation escaped: ${MUTATE}`);
  } else {
    console.log('PASS reading-scale: code, whole table and table readings grow m<l<xl, readings at least10px; both widths/languages/themes and no-JS; no writes');
  }
} catch (error) {
  if (error instanceof LockFired) {
    console.error(`caught: reading-scale ${error.site}: ${error.message}`);
    if (MUTATE && error.site === MODES[MUTATE].site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
  } else console.error(error);
  process.exitCode = error instanceof NotApplied ? 2 : 1;
  if (error instanceof NotApplied) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
} finally {
  await browser.close();
}
