// Behavior lock for the cover a course page opens with: the course note's own
// opening under the title, and the one verb under the counts.
//
// Two things about that verb cannot be seen from Go. Which lesson it reaches is
// a link a reader follows, and it changes with the place they kept — so the run
// keeps one place outside this course and one inside it, and reads the cover
// after each. And the line the verb and the lesson's name share has to hold at
// phone width: a name cut back to nothing is the reader's only way of telling
// where they are being sent.
//
// The place is kept through the reading page's own control, and the answer that
// stored it is read rather than assumed: a submission the server refuses leaves
// the marks file as it was, and the cover would then be read for a place nobody
// kept while the run reported on the one it meant to set.
//
// This probe leaves a kept place behind, which is why probes.sh holds it among
// the probes that run last. It never depends on there being none when it starts.
//
// Env: YOMIHON_BASE, PAGE_PATH (a course page), and MUTATE. MUTATE=list prints
// every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/syllabus/Maps/branches.md';
const MUTATE = process.env.MUTATE || '';

// The fixture course, as the vault writes it: three lessons on the main line
// with a side branch hanging off the second. AWAY is a note this course does
// not list, so a place kept there says nothing about it.
const FIRST_LESSON = '/notes/Course/C01.md';
const THIRD_LESSON = '/notes/Course/C03.md';
const AWAY = '/notes/Notes/alpha.md';
// Somewhere this course does list, and neither end of it: a mutation that sends
// the reader here is inside the course and still the wrong lesson.
const WRONG_LESSON = '/notes/Course/C02.md';

const VIEWPORT = { width: 1600, height: 900 };
const PHONE = { width: 390, height: 844 };
// Far enough down the lesson to be a place rather than the top of it.
const SCROLL_TO = 400;

// A lesson name long enough to need the line it shares with the verb, used to
// measure the box rather than the fixture's own short names.
const LONG_NAME = "Bits, Bytes and the Two's Complement Representation of Negative Integers";

// A run of characters with nowhere to break, longer than any phone column: the
// shape an address or an unspaced term takes in somebody's opening paragraph.
// Stamped into the opening for the same reason the name above is stamped into
// the verb — the fixture's own words fit however badly the column behaves.
const LONG_TOKEN = 'abcdefghijklmnopqrstuvwxyz'.repeat(3);

