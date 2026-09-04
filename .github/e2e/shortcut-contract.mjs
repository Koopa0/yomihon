// Behavior lock for the global shortcut boundary. The two printable-key
// actions are deliberately available without modifiers, while browser and OS
// chords remain untouched. Escape and the command-palette chord are outside
// that printable-key guard and keep their native roles.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a drawer filter), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';
const FILTER = '[data-nav-filter]';
const NAV_TOGGLE = '[data-nav-toggle]';
const DIALOG = '[data-search]';
const SHORTCUT_CONTROL = '[data-single-key-shortcuts-toggle]';
const SHORTCUT_LABEL = '.y-shortcutpref';
const SHORTCUT_ON = '.y-shortcutpref__on';
const SHORTCUT_OFF = '.y-shortcutpref__off';
const HELP_BUTTON = '[popovertarget="kbd-help"]';
const HELP_PANEL = '#kbd-help';

const SITES = [
  'modified-printables-stay-native',
  'plain-filter-opens',
  'plain-drawer-toggles',
  'wide-drawer-key-stays-native',
  'wide-filter-escape-moves-nothing',
  'escape-dismisses',
  'composing-escape-stays-with-the-input',
  'command-palette-chord',
  'default-on',
  'disabled-printables-stay-native',
  'disabled-command-and-escape',
  'disabled-persists',
  'reenable-restores',
  'visible-shortcut-state',
  'shortcut-focus-visible',
  'shortcut-control-reachable',
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
  // The key claimed at a width where it has nothing to fold. The keyboard help
  // says it does nothing there, so a page that swallows it anyway leaves the
  // reader with a key that neither works nor reaches the browser.
  'arm-the-drawer-key-at-every-width': {
    target: 'wide-drawer-key-stays-native',
    apply: rewriteScript('      if (drawer.isNarrow()) {', '      if (true) {'),
  },
  // The filter answering an Escape that dismisses nothing, which at this width
  // takes focus off the box for no change the reader asked for.
  'answer-escape-from-a-filter-with-nothing-to-clear': {
    target: 'wide-filter-escape-moves-nothing',
    apply: async (page) => {
      let requests = 0;
      let matches = 0;
      await page.route('**/sidebar.js', async (route) => {
        requests += 1;
        const response = await route.fetch();
        const original = await response.text();
        const needle = '      if (!input.value.trim()) return;\n';
        const count = original.split(needle).length - 1;
        matches += count;
        await route.fulfill({ response, body: count === 1 ? original.replace(needle, '') : original });
      });
      return () => {
        if (requests !== 1) return `sidebar runtime was requested ${requests} times, want exactly 1`;
        if (matches !== 1) return `filter-escape needle matched ${matches} times, want exactly 1`;
        return '';
      };
    },
  },
  'disable-global-escape': {
    target: 'escape-dismisses',
    apply: rewriteScript("    if (event.key === 'Escape') {", '    if (false) {'),
  },
  // The module reading keys again while an input method is still composing.
  'act-while-the-input-method-composes': {
    target: 'composing-escape-stays-with-the-input',
    apply: rewriteScript('    if (event.isComposing) return;\n', ''),
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
  'push-shortcut-control-off-the-panel': {
    target: 'shortcut-control-reachable',
    // Scoped to the narrow viewport the site measures. Pushed off at every
    // width it would also stop the probe clicking the control earlier on,
    // and the run would exit 1 from a timeout instead of from the assertion
    // this aims at — an exit code that proves nothing.
    apply: injectStyle('@media(max-width:520px){.y-kbdpref{position:fixed!important;left:-10000px!important}}'),
  },
  'hide-what-the-keys-cost': {
    target: 'shortcut-control-reachable',
    apply: injectStyle('.y-kbdpref__note{display:none!important}'),
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

const state = (page) => page.evaluate(({ dialog, filter, shortcutControl, shortcutOn, shortcutOff }) => ({
  nav: document.documentElement.dataset.nav,
  shortcuts: document.documentElement.dataset.singleKeyShortcuts,
  dialogOpen: Boolean(document.querySelector(dialog)?.open),
  filterFocused: document.activeElement === document.querySelector(filter),
  shortcutChecked: document.querySelector(shortcutControl)?.checked,
  shortcutOnDisplay: document.querySelector(shortcutOn) ? getComputedStyle(document.querySelector(shortcutOn)).display : null,
  shortcutOffDisplay: document.querySelector(shortcutOff) ? getComputedStyle(document.querySelector(shortcutOff)).display : null,
}), {
  dialog: DIALOG,
  filter: FILTER,
  shortcutControl: SHORTCUT_CONTROL,
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
  if (await page.locator(FILTER).count() !== 1) broken('the fixture has no single drawer filter');
  if (await page.locator(DIALOG).count() !== 1) broken('the fixture has no single search dialog');
  if (await page.locator(SHORTCUT_CONTROL).count() !== 1) broken('the page has no single-key-shortcut preference control');
  // The preference lives in the keyboard help layer, so every question about
  // what a reader can see or reach is asked with that layer open. Reading it
  // closed measures a subtree the reader is not looking at.
  // Opened from the keyboard, because that is how the reader who cares about
  // this panel arrives at it — and because a mouse click here would put the
  // browser in pointer modality, so the focus ring asked about below would be
  // absent for a reason that has nothing to do with the stylesheet.
  const openHelp = async () => {
    if (await page.locator(HELP_PANEL).evaluate((el) => el.matches(':popover-open')).catch(() => false)) return;
    await page.locator(HELP_BUTTON).focus();
    await page.keyboard.press('Enter');
    await page.locator(HELP_PANEL).waitFor({ state: 'visible' });
  };
  await openHelp();
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
    for (const key of ['/', '[']) {
      const result = await dispatch(page, 'keydown', key, values);
      const during = await state(page);
      await dispatch(page, 'keyup', key, values);
      if (result.defaultPrevented || during.nav !== 'closed' || during.filterFocused) {
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

  // A composing input method owns Escape: it is how a half-typed word is
  // abandoned, and the page taking it instead cancels nothing and closes
  // something the reader was still using. The composition flag is set on the
  // event here rather than driven by a real input method, which no headless
  // run has; what it exercises is the guard, not the platform.
  //
  // The drawer is what it is asked about, and the search dialog deliberately
  // is not. That dialog is opened with showModal(), so the browser closes it
  // on a real Escape through its own close request, whatever the page decides
  // — and a dispatched event runs no default action at all, so a synthetic
  // Escape could neither close it nor show that a real one would. An
  // assertion there would hold however the guard was written. Closing the
  // drawer has no browser behaviour behind it: the page is the only thing
  // that can do it, which is what makes a dispatched event able to tell.
  await page.locator(NAV_TOGGLE).click();
  await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');
  await dispatch(page, 'keydown', 'Escape', { isComposing: true });
  after = await state(page);
  await dispatch(page, 'keyup', 'Escape', { isComposing: true });
  if (after.nav !== 'open') {
    fail('composing-escape-stays-with-the-input', `Escape during composition left nav=${after.nav}, want open`);
  }
  // And the same key, no longer composing, still closes it — otherwise the
  // assertion above would pass just as well on a page that had stopped
  // answering Escape at all.
  await press(page, 'Escape');
  after = await state(page);
  if (after.nav !== 'closed') {
    fail('composing-escape-stays-with-the-input', `Escape after a composition left nav=${after.nav}, want closed`);
  }

  await openHelp();
  await page.locator(SHORTCUT_CONTROL).uncheck();
  after = await state(page);
  if (after.shortcuts !== 'off' || after.shortcutChecked !== false) {
    fail('disabled-printables-stay-native', `disabled state rendered shortcuts=${after.shortcuts}, checked=${after.shortcutChecked}`);
  }
  if (after.shortcutOnDisplay !== 'none' || after.shortcutOffDisplay === 'none') {
    fail('visible-shortcut-state', `disabled state displays on=${after.shortcutOnDisplay}, off=${after.shortcutOffDisplay}`);
  }

  for (const key of ['/', '[']) {
    const result = await dispatch(page, 'keydown', key);
    const during = await state(page);
    await dispatch(page, 'keyup', key);
    if (result.defaultPrevented || during.nav !== 'closed' || during.filterFocused) {
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

  await openHelp();
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

  // The sidebar key reaches a drawer, and above the width where the sidebar
  // folds into one there is no drawer to reach. The keyboard help says so, so
  // what is asked here is that the key really is the browser's at that width:
  // a page that takes it and then declines to act would leave the reader
  // holding a key that does nothing in either place. The media query is waited
  // on rather than the drawer's own state, which reads closed at both widths
  // and would let this pass without the viewport ever having changed.
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.waitForFunction(() => !window.matchMedia('(max-width: 900px)').matches);
  const wideBracket = await dispatch(page, 'keydown', '[');
  after = await state(page);
  await dispatch(page, 'keyup', '[');
  if (wideBracket.defaultPrevented || after.nav === 'open') {
    fail('wide-drawer-key-stays-native', `wide [ left nav=${after.nav}, prevented=${wideBracket.defaultPrevented}`);
  }

  // At this width the filter's Escape has nothing behind it to fall through
  // to — the sidebar is simply present and there is no drawer to close — so
  // declining the key means the press does nothing at all, focus included.
  // That is the same rule read at a width where its effect is visible: a key
  // that dismisses nothing should not move the reader. Answering it here
  // emptied a box that was already empty and put focus somewhere the reader
  // had not asked for.
  //
  // The second half is what keeps the first from passing on a filter that had
  // stopped reading Escape altogether.
  //
  // Where focus sits is the whole question, because moving it is the page's
  // own act and nothing else's. What the box then contains is not asked: a
  // browser may revert a field's text on Escape by itself, and an assertion
  // on the text failed here about one run in three for that reason alone —
  // it was reading the browser's behaviour and calling it this page's.
  const filterEscapeKeepsFocus = async (content) => {
    await page.locator(FILTER).fill(content);
    await page.locator(FILTER).focus();
    await page.locator(FILTER).press('Escape');
    return page.evaluate((selector) => document.activeElement === document.querySelector(selector), FILTER);
  };
  if (!(await filterEscapeKeepsFocus('   '))) {
    fail('wide-filter-escape-moves-nothing', 'Escape on a filter narrowing nothing took focus off the box');
  }
  if (await filterEscapeKeepsFocus('alpha')) {
    fail('wide-filter-escape-moves-nothing', 'Escape on a narrowing filter left focus in the box it had just cleared');
  }
  // Escape is also how the browser dismisses an open popover, so those presses
  // shut the keyboard help. It is reopened here, at a width where its button
  // still has a face: below 520 the button is deliberately absent, and the
  // measurement that follows needs the panel already open.
  await openHelp();

  // 360 has to reach the page as 360 pixels of room to lay out in. Where the
  // scrollbar keeps a column of its own, a 360-wide window leaves the page 345,
  // so the column is measured the way the layout sees it, from an element fixed
  // to the edges, and given back.
  await page.setViewportSize({ width: 360, height: 800 });
  const gutter = await page.evaluate(() => {
    const probe = document.createElement('div');
    probe.style.cssText = 'position:fixed;inset:0;visibility:hidden;pointer-events:none';
    document.body.appendChild(probe);
    const room = probe.getBoundingClientRect().width;
    probe.remove();
    return window.innerWidth - room;
  });
  if (gutter > 0) await page.setViewportSize({ width: 360 + gutter, height: 800 });
  await openHelp();
  const narrow = await page.evaluate(({ shortcutLabel, note }) => {
    const control = document.querySelector(shortcutLabel);
    const sentence = document.querySelector(note);
    const box = control ? control.getBoundingClientRect() : null;
    return {
      box: box ? { left: box.left, right: box.right, width: box.width, height: box.height } : null,
      noteText: sentence ? sentence.textContent.trim() : '',
      noteVisible: sentence ? getComputedStyle(sentence).display !== 'none' : false,
    };
  }, { shortcutLabel: SHORTCUT_LABEL, note: '.y-kbdpref__note' });
  if (
    !narrow.box
    || narrow.box.width <= 0
    || narrow.box.height <= 0
    || narrow.box.left < 0
    || narrow.box.right > 360
    || !narrow.noteVisible
    || !narrow.noteText.includes('/')
    || !narrow.noteText.includes('[')
  ) {
    fail('shortcut-control-reachable', `360px help panel: control=${JSON.stringify(narrow.box)}, noteVisible=${narrow.noteVisible}, note=${JSON.stringify(narrow.noteText)}`);
  }

  console.log('PASS shortcut-contract: single-key preference is default-on, exact, persisted, reversible, and reachable in the keyboard help layer with the keys it governs named; other keys remain available');
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
