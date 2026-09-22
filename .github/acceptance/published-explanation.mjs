// CI acceptance for the public example's publication boundary. The driver
// serves real Markdown through the production binary, then proves the content
// assertions reject the old explanations in disposable copies of the vault.
// Run with the already built binary as the sole argument. Nothing submits a
// status change; a route guard aborts and reports any attempted status POST.
import { spawnSync } from 'node:child_process';
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright-core';

const ROOT = fileURLToPath(new URL('../..', import.meta.url));
const SCRIPT = fileURLToPath(import.meta.url);
const PUBLISHED = 'A published note';
const LANGUAGES = {
  en: {
    course: 'Reading yomihon', lifecycle: 'The status lifecycle', search: 'Search',
    explanation: [
      'The diagram shows transitions the contract allows, including ready → published.',
      'Yomihon never sets a note to published, even when the contract permits that target',
      'the value records a publication that happened somewhere else, and a reading surface cannot attest to one.',
      'This restriction concerns the target, not a note already marked published.',
      'Its status panel offers archived because this example contract allows published → archived',
    ],
    warning: 'After archived, this offers no way back to the current status.',
    confirm: 'Confirm archived',
  },
  'zh-Hant': {
    course: '讀懂 yomihon', lifecycle: 'status 的生命週期', search: '搜尋',
    explanation: [
      '上圖呈現契約允許的轉換，包含 ready → published。',
      'yomihon 也不會把筆記設為 published',
      '這個值記錄的是在別處發生的發表，閱讀介面無法為它作證。',
      '這項限制針對的是轉換的目標，不是已經標為 published 的筆記。',
      '它的狀態面板仍提供 archived，因為這份範例契約允許 published → archived',
    ],
    warning: '設為 archived 之後，這裡不再有回到目前狀態的路。',
    confirm: '確認設為 archived',
  },
};

const REVERTS = {
  published: {
    file: 'Notes/A published note.md',
    current: "This note carries `status: published`, written by hand. Its status panel offers `archived`, which this example contract permits from `published`. Yomihon never sets a note **to** `published`: that value records a publication outside this vault, which yomihon cannot attest to. A note already marked `published` can still offer other transitions allowed by its contract. See [[The status lifecycle]].",
    previous: "This note carries `status: published`, written by hand. Its status panel names the status and offers nothing onward: the contract allows `ready → published`, and yomihon still does not make that move, because the value records something that happened outside this vault. See [[The status lifecycle]].",
  },
  en: {
    file: 'Notes/The status lifecycle.md',
    current: "The diagram shows transitions the contract allows, including `ready → published`. Yomihon never sets a note **to** `published`, even when the contract permits that target: the value records a publication that happened somewhere else, and a reading surface cannot attest to one.\n\nThis restriction concerns the target, not a note already marked `published`. [[A published note]] carries that value, written by hand. Its status panel offers `archived` because this example contract allows `published → archived`, as the diagram shows. Other vaults offer the onward transitions their own contracts permit.",
    previous: "The contract allows `ready → published` and yomihon still does not make that move. The value records a publication that happened somewhere else, and a reading surface cannot attest to one. [[A published note]] carries it, written by hand, and its status panel offers nothing onward.",
  },
  'zh-Hant': {
    file: 'Notes/中文/status 的生命週期.md',
    current: "上圖呈現契約允許的轉換，包含 `ready → published`。即使契約允許這個目標，yomihon 也不會把筆記**設為** `published`：這個值記錄的是在別處發生的發表，閱讀介面無法為它作證。\n\n這項限制針對的是轉換的目標，不是已經標為 `published` 的筆記。[[A published note]]（英文）手寫上了這個值；它的狀態面板仍提供 `archived`，因為這份範例契約允許 `published → archived`，上圖也畫出了這條路。其他知識庫能提供哪些後續轉換，要看各自的契約。",
    previous: "契約允許 `ready → published`，yomihon 仍然不走這一步。這個值記錄的是在別處發生的發表，閱讀介面無法為它作證。這個知識庫裡有一篇手寫上這個值的筆記，它的狀態面板沒有下一步。",
  },
};
const MUTATIONS = {
  'published-note-prose': { revert: 'published', language: 'en', route: 'course' },
  'published-search-excerpt': { revert: 'published', language: 'en', route: 'search' },
  'lifecycle-en-prose': { revert: 'en', language: 'en', route: 'course' },
  'lifecycle-zh-Hant-prose': { revert: 'zh-Hant', language: 'zh-Hant', route: 'course' },
};

