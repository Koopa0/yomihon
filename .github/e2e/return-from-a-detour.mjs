// Behavior lock for the second situation the reading page is built for: a reader
// goes from a section of one note into a note it points at, on into a third, and
// then comes back, and finds the section they left as it was, at the place they
// left it, not merely the note.
//
// The product adds nothing of its own to carry the reader out and back: it keeps
// every step an ordinary page of the browser's history and leaves the browser's
// own return to put the page and its position back. What a real browser shows is
// whether it still can: two steps back land on the first note's address, at the
// scroll position they left, with the section's own controls in place. And the
// place the reader chooses to leave off at, which is another thing than this way
// back (neither needs the other), is drawn in the margin of the section that holds
// it as soon as they press the control. (The press is answered by the probe, not
// stored: a kept place would stay in the vault's configuration for every probe
// after this one, and the probes that keep one stand last for that reason.)
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with sections, a link in the second one
// to a note that links on to a third), and MUTATE. MUTATE=list prints every
// watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/Reading%20flow.md';
const MUTATE = process.env.MUTATE || '';

const SITES = ['back-lands-on-the-note-they-left', 'the-section-stands-where-it-stood', 'the-place-is-drawn-where-it-is-kept'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN return-from-a-detour: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL return-from-a-detour: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN return-from-a-detour: ${message}`); };

const planted = { seen: 0, matched: 0 };
const rewrite = (match, needle, replacement) => async (page) => {
  await page.route(match, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    planted.seen += 1;
    if (original.includes(needle)) planted.matched += 1;
    await route.fulfill({ response, body: original.replaceAll(needle, replacement) });
  });
};
const addScript = (source) => async (page) => {
  await page.route('**/static/yomihon.js{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    planted.seen += 1;
    await route.fulfill({ response, body: `${original}\n${source}\n` });
  });
};

const MUTATIONS = {
  // Following a link replaces the page in the history, so there is no step back to
  // the note they left.
  'replace-the-page-when-a-link-is-followed': {
    target: 'back-lands-on-the-note-they-left',
    apply: addScript(`
      document.addEventListener('click', (event) => {
        const link = event.target.closest('a.wikilink');
        if (!link || event.defaultPrevented) return;
        event.preventDefault();
        location.replace(link.href);
      }, true);
    `),
  },
  // Pressing the control keeps the place and draws it nowhere.
  'draw-the-place-in-no-section-when-it-is-kept': {
    target: 'the-place-is-drawn-where-it-is-kept',
    apply: rewrite('**/mark.js{,?*}', 'if (kept) showPlace(place.anchor);', ''),
  },
  // Coming back puts the note at its top.
  'put-the-note-at-its-top-when-it-returns': {
    target: 'the-section-stands-where-it-stood',
    apply: addScript(`
      history.scrollRestoration = 'manual';
      addEventListener('pageshow', () => window.scrollTo(0, 0));
    `),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`return-from-a-detour: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`return-from-a-detour: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`return-from-a-detour: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  if (MUTATE) await MUTATIONS[MUTATE].apply(page);
  const start = BASE + PAGE;
  await page.goto(start, { waitUntil: 'load' });
  if (MUTATE && planted.seen === 0) throw new NotApplied(`NOT-APPLIED return-from-a-detour: ${MUTATE}: the planted change reached no page`);
  await page.evaluate(() => document.fonts.ready);

  // The reader chooses to keep their place, in the last section. The press is
  // answered here and not stored, so the page is as the reader finds it the moment
  // the control says the place is kept.
  await page.route('**/marks', async (route) => {
    if (route.request().method() === 'POST') await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    else await route.continue();
  });
  const placed = () => page.evaluate(() => [...document.querySelectorAll('.y-prose .y-sect')]
    .filter((section) => section.querySelector('[data-section-place]:not([hidden])'))
    .map((section) => section.firstElementChild.id));
  if ((await placed()).length !== 0) broken('the note is drawn with a place already kept in it');
  const target = 'the-third-mechanism';
  await page.evaluate((id) => window.scrollTo(0, document.getElementById(id).getBoundingClientRect().top + scrollY + 80), target);
  await page.waitForTimeout(150);
  await page.locator('.y-header [data-mark-button]').click();
  await page.waitForTimeout(400);
  const keptIn = await placed();
  if (keptIn.length !== 1 || keptIn[0] !== target) fail('the-place-is-drawn-where-it-is-kept', `the place was kept under ${target} and is drawn in ${JSON.stringify(keptIn)}`);

  // Stand at the second section, with its link in view.
  const link = page.locator('.y-prose .y-sect ~ p a.wikilink').first();
  if ((await link.count()) === 0) broken(`${PAGE} has no link in a section to follow`);
  await page.evaluate(() => window.scrollTo(0, document.getElementById('the-second-mechanism').getBoundingClientRect().top + scrollY + 150));
  await link.scrollIntoViewIfNeeded();
  await page.waitForTimeout(150);
  const left = await page.evaluate(() => {
    const sections = [...document.querySelectorAll('.y-prose .y-sect > [id]')];
    const above = sections.filter((heading) => heading.getBoundingClientRect().top <= 0).pop();
    return { scrollY: Math.round(scrollY), heading: above?.id ?? null, offset: above ? Math.round(above.getBoundingClientRect().top) : null };
  });
  if (left.scrollY < 400 || left.heading === null) broken(`the reader is not in a section to leave: ${JSON.stringify(left)}`);
  const startedAt = page.url();

  // On into a note, and on into a third.
  await link.click();
  await page.waitForURL((url) => url.pathname.endsWith('Reading%20flow%20detour.md') || decodeURIComponent(url.pathname).endsWith('Reading flow detour.md'));
  await page.locator('a.wikilink', { hasText: 'Reading flow deeper' }).first().click();
  await page.waitForURL((url) => decodeURIComponent(url.pathname).endsWith('Reading flow deeper.md'));
  if ((await page.locator('h1.y-title').textContent())?.trim() !== 'Reading flow deeper') broken('the third step is not the third note');

  // And back, twice.
  await page.goBack({ waitUntil: 'load' });
  await page.goBack({ waitUntil: 'load' });
  await page.waitForTimeout(400);
  if (page.url() !== startedAt) fail('back-lands-on-the-note-they-left', `two steps back is ${page.url()}, want the note they left, ${startedAt}`);
  const back = await page.evaluate(() => {
    const sections = [...document.querySelectorAll('.y-prose .y-sect > [id]')];
    const above = sections.filter((heading) => heading.getBoundingClientRect().top <= 0).pop();
    const margin = above?.parentElement.querySelector('[data-section-margin] [data-section-mark]');
    return { scrollY: Math.round(scrollY), heading: above?.id ?? null, offset: above ? Math.round(above.getBoundingClientRect().top) : null, controls: Boolean(margin) };
  });
  if (Math.abs(back.scrollY - left.scrollY) > 2 || back.heading !== left.heading) {
    fail('the-section-stands-where-it-stood', `the reader left at ${JSON.stringify(left)} and came back to ${JSON.stringify(back)}`);
  }
  if (!back.controls) fail('the-section-stands-where-it-stood', `the section they came back to has none of its controls: ${JSON.stringify(back)}`);

  console.log(`PASS return-from-a-detour: two steps on and two back land on the note at the section and the scroll position it was left at (${left.heading}, ${left.scrollY}px), with the section's controls in place; a place kept on the way is drawn in the section that holds it`);
} catch (error) {
  console.error(error.message);
  if (error instanceof NotApplied) {
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    if (MUTATE && error.site === MUTATIONS[MUTATE].target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
