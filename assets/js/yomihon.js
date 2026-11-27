// yomihon client-runtime entry. Every imported module is inert until this file
// calls its initializer. The rail captures and restores filter state at the parser
// boundary; this entry attaches interactions and composes the remaining capabilities.
// The server-rendered page remains usable when the graph is absent.
import { initCodeCopy } from './codecopy.js';
import { initCompareAlign, initContents, initRubySelection } from './contents.js';
import { initDiagrams } from './diagrams.js';
import { initDrawer } from './drawer.js';
import { initFreshness } from './freshness.js';
import { initLangForm } from './langform.js';
import { initLesson } from './lesson.js';
import { initList } from './list.js';
import { initMark } from './mark.js';
import { initMarks } from './marks.js';
import { initPreferences } from './preferences.js';
import { initPreview } from './preview.js';
import { initRail } from './rail.js';
import { initRunningHead } from './runninghead.js';
import { initSearch } from './search.js';
import { initShortcuts } from './shortcuts.js';
import { initSidebar } from './sidebar.js';
import { initThought } from './thought.js';
import { initQuestion } from './question.js';

function init() {
  const root = document.documentElement;
  root.dataset.js = 'on';
  if ('speechSynthesis' in window) root.dataset.speech = 'on';

  const drawer = initDrawer();
  const sidebar = initSidebar();
  initLangForm();
  const preferences = initPreferences();
  // The running head's list is counted before the contents are tracked, so
  // the tracking sees only the entries it keeps.
  const runningHead = initRunningHead();
  initContents(runningHead?.follow);
  initCompareAlign();
  initRubySelection();
  initFreshness();
  const rail = initRail(preferences);
  const search = initSearch();
  // The reader's marks are laid out before the reading position is put back, so
  // what stands in the margin is where it will stand when the page is read.
  const marks = initMarks();
  initShortcuts({ drawer, rail, sidebar, search, marks });
  const enhanceCodeCopy = initCodeCopy();
  initLesson(enhanceCodeCopy);
  initList();
  initMark();
  initThought();
  initQuestion();
  initPreview();
  initDiagrams(preferences).catch((error) => {
    root.setAttribute('data-mermaid-error', '');
    console.warn('[yomihon] mermaid rendering failed outside a diagram:', error);
  });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
