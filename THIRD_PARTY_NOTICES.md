# Third-party notices

This file records third-party source copied into the repository and embedded
in yomihon. Go module dependencies remain identified by `go.mod` and `go.sum`;
a future prebuilt-binary release must additionally ship a generated SBOM and
the notices required by the exact linked module graph.

## Mermaid 11.15.0

- Files: `assets/js/mermaid/`
- Source: <https://cdn.jsdelivr.net/npm/mermaid@11.15.0/dist/>
- Upstream: <https://github.com/mermaid-js/mermaid>
- Licence: MIT, reproduced in `assets/js/mermaid/LICENSE`
- Integrity: the complete vendored `.mjs` inventory and licence are locked by
  `assets/js/mermaid/SHA256SUMS`

## Geist and Geist Mono 1.500

- Files: `assets/fonts/Geist-Variable.woff2`,
  `assets/fonts/GeistMono-Variable.woff2`
- Upstream: <https://github.com/vercel/geist-font>, release 1.5.0
- Copyright: 2024 The Geist Project Authors
- Licence: SIL Open Font License 1.1, reproduced in
  `assets/fonts/LICENSE.txt`

## Newsreader 1.003

- Files: `assets/fonts/Newsreader-Variable.woff2`,
  `assets/fonts/Newsreader-Italic-Variable.woff2`
- Upstream: <https://github.com/productiontype/Newsreader>
- Copyright: 2020 The Newsreader Project Authors
- Licence: SIL Open Font License 1.1, reproduced in
  `assets/fonts/LICENSE.txt`

Exact hashes are machine-checked from `assets/fonts/SHA256SUMS`; the limits of
the retained provenance are recorded in `assets/fonts/README.md`.
