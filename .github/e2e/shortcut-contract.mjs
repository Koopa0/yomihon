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
const SHORTCUT_CONTROL = '[data-single-key-shortcuts-toggle]';
const SHORTCUT_LABEL = '.y-shortcutpref';
const SHORTCUT_ON = '.y-shortcutpref__on';
const SHORTCUT_OFF = '.y-shortcutpref__off';

const SITES = [
  'modified-printables-stay-native',
  'plain-filter-opens',
  'plain-drawer-toggles',
  'plain-r-holds',
  'shift-r-holds',
  'escape-dismisses',
  'command-palette-chord',
  'default-on',
  'disabled-printables-stay-native',
  'disable-cancels-held-r',
  'disabled-command-and-escape',
  'disabled-persists',
  'reenable-restores',
  'visible-shortcut-state',
  'shortcut-focus-visible',
  'narrow-shortcut-control',
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

const rewriteDocument = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(`**${PAGE}`, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({ response, body: count === 1 ? original.replace(needle, replacement) : original });
  });
  return () => {
    if (requests !== 1) return `document was requested ${requests} times before the lock, want exactly 1`;
    if (matches !== 1) return `document needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const injectStyle = (css) => async (page) => {
  await page.addInitScript((rule) => {
    window.addEventListener('DOMContentLoaded', () => {
      const style = document.createElement('style');
      style.dataset.shortcutContractMutation = '';
      style.textContent = rule;
      document.head.append(style);
    }, { once: true });
  }, css);
  return async () => {
    const count = await page.locator('style[data-shortcut-contract-mutation]').count();
    return count === 1 ? '' : `injected style count was ${count}, want exactly 1`;
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
  'default-shortcuts-off': {
    target: 'default-on',
    apply: rewriteDocument('data-single-key-shortcuts="on"', 'data-single-key-shortcuts="off"'),
  },
  'ignore-disabled-preference': {
    target: 'disabled-printables-stay-native',
    apply: rewriteScript("    if (root.dataset.singleKeyShortcuts === 'off') return;\n", ''),
  },
  'disable-does-not-cancel-hold': {
    target: 'disable-cancels-held-r',
    apply: async (page) => {
      let requests = 0;
      let matches = 0;
      await page.route('**/preferences.js', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const needle = '    if (!enabled) onSingleKeyShortcutsDisabled();\n';
        const count = original.split(needle).length - 1;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(needle, '') : original });
      });
      return () => {
        if (requests !== 1) return `preferences runtime was requested ${requests} times, want exactly 1`;
        if (matches !== 1) return `preferences cancellation needle matched ${matches} times, want exactly 1`;
        return '';
      };
    },
  },
  'disable-command-and-escape-with-printables': {
    target: 'disabled-command-and-escape',
    apply: rewriteScript(
      "    if ((event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {",
      "    if (root.dataset.singleKeyShortcuts === 'off') return;\n    if ((event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {",
    ),
  },
  'do-not-persist-shortcut-preference': {
    target: 'disabled-persists',
    apply: async (page) => {
      let requests = 0;
      let matches = 0;
      await page.route('**/preferences.js', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const placeholder = '$' + '{value}';
        const needle = `    document.cookie = \`yomihon_shortcuts=${placeholder};path=/;max-age=31536000;samesite=lax\`;\n`;
        const count = original.split(needle).length - 1;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(needle, '') : original });
      });
      return () => {
        if (requests !== 1) return `preferences runtime was requested ${requests} times, want exactly 1`;
        if (matches !== 1) return `shortcut-cookie needle matched ${matches} times, want exactly 1`;
        return '';
      };
    },
  },
  'never-reenable-single-keys': {
    target: 'reenable-restores',
    apply: async (page) => {
      let requests = 0;
      let matches = 0;
      await page.route('**/preferences.js', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const needle = "    const value = event.currentTarget.checked ? 'on' : 'off';\n";
        const count = original.split(needle).length - 1;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(needle, "    const value = 'off';\n") : original });
      });
      return () => {
        if (requests !== 1) return `preferences runtime was requested ${requests} times, want exactly 1`;
        if (matches !== 1) return `reenable needle matched ${matches} times, want exactly 1`;
        return '';
      };
    },
  },
  'hide-visible-shortcut-state': {
    target: 'visible-shortcut-state',
    apply: injectStyle('.y-shortcutpref__on,.y-shortcutpref__off{display:none!important}'),
  },
  'remove-shortcut-focus-ring': {
    target: 'shortcut-focus-visible',
    apply: injectStyle('.y-shortcutpref__input:focus-visible{outline:none!important}'),
  },
  'push-shortcut-control-out-of-header': {
    target: 'narrow-shortcut-control',
    apply: injectStyle('@media(max-width:520px){.y-shortcutpref{position:fixed!important;left:-10000px!important}}'),
  },
  'keep-wide-shortcut-label': {
    target: 'narrow-shortcut-control',
    // Keep the mutation at the observed desktop control width so its overflow
    // does not depend on platform-specific CJK fallback-font metrics.
    apply: injectStyle('@media(max-width:520px){.y-shortcutpref{gap:5px!important;padding-inline:8px!important;min-inline-size:86px!important}.y-shortcutpref__label{display:inline!important}.y-shortcutpref__label-compact{display:none!important}}'),
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

const state = (page) => page.evaluate(({ dialog, filter, fills, shortcutControl, shortcutLabel, shortcutOn, shortcutOff }) => ({
  nav: document.documentElement.dataset.nav,
  shortcuts: document.documentElement.dataset.singleKeyShortcuts,
  dialogOpen: Boolean(document.querySelector(dialog)?.open),
  filterFocused: document.activeElement === document.querySelector(filter),
  fillWidths: [...document.querySelectorAll(fills)].map((element) => element.style.width),
  shortcutChecked: document.querySelector(shortcutControl)?.checked,
  shortcutOnDisplay: document.querySelector(shortcutOn) ? getComputedStyle(document.querySelector(shortcutOn)).display : null,
  shortcutOffDisplay: document.querySelector(shortcutOff) ? getComputedStyle(document.querySelector(shortcutOff)).display : null,
  shortcutBox: (() => {
    const element = document.querySelector(shortcutLabel);
    if (!element) return null;
    const rect = element.getBoundingClientRect();
    return { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height };
  })(),
  header: (() => {
    const element = document.querySelector('.y-header');
    return element ? { clientWidth: element.clientWidth, scrollWidth: element.scrollWidth } : null;
  })(),
}), {
  dialog: DIALOG,
  filter: FILTER,
  fills: FILLS,
  shortcutControl: SHORTCUT_CONTROL,
  shortcutLabel: SHORTCUT_LABEL,
  shortcutOn: SHORTCUT_ON,
  shortcutOff: SHORTCUT_OFF,
});

const press = async (page, key, modifiers = {}) => {
  const result = await dispatch(page, 'keydown', key, modifiers);
  await dispatch(page, 'keyup', key, modifiers);
  return result;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const context = await browser.newContext({ viewport: { width: 800, height: 800 } });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('html[data-js][data-nav="closed"]');
  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const initial = await state(page);
  if (initial.fillWidths.length === 0) broken('the fixture has no seal fill to observe');
  if (await page.locator(FILTER).count() !== 1) broken('the fixture has no single drawer filter');
  if (await page.locator(DIALOG).count() !== 1) broken('the fixture has no single search dialog');
  if (await page.locator(SHORTCUT_CONTROL).count() !== 1) broken('the page has no single-key-shortcut preference control');
  if (initial.shortcuts !== 'on' || initial.shortcutChecked !== true) {
    fail('default-on', `default rendered shortcuts=${initial.shortcuts}, checked=${initial.shortcutChecked}`);
  }
  if (initial.shortcutOnDisplay === 'none' || initial.shortcutOffDisplay !== 'none') {
    fail('visible-shortcut-state', `enabled state displays on=${initial.shortcutOnDisplay}, off=${initial.shortcutOffDisplay}`);
  }
  await page.locator(SHORTCUT_CONTROL).focus();
  const focusRing = await page.locator(SHORTCUT_CONTROL).evaluate((element) => {
    const style = getComputedStyle(element);
    return { style: style.outlineStyle, width: style.outlineWidth, color: style.outlineColor };
  });
  if (focusRing.style === 'none' || Number.parseFloat(focusRing.width) <= 0 || focusRing.color === 'rgba(0, 0, 0, 0)') {
    fail('shortcut-focus-visible', `focused checkbox outline=${JSON.stringify(focusRing)}`);
  }

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

  const heldBeforeDisable = await dispatch(page, 'keydown', 'r');
  after = await state(page);
  if (!heldBeforeDisable.defaultPrevented || !after.fillWidths.every((width) => width === '100%')) {
    fail('plain-r-holds', 'plain R did not start the hold used by the disable transition');
  }
  await page.locator(SHORTCUT_CONTROL).uncheck();
  after = await state(page);
  if (!after.fillWidths.every((width) => width === '0px')) {
    fail('disable-cancels-held-r', `turning shortcuts off left fills ${JSON.stringify(after.fillWidths)}`);
  }
  await dispatch(page, 'keyup', 'r');
  if (after.shortcuts !== 'off' || after.shortcutChecked !== false) {
    fail('disabled-printables-stay-native', `disabled state rendered shortcuts=${after.shortcuts}, checked=${after.shortcutChecked}`);
  }
  if (after.shortcutOnDisplay !== 'none' || after.shortcutOffDisplay === 'none') {
    fail('visible-shortcut-state', `disabled state displays on=${after.shortcutOnDisplay}, off=${after.shortcutOffDisplay}`);
  }

  for (const key of ['/', '[', 'r', 'R']) {
    const modifiers = key === 'R' ? { shiftKey: true } : {};
    const result = await dispatch(page, 'keydown', key, modifiers);
    const during = await state(page);
    await dispatch(page, 'keyup', key, modifiers);
    if (
      result.defaultPrevented
      || during.nav !== 'closed'
      || during.filterFocused
      || during.fillWidths.some((width) => width === '100%')
    ) {
      fail('disabled-printables-stay-native', `disabled ${key} was captured`);
    }
  }

  const disabledChord = await dispatch(page, 'keydown', 'k', { ctrlKey: true });
  after = await state(page);
  if (!disabledChord.defaultPrevented || !after.dialogOpen) {
    fail('disabled-command-and-escape', `disabled Ctrl+K left open=${after.dialogOpen}, prevented=${disabledChord.defaultPrevented}`);
  }
  await dispatch(page, 'keyup', 'k', { ctrlKey: true });
  const disabledEscape = await press(page, 'Escape');
  after = await state(page);
  if (disabledEscape.defaultPrevented !== true || after.dialogOpen) {
    fail('disabled-command-and-escape', `disabled Escape left open=${after.dialogOpen}, prevented=${disabledEscape.defaultPrevented}`);
  }

  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.waitForSelector('html[data-js][data-nav="closed"]');
  after = await state(page);
  if (after.shortcuts !== 'off' || after.shortcutChecked !== false) {
    fail('disabled-persists', `reload rendered shortcuts=${after.shortcuts}, checked=${after.shortcutChecked}`);
  }
  const secondPage = await context.newPage();
  await secondPage.goto(BASE + '/', { waitUntil: 'domcontentloaded' });
  await secondPage.waitForSelector('html[data-js]');
  const secondState = await state(secondPage);
  if (secondState.shortcuts !== 'off' || secondState.shortcutChecked !== false) {
    fail('disabled-persists', `new page rendered shortcuts=${secondState.shortcuts}, checked=${secondState.shortcutChecked}`);
  }
  await secondPage.close();

  await page.locator(SHORTCUT_CONTROL).check();
  after = await state(page);
  if (after.shortcuts !== 'on' || after.shortcutChecked !== true) {
    fail('reenable-restores', `re-enabled state rendered shortcuts=${after.shortcuts}, checked=${after.shortcutChecked}`);
  }
  const restoredBracket = await dispatch(page, 'keydown', '[');
  after = await state(page);
  if (!restoredBracket.defaultPrevented || after.nav !== 'open') {
    fail('reenable-restores', `re-enabled [ left nav=${after.nav}, prevented=${restoredBracket.defaultPrevented}`);
  }
  await dispatch(page, 'keyup', '[');
  await press(page, '[');

  await page.setViewportSize({ width: 360, height: 800 });
  after = await state(page);
  if (
    !after.shortcutBox
    || after.shortcutBox.width <= 0
    || after.shortcutBox.height <= 0
    || after.shortcutBox.left < 0
    || after.shortcutBox.right > 360
    || !after.header
    || after.header.scrollWidth > after.header.clientWidth
  ) {
    fail('narrow-shortcut-control', `360px header=${JSON.stringify(after.header)}, shortcut=${JSON.stringify(after.shortcutBox)}`);
  }

  console.log('PASS shortcut-contract: single-key preference is default-on, exact, persisted, reversible, visible, and narrow-safe; other keys remain available');
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
