// What this browser keeps of where a reader stopped in each note: one list in
// this browser's local storage, for this address, newest last.
//
// An entry names the vault by its id as well as the note by its path. A vault
// that is removed gives its port to the next vault opened, and an address and a
// path alone would hand that vault's note of the same name an old place.
//
// The list is the reader's own and holds no text of the notes: a vault's id, a
// note's path, the id of a heading and a distance. It is capped, and the cap
// drops the note read longest ago. Storage this address may not use is refused
// silently, so every way in here answers as though nothing were kept.
const KEY = 'yomihon.positions';

// How many notes keep a place. An entry is a hundred and some bytes, so this is
// a few tens of kilobytes, and the whole list is read and written whole; two
// hundred notes is a long stretch of reading, and the browser's quota is far
// beyond it.
const MOST = 200;

function sound(entry) {
  return (
    entry !== null &&
    typeof entry === 'object' &&
    typeof entry.vault === 'string' &&
    typeof entry.path === 'string' &&
    typeof entry.anchor === 'string' &&
    Number.isFinite(entry.offset) &&
    entry.offset >= 0
  );
}

// kept is the list as the browser holds it, with anything that is not an entry
// left out: the storage is the reader's to edit, and a hand-written value must
// not take the page with it.
function kept() {
  try {
    const list = JSON.parse(localStorage.getItem(KEY) ?? '[]');
    return Array.isArray(list) ? list.filter(sound) : [];
  } catch {
    return [];
  }
}

function store(list) {
  try {
    if (list.length === 0) localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, JSON.stringify(list));
  } catch {
    // A browser that refuses this address its storage keeps no places.
  }
}

const same = (vault, path) => (entry) => entry.vault === vault && entry.path === path;

// positionOn answers whether the reader has the setting on, from the cookie that
// holds it. The root names it too, but another tab may have turned it off since
// this page was drawn, and a place written after that would be one the reader
// had asked not to be kept.
export function positionOn() {
  return !document.cookie.split(';').some((part) => part.trim() === 'yomihon_position=off');
}

// recallPosition is the place kept for the note, or null.
export function recallPosition(vault, path) {
  const entry = kept().find(same(vault, path));
  return entry ? { anchor: entry.anchor, offset: entry.offset } : null;
}

// rememberPosition keeps the place as the note's newest, in place of any it had.
export function rememberPosition(vault, path, { anchor, offset }) {
  const list = kept().filter((entry) => !same(vault, path)(entry));
  list.push({ vault, path, anchor, offset });
  store(list.slice(-MOST));
}

// forgetPosition takes the note's place away, for a note read from its top.
export function forgetPosition(vault, path) {
  const list = kept();
  const rest = list.filter((entry) => !same(vault, path)(entry));
  if (rest.length !== list.length) store(rest);
}

// forgetPositions takes every place away, for a reader who turned the setting off.
export function forgetPositions() {
  store([]);
}
