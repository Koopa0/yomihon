// Browser lock for GFM task-list markers. Goldmark emits a checkbox inside an
// ordinary list item and no task-list class, so the disc inherited from
// `.y-prose ul` sits beside the checkbox unless a CSS exception hides it.
// Go tests cannot see a list marker; this reads the computed style on a live
// page.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note that carries both a task list and an
// ordinary list), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['task-item-has-no-disc', 'ordinary-item-keeps-disc'];

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

const MUTATIONS = {
  'restore-task-disc': {
    target: 'task-item-has-no-disc',
    apply: (page) => injectRule(page, '.y-prose li:has(> input[type="checkbox"])', 'list-style: disc'),
  },
  'hide-ordinary-disc': {
    target: 'ordinary-item-keeps-disc',
    apply: (page) => injectRule(page, '.y-prose ul', 'list-style: none'),
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
    const tasks = [...document.querySelectorAll('.y-prose li:has(> input[type="checkbox"])')].map((li) => {
      const box = li.querySelector(':scope > input[type="checkbox"]');
      return {
        listStyleType: getComputedStyle(li).listStyleType,
        checked: box ? box.checked : null,
      };
    });
    const ordinary = [...document.querySelectorAll('.y-prose ul > li')].filter(
      (li) => !li.querySelector(':scope > input[type="checkbox"]'),
    ).map((li) => ({
      listStyleType: getComputedStyle(li).listStyleType,
    }));
    return { tasks, ordinary };
  });

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);

  if (MUTATE) await MUTATIONS[MUTATE].apply(page);

  const reading = await readMarkers(page);
  if (reading.tasks.length === 0) broken('the page carries no prose task item, so nothing was measured');
  if (!reading.tasks.some((item) => item.checked === false)) {
    broken('the page carries no unchecked task item, so one polarity was never measured');
  }
  if (!reading.tasks.some((item) => item.checked === true)) {
    broken('the page carries no checked task item, so one polarity was never measured');
  }
  if (reading.ordinary.length === 0) {
    broken('the page carries no ordinary prose list item, so the disc that must stay was never measured');
  }

  const tasked = reading.tasks.find((item) => item.listStyleType !== 'none');
  if (tasked) {
    fail('task-item-has-no-disc', `a task item computed list-style-type=${JSON.stringify(tasked.listStyleType)}, want "none"`);
  }

  const plain = reading.ordinary.find((item) => item.listStyleType !== 'disc');
  if (plain) {
    fail('ordinary-item-keeps-disc', `an ordinary list item computed list-style-type=${JSON.stringify(plain.listStyleType)}, want "disc"`);
  }

  console.log('PASS task-list-marker: task items compute no disc; ordinary items keep theirs');
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
