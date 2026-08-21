// Behavior lock for an incomplete native status submission. The failed POST
// must navigate to a truthful same-shell recovery page without changing the
// fixture note or exposing any way to repeat the mutation.
//
// Env: YOMIHON_BASE, PAGE_PATH (the writable L01 fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const RAW = '/raw/Writing/lessons/japanese/L01.md';
const STATUS = '/status';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'incomplete-post-response',
  'same-shell-recovery',
  'unchanged-state',
  'safe-get-navigation',
  'no-post-retry',
  'no-store-response',
  'focusable-main',
  'fixture-bytes-unchanged',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN status-recovery-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL status-recovery-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN status-recovery-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED status-recovery-contract: ${message}`); };

// route.fetch issues the intercepted request from Playwright's transport
// rather than from the page. Preserve the same-origin browser context of the
// native form submission so response mutations still cross the production
// CrossOriginProtection middleware instead of being replaced by its 403.
const fetchSameOrigin = (route) => route.fetch({
  headers: {
    ...route.request().headers(),
    origin: BASE,
    'sec-fetch-site': 'same-origin',
  },
});

const rewriteStatusBody = (needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + STATUS, async (route) => {
    requests += 1;
    const response = await fetchSameOrigin(route);
    const original = await response.text();
    matches += original.split(needle).length - 1;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (requests !== 1) return `${label} response was requested ${requests} times, want exactly 1`;
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const changeStatusCode = async (page) => {
  let requests = 0;
  let originalStatus = null;
  await page.route(BASE + STATUS, async (route) => {
    requests += 1;
    const response = await fetchSameOrigin(route);
    originalStatus = response.status();
    await route.fulfill({ response, status: 200 });
  });
  return () => {
    if (requests !== 1) return `status-code response was requested ${requests} times, want exactly 1`;
    if (originalStatus !== 422) return `status-code mutation saw ${originalStatus}, want the original 422`;
    return '';
  };
};

const changeCacheHeader = async (page) => {
  let requests = 0;
  let originalHeader = null;
  await page.route(BASE + STATUS, async (route) => {
    requests += 1;
    const response = await fetchSameOrigin(route);
    originalHeader = response.headers()['cache-control'] ?? null;
    const headers = { ...response.headers() };
    headers['cache-control'] = 'public, max-age=3600';
    await route.fulfill({ response, headers });
  });
  return () => {
    if (requests !== 1) return `cache-header response was requested ${requests} times, want exactly 1`;
    if (originalHeader !== 'no-store') return `cache-header mutation saw ${JSON.stringify(originalHeader)}, want the original "no-store"`;
    return '';
  };
};

const changeSecondRawRead = async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + RAW, async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    if (requests === 2) {
      const needle = 'status: draft';
      matches = original.split(needle).length - 1;
      await route.fulfill({ response, body: original.replace(needle, 'status: mutated') });
      return;
    }
    await route.fulfill({ response, body: original });
  });
  return () => {
    if (requests !== 2) return `fixture raw route was requested ${requests} times, want exactly 2`;
    if (matches !== 1) return `fixture status needle matched ${matches} times on the second read, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'change-recovery-status': {
    target: 'incomplete-post-response',
    apply: changeStatusCode,
  },
  'drop-recovery-shell': {
    target: 'same-shell-recovery',
    apply: rewriteStatusBody('<div class="y-shell2">', '<div class="y-shell-broken">', 'recovery shell'),
  },
  'claim-status-changed': {
    target: 'unchanged-state',
    apply: rewriteStatusBody('data-status-changed="false"', 'data-status-changed="true"', 'unchanged-state marker'),
  },
  'replace-note-recovery-link': {
    target: 'safe-get-navigation',
    apply: rewriteStatusBody(
      `<a class="y-recovery__action" href="${PAGE}">返回筆記</a>`,
      '<a class="y-recovery__action" href="/status">返回筆記</a>',
      'safe note recovery link',
    ),
  },
  'inject-post-retry': {
    target: 'no-post-retry',
    apply: rewriteStatusBody(
      '<a class="y-recovery__action y-recovery__action--quiet" href="/">返回首頁</a></nav>',
      '<a class="y-recovery__action y-recovery__action--quiet" href="/">返回首頁</a><form method="post" action="/status"><button type="submit">retry</button></form></nav>',
      'POST retry control',
    ),
  },
  'make-recovery-cacheable': {
    target: 'no-store-response',
    apply: changeCacheHeader,
  },
  'drop-main-tabindex': {
    target: 'focusable-main',
    apply: rewriteStatusBody(
      '<main id="main-content" tabindex="-1" class="y-main y-recoverymain">',
      '<main id="main-content" class="y-main y-recoverymain">',
      'focusable recovery main',
    ),
  },
  'alter-second-vault-read': {
    target: 'fixture-bytes-unchanged',
    apply: changeSecondRawRead,
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`status-recovery-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`status-recovery-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`status-recovery-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const noteResponse = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!noteResponse || noteResponse.status() !== 200) broken(`${PAGE} returned ${noteResponse?.status() ?? 'no response'}, want 200`);
  const rawBefore = await page.evaluate(async (path) => {
    const result = await fetch(path, { cache: 'no-store' });
    return { status: result.status, body: await result.text() };
  }, RAW);
  if (rawBefore.status !== 200) broken(`${RAW} returned ${rawBefore.status} before the POST, want 200`);

  const form = page.locator('form[action="/status"]').first();
  if (await form.count() !== 1) broken('the L01 fixture exposes no native status form');
  const formState = await form.evaluate((element) => ({
    path: Array.from(element.querySelectorAll('input[name="path"]'), (input) => input.value),
    from: Array.from(element.querySelectorAll('input[name="from"]'), (input) => input.value),
    to: Array.from(element.querySelectorAll('input[name="to"]'), (input) => input.value),
  }));
  if (JSON.stringify(formState.path) !== JSON.stringify(['Writing/lessons/japanese/L01.md']) ||
      JSON.stringify(formState.from) !== JSON.stringify(['draft']) ||
      formState.to.length !== 1 || formState.to[0] === '') {
    broken(`the fixture status form fields are ${JSON.stringify(formState)}, want one path/from/to for L01 draft`);
  }
  await form.locator('input[name="to"]').evaluate((input) => input.remove());
  const [postResponse] = await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    form.evaluate((element) => element.requestSubmit()),
  ]);
  if (!postResponse) broken('the incomplete native form submission produced no navigation response');

  const rawAfter = await page.evaluate(async (path) => {
    const result = await fetch(path, { cache: 'no-store' });
    return { status: result.status, body: await result.text() };
  }, RAW);
  if (rawAfter.status !== 200) broken(`${RAW} returned ${rawAfter.status} after the POST, want 200`);

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  if (postResponse.request().method() !== 'POST' || postResponse.url() !== BASE + STATUS || page.url() !== BASE + STATUS || postResponse.status() !== 422) {
    fail('incomplete-post-response', `navigation was ${postResponse.request().method()} ${postResponse.url()} -> ${postResponse.status()} at ${page.url()}, want POST ${BASE + STATUS} -> 422`);
  }

  const shell = await page.evaluate(() => ({
    language: document.documentElement.getAttribute('lang'),
    root: document.querySelectorAll('body > .yomihon').length,
    header: document.querySelectorAll('body > .yomihon > header.y-header').length,
    shell: document.querySelectorAll('body > .yomihon > .y-shell2').length,
    sidebar: document.querySelectorAll('body > .yomihon > .y-shell2 > aside.y-rail-left').length,
    main: document.querySelectorAll('body > .yomihon > .y-shell2 > main.y-recoverymain').length,
  }));
  if (shell.language !== 'zh-Hant' || shell.root !== 1 || shell.header !== 1 || shell.shell !== 1 || shell.sidebar !== 1 || shell.main !== 1) {
    fail('same-shell-recovery', `recovery shell shape is ${JSON.stringify(shell)}, want one zh-Hant shared header/sidebar/main shell`);
  }

  const recoveryState = await page.locator('article.y-recovery').evaluate((article) => ({
    changed: article.querySelector('[data-status-changed]')?.getAttribute('data-status-changed'),
    title: article.querySelector('#recovery-title')?.textContent,
    state: article.querySelector('.y-recovery__state')?.textContent,
    summary: article.querySelector('.y-recovery__summary')?.textContent,
  }));
  if (recoveryState.changed !== 'false' || recoveryState.title !== '狀態尚未變更' || recoveryState.state !== '這次操作沒有變更筆記檔案。' || recoveryState.summary !== 'path、from 與 to 都是必填欄位。') {
    fail('unchanged-state', `recovery state is ${JSON.stringify(recoveryState)}, want the exact unchanged missing-field state`);
  }

  const recoveryActions = await page.locator('nav.y-recovery__actions').evaluate((nav) => Array.from(nav.querySelectorAll(':scope > a'), (child) => ({
    tag: child.tagName,
    href: child.getAttribute('href'),
  })));
  // The middle action opens the note in Obsidian; its href carries the
  // serving machine's absolute vault path, so only its shape is pinned here.
  if (
    recoveryActions.length !== 3
    || recoveryActions[0].tag !== 'A' || recoveryActions[0].href !== PAGE
    || recoveryActions[1].tag !== 'A'
    || !recoveryActions[1].href.startsWith('obsidian://open?path=')
    || !recoveryActions[1].href.endsWith('/Writing/lessons/japanese/L01.md')
    || recoveryActions[2].tag !== 'A' || recoveryActions[2].href !== '/'
  ) {
    fail('safe-get-navigation', `recovery actions are ${JSON.stringify(recoveryActions)}, want the note link, an obsidian://open link, and home`);
  }

  const retryControls = await page.locator('form[method="post"], form[action="/status"], button[formaction="/status"]').count();
  if (retryControls !== 0) fail('no-post-retry', `recovery page exposes ${retryControls} POST controls, want 0`);

  const cacheControl = postResponse.headers()['cache-control'] ?? null;
  if (cacheControl !== 'no-store') {
    fail('no-store-response', `recovery Cache-Control is ${JSON.stringify(cacheControl)}, want "no-store"`);
  }

  const main = page.locator('main#main-content[tabindex="-1"]');
  if (await main.count() !== 1) fail('focusable-main', `recovery page has ${await main.count()} focusable main landmarks, want 1`);
  await main.focus();
  if (!await main.evaluate((element) => document.activeElement === element)) {
    fail('focusable-main', 'main has tabindex=-1 but did not receive programmatic focus');
  }

  if (rawAfter.body !== rawBefore.body) {
    fail('fixture-bytes-unchanged', 'the fixture raw bytes differ after the deliberately incomplete POST');
  }

  console.log('PASS status-recovery-contract: incomplete POST stays unchanged, navigates to no-store same-shell recovery, keeps safe GET exits, and offers no retry');
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    if (MUTATE && !mutationApplied) {
      console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
      process.exitCode = 2;
    } else {
      if (MUTATE) {
        const { target } = MUTATIONS[MUTATE];
        if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
        else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
      }
      process.exitCode = 1;
    }
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
