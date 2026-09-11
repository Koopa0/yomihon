// Behavior lock for the flip receipt's arrival: a short entrance fade and a
// clean address once the sentence has landed. The server mints the receipt from
// ?from=; the client drops that token so a copied URL does not suggest a replay
// the write path has already refused.
//
// Env: YOMIHON_BASE, PAGE_PATH (the writable L01 fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const RAW = '/raw/Writing/lessons/japanese/L01.md';
const MUTATE = process.env.MUTATE || '';

const SITES = ['entrance-fade', 'address-cleanup'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN flip-receipt-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL flip-receipt-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN flip-receipt-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED flip-receipt-contract: ${message}`); };

const rewriteFetched = (pattern, needle, replacement, label) => async (page) => {
  let matches = 0;
  let counted = false;
  await page.route(pattern, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    if (!counted) {
      matches = original.split(needle).length - 1;
      counted = true;
    }
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (matches !== 1) return `${label} needle matched ${matches} times, want exactly 1`;
    return '';
  };
};

const MUTATIONS = {
  'drop-entrance': {
    target: 'entrance-fade',
    apply: rewriteFetched(
      '**/app.css',
      'animation:y-flipreceipt-in',
      'animation:none',
      'flip receipt entrance animation',
    ),
  },
  'drop-address-cleanup': {
    target: 'address-cleanup',
    apply: rewriteFetched(
      '**/freshness.js',
      "  address.searchParams.delete('from');\n",
      '',
      'from query deletion',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`flip-receipt-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`flip-receipt-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`flip-receipt-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Author CSS must be in effect before the entrance declaration is read. Measuring
// as soon as the element exists can still see animation:none before /static/app.css
// applies, and a cross-document view transition can suspend painting on the
// arriving page until pagereveal — long enough for a 200ms fade to finish unseen.
const waitForAppStyles = (page) => page.waitForFunction(() =>
  [...document.styleSheets].some((sheet) => (sheet.href || '').includes('/static/app.css')),
);

const waitArrival = (page) => page.evaluate(() => new Promise((resolve) => {
  let settled = false;
  const finish = () => {
    if (settled) return;
    settled = true;
    resolve();
  };
  setTimeout(finish, 600);
  const afterPaint = () => requestAnimationFrame(() => requestAnimationFrame(finish));
  afterPaint();
  window.addEventListener('pagereveal', (event) => {
    if (event.viewTransition?.finished) {
      event.viewTransition.finished.then(afterPaint, afterPaint);
      return;
    }
    afterPaint();
  }, { once: true });
}));

const readEntranceDeclaration = (receipt) => receipt.evaluate((el) => {
  const style = getComputedStyle(el);
  return { name: style.animationName, duration: style.animationDuration };
});

// waitForFullOpacity waits until the receipt's entrance has landed at full
// strength. The declaration is checked separately because a 200ms fade cannot
// be sampled mid-flight once navigation and the selector wait have elapsed.
const waitForFullOpacity = async (receipt) => {
  try {
    return await receipt.evaluate((el) => new Promise((resolve, reject) => {
      let interval = 0;
      const deadline = setTimeout(() => {
        if (interval) clearInterval(interval);
        reject(new Error('the flip receipt never reached full opacity'));
      }, 2000);
      const tick = () => {
        if (getComputedStyle(el).opacity === '1') {
          clearTimeout(deadline);
          if (interval) clearInterval(interval);
          resolve(1);
          return true;
        }
        return false;
      };
      if (tick()) return;
      interval = setInterval(() => {
        if (tick()) clearInterval(interval);
      }, 16);
    }));
  } catch {
    return receipt.evaluate((el) => Number(getComputedStyle(el).opacity));
  }
};

const injectReceipt = async (page) => {
  const arrival = `${BASE}${PAGE}?from=draft`;
  await page.route(`${BASE}${PAGE}*`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    const response = await route.fetch();
    const original = await response.text();
    const receipt = '<p class="y-flipreceipt" role="status">fixture receipt</p>';
    const body = original.includes('class="y-flipreceipt"')
      ? original
      : original.replace('<main id="main-content"', `${receipt}<main id="main-content"`);
    await route.fulfill({ response, body });
  });
  const noteResponse = await page.goto(arrival, { waitUntil: 'domcontentloaded' });
  if (!noteResponse || noteResponse.status() !== 200) broken(`${arrival} returned ${noteResponse?.status() ?? 'no response'}, want 200`);
  await page.waitForSelector('html[data-js]');
  await page.waitForSelector('.y-flipreceipt', { timeout: 3000 });
  await page.waitForLoadState('load');
};

