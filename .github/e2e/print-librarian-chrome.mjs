// Behavior lock for librarian chrome that must not reach paper. The ruling on
// #148 asks for rendered print output, not each element's own computed display:
// a surface hidden only by an ancestor can still report display:block on itself.
//
// On screen the opening column carries path crumbs, the type and Obsidian
// metarow, and folder previous/next. In print media those three stay off the
// sheet; course lesson-order prev/next may remain.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with folder siblings), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const FOLDER_PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const COURSE_PAGE = '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'crumbs-off-paper',
  'metarow-off-paper',
  'folder-steps-off-paper',
  'course-steps-on-paper',
];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN print-librarian-chrome: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL print-librarian-chrome: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN print-librarian-chrome: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED print-librarian-chrome: ${message}`); };

const weakenStylesheet = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen > 0 ? '' : 'app.css was never requested, so the override reached no page');
};

const MUTATIONS = {
  'drop-hide-crumbs': {
    target: 'crumbs-off-paper',
    apply: weakenStylesheet('@media print{.y-crumbs{display:block !important}}'),
  },
  'drop-hide-metarow': {
    target: 'metarow-off-paper',
    apply: weakenStylesheet('@media print{.y-metarow{display:block !important}}'),
  },
  'drop-hide-folder-steps': {
    target: 'folder-steps-off-paper',
    apply: weakenStylesheet('@media print{.y-steps:not(.y-steps--course){display:block !important}}'),
  },
  'hide-course-steps': {
    target: 'course-steps-on-paper',
    apply: weakenStylesheet('@media print{nav.y-steps.y-steps--course{display:none !important}}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`print-librarian-chrome: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`print-librarian-chrome: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`print-librarian-chrome: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// laidOut answers whether the element's own box carries ink on the current
// medium. It does not read display alone: checkVisibility walks ancestors for
// opacity and content-visibility, and a zero box is not on paper regardless
// of what the element's own declaration says.
const laidOut = (locator) => locator.first().evaluate((element) => {
  element.scrollIntoView({ block: 'nearest' });
  const rect = element.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return false;
  const style = getComputedStyle(element);
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = style.color;
  ctx.fillRect(0, 0, 1, 1);
  const inkAlpha = ctx.getImageData(0, 0, 1, 1).data[3];
  if (inkAlpha === 0) return false;
  return element.checkVisibility({
    opacityProperty: true,
    visibilityProperty: true,
    contentVisibilityAuto: true,
  });
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 900, height: 900 } });
  const page = await context.newPage();
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  await page.goto(BASE + FOLDER_PAGE, { waitUntil: 'domcontentloaded' });
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  // Parse sanity on screen: the folder note must actually paint the three
  // surfaces this lock is about. A probe that never saw them on screen would
  // pass print hide for the wrong reason — the page was empty of chrome to
  // begin with.
  await page.emulateMedia({ media: 'screen' });
  if (!(await laidOut(page.locator('.y-crumbs')))) {
    broken('the folder note paints no path crumbs on screen, so print hide cannot be judged');
  }
  if (!(await laidOut(page.locator('.y-metarow')))) {
    broken('the folder note paints no metarow on screen, so print hide cannot be judged');
  }
  const folderSteps = page.locator('nav.y-steps:not(.y-steps--course)');
  if (!(await laidOut(folderSteps))) {
    broken('the folder note paints no folder previous/next on screen, so print hide cannot be judged');
  }

  await page.emulateMedia({ media: 'print' });
  if (await laidOut(page.locator('.y-crumbs'))) {
    fail('crumbs-off-paper', 'path crumbs still lay out on paper');
  }
  if (await laidOut(page.locator('.y-metarow'))) {
    fail('metarow-off-paper', 'the type and Obsidian metarow still lay out on paper');
  }
  if (await laidOut(folderSteps)) {
    fail('folder-steps-off-paper', 'folder previous/next still lay out on paper');
  }

  await page.goto(BASE + COURSE_PAGE, { waitUntil: 'domcontentloaded' });
  await page.emulateMedia({ media: 'print' });
  if (!(await laidOut(page.locator('nav.y-steps.y-steps--course')))) {
    fail('course-steps-on-paper', 'course lesson-order previous/next vanished from paper');
  }

  await context.close();
  console.log('PASS print-librarian-chrome: librarian chrome stays off paper; course steps may remain');
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
