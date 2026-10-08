// A long reading keeps every block in the document while the browser defers
// work outside the viewport. Paper needs every block at once. Read the stable
// declarations, not a transient skipped state or a rendering-time threshold.
// Env: YOMIHON_BASE, PAGE_PATH, MUTATE (list prints all watched regressions).
import { chromium } from 'playwright-core';
import { readFile, writeFile, unlink } from 'node:fs/promises';
import { createHash } from 'node:crypto';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = '/notes/Notes/mark-long-offset.md';
const OUTLINE_PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const MUTATE = process.env.MUTATE || '';
const CHILDREN = '.y-prose > *';

class LockFired extends Error {
  constructor(site, message) {
    super(`caught: prose-visibility: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}

const screen = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*auto\s*;/g;
const intrinsic = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)contain-intrinsic-block-size:\s*auto\s+[\d.]+em\s*;/g;
// Indented: the print rule sits inside its media block, unlike the screen
// semantic exceptions and the bounded prefix measurement on screen.
const paper = /(^[ \t]+\.y-prose\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*visible\s*;/gm;
const gutter = /(\.y-prose\s+\.y-reading\s*\{[^}]*?)overflow-clip-margin:\s*[\d.]+px\s*;/g;
const focus = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)overflow-clip-margin:\s*[\d.]+px\s*;/g;
const sheet = /(\.y-conceptsheet__body\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*visible\s*;/g;
const preview = /if \(!card.contains\(event.target\) && event.timeStamp >= askedAt\) close\(\);/g;
const focusQuestion = /link.addEventListener\('focus', \(event\) => schedule\(link, openDelay, event.timeStamp\)\);/g;
const MUTATIONS = {
  'hide-accessible-tail': {
    site: 'accessibility-tail', needle: screen,
    replace: '$1content-visibility: auto; }\n.y-prose > :nth-last-child(-n+3) { content-visibility: hidden;',
  },
  'render-offscreen-blocks': { site: 'screen', needle: screen, replace: '$1' },
  'omit-paragraph-blocks': {
    site: 'screen', needle: screen,
    replace: (match) => match.replace(/\*\s*\{/, ':not(p) {'),
  },
  'forget-block-size': { site: 'intrinsic', needle: intrinsic, replace: '$1' },
  'reserve-inline-size': {
    site: 'intrinsic', needle: intrinsic,
    replace: (match) => match.replace('contain-intrinsic-block-size', 'contain-intrinsic-inline-size'),
  },
  'defer-print-blocks': { site: 'print', needle: paper, replace: '$1' },
  'clip-speaking-control': { site: 'speech', needle: gutter, replace: '$1contain: paint; overflow-clip-margin: 0px;' },
  'clip-prose-focus': { site: 'focus', needle: focus, replace: '$1' },
  'cancel-new-hover-with-queued-scroll': {
    site: 'preview', asset: 'preview.js', needle: preview,
    replace: 'if (!card.contains(event.target)) close();',
  },
  'keep-preview-after-new-scroll': {
    site: 'preview-scroll', asset: 'preview.js', needle: preview,
    replace: 'if (false && !card.contains(event.target)) close();',
  },
  'discard-focused-preview-time': {
    site: 'preview-focus', asset: 'preview.js', needle: focusQuestion,
    replace: "link.addEventListener('focus', (event) => schedule(link, openDelay, 0));",
  },
  'defer-outline-columns': {
    site: 'outline',
    needle: /\.y-prose:has\(> \[data-level\] ~ \[data-level\]\) > \*,\n/g, replace: '',
  },
  'defer-nested-outline-column': {
    site: 'nested-outline',
    needle: /\.y-prose:has\(> :not\(\[data-level\]\) \[data-level\]\) > \*,\n/g, replace: '',
  },
  'defer-addressed-blocks': {
    site: 'addresses',
    needle: /\.y-prose > \[id\],\n\.y-prose > :has\(\[id\]\),\n/g, replace: '',
  },
  'defer-authored-controls': {
    site: 'controls',
    needle: /\.y-prose > :has\(input, select, textarea, button, summary, a\[href\], audio\[controls\], video\[controls\], \[contenteditable='true'\], \[tabindex\]\),\n/g, replace: '',
  },
  'defer-preview-passage': {
    site: 'preview-layout',
    needle: /(\.y-preview__body,\n\.y-preview \.y-prose > \* \{[^}]*?)content-visibility: visible;/g, replace: '$1content-visibility: auto;',
  },
  'defer-concept-sections': { site: 'sheet', needle: sheet, replace: '$1content-visibility: auto !important;' },
};
const SITES = ['accessibility-tail', 'screen', 'intrinsic', 'print', 'speech', 'sheet', 'focus', 'preview', 'preview-scroll', 'preview-focus', 'outline', 'controls', 'preview-layout', 'addresses', 'nested-outline'];
if (SITES.some((site) => !Object.values(MUTATIONS).some((mode) => mode.site === site))
  || Object.values(MUTATIONS).some((mode) => !SITES.includes(mode.site))) {
  throw new Error('BROKEN prose-visibility: mutation inventory differs from assertion sites');
}
if (MUTATE === 'list') {
  for (const mode of Object.keys(MUTATIONS)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`prose-visibility: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const declarations = (page) => page.locator(CHILDREN).evaluateAll((nodes) => nodes.map((element, index) => {
  const style = getComputedStyle(element);
  return {
    controls: element.matches('.y-reading, input, select, textarea, button, summary, a[href], audio[controls], video[controls], [contenteditable="true"], [tabindex]')
      || Boolean(element.querySelector('input, select, textarea, button, summary, a[href], audio[controls], video[controls], [contenteditable="true"], [tabindex]')),
    index, tag: element.tagName, addressed: element.hasAttribute('id') || Boolean(element.querySelector('[id]')), visibility: style.contentVisibility,
    block: style.containIntrinsicBlockSize, inline: style.containIntrinsicInlineSize, clip: style.overflowClipMargin,
  };
}));
const assert = (site, condition, message) => {
  if (!condition) throw new LockFired(site, message);
};

// Compare exact main asset bytes through in-memory routes. The fixture records
// only reversible source deltas and both hashes; a changed source is refused.
async function measureChapters(browser) {
  const root = process.env.YOMIHON_FIXTURE_ROOT;
  if (!root) throw new Error('BROKEN prose-visibility: measurement requires the disposable fixture vault');
  const baseline = JSON.parse(await readFile(new URL('./fixtures/prose-baseline.json', import.meta.url), 'utf8'));
  const assets = [];
  for (const asset of baseline.assets) {
    const head = await readFile(new URL(`../../${asset.path}`, import.meta.url), 'utf8');
    if (createHash('sha256').update(head).digest('hex') !== asset.headHash) throw new Error(`BROKEN baseline: stale head ${asset.path}`);
    let main = head;
    for (const edit of asset.edits) {
      if (main.split(edit.head).length !== 2) throw new Error(`BROKEN baseline: ambiguous delta ${asset.path}`);
      main = main.replace(edit.head, edit.main);
    }
    if (createHash('sha256').update(main).digest('hex') !== asset.mainHash) throw new Error(`BROKEN baseline: wrong main ${asset.path}`);
    assets.push({ ...asset, head, main });
  }
  console.log(`MEASURE prose-visibility baseline=${baseline.commit} viewport=1280x800 median=3 same rendered documents; exact CSS/mark/preferences/preview source hashes verified`);
  const owned = [];
  try {
    for (const [kb, chaptered] of [[24, true], [79, true], [240, true], [240, false]]) {
      const name = `prose-measure-${kb}-${chaptered ? 'chapters' : 'single'}.md`;
      let markdown = '---\ntitle: Measured reading\ntype: inbox\n---\n\n## First section\n\n';
      let index = 0;
      while (Buffer.byteLength(markdown) < kb * 1024) {
        if (chaptered) markdown += `## Section ${index}\n\n`;
        markdown += `Paragraph ${index} follows [[Glass Tide]] and its footnote.[^note-${index}] The reader changes the measure and light while following authored words.\n\n- First list item\n- Second list item\n\n\`\`\`go\nfunc section${index}() string { return "reading" }\n\`\`\`\n\n| Term | Meaning |\n| --- | --- |\n| Reading | <ruby>読む<rt>よむ</rt></ruby> |\n\n[^note-${index}]: Footnote ${index}.\n\n`;
        index += 1;
      }
      markdown += 'Measurement end.\n';
      const file = `${root}/Notes/${name}`;
      await writeFile(file, markdown, { flag: 'wx' });
      owned.push(file);
      const address = `${BASE}/notes/Notes/${name}`;
      let ready = false;
      for (let attempt = 0; attempt < 20; attempt += 1) {
        const response = await browser.newContext();
        try {
          const document = await response.request.get(address);
          ready = document.ok() && (await document.text()).includes('Measurement end.');
        } finally { await response.close(); }
        if (ready) break;
        await new Promise((resolve) => setTimeout(resolve, 500));
      }
      if (!ready) throw new Error(`BROKEN measurement: fixture scan did not admit ${name}`);
      const samples = { main: [], head: [] };
      for (let repetition = 0; repetition < 3; repetition += 1) {
        for (const variant of repetition % 2 ? ['head', 'main'] : ['main', 'head']) {
          const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: 'light' });
          try {
            const page = await context.newPage();
            let routed = 0;
            if (variant === 'main') {
              for (const asset of assets) {
                const endpoint = asset.path.endsWith('.css') ? 'app.css' : asset.path.split('/').at(-1);
                await page.route(`**/static/${endpoint}{,?*}`, async (route) => {
                  const response = await route.fetch();
                  const served = await response.text();
                  const body = asset.path.endsWith('.css') ? served.replace(asset.head, asset.main) : asset.main;
                  if (asset.path.endsWith('.css') && served.split(asset.head).length !== 2) throw new Error('BROKEN measurement: served stylesheet differs from source');
                  routed += 1;
                  await route.fulfill({ response, body });
                });
              }
            }
            const response = await page.goto(address, { waitUntil: 'domcontentloaded' });
            if (response?.status() !== 200) throw new Error(`BROKEN measurement: ${name} HTTP ${response?.status()}`);
            await page.waitForFunction(() => document.documentElement.dataset.js === 'on');
            await page.evaluate(async () => {
              await document.fonts.ready;
              await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
            });
            const timings = await page.evaluate(async () => {
              const press = async (selector) => {
                const button = document.querySelector(selector);
                if (!button) throw new Error(`missing real control ${selector}`);
                const start = performance.now();
                button.click();
                await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
                return performance.now() - start;
              };
              const dcl = performance.getEntriesByType('navigation')[0].domContentLoadedEventEnd;
              const size = await press('[data-textsize-toggle]');
              const theme = await press('[data-theme-toggle]');
              if (document.documentElement.dataset.textsize !== 'l' || document.documentElement.dataset.theme !== 'dark') throw new Error('measurement controls did not change the real preferences');
              return { dcl, size, theme, nodes: document.querySelectorAll('*').length,
                deferred: [...document.querySelectorAll('.y-prose > *')].filter((block) => getComputedStyle(block).contentVisibility === 'auto').length };
            });
            if (variant === 'main' && routed !== assets.length) throw new Error(`BROKEN measurement: ${routed} baseline assets, want ${assets.length}`);
            samples[variant].push(timings);
          } finally { await context.close(); }
        }
      }
      const medians = Object.fromEntries(Object.entries(samples).map(([variant, values]) => [variant,
        Object.fromEntries(['dcl', 'size', 'theme', 'nodes', 'deferred'].map((key) => [key, values.map((value) => value[key]).sort((a, b) => a - b)[1]]))]));
      console.log(`MEASURE prose-visibility ${name} bytes=${Buffer.byteLength(markdown)} ${JSON.stringify(medians)}`);
    }
  } finally { await Promise.all(owned.map((file) => unlink(file))); }
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  if (!MUTATE) await measureChapters(browser);
  for (const width of [1280, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage();
    let served = 0;
    let matches = 0;
    if (MUTATE) {
      const mode = MUTATIONS[MUTATE];
      await page.route(`**/static/${mode.asset || 'app.css'}{,?*}`, async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        served += 1;
        const count = [...original.matchAll(mode.needle)].length;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(mode.needle, mode.replace) : original });
      });
    }
    await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
    if (MUTATE && (served !== 1 || matches !== 1)) {
      throw new NotApplied(`NOT-APPLIED prose-visibility: ${MUTATE} served ${served} stylesheets, matched ${matches} sites; want one each`);
    }
    // Read the unignored AX text before asking for tail geometry or scrolling.
    console.log(`INVOKED prose-visibility accessibility-tail ${width}px initial document`);
    const session = await context.newCDPSession(page);
    try {
      const { nodes } = await session.send('Accessibility.getFullAXTree');
      assert('accessibility-tail', nodes.some((node) => !node.ignored
        && node.name?.value.includes('Reading paragraph 140.')),
      `${width}px the unread tail is absent from the initial accessibility tree`);
    } finally { await session.detach(); }
    const topBeforeTab = await page.evaluate(() => window.scrollY);
    if (topBeforeTab !== 0) throw new Error('BROKEN prose-visibility: AX tail was inspected after scrolling');
    const workload = await page.evaluate(() => ({ height: innerHeight, document: document.documentElement.scrollHeight }));
    const blocks = await declarations(page);
    if (MUTATE === 'omit-paragraph-blocks' && !blocks.some((block) => block.tag === 'P')) {
      throw new NotApplied('NOT-APPLIED prose-visibility: omit-paragraph-blocks has no paragraph to omit');
    }
    if (blocks.length === 0 || workload.document <= workload.height * 2) {
      throw new Error(`BROKEN prose-visibility: ${PAGE} is not a nonempty multi-viewport reading`);
    }
    assert('screen', blocks.some((block) => !block.addressed && !block.controls) && blocks.filter((block) => !block.addressed && !block.controls).every((block) => block.visibility === 'auto'),
      `${width}px screen blocks do not follow addressed-block and eligible-tail declarations: ${JSON.stringify(blocks.filter((block) => block.visibility !== 'auto'))}`);
    assert('addresses', blocks.some((block) => block.addressed) && blocks.filter((block) => block.addressed).every((block) => block.visibility === 'visible'),
      `${width}px an addressed authored block defers its geometry`);
    // Engines can serialize the unused axis as "auto none" when the other
    // axis remembers its size. Neither spelling reserves an inline fallback.
    assert('intrinsic', blocks.every((block) => /^auto [\d.]+px$/.test(block.block)
      && Number.parseFloat(block.block.slice(5)) > 0 && /^(auto )?none$/.test(block.inline)),
    `${width}px blocks do not remember a positive block-size fallback without reserving inline space: ${JSON.stringify(blocks)}`);
    // The global keyboard outline extends four pixels beyond its element.
    // Paint containment must reserve that edge even when a link starts a block.
    assert('focus', blocks.every((block) => Number.parseFloat(block.clip) >= 4),
      `${width}px prose blocks clip the keyboard focus edge: ${JSON.stringify(blocks.filter((block) => Number.parseFloat(block.clip) < 4))}`);
    // Focus the column entrance, then let native Tab choose the tail scroller.
    await page.locator('.y-prose').evaluate((prose) => {
      prose.setAttribute('tabindex', '-1');
      prose.focus({ preventScroll: true });
    });
    for (let step = 0; step < 3; step += 1) {
      await page.keyboard.press('Tab');
      if (await page.evaluate(() => document.activeElement === document.querySelector('.y-prose > pre:last-child'))) break;
    }
    const keyboardTail = await page.evaluate(() => {
      const tail = document.querySelector('.y-prose > pre:last-child');
      return tail && { focused: document.activeElement === tail,
        overflowing: tail.scrollWidth > tail.clientWidth,
        inView: tail.getBoundingClientRect().top < innerHeight && tail.getBoundingClientRect().bottom > 0 };
    });
    assert('accessibility-tail', keyboardTail?.focused && keyboardTail.overflowing && keyboardTail.inView,
      `${width}px native Tab cannot reach the overflowing tail: ${JSON.stringify(keyboardTail)}`);
    await page.emulateMedia({ media: 'print' });
    const printed = await declarations(page);
    assert('print', printed.length === blocks.length && printed.every((block) => block.visibility === 'visible'),
      `${width}px paper blocks are still deferred: ${JSON.stringify(printed.filter((block) => block.visibility !== 'visible'))}`);
    await page.emulateMedia({ media: 'screen' });
    served = 0;
    matches = 0;
    await page.goto(BASE + OUTLINE_PAGE, { waitUntil: 'networkidle' });
    if (MUTATE && (served !== 1 || matches !== 1)) {
      throw new NotApplied(`NOT-APPLIED prose-visibility: ${MUTATE} on outline matched ${matches} sites over ${served} loads`);
    }
    const outlined = await declarations(page);
    if (await page.locator('.y-prose > [data-level]').count() < 2) throw new Error('BROKEN prose-visibility: outline fixture lacks multiple headings');
    assert('outline', outlined.every((block) => block.visibility === 'visible'), 'multiple-heading prose leaves placeholder geometry before an outline target');
    // A heading inside a block still needs every predecessor's authored
    // height. Use the actual emitted heading and prose, in a separate column,
    // so its direct-heading exemption cannot conceal a missing nested rule.
    const nested = await page.evaluate(() => {
      const source = document.querySelector('.y-prose');
      const heading = source.querySelector(':scope > [data-level]');
      const preceding = [...source.children].filter((block) => block.matches('p') && !block.querySelector('[id], a[href], input, button'));
      if (!heading || preceding.length === 0) return [];
      const isolated = document.createElement('div');
      isolated.className = 'y-prose';
      for (const block of preceding) isolated.append(block.cloneNode(true));
      const quote = document.createElement('blockquote');
      quote.append(heading.cloneNode(true));
      isolated.append(quote);
      source.after(isolated);
      return [...isolated.children].map((block) => getComputedStyle(block).contentVisibility);
    });
    if (nested.length < 2) throw new Error('BROKEN prose-visibility: nested heading has no real predecessors');
    assert('nested-outline', nested.every((visibility) => visibility === 'visible'), 'nested heading coordinates still depend on deferred predecessor heights');
    served = 0;
    matches = 0;
    await page.goto(BASE + '/notes/Notes/reading-fidelity.md', { waitUntil: 'networkidle' });
    if (MUTATE && (served !== 1 || matches !== 1)) {
      throw new NotApplied(`NOT-APPLIED prose-visibility: ${MUTATE} on authored controls matched ${matches} sites over ${served} loads`);
    }
    // Isolate authored descendants from unrelated address and runtime speech
    // wrappers. The native task/link content and its classes stay intact; the
    // separate speech case below continues to use its actual .y-reading owner.
    const controls = await page.evaluate(() => {
      const selector = 'input, select, textarea, button, summary, a[href], audio[controls], video[controls], [contenteditable="true"], [tabindex]';
      const source = document.querySelector('.y-prose');
      const isolated = document.createElement('div');
      isolated.className = 'y-prose';
      for (const block of source.children) {
        const authored = block.matches('.y-reading') ? block.querySelector(':scope > p') : block;
        if (!authored?.querySelector(selector)) continue;
        const copy = authored.cloneNode(true);
        copy.removeAttribute('id');
        for (const addressed of copy.querySelectorAll('[id]')) addressed.removeAttribute('id');
        // A nested heading exempts the whole column, so it belongs to the
        // independent nested-outline case rather than this control stimulus.
        if (copy.matches('[data-level]') || copy.querySelector('[data-level]')) continue;
        isolated.append(copy);
      }
      source.after(isolated);
      return [...isolated.children].map((block) => ({
        tag: block.tagName,
        controls: block.querySelectorAll(selector).length,
        unrelatedExemption: block.matches('.y-reading, [id], [contenteditable="true"], [tabindex]') || Boolean(block.querySelector('[id]')),
        visibility: getComputedStyle(block).contentVisibility,
      }));
    });
    if (controls.length === 0 || controls.some((block) => block.controls === 0 || block.unrelatedExemption)) {
      throw new Error(`BROKEN prose-visibility: authored descendants have no independently eligible ancestor: ${JSON.stringify(controls)}`);
    }
    assert('controls', controls.every((block) => block.visibility === 'visible'), `authored control ancestors defer their initial layout: ${JSON.stringify(controls)}`);
    // The voice control stands in the paragraph's gutter on a wide screen.
    // A clipped button still reports its rectangle, so test the pointer's
    // actual destination as well as its size, without starting any speech.
    await page.emulateMedia({ media: 'screen' });
    served = 0;
    matches = 0;
    await page.goto(BASE + '/notes/Writing/lessons/japanese/L02.md', { waitUntil: 'networkidle' });
    if (MUTATE && (served !== 1 || matches !== 1)) {
      throw new NotApplied(`NOT-APPLIED prose-visibility: ${MUTATE} on the lesson served ${served} stylesheets, matched ${matches} sites; want one each`);
    }
    if ((await page.locator('.y-reading > .y-tts').count()) === 0) {
      throw new Error('BROKEN prose-visibility: the lesson has no paragraph speech controls');
    }
    for (const button of await page.locator('.y-reading > .y-tts').all()) {
      await button.locator('..').scrollIntoViewIfNeeded();
      const pointer = await button.evaluate((element) => {
        const rect = element.getBoundingClientRect();
        const hit = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
        return {
          speech: document.documentElement.hasAttribute('data-speech'),
          width: rect.width, height: rect.height,
          reachable: element === hit || element.contains(hit),
        };
      });
      if (!pointer.speech) throw new Error('BROKEN prose-visibility: browser speech capability is unavailable');
      assert('speech', pointer.width > 0 && pointer.height > 0 && pointer.reachable,
        `${width}px paragraph speech control is clipped or unreachable: ${JSON.stringify(pointer)}`);
    }
    // The concept drawer computes a named section's position once when it
    // opens. Its preceding blocks must already have their real heights.
    const sheetWidth = width === 1280 ? 1600 : width;
    await page.setViewportSize({ width: sheetWidth, height: 800 });
    served = 0;
    matches = 0;
    await page.goto(BASE + '/notes/Writing/lessons/japanese/L01.md', { waitUntil: 'networkidle' });
    if (MUTATE && (served !== 1 || matches !== 1)) {
      throw new NotApplied(`NOT-APPLIED prose-visibility: ${MUTATE} on the concept lesson served ${served} stylesheets, matched ${matches} sites; want one each`);
    }
    const link = page.locator('[data-concept][href*="#"]').first();
    if ((await link.count()) === 0) throw new Error('BROKEN prose-visibility: lesson has no named concept section');
    const fragment = decodeURIComponent((await link.getAttribute('href')).split('#')[1]);
    await link.click();
    await page.locator('[data-concept-sheet][open]').waitFor();
    const concept = await page.locator('[data-concept-body]').evaluate((body, id) => {
      const target = body.querySelector(`#${CSS.escape(id)}`);
      const blocks = [...body.children].map((block) => getComputedStyle(block).contentVisibility);
      if (!target || blocks.length === 0) return null;
      return { blocks, fromTop: target.getBoundingClientRect().top - body.getBoundingClientRect().top, height: body.clientHeight };
    }, fragment);
    if (!concept) throw new Error('BROKEN prose-visibility: concept body has no blocks or named section');
    assert('sheet', concept.blocks.every((visibility) => visibility === 'visible')
      && concept.fromTop >= -2 && concept.fromTop <= concept.height / 2,
    `${sheetWidth}px concept section lacks full layout at its named destination: ${JSON.stringify(concept)}`);
    await page.locator('[data-concept-sheet]').evaluate(async (sheet) => {
      sheet.close();
      await Promise.all(sheet.getAnimations().map((animation) => animation.finished.catch(() => {})));
    });
    const previewLink = page.locator('.y-prose a.wikilink[href="/notes/Notes/Glass%20Tide.md"]:not(.concept-link)');
    if ((await previewLink.count()) !== 1) throw new Error('BROKEN prose-visibility: lesson lacks a unique preview link');
    // Native scrolling queues its event for a later frame. An event created
    // before a new hover must not cancel the reader's newer question.
    await previewLink.evaluate((link) => {
      for (const kind of ['pointerenter', 'focus']) link.addEventListener(kind, (event) => {
        window.__prosePreviewQuestion = { kind, created: event.timeStamp };
      });
    });
    const earlierScroll = await page.evaluateHandle(() => new Event('scroll'));
    const earlierAt = await earlierScroll.evaluate((event) => event.timeStamp);
    await previewLink.hover();
    const question = await page.evaluate(() => window.__prosePreviewQuestion);
    if (question?.kind !== 'pointerenter' || earlierAt >= question.created) {
      throw new Error('BROKEN prose-visibility: the synthetic scroll does not precede the actual pointer question');
    }
    await page.evaluate((event) => document.dispatchEvent(event), earlierScroll);
    await earlierScroll.dispose();
    let opened = true;
    try {
      await page.waitForFunction(() => document.querySelector('[data-preview-card]')?.matches(':popover-open'), null, { timeout: 4000 });
    } catch (error) {
      if (error.name !== 'TimeoutError') throw error;
      opened = false;
    }
    assert('preview', opened, `${sheetWidth}px an earlier queued scroll canceled the newer preview hover`);
    // A scroll created after that question still dismisses its card.
    const dismissed = await page.evaluate(() => {
      document.dispatchEvent(new Event('scroll'));
      return !document.querySelector('[data-preview-card]').matches(':popover-open');
    });
    assert('preview-scroll', dismissed, `${sheetWidth}px a newer page scroll did not dismiss the preview`);
    await page.waitForFunction(() => !document.querySelector('[data-preview-open]'));
    const earlierFocusScroll = await page.evaluateHandle(() => new Event('scroll'));
    const earlierFocusAt = await earlierFocusScroll.evaluate((event) => event.timeStamp);
    await previewLink.focus();
    const focusQuestion = await page.evaluate(() => window.__prosePreviewQuestion);
    if (focusQuestion?.kind !== 'focus' || earlierFocusAt >= focusQuestion.created) {
      throw new Error('BROKEN prose-visibility: the synthetic scroll does not precede the actual focus question');
    }
    await page.evaluate((event) => document.dispatchEvent(event), earlierFocusScroll);
    await earlierFocusScroll.dispose();
    let focusedOpened = true;
    try {
      await page.waitForFunction(() => document.querySelector('[data-preview-card]')?.matches(':popover-open'), null, { timeout: 4000 });
    } catch (error) {
      if (error.name !== 'TimeoutError') throw error;
      focusedOpened = false;
    }
    assert('preview-focus', focusedOpened, `${sheetWidth}px an earlier queued scroll canceled the newer preview focus`);
    const previewLayout = await page.locator('.y-preview__body').evaluate((element) => getComputedStyle(element).contentVisibility);
    assert('preview-layout', previewLayout === 'visible', `${sheetWidth}px the imported preview passage is deferred`);
    const focusDismissed = await page.evaluate(() => {
      document.dispatchEvent(new Event('scroll'));
      return !document.querySelector('[data-preview-card]').matches(':popover-open');
    });
    assert('preview-scroll', focusDismissed, `${sheetWidth}px a newer scroll did not dismiss the focused preview`);
    await context.close();
  }
  console.log('PASS prose-visibility: eligible blocks defer with remembered sizes and focus edges; initial tail AX text and native keyboard scrolling remain available; paper, speech controls, named concept destinations and newer preview questions remain available');
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.site === MUTATIONS[MUTATE].site) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
