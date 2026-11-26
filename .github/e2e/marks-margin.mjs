// Behavior lock for where the notes of the reader's marks stand.
//
// Where the window has room for a margin, each note stands in it beside the
// words it is about, in the column the controls beside each section use, and no
// note stands over another or over a control: they are set in the order of the
// text, each as near its words as the ones above allow, and a note that would
// have to be pushed far from its words stands after its paragraph instead. A note
// is fixed to the document, so scrolling moves it with its words. Pointing at or
// focusing a mark lights it and its note together, and the other notes step back;
// pointing at a note does the same. Where there is no room for a margin, every
// note stands after its paragraph, where the server drew it.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints every watched regression.
import { choose, defineProbe, drawn, keepByHand } from './support/marks-probe.mjs';

const SITES = [
  'notes-stand-in-the-margin-column',
  'notes-do-not-overlap',
  'notes-stay-near-their-words',
  'notes-keep-the-order-of-the-text',
  'a-note-scrolls-with-its-words',
  'a-mark-and-its-note-light-together',
  'narrow-notes-stay-after-their-paragraph',
];

const script = (needle, replacement, site, file = 'marks-layer.js') => ({ site, script: { file, needle, replacement } });
const MODES = {
  // Every note at the top of its column.
  'notes-pile-up': { site: 'notes-do-not-overlap', css: '.y-mlayer .y-mnote{top:0 !important}' },
  // A gap that is less than nothing.
  'gap-is-negative': script('const GAP = 8;', 'const GAP = -300;', 'notes-do-not-overlap'),
  // A note is never put back after its paragraph, however far it was pushed.
  'no-docking': script('const MAX_DISPLACE = 160;', 'const MAX_DISPLACE = 100000;', 'notes-stay-near-their-words'),
  // The first two notes laid out the wrong way round.
  'first-two-swapped': script('const blocks = obstacles(top);', 'if (items.length > 1) [items[0], items[1]] = [items[1], items[0]];\n    const blocks = obstacles(top);', 'notes-keep-the-order-of-the-text'),
  // The margin's column moved off the column of the controls.
  'column-off-by-a-hundred': { site: 'notes-stand-in-the-margin-column', css: '@media (min-width:1321px){.y-mlayer{margin-inline-start:100px !important}}' },
  // A note that stays where it was laid out while the page moves.
  'notes-follow-the-window': { site: 'a-note-scrolls-with-its-words', css: '.y-mlayer .y-mnote{position:fixed !important}' },
  // Pointing at a mark lights nothing.
  'pointing-lights-nothing': script("prose.addEventListener('pointerover', (event) => {", "prose.addEventListener('pointerover-never', (event) => {", 'a-mark-and-its-note-light-together'),
  // Lighting a mark lights its words and not its note.
  'lit-without-the-note': script('`[data-mark="\x24{CSS.escape(id)}"], [data-mnote="\x24{CSS.escape(id)}"]`', '`[data-mark="\x24{CSS.escape(id)}"]`', 'a-mark-and-its-note-light-together'),
  // A mark that looks the same lit and not.
  'lit-looks-the-same': { site: 'a-mark-and-its-note-light-together', css: '.y-mark,.y-mark.is-lit{background:var(--reader-wash) !important}' },
  // Without room for a margin, the notes are not drawn at all.
  'narrow-hides-the-notes': { site: 'narrow-notes-stay-after-their-paragraph', css: '@media (max-width:1320px){.y-mnote{display:none !important}}' },
};

const probe = defineProbe('marks-margin', SITES, MODES);
const { runs, fire, broken } = probe;

// Thoughts on passages that stand close together, long enough that the notes
// beside them cannot all stand level with their words.
const PASSAGES = [
  'Alpha passage opens the first section',
  'Beta passage follows it in the same paragraph',
  'Gamma passage closes the paragraph',
  'Delta passage stands in a paragraph of its own',
  'Epsilon passage ends it',
  'Zeta passage is the first item of a list',
  'Eta passage is the second item of a list',
  'Theta passage opens the second section',
  'Nu passage opens the third section',
  'Omicron passage is the last of the note',
];
const LONG = 'A thought that runs on for some length, so that the note it makes is several lines tall in a margin that is only a little wider than a short line of the text beside it.';

const lifted = (page) => page.waitForFunction(() => document.documentElement.dataset.marksLifted !== undefined, null, { timeout: 5000 });

