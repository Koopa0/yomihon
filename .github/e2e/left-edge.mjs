// Behavior lock for the shared left edge. The text of every page that has a
// sidebar starts at one place, the reading column's, and the text of every page
// that has none starts at one place, the desk's. It is measured where the
// heading's first glyph is, at the widths a window is opened at and with the
// sidebar open and folded, so a page whose box was centred on its own measure,
// as every page used to be, starts somewhere else and the lock reads it.
//
// Env: YOMIHON_BASE and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';

const WITH_SIDEBAR = [
  ['a lesson', '/notes/Course/C02.md', '.y-article h1.y-title'],
  ['a course', '/courses/Maps/study.md', '.y-syl h1.y-title'],
  ['the findings', '/health', '.y-healthpage h1.y-title'],
  ['a search', '/search?q=alpha', '.y-searchpage h1.y-title'],
  ['a file', '/notes/Notes/plain.txt', '.y-article h1.y-title'],
];
const WITHOUT_SIDEBAR = [
  ['the desk', '/', '.y-home h1.y-title'],
  ['a shelf', '/courses', '.y-home h1.y-title'],
  ['the reading choices', '/preferences', '.y-prefs h1.y-title'],
  ['a course read aloud', '/listen/Maps/listen.md', '.y-listen h1.y-title'],
];

const MODES = {
  'centre-the-findings-page': { css: '.y-main > .y-healthpage{margin-left:auto !important;margin-right:auto !important}', group: 'sidebar' },
  'centre-the-search-page': { css: '.y-main > .y-searchpage{margin-left:auto !important;margin-right:auto !important}', group: 'sidebar' },
  'pin-the-course-to-the-sidebar': { css: '.y-main > .y-syl{margin-left:0 !important}', group: 'sidebar' },
  'centre-the-reading-choices': { css: '.y-prefs{margin-left:auto !important;margin-right:auto !important}', group: 'desk' },
  'centre-the-listening-page': { css: '.y-main > .y-listen{margin-left:auto !important;margin-right:auto !important}', group: 'desk' },
};

if (MUTATE === 'list') {
  console.log(Object.keys(MODES).join('\n'));
  process.exit(0);
}
if (MUTATE && !MODES[MUTATE]) {
  console.error(`left-edge: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

class LockFired extends Error {
  constructor(group, message) {
    super(`FAIL left-edge: ${message}`);
    this.group = group;
  }
}
class NotApplied extends Error {}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  console.log(`left-edge: Chrome ${browser.version()}`);
  const mode = MUTATE ? MODES[MUTATE] : null;
  let planted = 0;
  let requests = 0;
  const widths = [1280, 1440, 1700];
  for (const [rail, width] of [['open', 1280], ['open', 1440], ['open', 1700], ['collapsed', 1440]]) {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
    await context.addCookies([{ name: 'yomihon_rail', value: rail, url: BASE }]);
    const page = await context.newPage();
    if (mode) {
      await page.route('**/static/app.css{,?*}', async (route) => {
        const response = await route.fetch();
        const original = await response.text();
        requests += 1;
        planted += 1;
        await route.fulfill({ response, body: `${original}\n${mode.css}\n` });
      });
    }
    const measure = async (rows) => {
      const found = [];
      for (const [name, path, selector] of rows) {
        const response = await page.goto(BASE + path, { waitUntil: 'load' });
        if (response?.status() !== 200) throw new Error(`BROKEN left-edge: ${path} answered ${response?.status()}, want 200`);
        const left = await page.locator(selector).first().evaluate((heading) => heading.getBoundingClientRect().left);
        found.push({ name, left: Math.round(left * 10) / 10 });
      }
      return found;
    };
    const check = (group, found) => {
      const spread = Math.max(...found.map((f) => f.left)) - Math.min(...found.map((f) => f.left));
      console.log(`left-edge: ${rail} sidebar at ${width}px, pages ${group === 'sidebar' ? 'with' : 'without'} a sidebar start at ${JSON.stringify(found.map((f) => `${f.name} ${f.left}`))}`);
      if (spread > 1) {
        throw new LockFired(group, `${rail === 'open' ? 'open' : 'folded'} sidebar at ${width}px: the text of the pages ${group === 'sidebar' ? 'with' : 'without'} a sidebar starts ${spread}px apart: ${found.map((f) => `${f.name} at ${f.left}`).join(', ')}`);
      }
    };
    check('sidebar', await measure(WITH_SIDEBAR));
    check('desk', await measure(WITHOUT_SIDEBAR));
    await context.close();
  }
  if (mode && (requests < 1 || planted !== requests)) throw new NotApplied(`NOT-APPLIED left-edge: the planted rule reached ${planted} of ${requests} stylesheet requests`);
  console.log(`PASS left-edge: at ${widths.join(', ')}px and with the sidebar folded, pages with a sidebar start their text at one edge and pages without one at another`);
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else {
    if (MUTATE && error instanceof LockFired && error.group === MODES[MUTATE].group) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    }
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
