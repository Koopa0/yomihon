// Browser lock for what a reader selects and copies across a ruby. The page
// does not let a reading be selected, so a selection across a ruby is its base
// text, and the server places a highlight by exactly that. What only a browser
// can show is the rest: that a triple click selects the whole paragraph instead
// of stopping at the first reading, that a copy carries the readings the author
// wrote in full-width parentheses and only where a base is covered in full,
// and that all of it is the same with the readings shown and hidden.
//
// Every expectation is authored here, apart from the page, and the clipboard is
// the browser's own: a copy is made with the keyboard and read back.
//
// Env: YOMIHON_BASE, PAGE_PATH (the note whose paragraphs this reads).
import { arrived } from './support/arrival.mjs';
import { chromium } from 'playwright-core';

import { baseUrl as BASE } from './support/base.mjs';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/ruby-selection.md';
const NOTE = 'Notes/ruby-selection.md';

const SNOW = '国境の長いトンネルを抜けると雪国であった。夜の底が白くなった。';
const SNOW_WITH_READING = '国境の長いトンネルを抜けると雪国（ゆきぐに）であった。夜の底が白くなった。';
const STATION = '東京駅と仮名を読む。';
const STATION_WITH_READINGS = '東京（とうきょう）駅（えき）と仮名を読む。';
const CONTROL = '沒有讀音的一段中文，作為對照。';
// A place in a paragraph: the character at offset of the text node whose
// words are text.
const at = (text, offset) => ({ text, offset });
const BEFORE = '国境の長いトンネルを抜けると';
const BASE_OF_SNOW = '雪国';
const AFTER = 'であった。夜の底が白くなった。';

