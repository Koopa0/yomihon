// Behavior lock for the foot of the left rail: which folder the reader is in,
// how large it is, and what the health page has to say about it. The rail used
// to end where its tree ended, and all three were three pages away.
//
// Four things only a browser can answer. That the foot is drawn at reading
// width. That the same element, which the stylesheet turns into the drawer at
// phone width, still carries it once the drawer is open — one element, two
// widths, so a foot that goes missing there went missing for a rule nobody
// wrote down. That the number beside the dot is the health page's own: the page
// states the complete findings total, and the foot claims the same number.
// And that a folder whose name is one long unbroken word wraps inside the rail rather than making
// the drawer scroll sideways, which is why the name is stamped long before the
// last measurement rather than measured as the fixture happens to be named.
//
// Env: YOMIHON_BASE, PAGE_PATH (any page carrying a left rail), and MUTATE.
// MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const HEALTH = '/health';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'foot-on-the-rail',
  'foot-in-the-drawer',
  'count-agrees-with-health',
  'foot-does-not-scroll-sideways',
];

// serve.sh copies the fixture to a directory of this name, so this is the word
// the foot has to be showing if it took the name from the folder it serves
// rather than from a constant.
const SERVED_FOLDER = 'vault';

// A name with nowhere to break, long enough that no rail is wide enough for it.
const UNBREAKABLE_NAME = 'a-folder-named-without-a-single-place-to-break-the-line-anywhere-at-all';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN rail-foot: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL rail-foot: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN rail-foot: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED rail-foot: ${message}`);
};

// Rewrites the page the rail is read from. The needle is checked rather than
// assumed: a mutation that matched nothing would let the run below report a
// lock holding against a regression it never saw.
const rewritePage = (transform) => async (page) => {
  let applied = false;
  await page.route(BASE + PAGE, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = transform(original);
    if (body !== original) applied = true;
    return route.fulfill({ response, body });
  });
  return async () => (applied ? '' : 'the mutation changed nothing in the page it targeted');
};

// Appends a rule to the served stylesheet so it outranks the one it stands in
// for, and reads the browser's own verdict back, so a rule that was served and
// outranked reports itself instead of passing as a regression nobody noticed.
const appendRule = (rule, selector, property, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await page.evaluate(
      ([sel, prop]) => {
        const el = document.querySelector(sel);
        return el ? getComputedStyle(el).getPropertyValue(prop) : '';
      },
      [selector, property],
    );
    if (got !== wanted) return `${selector} resolves ${property} to ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

