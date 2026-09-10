// Browser lock for GFM task-list markers. Goldmark emits a checkbox and no
// task-list class, in two ul shapes: the input on the li (tight), or wrapped
// in a p (loose / multi-paragraph). The disc inherited from `.y-prose ul`
// sits beside either unless a CSS exception hides it. An ordered task is
// different: the number is what the author wrote, so it stays. Go tests
// cannot see a list marker; this reads the computed style on a live page.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note that carries a tight task list, a
// loose task list, an ordered task item, and an ordinary ul), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['task-item-has-no-disc', 'ordinary-item-keeps-disc', 'ordered-task-keeps-decimal'];

// The production exception, both goldmark shapes. Mutations that restore the
// disc have to name this, or a rewrite that only hits the tight item would
// leave the loose case unproved.
const UL_TASK_TIGHT = '.y-prose ul > li:has(> input[type="checkbox"]:first-child)';
const UL_TASK_LOOSE = '.y-prose ul > li:has(> p:first-child > input[type="checkbox"]:first-child)';
const UL_TASK_ITEM = `${UL_TASK_TIGHT}, ${UL_TASK_LOOSE}`;

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

const MUTATIONS = {
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
    apply: (page) => injectRule(page, '.y-prose ol > li:has(> input[type="checkbox"]:first-child)', 'list-style: none'),
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
      const direct = li.querySelector(':scope > input[type="checkbox"]:first-child');
      const wrapped = li.querySelector(':scope > p:first-child > input[type="checkbox"]:first-child');
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

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);

  if (MUTATE) await MUTATIONS[MUTATE].apply(page);

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

  console.log('PASS task-list-marker: tight and loose task items compute no disc; ordinary items keep theirs; ordered tasks keep their number');
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
