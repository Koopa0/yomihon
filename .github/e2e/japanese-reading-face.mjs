// Japanese passages need Japanese glyph shapes on both words and readings.
// Read actual computed families at local language boundaries: changing a font
// variable alone does not change the family inherited by a paragraph.
// Env: YOMIHON_BASE, PAGE_PATH (a Japanese lesson with ruby), and MUTATE.
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const READING_CHOICES = ['serif', 'sans', 'kai'];
const EXCLUDED = 'code, pre, kbd, samp, button, input, select, textarea, code *, pre *, kbd *, samp *, button *, select *, textarea *';
const FAMILY_SELECTOR = `.y-prose :where(p, rt, [data-level], [lang]):lang(ja):not(:where(${EXCLUDED}))`;
const RESET_SELECTOR = `.y-prose [lang]:not(:lang(ja)):not(:where(${EXCLUDED}))`;
const SANS_PROSE_SELECTOR = '.y-prose :where(.footnotes, th, .embed__source, .embed__note, .callout-title):lang(ja)';
const SERIF_PROSE_SELECTOR = '.y-prose :where(td, .callout-body):lang(ja)';
const PROSE_FACES = [
  ['.callout-body', 'serif'],
  ['.callout-title', 'sans'],
  ['.embed__note', 'sans'],
  ['.embed__source', 'sans'],
  ['.footnotes', 'sans'],
  ['td', 'serif'],
  ['th', 'sans'],
];
const MODES = {
  'drop-japanese-family': { selector: FAMILY_SELECTOR, property: 'font-family', value: 'inherit', font: 'serif', target: 'japanese-paragraph-and-reading' },
  'use-serif-for-sans': { selector: ":root[data-font='sans']", property: '--font-read-ja', value: 'var(--font-serif-ja)', font: 'sans', target: 'japanese-paragraph-and-reading' },
  'use-serif-for-kai': { selector: ":root[data-font='kai']", property: '--font-read-ja', value: 'var(--font-serif-ja)', font: 'kai', target: 'japanese-paragraph-and-reading' },
  'drop-nonjapanese-reset': { selector: RESET_SELECTOR, property: 'font-family', value: 'inherit', font: 'serif', target: 'nested-language-returns-to-ordinary-face' },
  'drop-code-and-control-exclusions': { selector: FAMILY_SELECTOR, property: 'font-family', font: 'serif', target: 'code-and-controls-keep-their-faces', removeExclusions: true },
  'add-unchecked-typeface': { catalog: true, target: 'whole-font-choice-catalog' },
  'drop-japanese-sans-prose': { selector: SANS_PROSE_SELECTOR, property: 'font-family', removeRule: true, font: 'serif', target: 'japanese-authored-prose-roles' },
  'drop-japanese-serif-prose': { selector: SERIF_PROSE_SELECTOR, property: 'font-family', removeRule: true, font: 'serif', target: 'japanese-authored-prose-roles' },
  'add-unchecked-prose-family': { selector: SANS_PROSE_SELECTOR, property: 'font-family', proseCatalog: true, font: 'serif', target: 'whole-prose-family-catalog' },
};
class NotApplied extends Error {}
class LockFired extends Error {
  constructor(site, detail) { super(`caught: ${site}: ${detail}`); this.site = site; }
}
const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const mutate = async (page, mode) => {
  const proof = { requests: 0, blocks: 0, declarations: 0, changes: 0, errors: [] };
  await page.route((url) => url.origin === new URL(BASE).origin && url.pathname === '/static/app.css', async (route) => {
    proof.requests += 1;
    try {
      const response = await route.fetch();
      if (response.status() !== 200) throw new Error(`stylesheet status ${response.status()}`);
      const css = await response.text();
      const blocks = [...css.matchAll(/[^{}]+\{[^{}]*\}/g)].filter(([block]) => block.slice(0, block.indexOf('{')).replace(/\/\*[\s\S]*?\*\//g, '').split(',\n').some((selector) => selector.trim() === mode.selector));
      proof.blocks += blocks.length;
      let body = css;
      if (blocks.length === 1) {
        const block = blocks[0][0];
        const declaration = new RegExp(`(^|[;{]\\s*)${escaped(mode.property)}\\s*:[^;]+;`, 'g');
        const declarations = [...block.matchAll(declaration)];
        proof.declarations += declarations.length;
        if (declarations.length === 1) {
          const exclusion = `:not(:where(${EXCLUDED}))`;
          if (mode.removeExclusions && block.split(exclusion).length !== 2) {
            await route.fulfill({ response });
            return;
          }
          let changed;
          if (mode.removeRule) changed = '';
          else if (mode.proseCatalog) changed = `${block}\n.y-prose .unchecked-prose-family { font-family: var(--font-sans); }\n`;
          else changed = mode.removeExclusions ? block.replace(exclusion, '') : block.replace(declaration, `$1${mode.property}: ${mode.value};`);
          if (changed !== block) proof.changes += 1;
          body = css.replace(block, changed);
        }
      }
      await route.fulfill({ response, body });
    } catch (error) {
      proof.errors.push(error.message);
      await route.abort();
    }
  });
  return () => {
    if (proof.errors.length) throw new Error(proof.errors.join('; '));
    if (proof.requests !== 1 || proof.blocks !== 1 || proof.declarations !== 1 || proof.changes !== 1) throw new NotApplied(`not-applied: ${JSON.stringify(proof)}`);
    console.log(`applied: ${MUTATE} ${JSON.stringify(proof)}`);
  };
};

const family = (value) => value.split(',').map((name) => name.trim().replace(/^['"]|['"]$/g, ''));
const japanese = (value, choice) => {
  const names = family(value);
  const jp = names.indexOf(choice === 'sans' ? 'Noto Sans JP' : 'Noto Serif JP');
  const system = names.indexOf(choice === 'sans' ? 'Hiragino Sans' : 'Hiragino Mincho ProN');
  const tc = names.findIndex((name) => /(?: TC| SC)$/.test(name));
  return jp >= 0 && system > jp && tc > system &&
    (choice === 'sans' ? names[0] === 'Geist' && !names.includes('Hiragino Mincho ProN') :
      choice === 'kai' ? names.includes('Kaiti TC') && names[0] === 'Noto Serif JP' : names[0] === 'Newsreader' && !names.includes('Kaiti TC'));
};

// The settings page is the offered choice set. Pin every member before driving
// it so an added face demands a glyph-selection case instead of going untested.
const offeredChoices = async (browser, lang) => {
  const context = await browser.newContext();
  try {
    await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }]);
    const page = await context.newPage();
    const proof = { requests: 0, blocks: 0, changes: 0, errors: [] };
    if (MODES[MUTATE]?.catalog) {
      await page.route((url) => url.origin === new URL(BASE).origin && url.pathname === '/preferences', async (route) => {
        proof.requests += 1;
        try {
          const response = await route.fetch();
          if (response.status() !== 200) throw new Error(`settings status ${response.status()}`);
          const html = await response.text();
          const blocks = [...html.matchAll(/<fieldset\b(?=[^>]*\bdata-pref-field="font")[^>]*>[\s\S]*?<\/fieldset>/g)];
          proof.blocks += blocks.length;
          let body = html;
          if (blocks.length === 1) {
            const extra = '<label><input type="radio" name="font" value="unchecked-face"><span>Untracked font</span></label>';
            const changed = blocks[0][0].replace('</fieldset>', `${extra}</fieldset>`);
            if (changed !== blocks[0][0]) proof.changes += 1;
            body = html.replace(blocks[0][0], changed);
          }
          await route.fulfill({ response, body });
        } catch (error) { proof.errors.push(error.message); await route.abort(); }
      });
    }
    const response = await page.goto(`${BASE}/preferences`, { waitUntil: 'load' });
    if (!response || response.status() !== 200) throw new Error(`settings status ${response?.status()}`);
    if (MODES[MUTATE]?.catalog) {
      if (proof.errors.length) throw new Error(proof.errors.join('; '));
      if (proof.requests !== 1 || proof.blocks !== 1 || proof.changes !== 1) throw new NotApplied(`not-applied: catalog ${JSON.stringify(proof)}`);
    }
    const fields = page.locator('[data-pref-field="font"]');
    if (await fields.count() !== 1) throw new Error('settings have no unique font-choice field');
    const values = await fields.locator('input[type="radio"][name="font"]').evaluateAll((nodes) => nodes.map((node) => node.value));
    if (MODES[MUTATE]?.catalog) {
      if (proof.errors.length) throw new Error(proof.errors.join('; '));
      if (proof.requests !== 1 || proof.blocks !== 1 || proof.changes !== 1 || values.filter((value) => value === 'unchecked-face').length !== 1) throw new NotApplied(`not-applied: catalog ${JSON.stringify(proof)} values=${JSON.stringify(values)}`);
      console.log(`applied: ${MUTATE} actual offered set=${JSON.stringify(values)}`);
    }
    console.log(`invoked: offered font choices ${lang} ${JSON.stringify(values)}`);
    if (JSON.stringify([...values].sort()) !== JSON.stringify([...READING_CHOICES].sort())) throw new LockFired('whole-font-choice-catalog', `actual settings offer ${JSON.stringify(values)}, expected every case ${JSON.stringify(READING_CHOICES)}`);
    return values;
  } finally { await context.close(); }
};

if (MUTATE === 'list') { for (const mode of Object.keys(MODES)) console.log(mode); process.exit(0); }
if (MUTATE && !Object.hasOwn(MODES, MUTATE)) { console.error(`not-applied: unknown mode ${MUTATE}`); process.exit(2); }
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let measured = 0;
try {
  const offered = await offeredChoices(browser, 'en');
  const translated = await offeredChoices(browser, 'zh-Hant');
  if (JSON.stringify(offered) !== JSON.stringify(translated)) throw new LockFired('whole-font-choice-catalog', 'the interface languages offer different font choices');
  const choices = MUTATE ? [MODES[MUTATE].font] : ['', ...offered];
  for (const choice of choices) for (const lang of ['en', 'zh-Hant']) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: theme });
    try {
      await context.addCookies([
        { name: 'yomihon_lang', value: lang, url: BASE },
        { name: 'yomihon_theme', value: theme, url: BASE },
        ...(choice ? [{ name: 'yomihon_font', value: choice, url: BASE }] : []),
      ]);
      const page = await context.newPage();
      const proof = MUTATE ? await mutate(page, MODES[MUTATE]) : null;
      const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
      if (!response || response.status() !== 200) throw new Error(`lesson status ${response?.status()}`);
      await page.evaluate(() => document.fonts.ready);
      await arrived(page);
      if (proof) proof();
      // Inspect authored components before moving the paragraph's language
      // boundary. Their original family declarations own the complete set;
      // the expected role catalog is independent of the coverage selectors.
      const proseFacts = await page.evaluate((roles) => {
        const article = document.querySelector('.y-article');
        const prose = article?.querySelector('.y-prose');
        if (!article || !prose || !article.matches(':lang(ja)')) throw new Error('original authored lesson is not Japanese');
        const declared = [];
        const walkRules = (rules) => {
          for (const rule of rules) {
            const ordinary = rule.style?.fontFamily;
            if (rule.selectorText?.includes('.y-prose') && ['var(--font-sans)', 'var(--font-serif)'].includes(ordinary)) {
              for (const selector of rule.selectorText.split(',').map((value) => value.trim())) {
                const match = /^\.y-prose (\.[\w-]+|th|td)$/.exec(selector);
                if (!match) throw new Error(`unreadable prose family declaration ${selector}`);
                declared.push([match[1], ordinary === 'var(--font-sans)' ? 'sans' : 'serif']);
              }
            }
            if (rule.cssRules) walkRules(rule.cssRules);
          }
        };
        for (const sheet of document.styleSheets) {
          if ((sheet.href || '').includes('/static/app.css')) walkRules(sheet.cssRules);
        }
        declared.sort(([a], [b]) => a.localeCompare(b, 'en'));
        const sites = roles.flatMap(([selector, role]) => {
          const nodes = [...prose.querySelectorAll(selector)];
          if (!nodes.length) throw new Error(`missing authored prose component ${selector}`);
          return nodes.map((node) => ({ selector, role, tag: node.tagName, japanese: node.matches(':lang(ja)'), family: getComputedStyle(node).fontFamily, text: node.textContent }));
        });
        return { declared, sites };
      }, PROSE_FACES);
      console.log(`invoked: authored prose family catalog ${JSON.stringify(proseFacts)}`);
      if (JSON.stringify(proseFacts.declared) !== JSON.stringify(PROSE_FACES)) throw new LockFired('whole-prose-family-catalog', `actual declarations ${JSON.stringify(proseFacts.declared)}, expected every case ${JSON.stringify(PROSE_FACES)}`);
      const expectedRoles = {
        sans: ['Geist', 'Noto Sans JP', 'Hiragino Sans', 'Noto Sans TC', 'PingFang TC', 'system-ui', 'sans-serif'],
        serif: ['Newsreader', 'Noto Serif JP', 'Hiragino Mincho ProN', 'Noto Serif TC', 'Songti TC', 'Georgia', 'serif'],
      };
      const mismatches = proseFacts.sites.filter((site) => !site.japanese || JSON.stringify(family(site.family)) !== JSON.stringify(expectedRoles[site.role]));
      if (mismatches.length) throw new LockFired('japanese-authored-prose-roles', JSON.stringify(mismatches));
      // The original lesson supplies the paragraph and ruby; only its language
      // boundary is moved to exercise whole-note and mixed-note inheritance.
      for (const boundary of ['article', 'div', 'paragraph', 'subtag']) {
        const facts = await page.evaluate(({ boundary }) => {
          const article = document.querySelector('.y-article');
          const prose = article?.querySelector('.y-prose');
          const p = prose?.querySelector(':scope > p:has(rt), :scope > .y-reading > p:has(rt), :scope > [data-font-boundary] > p:has(rt)');
          const rt = p?.querySelector('rt');
          if (!article || !prose || !p || !rt || !p.textContent.trim() || !rt.textContent.trim()) throw new Error('missing original Japanese paragraph/readings');
          let reading = p.closest('.y-reading, [data-font-boundary]');
          if (!reading) {
            if (!p.matches(':lang(ja)')) throw new Error('original ruby paragraph is not Japanese');
            reading = document.createElement('div'); reading.dataset.fontBoundary = '1';
            p.before(reading); reading.append(p);
          }
          article.lang = boundary === 'article' ? 'ja' : 'zh-Hant';
          reading.removeAttribute('lang');
          p.removeAttribute('lang');
          if (boundary === 'div') reading.lang = 'ja';
          if (boundary === 'paragraph' || boundary === 'subtag') p.lang = boundary === 'subtag' ? 'ja-JP' : 'ja';
          let controls = prose.querySelector('[data-font-probe]');
          if (!controls) {
            controls = document.createElement('div'); controls.dataset.fontProbe = '1';
            const nested = document.createElement('span'); nested.lang = 'zh-Hant'; nested.textContent = '中文'; nested.dataset.nestedLanguage = '1'; p.append(nested);
            const code = document.createElement('code'); code.lang = 'ja'; const span = document.createElement('span'); span.lang = 'ja'; span.textContent = 'fmt.Println'; code.append(span); controls.append(code);
            const button = document.createElement('button'); button.className = 'y-langbtn'; button.lang = 'ja'; button.textContent = '送信'; controls.append(button);
            prose.append(controls);
          }
          const ordinary = getComputedStyle(document.documentElement).getPropertyValue('--font-read').trim();
          const root = document.documentElement;
          const style = (node) => getComputedStyle(node).fontFamily;
          return { lang: root.lang, theme: root.dataset.theme, choice: root.dataset.font || '', paragraph: style(p), reading: style(rt), nested: style(p.querySelector('[data-nested-language]')), ordinary, code: style(controls.querySelector('code')), codeChild: style(controls.querySelector('code span')), button: style(controls.querySelector('button')), mono: getComputedStyle(root).getPropertyValue('--font-mono').trim(), sans: getComputedStyle(root).getPropertyValue('--font-sans').trim(), matchesJapanese: p.matches(':lang(ja)') && rt.matches(':lang(ja)'), text: p.textContent };
        }, { boundary });
        if (facts.lang !== lang || facts.theme !== theme || facts.choice !== choice || !facts.matchesJapanese) throw new Error(`unexpected reading context ${JSON.stringify(facts)}`);
        const selected = choice || 'serif';
        if (proof) {
          if (MUTATE === 'drop-japanese-family' && japanese(facts.paragraph, selected)) throw new NotApplied('not-applied: Japanese family still wins');
          if (MUTATE.startsWith('use-serif-') && !japanese(facts.paragraph, 'serif')) throw new NotApplied('not-applied: serif override not computed');
          if (MUTATE === 'drop-nonjapanese-reset' && !japanese(facts.nested, selected)) throw new NotApplied('not-applied: nested text did not inherit Japanese family');
          if (MUTATE === 'drop-code-and-control-exclusions' && (!japanese(facts.code, selected) || !japanese(facts.codeChild, selected) || !japanese(facts.button, selected))) throw new NotApplied('not-applied: code/control override not computed');
        }
        console.log(`invoked: computed Japanese paragraph/readings ${choice || 'default'}/${lang}/${theme}/${boundary} ${JSON.stringify(facts)}`);
        if (!japanese(facts.paragraph, selected) || !japanese(facts.reading, selected) || facts.paragraph !== facts.reading) throw new LockFired('japanese-paragraph-and-reading', JSON.stringify(facts));
        if (JSON.stringify(family(facts.nested)) !== JSON.stringify(family(facts.ordinary))) throw new LockFired('nested-language-returns-to-ordinary-face', JSON.stringify(facts));
        if (JSON.stringify(family(facts.code)) !== JSON.stringify(family(facts.mono)) || facts.code !== facts.codeChild || JSON.stringify(family(facts.button)) !== JSON.stringify(family(facts.sans))) throw new LockFired('code-and-controls-keep-their-faces', JSON.stringify(facts));
        measured += 1;
      }
    } finally { await context.close(); }
  }
  console.log(`PASS japanese-reading-face: ${measured} complete computed paragraph/rt/preference/language-boundary cases plus seven authored prose roles`);
} catch (error) {
  console.error(error.message || error);
  if (error instanceof NotApplied) { console.log(`MUTATE-RESULT: not-applied ${MUTATE}`); process.exitCode = 2; }
  else { if (MUTATE && error instanceof LockFired && error.site === MODES[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`); process.exitCode = 1; }
} finally { await browser.close(); }
