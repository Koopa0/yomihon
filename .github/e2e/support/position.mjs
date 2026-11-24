// What the probes of the per-note reading position share: the key the browser
// keeps places under, what a place looks like, and the waits that tell a probe
// the page has stopped moving or the browser has written. A probe never reads
// the module's own state, only what the browser holds and where the window is.
import { performance } from 'node:perf_hooks';

import { arrived } from './arrival.mjs';

export const KEY = 'yomihon.positions';

// settled waits until the fonts are in and the document has stopped growing, so
// a place is measured against the page the reader will come back to.
export const settled = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    let height = -1;
    while (height !== document.documentElement.scrollHeight) {
      height = document.documentElement.scrollHeight;
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  });

export const frames = (page, count = 2) =>
  page.evaluate(
    (n) =>
      new Promise((resolve) => {
        const step = (left) => (left === 0 ? resolve() : requestAnimationFrame(() => step(left - 1)));
        step(n);
      }),
    count,
  );

export const scrollTop = (page) => page.evaluate(() => Math.round(window.scrollY));

// storedPlaces is what this origin's storage holds under the key: null where
// nothing is kept, otherwise the parsed list, oldest first.
export const storedPlaces = (page) =>
  page.evaluate((key) => {
    const raw = localStorage.getItem(key);
    return raw === null ? null : JSON.parse(raw);
  }, KEY);

// waitForPlace polls until the storage holds a place for the note at path, and
// answers it. A place the browser has not written yet is not a failure of the
// page, so the wait is the probe's, not the page's: the page is allowed its
// debounce.
export async function waitForPlace(page, path, { timeout = 4000, changedFrom = null } = {}) {
  const deadline = performance.now() + timeout;
  for (;;) {
    const places = await storedPlaces(page);
    const found = places?.find((place) => place.path === path) ?? null;
    if (found && JSON.stringify(found) !== changedFrom) return found;
    if (performance.now() >= deadline) return null;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}

// openNote loads the note afresh, the way a link to it from anywhere does, and
// waits for the page to have been placed and to have stopped moving.
export async function openNote(page, base, note, { hash = '' } = {}) {
  const response = await page.goto(`${base}${note}${hash}`, { waitUntil: 'load' });
  if (response?.status() !== 200) throw new Error(`BROKEN: ${note} answered ${response?.status()}, want 200`);
  await arrived(page);
  await settled(page);
  await frames(page);
}