const MUTATIONS = {
  // The rail ends where its tree ends again.
  'drop-foot': {
    target: 'foot-on-the-rail',
    apply: rewritePage((body) => body.replace(/<div class="y-railfoot"[\s\S]*?<\/a><\/div>/, '')),
  },
  // The foot survives at reading width and is taken away from the drawer,
  // which is the shape a rule written for one width and not the other has.
  'hide-foot-in-the-drawer': {
    target: 'foot-in-the-drawer',
    apply: appendRule(
      '@media (max-width: 900px){.y-railfoot{display:none}}',
      '.y-railfoot',
      'display',
      'none',
    ),
  },
  // The foot states a number of its own rather than the page's.
  'bump-findings': {
    target: 'count-agrees-with-health',
    apply: rewritePage((body) =>
      body.replace(/data-rail-foot-findings="(\d+)"/, (_, n) => `data-rail-foot-findings="${Number(n) + 7}"`),
    ),
  },
  // The folder name refuses to break, which is what the rail's own width used
  // to mean before the name was allowed to wrap anywhere.
  'name-will-not-wrap': {
    target: 'foot-does-not-scroll-sideways',
    apply: appendRule(
      '.y-railfoot__name{overflow-wrap:normal;word-break:normal;white-space:nowrap}',
      '.y-railfoot__name',
      'overflow-wrap',
      'normal',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`rail-foot: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`rail-foot: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`rail-foot: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What the foot says, read off the page rather than off the markup, so a line
// the stylesheet emptied reads as empty here too.
const readFoot = (page) =>
  page.evaluate(() => {
    const foot = document.querySelector('[data-rail-foot]');
    if (!foot) return null;
    const text = (selector) => foot.querySelector(selector)?.innerText.trim() ?? '';
    const link = foot.querySelector('.y-railfoot__health');
    return {
      inRail: Boolean(foot.closest('aside.y-rail-left')),
      last: foot.parentElement?.lastElementChild === foot,
      count: document.querySelectorAll('[data-rail-foot]').length,
      name: text('.y-railfoot__name'),
      size: text('.y-railfoot__size'),
      findings: text('.y-railfoot__count'),
      claimed: link?.getAttribute('data-rail-foot-findings') ?? '',
      href: link?.getAttribute('href') ?? '',
    };
  });

// Read the complete findings total the page states, without leaving the
// requested page or deriving a different unit from table rows or files.
// With no findings the page draws no table or shape line.
const readHealthTotal = async (page) => {
  const total = page.locator('.y-healthshape__total');
  if (await total.count() === 0 && await page.locator('.y-findings').count() === 0) return 0;
  if (await total.count() !== 1 || !await total.isVisible()) {
    fail('count-agrees-with-health', 'the health page has no single visible findings total');
  }
  const text = (await total.innerText()).trim();
  const match = /^(\d+) (?:finding|findings|項發現)$/.exec(text);
  if (!match) fail('count-agrees-with-health', `the health total has no findings unit: ${JSON.stringify(text)}`);
  return Number(match[1]);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });

  const wide = await readFoot(page);
  if (wide === null) fail('foot-on-the-rail', 'the page carrying a left rail draws no foot at 1280px');
  if (wide.count !== 1) fail('foot-on-the-rail', `the page draws ${wide.count} rail feet, want exactly 1`);
  if (!wide.inRail) fail('foot-on-the-rail', 'the foot is not inside the left rail');
  if (!wide.last) fail('foot-on-the-rail', 'the foot is not the last thing in the rail, so the rail does not end with it');
  for (const [line, value] of [['name', wide.name], ['size', wide.size], ['findings', wide.findings]]) {
    if (value === '') fail('foot-on-the-rail', `the foot's ${line} line is blank, and a blank is not a sentence`);
  }
  if (wide.href !== HEALTH) fail('foot-on-the-rail', `the foot's health line points at ${JSON.stringify(wide.href)}, want ${HEALTH}`);
  if (wide.name !== SERVED_FOLDER) {
    fail('foot-on-the-rail', `the foot calls the folder ${JSON.stringify(wide.name)}, want ${JSON.stringify(SERVED_FOLDER)} — the name of the directory serve.sh serves`);
  }
  if (!/^\d+$/.test(wide.claimed)) {
    broken(`the foot claims ${JSON.stringify(wide.claimed)} findings, which is not a number, so nothing below can be compared`);
  }

  await page.goto(BASE + HEALTH, { waitUntil: 'domcontentloaded' });
  const total = await readHealthTotal(page);
  if (Number(wide.claimed) !== total) {
    fail(
      'count-agrees-with-health',
      `the rail's foot claims ${wide.claimed} findings and the health page states ${total}`,
    );
  }
  // The number the reader sees and the number the foot states have to be the
  // same number: a count agreeing with the page while the words beside it say
  // something else is the same disagreement one step further along.
  if (total > 0 && !wide.findings.includes(String(total))) {
    fail(
      'count-agrees-with-health',
      `the foot shows ${JSON.stringify(wide.findings)} while claiming ${wide.claimed}`,
    );
  }

  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto(BASE + PAGE, { waitUntil: 'load' });
  const toggle = page.locator('[data-nav-toggle]');
  if (await toggle.count() !== 1) broken('the page has no single drawer control to open the rail with');
  if (!await toggle.isVisible()) broken('the drawer control is not shown at 390px, so there is no drawer to look into');
  await toggle.click();
  await page.waitForFunction(() => document.documentElement.dataset.nav === 'open');

  const drawer = await page.evaluate(() => {
    const foot = document.querySelector('[data-rail-foot]');
    const rail = document.querySelector('aside.y-rail-left');
    if (!foot || !rail) return null;
    const footBox = foot.getBoundingClientRect();
    const railBox = rail.getBoundingClientRect();
    return {
      shown: footBox.width > 0 && footBox.height > 0 && getComputedStyle(foot).display !== 'none',
      inside: footBox.left >= railBox.left - 1 && footBox.right <= railBox.right + 1,
      footBox: { left: footBox.left, right: footBox.right },
      railBox: { left: railBox.left, right: railBox.right },
    };
  });
  if (drawer === null) fail('foot-in-the-drawer', 'the opened drawer holds no foot');
  if (!drawer.shown) fail('foot-in-the-drawer', 'the foot is drawn away at 390px, so the drawer ends where its tree ends');
  if (!drawer.inside) {
    fail(
      'foot-in-the-drawer',
      `the foot spans ${drawer.footBox.left}–${drawer.footBox.right} while the drawer spans ${drawer.railBox.left}–${drawer.railBox.right}`,
    );
  }

  // The fixture folder's own name is short, so measuring it would say nothing
  // about a name that is not. The hard case is stamped on and then measured.
  const overflow = await page.evaluate((name) => {
    const foot = document.querySelector('[data-rail-foot]');
    const line = foot?.querySelector('.y-railfoot__name');
    if (!line) return null;
    // The name goes in through an inline span, because only an inline box
    // reports one rectangle per line it was laid out on, and how many lines it
    // took is what says the hard case was actually reached.
    const span = document.createElement('span');
    span.textContent = name;
    line.replaceChildren(span);
    const boxes = [foot, ...foot.querySelectorAll('*')];
    const over = boxes
      .filter((el) => el.scrollWidth > el.clientWidth && el.clientWidth > 0)
      .map((el) => ({ what: el.className || el.tagName, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth }));
    const rail = document.querySelector('aside.y-rail-left');
    return {
      over,
      lines: span.getClientRects().length,
      railScroll: rail ? rail.scrollWidth : 0,
      railClient: rail ? rail.clientWidth : 0,
    };
  }, UNBREAKABLE_NAME);
  if (overflow === null) broken('the foot has no name line to stamp a long folder name onto');
  if (overflow.over.length > 0) {
    const [first] = overflow.over;
    fail(
      'foot-does-not-scroll-sideways',
      `at 390px ${first.what} has scrollWidth=${first.scrollWidth} > clientWidth=${first.clientWidth}`,
    );
  }
  if (overflow.railScroll > overflow.railClient) {
    fail(
      'foot-does-not-scroll-sideways',
      `at 390px the drawer scrolls sideways: scrollWidth=${overflow.railScroll} > clientWidth=${overflow.railClient}`,
    );
  }
  // A name that fitted on one line was never the hard case, so the sentence
  // above would be true of a rail that had lost the rule entirely.
  if (overflow.lines < 2) {
    broken(`the stamped name lays out on ${overflow.lines} line at 390px, so a name that refused to wrap would measure the same`);
  }

  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  console.log(
    `PASS rail-foot: the rail ends with ${JSON.stringify(wide.name)}, ${JSON.stringify(wide.size)} and ${total} findings, at 1280px and inside the opened drawer at 390px`,
  );
} catch (err) {
  if (err instanceof NotApplied) {
    console.error(err.message);
    console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
    process.exitCode = 2;
  } else if (err instanceof LockFired) {
    console.error(err.message);
    // Whether the mutation reached the page is asked here rather than before
    // the assertions, because two of these mutations only take effect at the
    // width the assertion they aim at is made at: asked early, a rule that
    // works would report itself as never applied.
    const issue = MUTATE && proof ? await proof() : '';
    if (issue) {
      console.error(`${MUTATE}: ${issue}`);
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
