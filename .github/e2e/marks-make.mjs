// Behavior lock for making and changing the reader's own marks in a note.
//
// A passage chosen and a key pressed leaves a mark in the text: a highlight, a
// thought with the reader's words beside it, or a question that stays open until
// the reader says it is answered. The page that has just made a mark is the page
// that arrives with it: every mark is checked again after a reload, because the
// server draws them, and the page draws only what the server answered. A mark
// changes state one way and another (a thought turned to a question, a question
// answered and asked again), a deleted mark comes back by the one word the line
// offers or by the undo key, and a mark whose answer was lost on the way is still
// one mark. Two tabs writing the same mark are both kept and the newer is shown,
// with nothing said of it: a write the page was told it had not read the newest of
// is sent again once, quietly, and only a second refusal is said, in the words of
// a write that was not kept. A reason says what happened to the write and never
// where a mark is kept or what else it reaches.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every watched regression.
import { BASE, STORAGE_WORDS, choose, defineProbe, drawn } from './support/marks-probe.mjs';

const SITES = [
  'a-highlight-is-kept',
  'a-thought-is-kept',
  'a-question-is-kept',
  'a-state-changes',
  'a-delete-is-undone',
  'a-highlight-is-acted-on-by-its-bar',
  'a-lost-answer-keeps-one',
  'a-stale-write-is-rebased-quietly',
  'a-failed-write-is-said',
  'no-storage-words',
];

const script = (needle, replacement, site, file = 'marks.js') => ({ site, script: { file, needle, replacement } });
const MODES = {
  // A highlight the page believes it did not make.
  'highlight-not-kept': script(
    '    if (answer.status !== 200 && answer.status !== 201) return { ok: false, status: answer.status };\n    if (hidden()) setShown(true);\n    spans.set(id',
    '    return { ok: false, status: 500 };\n    if (hidden()) setShown(true);\n    spans.set(id',
    'a-highlight-is-kept',
  ),
  // A thought that keeps only the first letter of what was written.
  'thought-words-cut': script("return { patch: { kind: 'thought', body: value } };", "return { patch: { kind: 'thought', body: value.slice(0, 1) } };", 'a-thought-is-kept'),
  // A question made as a thought.
  'question-made-a-thought': script("const result = await keep(captured, 'question');", "const result = await keep(captured, 'thought');", 'a-question-is-kept'),
  // Answering a question changes nothing.
  'resolve-does-nothing': script("const result = await change('PATCH', id, { resolved });", 'const result = { ok: true };', 'a-state-changes'),
  // A thought turned to a question loses its words.
  'turn-drops-the-words': script("    const patch = { kind };\n    if (kind === 'highlight') patch.body = '';", "    const patch = { kind, body: '' };", 'a-state-changes'),
  // The word that brings a deleted mark back does nothing.
  'undo-brings-nothing-back': script("const back = await change('PUT', id, {});", 'const back = { ok: true };', 'a-delete-is-undone'),
  // A highlight whose actions are never shown: a pointer cannot delete it.
  'highlight-bar-never-shown': script('    actions.hidden = false;', '    actions.hidden = true;', 'a-highlight-is-acted-on-by-its-bar'),
  // A mark whose answer was lost is told as not kept although it is.
  'lost-answer-is-a-failure': script(
    "        answer = await send('GET', `\x24{endpoint}/\x24{id}`);\n        if (answer.status !== 200) answer = await send('POST', endpoint, request);\n      } catch {\n        return { ok: false, status: 0 };\n      }\n    }\n    if (answer.status !== 200 && answer.status !== 201) return { ok: false, status: answer.status };\n    if (hidden()) setShown(true);\n    spans.set(id",
    "        return { ok: false, status: 0 };\n      } catch {\n        return { ok: false, status: 0 };\n      }\n    }\n    if (answer.status !== 200 && answer.status !== 201) return { ok: false, status: answer.status };\n    if (hidden()) setShown(true);\n    spans.set(id",
    'a-lost-answer-keeps-one',
  ),
  // A write that was told it had not read the newest is not sent again.
  'no-quiet-resend': script("if (reason === 'stale' && attempt === 0) {", 'if (false) {', 'a-stale-write-is-rebased-quietly'),
  // A write that was refused says so only in a line that is not announced.
  'failure-not-announced': script("said.setAttribute('role', 'alert');", "said.setAttribute('role', 'status');", 'a-failed-write-is-said'),
  // A reason that names where the words would have gone.
  'reason-names-the-storage': { site: 'no-storage-words', doc: { needle: '書庫現在不能寫入', replacement: '無法同步到其他電腦的檔案' } },
};

