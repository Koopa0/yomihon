// Behavior lock for the global shortcut boundary. The three printable-key
// actions are deliberately available without modifiers, while browser and OS
// chords remain untouched. Escape and the command-palette chord are outside
// that printable-key guard and keep their native roles.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a drawer filter and seal form), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const FILTER = '[data-nav-filter]';
const NAV_TOGGLE = '[data-nav-toggle]';
const DIALOG = '[data-search]';
const FILLS = '.y-sealfill';

const SITES = [
  'modified-printables-stay-native',
  'plain-filter-opens',
  'plain-drawer-toggles',
  'plain-r-holds',
  'shift-r-holds',
  'escape-dismisses',
  'command-palette-chord',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN shortcut-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL shortcut-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN shortcut-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED shortcut-contract: ${message}`); };

const rewriteScript = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/shortcuts.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `runtime was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `runtime needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'remove-modifier-guard': {
    target: 'modified-printables-stay-native',
    apply: rewriteScript('    if (event.metaKey || event.ctrlKey || event.altKey) return;\n', ''),
  },
  'disable-plain-filter': {
    target: 'plain-filter-opens',
    apply: rewriteScript("    if (event.key === '/') {", '    if (false) {'),
  },
  'disable-plain-drawer': {
    target: 'plain-drawer-toggles',
    apply: rewriteScript("    if (event.key === '[') {", '    if (false) {'),
  },
  'disable-lowercase-r': {
    target: 'plain-r-holds',
    apply: rewriteScript(
      "    if ((event.key === 'r' || event.key === 'R') && !event.repeat && status.canStartShortcutHold()) {",
      "    if (event.key === 'R' && !event.repeat && status.canStartShortcutHold()) {",
    ),
  },
  'disable-uppercase-r': {
    target: 'shift-r-holds',
    apply: rewriteScript(
      "    if ((event.key === 'r' || event.key === 'R') && !event.repeat && status.canStartShortcutHold()) {",
      "    if (event.key === 'r' && !event.repeat && status.canStartShortcutHold()) {",
    ),
  },
  'disable-global-escape': {
    target: 'escape-dismisses',
    apply: rewriteScript("    if (event.key === 'Escape') {", '    if (false) {'),
  },
  'disable-command-palette-chord': {
    target: 'command-palette-chord',
    apply: rewriteScript(
      "    if ((event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {",
      '    if (false) {',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`shortcut-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`shortcut-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`shortcut-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const dispatch = (page, type, key, modifiers = {}) => page.evaluate(({ type, key, modifiers }) => {
  const event = new KeyboardEvent(type, {
    key,
    bubbles: true,
    cancelable: true,
    repeat: false,
    ...modifiers,
  });
  const accepted = window.dispatchEvent(event);
  return { defaultPrevented: event.defaultPrevented || !accepted };
}, { type, key, modifiers });

const state = (page) => page.evaluate(({ dialog, filter, fills }) => ({
  nav: document.documentElement.dataset.nav,
  dialogOpen: Boolean(document.querySelector(dialog)?.open),
  filterFocused: document.activeElement === document.querySelector(filter),
  fillWidths: [...document.querySelectorAll(fills)].map((element) => element.style.width),
}), { dialog: DIALOG, filter: FILTER, fills: FILLS });

const press = async (page, key, modifiers = {}) => {
  const result = await dispatch(page, 'keydown', key, modifiers);
  await dispatch(page, 'keyup', key, modifiers);
  return result;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 800, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('html[data-js][data-nav="closed"]');
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const initial = await state(page);
  if (initial.fillWidths.length === 0) broken('the fixture has no seal fill to observe');
  if (await page.locator(FILTER).count() !== 1) broken('the fixture has no single drawer filter');
  if (await page.locator(DIALOG).count() !== 1) broken('the fixture has no single search dialog');

  const modified = [
    { name: 'Meta', values: { metaKey: true } },
    { name: 'Control', values: { ctrlKey: true } },
    { name: 'Alt', values: { altKey: true } },
  ];
  for (const { name, values } of modified) {
    for (const key of ['/', '[', 'r', 'R']) {
      const modifiers = key === 'R' ? { ...values, shiftKey: true } : values;
      const result = await dispatch(page, 'keydown', key, modifiers);
      const during = await state(page);
      await dispatch(page, 'keyup', key, modifiers);
      if (result.defaultPrevented || during.nav !== 'closed' || during.filterFocused || during.fillWidths.some((width) => width === '100%')) {
        fail('modified-printables-stay-native', `${name}+${key} was captured by a printable shortcut`);
      }
    }
  }

  const slash = await dispatch(page, 'keydown', '/');
  let after = await state(page);
  if (!slash.defaultPrevented || after.nav !== 'open' || !after.filterFocused) {
    fail('plain-filter-opens', `plain / left nav=${after.nav}, filterFocused=${after.filterFocused}, prevented=${slash.defaultPrevented}`);
  }
  await page.locator(NAV_TOGGLE).click();
  await page.waitForFunction(() => document.documentElement.dataset.nav === 'closed');

  const bracket = await dispatch(page, 'keydown', '[');
  after = await state(page);
  if (!bracket.defaultPrevented || after.nav !== 'open') {
    fail('plain-drawer-toggles', `plain [ left nav=${after.nav}, prevented=${bracket.defaultPrevented}`);
  }
  await press(page, '[');
  if ((await state(page)).nav !== 'closed') fail('plain-drawer-toggles', 'the second plain [ did not close the drawer');

  const plainR = await dispatch(page, 'keydown', 'r');
  after = await state(page);
  if (!plainR.defaultPrevented || !after.fillWidths.every((width) => width === '100%')) {
    fail('plain-r-holds', `plain R produced fills ${JSON.stringify(after.fillWidths)} and prevented=${plainR.defaultPrevented}`);
  }
  await press(page, 'Escape');
  after = await state(page);
  if (!after.fillWidths.every((width) => width === '0px')) fail('escape-dismisses', 'Escape did not cancel an active seal hold');
  await dispatch(page, 'keyup', 'r');

  const shiftedR = await dispatch(page, 'keydown', 'R', { shiftKey: true });
  after = await state(page);
  if (!shiftedR.defaultPrevented || !after.fillWidths.every((width) => width === '100%')) {
    fail('shift-r-holds', `Shift+R produced fills ${JSON.stringify(after.fillWidths)} and prevented=${shiftedR.defaultPrevented}`);
  }
  await dispatch(page, 'keyup', 'R', { shiftKey: true });

  for (const modifiers of [{ ctrlKey: true }, { metaKey: true }]) {
    for (const key of ['k', 'K']) {
      const chordModifiers = key === 'K' ? { ...modifiers, shiftKey: true } : modifiers;
      const chord = await dispatch(page, 'keydown', key, chordModifiers);
      after = await state(page);
      if (!chord.defaultPrevented || !after.dialogOpen) {
        fail('command-palette-chord', `command-palette chord left open=${after.dialogOpen}, prevented=${chord.defaultPrevented}`);
      }
      await dispatch(page, 'keyup', key, chordModifiers);
      await press(page, 'Escape');
      after = await state(page);
      if (after.dialogOpen) fail('escape-dismisses', 'Escape did not close the command palette');
    }
  }

  console.log('PASS shortcut-contract: modified printables stay native; plain /, [, R, Shift+R, Escape, and Cmd/Ctrl+K remain available');
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
