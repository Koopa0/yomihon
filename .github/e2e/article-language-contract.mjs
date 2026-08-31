// Behavior lock for the authored-language boundary. The fixture and contract
// must supply the authority; each note article carries its own exact language
// or, where nothing was declared, no lang attribute at all — inheriting the
// page's language instead of stamping one nobody chose.
//
// The document's own language is the reader's, not a constant. The run below
// therefore walks the same page twice: once as a reader who chose nothing, and
// once as one who chose English. The second pass is what tells a chrome that
// follows the reader from a chrome that merely happens to agree with the
// default — and it holds the nesting the two answers make together, which is
// where the fault would actually be: an English frame declaring English around
// a Japanese article that must still declare Japanese.
//
// Env: YOMIHON_BASE, PAGE_PATH (the L01 fixture), and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const DECLARED_PAGE = process.env.PAGE_PATH || '/notes/Writing/lessons/japanese/L01.md';
const MISSING_PAGE = '/notes/Notes/alpha.md';
const DECLARED_RAW = '/raw/Writing/lessons/japanese/L01.md';
const CONTRACT_RAW = '/raw/System/schemas/vault-schema.toml';
const MUTATE = process.env.MUTATE || '';

const SITES = [
  'fixture-declares-ja',
  'contract-declares-lang',
  'declared-shell-language',
  'declared-article-language',
  'inline-aids-chrome-language',
  'slot-machine-chrome-language',
  'slot-output-authored-language',
  'tts-chrome-language',
  'tts-authored-language',
  'missing-shell-language',
  'missing-article-language',
  'switched-shell-language',
  'switched-chrome-language',
  'switched-authored-language',
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN article-language-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL article-language-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN article-language-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED article-language-contract: ${message}`); };

const rewritePath = (path, needle, replacement, label) => async (page) => {
  let requests = 0;
  let matches = 0;
  await page.route(BASE + path, async (route) => {
    requests += 1;
    const response = await route.fetch();
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

const MUTATIONS = {
  'drop-fixture-lang': {
    target: 'fixture-declares-ja',
    apply: rewritePath(DECLARED_RAW, '\nlang: ja\n', '\n', 'L01 frontmatter language'),
  },
  'drop-contract-lang': {
    target: 'contract-declares-lang',
    apply: rewritePath(CONTRACT_RAW, ', "lang"]', ']', 'contract known-field language'),
  },
  'change-declared-shell-lang': {
    target: 'declared-shell-language',
    apply: rewritePath(DECLARED_PAGE, '<html lang="zh-Hant"', '<html lang="en"', 'declared-note shell language'),
  },
  'change-declared-article-lang': {
    target: 'declared-article-language',
    apply: rewritePath(DECLARED_PAGE, '<article class="y-article" lang="ja">', '<article class="y-article" lang="und">', 'declared note article language'),
  },
  'drop-inline-aids-chrome-lang': {
    target: 'inline-aids-chrome-language',
    apply: rewritePath(DECLARED_PAGE, '<div class="y-inlineaids" lang="zh-Hant">', '<div class="y-inlineaids">', 'inline reading aids language'),
  },
  'drop-slot-machine-chrome-lang': {
    target: 'slot-machine-chrome-language',
    apply: rewritePath(DECLARED_PAGE, '<section class="y-slotmachine" lang="zh-Hant" aria-label="句型練習">', '<section class="y-slotmachine" aria-label="句型練習">', 'slot-machine chrome language'),
  },
  'change-slot-output-authored-lang': {
    target: 'slot-output-authored-language',
    apply: rewritePath(DECLARED_PAGE, '<p class="y-slotoutput" lang="ja">', '<p class="y-slotoutput" lang="zh-Hant">', 'slot-machine Japanese output language'),
  },
  'drop-tts-chrome-lang': {
    target: 'tts-chrome-language',
    apply: rewritePath(DECLARED_PAGE, ' lang="zh-Hant" aria-label="朗讀這段日文">', ' aria-label="朗讀這段日文">', 'read-aloud control language'),
  },
  'change-tts-authored-lang': {
    target: 'tts-authored-language',
    apply: rewritePath(DECLARED_PAGE, '<div class="y-reading" lang="ja">', '<div class="y-reading" lang="zh-Hant">', 'read-aloud Japanese segment language'),
  },
  'change-missing-shell-lang': {
    target: 'missing-shell-language',
    apply: rewritePath(MISSING_PAGE, '<html lang="zh-Hant"', '<html lang="en"', 'missing-note shell language'),
  },
  'change-missing-article-lang': {
    target: 'missing-article-language',
    apply: rewritePath(MISSING_PAGE, '<article class="y-article">', '<article class="y-article" lang="ja">', 'missing note article language'),
  },
  // These three run against the second pass, where the reader chose English.
  // Each injects the shape a chrome that ignored the choice would produce.
  'switched-shell-stays-default': {
    target: 'switched-shell-language',
    on: 'switched',
    apply: rewritePath(DECLARED_PAGE, '<html lang="en"', '<html lang="zh-Hant"', 'switched shell language'),
  },
  'switched-chrome-stays-default': {
    target: 'switched-chrome-language',
    on: 'switched',
    apply: rewritePath(DECLARED_PAGE, '<div class="y-inlineaids" lang="en">', '<div class="y-inlineaids" lang="zh-Hant">', 'switched inline reading aids language'),
  },
  'switched-article-follows-chrome': {
    target: 'switched-authored-language',
    on: 'switched',
    apply: rewritePath(DECLARED_PAGE, '<article class="y-article" lang="ja">', '<article class="y-article" lang="en">', 'switched note article language'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`article-language-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`article-language-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`article-language-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
let mutationApplied = false;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const mutation = MUTATE ? MUTATIONS[MUTATE] : null;
  proof = mutation && mutation.on !== 'switched' ? await mutation.apply(page) : null;

  let response = await page.goto(BASE + DECLARED_PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${DECLARED_PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  const declaredDOM = await page.evaluate(() => ({
    shellLanguage: document.documentElement.getAttribute('lang'),
    articleLanguages: Array.from(document.querySelectorAll('article.y-article'), (article) => article.getAttribute('lang')),
    inlineAidsLanguages: Array.from(document.querySelectorAll('.y-inlineaids'), (element) => element.getAttribute('lang')),
    slotMachineLanguages: Array.from(document.querySelectorAll('.y-slotmachine'), (element) => element.getAttribute('lang')),
    slotOutputLanguages: Array.from(document.querySelectorAll('.y-slotoutput'), (element) => element.getAttribute('lang')),
    ttsLanguages: Array.from(document.querySelectorAll('.y-tts'), (element) => element.getAttribute('lang')),
    readingLanguages: Array.from(document.querySelectorAll('.y-reading'), (element) => element.getAttribute('lang')),
  }));
  const declaredRaw = await page.evaluate(async (path) => {
    const result = await fetch(path, { cache: 'no-store' });
    return { status: result.status, body: await result.text() };
  }, DECLARED_RAW);
  const contractRaw = await page.evaluate(async (path) => {
    const result = await fetch(path, { cache: 'no-store' });
    return { status: result.status, body: await result.text() };
  }, CONTRACT_RAW);

  response = await page.goto(BASE + MISSING_PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${MISSING_PAGE} returned ${response?.status() ?? 'no response'}, want 200`);
  const missingDOM = await page.evaluate(() => ({
    shellLanguage: document.documentElement.getAttribute('lang'),
    articleLanguages: Array.from(document.querySelectorAll('article.y-article'), (article) => article.getAttribute('lang')),
  }));

  // The same page again, for a reader who chose English. A separate context
  // rather than a second visit: the cookie belongs to the reader, and the first
  // pass's route interceptions belong to its own page, so neither run can be
  // mistaken for the other.
  const switched = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  await switched.addCookies([{ name: 'yomihon_lang', value: 'en', url: BASE }]);
  const switchedPage = await switched.newPage();
  if (mutation && mutation.on === 'switched') proof = await mutation.apply(switchedPage);
  response = await switchedPage.goto(BASE + DECLARED_PAGE, { waitUntil: 'domcontentloaded' });
  if (!response || response.status() !== 200) broken(`${DECLARED_PAGE} in English returned ${response?.status() ?? 'no response'}, want 200`);
  const switchedDOM = await switchedPage.evaluate(() => ({
    shellLanguage: document.documentElement.getAttribute('lang'),
    articleLanguages: Array.from(document.querySelectorAll('article.y-article'), (article) => article.getAttribute('lang')),
    inlineAidsLanguages: Array.from(document.querySelectorAll('.y-inlineaids'), (element) => element.getAttribute('lang')),
    slotMachineLanguages: Array.from(document.querySelectorAll('.y-slotmachine'), (element) => element.getAttribute('lang')),
    slotOutputLanguages: Array.from(document.querySelectorAll('.y-slotoutput'), (element) => element.getAttribute('lang')),
    readingLanguages: Array.from(document.querySelectorAll('.y-reading'), (element) => element.getAttribute('lang')),
  }));

  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    mutationApplied = true;
  }

  if (declaredRaw.status !== 200) broken(`${DECLARED_RAW} returned ${declaredRaw.status}, want 200`);
  const frontmatter = declaredRaw.body.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  if (!frontmatter) fail('fixture-declares-ja', 'L01 has no complete frontmatter block');
  const languageLines = frontmatter[1].split(/\r?\n/).filter((line) => line.startsWith('lang:'));
  if (languageLines.length !== 1 || languageLines[0] !== 'lang: ja') {
    fail('fixture-declares-ja', `L01 language lines are ${JSON.stringify(languageLines)}, want exactly ["lang: ja"]`);
  }

  if (contractRaw.status !== 200) broken(`${CONTRACT_RAW} returned ${contractRaw.status}, want 200`);
  const knownLines = contractRaw.body.split(/\r?\n/).filter((line) => line.startsWith('known = '));
  const knownFields = knownLines.length === 1 ? Array.from(knownLines[0].matchAll(/"([^"]+)"/g), (match) => match[1]) : [];
  if (knownLines.length !== 1 || knownFields.filter((field) => field === 'lang').length !== 1) {
    fail('contract-declares-lang', `contract known fields are ${JSON.stringify(knownFields)}, want lang exactly once`);
  }

  if (declaredDOM.shellLanguage !== 'zh-Hant') {
    fail('declared-shell-language', `L01 document lang is ${JSON.stringify(declaredDOM.shellLanguage)}, want "zh-Hant"`);
  }
  if (declaredDOM.articleLanguages.length !== 1 || declaredDOM.articleLanguages[0] !== 'ja') {
    fail('declared-article-language', `L01 article langs are ${JSON.stringify(declaredDOM.articleLanguages)}, want exactly ["ja"]`);
  }
  if (declaredDOM.inlineAidsLanguages.length !== 1 || declaredDOM.inlineAidsLanguages[0] !== 'zh-Hant') {
    fail('inline-aids-chrome-language', `inline reading-aids langs are ${JSON.stringify(declaredDOM.inlineAidsLanguages)}, want exactly ["zh-Hant"]`);
  }
  if (declaredDOM.slotMachineLanguages.length !== 1 || declaredDOM.slotMachineLanguages[0] !== 'zh-Hant') {
    fail('slot-machine-chrome-language', `slot-machine langs are ${JSON.stringify(declaredDOM.slotMachineLanguages)}, want exactly ["zh-Hant"]`);
  }
  if (declaredDOM.slotOutputLanguages.length !== 1 || declaredDOM.slotOutputLanguages[0] !== 'ja') {
    fail('slot-output-authored-language', `slot-output langs are ${JSON.stringify(declaredDOM.slotOutputLanguages)}, want exactly ["ja"]`);
  }
  if (declaredDOM.ttsLanguages.length !== 1 || declaredDOM.ttsLanguages[0] !== 'zh-Hant') {
    fail('tts-chrome-language', `read-aloud control langs are ${JSON.stringify(declaredDOM.ttsLanguages)}, want exactly ["zh-Hant"]`);
  }
  if (declaredDOM.readingLanguages.length !== 1 || declaredDOM.readingLanguages[0] !== 'ja') {
    fail('tts-authored-language', `read-aloud segment langs are ${JSON.stringify(declaredDOM.readingLanguages)}, want exactly ["ja"]`);
  }
  if (missingDOM.shellLanguage !== 'zh-Hant') {
    fail('missing-shell-language', `note-without-lang document lang is ${JSON.stringify(missingDOM.shellLanguage)}, want "zh-Hant"`);
  }
  if (missingDOM.articleLanguages.length !== 1 || missingDOM.articleLanguages[0] !== null) {
    fail('missing-article-language', `note-without-lang article langs are ${JSON.stringify(missingDOM.articleLanguages)}, want exactly [null]: no attribute, so the page language is inherited`);
  }

  // The second pass. Everything the interface says is now English and
  // everything the author wrote is still Japanese, and the two are nested
  // inside each other: the frame declares one language and the article inside
  // it declares another, which is the only arrangement a screen reader can act
  // on. A chrome that ignored the choice would agree with the first pass here
  // and be wrong in exactly the way nobody would see.
  if (switchedDOM.shellLanguage !== 'en') {
    fail('switched-shell-language', `with the language set to English the document lang is ${JSON.stringify(switchedDOM.shellLanguage)}, want "en": the chrome did not follow the reader`);
  }
  for (const [what, langs] of [
    ['inline reading aids', switchedDOM.inlineAidsLanguages],
    ['the slot machine', switchedDOM.slotMachineLanguages],
  ]) {
    if (langs.length !== 1 || langs[0] !== 'en') {
      fail('switched-chrome-language', `with the language set to English ${what} declares ${JSON.stringify(langs)}, want exactly ["en"]: it is chrome, so it moves with the chrome`);
    }
  }
  for (const [what, langs] of [
    ['the article', switchedDOM.articleLanguages],
    ['the slot output', switchedDOM.slotOutputLanguages],
    ['the read-aloud segment', switchedDOM.readingLanguages],
  ]) {
    if (langs.length !== 1 || langs[0] !== 'ja') {
      fail('switched-authored-language', `with the language set to English ${what} declares ${JSON.stringify(langs)}, want exactly ["ja"]: the author's language is not the reader's to change`);
    }
  }

  console.log('PASS article-language-contract: article, chrome islands, read-aloud, and slot output keep their ruled language boundaries, in both languages the chrome speaks');
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
