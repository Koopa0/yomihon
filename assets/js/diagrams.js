// Mermaid enhancement. Diagram source remains visible unless the lazy,
// same-origin renderer successfully produces an SVG.
function decodeMermaidCode(raw) {
  return decodeURIComponent(raw.replace(/\+/g, ' '));
}

export async function initDiagrams() {
  const blocks = document.querySelectorAll('.mermaid-diagram');
  if (blocks.length === 0) return;

  const { default: mermaid } = await import('/static/mermaid.esm.min.mjs');
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' });

  let next = 0;
  for (const element of blocks) {
    const code = decodeMermaidCode(element.getAttribute('data-mermaid-code') || '');
    if (!code) continue;
    const id = `mermaid-diagram-${next++}`;
    try {
      const { svg } = await mermaid.render(id, code);
      const parsed = new DOMParser().parseFromString(svg, 'image/svg+xml');
      const svgElement = parsed.documentElement;
      if (svgElement.nodeName.toLowerCase() !== 'svg') continue;
      element.replaceChildren();
      element.appendChild(document.importNode(svgElement, true));
    } catch (error) {
      element.setAttribute('data-mermaid-error', '');
      console.warn('[yomihon] mermaid diagram failed to render:', error);
    }
  }
}
