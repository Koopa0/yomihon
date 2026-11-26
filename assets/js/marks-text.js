// The words of a note as the server counts them, and the marks drawn on them.
//
// The server places a mark by the text of the page it rendered: the text nodes in
// order, with a space where a block ends, and none of a script, a style, a
// template or a ruby reading. This reads the document the same way, so a
// selection becomes the words, and the words around them, that the server looks
// for. Nothing the page adds for itself is a word of the note: not the margin
// beside a heading, not a note, a field, the toolbar, the layer the notes are
// laid out in, or the control that copies code.

const HIDES = new Set(['SCRIPT', 'STYLE', 'TEMPLATE', 'RP', 'RT', 'RTC', 'SVG']);
const LINES = new Set([
  'ADDRESS', 'ARTICLE', 'ASIDE', 'BLOCKQUOTE', 'BR', 'DD', 'DETAILS', 'DIV', 'DL', 'DT',
  'FIGCAPTION', 'FIGURE', 'FOOTER', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HEADER', 'HR',
  'LI', 'MAIN', 'NAV', 'OL', 'P', 'PRE', 'SECTION', 'SUMMARY', 'TABLE', 'TBODY', 'TD',
  'TFOOT', 'TH', 'THEAD', 'TR', 'UL',
]);
const OURS = '.y-margin, .y-mnote, .y-mfield, .y-mlayer, .y-mbar, .y-mstatus, .y-codecopy, [data-mark-skip]';

// A heading's id names the section a mark is made under.
const HEADING = 'h1, h2, h3, h4, h5, h6';

const skipped = (element) => HIDES.has(element.tagName.toUpperCase()) || element.matches(OURS);

// readWords is the words of a container and where each text node begins in them.
export function readWords(container) {
  const pieces = [];
  let text = '';
  const walk = (node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      pieces.push({ node, start: text.length });
      text += node.data;
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE || skipped(node)) return;
    const line = LINES.has(node.tagName.toUpperCase());
    if (line) text += ' ';
    for (const child of node.childNodes) walk(child);
    if (line) text += ' ';
  };
  for (const child of container.childNodes) walk(child);
  return {
    text,
    pieces,
    // offsetOf is where a boundary point of a range is in the words, or -1 where
    // it is in something that is not words.
    offsetOf(node, offset) {
      if (node.nodeType === Node.TEXT_NODE) {
        const piece = pieces.find((p) => p.node === node);
        return piece ? piece.start + offset : -1;
      }
      // A boundary between children is the start of the next text, or the end of
      // the last one before it.
      const next = node.childNodes[offset];
      if (next) {
        const first = firstText(next);
        if (first) return pieces.find((p) => p.node === first)?.start ?? -1;
        return endBefore(pieces, next);
      }
      return endBefore(pieces, null, node);
    },
  };
}

function firstText(node) {
  if (node.nodeType === Node.TEXT_NODE) return node;
  const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
  return walker.nextNode();
}

// endBefore is the end of the last text that comes before a node, or, with no
// node, the end of the last text inside a container.
function endBefore(pieces, node, container) {
  let found = -1;
  for (const p of pieces) {
    const before = node
      ? Boolean(node.compareDocumentPosition(p.node) & Node.DOCUMENT_POSITION_PRECEDING)
      : container.contains(p.node);
    if (before) found = p.start + p.node.data.length;
  }
  return found;
}

// squeeze makes every run of white space one space, as the server does when it
// looks for words.
const squeeze = (s) => s.replace(/\s+/g, ' ');

// within says whether a node is in the note's text and not in anything the page
// added to it.
export function within(prose, node) {
  const element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
  return Boolean(element) && prose.contains(element) && !element.closest(OURS);
}

// The most context asked for on either side of a selection, in bytes of UTF-8,
// and no more than the server keeps.
const CONTEXT = 512;

function utf8Tail(s, limit) {
  const bytes = new TextEncoder().encode(s);
  if (bytes.length <= limit) return s;
  let at = bytes.length - limit;
  while (at < bytes.length && (bytes[at] & 0xc0) === 0x80) at++;
  return new TextDecoder().decode(bytes.slice(at));
}

function utf8Head(s, limit) {
  const bytes = new TextEncoder().encode(s);
  if (bytes.length <= limit) return s;
  let at = limit;
  while (at > 0 && (bytes[at] & 0xc0) === 0x80) at--;
  return new TextDecoder().decode(bytes.slice(0, at));
}

