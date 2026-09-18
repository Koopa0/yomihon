// Browser lock for the header row at the widths where it cannot hold
// everything. Five of its controls say nothing about the note being read — the
// reading choices, the text size, the keyboard explanation, the language and
// the theme — and below the width the stylesheet names they gather behind one
// button that opens a popover. Above that width the box around them dissolves
// and they are the row's own items again.
//
// What is asked here is what a reader can see and reach, measured rather than
// read off the stylesheet: at each width the row fits inside the window, the
// wordmark keeps all of its letters, the five are either all in the row or all
// behind the button and never half of each, and each of them exists exactly
// once in the document — a control that had been copied into the panel would
// pass every visibility question and still be two controls with one name. Then
// the panel is opened and the same names come back out of it, and it answers
// the keyboard and a click beside it the way the browser's own popover does.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/';
const MUTATE = process.env.MUTATE || '';

// The width the stylesheet folds at, and the widths the row is asked about.
// FOLD is the first width that holds the whole row; one pixel under it the
// five are behind the button. Naming both sides is what makes a breakpoint
// that has moved in either direction show up here.
const FOLD = 938;
const WIDE = [1280, FOLD];
const NARROW = [FOLD - 1, 720, 521, 390, 375];

// Each folded control, named by the hook that survives a restyling: the class
// the stylesheet already dresses, or the attribute the runtime already finds.
const FOLDED = [
  '.y-prefslink',
  '[data-textsize-toggle]',
  '[popovertarget="kbd-help"]',
  '.y-langbtn',
  '[data-theme-toggle]',
];
const FOLD_BUTTON = '[popovertarget="header-fold"]';
const PANEL = '.y-headerfold';

