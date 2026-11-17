// The reader's recent searches, kept in this browser and nowhere else. This is
// the one place that knows where they live, so moving them (the reader's small
// state may later be kept with their other settings) changes this file alone.
// Storage may be absent, full or refused; every call then does nothing and the
// surfaces simply have no history to show.
const key = 'yomihon.recentSearches';
const most = 5;

function read() {
  try {
    const stored = JSON.parse(localStorage.getItem(key) || '[]');
    return Array.isArray(stored)
      ? stored.filter((q) => typeof q === 'string' && q !== '').slice(0, most)
      : [];
  } catch {
    return [];
  }
}

function write(list) {
  try {
    if (list.length === 0) localStorage.removeItem(key);
    else localStorage.setItem(key, JSON.stringify(list));
  } catch {
    // Nothing to keep them in: the next visit starts with none.
  }
}

export const recentSearches = {
  list: read,
  // A search asked again moves to the front rather than standing twice.
  add(query) {
    const q = query.trim();
    if (!q) return;
    write([q, ...read().filter((old) => old !== q)].slice(0, most));
  },
  clear() {
    write([]);
  },
};

// fillRecent writes the reader's recent searches into a section the server
// drew empty and hidden, and shows it only when there is something to show. It
// returns how many rows it wrote.
export function fillRecent(section) {
  const list = section.querySelector('[data-recent-list]');
  if (!list) return 0;
  const found = recentSearches.list();
  list.replaceChildren(
    ...found.map((q) => {
      const item = document.createElement('li');
      const link = document.createElement('a');
      link.className = 'y-lrow';
      link.href = `/search?${new URLSearchParams({ q })}`;
      link.dataset.liveSearchRow = '';
      link.dataset.recentQuery = q;
      const title = document.createElement('span');
      title.className = 'y-lrow__title';
      title.textContent = q;
      link.append(title);
      item.append(link);
      return item;
    }),
  );
  section.hidden = found.length === 0;
  return found.length;
}
