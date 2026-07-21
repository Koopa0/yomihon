// Package asset serves yomihon's fixed, compile-time-known set of static
// files: the official brand mark, the vendored mermaid ES-module runtime,
// yomihon's own fixed native client-module graph, and generated stylesheets.
//
// This is the FIRST route in this repo that serves something other than
// rendered vault content, and its entire security property rests on one
// invariant: registry (built once, at package init, from what is
// compiled into the binary via github.com/koopa0/yomihon/assets'
// embed.FS, or computed in memory by render.ChromaCSS) is a fixed, closed
// map. serve does exactly one thing with request input — an exact lookup
// of r.PathValue("path") against that map — and nothing else. There is no
// filepath.Join, no os.Open, no directory listing, and no way for a
// request to add, remove, or address anything outside the set decided at
// build time. That is deliberately narrower than the deferred
// vault-media-embed route (images/PDFs for the embed feature — still not
// built, and this package must never grow into that): serving arbitrary
// vault content is a separate, larger design question this package does
// not answer.
package asset

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/render"
)

const (
	jsContentType    = "text/javascript; charset=utf-8"
	cssContentType   = "text/css; charset=utf-8"
	svgContentType   = "image/svg+xml"
	woff2ContentType = "font/woff2"
)

// entry is one servable asset: its Content-Type and its body. body is a
// func rather than a plain []byte so registry can hold both
// already-resolved embedded bytes and render.ChromaCSS's lazily-computed
// (but internally memoized, via sync.OnceValue) stylesheet uniformly.
type entry struct {
	contentType string
	body        func() []byte
}

// registry is yomihon's entire static-asset name space, built once at
// package init (see buildRegistry) and never mutated afterward.
var registry = buildRegistry()

// buildRegistry assembles the fixed asset set: the explicitly named product
// files and the whole vendored mermaid/ subtree from assets.Files (see that
// package's doc comment for why the mermaid tree is more than one file), plus
// render.ChromaCSS's computed stylesheet, which has no embedded file backing
// it at all.
func buildRegistry() map[string]entry {
	reg := map[string]entry{
		"chroma.css": {
			contentType: cssContentType,
			body:        func() []byte { return []byte(render.ChromaCSS()) },
		},
	}
	for _, name := range []string{
		"yomihon.js",
		"preferences.js",
		"drawer.js",
		"sidebar.js",
		"contents.js",
		"status.js",
		"search.js",
		"shortcuts.js",
		"diagrams.js",
		"lesson.js",
	} {
		embedFile(reg, name, "js/"+name, jsContentType)
	}
	embedFile(reg, "app.css", "css/output.css", cssContentType)
	embedFile(reg, "yomihon-mark.svg", "brand/yomihon-mark.svg", svgContentType)
	embedTree(reg, "js/mermaid")
	embedFonts(reg, "fonts")
	return reg
}

// embedFile registers one embedded file under name, read once here (never
// per-request). A missing file is a build-time invariant violation (the
// embed directive in assets/assets.go and this call site have drifted
// apart) — panicking at package init surfaces that immediately, the same
// way render.New panics on a nil required dependency, rather than letting
// every request for name silently 404 forever.
func embedFile(reg map[string]entry, name, embeddedPath, contentType string) {
	b, err := assets.Files.ReadFile(embeddedPath)
	if err != nil {
		panic(fmt.Sprintf("asset: embedded file missing: %s: %v", embeddedPath, err))
	}
	reg[name] = entry{contentType: contentType, body: func() []byte { return b }}
}

// embedFonts registers every .woff2 under dir (self-hosted, vendored under
// assets/fonts/), keyed by its full embedded path — so assets/fonts/Geist-
// Variable.woff2 becomes the URL-facing name "fonts/Geist-Variable.woff2",
// exactly the path fonts.css's @font-face src references at /static/fonts/….
// Only .woff2 is registered; any LICENSE/README dropped beside the fonts is
// deliberately not servable, the same closed-set discipline as embedTree.
func embedFonts(reg map[string]entry, dir string) {
	err := fs.WalkDir(assets.Files, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".woff2") {
			return nil
		}
		b, rerr := assets.Files.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		reg[p] = entry{contentType: woff2ContentType, body: func() []byte { return b }}
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("asset: embedding %s: %v", dir, err))
	}
}

// embedTree registers every .mjs file under dir (recursively), keyed by its
// path relative to dir — so assets/js/mermaid/chunks/mermaid.esm.min/x.mjs
// becomes the URL-facing name "chunks/mermaid.esm.min/x.mjs", exactly the
// relative path mermaid's own dynamic import() calls expect next to
// mermaid.esm.min.mjs (see assets.Files's doc comment).
//
// Only .mjs is registered because only .mjs is runtime. Non-runtime files
// living in the same tree — the vendored mermaid LICENSE (embedded for
// provenance, MIT requires the notice travel with the copied code), or any
// future README / source map — are deliberately NOT servable: this keeps
// the closed set exactly the runtime it needs to be, so dropping a
// documentation file next to the bundle can never silently expose it at
// /static/ with a text/javascript content-type.
func embedTree(reg map[string]entry, dir string) {
	err := fs.WalkDir(assets.Files, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".mjs") {
			return nil
		}
		b, rerr := assets.Files.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		name := strings.TrimPrefix(p, dir+"/")
		reg[name] = entry{contentType: jsContentType, body: func() []byte { return b }}
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("asset: embedding %s: %v", dir, err))
	}
}

// Register mounts the static-asset route.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /static/{path...}", serve)
}

// serve answers a fixed-asset request: an exact registry lookup on the
// request path, nothing else. Any name not already a registry key —
// including anything shaped like a path-traversal attempt — is a 404;
// there is no filesystem access on this path at all, so there is nothing
// for such a name to traverse into.
func serve(w http.ResponseWriter, r *http.Request) {
	e, ok := registry[r.PathValue("path")]
	if !ok {
		http.Error(w, "找不到指定的資產", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", e.contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(e.body()) //nolint:errcheck // response is committed and Handler has no later recovery channel
}