const SITES = [
  'row-unfolded-above',
  'row-folded-below',
  'row-fits',
  'wordmark-whole',
  'moved-not-copied',
  'names-intact',
  'panel-fits',
  'light-dismiss',
  'keyboard-reaches-and-closes',
  'open-panel-keeps-its-button',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN header-fold: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL header-fold: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN header-fold: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED header-fold: ${message}`); };

// A rule injected at a selector that matches nothing styles nothing, and a
// mutation that styled nothing would be reported as caught by whatever the
// page was already doing.
const injectRule = async (page, selector, declarations) => {
  if (await page.locator(selector).count() === 0) {
    notApplied(`no element matches ${selector}, so the injected rule styles nothing`);
  }
  await page.addStyleTag({ content: `${selector} { ${declarations} }` });
};

const changeOne = async (page, selector, change) => {
  const changed = await page.evaluate(({ selector: chosen, change: operation }) => {
    const elements = [...document.querySelectorAll(chosen)];
    if (elements.length !== 1) return elements.length;
    const element = elements[0];
    if (operation.kind === 'attribute') element.setAttribute(operation.name, operation.value);
    if (operation.kind === 'lift') {
      // Out of the panel and into the row, where the fold exists to stop it
      // being at these widths.
      const panel = element.closest('[popover]');
      panel.parentElement.insertBefore(element, panel);
    }
    if (operation.kind === 'copy') {
      // A second control with the same name and the same hooks: what folding
      // by duplication rather than by moving would leave behind.
      const panel = element.closest('[popover]');
      panel.parentElement.insertBefore(element.cloneNode(true), panel);
    }
    return 1;
  }, { selector, change });
  if (changed !== 1) notApplied(`${selector} matched ${changed} elements, want exactly 1`);
};

const MUTATIONS = {
  // The fold reaches up past every width a desk window is, so the wide row is
  // folded too.
  'fold-above-the-measured-width': {
    target: 'row-unfolded-above',
    apply: async (page) => {
      if (await page.locator(FOLD_BUTTON).count() !== 1) notApplied('there is no fold button for the moved breakpoint to show');
      await page.addStyleTag({
        content: `@media (max-width: 1400px) { ${FOLD_BUTTON} { display: inline-flex; } ${PANEL}, ${PANEL}:popover-open { display: none; } }`,
      });
    },
  },
  // One control stays out on the row below the fold width, which is the shape
  // a fold half-applied takes.
  'leave-one-control-in-the-row': {
    target: 'row-folded-below',
    apply: (page) => changeOne(page, '.y-prefslink', { kind: 'lift' }),
  },
  // The row wants more width than the window has.
  'overflow-the-row': {
    target: 'row-fits',
    apply: (page) => injectRule(page, '.y-header', 'min-width: 700px;'),
  },
  // A control grows and the wordmark pays for it, which is the cost the fold
  // exists to stop the reader paying.
  'let-a-control-take-the-name': {
    target: 'wordmark-whole',
    apply: (page) => injectRule(page, '.y-rubybtn', 'min-width: 200px;'),
  },
  // The panel is filled with copies instead of the controls themselves.
  'copy-a-control-into-the-row': {
    target: 'moved-not-copied',
    apply: (page) => changeOne(page, '[data-theme-toggle]', { kind: 'copy' }),
  },
  // The control is renamed on its way into the panel.
  'rename-a-control-in-the-panel': {
    target: 'names-intact',
    phase: 'after-row-names',
    apply: (page) => changeOne(page, '[data-theme-toggle]', { kind: 'attribute', name: 'aria-label', value: 'theme' }),
  },
  // The panel is wider than the window it opens over.
  'widen-the-panel-past-the-window': {
    target: 'panel-fits',
    // Guarded on the panel itself rather than on its open state: nothing has
    // opened it yet, so asking for the open state here would call a rule that
    // styles the right element unapplied.
    apply: async (page) => {
      if (await page.locator(PANEL).count() !== 1) notApplied('there is no panel for the injected width to widen');
      await page.addStyleTag({ content: `${PANEL}:popover-open { max-width: none; width: 600px; }` });
    },
  },
  // The browser's light dismiss is taken away, so a click beside the panel
  // leaves it open over the reading.
  'take-away-light-dismiss': {
    target: 'light-dismiss',
    apply: (page) => changeOne(page, PANEL, { kind: 'attribute', name: 'popover', value: 'manual' }),
  },
  // The button the panel hangs from is out of the tab order, so a reader on
  // the keyboard cannot open it at all.
  'take-the-button-out-of-tab-order': {
    target: 'keyboard-reaches-and-closes',
    apply: (page) => changeOne(page, FOLD_BUTTON, { kind: 'attribute', name: 'tabindex', value: '-1' }),
  },
  // The button goes away the moment the window is wide, leaving a panel open
  // over the reading with nothing pointing at it.
  'hide-the-button-under-an-open-panel': {
    target: 'open-panel-keeps-its-button',
    apply: async (page) => {
      if (await page.locator(FOLD_BUTTON).count() !== 1) notApplied('there is no fold button for the injected rule to hide');
      await page.addStyleTag({
        content: `@media (min-width: ${FOLD}px) { .y-header:has(${PANEL}:popover-open) ${FOLD_BUTTON} { display: none; } }`,
      });
    },
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`header-fold: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`header-fold: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`header-fold: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What one width looks like. The window's own width comes from an element
// pinned to the viewport rather than from the document, because a document
// narrowed by its own scrollbar reports a width the header was never given.
const readRow = (page, selectors) => page.evaluate((folded) => {
  const ruler = document.createElement('div');
  ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
  document.body.append(ruler);
  const windowWidth = ruler.getBoundingClientRect().width;
  ruler.remove();

  const header = document.querySelector('.y-header');
  const name = document.querySelector('.y-brand__name > span');
  if (!header || !name) return { issue: 'the page carries no header row or no wordmark' };
  const box = header.getBoundingClientRect();
  const inRow = (selector) => {
    const elements = [...document.querySelectorAll(selector)];
    if (elements.length !== 1) return { count: elements.length };
    const element = elements[0];
    if (!element.checkVisibility()) return { count: 1, shown: false };
    const rect = element.getBoundingClientRect();
    return {
      count: 1,
      shown: true,
      // Drawn inside the header's own band, rather than in a panel hanging
      // under it. A control in the row and a control in an open popover are
      // both "visible"; only where it is drawn tells them apart.
      inBand: rect.top >= box.top - 1 && rect.bottom <= box.bottom + 1,
    };
  };
  return {
    windowWidth,
    headerLeft: box.left,
    headerRight: box.right,
    headerOverflow: header.scrollWidth - header.clientWidth,
    wordmarkLost: name.scrollWidth - name.clientWidth,
    foldButton: inRow('[popovertarget="header-fold"]'),
    folded: Object.fromEntries(folded.map((selector) => [selector, inRow(selector)])),
  };
}, selectors);

// Every control's accessible name and pressed state, wherever it is drawn.
const readNames = (page, selectors) => page.evaluate((folded) => Object.fromEntries(folded.map((selector) => {
  const elements = [...document.querySelectorAll(selector)];
  if (elements.length !== 1) return [selector, { count: elements.length }];
  const element = elements[0];
  return [selector, {
    count: 1,
    // The name a screen reader speaks: the label when the markup gives one,
    // the visible words when it does not.
    name: (element.getAttribute('aria-label') || element.textContent || '').trim(),
    pressed: element.getAttribute('aria-pressed'),
  }];
})), selectors);

const isOpen = (page) => page.evaluate((selector) => {
  const panel = document.querySelector(selector);
  return Boolean(panel && panel.matches(':popover-open'));
}, PANEL);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const language of ['zh-Hant', 'en']) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    if (language === 'en') await context.addCookies([{ name: 'yomihon_lang', value: 'en', url: BASE }]);
    const page = await context.newPage();
    const response = await page.goto(BASE + PAGE, { waitUntil: 'load' });
    if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);
    if (await page.locator(PANEL).count() !== 1) broken('the header carries no single folded group');
    if (MUTATE && !MUTATIONS[MUTATE].phase) await MUTATIONS[MUTATE].apply(page);

    for (const width of [...WIDE, ...NARROW]) {
      await page.setViewportSize({ width, height: 800 });
      const row = await readRow(page, FOLDED);
      if (row.issue) broken(row.issue);
      const where = `${language} at ${width}px`;

      for (const [selector, seen] of Object.entries(row.folded)) {
        if (seen.count !== 1) {
          fail('moved-not-copied', `${where}: ${selector} matches ${seen.count} elements, want exactly 1 — a folded control is drawn in one place or the other, never both`);
        }
      }
      if (row.foldButton.count !== 1) {
        fail('moved-not-copied', `${where}: the button the panel hangs from matches ${row.foldButton.count} elements, want exactly 1`);
      }

      if (WIDE.includes(width)) {
        const missing = Object.entries(row.folded).filter(([, seen]) => !(seen.shown && seen.inBand)).map(([selector]) => selector);
        if (missing.length > 0 || row.foldButton.shown) {
          fail('row-unfolded-above', `${where}: the row should carry every control itself; missing from the row = ${JSON.stringify(missing)}, fold button shown = ${row.foldButton.shown}`);
        }
      } else {
        const loose = Object.entries(row.folded).filter(([, seen]) => seen.shown).map(([selector]) => selector);
        if (loose.length > 0 || !(row.foldButton.shown && row.foldButton.inBand)) {
          fail('row-folded-below', `${where}: the five belong behind the button here; still drawn = ${JSON.stringify(loose)}, fold button in the row = ${JSON.stringify(row.foldButton)}`);
        }
      }

      if (row.headerLeft < -0.5 || row.headerRight > row.windowWidth + 0.5 || row.headerOverflow > 0.5) {
        fail('row-fits', `${where}: the row runs past the window — ${JSON.stringify({ left: row.headerLeft, right: row.headerRight, windowWidth: row.windowWidth, overflow: row.headerOverflow })}`);
      }
      if (row.wordmarkLost > 0) {
        fail('wordmark-whole', `${where}: the wordmark is ${row.wordmarkLost}px short of its own text, so the name of the book is paying for the row`);
      }
    }

    // The names the controls answer to while they are out on the row.
    await page.setViewportSize({ width: 1280, height: 800 });
    const onTheRow = await readNames(page, FOLDED);
    for (const [selector, seen] of Object.entries(onTheRow)) {
      if (seen.count !== 1 || !seen.name) {
        fail('names-intact', `${language}: ${selector} on the row reads ${JSON.stringify(seen)}, want exactly one control with a name`);
      }
    }
    if (MUTATE && MUTATIONS[MUTATE].phase === 'after-row-names') await MUTATIONS[MUTATE].apply(page);

    // Opened by the keyboard, because that is the reader this panel has to
    // answer to — and because a pointer press would leave the browser in
    // pointer modality, which changes what focus looks like below.
    await page.setViewportSize({ width: 375, height: 800 });
    await page.locator('.y-brand__name').focus();
    let reached = false;
    for (let press = 0; press < 12 && !reached; press += 1) {
      await page.keyboard.press('Tab');
      reached = await page.evaluate((selector) => document.activeElement?.matches(selector) ?? false, FOLD_BUTTON);
    }
    if (!reached) {
      fail('keyboard-reaches-and-closes', `${language}: twelve tabs from the wordmark never reached the button the panel hangs from`);
    }
    await page.keyboard.press('Enter');
    await page.locator(PANEL).waitFor({ state: 'visible' }).catch(() => {});
    if (!(await isOpen(page))) {
      fail('keyboard-reaches-and-closes', `${language}: Enter on the button did not open the panel`);
    }

    const inPanel = await readNames(page, FOLDED);
    for (const [selector, seen] of Object.entries(inPanel)) {
      const row = onTheRow[selector];
      if (seen.count !== 1 || seen.name !== row.name || seen.pressed !== row.pressed) {
        fail('names-intact', `${language}: ${selector} reads ${JSON.stringify(seen)} in the panel but ${JSON.stringify(row)} on the row — a control that moves keeps its words and its state`);
      }
    }

    const panel = await page.evaluate((selector) => {
      const ruler = document.createElement('div');
      ruler.style.cssText = 'position:fixed;inset:0;pointer-events:none;visibility:hidden';
      document.body.append(ruler);
      const box = ruler.getBoundingClientRect();
      ruler.remove();
      const element = document.querySelector(selector);
      const rect = element.getBoundingClientRect();
      return {
        left: rect.left,
        right: rect.right,
        top: rect.top,
        bottom: rect.bottom,
        window: { width: box.width, height: box.height },
        overflow: Math.max(element.scrollWidth - element.clientWidth, element.scrollHeight - element.clientHeight),
      };
    }, PANEL);
    if (panel.left < -0.5 || panel.right > panel.window.width + 0.5 ||
        panel.top < -0.5 || panel.bottom > panel.window.height + 0.5 || panel.overflow > 0.5) {
      fail('panel-fits', `${language}: the open panel is not wholly inside the window — ${JSON.stringify(panel)}`);
    }

    // Tab walks into the panel rather than past it: every one of the five is
    // reachable while it is open, in the order they are written.
    const order = [];
    for (let press = 0; press < FOLDED.length; press += 1) {
      await page.keyboard.press('Tab');
      order.push(await page.evaluate((selectors) => selectors.findIndex((selector) => document.activeElement?.matches(selector) ?? false), FOLDED));
    }
    if (order.join(',') !== FOLDED.map((_, index) => index).join(',')) {
      fail('keyboard-reaches-and-closes', `${language}: tabbing through the open panel reached ${JSON.stringify(order)}, want each of the five in turn (-1 is somewhere else)`);
    }

    // A press beside the panel puts it away, which is the whole reason it is
    // the browser's popover and not a box this page opens and closes itself.
    // Asked before Escape, because a panel that had lost light dismiss would
    // have lost Escape with it, and the answer here would then be read as the
    // keyboard's.
    const before = page.url();
    await page.mouse.click(6, 780);
    await page.waitForTimeout(80);
    if (await isOpen(page)) {
      fail('light-dismiss', `${language}: a press beside the panel left it open over the reading`);
    }
    if (page.url() !== before) broken(`the press beside the panel navigated to ${page.url()}`);

    await page.locator(FOLD_BUTTON).focus();
    await page.keyboard.press('Enter');
    if (!(await isOpen(page))) broken('the panel did not reopen for the Escape question');
    await page.keyboard.press('Escape');
    if (await isOpen(page)) {
      fail('keyboard-reaches-and-closes', `${language}: Escape left the panel open`);
    }
    const returned = await page.evaluate((selector) => document.activeElement?.matches(selector) ?? false, FOLD_BUTTON);
    if (!returned) {
      fail('keyboard-reaches-and-closes', `${language}: closing the panel left focus off the button it hangs from`);
    }

    // A window dragged wider while the panel is open. An open popover keeps a
    // box of its own whatever width the window reaches, so the panel is still
    // a panel — and the button it hangs from has to still be there, because a
    // panel open over the reading with nothing pointing at it is a panel the
    // reader has to guess their way out of.
    await page.locator(FOLD_BUTTON).click();
    if (!(await isOpen(page))) broken('the panel did not open for the widening question');
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.waitForTimeout(80);
    const widened = await page.evaluate((selector) => {
      const button = document.querySelector(selector);
      return { shown: Boolean(button?.checkVisibility()), open: document.querySelector('.y-headerfold').matches(':popover-open') };
    }, FOLD_BUTTON);
    if (!widened.open) broken('widening the window closed the panel by itself, so what follows asks nothing');
    if (!widened.shown) {
      fail('open-panel-keeps-its-button', `${language}: widening the window past the fold took the button away from under an open panel`);
    }
    await page.keyboard.press('Escape');

    await context.close();
  }

  console.log('PASS header-fold: below the measured width the five interface controls are behind one button and above it they are the row itself, one of each, names and states intact, the row inside the window and the wordmark whole, and the panel answers the keyboard and a press beside it');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE) {
      const { target } = MUTATIONS[MUTATE];
      if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
      else console.error(`no catch: ${MUTATE} targets ${target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else if (error instanceof ProbeBroken) {
    console.error(error.message);
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
