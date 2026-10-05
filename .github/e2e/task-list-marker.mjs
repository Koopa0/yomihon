// Browser lock for GFM task-list markers. The renderer emits a named checkbox
// in two ul shapes: the native label on the li (tight), or wrapped
// in a p (loose / multi-paragraph). The disc inherited from `.y-prose ul`
// sits beside either unless a CSS exception hides it. An ordered task is
// different: the number is what the author wrote, so it stays. Go tests
// cannot see a list marker; this reads the computed style on a live page.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note that carries a tight task list, a
// loose task list, an ordered task item, and an ordinary ul), and MUTATE.
import { readFileSync } from 'node:fs';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['task-item-has-no-disc', 'ordinary-item-keeps-disc', 'ordered-task-keeps-decimal', 'task-checkbox-has-name', 'neutral-markers-render-as-tasks', 'task-block-name-keeps-own-words', 'task-link-stays-independent'];

// The production exception, both goldmark shapes. Mutations that restore the
// disc have to name this, or a rewrite that only hits the tight item would
// leave the loose case unproved.
const UL_TASK_TIGHT = '.y-prose ul > li:has(> .y-task:first-child > input[type="checkbox"]:first-child)';
const UL_TASK_LOOSE = '.y-prose ul > li:has(> p:first-child > .y-task:first-child > input[type="checkbox"]:first-child)';
const UL_TASK_ITEM = `${UL_TASK_TIGHT}, ${UL_TASK_LOOSE}`;
const markerDeclarations = [...readFileSync(new URL('../../internal/render/tasklist.go', import.meta.url), 'utf8').matchAll(/const neutralTaskMarkers = "([^"]+)"/g)];
if (markerDeclarations.length !== 1) throw new Error('task-list-marker: expected exactly one neutral marker declaration');
const NEUTRAL_MARKERS = [...markerDeclarations[0][1]];
if (NEUTRAL_MARKERS.length === 0 || new Set(NEUTRAL_MARKERS).size !== NEUTRAL_MARKERS.length) throw new Error('task-list-marker: empty or repeated neutral marker set');

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN task-list-marker: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL task-list-marker: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN task-list-marker: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED task-list-marker: ${message}`); };

// Injects one rule, first proving its selector matches something. A rule that
// styles no element leaves the page exactly as it was, and the probe would
// then pass the self-test while showing nothing at all about its ability to fail.
const injectRule = async (page, selector, declarations) => {
  const matched = await page.evaluate((s) => document.querySelectorAll(s).length, selector);
  if (matched === 0) notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  await page.addStyleTag({ content: `${selector} { ${declarations} }` });
};

const restoreTaskDisc = async (page) => {
  for (const selector of [UL_TASK_TIGHT, UL_TASK_LOOSE]) {
    const matched = await page.evaluate((s) => document.querySelectorAll(s).length, selector);
    if (matched === 0) notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  }
  await page.addStyleTag({ content: `${UL_TASK_ITEM} { list-style: disc }` });
};

const mutateUnique = async (page, selector, edit) => {
  const count = await page.locator(selector).count();
  if (count !== 1) notApplied(`${selector} matched ${count} elements, want exactly one`);
  await page.locator(selector).evaluate(edit);
};

const MUTATIONS = {
  'rename-block-task': {
    target: 'task-block-name-keeps-own-words',
    apply: (page) => mutateUnique(page, 'label.y-task > span.y-offscreen:text-is("Before own after independent link")', (span) => { span.textContent = 'Borrowed inner words'; }),
  },
  'disable-task-link': {
    target: 'task-link-stays-independent',
    apply: (page) => mutateUnique(page, 'a.wikilink:text-is("independent link")', (link) => { link.href = '#'; }),
  },
  'drop-task-label': {
    target: 'task-checkbox-has-name',
    apply: (page) => mutateUnique(page, 'label.y-task:text-is("Unfinished task")', (label) => label.replaceWith(...label.childNodes)),
  },
  'drop-neutral-checkbox': {
    target: 'neutral-markers-render-as-tasks',
    apply: (page) => mutateUnique(page, '.y-prose input[data-task="/"]', (input) => input.remove()),
  },
  'restore-task-disc': {
    target: 'task-item-has-no-disc',
    apply: restoreTaskDisc,
  },
  'hide-ordinary-disc': {
    target: 'ordinary-item-keeps-disc',
    apply: (page) => injectRule(page, '.y-prose ul', 'list-style: none'),
  },
  'hide-ordered-task-number': {
    target: 'ordered-task-keeps-decimal',
    apply: (page) => injectRule(page, '.y-prose ol > li:has(> .y-task:first-child > input[type="checkbox"]:first-child)', 'list-style: none'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`task-list-marker: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`task-list-marker: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`task-list-marker: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const readMarkers = (page) =>
  page.evaluate(() => {
    const classify = (li) => {
      const direct = li.querySelector(':scope > .y-task:first-child > input[type="checkbox"]:first-child');
      const wrapped = li.querySelector(':scope > p:first-child > .y-task:first-child > input[type="checkbox"]:first-child');
      const box = direct || wrapped;
      return {
        listStyleType: getComputedStyle(li).listStyleType,
        checked: box ? box.checked : null,
        shape: direct ? 'tight' : wrapped ? 'loose' : 'plain',
      };
    };
    const ulItems = [...document.querySelectorAll('.y-prose ul > li')].map(classify);
    const olItems = [...document.querySelectorAll('.y-prose ol > li')].map(classify);
    return {
      tight: ulItems.filter((item) => item.shape === 'tight'),
      loose: ulItems.filter((item) => item.shape === 'loose'),
      ordinary: ulItems.filter((item) => item.shape === 'plain'),
      ordered: olItems.filter((item) => item.shape !== 'plain'),
    };
  });

