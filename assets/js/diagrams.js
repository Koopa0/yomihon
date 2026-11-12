// Mermaid enhancement. Diagram source remains visible unless the lazy,
// same-origin renderer successfully produces an SVG.
function decodeMermaidCode(raw) {
  return decodeURIComponent(raw.replace(/\+/g, ' '));
}

// The tallest and shortest a figure may ask its frame to be. A figure reports
// its own height, and nothing it reports may make the page unusable: a frame
// asking for a million pixels gets the ceiling, one asking for none the floor.
const FIGURE_HEIGHT_MIN = 48;
const FIGURE_HEIGHT_MAX = 8000;

// Figures are a vault's interactive HTML, each in a frame of its own. Only a
// frame the server drew for an approved figure runs script, and only those
// frames are spoken to or listened to: a figure without script cannot answer,
// and anything else posting to this page is not a figure.
function initFigures(preferences) {
  const scripted = () => document.querySelectorAll('iframe.figure-embed__frame[data-figure-scripts]');
  // The figure's origin is opaque, so the message has to be addressed to any
  // origin. It carries the theme name and nothing else.
  const tellTheme = (frame) => {
    const theme = preferences.theme() === 'dark' ? 'dark' : 'light';
    frame.contentWindow?.postMessage({ type: 'yomihon-theme', theme }, '*');
  };
  for (const frame of scripted()) tellTheme(frame);
  // Load does not bubble, so it is caught on the way down; a frame that loads
  // after this ran, or reloads, is told the theme again.
  document.addEventListener(
    'load',
    (event) => {
      const frame = event.target;
      if (frame instanceof HTMLIFrameElement && frame.matches('.figure-embed__frame[data-figure-scripts]')) {
        tellTheme(frame);
      }
    },
    true,
  );
  window.addEventListener('message', (event) => {
    // A sandboxed figure's origin is opaque and serializes as "null"; its
    // window is the one way to tell which frame spoke.
    if (event.origin !== 'null') return;
    const data = event.data;
    if (data === null || typeof data !== 'object' || data.type !== 'yomihon-figure-height') return;
    const height = data.height;
    if (typeof height !== 'number' || !Number.isFinite(height)) return;
    for (const frame of scripted()) {
      if (frame.contentWindow !== event.source) continue;
      const clamped = Math.min(FIGURE_HEIGHT_MAX, Math.max(FIGURE_HEIGHT_MIN, Math.round(height)));
      frame.setAttribute('height', String(clamped));
      return;
    }
  });
  return () => {
    for (const frame of scripted()) tellTheme(frame);
  };
}

// preferences reports the theme the reader is actually looking at and says when
// it moves. A diagram is drawn in that theme's colours, so it is part of the
// prose rather than chrome around it, and it cannot sit a frame behind the page
// it is printed on. A figure is told the theme too, so it can follow it.
export async function initDiagrams(preferences) {
  const retheme = initFigures(preferences);
  const blocks = document.querySelectorAll('.mermaid-diagram');
  if (blocks.length === 0) {
    preferences.onThemeChange(retheme);
    return;
  }

  const { default: mermaid } = await import('/static/mermaid.esm.min.mjs');

  // Ids are handed out across every drawing rather than within one: a redraw
  // renders while the diagram it replaces is still in the document, and the
  // renderer works under the id it is given.
  let next = 0;

  async function draw() {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      // Read as the drawing starts rather than when it was asked for, so a
      // reader who changes their mind twice ends on the theme they chose last.
      theme: preferences.theme() === 'dark' ? 'dark' : 'default',
      // A label carrying a line break is drawn as HTML by default, inside a
      // foreign object, and the unclosed break that produces is not well-formed
      // XML — which is what the parse below requires. Drawing labels as shapes
      // keeps every diagram in one grammar. This belongs at the top level: the
      // per-diagram setting of the same name is read too late to matter.
      htmlLabels: false,
    });

    for (const element of blocks) {
      // The placeholder carries the authored source twice, and the encoded copy
      // is the one that survives being drawn: the element's own text is what a
      // diagram replaces. So both the source to draw and the words that stand
      // in for it are read from the attribute, on the first drawing and on
      // every later one alike.
      const source = decodeMermaidCode(element.getAttribute('data-mermaid-code') || '');
      if (!source) continue;
      const id = `mermaid-diagram-${next++}`;
      try {
        const { svg } = await mermaid.render(id, source);
        const parsed = new DOMParser().parseFromString(svg, 'image/svg+xml');
        const svgElement = parsed.documentElement;
        if (svgElement.nodeName.toLowerCase() !== 'svg') {
          // The parse failed and handed back its own error document. Falling
          // through here left the block on its loading shimmer for as long as the
          // page stayed open — a diagram that never arrives and never says so.
          // Every way this can fail now ends in the same place.
          throw new Error('the renderer produced markup that is not an SVG');
        }
        // A drawing carries no words a reader can hear. Announcing it as a single
        // image stops assistive technology walking its shapes one by one, and the
        // source the author wrote stays in the page as the text that says what it
        // shows — the same text a sighted reader gets when rendering fails.
        svgElement.setAttribute('role', 'img');
        const alternative = document.createElement('span');
        alternative.className = 'y-offscreen';
        alternative.textContent = source;
        alternative.id = `${id}-source`;
        // Mermaid retains an author's title and description. A drawing without
        // those words takes its name from the source already kept beside it.
        if (!svgElement.getAttribute('aria-label')?.trim() && !svgElement.getAttribute('aria-labelledby')?.trim()) {
          svgElement.setAttribute('aria-labelledby', alternative.id);
        }
        element.replaceChildren(alternative);
        element.appendChild(document.importNode(svgElement, true));
      } catch (error) {
        element.setAttribute('data-mermaid-error', '');
        console.warn('[yomihon] mermaid diagram failed to render:', error);
      }
    }
  }

  let drawing = draw();
  preferences.onThemeChange(() => {
    retheme();
    // Drawings are chained rather than raced, so two quick changes of mind
    // leave the theme chosen last on the page; a drawing that failed does not
    // stop the one after it.
    drawing = drawing.then(draw, draw).catch((error) => {
      console.warn('[yomihon] mermaid diagram failed to redraw:', error);
    });
  });
  await drawing;
}