const probe = defineProbe('marks-make', SITES, MODES);
const { runs, fire, broken } = probe;

// What the page said while it ran: the line, the fields, the notes and the
// toolbar, whenever any of them changed.
const watch = (page) =>
  page.evaluate(() => {
    window.__said = new Set();
    const take = () => {
      for (const element of document.querySelectorAll('[data-marks-status], .y-mfield, .y-mnote, .y-mbar')) {
        window.__said.add(element.textContent.replace(/\s+/g, ' ').trim());
        const label = element.getAttribute('aria-label');
        if (label) window.__said.add(label);
      }
    };
    new MutationObserver(take).observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ['aria-label', 'hidden'] });
    take();
  });
const said = (page) => page.evaluate(() => [...(window.__said ?? [])]);

const lifted = (page) => page.waitForFunction(() => document.documentElement.dataset.marksLifted !== undefined, null, { timeout: 5000 });
const reload = async (page) => {
  await page.reload({ waitUntil: 'load' });
  await lifted(page);
  await watch(page);
};
const find = async (page, words) => (await drawn(page)).find((entry) => entry.text.includes(words) || entry.note?.words.includes(words));
const settled = async (page) => {
  await page.waitForFunction(() => !document.querySelector('.y-mfield'), null, { timeout: 5000 }).catch(() => {});
};
const field = (page) => page.locator('.y-mfield textarea');

