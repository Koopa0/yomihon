# Brand brief

Status: **identity brief, not a selected mark**. This document defines what a
future logo and favicon must communicate and how they are judged. It does not
authorize an arbitrary icon to fill an empty asset slot.

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
or cannot be reproduced as simple deterministic vector geometry. Image
generation is not a source for the logo or favicon.

## Direction to explore first

Start with a wordmark-first system: a deliberately drawn lowercase `yomihon`
wordmark that grows from the proportions and restraint of the existing header
rather than attaching a foreign symbol to it. One identifiable construction
detail may be reduced into the favicon, but the favicon must remain part of the
same system rather than an unrelated glyph.

Two secondary territories may be compared, not presumed correct:

1. A restrained `y` whose upper strokes converge into one reading path.
2. A margin-and-ruling construction with at most two major shapes and one
   negative-space relationship.

Reject the `y` if it reads as a funnel, martini glass, location pin, or generic
letter tile. Reject the margin direction if it reads as a menu, list, document,
or code editor.

## Construction contract

The selected asset is a reviewable, hand-authored SVG with a fixed `viewBox`
and filled geometry. It contains no `<text>`, embedded font, raster image,
filter, gradient, script, external reference, or generator metadata.

The same geometry is checked at 16, 24, 32, and 180 CSS pixels. Critical stems
and gaps must survive the 16-pixel rendering; detail visible only when enlarged
does not count. The required proofs are:

- pure black on white and pure white on black;
- product ink on warm paper and light ink on charcoal;
- grayscale and forced-colors use, with the textual name still present;
- actual browser tabs at 16 and 32 pixels;
- the real 56-pixel application header in both themes beside the live status
  and seal surfaces.

The visible `yomihon` link remains the accessible name. A neighboring mark is
normally decorative and `aria-hidden`. Implementation tests must pin local
serving, the SVG MIME type, the document-head reference, absence of external
requests, and deterministic asset bytes. A favicon does not imply a manifest,
install flow, or PWA.

## Selection evidence

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

Before `v0.1.0`, human review must include a five-second product-impression
test, recognition among ordinary browser tabs, delayed recall of the name and
distinguishing feature, comparison in both real application themes, and a
basic similarity and trademark-conflict screen. Agent sessions may verify
assets, references, accessibility, and documentation, but do not substitute
for human visual judgment.

The current self-hosted typefaces are optical references, not runtime logo
dependencies. Final wordmark paths need documented provenance. Before public
release, Koopa also decides whether the mark follows the repository's MIT
license or has a separate, plainly stated trademark policy; no restriction is
implied before that decision.
