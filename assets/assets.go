// Package assets embeds kurodo's small, fixed set of vendored and
// hand-written client-side files. This file — not internal/asset — is
// where the go:embed directive has to live: an embed pattern can only
// reach files at or below the declaring .go file's own directory (no
// "../"), and this repo's static-asset directory convention (the
// Makefile's css target, mirroring yomihon's assets/{css,js,fonts}) fixes
// assets/ at the module root, not under internal/. internal/asset reads
// Files at package init to build its fixed, closed name→content registry;
// it never reads this package's contents at request time.
package assets

import "embed"

// Files holds:
//
//   - js/kurodo.js — kurodo's own hand-written client script (see that
//     file's doc comment).
//
//   - js/mermaid/ — the vendored mermaid@11.15.0 ES-module runtime.
//     mermaid's published ESM build is itself code-split: dist/mermaid.esm.min.mjs
//     is a ~28KB facade whose dynamic import() calls pull in
//     dist/chunks/mermaid.esm.min/*.mjs (one file per diagram type plus
//     shared chunks, ~3.1MB total) on demand. Vendoring only the facade
//     would leave every real diagram broken at runtime (a 404 on its
//     first dynamic import) — a bug the research phase's "few-MB
//     ballpark" size expectation actually anticipates once you account
//     for the full chunk tree, not just the entry file. The whole tree
//     (js/mermaid/mermaid.esm.min.mjs + js/mermaid/chunks/mermaid.esm.min/*.mjs,
//     source maps excluded — dev-only, no runtime benefit) is vendored
//     here, directory structure preserved exactly, so the entry module's
//     relative imports resolve the same way under internal/asset's
//     /static/ route as they do in mermaid's own dist/ layout.
//     Source: https://cdn.jsdelivr.net/npm/mermaid@11.15.0/dist/ (fetched
//     2026-07-02; update by re-fetching a newer @<version> from the same
//     path and re-running the same file list).
//
//   - css/output.css — the Tailwind v4 stylesheet, built by `make css`
//     from css/input.css (@import tailwindcss + the design tokens + the
//     product layer). Served at /static/app.css. Committed like the
//     generated *_templ.go so `go build ./...` needs no prior css step.
//
//   - fonts/ — the self-hosted woff2 (Geist, Geist Mono, Newsreader),
//     served at /static/fonts/*.woff2 by fonts.css's @font-face, so no
//     request ever leaves the machine (D-brief: zero external requests).
//
//go:embed js/kurodo.js js/mermaid css/output.css fonts
var Files embed.FS
