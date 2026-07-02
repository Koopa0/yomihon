/* kurodo runtime — the first (and so far only) client-side script this
   repo ships. All vanilla, zero framework, loaded `defer` from
   internal/ui/layouts/base.templ. Its one job: render ```mermaid fences
   (internal/render's consumeMermaid leaves each as a
   <div class="mermaid-diagram" data-mermaid-code="…"> placeholder,
   already-readable as plain text) into an actual SVG diagram, client-side,
   lazily — only on pages that have at least one such element, so a note
   with no diagrams never pulls in the mermaid bundle at all.

   Mirrors koopa0.dev's frontend/src/app/core/services/markdown.service.ts
   (processMermaidDiagrams/renderMermaid) DOM/JS convention exactly: the
   div.mermaid-diagram + data-mermaid-code shape, mermaid.initialize's
   startOnLoad:false/securityLevel:'strict' options, and DOMParser-based
   SVG replacement instead of innerHTML (defense in depth kurodo doesn't
   strictly need today — this corpus is trusted, docs/vault-model.md — but
   consistency of practice matters since kurodo may eventually gain
   less-trusted content paths). The one intentional difference is HOW the
   mermaid runtime is loaded: koopa0.dev's Angular build bundles `mermaid`
   from npm and resolves `import('mermaid')` itself; kurodo has no
   bundler, so this imports the vendored ES module straight from
   internal/asset's static route instead. */
(() => {
  'use strict';

  // internal/render's consumeMermaid encodes data-mermaid-code with Go's
  // net/url.QueryEscape, which — unlike JS's encodeURIComponent — encodes
  // a literal space as "+", not "%20". decodeURIComponent alone leaves a
  // stray "+" untouched (it is not one of the %XX escapes it knows), so
  // every space in the diagram source would otherwise survive as a
  // literal "+" and break mermaid's parser. Un-doing the "+"-for-space
  // convention first, exactly like decoding an
  // application/x-www-form-urlencoded value, keeps the two sides in sync.
  function decodeMermaidCode(raw) {
    return decodeURIComponent(raw.replace(/\+/g, ' '));
  }

  async function renderMermaidDiagrams() {
    const blocks = document.querySelectorAll('.mermaid-diagram');
    if (blocks.length === 0) return; // never fetch the mermaid bundle on a page with no diagrams

    const { default: mermaid } = await import('/static/mermaid.esm.min.mjs');
    mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' });

    let next = 0;
    for (const el of blocks) {
      const code = decodeMermaidCode(el.getAttribute('data-mermaid-code') || '');
      if (!code) continue;
      const id = 'mermaid-diagram-' + next++;
      try {
        const { svg } = await mermaid.render(id, code);
        const parsed = new DOMParser().parseFromString(svg, 'image/svg+xml');
        const svgEl = parsed.documentElement;
        if (svgEl.nodeName.toLowerCase() !== 'svg') continue; // parse error — keep the source-text fallback
        // DOM-API replacement (no innerHTML); mermaid securityLevel:'strict'
        // sanitizes diagram content, DOMParser does not execute script.
        el.replaceChildren();
        el.appendChild(document.importNode(svgEl, true));
      } catch (err) {
        console.warn('[kurodo] mermaid diagram failed to render:', err);
        // Invalid diagram — leave the source text in place as a fallback.
      }
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', renderMermaidDiagrams);
  } else {
    renderMermaidDiagrams();
  }
})();