class Fired extends Error {}
const check = (ok, message) => {
  if (!ok) throw new Fired(message);
};
const same = (got, want, what) => check(got === want, `${what}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);

const paragraphWith = (page, words) => page.locator('.y-prose p', { hasText: words });

// Where a character is on the screen, to click on it.
const pointAt = async (paragraph, place) => {
  await paragraph.scrollIntoViewIfNeeded();
  return paragraph.evaluate((p, { text, offset }) => {
    const walker = document.createTreeWalker(p, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      if (node.data.trim() !== text) continue;
      const range = document.createRange();
      range.setStart(node, offset);
      range.setEnd(node, offset + 1);
      const box = range.getBoundingClientRect();
      return { x: box.left + box.width / 2, y: box.top + box.height / 2 };
    }
    throw new Error(`no text node of ${text}`);
  }, place);
};

const tripleClick = async (page, paragraph, place) => {
  const point = await pointAt(paragraph, place);
  await page.mouse.click(point.x, point.y, { clickCount: 3 });
};

// Selects from one place to another within a paragraph. A selection made from a
// later place to an earlier one is the reader's selecting backwards.
const select = (paragraph, from, to) =>
  paragraph.evaluate((p, [start, end]) => {
    const find = ({ text, offset }) => {
      const walker = document.createTreeWalker(p, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (node.data.trim() === text) return [node, offset];
      }
      throw new Error(`no text node of ${text}`);
    };
    const [anchor, anchorOffset] = find(start);
    const [focus, focusOffset] = find(end);
    getSelection().setBaseAndExtent(anchor, anchorOffset, focus, focusOffset);
  }, [from, to]);

// The paragraph as a selection of all of its children, which is what a triple
// click is asked to leave.
const selectAll = (paragraph) => paragraph.evaluate((p) => getSelection().selectAllChildren(p));

const selected = (page) => page.evaluate(() => getSelection().toString());

// The state of a selection, for what a copy must leave as it found it.
const selectionState = (page) =>
  page.evaluate(() => {
    const s = getSelection();
    const path = (node) => {
      const steps = [];
      for (let n = node; n && n !== document; n = n.parentNode) steps.push([...n.parentNode.childNodes].indexOf(n));
      return steps.join('.');
    };
    return { anchor: [path(s.anchorNode), s.anchorOffset], focus: [path(s.focusNode), s.focusOffset], text: s.toString() };
  });

// copy makes the reader's copy and says what the clipboard holds then. The
// clipboard is filled first with words nobody selected, so a copy that never
// happened is not mistaken for one that wrote nothing.
const copy = async (page) => {
  await page.evaluate(() => navigator.clipboard.writeText('stale'));
  await page.keyboard.press('ControlOrMeta+C');
  return page.evaluate(async () => {
    const out = { plain: await navigator.clipboard.readText(), markup: null };
    for (const item of await navigator.clipboard.read()) {
      if (item.types.includes('text/html')) out.markup = await (await item.getType('text/html')).text();
    }
    return out;
  });
};

const copiedText = async (page) => (await copy(page)).plain;

// keep is what the server says to a highlight of the words a reader selected:
// kept, and placed, at the words, which is what a reader who selected across a
// ruby asks of it. The mark is taken away again, so the next one is kept anew.
const keep = async (page, quote) => {
  const sent = await page.evaluate(
    async ([path, words]) => {
      const response = await fetch('/mark', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ kind: 'highlight', path, anchor: '', quote: words, before: '', after: '', body: '' }),
      });
      return { status: response.status, body: await response.json().catch(() => null) };
    },
    [NOTE, quote],
  );
  const removed =
    sent.status === 201
      ? await page.evaluate(
          async ([id, seen]) =>
            (await fetch(`/mark/${id}`, { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ seen }) })).status,
          [sent.body.id, sent.body.seen],
        )
      : null;
  check(sent.status === 201, `a highlight of ${JSON.stringify(quote)} was answered ${sent.status}, want 201: the server does not find the words the reader selected`);
  same(sent.body.placement, 'found', `placement of the highlight of ${JSON.stringify(quote)}`);
  same(sent.body.quote, quote, 'the words kept');
  same(removed, 200, 'removing the mark this probe kept');
};

const toggleReadings = async (page, want) => {
  const toggle = page.locator('[data-ruby-toggle]');
  if ((await page.locator('html').getAttribute('data-ruby')) !== want) await toggle.click();
  await page.waitForFunction((state) => document.documentElement.dataset.ruby === state, want);
  const visibility = await page.locator('.y-prose rt').evaluateAll((nodes) => [...new Set(nodes.map((n) => getComputedStyle(n).visibility))]);
  same(visibility.join(), want === 'off' ? 'hidden' : 'visible', `the readings are ${want}`);
};

// What this reads: the paragraphs of the note, and the three things asked of
// them. Each is run on its own and every one that fails is reported, so a run
// against a build that has fixed one of them says which.
const paragraphs = (page) => ({
  snow: paragraphWith(page, '国境'),
  station: paragraphWith(page, '駅'),
  control: paragraphWith(page, '沒有讀音'),
  list: page.locator('.y-prose li', { hasText: '辞書' }),
  nested: page.locator('.y-prose li li', { hasText: '図書館' }),
});

// A triple click selects the whole paragraph wherever in it the click is:
// before the ruby, on its base, after it, and on the reading itself.
const tripleClickSelectsTheParagraph = async (page, state) => {
  const { snow, station, control, list, nested } = paragraphs(page);
  const userSelect = await page.locator('.y-prose rt').first().evaluate((n) => getComputedStyle(n).userSelect);
  same(userSelect, 'none', `${state}: the reading cannot be selected`);
  const clicks = [
    ['before the ruby', at(BEFORE, 1)],
    ['on the base', at(BASE_OF_SNOW, 0)],
    ['after the ruby', at(AFTER, 2)],
  ];
  if (state === 'shown') clicks.push(['on the reading', null, await snow.locator('rt').boundingBox()]);
  for (const [where, place, box] of clicks) {
    if (place) await tripleClick(page, snow, place);
    else await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2, { clickCount: 3 });
    same(await selected(page), SNOW, `${state}: a triple click ${where} selects`);
    const covers = await snow.evaluate((p) => {
      const range = getSelection().getRangeAt(0);
      return range.startContainer === p && range.startOffset === 0 && range.endContainer === p && range.endOffset === p.childNodes.length;
    });
    check(covers, `${state}: a triple click ${where} did not select every child of the paragraph`);
  }
  await tripleClick(page, station, at('と', 0));
  same(await selected(page), STATION, `${state}: a triple click in a paragraph with a ruby of two pairs and one with no reading`);
  // A control inside a paragraph is acted on by a click, not selected by it.
  await station.evaluate((p) => {
    const control = document.createElement('button');
    control.type = 'button';
    control.id = 'probe-control';
    control.innerHTML = '<ruby>漢<rt>かん</rt></ruby>字';
    p.append(control);
  });
  const pressed = await station.locator('#probe-control').boundingBox();
  await page.mouse.click(pressed.x + pressed.width / 2, pressed.y + pressed.height / 2, { clickCount: 3 });
  const covers = await station.evaluate((p) => {
    const range = getSelection().rangeCount ? getSelection().getRangeAt(0) : null;
    return range !== null && range.startContainer === p && range.startOffset === 0 && range.endOffset === p.childNodes.length;
  });
  await station.evaluate(() => document.getElementById('probe-control').remove());
  check(!covers, `${state}: a triple click on a button in a paragraph selected the paragraph`);
  await tripleClick(page, control, at(CONTROL, 3));
  // The browser's own triple click carries the line break after the paragraph.
  same((await selected(page)).trimEnd(), CONTROL, `${state}: a triple click in a paragraph with no ruby`);
  await tripleClick(page, list, at('を引く。', 1));
  same(await selected(page), '辞書を引く。', `${state}: a triple click on a list item that has a list nested in it`);
  await tripleClick(page, nested, at('へ行く。', 1));
  same(await selected(page), '図書館へ行く。', `${state}: a triple click on the nested item`);
};

// The words that are selected are the words the server finds, whether the
// readings are shown or hidden.
const theServerFindsWhatIsSelected = async (page, state) => {
  const { snow, station } = paragraphs(page);
  await selectAll(snow);
  same(await selected(page), SNOW, `${state}: the paragraph is selected`);
  await keep(page, await selected(page));
  await select(snow, at(BEFORE, 10), at(AFTER, 4));
  same(await selected(page), '抜けると雪国であった', `${state}: a selection across the ruby`);
  await keep(page, await selected(page));
  await select(station, at('東京', 1), at('と', 0));
  same(await selected(page), '京駅', `${state}: a selection across two pairs`);
  await keep(page, await selected(page));
};

// What is copied is the same whichever state the readings are in.
const copyCarriesTheReadings = async (page, state) => {
  const { snow, station, control } = paragraphs(page);
  await selectAll(snow);
  same(await copiedText(page), SNOW_WITH_READING, `${state}: a copy of the paragraph, whose ruby has brackets`);
  await selectAll(station);
  same(await copiedText(page), STATION_WITH_READINGS, `${state}: a copy of a ruby with two pairs, and one with no reading`);

  for (const [what, from, to, want] of [
    ['a base covered in full, to the end of it', at(BEFORE, 10), at(BASE_OF_SNOW, 2), '抜けると雪国（ゆきぐに）'],
    ['a base covered in full, from the start of it', at(BASE_OF_SNOW, 0), at(AFTER, 2), '雪国（ゆきぐに）であ'],
    ['a base covered in part at its end', at(BEFORE, 10), at(BASE_OF_SNOW, 1), '抜けると雪'],
    ['a base covered in part at its start', at(BASE_OF_SNOW, 1), at(AFTER, 4), '国であった'],
    ['inside a base', at(BASE_OF_SNOW, 0), at(BASE_OF_SNOW, 1), '雪'],
    ['no ruby at all', at(AFTER, 0), at(AFTER, 4), 'であった'],
    ['a selection made backwards over a ruby', at(AFTER, 2), at(BEFORE, 10), '抜けると雪国（ゆきぐに）であ'],
  ]) {
    await select(snow, from, to);
    const kept = JSON.stringify(await selectionState(page));
    same(await copiedText(page), want, `${state}: a copy of ${what}`);
    same(JSON.stringify(await selectionState(page)), kept, `${state}: the selection after a copy of ${what}`);
  }

  // Of a ruby with two pairs, a pair is written when its own base is covered:
  // the second here, with the first covered only in part.
  await select(station, at('東京', 1), at('と', 0));
  same(await copiedText(page), '京駅（えき）', `${state}: a copy that covers the second pair of a ruby and part of the first`);
  await select(station, at('東京', 0), at('東京', 2));
  same(await copiedText(page), '東京（とうきょう）', `${state}: a copy that covers the first pair of a ruby alone`);

  // A selection of the whole page, and so of every ruby on it, in the blocks
  // between them.
  await page.keyboard.press('ControlOrMeta+A');
  const everything = await copiedText(page);
  for (const words of [SNOW_WITH_READING, STATION_WITH_READINGS, '辞書（じしょ）を引く。', '図書館（としょかん）へ行く。']) {
    check(everything.includes(words), `${state}: a copy of the whole page does not hold ${JSON.stringify(words)}`);
  }

  // A copy of what holds no ruby is the browser's own, markup included.
  await selectAll(control);
  const plain = await copy(page);
  same(plain.plain, CONTROL, `${state}: a copy of a paragraph with no ruby`);
  check(plain.markup !== null, `${state}: a copy of a paragraph with no ruby lost the markup the browser writes`);

  // Where the readings are written the clipboard holds the plain text, and any
  // markup it holds does not carry them in another form.
  await selectAll(snow);
  const written = await copy(page);
  check(written.markup === null || !/\uFF08|<rt/.test(written.markup), `${state}: the markup of a copy with readings carries them: ${written.markup}`);
};

// The reader's own sequence: a triple click, then a copy.
const tripleClickThenCopy = async (page, state) => {
  const { snow, station } = paragraphs(page);
  await tripleClick(page, snow, at(AFTER, 2));
  same(await copiedText(page), SNOW_WITH_READING, `${state}: a copy after a triple click on the paragraph, whose ruby has brackets`);
  await tripleClick(page, station, at('と', 0));
  same(await copiedText(page), STATION_WITH_READINGS, `${state}: a copy after a triple click on the paragraph of a ruby with two pairs`);
};

const failures = [];
const attempt = async (name, run) => {
  try {
    await run();
  } catch (error) {
    if (!(error instanceof Fired)) throw error;
    failures.push(`${name}: ${error.message}`);
  }
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, permissions: ['clipboard-read', 'clipboard-write'] });
  const page = await context.newPage();
  const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  check(response?.status() === 200, `${PAGE}: HTTP ${response?.status()}`);
  await page.evaluate(() => document.fonts.ready);
  await arrived(page);
  for (const state of ['shown', 'hidden']) {
    await toggleReadings(page, state === 'shown' ? 'on' : 'off');
    await attempt(`triple click, readings ${state}`, () => tripleClickSelectsTheParagraph(page, state));
    await attempt(`placement, readings ${state}`, () => theServerFindsWhatIsSelected(page, state));
    await attempt(`copy, readings ${state}`, () => copyCarriesTheReadings(page, state));
    await attempt(`triple click then copy, readings ${state}`, () => tripleClickThenCopy(page, state));
  }
} catch (error) {
  if (!(error instanceof Fired)) throw error;
  failures.push(error.message);
} finally {
  await browser.close();
}
if (failures.length > 0) {
  for (const failure of failures) console.error(`FAIL ruby-selection ${failure}`);
  process.exit(1);
}
console.log('PASS ruby-selection: triple click, copy and placement of a selection across ruby, readings shown and hidden');