const SITES = [
  'opening-is-printed',
  'starts-at-the-first-lesson',
  'continues-at-the-kept-lesson',
  'cover-fits-at-phone-width',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN course-cover: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL course-cover: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN course-cover: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED course-cover: ${message}`);
};

// servedCourse rewrites the course page on its way to the browser, which is how
// a regression in what the server decided is injected without touching the
// served file. The proof counts the pages it actually changed, so a rewrite
// whose needle stopped matching reports itself instead of passing.
const servedCourse = (rewrite, needle) => async (page) => {
  let rewrites = 0;
  await page.route('**/syllabus/**', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const body = rewrite(original);
    if (body !== original) rewrites += 1;
    await route.fulfill({ response, body });
  });
  return () => (rewrites >= 1 ? '' : `no served course page carried ${needle}`);
};

// sendTo rewrites the address behind the verb wearing one token, leaving the
// token and the words alone: the page goes on saying the right thing and leads
// somewhere else.
const sendTo = (token, href) => servedCourse(
  (html) => html.replaceAll(
    new RegExp(`<a[^>]*\\bdata-course-action="${token}"[^>]*>`, 'g'),
    (tag) => tag.replace(/href="[^"]*"/, `href="${href}"`),
  ),
  `a verb reading ${token}`,
);

// appendRule puts a rule outside the product layer so it outranks what it
// stands in for, without touching the served stylesheet.
const appendRule = (rule) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (seen === 0 ? 'the stylesheet was never requested, so the rule reached no page' : '');
};

const MUTATIONS = {
  // The author's own words go missing and the page opens on its counts.
  'drop-the-paragraph': {
    target: 'opening-is-printed',
    apply: servedCourse(
      (html) => html.replace(/<div class="y-cover__opening[^>]*>[\s\S]*?<\/div>/, ''),
      'an opening paragraph',
    ),
  },
  // The verb never learns the reader kept a place inside this course, so a
  // reader who is halfway through is invited to start again.
  'always-start': {
    target: 'continues-at-the-kept-lesson',
    apply: servedCourse(
      (html) => html.replaceAll('data-course-action="continue"', 'data-course-action="start"'),
      'a verb reading continue',
    ),
  },
  // The verb says it goes back to where the reader was and goes somewhere else
  // in the same course, which is the failure a reader cannot see coming.
  'land-on-the-wrong-lesson': {
    target: 'continues-at-the-kept-lesson',
    apply: sendTo('continue', WRONG_LESSON),
  },
  // The same fault on the other side: the course starts at a lesson that is not
  // its first.
  'start-at-the-wrong-lesson': {
    target: 'starts-at-the-first-lesson',
    apply: sendTo('start', WRONG_LESSON),
  },
  // The page keeps the two states apart in the markup and says one word on
  // both, so a reader is never told which of the two the verb is doing.
  'say-the-same-word': {
    target: 'continues-at-the-kept-lesson',
    apply: servedCourse(
      (html) => html.replaceAll(/(<span class="y-cover__verb">)[^<]*(<\/span>)/g, '$1讀$2'),
      'a verb with words in it',
    ),
  },
  // The verb goes back into the middle of the course and stops naming the
  // lesson it goes to, which is the one thing adjacency cannot supply there.
  'drop-the-lesson-name': {
    target: 'continues-at-the-kept-lesson',
    apply: servedCourse(
      (html) => html.replaceAll(/<span class="y-cover__lesson">[^<]*<\/span>/g, ''),
      'a lesson name beside the verb',
    ),
  },
  // The verb and the name beside it stop sharing a line that wraps, so at phone
  // width the name runs out of the box it is drawn in.
  'nowrap-the-verb': {
    target: 'cover-fits-at-phone-width',
    apply: appendRule('.y-cover__open{flex-wrap:nowrap}.y-cover__lesson{overflow-wrap:normal;white-space:nowrap}'),
  },
  // The opening loses the rule that breaks a run of characters with nowhere to
  // break, so an address or a term written without spaces runs out of the
  // column at the width where it has the least of it.
  'keep-the-opening-unbroken': {
    target: 'cover-fits-at-phone-width',
    apply: appendRule('.y-cover__opening{overflow-wrap:normal}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`course-cover: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`course-cover: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`course-cover: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// keepThePlace stops partway down a lesson and presses the reader's own
// control, then reads the answer that stored it. Anything but the server's
// no-content answer means no place was kept, and everything below would be
// measuring the one that was there before.
const keepThePlace = async (page, path) => {
  await page.goto(BASE + path, { waitUntil: 'domcontentloaded' });
  const control = page.locator('[data-mark-control]:visible');
  if (await control.count() !== 1) {
    broken(`${path} shows ${await control.count()} mark controls at this width, want exactly 1`);
  }
  await page.evaluate((y) => window.scrollTo(0, y), SCROLL_TO);
  const stored = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/marks',
    { timeout: 4000 },
  ).catch(() => null);
  await control.locator('[data-mark-button]').click();
  const response = await stored;
  if (!response) broken(`pressing the control on ${path} sent nothing to /marks`);
  if (response.status() !== 204) {
    broken(`keeping a place in ${path} answered ${response.status()}, so no place was kept`);
  }
  await control.locator('[data-mark-said]:not(:empty)').waitFor({ state: 'attached', timeout: 4000 });
};

// readCover is what the page says about itself: the opening, the verb's own
// machine word, where it leads, and the words a reader sees on it.
const readCover = (page) => page.evaluate(() => {
  const opening = document.querySelector('.y-cover__opening');
  const action = document.querySelector('[data-course-action]');
  const lesson = action ? action.querySelector('.y-cover__lesson') : null;
  return {
    opening: opening ? opening.textContent.trim() : null,
    token: action ? action.dataset.courseAction : null,
    href: action ? action.getAttribute('href') : null,
    verb: action ? (action.querySelector('.y-cover__verb')?.textContent.trim() ?? '') : null,
    lesson: lesson ? lesson.textContent.trim() : '',
    lessons: [...document.querySelectorAll('a.y-lesson')].map((row) => row.getAttribute('href')),
  };
});

// leadsTo compares an address the page wrote against a note path. The verb
// carries the place inside the note as well — an anchor and the distance below
// it — so what is compared is which note it opens, not the whole address.
const leadsTo = (href, path) => href !== null && decodeURIComponent(new URL(href, BASE).pathname) === path;

