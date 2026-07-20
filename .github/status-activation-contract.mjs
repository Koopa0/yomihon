// Browser contract for D27's activation boundary. A preceding pointer or
// keyboard hold owns (and suppresses) its native click, while activation with
// no such ceremony — including HTMLElement.click() and assistive technology —
// keeps the ordinary submit-button behavior.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a seal form), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'programmatic-activation-submits',
  'programmatic-activation-during-hold-submits',
  'pointer-ceremony-suppresses-click',
  'keyboard-ceremony-suppresses-click',
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
  if (!SITES.includes(site)) {
    throw new ProbeBroken(`BROKEN status-activation-contract: unknown assertion site ${site}`);
  }
  throw new LockFired(site, `FAIL status-activation-contract: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN status-activation-contract: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED status-activation-contract: ${message}`);
};

const rewriteStatusScript = (needle, replacement) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/status.js', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const count = original.split(needle).length - 1;
    matches += count;
    await route.fulfill({
      response,
      body: count === 1 ? original.replace(needle, replacement) : original,
    });
  });
  return () => {
    if (requests !== 1) return `status.js was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `the mutation needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const clickGuard = `    button.addEventListener('click', (event) => {
      if (!ownsHoldClick(event, button)) return;
      event.preventDefault();
    });`;

const keyboardGuard = `    button.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      if (event.repeat) return;
      armHoldClick({ button, source: 'keyboard' });
      startHold(form);
    });`;

const trustedOwnershipGuard = `    if (!activation || activation.button !== button || !event.isTrusted) return false;`;

const MUTATIONS = {
  'cancel-programmatic-activation': {
    target: 'programmatic-activation-submits',
    apply: rewriteStatusScript(
      clickGuard,
      `    button.addEventListener('click', (event) => {
      event.preventDefault();
    });`,
    ),
  },
  'claim-programmatic-click-during-hold': {
    target: 'programmatic-activation-during-hold-submits',
    apply: rewriteStatusScript(
      trustedOwnershipGuard,
      `    if (!activation || activation.button !== button) return false;`,
    ),
  },
  'allow-pointer-click': {
    target: 'pointer-ceremony-suppresses-click',
    apply: rewriteStatusScript(clickGuard, `    button.addEventListener('click', () => {});`),
  },
  'allow-keyboard-click': {
    target: 'keyboard-ceremony-suppresses-click',
    apply: rewriteStatusScript(
      keyboardGuard,
      `    button.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' && event.key !== ' ' && event.repeat) return;
      startHold(form);
    });`,
    ),
  },
};

const mutationTargets = Object.values(MUTATIONS).map(({ target }) => target).sort();
const assertionSites = [...SITES].sort();
if (JSON.stringify(mutationTargets) !== JSON.stringify(assertionSites)) {
  broken(`mutation targets ${JSON.stringify(mutationTargets)} do not cover assertion sites ${JSON.stringify(assertionSites)}`);
}

if (MUTATE === 'list') {
  console.log(Object.keys(MUTATIONS).join('\n'));
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`status-activation-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) {
    broken(`${PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  }
  if ((await page.locator('html[data-js="on"]').count()) !== 1) {
    broken('the client runtime did not initialize');
  }

  const form = page.locator('form[data-seal][action="/status"]').first();
  const button = form.locator('button[type="submit"][data-seal-btn]');
  if ((await form.count()) !== 1 || (await button.count()) !== 1) {
    broken('the fixture exposes no native seal form and submit button');
  }

  await form.evaluate((element) => {
    element.dataset.contractSubmissions = '0';
    element.addEventListener('submit', (event) => {
      event.preventDefault();
      element.dataset.contractSubmissions = String(Number(element.dataset.contractSubmissions) + 1);
    });
  });

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  const resetSubmissions = () => form.evaluate((element) => {
    element.dataset.contractSubmissions = '0';
  });
  const submissionCount = () => form.evaluate((element) => Number(element.dataset.contractSubmissions));

  await button.evaluate((element) => element.click());
  let count = await submissionCount();
  if (count !== 1) {
    fail('programmatic-activation-submits', `HTMLElement.click() raised ${count} submit events, want 1`);
  }

  await resetSubmissions();
  await button.click();
  count = await submissionCount();
  if (count !== 0) {
    fail('pointer-ceremony-suppresses-click', `a quick pointer ceremony raised ${count} submit events, want 0`);
  }

  await resetSubmissions();
  await button.focus();
  await page.keyboard.press('Enter');
  count = await submissionCount();
  if (count !== 0) {
    fail('keyboard-ceremony-suppresses-click', `a quick Enter ceremony raised ${count} submit events, want 0`);
  }

  await resetSubmissions();
  await button.focus();
  await page.keyboard.down('Enter');
  await button.evaluate((element) => element.click());
  await page.keyboard.up('Enter');
  count = await submissionCount();
  if (count !== 1) {
    fail(
      'programmatic-activation-during-hold-submits',
      `HTMLElement.click() during a keyboard hold raised ${count} submit events, want 1`,
    );
  }

  console.log('PASS status-activation-contract: native programmatic activation submits while D27 pointer and keyboard clicks stay owned by the hold');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE && mutationApplied && error.site === MUTATIONS[MUTATE].target) {
      console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    } else if (MUTATE) {
      console.error(`no catch: ${MUTATE} targets ${MUTATIONS[MUTATE].target}, but ${error.site} fired first`);
    }
    process.exitCode = 1;
  } else {
    console.error(error);
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
