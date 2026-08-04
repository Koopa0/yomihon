// Mermaid enhancement. Diagram source remains visible unless the lazy,
// same-origin renderer successfully produces an SVG.
function decodeMermaidCode(raw) {
  return decodeURIComponent(raw.replace(/\+/g, ' '));
}

// The reader's theme is stamped on the root element before the first paint, so
// a diagram is drawn for the surface it will sit on instead of for the light
// default the renderer would otherwise assume.
function diagramTheme() {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'default';
}

export async function initDiagrams() {
  const blocks = document.querySelectorAll('.mermaid-diagram');
  if (blocks.length === 0) return;

  const { default: mermaid } = await import('/static/mermaid.esm.min.mjs');
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: diagramTheme(),
    // A label carrying a line break is drawn as HTML by default, inside a
    // foreign object, and the unclosed break that produces is not well-formed
    // XML — which is what the parse below requires. Drawing labels as shapes
    // keeps every diagram in one grammar. This belongs at the top level: the
    // per-diagram setting of the same name is read too late to matter.
    htmlLabels: false,
  });

  let next = 0;
  for (const element of blocks) {
    const source = element.textContent;
    const code = decodeMermaidCode(element.getAttribute('data-mermaid-code') || '');
    if (!code) continue;
    const id = `mermaid-diagram-${next++}`;
    try {
      const { svg } = await mermaid.render(id, code);
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
