// Browser lock for the code face drawing the characters that were written.
// Geist Mono joins pairs such as `<-`, `->`, `=>` and `!=` into one glyph
// through its `liga` feature, which stays on unless the stylesheet turns it
// off. In a lesson that teaches Go's channel operators, a reader who retypes
// what they see types an arrow, not the two characters the language wants. The
// DOM text never changes, so no Go test can see it; what a browser resolves for
// `font-variant-ligatures` on the code elements is the property this reads.
//
// The set read is every element Preflight puts in the mono face — code, kbd,
// samp and pre — taken from the page and not from a list of the ones that
// happened to be noticed. It is split three ways because each way is a place the
// rule can be written too narrowly: a block of code, a span of code in the
// article's prose, and code quoted outside the article (the diagnostics rail and
// the notices), which a rule scoped to `.y-prose` alone would leave behind.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note that carries a fenced block, inline code
// with `<-` in its prose, and code quoted in a notice outside the prose), and
// MUTATE. MUTATE=list prints every watched regression.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';
const SITES = [
  'code-blocks-draw-their-characters',
  'inline-code-draws-its-characters',
  'code-outside-the-article-draws-its-characters',
];

// The characters the lock exists for. The page has to carry them in the prose,
// or it measures a sentence that never showed the fault.
const JOINED_PAIR = '<-';

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN code-ligatures: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL code-ligatures: ${message}`);
};
const broken = (message) => {
  throw new ProbeBroken(`BROKEN code-ligatures: ${message}`);
};
const notApplied = (message) => {
  throw new NotApplied(`NOT-APPLIED code-ligatures: ${message}`);
};

// Appends a rule to the served stylesheet, so it lands after everything the
// product's own sheet declares and stands in for a later rule that undid the
// one being locked. It reads back what the browser resolved for one element, so
// a rule that was served but outranked reports itself instead of passing as a
// regression nobody noticed.
const appendRule = (rule, read, wanted) => async (page) => {
  let seen = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    seen += 1;
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return async () => {
    if (seen === 0) return 'the stylesheet was never requested, so the rule reached no page';
    const got = await page.evaluate(read);
    if (got !== wanted) return `the page resolves ${JSON.stringify(got)}, want ${JSON.stringify(wanted)}`;
    return '';
  };
};

// Takes the declaration itself out of the served stylesheet. The needle is the
// property and value the source declares, read with whatever whitespace the
// build left, and it has to be found exactly once in each served copy: none
// means the self-test died against a rewritten sheet, and more than one means
// it would be guessing which of several sites it was meant to edit.
const removeDeclaration = (declaration) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route('**/static/app.css{,?*}', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    matches += (original.match(declaration) || []).length;
    await route.fulfill({ response, body: original.replace(declaration, '') });
  });
  return async () => {
    if (requests < 1) return 'the stylesheet was never requested, so the rewrite reached no page';
    if (matches !== requests) {
      return `the declaration matched ${matches} times over ${requests} served copies, want one each`;
    }
    return '';
  };
};

const LIGATURES_NONE = /font-variant-ligatures\s*:\s*none\s*;?/g;

const MUTATIONS = {
  // The rule gone altogether: the face is back to what a font ships with.
  // Every code element then joins its pairs, and the first place the lock
  // looks is where it is named.
  'remove-the-rule': {
    target: 'code-blocks-draw-their-characters',
    apply: removeDeclaration(LIGATURES_NONE),
  },
  // Blocks of code joined again while everything else stays as written.
  'ligate-code-blocks': {
    target: 'code-blocks-draw-their-characters',
    apply: appendRule(
      'pre,pre code{font-variant-ligatures:normal}',
      () => getComputedStyle(document.querySelector('pre')).fontVariantLigatures,
      'normal',
    ),
  },
  // The sentence is where a learner meets `<-chan int` first, and it is the one
  // place a rule written only for fenced blocks would miss.
  'ligate-inline-code': {
    target: 'inline-code-draws-its-characters',
    apply: appendRule(
      '.y-prose :not(pre)>code{font-variant-ligatures:normal}',
      () => getComputedStyle(document.querySelector('.y-prose :not(pre)>code')).fontVariantLigatures,
      'normal',
    ),
  },
  // The rule as first written, on the article's own code: what the diagnostics
  // and the notices quote is left drawing joined pairs.
  'scope-the-rule-to-the-article': {
    target: 'code-outside-the-article-draws-its-characters',
    apply: appendRule(
      ':is(code,kbd,pre,samp):not(.y-prose *){font-variant-ligatures:normal}',
      () => {
        const outside = [...document.querySelectorAll('code,kbd,pre,samp')].find((el) => !el.closest('.y-prose'));
        return outside ? getComputedStyle(outside).fontVariantLigatures : 'no element';
      },
      'normal',
    ),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`code-ligatures: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`code-ligatures: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`code-ligatures: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// Every element the code face is set on, as the page has it. A block is a pre
// and whatever it holds; inline code is what sits in the article's prose outside
// any pre; the rest is code quoted somewhere other than the article.
const readCode = (page) =>
  page.evaluate(() => {
    const describe = (el) => ({
      tag: el.tagName.toLowerCase(),
      text: (el.textContent || '').trim().slice(0, 40),
      ligatures: getComputedStyle(el).fontVariantLigatures,
    });
    const all = [...document.querySelectorAll('code, kbd, pre, samp')];
    return {
      blocks: all.filter((el) => el.closest('pre')).map(describe),
      inline: all.filter((el) => !el.closest('pre') && el.closest('.y-prose')).map(describe),
      outside: all.filter((el) => !el.closest('pre') && !el.closest('.y-prose')).map(describe),
    };
  });

const firstJoined = (elements) => elements.find((el) => el.ligatures !== 'none');
const say = (el) => `<${el.tag}> ${JSON.stringify(el.text)} computed font-variant-ligatures=${JSON.stringify(el.ligatures)}, want "none"`;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;

  const response = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);

  if (proof) {
    const issue = await proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  const code = await readCode(page);
  if (!code.blocks.some((el) => el.tag === 'pre') || !code.blocks.some((el) => el.tag === 'code')) {
    broken('the page carries no fenced block (a pre with a code in it), so no block was measured');
  }
  if (!code.inline.some((el) => el.text.includes(JOINED_PAIR))) {
    broken(`the page carries no inline code in its prose with ${JSON.stringify(JOINED_PAIR)} in it, so the sentence that shows the fault was never measured`);
  }
  if (code.outside.length === 0) {
    broken('the page quotes no code outside the article, so a rule scoped to the prose alone would pass');
  }

  const block = firstJoined(code.blocks);
  if (block) fail('code-blocks-draw-their-characters', `in a block of code, ${say(block)}`);
  const inline = firstJoined(code.inline);
  if (inline) fail('inline-code-draws-its-characters', `in the article's prose, ${say(inline)}`);
  const outside = firstJoined(code.outside);
  if (outside) fail('code-outside-the-article-draws-its-characters', `outside the article, ${say(outside)}`);

  console.log(
    `PASS code-ligatures: ${code.blocks.length} block elements, ${code.inline.length} inline in the prose and ${code.outside.length} outside the article all compute font-variant-ligatures none`,
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
