// The drawn image is named by its own preserved source unless the author
// supplied Mermaid accessibility metadata. Names survive a theme redraw.
// The existing note-response harness supplies real Mermaid input before boot;
// no tracked note or renderer fixture is changed.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SOURCES = [
  'graph TD\n  Alpha --> Beta',
  'graph LR\n  Gamma --> Delta',
  'flowchart LR\n  accTitle: A named learning path\n  accDescr: One topic leads to another.\n  First --> Next',
];
const label = "          svgElement.setAttribute('aria-labelledby', alternative.id);";
const preserve = "        if (!svgElement.getAttribute('aria-label')?.trim() && !svgElement.getAttribute('aria-labelledby')?.trim()) {";
const MUTATIONS = {
  'drop-source-label': { site: 'source-name', needle: label, replacement: '' },
  'reuse-source-id': { site: 'distinct-source-names', needle: `        alternative.id = \`\${id}-source\`;`, replacement: "        alternative.id = 'mermaid-diagram-source';" },
  'overwrite-authored-name': { site: 'authored-name', needle: preserve, replacement: '        if (true) {' },
  'skip-redraw-label': { site: 'redraw-name', needle: label, replacement: `          if (next <= blocks.length) ${label.trim()}` },
};
const SITES = ['source-name', 'distinct-source-names', 'authored-name', 'redraw-name'];
if (Object.keys(MUTATIONS).length !== SITES.length || SITES.some((site) => !Object.values(MUTATIONS).some((entry) => entry.site === site))) {
  console.error('mermaid-name: every named assertion needs a mutation');
  process.exit(2);
}
if (MUTATE === 'list') {
  for (const mode of Object.keys(MUTATIONS)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`mermaid-name: unknown mutation ${MUTATE}`);
  process.exit(2);
}
class Broken extends Error {}
class NotApplied extends Error {}
class LockFired extends Error {
  constructor(site, message) { super(message); this.site = site; }
}
const fail = (site, message) => { throw new LockFired(site, message); };
const text = (value) => value.replace(/\s+/gu, ' ').trim();
const escapeHTML = (value) => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const html = SOURCES.map((source) => `<div class="mermaid-diagram" data-mermaid-code="${encodeURIComponent(source)}">${escapeHTML(source)}</div>`).join('');
const arrived = (page) => page.waitForFunction(async () => {
  if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
  await Promise.all(document.getAnimations().filter((animation) => animation.animationName === 'y-come-forward').map((animation) => animation.finished.catch(() => {})));
  return true;
}, null, { timeout: 3000 });

// Read Chrome's committed accessibility tree, rather than treating a nonempty
// attribute or a guessed ARIA calculation as an accessible name.
const names = async (session) => {
  const { root } = await session.send('DOM.getDocument');
  const { nodeIds } = await session.send('DOM.querySelectorAll', { nodeId: root.nodeId, selector: '.mermaid-diagram > svg' });
  if (nodeIds.length !== SOURCES.length) throw new Broken(`drawn diagram set has ${nodeIds.length} members, want ${SOURCES.length}`);
  const answer = [];
  for (const nodeId of nodeIds) {
    const { nodes } = await session.send('Accessibility.getPartialAXTree', { nodeId, fetchRelatives: false });
    const image = nodes.find((node) => !node.ignored && node.role?.value === 'image');
    if (!image) throw new Broken('a drawn SVG was not exposed as an image');
    answer.push({ name: image.name?.value || '', description: image.description?.value || '' });
  }
  return answer;
};
let browser;
try {
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  for (const width of [1280, 390]) {
    for (const lang of ['zh-Hant', 'en']) {
      for (const theme of ['light', 'dark']) {
        const context = await browser.newContext({ viewport: { width, height: 900 }, colorScheme: theme });
        try {
          await context.addCookies([{ name: 'yomihon_lang', value: lang, url: BASE }, { name: 'yomihon_theme', value: theme, url: BASE }]);
          let injected = -1;
          await context.route(BASE + PAGE, async (route) => {
            const response = await route.fetch();
            const body = await response.text();
            const needle = '<div class="y-prose">';
            injected = body.split(needle).length - 1;
            await route.fulfill({ response, body: injected === 1 ? body.replace(needle, needle + html) : body });
          });
          let matches = -1;
          if (MUTATE) await context.route('**/static/diagrams.js{,?*}', async (route) => {
            const response = await route.fetch();
            const body = await response.text();
            const mutation = MUTATIONS[MUTATE];
            matches = body.split(mutation.needle).length - 1;
            await route.fulfill({ response, body: matches === 1 ? body.replace(mutation.needle, mutation.replacement) : body });
          });
          const page = await context.newPage();
          const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
          if (response?.status() !== 200 || injected !== 1) throw new Broken(`note/injection setup returned ${response?.status()}/${injected}, want 200/1`);
          await arrived(page);
          await page.waitForFunction((count) => document.querySelectorAll('.mermaid-diagram > svg').length === count, SOURCES.length, { timeout: 5000 });
          console.log(`invocation-hit mermaid-name initial ${width} ${lang} ${theme}: real SVG set rendered`);
          if (MUTATE && matches !== 1) throw new NotApplied(`diagrams mutation matched ${matches} sites, want 1`);
          const session = await context.newCDPSession(page);
          await session.send('Accessibility.enable');
          const initial = await names(session);
          if (initial[0].name !== text(SOURCES[0])) fail('source-name', `first native image name ${JSON.stringify(initial[0].name)}, want ${JSON.stringify(text(SOURCES[0]))}`);
          if (initial[1].name !== text(SOURCES[1])) fail('distinct-source-names', `second native image name ${JSON.stringify(initial[1].name)}, want its own source ${JSON.stringify(text(SOURCES[1]))}`);
          if (initial[2].name !== 'A named learning path' || initial[2].description !== 'One topic leads to another.') {
            fail('authored-name', `author metadata became ${JSON.stringify(initial[2])}`);
          }
          const oldIds = await page.locator('.mermaid-diagram > svg').evaluateAll((elements) => elements.map((element) => element.id));
          const themeControl = page.locator('[data-theme-toggle]');
          // At a narrow window the same native control lives in the header's
          // popover, so a reader opens it before choosing a theme.
          if (!(await themeControl.isVisible())) await page.click('[popovertarget="_y-header-fold"]');
          await themeControl.click();
          await page.waitForFunction((previous) => {
            const elements = [...document.querySelectorAll('.mermaid-diagram > svg')];
            return elements.length === previous.length && elements.every((element, index) => element.id !== previous[index]);
          }, oldIds, { timeout: 5000 });
          console.log(`invocation-hit mermaid-name redraw ${width} ${lang} ${theme}: real SVG set replaced`);
          const redrawn = await names(session);
          if (JSON.stringify(redrawn) !== JSON.stringify(initial)) fail('redraw-name', `redrawn native names ${JSON.stringify(redrawn)}, want ${JSON.stringify(initial)}`);
          console.log(`PASS mermaid-name ${width} ${lang} ${theme}: ${JSON.stringify(redrawn)}`);
          await session.detach();
        } finally {
          await context.close();
        }
      }
    }
  }
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.log(`caught: mermaid-name ${error.site}`);
    if (MUTATE && MUTATIONS[MUTATE].site === error.site) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 2;
  }
} finally {
  try { await browser?.close(); } catch (error) {
    console.error(`mermaid-name browser cleanup: ${error.message}`);
    process.exitCode = 2;
  }
}