const requirePolarity = (items, shape) => {
  if (items.length === 0) broken(`the page carries no ${shape} task item, so nothing was measured`);
  if (!items.some((item) => item.checked === false)) {
    broken(`the page carries no unchecked ${shape} task item, so one polarity was never measured`);
  }
  if (!items.some((item) => item.checked === true)) {
    broken(`the page carries no checked ${shape} task item, so one polarity was never measured`);
  }
};

const arrived = (page) => page.waitForFunction(
  async () => {
    if (![...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css'))) return false;
    await Promise.all(document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => animation.finished.catch(() => {})));
    return true;
  },
  null,
  { timeout: 3000 },
);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);

  await arrived(page);
  if (MUTATE && MUTATE !== 'disable-task-link') await MUTATIONS[MUTATE].apply(page);
  const taskNames = await page.locator('.y-prose input[type="checkbox"]').evaluateAll((inputs) => inputs.map((input) => ({ disabled: input.disabled, labels: input.labels.length, text: [...input.labels].map((label) => label.textContent.trim()).join(' '), marker: input.dataset.task ?? null, checked: input.checked })));
  const cdp = await page.context().newCDPSession(page);
  const ax = await cdp.send('Accessibility.getFullAXTree');
  const checkboxes = ax.nodes.filter((node) => !node.ignored && node.role?.value === 'checkbox');
  if (taskNames.length === 0 || checkboxes.length !== taskNames.length) broken('the DOM/AX checkbox set is empty or differs');
  if (taskNames.some((task) => !task.disabled || task.labels !== 1 || task.text === '') || checkboxes.some((node) => !(node.name?.value ?? '').trim())) {
    fail('task-checkbox-has-name', 'every disabled task checkbox must have one native text label and a nonempty accessibility-tree name');
  }
  const actualMarkers = taskNames.filter((task) => task.marker !== null).map((task) => task.marker).sort();
  if (JSON.stringify(actualMarkers) !== JSON.stringify([...NEUTRAL_MARKERS].sort()) || taskNames.some((task) => task.marker !== null && task.checked)) {
    fail('neutral-markers-render-as-tasks', `neutral marker inputs ${JSON.stringify(actualMarkers)} differ from the whole declared set ${JSON.stringify(NEUTRAL_MARKERS)}, or claim completion`);
  }

  const names = checkboxes.map((node) => node.name.value.trim());
  const ownNames = ['Before own after independent link', 'Loose own after', 'Repeated own after'];
  const labelsArePhrasing = await page.locator('label.y-task').evaluateAll((labels) => labels.every((label) => label.querySelectorAll('input').length === 1 && !label.querySelector('div, p, ul, ol, label')));
  if (!labelsArePhrasing || ownNames.some((name) => names.filter((value) => value === name).length !== 1) || names.filter((name) => name === 'Inner task').length !== 5) {
    fail('task-block-name-keeps-own-words', 'a block-containing task must name only its own inline words, with every embedded task independently named and outside its label');
  }

  const reading = await readMarkers(page);
  requirePolarity(reading.tight, 'tight');
  requirePolarity(reading.loose, 'loose');
  if (reading.ordered.length === 0) {
    broken('the page carries no ordered task item, so the number that must stay was never measured');
  }
  if (reading.ordinary.length === 0) {
    broken('the page carries no ordinary prose list item, so the disc that must stay was never measured');
  }

  // Loose first: that is the shape the earlier selector missed, and the
  // failure this lock exists to name.
  const loose = reading.loose.find((item) => item.listStyleType !== 'none');
  if (loose) {
    fail('task-item-has-no-disc', `a loose task item computed list-style-type=${JSON.stringify(loose.listStyleType)}, want "none"`);
  }
  const tight = reading.tight.find((item) => item.listStyleType !== 'none');
  if (tight) {
    fail('task-item-has-no-disc', `a tight task item computed list-style-type=${JSON.stringify(tight.listStyleType)}, want "none"`);
  }

  const plain = reading.ordinary.find((item) => item.listStyleType !== 'disc');
  if (plain) {
    fail('ordinary-item-keeps-disc', `an ordinary list item computed list-style-type=${JSON.stringify(plain.listStyleType)}, want "disc"`);
  }

  const numbered = reading.ordered.find((item) => item.listStyleType !== 'decimal');
  if (numbered) {
    fail('ordered-task-keeps-decimal', `an ordered task item computed list-style-type=${JSON.stringify(numbered.listStyleType)}, want "decimal"`);
  }

  const ordinaryLink = page.locator('a.wikilink:text-is("ordinary task link")');
  if (await ordinaryLink.count() !== 1) broken('the ordinary inline task link is absent or duplicated');
  await ordinaryLink.click();
  await arrived(page);
  if (!page.url().endsWith('/notes/Notes/task-child.md')) fail('task-link-stays-independent', 'the ordinary task label swallowed its authored link navigation');
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await arrived(page);
  if (MUTATE === 'disable-task-link') await MUTATIONS[MUTATE].apply(page);
  const authoredLink = page.locator('a.wikilink:text-is("independent link")');
  if (await authoredLink.count() !== 1) broken('the independent authored task link is absent or duplicated');
  if (await authoredLink.evaluate((link) => !!link.closest('label'))) fail('task-link-stays-independent', 'the block task link must remain outside its detached label');
  await authoredLink.click();
  await arrived(page);
  if (!page.url().endsWith('/notes/Notes/task-child.md')) fail('task-link-stays-independent', 'the authored task link did not navigate to its own note');

  console.log('PASS task-list-marker: all native checkboxes have AX names; every neutral marker renders unchecked; tight/loose/ordinary/ordered list styles are retained');
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