// capture is what a selection says to the server: the words chosen, the words
// before and after them, and the section they stand in. It is null where the
// selection holds no words or reaches out of the note.
export function capture(prose, range) {
  if (!within(prose, range.startContainer) || !within(prose, range.endContainer)) return null;
  const words = readWords(prose);
  let start = words.offsetOf(range.startContainer, range.startOffset);
  let end = words.offsetOf(range.endContainer, range.endOffset);
  if (start < 0 || end < 0 || end <= start) return null;
  // A selection that begins or ends in white space is the words between.
  while (start < end && /\s/.test(words.text[start])) start++;
  while (end > start && /\s/.test(words.text[end - 1])) end--;
  if (end <= start) return null;
  const quote = squeeze(words.text.slice(start, end)).trim();
  const before = utf8Tail(squeeze(words.text.slice(0, start)), CONTEXT);
  const after = utf8Head(squeeze(words.text.slice(end)), CONTEXT);
  return { start, end, quote, before, after, anchor: sectionAt(prose, range.startContainer) };
}

// sectionAt is the id of the heading a node stands under. Marks are kept only on
// a note that stands alone on its page, so the id is the section's own.
export function sectionAt(prose, node) {
  const element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
  let heading = null;
  for (const candidate of prose.querySelectorAll(HEADING)) {
    if (!candidate.id || !candidate.hasAttribute('data-level')) continue;
    if (candidate === element || candidate.contains(element) || candidate.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING) {
      heading = candidate;
    } else {
      break;
    }
  }
  return heading ? heading.id : '';
}

// The element a note about words ends in is put after: a paragraph, a heading, a
// block of code or a table, and at the end of a list item or a block that holds
// the words directly. It is the same choice the server makes for a page that
// arrives with the note, so a note made now is where it will be after a reload.
const AFTER = new Set(['P', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'PRE', 'TABLE', 'DD', 'DT', 'FIGCAPTION', 'SUMMARY']);
const CARRIERS = new Set([...AFTER, 'LI', 'BLOCKQUOTE', 'DIV', 'SECTION', 'ARTICLE', 'ASIDE', 'DETAILS', 'TD', 'TH']);

// slotFor is where a note about words ending in a node goes: the parent and the
// node it goes before, or null to go last.
export function slotFor(prose, node) {
  let element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
  while (element && element !== prose && !CARRIERS.has(element.tagName.toUpperCase())) element = element.parentElement;
  if (!element || element === prose) return { parent: prose, before: null };
  let carrier = element;
  if (carrier.tagName === 'TD' || carrier.tagName === 'TH') carrier = carrier.closest('table') ?? carrier;
  if (AFTER.has(carrier.tagName.toUpperCase())) {
    // A note belongs after the carrier and after the notes already there.
    let at = carrier;
    while (at.nextElementSibling?.matches('.y-mnote, .y-mfield')) at = at.nextElementSibling;
    return { parent: at.parentElement, before: at.nextSibling };
  }
  return { parent: carrier, before: null };
}

// wrap draws a span of the words as marks: each text it runs through is split
// where the span begins and ends, and the part inside is wrapped. The first
// piece is the one a keyboard meets. It reports the pieces it made.
export function wrap(prose, start, end, attributes, first) {
  const words = readWords(prose);
  const made = [];
  const inside = words.pieces.filter((p) => p.start < end && p.start + p.node.data.length > start);
  for (const [index, piece] of inside.entries()) {
    let node = piece.node;
    const from = Math.max(start - piece.start, 0);
    const to = Math.min(end - piece.start, node.data.length);
    if (to < node.data.length) node.splitText(to);
    if (from > 0) node = node.splitText(from);
    const mark = document.createElement('mark');
    for (const [name, value] of Object.entries(attributes)) mark.setAttribute(name, value);
    if (index === 0) for (const [name, value] of Object.entries(first)) mark.setAttribute(name, value);
    node.parentNode.insertBefore(mark, node);
    mark.append(node);
    made.push(mark);
  }
  return made;
}

// unwrap takes the marks off their words and leaves the words as they were,
// joined again where they were split.
export function unwrap(pieces) {
  for (const mark of pieces) {
    const parent = mark.parentNode;
    if (!parent) continue;
    while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
    mark.remove();
    parent.normalize();
  }
}

// spanOf is where the pieces of a mark are in the words: from the start of the
// first to the end of the last, which does not change while the text does not.
export function spanOf(prose, pieces) {
  if (pieces.length === 0) return null;
  const words = readWords(prose);
  const first = words.offsetOf(pieces[0], 0);
  const last = pieces[pieces.length - 1];
  const end = words.offsetOf(last, last.childNodes.length);
  return first < 0 || end < 0 ? null : { start: first, end };
}
