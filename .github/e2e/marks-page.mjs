// Behavior lock for how the page of a note shows the reader's own marks.
//
// The page is read without any script: every mark the reader left is in the HTML
// the server sends, the passage washed and each note after the paragraph it is
// about, so the marks are there to a reader whose script never ran. Each mark has
// a name that begins with its state, and the keyboard walks the marks in the
// order of the text. The key m hides every mark and shows them again; the choice
// is kept in a cookie the server reads, so a page that arrives with marks hidden
// is drawn without them in its first frame. The colours say whose a thing is and
// whether it is open and nothing else: the reader's own blue for what they left,
// ochre for a question still open, crimson only where a write failed, ink for the
// rest, a question that was answered included; every word reads at 4.5 to 1 in
// light and in dark. A touch screen, which has nothing to rest on a heading, is
// given the section buttons on the headings of the contents and not on every
// level.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every watched regression.
import { BASE, axOf, channels, choose, contrast, defineProbe, keepByHand, resolveColour } from './support/marks-probe.mjs';

const SITES = [
  'marks-read-without-script',
  'every-mark-has-a-name',
  'braces-walk-the-marks',
  'm-hides-and-shows',
  'the-choice-is-in-the-first-frame',
  'colour-roles',
  'every-word-is-readable',
  'touch-section-buttons',
  'the-status-line-stays-in-the-tree',
  'the-first-frame-hides-notes-only-where-the-margin-is',
];

const script = (needle, replacement, site, file = 'marks.js') => ({ site, script: { file, needle, replacement } });
const MODES = {
  // The notes are drawn only once a script runs.
  'notes-need-a-script': { site: 'marks-read-without-script', css: ':root:not([data-js]) .y-mnote{display:none !important}' },
  // A note drawn before the passage it is about.
  'note-above-its-words': { site: 'marks-read-without-script', css: '.y-prose{display:flex !important;flex-direction:column !important}.y-mnote{order:-1 !important}' },
  // The first piece of a mark that says nothing of its state.
  'name-without-the-state': { site: 'every-mark-has-a-name', doc: { needle: 'aria-label="想法：', replacement: 'aria-label="' } },
  // The marks walked from the last to the first.
  'braces-go-backwards': script('const next = event.key === \'}\' ? all[(at + 1) % all.length] : all[(at - 1 + all.length) % all.length];', 'const next = event.key === \'{\' ? all[(at + 1) % all.length] : all[(at - 1 + all.length) % all.length];', 'braces-walk-the-marks'),
  // m does nothing.
  'm-does-nothing': script("if (event.key === 'm') {\n      setShown(hidden());\n      return true;\n    }", "if (event.key === 'm') {\n      return true;\n    }", 'm-hides-and-shows'),
  // Hidden marks that are still tinted.
  'hidden-marks-still-washed': { site: 'm-hides-and-shows', css: ":root[data-marks='hide'] .y-mark{background:var(--reader-wash) !important}" },
  // Hidden marks that the keyboard still stops on.
  'hidden-marks-still-stops': script("piece.tabIndex = hidden() ? -1 : 0;", 'piece.tabIndex = 0;', 'm-hides-and-shows'),
  // The choice is not stamped on the page the server sends.
  'choice-not-stamped': { site: 'the-choice-is-in-the-first-frame', doc: { needle: ' data-marks="hide"', replacement: ' data-marks=""' } },
  // An open question underlined in the colour of a failed write.
  'question-in-crimson': { site: 'colour-roles', css: ":root:not([data-marks='hide']) .y-mark[data-state='question']{text-decoration-color:var(--crimson) !important}" },
  // A question that was answered keeps the ochre rule.
  'answered-keeps-ochre': { site: 'colour-roles', css: ".y-mnote[data-state='resolved']{border-inline-start-color:var(--ochre) !important}" },
  // A thought's rule fainter than the reader's own indigo.
  'thought-rule-faint': { site: 'colour-roles', css: ".y-mnote[data-kind='thought']{border-inline-start-color:var(--reader-line) !important}" },
  // An open question's rule as faint as the line the page draws between things.
  'question-rule-faint': { site: 'colour-roles', css: ".y-mnote[data-kind='question']:not([data-state='resolved']){border-inline-start-color:var(--ochre-line) !important}" },
  // A lit highlight that is washed more and underlined by nothing.
  'lit-highlight-without-rule': { site: 'colour-roles', css: '.y-mark.is-lit{box-shadow:none !important}' },
  // The line from a lit note to its words in indigo, whatever the mark is.
  'connector-always-indigo': { site: 'colour-roles', css: '.y-mnote[data-lifted].is-lit::before{background:var(--reader) !important}' },
  // The Done button in the reader's indigo.
  'done-in-indigo': { site: 'colour-roles', css: '.y-mfield .y-mfield__done{background:var(--reader) !important}' },
  // The word that undoes in the reader's indigo.
  'undo-in-indigo': { site: 'colour-roles', css: '.y-mstatus .y-mstatus__act{color:var(--reader) !important}' },
  // The toolbar's hover in the reader's wash.
  'toolbar-hover-in-indigo': { site: 'colour-roles', css: '.y-mbar .y-mbar__act:hover{background:var(--reader-wash) !important}' },
  // A highlight in the colour of a question.
  'highlight-in-ochre': { site: 'colour-roles', css: ":root:not([data-marks='hide']) .y-mark[data-state='highlight']{background:var(--ochre-wash) !important}" },
  // A failed write that is not told in crimson.
  'failure-in-ink': { site: 'colour-roles', css: '.y-mfield .y-mfield__said{color:var(--fg) !important}' },
  // The day under a note fainter than text may be.
  'day-too-faint': { site: 'every-word-is-readable', css: '.y-mnote .y-mnote__meta{color:var(--fg-ornament) !important}' },
  // Buttons on every level on a touch screen.
  'touch-buttons-on-every-level': { site: 'touch-section-buttons', css: '@media (pointer: coarse){.y-prose .y-sect .y-margin .y-margin__act{display:inline-flex !important}}' },
  // The line is taken out of the page when it has nothing to say, and put back
  // with its words, which a screen reader does not announce.
  'status-removed-when-idle': script("element.dataset.idle = '';", 'element.hidden = true;', 'the-status-line-stays-in-the-tree', 'marks-status.js'),
  // The notes are held out of sight in every window the margin might be in, and
  // not only where it is.
  'hidden-where-there-is-no-margin': { site: 'the-first-frame-hides-notes-only-where-the-margin-is', css: ":root[data-js='on']:not([data-marks-lifted]) .y-main > .y-article .y-prose :is(.y-mnote, .y-mfield){position:absolute !important;visibility:hidden !important}" },
  // Buttons that are too small for a finger.
  'touch-buttons-small': { site: 'touch-section-buttons', css: '@media (pointer: coarse){.y-margin .y-margin__act,.y-mbar__act{min-height:20px !important;padding-block:0 !important}}' },
};