const flipToReceipt = async (page) => {
  const noteResponse = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!noteResponse || noteResponse.status() !== 200) broken(`${PAGE} returned ${noteResponse?.status() ?? 'no response'}, want 200`);
  await page.waitForSelector('html[data-js]');

  const form = page.locator('form.y-statusform')
    .filter({ has: page.locator('input[name="to"][value="ready"]') })
    .first();
  if (await form.count() !== 1) broken('the L01 fixture exposes no draft-to-ready status form');
  const [postResponse] = await Promise.all([
    page.waitForResponse((response) => response.url().includes('/status') && response.request().method() === 'POST'),
    form.evaluate((element) => element.requestSubmit()),
  ]);
  if (!postResponse) broken('the status flip produced no status response');
  if (postResponse.status() !== 303) {
    broken(`status flip returned ${postResponse.status()}, want 303`);
  }
  const location = postResponse.headers().location ?? '';
  if (!location.includes('from=draft')) {
    broken(`status flip Location is ${JSON.stringify(location)}, want a ?from=draft arrival address`);
  }
  await page.waitForSelector('.y-flipreceipt', { timeout: 3000 });
  await page.waitForLoadState('load');
  await page.waitForSelector('html[data-js]');
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const flipped = MUTATE !== 'drop-address-cleanup';
  if (MUTATE === 'drop-address-cleanup') await injectReceipt(page);
  else await flipToReceipt(page);

  const receipt = page.locator('.y-flipreceipt');
  if (await receipt.count() !== 1) broken(`the arrival page carries ${await receipt.count()} flip receipts, want 1`);
  await waitForAppStyles(page);
  await waitArrival(page);

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const entrance = await readEntranceDeclaration(receipt);
  if (entrance.name !== 'y-flipreceipt-in') {
    fail('entrance-fade', `entrance animation is ${JSON.stringify(entrance.name)}, want "y-flipreceipt-in"`);
  }
  if (parseFloat(entrance.duration) <= 0) {
    fail('entrance-fade', `entrance duration is ${JSON.stringify(entrance.duration)}, want a non-zero duration`);
  }
  const opacity = await waitForFullOpacity(receipt);
  if (opacity !== 1) {
    fail('entrance-fade', `the receipt never reached full opacity: sampled opacity ${opacity}`);
  }

  await page.waitForFunction(() => !new URL(location.href).searchParams.has('from'), null, { timeout: 2000 }).catch(() => {});
  const address = new URL(page.url());
  if (address.searchParams.has('from')) {
    fail('address-cleanup', `the address still carries from=${JSON.stringify(address.searchParams.get('from'))}`);
  }
  if (address.pathname !== PAGE) {
    fail('address-cleanup', `cleanup changed the path to ${address.pathname}, want ${PAGE}`);
  }

  if (flipped) {
    const rawAfter = await page.evaluate(async (path) => {
      const result = await fetch(path, { cache: 'no-store' });
      return { status: result.status, body: await result.text() };
    }, RAW);
    if (rawAfter.status !== 200) broken(`${RAW} returned ${rawAfter.status} after the flip, want 200`);
    if (!rawAfter.body.includes('status: ready')) {
      broken('the fixture note did not flip to ready');
    }
  }

  console.log(`PASS flip-receipt-contract: entrance ${entrance.name} (${entrance.duration}) at opacity ${opacity} and address ${address.pathname}`);
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
      else console.error(`no catch: ${MUTATE} injects a regression the ${target} assertion watches for, but the ${err.site} assertion fired first`);
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
