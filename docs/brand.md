# Brand identity

Status: **selected canon**. On 2026-07-21 Koopa delegated the choice and
application of the mark to Codex; Codex selected Concept A, the converging-path
`y` direction. The repository source at
[`assets/brand/yomihon-mark.svg`](../assets/brand/yomihon-mark.svg) is the only
official mark geometry. This document owns its meaning, construction,
provenance, and permitted projections; the SVG owns its exact vector bytes.

## Product truth

Yomihon is a local reading and adjudication interface for one person's
knowledge vault. Reading is the visible center. Diagnostics, retrieval, and
agent machinery support that reading without turning the product into an
editor, hosted AI service, generic note application, or search engine.

The identity should feel quiet, exact, editorial, intimate, local, and
deliberate. It may be CJK-literate, but it must not use Japanese or Chinese
writing as visual costume. It should be contemporary rather than faux
traditional, and technical without looking like developer tooling.

The visual hierarchy is part of that truth:

- ink and paper carry the product identity;
- vermilion remains the scarce ceremonial color of `ready` and the seal, so
  the logo and favicon must remain recognizable without it;
- `書庫` is supporting Traditional Chinese interface text, not part of the
  universal mark;
- the real reading surface, not an illustration, is the primary public product
  image.

## Rejected shortcuts

Do not reduce the product to a generic open book, bookshelf, page curl,
bookmark, magnifying glass, search result, AI sparkle, neural network,
constellation, gradient orb, Obsidian crystal, terminal prompt, dashboard grid,
random Japanese glyph, faux brush calligraphy, distressed ink, or fake hanko.

A direction also fails if it needs an explanatory paragraph before its shape
makes sense, competes with the seal action, looks detached from the shipped UI,
or cannot be reproduced as simple deterministic vector geometry. Generated
images may explore a direction, but generated pixels, traced paths, and opaque
generator output are not shippable identity assets or reproducible build
sources.

## Selected system

The system remains wordmark-first in use: the visible lowercase `yomihon` name
is always the identity and accessible name in the application header. The mark
is a restrained lowercase-`y` construction. Two upright upper strokes turn
inward, converge through one angular reading path, and continue as one vertical
descender. Its single filled silhouette and open upper field keep it legible at
favicon scale without borrowing the ceremonial vermilion used by `ready` and
the seal.

In product chrome and documentation where text can be presented, the mark must
be shown beside the name; it is not a replacement for the product name. The
favicon is the deliberate small-format exception. Do not embellish the mark
into a funnel, martini glass, location pin, generic letter tile, book, page, or
seal. The rejected margin-and-ruling exploration is not an alternate logo.

## Construction and projections

The mark is a deterministic, hand-authored SVG with `viewBox="0 0 32 32"` and
one filled `<path>`. Its canonical SHA-256 is
`44d45c7fc1da3fc765f5cade688ecb29dafd6f7494f7887d0d6fae82acd3f31c`.
It contains no text, font, raster image, filter, gradient, script, external
reference, generator metadata, arbitrary style, or event handler. Its only
style content is an exact, validator-locked `prefers-color-scheme` rule: deep
ink in light browser chrome and light ink in dark browser chrome.

ChatGPT Images 2.0 produced a three-direction concept board for ideation. Codex
selected its Concept A under Koopa's delegation, then authored the final path
coordinates directly in this repository. No generated pixel or vector data was
imported or traced, and no third-party artwork or font outline is part of the
asset. Image generation is therefore recorded conceptual input, while the
reviewable SVG bytes are the final geometry and production source.

Every shipped projection uses those exact source bytes or that exact source as
a CSS mask:

- `/static/yomihon-mark.svg` is the sole served URL and the sole SVG favicon;
- the header uses that URL as a `currentColor` CSS mask beside the live text;
- this repository's README embeds the source file decoratively.

There is no ICO, Apple touch icon, web app manifest, install flow, PWA claim,
or independent wordmark asset. New identity projections require an explicit
canon update; they must not fork or trace the geometry.

The same geometry is checked at 16, 24, 32, and 180 CSS pixels. Critical stems
and gaps must survive the 16-pixel rendering; detail visible only when enlarged
does not count. The required proofs are:

- deep ink on white and warm paper, and light ink on black and charcoal;
- grayscale and forced-colors use, with the textual name still present;
- actual browser tabs at 16 and 32 pixels;
- the real 56-pixel application header in both themes beside the live status
  and seal surfaces.

The visible `yomihon` link remains the exact accessible name. Its neighboring
mark is decorative and `aria-hidden`. The mask takes its color from
`currentColor` in light and dark themes and uses a system-color fallback in
forced-colors mode. Implementation tests pin the deterministic bytes, restricted
SVG grammar and color rule, closed local route, `image/svg+xml` MIME type,
`nosniff` response, single document-head reference, accessible header name,
mask projection, and standalone 16/32-pixel contrast on simulated light and
dark browser backgrounds. Actual browser-tab chrome remains a human release
check; an in-page rendering test does not claim to screenshot the toolbar.

## Selection and release evidence

Hard rejection gates come first: monochrome failure, 16-pixel failure, trope
similarity, conflict with the seal, unexplained geometry, or incompatibility
with the real interface. Surviving directions are compared with these weights:

| Criterion | Weight |
|---|---:|
| Truth to reading and bounded adjudication | 30 |
| Recognition and integrity at 16/32 pixels | 25 |
| Fit with the shipped typography and surfaces | 20 |
| Distinctiveness without novelty theatre | 15 |
| Licensing, accessibility, and production hygiene | 10 |

The selected direction records delegated agent selection and owner-authorized
application. Repository checks cover its construction and projections; they do
not close the human visual gates. Before
`v0.1.0`, human review must still include a five-second product-impression test,
recognition among ordinary browser tabs, delayed recall of the name and
distinguishing feature, comparison in both real application themes, and a
basic similarity and trademark-conflict screen. Agent sessions may verify
assets, references, accessibility, and documentation, but do not substitute
for human visual judgment.

The current self-hosted typefaces remain runtime presentation for the live
text, not a dependency of the SVG. Before public release, Koopa also decides
whether the mark follows the repository's MIT license or has a separate,
plainly stated trademark policy; no restriction is implied before that
decision. Until the remaining human checks and that owner decision are
recorded, this selected identity is not by itself release-readiness evidence.