await probe.run(async ({ open }) => {
  const { page } = await open();
  await watch(page);
  const ids = {};

  // A highlight, by the key.
  const ALPHA = 'Alpha passage opens the first section';
  await choose(page, ALPHA);
  await page.keyboard.press('h');
  await field(page).waitFor({ timeout: 3000 }).catch(() => {});
  await page.keyboard.press('Meta+Enter');
  await settled(page);
  let entry = await find(page, ALPHA);
  if (runs('a-highlight-is-kept')) {
    if (!entry) fire('a-highlight-is-kept', `the highlight on "${ALPHA}" is not drawn after the key and its field were closed`);
    if (entry.kind !== 'highlight' || entry.state !== 'highlight' || entry.note) fire('a-highlight-is-kept', `the highlight is drawn as ${JSON.stringify(entry)}, want a highlight with no note`);
    if (entry.text !== ALPHA) fire('a-highlight-is-kept', `the highlight holds ${JSON.stringify(entry.text)}, want exactly the chosen words`);
    await reload(page);
    entry = await find(page, ALPHA);
    if (!entry || entry.kind !== 'highlight' || entry.text !== ALPHA) fire('a-highlight-is-kept', `after a reload the highlight on "${ALPHA}" is ${JSON.stringify(entry)}`);
    console.log('marks-make: a chosen passage and h leave a highlight, and a reload draws the same');
  } else if (!entry) {
    broken('the highlight was not made, so the other sites have nothing to read');
  }
  await watch(page);

  // A thought, by the toolbar and the field.
  const BETA = 'Beta passage follows it in the same paragraph';
  await choose(page, BETA);
  await page.click('[data-mbar="highlight"]');
  await field(page).waitFor({ timeout: 3000 }).catch(() => {});
  await page.keyboard.type('the tide is late');
  await page.keyboard.press('Meta+Enter');
  await settled(page);
  entry = await find(page, BETA);
  if (runs('a-thought-is-kept')) {
    if (!entry || entry.kind !== 'thought' || entry.note?.words !== 'the tide is late') fire('a-thought-is-kept', `the thought on "${BETA}" is ${JSON.stringify(entry)}, want a thought saying "the tide is late"`);
    if (!entry.note.label.startsWith('想法')) fire('a-thought-is-kept', `the thought is named ${JSON.stringify(entry.note.label)}, want a name that begins with its state`);
    await reload(page);
    entry = await find(page, BETA);
    if (!entry || entry.kind !== 'thought' || entry.note?.words !== 'the tide is late') fire('a-thought-is-kept', `after a reload the thought is ${JSON.stringify(entry)}`);
    console.log('marks-make: the toolbar and a field leave a thought with the reader\'s words, and a reload draws the same');
  }
  ids.beta = entry?.id;
  await watch(page);

  // A question, by the key.
  const GAMMA = 'Gamma passage closes the paragraph';
  await choose(page, GAMMA);
  await page.keyboard.press('q');
  await page.waitForFunction((words) => [...document.querySelectorAll('mark[data-mark]')].some((m) => m.textContent.includes(words)), GAMMA, { timeout: 3000 }).catch(() => {});
  entry = await find(page, GAMMA);
  if (runs('a-question-is-kept')) {
    if (!entry || entry.kind !== 'question' || entry.state !== 'question') fire('a-question-is-kept', `the question on "${GAMMA}" is ${JSON.stringify(entry)}, want a question that is open`);
    if (!entry.note?.label.startsWith('疑問')) fire('a-question-is-kept', `the question is named ${JSON.stringify(entry.note?.label)}, want a name that begins with 疑問`);
    await reload(page);
    entry = await find(page, GAMMA);
    if (!entry || entry.kind !== 'question') fire('a-question-is-kept', `after a reload the question is ${JSON.stringify(entry)}`);
    console.log('marks-make: a chosen passage and q leave an open question, and a reload draws the same');
  }
  ids.gamma = entry?.id;
  await watch(page);

  // The question is answered and asked again; the thought becomes a question and
  // goes back.
  if (runs('a-state-changes')) {
    const stateOf = async (id) => (await drawn(page)).find((e) => e.id === id);
    await page.locator(`#mn-${ids.gamma}`).focus();
    await page.keyboard.press('r');
    await page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.dataset.state === 'resolved', ids.gamma, { timeout: 3000 }).catch(() => {});
    let now = await stateOf(ids.gamma);
    if (now?.note?.state !== 'resolved' || !now.note.label.startsWith('已解決')) fire('a-state-changes', `after r the question is ${JSON.stringify(now)}, want it answered and named so`);
    await page.locator(`#mn-${ids.gamma}`).focus();
    await page.keyboard.press('r');
    await page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.dataset.state === 'question', ids.gamma, { timeout: 3000 }).catch(() => {});
    now = await stateOf(ids.gamma);
    if (now?.note?.state !== 'question') fire('a-state-changes', `after r again the question is ${JSON.stringify(now)}, want it open again`);

    await page.locator(`#mn-${ids.beta}`).focus();
    await page.keyboard.press('q');
    await page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.dataset.state === 'question', ids.beta, { timeout: 3000 }).catch(() => {});
    now = await stateOf(ids.beta);
    if (now?.kind !== 'question' || now.note?.words !== 'the tide is late') fire('a-state-changes', `after q the thought is ${JSON.stringify(now)}, want a question that keeps "the tide is late"`);
    await page.keyboard.press('Meta+z');
    await page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.dataset.state === 'thought', ids.beta, { timeout: 3000 }).catch(() => {});
    now = await stateOf(ids.beta);
    if (now?.kind !== 'thought') fire('a-state-changes', `after the undo key the question is ${JSON.stringify(now)}, want the thought it was`);
    await reload(page);
    now = await stateOf(ids.beta);
    if (now?.kind !== 'thought' || now.note?.words !== 'the tide is late') fire('a-state-changes', `after a reload the thought is ${JSON.stringify(now)}`);
    console.log('marks-make: a question is answered and asked again, a thought becomes a question and comes back, all of it kept');
  }

  // A mark deleted comes back by the word on the line and by the undo key.
  if (runs('a-delete-is-undone')) {
    await page.locator(`#mn-${ids.beta}`).focus();
    await page.keyboard.press('Delete');
    await page.waitForFunction((id) => !document.getElementById(`mn-${id}`) && !document.querySelector(`mark[data-mark="${id}"]`), ids.beta, { timeout: 3000 }).catch(() => {});
    if (await find(page, BETA)) fire('a-delete-is-undone', 'a deleted thought is still drawn');
    const line = await page.locator('[data-marks-status]').innerText();
    if (!line.includes('已刪除') || !line.includes('復原')) fire('a-delete-is-undone', `after a delete the line says ${JSON.stringify(line)}, want 已刪除 and the word that undoes it, 復原`);
    await page.click('[data-mstatus-act]');
    await page.waitForFunction((id) => Boolean(document.getElementById(`mn-${id}`)), ids.beta, { timeout: 3000 }).catch(() => {});
    entry = await find(page, BETA);
    if (!entry || entry.kind !== 'thought' || entry.note?.words !== 'the tide is late') fire('a-delete-is-undone', `after 復原 the thought is ${JSON.stringify(entry)}, want it back as it was`);
    await reload(page);
    if (!(await find(page, BETA))) fire('a-delete-is-undone', 'after a reload the thought that was brought back is gone');
    // And gone for good once deleted and left.
    await page.locator(`#mn-${ids.beta}`).focus();
    await page.keyboard.press('Delete');
    await page.waitForFunction((id) => !document.getElementById(`mn-${id}`), ids.beta, { timeout: 3000 }).catch(() => {});
    await reload(page);
    if (await find(page, BETA)) fire('a-delete-is-undone', 'a deleted thought is drawn again by a reload');
    console.log('marks-make: a deleted mark comes back by 復原, and stays deleted when it is not brought back');
  }

  // A highlight has no note, so the actions stand above its words while it has
  // the keyboard, and a pointer reaches them: write a thought, ask, delete.
  if (runs('a-highlight-is-acted-on-by-its-bar')) {
    const bar = await open();
    const ETA = 'Eta passage is the second item of a list';
    await choose(bar.page, ETA);
    await bar.page.keyboard.press('h');
    await field(bar.page).waitFor({ timeout: 3000 }).catch(() => {});
    await bar.page.keyboard.press('Meta+Enter');
    await settled(bar.page);
    const mark = bar.page.locator('mark[data-kind="highlight"]', { hasText: ETA }).first();
    await mark.focus();
    const actions = await bar.page.locator('.y-mbar[data-for]:not([hidden]) .y-mbar__act').allInnerTexts();
    if (actions.join('|') !== '寫想法|標為疑問|刪除') fire('a-highlight-is-acted-on-by-its-bar', `a highlight that has the keyboard offers ${JSON.stringify(actions)}, want 寫想法, 標為疑問 and 刪除`);
    await bar.page.click('.y-mbar[data-for] [data-act="delete"]');
    await bar.page.waitForFunction((words) => ![...document.querySelectorAll('mark[data-mark]')].some((m) => m.textContent.includes(words)), ETA, { timeout: 3000 }).catch(() => {});
    if (await find(bar.page, ETA)) fire('a-highlight-is-acted-on-by-its-bar', 'a highlight deleted from its actions is still drawn');
    console.log('marks-make: a highlight offers write, ask and delete over its words, and a pointer deletes it');
    await bar.close();
  }

  // The answer to a keep is lost, and the mark was kept: it is one mark, and no
  // failure is said.
  if (runs('a-lost-answer-keeps-one')) {
    const second = await open();
    let lost = 0;
    await second.page.route('**/mark', async (route) => {
      if (route.request().method() !== 'POST' || lost > 0) return route.fallback();
      lost += 1;
      await route.fetch();
      return route.abort('connectionreset');
    });
    await watch(second.page);
    const DELTA = 'Delta passage stands in a paragraph of its own';
    await choose(second.page, DELTA);
    await second.page.keyboard.press('q');
    await second.page.waitForFunction((words) => [...document.querySelectorAll('mark[data-mark]')].some((m) => m.textContent.includes(words)) || document.querySelector('[data-mstatus-alert]').textContent !== '', DELTA, { timeout: 4000 }).catch(() => {});
    const alert = await second.page.locator('[data-mstatus-alert]').textContent();
    if (lost !== 1) broken(`the answer was lost ${lost} times, want once`);
    if (alert !== '') fire('a-lost-answer-keeps-one', `a mark that was kept and whose answer was lost is told as not kept: ${JSON.stringify(alert)}`);
    entry = await find(second.page, DELTA);
    if (!entry || entry.kind !== 'question') fire('a-lost-answer-keeps-one', `the mark whose answer was lost is ${JSON.stringify(entry)}, want it drawn as a question`);
    await reload(second.page);
    const again = (await drawn(second.page)).filter((e) => e.text.includes(DELTA));
    if (again.length !== 1) fire('a-lost-answer-keeps-one', `after a reload the passage carries ${again.length} marks, want one`);
    console.log('marks-make: a mark whose answer was lost is one mark, drawn, with no failure said');
    await second.close();
  }

  // Another tab changed the mark first: the page's write is sent again once,
  // quietly, and the page's words are the newest.
  if (runs('a-stale-write-is-rebased-quietly')) {
    const third = await open();
    await watch(third.page);
    const EPSILON = 'Epsilon passage ends it';
    await choose(third.page, EPSILON);
    await third.page.keyboard.press('h');
    await field(third.page).waitFor({ timeout: 3000 }).catch(() => {});
    await third.page.keyboard.type('first words');
    await third.page.keyboard.press('Meta+Enter');
    await settled(third.page);
    entry = await find(third.page, EPSILON);
    if (!entry?.note) broken('the thought that the other tab changes was not made');
    const mine = await third.context.request.get(`${BASE}/mark/${entry.id}`);
    const seen = (await mine.json()).seen;
    const other = await third.context.request.patch(`${BASE}/mark/${entry.id}`, { data: { seen, kind: 'thought', body: 'from the other tab' } });
    if (other.status() !== 200) broken(`the other tab's change answered ${other.status()}, want 200`);
    const refusals = [];
    third.page.on('response', (response) => {
      if (response.request().method() === 'PATCH' && response.status() === 409) refusals.push(response.url());
    });
    await third.page.locator(`#mn-${entry.id}`).focus();
    await third.page.keyboard.press('Enter');
    await field(third.page).waitFor({ timeout: 3000 }).catch(() => {});
    await third.page.keyboard.press('End');
    await third.page.keyboard.type(' and more');
    await third.page.keyboard.press('Meta+Enter');
    await third.page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.querySelector('.y-mnote__words')?.textContent === 'first words and more' || document.querySelector('.y-mfield__said:not([hidden])'), entry.id, { timeout: 4000 }).catch(() => {});
    const shown = await find(third.page, EPSILON);
    if (shown?.note?.words !== 'first words and more') fire('a-stale-write-is-rebased-quietly', `after a write over a newer change the note says ${JSON.stringify(shown?.note?.words)}, want the page's words, "first words and more"`);
    if ((await third.page.locator('.y-mfield__said:not([hidden])').count()) > 0 || (await third.page.locator('[data-mstatus-alert]').textContent()) !== '') fire('a-stale-write-is-rebased-quietly', 'a write that was sent again once is said as not kept');
    const body = (await (await third.context.request.get(`${BASE}/mark/${entry.id}`)).json()).body;
    if (body !== 'first words and more') fire('a-stale-write-is-rebased-quietly', `the server keeps ${JSON.stringify(body)}, want the page's words`);
    if (refusals.length !== 1) fire('a-stale-write-is-rebased-quietly', `the page was refused ${refusals.length} times, want the one refusal that its quiet resend answers`);
    const spoken = (await said(third.page)).join('\n');
    if (/另一個分頁|別的分頁|其他分頁|another tab|other tab|changed elsewhere/i.test(spoken)) fire('a-stale-write-is-rebased-quietly', `the page says another tab changed the mark: ${spoken}`);
    console.log('marks-make: a write over a newer change is sent again once, quietly, and the newest words are shown');
    await third.close();
  }

  // A write that is refused is said where the reader is writing, announced, in
  // the words of a write that was not kept.
  if (runs('a-failed-write-is-said') || runs('no-storage-words')) {
    const fourth = await open();
    await watch(fourth.page);
    await fourth.page.route('**/mark/*', (route) => (route.request().method() === 'PATCH' ? route.fulfill({ status: 500, contentType: 'text/plain', body: 'internal detail: disk /var/vault is full' }) : route.fallback()));
    const ZETA = 'Zeta passage is the first item of a list';
    await choose(fourth.page, ZETA);
    await fourth.page.keyboard.press('h');
    await field(fourth.page).waitFor({ timeout: 3000 }).catch(() => {});
    await fourth.page.keyboard.type('a thought that is refused');
    await fourth.page.waitForSelector('.y-mfield__said:not([hidden])', { timeout: 4000 }).catch(() => {});
    const refusal = await fourth.page.evaluate(() => {
      const element = document.querySelector('.y-mfield__said:not([hidden])');
      return element ? { text: element.textContent.trim(), role: element.getAttribute('role') } : null;
    });
    if (runs('a-failed-write-is-said')) {
      if (!refusal) fire('a-failed-write-is-said', 'a write the server refused is not said where the reader is writing');
      if (!refusal.text.includes('沒有留下')) fire('a-failed-write-is-said', `a refused write says ${JSON.stringify(refusal.text)}, want the words of a write that was not kept`);
      if (refusal.role !== 'alert') fire('a-failed-write-is-said', `the refusal has the role ${refusal.role}, want alert so that it is read at once`);
      if (/internal detail|disk|\/var/.test(refusal.text)) fire('a-failed-write-is-said', `the refusal repeats what the server said: ${JSON.stringify(refusal.text)}`);
      console.log('marks-make: a refused write is said at once, in the words of a write that was not kept, with nothing of the server\'s own');
    }
    await fourth.page.unroute('**/mark/*');
    await fourth.page.keyboard.press('Escape');
    await fourth.close();
  }

  if (runs('no-storage-words')) {
    const words = (await said(page)).join('\n');
    const pagesWords = await page.evaluate(() => document.querySelector('.y-article')?.getAttribute('data-marks-strings') ?? '');
    for (const text of [words, pagesWords]) {
      const hit = text.match(STORAGE_WORDS);
      if (hit) fire('no-storage-words', `the page of marks says ${JSON.stringify(hit[0])}; the reader only reads, and the page never says where a mark is kept`);
    }
    // The refusal above is read too, where the mutation puts the reason.
    const refused = await open();
    await refused.page.route('**/mark/*', (route) => (route.request().method() === 'PATCH' ? route.fulfill({ status: 500, contentType: 'text/plain', body: 'no' }) : route.fallback()));
    await choose(refused.page, 'Theta passage opens the second section');
    await refused.page.keyboard.press('h');
    await field(refused.page).waitFor({ timeout: 3000 }).catch(() => {});
    await refused.page.keyboard.type('x');
    await refused.page.waitForSelector('.y-mfield__said:not([hidden])', { timeout: 4000 }).catch(() => {});
    const spoken = await refused.page.locator('.y-mfield__said').textContent();
    const hit = `${spoken}\n${await refused.page.locator('.y-article').getAttribute('data-marks-strings')}`.match(STORAGE_WORDS);
    if (hit) fire('no-storage-words', `a refused write says ${JSON.stringify(hit[0])}, which names where a mark goes`);
    console.log('marks-make: nothing the page says names where a mark is kept');
    await refused.page.unroute('**/mark/*');
    await refused.page.keyboard.press('Escape');
    await refused.close();
  }
  await page.close();
});