class ContentFailure extends Error {
  constructor(site, message) {
    super(`${site}: ${message}`);
    this.site = site;
  }
}
const requireThat = (condition, message) => {
  if (!condition) throw new Error(message);
};
const words = (text) => text.replace(/\s+/gu, ' ').trim();
const requireText = (site, actual, expected) => {
  for (const phrase of expected) {
    if (!words(actual).includes(phrase)) {
      throw new ContentFailure(site, `missing ${JSON.stringify(phrase)} in ${JSON.stringify(words(actual))}`);
    }
  }
  if (/offers nothing onward|沒有下一步/u.test(actual)) {
    throw new ContentFailure(site, 'the explanation still denies the offered onward operation');
  }
};

async function follow(link) {
  requireThat(await link.count() === 1, 'the route must offer exactly one matching link');
  await link.click();
}

async function noPublishedTarget(page) {
  requireThat(await page.locator('form[action="/status"] input[name="to"][value="published"]').count() === 0,
    'the reading page offered published as a target');
}

async function readPublished(page, language) {
  requireThat(await page.locator('.y-article h1').innerText() === PUBLISHED, 'the route did not reach A published note');
  requireThat(await page.locator('.y-article').getAttribute('lang') === 'en', 'the English example lost its authored language');
  requireText('published-note-prose', await page.locator('.y-prose').innerText(), [
    'Its status panel offers archived, which this example contract permits from published.',
    'Yomihon never sets a note to published',
    'that value records a publication outside this vault, which yomihon cannot attest to.',
    'A note already marked published can still offer other transitions allowed by its contract.',
  ]);
  await noPublishedTarget(page);
  const face = page.locator('.y-statuspanel:visible, .y-sealbar:visible');
  requireThat(await face.count() === 1, 'the note must show one status face at this width');
  const forms = face.locator('form[action="/status"]');
  requireThat(await forms.count() === 1, 'the example published note must offer exactly one transition');
  requireThat(await forms.locator('input[name="from"]').getAttribute('value') === 'published', 'wrong current status');
  requireThat(await forms.locator('input[name="to"]').getAttribute('value') === 'archived', 'missing archived target');
  const disclosure = forms.locator('details');
  requireThat(await disclosure.getAttribute('open') === null, 'confirmation must start closed');
  await disclosure.locator('summary').click();
  requireThat(await disclosure.getAttribute('open') !== null, 'native disclosure did not open');
  const confirm = disclosure.getByRole('button', { name: LANGUAGES[language].confirm, exact: true });
  await confirm.waitFor({ state: 'visible' });
  requireThat(words(await disclosure.locator('.y-statusconfirm__note').innerText()) === LANGUAGES[language].warning,
    'the opened disclosure did not explain the no-return consequence');
  requireThat(await confirm.isVisible(), 'the opened disclosure has no visible confirmation button');
  // Deliberately stop here. Neither this button nor any other status submit is pressed.
}

async function courseRoute(page, base, language) {
  const labels = LANGUAGES[language];
  await page.goto(base, { waitUntil: 'domcontentloaded' });
  requireThat(await page.locator('html').getAttribute('lang') === language, 'wrong interface language');
  await follow(page.locator('[data-home-block="paths"] [data-desk-item]').filter({ hasText: labels.course }));
  requireThat(new URL(page.url()).pathname.startsWith('/syllabus/'), 'Home did not open the course');
  await follow(page.locator('main a.y-lesson').filter({ hasText: labels.lifecycle }));
  requireThat(await page.locator('.y-article h1').innerText() === labels.lifecycle, 'course did not open the lifecycle note');
  requireThat(await page.locator('.y-article').getAttribute('lang') === language, 'wrong authored explanation language');
  requireText(`lifecycle-${language}-prose`, await page.locator('.y-prose').innerText(), labels.explanation);
  const encodedDiagram = await page.locator('.y-prose .mermaid-diagram').getAttribute('data-mermaid-code');
  requireThat(encodedDiagram !== null, 'the lifecycle diagram has no authored source');
  const diagram = decodeURIComponent(encodedDiagram.replace(/\+/gu, ' '));
  requireThat(diagram.includes('ready --> published') && diagram.includes('published --> archived'),
    'the contract diagram lost the target or onward transition explained by the prose');
  await noPublishedTarget(page);
  await follow(page.locator('.y-prose').getByRole('link', { name: PUBLISHED, exact: true }));
  await readPublished(page, language);
}

