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
import { BASE, STORAGE_WORDS, choose, defineProbe, drawn, keepByHand } from './support/marks-probe.mjs';

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
  'a-section-thought-is-one',
  'a-section-thought-keeps-its-id',
  'typed-words-outlive-a-delete',
  'a-leaving-write-is-sent-at-once',
  'an-undo-does-not-rebase',
  'one-field-is-open-at-a-time',
  'a-cancelled-section-thought-comes-back',
  'a-name-keeps-the-words-as-they-are',
  'a-too-long-write-says-so',
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
  'undo-brings-nothing-back': script("const back = await change('PUT', id, {}, { rebase: false });", 'const back = { ok: true };', 'a-delete-is-undone'),
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
  // A field left in a hurry makes the section's thought twice.
  'section-thought-not-waited': script('    while (session.creating) await session.creating;\n', '', 'a-section-thought-is-one'),
  // A section's thought whose first try was lost is made again under a new id.
  'section-id-per-attempt': script('const id = session.id ?? session.draftId;', 'const id = session.id ?? newId();', 'a-section-thought-keeps-its-id'),
  // A mark deleted elsewhere takes the typed words with it.
  'gone-takes-the-words': script('if (result.gone && !restored && session.parts.text.value !== \'\') {', 'if (false) {', 'typed-words-outlive-a-delete'),
  // A send made as the page is left waits behind the one in flight.
  'leaving-write-queued': script('const queued = immediate ? run() : before.then(run, run);', 'const queued = before.then(run, run);', 'a-leaving-write-is-sent-at-once'),
  // A send made as the page is left is not one that survives it.
  'leaving-write-not-keepalive': script('const answer = await send(method, `\x24{endpoint}/\x24{id}`, { seen: base, ...patch }, { keepalive });', 'const answer = await send(method, `\x24{endpoint}/\x24{id}`, { seen: base, ...patch }, {});', 'a-leaving-write-is-sent-at-once'),
  // Taking a step back writes over what another tab has since changed.
  'undo-rebases': script("if (reason === 'stale' && !rebase) return { ok: false, stale: true };", '', 'an-undo-does-not-rebase'),
  // Two presses before the first field is open make two fields.
  'two-fields-open': script('    if (claimed) return;\n', '', 'one-field-is-open-at-a-time'),
  // A section's thought that was thrown away cannot be got back.
  'cancelled-thought-is-lost': script("undoMade = record(strings.whatWords, () => change('PUT', id, {}, { rebase: false }));", 'undoMade = null;', 'a-cancelled-section-thought-comes-back'),
  // A passage whose words hold a replacement pattern is named by something else.
  'name-eats-the-dollar-signs': script("const fmt = (text, value) => text.replace('%s', () => value);", "const fmt = (text, value) => text.replace('%s', value);", 'a-name-keeps-the-words-as-they-are'),
  // A write that is twice as long as the page allows is told as not kept for no reason.
  'too-long-is-twice-too-long': script('const TOO_LONG = 32 * 1024;', 'const TOO_LONG = 64 * 1024;', 'a-too-long-write-says-so'),
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

  // A section's thought, left in a hurry: the field is left by a press elsewhere
  // while its first send is still on its way. It is one thought, sent once.
  if (runs('a-section-thought-is-one')) {
    const hurry = await open();
    await watch(hurry.page);
    const posts = [];
    await hurry.page.route('**/mark', async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      posts.push(route.request().postDataJSON());
      await new Promise((resolve) => setTimeout(resolve, 600));
      return route.fallback();
    });
    await hurry.page.locator('[data-section-thought]').first().evaluate((button) => button.click());
    await field(hurry.page).waitFor({ timeout: 3000 }).catch(() => {});
    const SAID = 'a thought for the whole section';
    await hurry.page.keyboard.type(SAID);
    await hurry.page.evaluate(() => document.activeElement.blur());
    await hurry.page.waitForFunction(() => !document.querySelector('.y-mfield'), null, { timeout: 6000 }).catch(() => {});
    await hurry.page.waitForTimeout(800);
    if (posts.length !== 1) fire('a-section-thought-is-one', `a field left in a hurry sent ${posts.length} requests to make the thought, want one`);
    await reload(hurry.page);
    const notes = (await drawn(hurry.page)).filter((e) => e.note?.words === SAID);
    if (notes.length !== 1) fire('a-section-thought-is-one', `after a reload the section carries ${notes.length} notes saying ${JSON.stringify(SAID)}, want one`);
    console.log('marks-make: a section thought whose field is left in a hurry is made once');
    await hurry.close();
  }

  // The first try of a section's thought was kept and its answer lost, and the
  // check for it was lost too: the reader asks again, and it is still one.
  if (runs('a-section-thought-keeps-its-id')) {
    const lostTwice = await open();
    await watch(lostTwice.page);
    let lost = 0;
    await lostTwice.page.route('**/mark', async (route) => {
      if (route.request().method() !== 'POST' || lost > 0) return route.fallback();
      lost += 1;
      await route.fetch();
      return route.abort('connectionreset');
    });
    await lostTwice.page.route('**/mark/*', (route) => (route.request().method() === 'GET' && lost === 1 ? route.abort('connectionreset') : route.fallback()));
    await lostTwice.page.locator('[data-section-thought]').nth(1).evaluate((button) => button.click());
    await field(lostTwice.page).waitFor({ timeout: 3000 }).catch(() => {});
    const SAID = 'a thought whose answer was lost';
    await lostTwice.page.keyboard.type(SAID);
    await lostTwice.page.waitForSelector('.y-mfield__said:not([hidden])', { timeout: 4000 }).catch(() => {});
    await lostTwice.page.unroute('**/mark/*');
    await lostTwice.page.click('.y-mfield__retry').catch(() => {});
    await lostTwice.page.waitForTimeout(800);
    await lostTwice.page.keyboard.press('Meta+Enter');
    await settled(lostTwice.page);
    await reload(lostTwice.page);
    const notes = (await drawn(lostTwice.page)).filter((e) => e.note?.words === SAID);
    if (lost !== 1) broken(`the answer was lost ${lost} times, want once`);
    if (notes.length !== 1) fire('a-section-thought-keeps-its-id', `after the answer was lost and the thought was asked for again, the section carries ${notes.length} notes, want one`);
    console.log('marks-make: a section thought asked for again after a lost answer is still one');
    await lostTwice.close();
  }

  // The mark is deleted elsewhere while the reader is writing in it: what was
  // typed is not dropped, the mark comes back with it.
  if (runs('typed-words-outlive-a-delete')) {
    const gone = await open();
    await watch(gone.page);
    const NU = 'Nu passage opens the third section';
    await choose(gone.page, NU);
    await gone.page.keyboard.press('h');
    await field(gone.page).waitFor({ timeout: 3000 }).catch(() => {});
    await gone.page.keyboard.type('kept words');
    await gone.page.waitForTimeout(900);
    const id = await gone.page.evaluate(() => document.querySelector('.y-mfield')?.dataset.mfield ?? '');
    if (!id) broken('the field of the mark has no id after its first words were sent');
    const seen = (await (await gone.context.request.get(`${BASE}/mark/${id}`)).json()).seen;
    const deleted = await gone.context.request.delete(`${BASE}/mark/${id}`, { data: { seen } });
    if (deleted.status() !== 200) broken(`the other tab's delete answered ${deleted.status()}, want 200`);
    await gone.page.keyboard.type(' and typed while it was gone');
    await gone.page.waitForTimeout(900);
    const typing = await gone.page.locator('.y-mfield textarea').count();
    if (typing !== 1) fire('typed-words-outlive-a-delete', 'the field closed when its mark was found deleted, and the words typed after it were not in it');
    await gone.page.keyboard.press('Meta+Enter');
    await settled(gone.page);
    const back = await (await gone.context.request.get(`${BASE}/mark/${id}`)).json();
    if (back.deleted || back.body !== 'kept words and typed while it was gone') fire('typed-words-outlive-a-delete', `the server keeps ${JSON.stringify({ deleted: back.deleted, body: back.body })}, want the mark back with "kept words and typed while it was gone"`);
    await reload(gone.page);
    entry = await find(gone.page, 'kept words and typed while it was gone');
    if (!entry || entry.text !== NU) fire('typed-words-outlive-a-delete', `after a reload the mark is ${JSON.stringify(entry)}, want it on "${NU}" with the typed words`);
    console.log('marks-make: words typed into a mark that was deleted elsewhere are kept, and the mark with them');
    await gone.close();
  }

  // What is typed goes at once when the page is hidden, whatever is in flight,
  // and as a send that survives the page.
  if (runs('a-leaving-write-is-sent-at-once')) {
    const leaving = await open();
    await leaving.page.addInitScript(() => {
      window.__sends = [];
      const original = window.fetch;
      window.fetch = (url, init) => {
        window.__sends.push({ method: init?.method ?? 'GET', url: String(url), keepalive: Boolean(init?.keepalive) });
        return original.call(window, url, init);
      };
    });
    await reload(leaving.page);
    const XI = 'Xi passage is a short one';
    await choose(leaving.page, XI);
    await leaving.page.keyboard.press('h');
    await field(leaving.page).waitFor({ timeout: 3000 }).catch(() => {});
    await leaving.page.keyboard.type('one');
    await leaving.page.waitForTimeout(900);
    const arrived = [];
    await leaving.page.route('**/mark/*', async (route) => {
      if (route.request().method() !== 'PATCH') return route.fallback();
      arrived.push(route.request().postDataJSON().body);
      if (arrived.length === 1) await new Promise((resolve) => setTimeout(resolve, 1800));
      return route.fallback();
    });
    await leaving.page.keyboard.type(' two');
    for (let waited = 0; arrived.length < 1 && waited < 3000; waited += 50) await leaving.page.waitForTimeout(50);
    await leaving.page.keyboard.type(' three');
    await leaving.page.evaluate(() => window.dispatchEvent(new Event('pagehide')));
    await leaving.page.waitForTimeout(500);
    const sends = await leaving.page.evaluate(() => window.__sends.filter((s) => s.method === 'PATCH'));
    const last = sends[sends.length - 1];
    if (arrived.length < 2 || !arrived[arrived.length - 1].endsWith('three')) fire('a-leaving-write-is-sent-at-once', `with a send in flight the words typed after it were sent as ${JSON.stringify(arrived)}, want the last of them to reach the server at once, ending in "three"`);
    if (!last?.keepalive) fire('a-leaving-write-is-sent-at-once', `the send made as the page is hidden is ${JSON.stringify(last)}, want one that survives the page, with keepalive`);
    console.log('marks-make: what is typed goes at once as the page is hidden, ahead of what is in flight, and survives it');
    await leaving.page.unroute('**/mark/*');
    await leaving.page.keyboard.press('Escape');
    await leaving.close();
  }

  // A step is taken back only if the mark is as that step left it. If another
  // tab has changed it since, the step cannot be undone, and is said so.
  if (runs('an-undo-does-not-rebase')) {
    const undo = await open();
    await watch(undo.page);
    const MU = 'Mu passage is code and is chosen as code.';
    await choose(undo.page, MU);
    await undo.page.keyboard.press('h');
    await field(undo.page).waitFor({ timeout: 3000 }).catch(() => {});
    await undo.page.keyboard.type('a thought on code');
    await undo.page.keyboard.press('Meta+Enter');
    await settled(undo.page);
    const mine = await find(undo.page, MU);
    if (!mine?.note) broken('the thought that is turned and taken back was not made');
    await undo.page.locator(`#mn-${mine.id}`).focus();
    await undo.page.keyboard.press('q');
    await undo.page.waitForFunction((id) => document.getElementById(`mn-${id}`)?.dataset.state === 'question', mine.id, { timeout: 3000 }).catch(() => {});
    const seen = (await (await undo.context.request.get(`${BASE}/mark/${mine.id}`)).json()).seen;
    const other = await undo.context.request.patch(`${BASE}/mark/${mine.id}`, { data: { seen, body: 'from the other tab' } });
    if (other.status() !== 200) broken(`the other tab's change answered ${other.status()}, want 200`);
    await undo.page.keyboard.press('Meta+z');
    await undo.page.waitForFunction(() => document.querySelector('[data-marks-status]')?.textContent.includes('不能復原') ?? false, null, { timeout: 3000 }).catch(() => {});
    const line = await undo.page.locator('[data-marks-status]').innerText();
    const now = await (await undo.context.request.get(`${BASE}/mark/${mine.id}`)).json();
    if (now.kind !== 'question' || now.body !== 'from the other tab') fire('an-undo-does-not-rebase', `the undo wrote over the other tab's change: the server keeps ${JSON.stringify({ kind: now.kind, body: now.body })}, want a question saying "from the other tab"`);
    if (!line.includes('已經不能復原')) fire('an-undo-does-not-rebase', `after an undo that could not be made the line says ${JSON.stringify(line)}, want 這一步已經不能復原了`);
    console.log('marks-make: a step that another tab has changed since is not undone over it, and is said so');
    await undo.close();
  }

  // Asking to write in two marks at once leaves one field, not two.
  if (runs('one-field-is-open-at-a-time')) {
    const seed = await open();
    await keepByHand(seed.context, { kind: 'thought', quote: 'Kappa passage', body: 'first of the two' });
    await keepByHand(seed.context, { kind: 'thought', quote: 'Lambda passage', body: 'second of the two' });
    await reload(seed.page);
    await seed.page.evaluate(() => {
      const [first, second] = [...document.querySelectorAll('.y-mnote [data-act="edit"]')];
      first.click();
      second.click();
    });
    await seed.page.waitForTimeout(800);
    const fields = await seed.page.locator('.y-mfield').count();
    if (fields !== 1) fire('one-field-is-open-at-a-time', `two presses to write, one after the other, left ${fields} fields open, want one`);
    console.log('marks-make: two presses to write leave one field open');
    await seed.page.keyboard.press('Escape');
    await seed.close();
  }

  // A section's thought is written and then thrown away with Cancel: the line says
  // so with the word that brings it back, and it comes back as it was.
  if (runs('a-cancelled-section-thought-comes-back')) {
    const thrown = await open();
    await watch(thrown.page);
    await thrown.page.locator('[data-section-thought]').nth(2).evaluate((button) => button.click());
    await field(thrown.page).waitFor({ timeout: 3000 }).catch(() => {});
    const SAID = 'a thought that is thrown away';
    await thrown.page.keyboard.type(SAID);
    await thrown.page.waitForTimeout(900);
    await thrown.page.click('.y-mfield__cancel');
    await settled(thrown.page);
    const line = await thrown.page.locator('[data-marks-status]').innerText();
    if (!line.includes('復原')) fire('a-cancelled-section-thought-comes-back', `after Cancel the line says ${JSON.stringify(line)}, want the word that brings the thought back, 復原`);
    else {
      if ((await drawn(thrown.page)).some((e) => e.note?.words === SAID)) fire('a-cancelled-section-thought-comes-back', 'a thought that was thrown away is still drawn');
      await thrown.page.click('[data-mstatus-act]');
      await thrown.page.waitForFunction((words) => [...document.querySelectorAll('.y-mnote .y-mnote__words')].some((e) => (e.textContent || e.dataset.text) === words), SAID, { timeout: 3000 }).catch(() => {});
      await reload(thrown.page);
      const back = (await drawn(thrown.page)).filter((e) => e.note?.words === SAID);
      if (back.length !== 1) fire('a-cancelled-section-thought-comes-back', `after 復原 and a reload the section carries ${back.length} notes saying ${JSON.stringify(SAID)}, want one`);
    }
    console.log('marks-make: a section thought thrown away with Cancel comes back by 復原');
    await thrown.close();
  }

  // The name a mark is given is its words, as they are: a passage that holds the
  // marks a replacement pattern is made of is still named by itself.
  if (runs('a-name-keeps-the-words-as-they-are')) {
    const dollars = await open();
    const RHO = 'Rho passage costs $& and';
    await choose(dollars.page, RHO);
    await dollars.page.keyboard.press('h');
    await field(dollars.page).waitFor({ timeout: 3000 }).catch(() => {});
    await dollars.page.keyboard.press('Meta+Enter');
    await settled(dollars.page);
    const named = await dollars.page.locator('mark[data-mark]', { hasText: 'Rho passage' }).first().getAttribute('aria-label');
    if (!named?.includes('Rho passage costs $&')) fire('a-name-keeps-the-words-as-they-are', `a passage that holds $& is named ${JSON.stringify(named)}, want its own words, "Rho passage costs $&"`);
    console.log('marks-make: a passage is named by its own words, whatever characters they hold');
    await dollars.close();
  }

  // A write longer than the page can keep is told so, and in no other way.
  if (runs('a-too-long-write-says-so')) {
    const long = await open();
    await long.page.locator('[data-section-thought]').nth(3).evaluate((button) => button.click());
    await field(long.page).waitFor({ timeout: 3000 }).catch(() => {});
    await long.page.locator('.y-mfield textarea').fill('a'.repeat(40 * 1024));
    await long.page.waitForSelector('.y-mfield__said:not([hidden])', { timeout: 6000 }).catch(() => {});
    const refusal = await long.page.locator('.y-mfield__said').textContent();
    if (!refusal.includes('太長')) fire('a-too-long-write-says-so', `a write of 40KB is refused with ${JSON.stringify(refusal)}, want the words that say it is too long`);
    console.log('marks-make: a write that is too long is said to be too long');
    await long.page.locator('.y-mfield textarea').fill('');
    await long.page.keyboard.press('Escape');
    await long.close();
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
