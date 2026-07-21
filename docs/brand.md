# Brand identity

Status: **selected application and repository identity**. Koopa selected this
direction on 2026-07-21 after correcting the publication band so it remains on
the cover. Automated implementation evidence can make the asset merge-ready;
the human and trademark checks below still separate that claim from release
readiness.

## Product truth

Yomihon is a local reading and adjudication interface for one person's
knowledge vault. Reading is the visible center. Diagnostics, retrieval, and
agent machinery support it without turning the product into an editor, hosted
AI service, generic note application, or search engine.

The mark is a shallow-oblique, closed black book with a warm-white page
fore-edge and a vermilion publication obi:

- the closed book represents a bounded reading object under its owner's care;
- the page fore-edge represents the source material and remains continuously
  visible rather than becoming interface decoration;
- the publication obi represents the deliberate status Yomihon assigns after
  reading; it belongs to the cover and never crosses onto the pages.

The result should feel quiet, exact, editorial, intimate, local, and
contemporary. It is a bound-book identity, not an open-book illustration or a document
icon. `書庫` remains supporting Traditional Chinese interface text and is not
part of the universal mark.

## Canonical construction

[`assets/brand/yomihon-mark.svg`](../assets/brand/yomihon-mark.svg) is the sole
geometry authority. It is a hand-authored deterministic vector derived from
the approved hero concept, not traced or exported from the generated concept
board. The board's inconsistent lower monochrome examples were explicitly
rejected as geometry sources.

Any edit to a path's `d` value changes the selected identity and requires a
new Koopa brand decision. Automated checks protect the construction,
projection, and page/obi invariants; they do not replace that visual authority
with a duplicate coordinate list or test-owned hash.

The fixed `0 0 32 32` canvas contains exactly three filled paths in this order:

1. `cover` — book ink `#0F0F0F`;
2. `pages` — warm paper `#F5F1E6`;
3. `obi` — publication vermilion `#D62A0F`.

The cover path also provides the dark outline around the fore-edge. The pages
are one continuous path. The obi has zero rendered overlap with that page
path. This separation is the load-bearing invariant: a revision that paints
red over the white fore-edge is a different and invalid mark.

The SVG contains no text, font, style sheet, raster image, transform, mask,
filter, gradient, script, external reference, generator metadata, or alternate
shape. Image generation was used only to compare art directions; generated
pixels are not a production asset.

## Projection contract

Every supported surface references the canonical SVG directly:

- embedded application bytes and `/static/yomihon-mark.svg`;
- the browser favicon;
- the application header beside the visible lowercase `yomihon` name;
- this repository's README.

There is no separately drawn favicon, monochrome asset, reversed asset, raster
derivative, CSS mask, wordmark file, manifest, ICO, Apple touch icon, install
flow, or PWA. A new projection must reuse the same source rather than copy its
coordinates.

The header image is decorative: its empty alternative text and
`aria-hidden="true"` leave the visible lowercase `yomihon` link as the exact
accessible name. In forced-colors mode the fixed-colour decorative image may
be hidden while that textual identity remains visible. Width and height are
declared so loading the local asset cannot shift the 56-pixel header.

## Selection and release evidence

The selected hero passed owner direction review and is implemented at 16, 24,
32, and header sizes in both application themes. Automated checks must pin the
restricted SVG grammar, the three colours, continuous page path, zero
page/obi overlap, exact local serving, MIME type, projection source,
accessibility semantics, and small-viewport fit. The evidence system must also
prove those locks can fail, especially when the obi is mutated across the page
fore-edge.

Before a public release, human review still includes:

- actual browser-tab recognition at 16 and 32 pixels;
- a five-second product-impression test and delayed recall;
- comparison beside the real status and seal surfaces in both themes;
- a basic similarity and trademark-conflict screen;
- Koopa's decision whether the mark follows the repository's MIT license or
  has a separate, plainly stated trademark policy.

Until those checks are recorded, the identity may be merge-ready without
being release-ready. No trademark restriction is implied before the owner
makes that decision.
