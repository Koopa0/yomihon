// Behavior lock for choosing a hit in the search palette by keyboard. The
// palette is where a keyboard reader lives — open it, type, go — and the last
// step used to leave the keyboard: Enter could only submit the query to the
// full search page. Now the field is a combobox over its rows: ↑ and ↓ move a
// choice while focus stays in the field, Enter opens the chosen row at the
// match it names, and with nothing chosen, or with Shift held, Enter is the
// form's own submit. A key that is still part of composing a word is left to
// the input method, and a choice never outlives the rows it was made among or
// the palette being closed.
//
// Go tests cannot see this: it is a handler on a live field over rows that a
// fetch replaces.
//
// Env: YOMIHON_BASE, PAGE_PATH (any page carrying the header's palette), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';
// A word the fixture holds in several notes, so there is a second row to move
// to and a last one to wrap from.
const QUERY = 'alpha';
const SITES = [
  'arrows-move-the-choice',
  'enter-opens-the-chosen-row',
  'enter-without-a-choice-searches',
  'shift-enter-searches',
  'typing-forgets-the-choice',
  'closing-forgets-the-choice',
  'composition-keeps-its-keys',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN search-keys: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL search-keys: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN search-keys: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED search-keys: ${message}`); };

// Rewrites the served search module, and refuses to report a catch when the
// needle no longer names anything in it.
const rewriteModule = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/search.js{,?*}', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (requests < 1) return 'the search module was never requested';
    if (matches !== requests) return `the needle matched ${matches} times over ${requests} requests, want exactly 1 each`;
    return '';
  };
};

const MUTATIONS = {
  // The field answers no arrow key, as it did before.
  'drop-the-key-handler': {
    target: 'arrows-move-the-choice',
    apply: rewriteModule("  input.addEventListener('keydown', (event) => {", "  input.addEventListener('keydown-gone', (event) => {"),
  },
  // Enter goes on submitting whatever has been chosen.
  'enter-ignores-the-choice': {
    target: 'enter-opens-the-chosen-row',
    apply: rewriteModule("event.key === 'Enter' && !event.shiftKey && found[chosen]", "event.key === 'Enter-gone' && !event.shiftKey && found[chosen]"),
  },
  // Enter opens the first row even when nothing was chosen.
  'enter-opens-the-first-row-unasked': {
    target: 'enter-without-a-choice-searches',
    apply: rewriteModule("!event.shiftKey && found[chosen]) {\n      event.preventDefault();\n      found[chosen].click();", "!event.shiftKey && found[Math.max(chosen, 0)]) {\n      event.preventDefault();\n      found[Math.max(chosen, 0)].click();"),
  },
  // Shift stops meaning "the whole search page".
  'shift-enter-opens-the-row': {
    target: 'shift-enter-searches',
    apply: rewriteModule("event.key === 'Enter' && !event.shiftKey && found[chosen]", "event.key === 'Enter' && found[chosen]"),
  },
  // Shift+Enter left to the engine, which does not submit for it.
  'drop-the-shift-submit': {
    target: 'shift-enter-searches',
    apply: rewriteModule('      input.form?.requestSubmit();\n', ''),
  },
  // Typing leaves the old choice standing over rows that are about to change.
  'keep-a-stale-choice': {
    target: 'typing-forgets-the-choice',
    apply: rewriteModule("    input.addEventListener('input', () => {\n      rowsChanged();", "    input.addEventListener('input', () => {"),
  },
  // A choice left standing while the palette is closed, so Enter on the way
  // back in opens a row the reader chose before they left.
  'keep-the-choice-across-a-close': {
    target: 'closing-forgets-the-choice',
    apply: rewriteModule('          cancelPending();\n          rowsChanged();\n', '          cancelPending();\n'),
  },
  // Arrow keys taken from an input method in the middle of a word.
  'take-keys-from-composition': {
    target: 'composition-keeps-its-keys',
    apply: rewriteModule('    if (event.isComposing) return;\n', ''),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`search-keys: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`search-keys: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`search-keys: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const INPUT = '.y-searchdialog__input';

// What the field and its rows say about the choice, as the page holds it.
const choice = (page) => page.evaluate((selector) => {
  const input = document.querySelector(selector);
  const rows = [...document.querySelectorAll('.y-searchdialog a.y-result')];
  return {
    focused: document.activeElement === input,
    role: input?.getAttribute('role'),
    active: input?.getAttribute('aria-activedescendant') ?? null,
    selected: rows.map((row) => row.getAttribute('aria-selected')),
    ids: rows.map((row) => row.id),
    hrefs: rows.map((row) => row.getAttribute('href')),
  };
}, INPUT);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  // Opens the palette on a fresh page, types the query, and waits until its
  // answer has landed.
  const openWithRows = async () => {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();
    const pageProof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
    proof ??= pageProof;
    await page.goto(BASE + PAGE, { waitUntil: 'load' });
    await page.waitForSelector('html[data-js]');
    await page.locator('[data-search-open]').click();
    await page.waitForFunction((selector) => document.activeElement === document.querySelector(selector), INPUT);
    await page.keyboard.type(QUERY);
    await page.waitForFunction(() => {
      const results = document.querySelector('.y-searchdialog [data-live-search-results]');
      return results?.getAttribute('aria-busy') === 'false' && results.querySelectorAll('a.y-result').length >= 2;
    }, null, { timeout: 5000 }).catch(() => broken(`typing ${QUERY} into the palette brought back fewer than two rows`));
    return { page, context };
  };

  // ↑ and ↓ move the choice, focus stays in the field, and the field names the
  // row it is on.
  {
    const { page, context } = await openWithRows();
    const before = await choice(page);
    if (before.role !== 'combobox') fail('arrows-move-the-choice', `the palette's field has role=${before.role}, want combobox`);
    if (before.active !== null || before.selected.includes('true')) fail('arrows-move-the-choice', `a row is chosen before any key was pressed: ${JSON.stringify(before)}`);
    await page.keyboard.press('ArrowDown');
    const first = await choice(page);
    if (!first.focused || first.selected[0] !== 'true' || first.active !== first.ids[0] || !first.ids[0]) {
      fail('arrows-move-the-choice', `↓ did not choose the first row with focus kept in the field: ${JSON.stringify(first)}`);
    }
    await page.keyboard.press('ArrowDown');
    const second = await choice(page);
    if (second.selected[1] !== 'true' || second.selected[0] !== 'false' || second.active !== second.ids[1]) {
      fail('arrows-move-the-choice', `a second ↓ did not move the choice to the second row: ${JSON.stringify(second)}`);
    }
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowUp');
    const wrapped = await choice(page);
    const last = wrapped.ids.length - 1;
    if (wrapped.selected[last] !== 'true' || wrapped.active !== wrapped.ids[last]) {
      fail('arrows-move-the-choice', `↑ from the first row did not wrap to the last: ${JSON.stringify(wrapped)}`);
    }

    // Enter opens the chosen row at the match it names.
    await page.keyboard.press('ArrowDown');
    const chosen = await choice(page);
    const href = chosen.hrefs[0];
    if (chosen.selected[0] !== 'true' || !href) fail('arrows-move-the-choice', `↓ from the last row did not come back to the first: ${JSON.stringify(chosen)}`);
    // The row names a place inside its note as a text directive, which the
    // browser keeps out of the address it reports; the note's own path is
    // what can be compared, and the directive is required of the row so the
    // case is about opening a hit rather than a note.
    if (!href.includes('#:~:text=')) broken(`the first row ${href} names no match inside its note`);
    const wanted = new URL(href, BASE);
    await Promise.all([page.waitForURL((url) => url.pathname === wanted.pathname, { timeout: 5000 }).catch(() => {}), page.keyboard.press('Enter')]);
    const landed = new URL(page.url());
    if (landed.pathname !== wanted.pathname) {
      fail('enter-opens-the-chosen-row', `Enter on the first row landed on ${landed.href}, want ${wanted.pathname}`);
    }
    await context.close();
  }

  // With nothing chosen, Enter is the form's own submit.
  {
    const { page, context } = await openWithRows();
    await Promise.all([page.waitForURL(/\/search\?/, { timeout: 5000 }).catch(() => {}), page.keyboard.press('Enter')]);
    const landed = new URL(page.url());
    if (landed.pathname !== '/search' || landed.searchParams.get('q') !== QUERY) {
      fail('enter-without-a-choice-searches', `Enter with no row chosen landed on ${landed.href}, want /search?q=${QUERY}`);
    }
    await context.close();
  }

  // With Shift held, Enter submits even over a chosen row.
  {
    const { page, context } = await openWithRows();
    await page.keyboard.press('ArrowDown');
    if ((await choice(page)).selected[0] !== 'true') fail('arrows-move-the-choice', 'no row was chosen before Shift+Enter');
    await Promise.all([page.waitForURL(/\/search\?/, { timeout: 5000 }).catch(() => {}), page.keyboard.press('Shift+Enter')]);
    const landed = new URL(page.url());
    if (landed.pathname !== '/search' || landed.searchParams.get('q') !== QUERY) {
      fail('shift-enter-searches', `Shift+Enter over a chosen row landed on ${landed.href}, want /search?q=${QUERY}`);
    }
    await context.close();
  }

  // Typing forgets the choice at once, before the new rows land, so Enter in
  // that moment searches for what is in the box rather than opening a row the
  // reader has typed past.
  {
    const { page, context } = await openWithRows();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.type('x');
    const typed = await choice(page);
    if (typed.active !== null || typed.selected.includes('true')) {
      fail('typing-forgets-the-choice', `a keystroke after choosing left the choice standing: ${JSON.stringify(typed)}`);
    }
    await context.close();
  }

  // Closing the palette forgets the choice, so the reader comes back to the
  // field and the rows rather than to a row chosen before they left.
  {
    const { page, context } = await openWithRows();
    await page.keyboard.press('ArrowDown');
    if ((await choice(page)).selected[0] !== 'true') fail('arrows-move-the-choice', 'no row was chosen before the palette was closed');
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => !document.querySelector('.y-searchdialog')?.open);
    await page.locator('[data-search-open]').click();
    await page.waitForFunction((selector) => document.activeElement === document.querySelector(selector), INPUT);
    const back = await choice(page);
    if (back.active !== null || back.selected.includes('true')) {
      fail('closing-forgets-the-choice', `reopening the palette found the old choice still standing: ${JSON.stringify(back)}`);
    }
    await context.close();
  }

  // A key that is part of composing a word belongs to the input method.
  {
    const { page, context } = await openWithRows();
    await page.evaluate((selector) => {
      document.querySelector(selector).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', isComposing: true, bubbles: true, cancelable: true }));
    }, INPUT);
    const composing = await choice(page);
    if (composing.active !== null || composing.selected.includes('true')) {
      fail('composition-keeps-its-keys', `an arrow key sent while composing chose a row: ${JSON.stringify(composing)}`);
    }
    await context.close();
  }

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }
  console.log('PASS search-keys: in the palette ↑ and ↓ move a choice with focus kept in the field, Enter opens the chosen row at its match, Enter alone and Shift+Enter search, typing and closing forget the choice, and composition keeps its keys');
} catch (err) {
  if (proof && !(err instanceof NotApplied)) {
    const issue = proof();
    if (issue) {
      console.error(`NOT-APPLIED search-keys: ${MUTATE}: ${issue}`);
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
      await browser.close();
      process.exit();
    }
  }
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