const openCourse = async (page) => {
  await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  return readCover(page);
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let away = null;
let kept = null;
try {
  const context = await browser.newContext({ viewport: VIEWPORT });
  const page = await context.newPage();
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  // A place outside this course first, so the reading below is of a course that
  // has nowhere of its own to send anyone back to — whatever the marks file
  // held when this run started.
  await keepThePlace(page, AWAY);
  away = await openCourse(page);

  await keepThePlace(page, THIRD_LESSON);
  kept = await openCourse(page);

  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  // The fixture has to be the course these sentences are about. A run against
  // one that no longer lists these lessons, or that opens on the third, would
  // pass without ever telling the two verbs apart.
  if (away.lessons.length === 0) broken(`${PAGE} draws no lesson rows at all`);
  for (const lesson of [FIRST_LESSON, THIRD_LESSON, WRONG_LESSON]) {
    if (!away.lessons.some((href) => leadsTo(href, lesson))) {
      broken(`${PAGE} no longer lists ${lesson}, so what the verb reaches cannot be judged against it`);
    }
  }
  if (leadsTo(away.lessons[0], THIRD_LESSON)) {
    broken(`${PAGE} opens on ${THIRD_LESSON}, so starting and continuing lead to the same place`);
  }

  if (!away.opening) {
    fail('opening-is-printed', `${PAGE} prints nothing of what its own note says the course is`);
  }

  if (away.token !== 'start' || !leadsTo(away.href, FIRST_LESSON)) {
    fail(
      'starts-at-the-first-lesson',
      `with no place kept in this course the verb reads ${JSON.stringify(away.token)} and leads to `
      + `${JSON.stringify(away.href)}, want start and ${FIRST_LESSON}`,
    );
  }

  if (kept.token !== 'continue' || !leadsTo(kept.href, THIRD_LESSON)) {
    fail(
      'continues-at-the-kept-lesson',
      `after a place was kept in ${THIRD_LESSON} the verb reads ${JSON.stringify(kept.token)} and leads to `
      + `${JSON.stringify(kept.href)}, want continue and that lesson`,
    );
  }
  if (kept.verb === away.verb) {
    fail(
      'continues-at-the-kept-lesson',
      `the verb reads ${JSON.stringify(kept.verb)} whether or not a place was kept, so the reader is not told`
      + ' which of the two it is doing',
    );
  }
  if (!away.verb || !kept.verb) {
    broken('the verb carries no words, so comparing what it says in the two states proves nothing');
  }
  if (kept.lesson === '') {
    fail(
      'continues-at-the-kept-lesson',
      'the verb goes back into the middle of the course without naming the lesson it goes to',
    );
  }

  // The line the verb and the name share, at the width where it has the least
  // of it. The name is grown on the row rather than taken from the fixture:
  // the fixture's names are three characters long and would fit however badly
  // the row behaved.
  const narrow = await browser.newContext({ viewport: PHONE });
  const small = await narrow.newPage();
  if (MUTATE) await MUTATIONS[MUTATE].apply(small);
  await small.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  await small.evaluate(() => document.fonts.ready);
  const box = await small.evaluate(({ name, token }) => {
    const action = document.querySelector('[data-course-action]');
    if (!action) return null;
    const lesson = action.querySelector('.y-cover__lesson');
    if (lesson) lesson.textContent = name;
    const unbroken = document.querySelector('.y-cover__opening > *');
    if (unbroken) unbroken.textContent = token;
    const main = document.querySelector('main');
    const opening = document.querySelector('.y-cover__opening');
    // The measurements below are line counts and widths in disguise, so they
    // are taken after the name is stamped rather than before.

    return {
      action: { scroll: action.scrollWidth, client: action.clientWidth },
      // Measured on the blocks the words are actually laid out in, not on the
      // box around them: a line overflowing a paragraph leaves that
      // paragraph's own border box the width it was, and a reading taken on
      // the container would report a column that fits over text that does not.
      opening: opening
        ? [...opening.children].map((block) => ({ scroll: block.scrollWidth, client: block.clientWidth }))
        : [],
      column: main ? main.clientWidth : 0,
    };
  }, { name: LONG_NAME, token: LONG_TOKEN });
  if (box === null) broken(`${PAGE} draws no verb at ${PHONE.width}px, so nothing there can be measured`);
  if (box.action.client <= 0) broken('the verb has no width, so overflow cannot be judged');
  if (box.column <= 0 || box.column > PHONE.width) {
    broken(`the reading column measures ${box.column}px at ${PHONE.width}px, so the page was never narrowed`);
  }
  if (box.action.scroll > box.action.client) {
    fail(
      'cover-fits-at-phone-width',
      `at ${PHONE.width}px the verb lays out ${box.action.scroll}px inside a ${box.action.client}px box`,
    );
  }
  if (box.opening.length === 0) {
    broken(`${PAGE} lays the opening out in no blocks at ${PHONE.width}px, so nothing of it can be measured`);
  }
  for (const block of box.opening) {
    if (block.scroll > block.client) {
      fail(
        'cover-fits-at-phone-width',
        `at ${PHONE.width}px a line of the opening lays out ${block.scroll}px inside a ${block.client}px box`,
      );
    }
  }
  await narrow.close();
  await context.close();

  console.log(
    `PASS course-cover: ${PAGE} opens with its own words, starts at ${FIRST_LESSON}, goes back to `
    + `${THIRD_LESSON} beside the name ${JSON.stringify(kept.lesson)}, and holds its line at ${PHONE.width}px`,
  );
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