// Where every note and every mark stands, in the document's own coordinates.
const geometry = (page) =>
  page.evaluate(() => {
    const at = (element) => {
      const r = element.getBoundingClientRect();
      return { left: r.left + scrollX, right: r.right + scrollX, top: r.top + scrollY, bottom: r.bottom + scrollY, width: r.width, height: r.height };
    };
    const prose = document.querySelector('.y-article > .y-prose');
    const layer = prose.querySelector(':scope > .y-mlayer');
    const notes = [...prose.querySelectorAll('.y-mnote[data-mnote]')]
      .filter((n) => n.checkVisibility())
      .map((n) => {
        const mark = prose.querySelector(`mark[data-mark="${n.dataset.mnote}"]`);
        const rect = mark?.getClientRects()[0];
        return {
          id: n.dataset.mnote,
          inLayer: Boolean(layer?.contains(n)),
          docked: n.hasAttribute('data-docked'),
          box: at(n),
          markTop: rect ? rect.top + scrollY : null,
          markLeft: rect ? rect.left + scrollX : null,
        };
      });
    const controls = [...prose.querySelectorAll('.y-margin')].filter((m) => m.getBoundingClientRect().height > 0).map(at);
    const margin = prose.querySelector('.y-margin');
    return {
      layerShown: layer ? getComputedStyle(layer).display !== 'none' : false,
      notes,
      controls,
      controlsLeft: margin ? at(margin).left : null,
      prose: at(prose),
      wideRead: (() => {
        const probe = document.createElement('div');
        probe.style.cssText = 'position:absolute;visibility:hidden;inline-size:var(--wide-read)';
        document.body.append(probe);
        const width = probe.getBoundingClientRect().width;
        probe.remove();
        return width;
      })(),
      sect: (() => {
        const sect = prose.querySelector('.y-sect');
        return sect ? at(sect) : null;
      })(),
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth,
    };
  });

const overlaps = (a, b) => Math.min(a.right, b.right) - Math.max(a.left, b.left) > 0.5 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 0.5;