const probe = defineProbe('marks-page', SITES, MODES);
const { runs, fire, broken } = probe;

const lifted = (page) => page.waitForFunction(() => document.documentElement.dataset.marksLifted !== undefined, null, { timeout: 5000 });

// The kinds of mark the page has to draw, each on words of its own.
const KINDS = [
  { name: 'highlight', body: { kind: 'highlight', quote: 'Iota passage ends it' }, label: '劃線' },
  { name: 'thought', body: { kind: 'thought', quote: 'Xi passage is a short one', body: 'a short thought' }, label: '想法' },
  { name: 'question', body: { kind: 'question', quote: 'Kappa passage' }, label: '疑問' },
  { name: 'answered', body: { kind: 'question', quote: 'Lambda passage' }, label: '已解決的疑問', resolve: true },
  { name: 'cjk', body: { kind: 'thought', quote: '標記這一段的一部分', body: '中文的想法' }, label: '想法' },
  { name: 'code', body: { kind: 'thought', quote: 'Mu passage is code and is chosen as code.', body: 'on code' }, label: '想法' },
  { name: 'section', body: { kind: 'thought', anchor: 'the-third-passage', body: 'about the whole third section' }, label: '想法', section: true },
];

await probe.run(async ({ open }) => {
  const seed = await open({ plantHere: false });
  const made = [];
  for (const kind of KINDS) {
    const mark = await keepByHand(seed.context, kind.body);
    if (kind.resolve) {
      const response = await seed.context.request.patch(`${BASE}/mark/${mark.id}`, { data: { seen: mark.seen, resolved: true } });
      if (response.status() !== 200) broken(`answering a question by hand answered ${response.status()}`);
    }
    made.push({ ...kind, id: mark.id });
  }
  await seed.close();

  // Read with no script running at all.
  if (runs('marks-read-without-script') || runs('every-mark-has-a-name')) {
    for (const width of [1512, 390]) {
      const { page, close } = await open({ width, height: 900, script: false });
      const got = await page.evaluate(
        (made) =>
          made.map((kind) => {
            const pieces = [...document.querySelectorAll(`mark[data-mark="${kind.id}"]`)];
            const note = document.getElementById(`mn-${kind.id}`);
            const last = pieces.at(-1)?.getBoundingClientRect();
            const words = note?.querySelector('.y-mnote__words');
            return {
              name: kind.name,
              pieces: pieces.length,
              text: pieces.map((p) => p.textContent).join(''),
              noteShown: Boolean(note?.checkVisibility()),
              noteWords: words?.dataset.text ?? words?.textContent ?? '',
              noteTop: note?.getBoundingClientRect().top ?? null,
              wordsBottom: last?.bottom ?? null,
              noteRight: note?.getBoundingClientRect().right ?? null,
              first: pieces[0] ? { tabindex: pieces[0].getAttribute('tabindex'), describedby: pieces[0].getAttribute('aria-describedby') } : null,
              noteLabel: note?.getAttribute('aria-label') ?? null,
              wide: document.documentElement.scrollWidth,
              inner: innerWidth,
              toolbarShown: Boolean(document.querySelector('[data-marks-toolbar]')?.checkVisibility()),
              statusShown: Boolean(document.querySelector('[data-marks-status]:not([data-idle])')?.checkVisibility()),
              sectionButtonEnabled: [...document.querySelectorAll('[data-section-thought]')].some((b) => !b.disabled),
            };
          }),
        made,
      );
      if (runs('marks-read-without-script')) {
        for (const row of got) {
          const kind = made.find((k) => k.name === row.name);
          if (kind.section) {
            if (!row.noteShown) fire('marks-read-without-script', `at ${width}px with no script the thought on a whole section is not drawn`);
            continue;
          }
          if (row.pieces < 1) fire('marks-read-without-script', `at ${width}px with no script the ${row.name} is not washed in the page the server sent`);
          if (row.text !== kind.body.quote) fire('marks-read-without-script', `at ${width}px the ${row.name} holds ${JSON.stringify(row.text)}, want exactly ${JSON.stringify(kind.body.quote)}`);
          if (kind.body.body) {
            if (!row.noteShown || row.noteWords !== kind.body.body) fire('marks-read-without-script', `at ${width}px with no script the ${row.name}'s note says ${JSON.stringify(row.noteWords)} (shown ${row.noteShown}), want ${JSON.stringify(kind.body.body)}`);
          }
          if (row.noteShown && row.noteTop < row.wordsBottom - 1) fire('marks-read-without-script', `at ${width}px the ${row.name}'s note stands above the end of its words`);
          if (row.noteShown && row.noteRight > row.inner + 0.5) fire('marks-read-without-script', `at ${width}px the ${row.name}'s note reaches past the window`);
        }
        if (got[0].wide > got[0].inner) fire('marks-read-without-script', `at ${width}px the page is ${got[0].wide}px wide in a ${got[0].inner}px window`);
        if (got[0].toolbarShown || got[0].statusShown) fire('marks-read-without-script', `with no script the toolbar (${got[0].toolbarShown}) or the status line (${got[0].statusShown}) is drawn: nothing there can answer`);
        if (got[0].sectionButtonEnabled) fire('marks-read-without-script', 'with no script a button to write a thought is enabled, and nothing answers it');
      }
      if (runs('every-mark-has-a-name') && width === 1512) {
        for (const row of got) {
          const kind = made.find((k) => k.name === row.name);
          if (kind.section) {
            if (!row.noteLabel?.startsWith(kind.label)) fire('every-mark-has-a-name', `the thought on a whole section is named ${JSON.stringify(row.noteLabel)}, want a name that begins with ${kind.label}`);
            continue;
          }
          // The name a screen reader is given, read from the accessibility tree
          // and not from the markup that was meant to give it.
          const ax = await axOf(page, `mark[data-mark="${kind.id}"]`);
          const name = ax?.name ?? '';
          if (!ax || ax.ignored || ax.role !== 'mark') fire('every-mark-has-a-name', `the ${row.name} is ${JSON.stringify(ax)} in the accessibility tree, want a mark that is not left out of it`);
          if (!name.startsWith(kind.label)) fire('every-mark-has-a-name', `the ${row.name} is named ${JSON.stringify(name)}, want a name that begins with its state, ${kind.label}`);
          if (row.first.tabindex !== '0') fire('every-mark-has-a-name', `the first piece of the ${row.name} has tabindex ${row.first.tabindex}, want 0 so that a keyboard meets it`);
          if (kind.body.body && row.first.describedby !== `mn-${kind.id}`) fire('every-mark-has-a-name', `the ${row.name} is described by ${row.first.describedby}, want its note`);
        }
        console.log('marks-page: every kind of mark is in the page with no script, each with a name that begins with its state');
      }
      if (runs('marks-read-without-script') && width === 1512) console.log('marks-page: with no script every mark is washed and every note stands after its words, in a window no wider than itself');
      await close();
    }
  }

  // The keyboard walks the marks in the order of the text.
  if (runs('braces-walk-the-marks')) {
    const { page, close } = await open({ width: 1512, height: 900 });
    await lifted(page);
    const tops = [];
    for (let i = 0; i < 4; i++) {
      await page.keyboard.press('}');
      const at = await page.evaluate(() => {
        const element = document.activeElement.closest('mark.y-mark, .y-mnote');
        return element ? element.getBoundingClientRect().top + scrollY : null;
      });
      if (at === null) fire('braces-walk-the-marks', `the ${i + 1}th press of } left the focus on ${await page.evaluate(() => document.activeElement.tagName)}, want a mark or a note`);
      tops.push(at);
    }
    for (let i = 1; i < tops.length; i++) {
      if (tops[i] < tops[i - 1] - 0.5) fire('braces-walk-the-marks', `} went from a mark ${tops[i - 1]}px down the page to one ${tops[i]}px down it: it walks the text forwards`);
    }
    await page.keyboard.press('{');
    const back = await page.evaluate(() => document.activeElement.closest('mark.y-mark, .y-mnote')?.getBoundingClientRect().top + scrollY);
    if (!(back < tops.at(-1) + 0.5)) fire('braces-walk-the-marks', '{ did not go back to the mark before');
    console.log('marks-page: } walks the marks forwards in the order of the text, { back');
    await close();
  }

  // m hides and shows, and is kept.
  if (runs('m-hides-and-shows')) {
    const { page, close } = await open({ width: 1512, height: 900 });
    await lifted(page);
    const state = () =>
      page.evaluate((id) => {
        const piece = document.querySelector(`mark[data-mark="${id}"]`);
        const note = document.getElementById(`mn-${id}`);
        const style = getComputedStyle(piece);
        return {
          stamp: document.documentElement.dataset.marks,
          wash: style.backgroundColor,
          underline: style.textDecorationLine,
          tabindex: piece.tabIndex,
          noteShown: Boolean(note?.checkVisibility()),
          cookie: document.cookie,
        };
      }, made[1].id);
    const shown = await state();
    if (!shown.noteShown || shown.wash === 'rgba(0, 0, 0, 0)') broken('the marks are not drawn before m is pressed');
    await page.keyboard.press('m');
    await page.waitForFunction(() => document.documentElement.dataset.marks === 'hide', null, { timeout: 2000 }).catch(() => {});
    const hidden = await state();
    if (hidden.stamp !== 'hide') fire('m-hides-and-shows', `after m the page is stamped ${JSON.stringify(hidden.stamp)}, want hide`);
    if (hidden.noteShown) fire('m-hides-and-shows', 'after m a note is still drawn');
    if (hidden.wash !== 'rgba(0, 0, 0, 0)') fire('m-hides-and-shows', `after m a passage is still washed ${hidden.wash}`);
    if (hidden.tabindex !== -1) fire('m-hides-and-shows', `after m a keyboard still stops on a hidden mark (tabindex ${hidden.tabindex})`);
    if (!hidden.cookie.includes('yomihon_marks=hide')) fire('m-hides-and-shows', `after m the choice is not kept in the cookie the server reads: ${hidden.cookie}`);
    // } while they are hidden says so, with the word that shows them.
    await page.keyboard.press('}');
    const line = await page.locator('[data-marks-status]').innerText();
    if (!line.includes('文中留下的隱藏著') || !line.includes('顯示')) fire('m-hides-and-shows', `} with the marks hidden says ${JSON.stringify(line)}, want that they are hidden and the word that shows them`);
    await page.click('[data-mstatus-act]');
    if ((await state()).stamp !== 'show') fire('m-hides-and-shows', 'the word on the line does not show the marks');
    await page.keyboard.press('m');
    await page.keyboard.press('m');
    const again = await state();
    if (again.stamp !== 'show' || !again.noteShown || again.tabindex !== 0 || again.wash === 'rgba(0, 0, 0, 0)') fire('m-hides-and-shows', `after m twice the marks are ${JSON.stringify(again)}, want them drawn as at first`);
    // Typing an m in a field is typing.
    await choose(page, 'Nu passage opens the third section');
    await page.keyboard.press('h');
    await page.locator('.y-mfield textarea').waitFor({ timeout: 3000 });
    await page.keyboard.type('mmm');
    if ((await page.locator('.y-mfield textarea').inputValue()) !== 'mmm' || (await state()).stamp !== 'show') fire('m-hides-and-shows', 'an m typed in a field is read as the key that hides the marks');
    await page.keyboard.press('Escape');
    console.log('marks-page: m hides every mark, } says so with the word that shows them, m shows them again, and typing an m is only typing');
    await close();
  }

  // The choice is read by the server.
  if (runs('the-choice-is-in-the-first-frame')) {
    const { page, close } = await open({ width: 1512, height: 900, script: false, cookies: { yomihon_marks: 'hide' } });
    const got = await page.evaluate((id) => {
      const piece = document.querySelector(`mark[data-mark="${id}"]`);
      return { stamp: document.documentElement.dataset.marks, wash: getComputedStyle(piece).backgroundColor, note: Boolean(document.getElementById(`mn-${id}`)?.checkVisibility()) };
    }, made[1].id);
    if (got.stamp !== 'hide') fire('the-choice-is-in-the-first-frame', `a page asked for with the cookie says ${JSON.stringify(got.stamp)} on its root, want hide: the server reads the choice`);
    if (got.note || got.wash !== 'rgba(0, 0, 0, 0)') fire('the-choice-is-in-the-first-frame', `a page that arrives with the marks hidden draws a note (${got.note}) or a wash (${got.wash}) before any script runs`);
    console.log('marks-page: a page that arrives with the marks hidden is drawn without them before any script runs');
    await close();
  }

  // The line the changes are said in is always there for a screen reader, empty:
  // a region that comes into the page with its words is not announced.
  if (runs('the-status-line-stays-in-the-tree')) {
    const { page, close } = await open({ width: 1512, height: 900 });
    await lifted(page);
    const regions = async () => ({ said: await axOf(page, '[data-mstatus-said]'), alert: await axOf(page, '[data-mstatus-alert]') });
    let now = await regions();
    if (!now.said || now.said.ignored || now.said.role !== 'status' || !now.alert || now.alert.ignored || now.alert.role !== 'alert') fire('the-status-line-stays-in-the-tree', `before anything is said the regions are ${JSON.stringify(now)} in the accessibility tree, want a status and an alert that are not left out of it`);
    const box = () => page.locator('[data-marks-status]').evaluate((element) => ({ width: element.getBoundingClientRect().width, height: element.getBoundingClientRect().height, text: element.textContent.replace(/\s+/g, '') }));
    const idle = await box();
    if (idle.width > 2 || idle.height > 2 || idle.text !== '') fire('the-status-line-stays-in-the-tree', `before anything is said the line is ${JSON.stringify(idle)}, want nothing drawn and nothing in it`);
    await page.keyboard.press('m');
    await page.keyboard.press('}');
    await page.waitForFunction(() => document.querySelector('[data-mstatus-said]').textContent !== '', null, { timeout: 2000 }).catch(() => {});
    const said = await box();
    if (said.width < 100 || !said.text) fire('the-status-line-stays-in-the-tree', `after something is said the line is ${JSON.stringify(said)}, want it drawn with its words`);
    await page.click('[data-mstatus-close]');
    now = await regions();
    if (!now.said || now.said.ignored || !now.alert || now.alert.ignored) fire('the-status-line-stays-in-the-tree', `after the line is dismissed the regions are ${JSON.stringify(now)} in the accessibility tree, want them still there, empty`);
    const after = await box();
    if (after.width > 2 || after.height > 2 || after.text !== '') fire('the-status-line-stays-in-the-tree', `after the line is dismissed it is ${JSON.stringify(after)}, want nothing drawn`);
    console.log('marks-page: the status and alert regions stay in the accessibility tree, empty and undrawn, and are drawn with their words');
    await close();
  }

  // Before the script has set the notes in the margin they are kept out of sight,
  // where the margin is, and nowhere else: a window with no room for one shows
  // them after their paragraphs at once, and does not hold them back.
  if (runs('the-first-frame-hides-notes-only-where-the-margin-is')) {
    for (const [width, size, hides] of [[1512, '', true], [1330, 'xl', false]]) {
      const { page, close } = await open({ width, height: 900, size, settle: false, before: (page) => page.route(/\/static\/marks[a-z-]*\.js(\?.*)?$/, (route) => route.abort()) });
      const got = await page.evaluate(() => {
        const notes = [...document.querySelectorAll('.y-mnote')];
        return { count: notes.length, hidden: notes.filter((n) => getComputedStyle(n).visibility === 'hidden').length, lifted: document.documentElement.dataset.marksLifted !== undefined, js: document.documentElement.dataset.js };
      });
      if (got.js !== 'on' || got.lifted || got.count < 3) broken(`at ${width}px the page is ${JSON.stringify(got)}, want a page with script that has not set its notes`);
      if (hides && got.hidden !== got.count) fire('the-first-frame-hides-notes-only-where-the-margin-is', `at ${width}px, where the margin is, ${got.hidden} of ${got.count} notes are held out of sight before the script sets them, want all`);
      if (!hides && got.hidden > 0) fire('the-first-frame-hides-notes-only-where-the-margin-is', `at ${width}px at text size ${size}, with no room for a margin, ${got.hidden} of ${got.count} notes are held out of sight, want them shown after their paragraphs`);
      await close();
    }
    console.log('marks-page: before the script has run the notes are held out of sight where the margin is and shown where it is not');
  }

  // The colours.
  if (runs('colour-roles') || runs('every-word-is-readable')) {
    for (const theme of ['light', 'dark']) {
      const { page, close } = await open({ width: 1512, height: 900, theme });
      await lifted(page);
      const token = (name) => resolveColour(page, `var(${name})`);
      const tokens = { ink: await token('--fg'), reader: await token('--reader'), readerLine: await token('--reader-line'), readerWashStrong: await token('--reader-wash-strong'), readerWash: await token('--reader-wash'), ochre: await token('--ochre'), ochreLine: await token('--ochre-line'), ochreWash: await token('--ochre-wash'), crimson: await token('--crimson'), crimsonWash: await token('--crimson-wash'), line: await token('--line-strong') };
      const ochres = new Set([tokens.ochre, tokens.ochreLine, tokens.ochreWash, await token('--ochre-wash-strong')]);
      const crimsons = new Set([tokens.crimson, tokens.crimsonWash]);
      const paint = await page.evaluate(
        ({ made }) => {
          const props = ['color', 'backgroundColor', 'borderInlineStartColor', 'borderBlockStartColor', 'borderBlockEndColor', 'textDecorationColor', 'outlineColor', 'caretColor'];
          const used = (element) => {
            const style = getComputedStyle(element);
            const out = [];
            for (const prop of props) {
              if (prop === 'textDecorationColor' && style.textDecorationLine === 'none') continue;
              if (prop === 'borderInlineStartColor' && style.borderInlineStartWidth === '0px') continue;
              if (prop.startsWith('borderBlock') && style[prop.replace('Color', 'Width')] === '0px') continue;
              if (prop === 'outlineColor' && style.outlineStyle === 'none') continue;
              out.push([prop, style[prop]]);
            }
            return out;
          };
          const rows = [];
          for (const kind of made) {
            for (const element of [...document.querySelectorAll(`mark[data-mark="${kind.id}"]`), document.getElementById(`mn-${kind.id}`)]) {
              if (!element) continue;
              const state = element.dataset.state;
              rows.push({ name: kind.name, state, tag: element.tagName.toLowerCase(), used: used(element) });
              for (const inner of element.querySelectorAll('*')) rows.push({ name: kind.name, state, tag: inner.tagName.toLowerCase(), used: used(inner) });
            }
          }
          return rows;
        },
        { made },
      );
      const family = (set, value) => [...set].some((c) => c === value);
      if (runs('colour-roles')) {
        for (const row of paint) {
          for (const [prop, value] of row.used) {
            if (family(crimsons, value)) fire('colour-roles', `in ${theme} the ${row.name} (${row.tag}) is drawn in crimson on ${prop}; crimson is for a failed write and nothing else`);
            if (family(ochres, value) && row.state !== 'question') fire('colour-roles', `in ${theme} the ${row.name} (${row.tag}, state ${row.state}) is drawn in ochre on ${prop}; ochre is for a question that is open`);
          }
        }
        const asking = paint.filter((row) => row.state === 'question' && row.tag === 'mark');
        if (!asking.some((row) => row.used.some(([prop, value]) => prop === 'textDecorationColor' && value === tokens.ochre))) fire('colour-roles', `in ${theme} an open question is not underlined in ochre`);
        const noteRule = paint.find((row) => row.state === 'question' && row.tag === 'aside');
        const ruleOf = (row) => row?.used.find(([prop]) => prop === 'borderInlineStartColor')?.[1];
        if (noteRule && ruleOf(noteRule) !== tokens.ochre) fire('colour-roles', `in ${theme} the rule down an open question's note is ${ruleOf(noteRule)}, want ochre ${tokens.ochre}, as strong as the underline under its words`);
        const thoughtRule = paint.find((row) => row.name === 'thought' && row.tag === 'aside');
        if (ruleOf(thoughtRule) !== tokens.reader) fire('colour-roles', `in ${theme} the rule down a thought's note is ${ruleOf(thoughtRule)}, want the reader's indigo ${tokens.reader}, as strong as the wash on its words`);
        const answeredRule = paint.find((row) => row.state === 'resolved' && row.tag === 'aside');
        if (ruleOf(answeredRule) !== tokens.line) fire('colour-roles', `in ${theme} the rule down an answered question's note is ${ruleOf(answeredRule)}, want the line colour ${tokens.line}`);
        // A mark that is lit says so on its words and not in the wash alone.
        const litShadow = await page.evaluate(() => {
          const piece = document.querySelector('mark[data-kind="highlight"]');
          piece.classList.add('is-lit');
          const shadow = getComputedStyle(piece).boxShadow;
          piece.classList.remove('is-lit');
          return shadow;
        });
        if (!litShadow.includes(tokens.reader)) fire('colour-roles', `in ${theme} a lit highlight has the shadow ${JSON.stringify(litShadow)}, want a line under it in the reader's indigo ${tokens.reader}`);
        // The line from a lit note to its words, and the rule that says it was pushed
        // down, follow the state of the mark.
        const links = await page.evaluate(() => {
          const out = [];
          for (const kind of ['thought', 'question', 'resolved']) {
            const note = document.querySelector(`.y-mnote[data-state="${kind}"]`);
            if (!note) continue;
            const was = [note.hasAttribute('data-lifted'), note.hasAttribute('data-displaced')];
            note.setAttribute('data-lifted', '');
            note.setAttribute('data-displaced', '');
            note.classList.add('is-lit');
            out.push({ kind, connector: getComputedStyle(note, '::before').backgroundColor, tick: getComputedStyle(note, '::after').backgroundColor });
            note.classList.remove('is-lit');
            if (!was[0]) note.removeAttribute('data-lifted');
            if (!was[1]) note.removeAttribute('data-displaced');
          }
          return out;
        });
        const wantLink = { thought: tokens.reader, question: tokens.ochre, resolved: tokens.line };
        if (links.length < 3) broken(`only ${links.length} of the three states of a note are drawn`);
        for (const link of links) {
          if (link.connector !== wantLink[link.kind]) fire('colour-roles', `in ${theme} the line from a lit ${link.kind} note to its words is ${link.connector}, want ${wantLink[link.kind]}`);
          if (link.tick !== wantLink[link.kind]) fire('colour-roles', `in ${theme} the rule that says a ${link.kind} note was pushed down is ${link.tick}, want ${wantLink[link.kind]}`);
        }
        const answered = paint.filter((row) => row.state === 'resolved');
        if (answered.length === 0) broken('the answered question is not drawn as one');
        for (const kind of ['highlight', 'thought']) {
          const rows = paint.filter((row) => row.name === (kind === 'highlight' ? 'highlight' : 'thought') && row.tag === 'mark');
          if (!rows.some((row) => row.used.some(([prop, value]) => prop === 'backgroundColor' && value === tokens.readerWash))) fire('colour-roles', `in ${theme} the ${kind} is not washed in the reader's own blue`);
        }
        // A write that fails is crimson, in the field it was written in.
        const second = await open({ width: 1512, height: 900, theme });
        const failing = second.page;
        await failing.route('**/mark/*', (route) => (route.request().method() === 'PATCH' ? route.fulfill({ status: 500, contentType: 'text/plain', body: 'no' }) : route.fallback()));
        await choose(failing, 'Omicron passage is the last of the note');
        await failing.keyboard.press('h');
        await failing.locator('.y-mfield textarea').waitFor({ timeout: 3000 });
        await failing.keyboard.type('refused');
        await failing.waitForSelector('.y-mfield__said:not([hidden])', { timeout: 4000 });
        const said = await failing.evaluate(() => getComputedStyle(document.querySelector('.y-mfield__said')).color);
        if (said !== tokens.crimson) fire('colour-roles', `in ${theme} a failed write is said in ${said}, want crimson ${tokens.crimson}`);
        // Indigo is the reader's own marks and the focus ring, and not the buttons:
        // the Done button is ink.
        const done = await failing.evaluate(() => getComputedStyle(document.querySelector('.y-mfield__done')).backgroundColor);
        if (done !== tokens.ink) fire('colour-roles', `in ${theme} the Done button is ${done}, want ink ${tokens.ink}: indigo is for the reader's own marks`);
        await failing.keyboard.press('Escape');
        await second.close();
        // The word that undoes, and the toolbar under a pointer.
        const third = await open({ width: 1512, height: 900, theme });
        await lifted(third.page);
        await third.page.keyboard.press('m');
        await third.page.keyboard.press('}');
        await third.page.waitForSelector('[data-mstatus-act]:not([hidden])', { timeout: 3000 });
        const act = await third.page.evaluate(() => getComputedStyle(document.querySelector('[data-mstatus-act]')).color);
        if (act !== tokens.ink) fire('colour-roles', `in ${theme} the word on the status line that does what it says is ${act}, want ink ${tokens.ink}`);
        await third.page.keyboard.press('m');
        await choose(third.page, 'Theta passage opens the second section');
        await third.page.hover('[data-mbar="highlight"]');
        const hover = await third.page.evaluate(() => getComputedStyle(document.querySelector('[data-mbar="highlight"]')).backgroundColor);
        if ([tokens.reader, tokens.readerWash, tokens.readerWashStrong, tokens.readerLine].includes(hover)) fire('colour-roles', `in ${theme} the toolbar under a pointer is washed ${hover}, in the reader's indigo: that is for the reader's marks`);
        await third.close();
      }
      if (runs('every-word-is-readable')) {
        const probes = await page.evaluate(() => {
          const backdrop = (element) => {
            const layers = [];
            for (let at = element; at; at = at.parentElement) {
              const colour = getComputedStyle(at).backgroundColor;
              layers.push(colour);
              if (/^rgb\(/.test(colour) || /, 1\)$/.test(colour)) break;
            }
            return layers.reverse();
          };
          const pick = (selector) => [...document.querySelectorAll(selector)].filter((e) => e.checkVisibility()).slice(0, 2);
          const rows = [];
          for (const [what, selector] of [
            ['a note\'s words', '.y-mnote .y-mnote__words'],
            ['the day under a note', '.y-mnote .y-mnote__meta'],
            ['a passage with a mark on it', 'mark.y-mark'],
          ]) {
            for (const element of pick(selector)) rows.push({ what, colour: getComputedStyle(element).color, layers: backdrop(element) });
          }
          return rows;
        });
        const over = (layers) => {
          // The layers are composited from the bottom, each over what is under it.
          let base = [255, 255, 255];
          for (const layer of layers) {
            const parts = layer.match(/[\d.]+/g)?.map(Number) ?? [0, 0, 0, 0];
            const alpha = parts.length > 3 ? parts[3] : 1;
            base = base.map((c, i) => Math.round(parts[i] * alpha + c * (1 - alpha)));
          }
          return base;
        };
        for (const row of probes) {
          const ratio = contrast(channels(row.colour), over(row.layers));
          if (ratio < 4.5) fire('every-word-is-readable', `in ${theme} ${row.what} reads at ${ratio.toFixed(2)} to 1, want 4.5 or more`);
        }
      }
      await close();
    }
    if (runs('colour-roles')) console.log('marks-page: blue for what the reader left, ochre only for an open question, crimson only for a failed write, in light and in dark');
    if (runs('every-word-is-readable')) console.log('marks-page: a note\'s words, its day and a marked passage read at 4.5 to 1 or more in light and in dark');
  }

  // A touch screen.
  if (runs('touch-section-buttons')) {
    const { page, close } = await open({ width: 390, height: 844, touch: true });
    const got = await page.evaluate(() =>
      [...document.querySelectorAll('.y-sect')].map((sect) => {
        const heading = sect.querySelector(':scope > [data-level]');
        const buttons = [...sect.querySelectorAll('.y-margin .y-margin__act[data-section-mark], .y-margin .y-margin__act[data-section-thought]')];
        return {
          level: heading?.dataset.level,
          text: heading?.textContent.trim(),
          shown: buttons.filter((b) => b.checkVisibility()).map((b) => ({ height: b.getBoundingClientRect().height, right: b.getBoundingClientRect().right })),
        };
      }),
    );
    const h2 = got.filter((s) => s.level === '2');
    const deeper = got.filter((s) => s.level !== '2');
    if (h2.length < 3 || deeper.length < 1) broken(`the note has ${h2.length} sections of the second level and ${deeper.length} deeper; the probe needs three and one`);
    for (const s of h2) {
      if (s.shown.length < 2) fire('touch-section-buttons', `on a touch screen the section "${s.text}" shows ${s.shown.length} buttons, want the question's and the thought's`);
      for (const b of s.shown) {
        if (b.height < 44) fire('touch-section-buttons', `on a touch screen a button in "${s.text}" is ${b.height}px tall, want 44 or more for a finger`);
        if (b.right > 390.5) fire('touch-section-buttons', `on a touch screen a button in "${s.text}" reaches ${b.right}px in a 390px window`);
      }
    }
    for (const s of deeper) {
      if (s.shown.length > 0) fire('touch-section-buttons', `on a touch screen the section "${s.text}" of a deeper level shows ${s.shown.length} buttons: only the headings of the contents are given them`);
    }
    // The toolbar over a chosen passage is a bar along the bottom a thumb reaches.
    await choose(page, 'Theta passage opens the second section');
    const bar = await page.evaluate(() => {
      const r = document.querySelector('[data-marks-toolbar]').getBoundingClientRect();
      return { left: r.left, right: r.right, bottom: r.bottom, height: r.height, fixed: getComputedStyle(document.querySelector('[data-marks-toolbar]')).position };
    });
    if (bar.fixed !== 'fixed' || bar.bottom > 844 || bar.left < 0 || bar.right > 390.5) fire('touch-section-buttons', `on a phone the toolbar is ${JSON.stringify(bar)}, want a bar along the bottom that stays inside the window`);
    if ((await page.evaluate(() => document.documentElement.scrollWidth)) > 390) fire('touch-section-buttons', 'on a phone the marks make the page wider than the window');
    console.log('marks-page: a touch screen is given 44px buttons on the headings of the contents and none on deeper ones, and a toolbar along the bottom');
    await close();
  }
});
