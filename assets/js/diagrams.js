// Mermaid enhancement. Diagram source remains visible unless the lazy,
// same-origin renderer successfully produces an SVG.
function decodeMermaidCode(raw) {
  return decodeURIComponent(raw.replace(/\+/g, ' '));
}

// preferences reports the theme the reader is actually looking at and says when
// it moves. A diagram is drawn in that theme's colours, so it is part of the
// prose rather than chrome around it, and it cannot sit a frame behind the page
// it is printed on.
export async function initDiagrams(preferences) {
  const blocks = document.querySelectorAll('.mermaid-diagram');
  if (blocks.length === 0) return;

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
    // Drawings are chained rather than raced, so two quick changes of mind
    // leave the theme chosen last on the page; a drawing that failed does not
    // stop the one after it.
    drawing = drawing.then(draw, draw).catch((error) => {
      console.warn('[yomihon] mermaid diagram failed to redraw:', error);
    });
  });
  await drawing;
}
