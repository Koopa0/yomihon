// yomihon client-runtime entry. Every imported module is inert until this file
// calls its initializer; this is the single owner of capability composition and
// boot order. The server-rendered page remains usable when the graph is absent.
import { initContents } from './contents.js';
import { initDiagrams } from './diagrams.js';
import { initDrawer } from './drawer.js';
import { initFreshness } from './freshness.js';
import { initLesson } from './lesson.js';
import { initPreferences } from './preferences.js';
import { initPreview } from './preview.js';
import { initSearch } from './search.js';
import { initShortcuts } from './shortcuts.js';
import { initSidebar } from './sidebar.js';

function init() {
  const root = document.documentElement;
  root.dataset.js = 'on';
  if ('speechSynthesis' in window) root.dataset.speech = 'on';

  const drawer = initDrawer();
  const sidebar = initSidebar();
  initPreferences();
  initContents();
  initFreshness();
  const search = initSearch();
  initShortcuts({ drawer, sidebar, search });
  initLesson();
  initPreview();
  initDiagrams().catch((error) => {
    root.setAttribute('data-mermaid-error', '');
    console.warn('[yomihon] mermaid rendering failed outside a diagram:', error);
  });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
