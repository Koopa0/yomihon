// A long reading keeps every block in the document while the browser defers
// work outside the viewport. Paper needs every block at once. Read the stable
// declarations, not a transient skipped state or a rendering-time threshold.
// Env: YOMIHON_BASE, PAGE_PATH, MUTATE (list prints all watched regressions).
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Glass%20Tide.md';
const MUTATE = process.env.MUTATE || '';
const CHILDREN = '.y-prose > *';

class LockFired extends Error {
  constructor(site, message) {
    super(`FAIL prose-visibility: ${message}`);
    this.site = site;
  }
}
class NotApplied extends Error {}

const screen = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*auto\s*;/g;
const intrinsic = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)contain-intrinsic-block-size:\s*auto\s+[\d.]+em\s*;/g;
// Indented: the print rule sits inside its media block, unlike the screen
// rules that lay every block out while the column folds or a mark lands.
const paper = /(^[ \t]+\.y-prose\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*visible\s*;/gm;
const gutter = /(\.y-prose\s+\.y-reading\s*\{[^}]*?)overflow-clip-margin:\s*[\d.]+px\s*;/g;
const focus = /(\.y-prose\s*>\s*\*\s*\{[^}]*?)overflow-clip-margin:\s*[\d.]+px\s*;/g;
const sheet = /(\.y-conceptsheet__body\s*>\s*\*\s*\{[^}]*?)content-visibility:\s*visible\s*;/g;
const preview = /if \(!card.contains\(event.target\) && event.timeStamp >= askedAt\) close\(\);/g;
const focusQuestion = /link.addEventListener\('focus', \(event\) => schedule\(link, openDelay, event.timeStamp\)\);/g;
const MUTATIONS = {
  'render-offscreen-blocks': { site: 'screen', needle: screen, replace: '$1' },
  'omit-list-blocks': {
    site: 'screen', needle: screen,
    replace: (match) => match.replace(/\*\s*\{/, ':not(ul) {'),
  },
  'forget-block-size': { site: 'intrinsic', needle: intrinsic, replace: '$1' },
  'reserve-inline-size': {
    site: 'intrinsic', needle: intrinsic,
    replace: (match) => match.replace('contain-intrinsic-block-size', 'contain-intrinsic-inline-size'),
  },
  'defer-print-blocks': { site: 'print', needle: paper, replace: '$1' },
  'clip-speaking-control': { site: 'speech', needle: gutter, replace: '$1' },
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
  'defer-concept-sections': { site: 'sheet', needle: sheet, replace: '$1' },
};
const SITES = ['screen', 'intrinsic', 'print', 'speech', 'sheet', 'focus', 'preview', 'preview-scroll', 'preview-focus'];
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
    index, tag: element.tagName, visibility: style.contentVisibility,
    block: style.containIntrinsicBlockSize, inline: style.containIntrinsicInlineSize, clip: style.overflowClipMargin,
  };
}));
const assert = (site, condition, message) => {
  if (!condition) throw new LockFired(site, message);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const width of [1280, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage();
    let served = 0;
    let matches = 0;
    if (MUTATE) {
      const mode = MUTATIONS[MUTATE];
      await page.route(`**/static/${mode.asset || 'app.css'}`, async (route) => {
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
    const workload = await page.evaluate(() => ({ height: innerHeight, document: document.documentElement.scrollHeight }));
    const blocks = await declarations(page);
    if (MUTATE === 'omit-list-blocks' && !blocks.some((block) => block.tag === 'UL')) {
      throw new NotApplied('NOT-APPLIED prose-visibility: omit-list-blocks has no list to omit');
    }
    if (blocks.length === 0 || workload.document <= workload.height * 2) {
      throw new Error(`BROKEN prose-visibility: ${PAGE} is not a nonempty multi-viewport reading`);
    }
    assert('screen', blocks.every((block) => block.visibility === 'auto'),
      `${width}px screen blocks do not all defer offscreen work: ${JSON.stringify(blocks.filter((block) => block.visibility !== 'auto'))}`);
    // Engines can serialize the unused axis as "auto none" when the other
    // axis remembers its size. Neither spelling reserves an inline fallback.
    assert('intrinsic', blocks.every((block) => /^auto [\d.]+px$/.test(block.block)
      && Number.parseFloat(block.block.slice(5)) > 0 && /^(auto )?none$/.test(block.inline)),
    `${width}px blocks do not remember a positive block-size fallback without reserving inline space: ${JSON.stringify(blocks)}`);
    // The global keyboard outline extends four pixels beyond its element.
    // Paint containment must reserve that edge even when a link starts a block.
    assert('focus', blocks.every((block) => Number.parseFloat(block.clip) >= 4),
      `${width}px prose blocks clip the keyboard focus edge: ${JSON.stringify(blocks.filter((block) => Number.parseFloat(block.clip) < 4))}`);
    await page.emulateMedia({ media: 'print' });
    const printed = await declarations(page);
    assert('print', printed.length === blocks.length && printed.every((block) => block.visibility === 'visible'),
      `${width}px paper blocks are still deferred: ${JSON.stringify(printed.filter((block) => block.visibility !== 'visible'))}`);
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
    const focusDismissed = await page.evaluate(() => {
      document.dispatchEvent(new Event('scroll'));
      return !document.querySelector('[data-preview-card]').matches(':popover-open');
    });
    assert('preview-scroll', focusDismissed, `${sheetWidth}px a newer scroll did not dismiss the focused preview`);
    await context.close();
  }
  console.log('PASS prose-visibility: chapter blocks defer with remembered sizes and focus edges; paper, speech controls, named concept destinations and newer preview questions remain available');
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