await probe.run(async ({ open }) => {
  // Thoughts made as another tab would make them, so that the page under test
  // draws what the server sends and nothing it made itself.
  const seed = await open({ plantHere: false });
  for (const quote of PASSAGES) {
    await keepByHand(seed.context, { kind: 'thought', quote, body: `${quote.split(' ')[0]}: ${LONG}` });
  }
  await seed.close();

  for (const width of [1512, 1760]) {
    const { page, close } = await open({ width, height: 900 });
    await lifted(page);
    const g = await geometry(page);
    if (!g.layerShown) broken(`at ${width}px there is no margin layer, so the notes have nowhere to stand`);
    const stand = g.notes.filter((n) => n.inLayer);
    if (stand.length < 5) broken(`at ${width}px only ${stand.length} of the notes stand in the margin; the vault holds ${PASSAGES.length} thoughts`);

    if (runs('notes-stand-in-the-margin-column')) {
      for (const n of stand) {
        if (Math.abs(n.box.left - g.controlsLeft) > 1) fire('notes-stand-in-the-margin-column', `at ${width}px a note stands ${n.box.left}px from the left, and the controls beside the sections ${g.controlsLeft}px: they share a column`);
        if (n.box.left < g.prose.right) fire('notes-stand-in-the-margin-column', `at ${width}px a note starts at ${n.box.left}px, inside the text that ends at ${g.prose.right}px`);
      }
      if (g.scrollWidth > g.innerWidth) fire('notes-stand-in-the-margin-column', `at ${width}px the notes make the page ${g.scrollWidth}px wide in a ${g.innerWidth}px window`);
      if (width === 1760) {
        // The wide column: half the text's width past the middle, and thirty-six.
        const middle = (g.sect.left + g.sect.right) / 2;
        const wantLeft = Math.round(middle + g.wideRead / 2 + 36);
        if (Math.abs(g.controlsLeft - wantLeft) > 1) fire('notes-stand-in-the-margin-column', `at 1760px the margin column starts at ${g.controlsLeft}px, want ${wantLeft}px, half the wide measure (${g.wideRead}px) and thirty-six past the middle of the section`);
      }
      console.log(`marks-margin: at ${width}px the notes stand in the controls' column and do not widen the page`);
    }

    if (runs('notes-do-not-overlap')) {
      const things = [...stand.map((n) => ({ what: `the note ${n.id.slice(0, 6)}`, box: n.box })), ...g.controls.map((box, i) => ({ what: `the controls beside section ${i + 1}`, box }))];
      for (let i = 0; i < things.length; i++) {
        for (let j = i + 1; j < things.length; j++) {
          if (overlaps(things[i].box, things[j].box)) fire('notes-do-not-overlap', `at ${width}px ${things[i].what} and ${things[j].what} stand over each other: ${JSON.stringify(things[i].box)} and ${JSON.stringify(things[j].box)}`);
        }
      }
      console.log(`marks-margin: at ${width}px ${stand.length} notes and ${g.controls.length} sets of controls stand clear of one another`);
    }

    if (runs('notes-stay-near-their-words')) {
      for (const n of g.notes) {
        if (n.markTop === null) continue;
        if (n.inLayer && (n.box.top < n.markTop - 2 || n.box.top - n.markTop > 170)) fire('notes-stay-near-their-words', `at ${width}px a note stands ${Math.round(n.box.top - n.markTop)}px from its words; the most is 160, after which it stands after its paragraph`);
        if (!n.inLayer && !n.docked) fire('notes-stay-near-their-words', `at ${width}px a note is neither in the margin nor marked as standing after its paragraph`);
      }
      console.log(`marks-margin: at ${width}px each note stands within 160px of its words, or after its paragraph`);
    }

    if (runs('notes-keep-the-order-of-the-text')) {
      const order = stand.filter((n) => n.markTop !== null).sort((a, b) => a.markTop - b.markTop || a.markLeft - b.markLeft);
      for (let i = 1; i < order.length; i++) {
        if (order[i].box.top < order[i - 1].box.top - 0.5) fire('notes-keep-the-order-of-the-text', `at ${width}px the note for words lower in the text stands above the note for words higher up`);
      }
      console.log(`marks-margin: at ${width}px the notes stand in the order of their words`);
    }

    if (runs('a-note-scrolls-with-its-words')) {
      const before = (await geometry(page)).notes.filter((n) => n.inLayer && n.markTop !== null);
      const probeOf = (list) => list.map((n) => Math.round(n.box.top - n.markTop));
      await page.evaluate(() => scrollTo(0, 420));
      await page.waitForFunction(() => scrollY >= 400);
      const after = (await geometry(page)).notes.filter((n) => n.inLayer && n.markTop !== null);
      if (JSON.stringify(probeOf(before)) !== JSON.stringify(probeOf(after))) fire('a-note-scrolls-with-its-words', `after scrolling the notes stand ${JSON.stringify(probeOf(after))}px from their words, they stood ${JSON.stringify(probeOf(before))}px before`);
      const moved = await page.evaluate(() => {
        const note = document.querySelector('.y-mlayer .y-mnote');
        return note ? note.getBoundingClientRect().top + scrollY : null;
      });
      if (moved === null || Math.abs(moved - before[0].box.top) > 1) fire('a-note-scrolls-with-its-words', `a note stood ${before[0].box.top}px down the document and stands ${moved}px down it after the page was scrolled`);
      await page.evaluate(() => scrollTo(0, 0));
      console.log(`marks-margin: at ${width}px a scroll moves each note with its words`);
    }

    if (runs('a-mark-and-its-note-light-together')) {
      const target = stand.find((n) => n.markTop !== null);
      const piece = page.locator(`mark[data-mark="${target.id}"]`).first();
      const note = page.locator(`#mn-${target.id}`);
      const other = stand.find((n) => n.id !== target.id);
      const paint = () =>
        page.evaluate(
          ({ id, otherId }) => {
            const piece = document.querySelector(`mark[data-mark="${id}"]`);
            const note = document.getElementById(`mn-${id}`);
            const rest = document.getElementById(`mn-${otherId}`);
            return {
              pieceLit: piece.classList.contains('is-lit'),
              noteLit: note.classList.contains('is-lit'),
              wash: getComputedStyle(piece).backgroundColor,
              rule: getComputedStyle(note).borderInlineStartWidth,
              rest: Number(getComputedStyle(rest).opacity),
              height: note.getBoundingClientRect().height,
            };
          },
          { id: target.id, otherId: other.id },
        );
      await page.mouse.move(2, 2);
      const calm = await paint();
      await piece.scrollIntoViewIfNeeded();
      await piece.hover();
      const lit = await paint();
      if (!lit.pieceLit || !lit.noteLit) fire('a-mark-and-its-note-light-together', `pointing at the words lights the words ${lit.pieceLit} and the note ${lit.noteLit}, want both`);
      if (lit.wash === calm.wash) fire('a-mark-and-its-note-light-together', `the words are washed ${lit.wash} lit and not: a lit mark looks the same as one that is not`);
      if (lit.rule === calm.rule) fire('a-mark-and-its-note-light-together', 'the note\'s rule is as wide lit as not');
      if (!(lit.rest < 1)) fire('a-mark-and-its-note-light-together', `the other notes are drawn at ${lit.rest} while one is lit, want them stepped back`);
      if (Math.abs(lit.height - calm.height) > 0.5) fire('a-mark-and-its-note-light-together', `pointing at the words changed the note's height from ${calm.height} to ${lit.height}: the notes below it would be run over`);
      await page.mouse.move(2, 2);
      await note.scrollIntoViewIfNeeded();
      await note.hover();
      const back = await paint();
      if (!back.pieceLit || !back.noteLit) fire('a-mark-and-its-note-light-together', `pointing at the note lights the words ${back.pieceLit} and the note ${back.noteLit}, want both`);
      await page.mouse.move(2, 2);
      await page.locator(`#mn-${target.id}`).focus();
      await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));
      const focused = await paint();
      if (!focused.pieceLit) fire('a-mark-and-its-note-light-together', 'focusing the note does not light its words');
      // A note with the keyboard shows all it says, and what stands below it gives way.
      const after = await geometry(page);
      const here = after.notes.filter((n) => n.inLayer);
      for (let i = 0; i < here.length; i++) {
        for (let j = i + 1; j < here.length; j++) {
          if (overlaps(here[i].box, here[j].box)) fire('a-mark-and-its-note-light-together', `at ${width}px a note that has the keyboard stands over the one below it`);
        }
      }
      await page.evaluate(() => document.activeElement?.blur());
      console.log(`marks-margin: at ${width}px pointing at or focusing a mark or its note lights both, steps the others back, and moves nothing`);
    }
    await close();
  }

  if (runs('narrow-notes-stay-after-their-paragraph')) {
    for (const width of [1100, 390]) {
      const { page, close } = await open({ width, height: 900 });
      const g = await geometry(page);
      if (g.layerShown) fire('narrow-notes-stay-after-their-paragraph', `at ${width}px the margin layer is shown, and there is no room for a margin`);
      if (g.notes.length < 5) fire('narrow-notes-stay-after-their-paragraph', `at ${width}px ${g.notes.length} notes are drawn, want all ${PASSAGES.length}: without a margin they stand after their paragraphs`);
      for (const n of g.notes) {
        if (n.box.right > g.innerWidth + 0.5) fire('narrow-notes-stay-after-their-paragraph', `at ${width}px a note reaches ${n.box.right}px in a ${g.innerWidth}px window`);
        if (n.inLayer) fire('narrow-notes-stay-after-their-paragraph', `at ${width}px a note stands in the margin layer`);
      }
      if (g.scrollWidth > g.innerWidth) fire('narrow-notes-stay-after-their-paragraph', `at ${width}px the page is ${g.scrollWidth}px wide in a ${g.innerWidth}px window`);
      await close();
    }
    console.log('marks-margin: where there is no room for a margin, every note stands after its paragraph and the page is no wider than the window');
  }

  // The words chosen in a note with notes in the margin are the words chosen: the
  // margin is not part of what the server counts.
  const { page, close } = await open({ width: 1512, height: 900 });
  if (runs('notes-stand-in-the-margin-column')) {
    const SPAN = 'Pi passage stands under a heading';
    await choose(page, SPAN);
    await page.keyboard.press('q');
    await page.waitForFunction((words) => [...document.querySelectorAll('mark[data-mark]')].some((m) => m.textContent.includes(words)), SPAN, { timeout: 4000 });
    const kept = (await drawn(page)).find((e) => e.text === SPAN);
    if (!kept) fire('notes-stand-in-the-margin-column', `a passage chosen beside notes in the margin is drawn as ${JSON.stringify((await drawn(page)).map((e) => e.text))}, want exactly ${JSON.stringify(SPAN)}`);
  }
  await close();
});