async function searchRoute(page, base, language) {
  await page.goto(base, { waitUntil: 'domcontentloaded' });
  const form = page.locator('form.y-homesearch');
  requireThat(await form.getAttribute('method') === 'get' && await form.getAttribute('action') === '/search',
    'Home search must remain a native GET form');
  await form.locator('input[name="q"]').fill('published');
  await form.getByRole('button', { name: LANGUAGES[language].search, exact: true }).click();
  const address = new URL(page.url());
  requireThat(address.pathname === '/search' && address.searchParams.get('q') === 'published', 'native search did not submit the query');
  const result = page.locator('a.y-result').filter({
    has: page.locator('.y-result__title').filter({ hasText: /^A published note$/u }),
  });
  requireThat(await result.count() === 1, 'search must offer A published note exactly once');
  requireText('published-search-excerpt', await result.locator('.y-result__snippet').innerText(), [
    'Its status panel offers archived',
  ]);
  await follow(result);
  await readPublished(page, language);
}

async function probe(mode) {
  const base = process.env.YOMIHON_BASE;
  requireThat(base, 'serve.sh must supply YOMIHON_BASE');
  requireThat(!mode || Object.hasOwn(MUTATIONS, mode), `unknown mutation ${mode}`);
  const mutation = MUTATIONS[mode];
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    for (const language of mutation ? [mutation.language] : Object.keys(LANGUAGES)) {
      for (const width of mutation ? [1600] : [390, 1600]) {
        for (const javaScriptEnabled of mutation ? [false] : [false, true]) {
          const context = await browser.newContext({ viewport: { width, height: 1000 }, javaScriptEnabled });
          const posts = [];
          await context.route('**/status', async (route) => {
            if (route.request().method() === 'POST') {
              posts.push(route.request().url());
              await route.abort();
            } else {
              await route.continue();
            }
          });
          await context.addCookies([{ name: 'yomihon_lang', value: language, url: base }]);
          const page = await context.newPage();
          page.setDefaultTimeout(8000);
          try {
            if (!mutation || mutation.route === 'course') await courseRoute(page, base, language);
            if (!mutation || mutation.route === 'search') await searchRoute(page, base, language);
            const routes = mutation ? mutation.route : 'course and native search';
            console.log(`PASS published-explanation: ${language}, ${width}px, JavaScript ${javaScriptEnabled}, ${routes}`);
          } finally {
            await context.close();
            requireThat(posts.length === 0, `forbidden status POST attempt(s), all aborted: ${posts.join(', ')}`);
          }
        }
      }
    }
  } catch (error) {
    console.error(error);
    if (mode && error instanceof ContentFailure && error.site === mode) {
      console.log(`MUTATE-RESULT: caught ${mode}`);
    }
    process.exitCode = 1;
  } finally {
    await browser.close();
  }
}

function serve(binary, fixture, mode) {
  const result = spawnSync('bash', [
    join(ROOT, '.github/e2e/serve.sh'), binary, '19762', '--', process.execPath, SCRIPT, '--probe', mode,
  ], {
    cwd: ROOT, env: { ...process.env, YOMIHON_FIXTURE: fixture },
    encoding: 'utf8', timeout: 180000, maxBuffer: 8 * 1024 * 1024,
  });
  process.stdout.write(result.stdout ?? '');
  process.stderr.write(result.stderr ?? '');
  requireThat(!result.error && !result.signal, `acceptance process failed: ${result.error ?? result.signal}`);
  if (!mode) {
    requireThat(result.status === 0, `example acceptance exited ${result.status}`);
    return;
  }
  requireThat(result.status === 1 && result.stdout.split(/\r?\n/u).includes(`MUTATE-RESULT: caught ${mode}`),
    `${mode} did not fail at its intended content assertion (exit ${result.status})`);
}

function campaign(binary) {
  requireThat(binary, 'usage: node published-explanation.mjs <yomihon-binary>');
  const source = join(ROOT, 'examples/vault');
  serve(resolve(binary), source, '');
  for (const [mode, mutation] of Object.entries(MUTATIONS)) {
    const work = mkdtempSync(join(tmpdir(), 'yomihon-published-explanation-'));
    try {
      const fixture = join(work, 'vault');
      cpSync(source, fixture, { recursive: true });
      const change = REVERTS[mutation.revert];
      const file = join(fixture, change.file);
      const original = readFileSync(file, 'utf8');
      const matches = original.split(change.current).length - 1;
      if (matches !== 1) {
        console.error(`${mode}: passage matched ${matches} times, want exactly one`);
        console.log(`MUTATE-RESULT: not-applied ${mode}`);
        process.exitCode = 2;
        return;
      }
      writeFileSync(file, original.replace(change.current, change.previous));
      console.log(`MUTATE-APPLIED: ${mode}, exactly one passage in ${change.file}`);
      serve(resolve(binary), fixture, mode);
    } finally {
      rmSync(work, { recursive: true, force: true });
    }
  }
  console.log('PASS published-explanation: public-vault routes and all four authored-content reversions');
}

if (process.argv[2] === '--probe') {
  await probe(process.argv[3] ?? '');
} else {
  campaign(process.argv[2]);
}
