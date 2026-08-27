// Package asset serves yomihon's fixed, compile-time-known set of static
// files: the vendored mermaid ES-module runtime, yomihon's own fixed native
// client-module graph, the canonical brand mark, and the generated chroma
// stylesheet.
//
// Its entire security property rests on one invariant: registry (built
// once, at package init, from what is compiled into the binary via
// github.com/koopa0/yomihon/assets' embed.FS, or computed in memory by
// render.ChromaCSS) is a fixed, closed map. serve does exactly one thing
// with request input — an exact lookup of r.PathValue("path") against that
// map — and nothing else. There is no filepath.Join, no os.Open, no
// directory listing, and no way for a request to add, remove, or address
// anything outside the set decided at build time.
//
// Handing a reader arbitrary bytes out of the vault is a different problem
// with a different answer, and this package is not where it was answered.
// The route that serves a vault image or PDF lives in internal/note, and it
// admits a path by rule and by membership of the current scan rather than by
// lookup in a closed set. An audit of how vault files reach a browser has to
// read that route; nothing about this one constrains it. This package is
// narrower on purpose and must not grow into it.
package asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/render"
)

const (
	jsContentType    = "text/javascript; charset=utf-8"
	cssContentType   = "text/css; charset=utf-8"
	svgContentType   = "image/svg+xml"
	woff2ContentType = "font/woff2"
)

// entry is one servable asset: its Content-Type, its body, and the validator
// a browser may quote back to ask whether its stored copy is still good. body
// and etag are funcs rather than plain values so registry can hold both
// already-resolved embedded bytes and render.ChromaCSS's lazily-computed
// stylesheet uniformly; each lazy one is memoized so the work happens once
// rather than per request.
type entry struct {
	contentType string
	body        func() []byte
	etag        func() string
}

// etagOf is the strong validator for one asset's bytes. The quotes are
// load-bearing: http.ServeContent ignores a tag that is not quoted, which
// would leave every response looking cacheable while never once answering a
// conditional request with 304.
func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// fixed builds an entry over bytes already known — every embedded asset — so
// both the body and its validator are resolved before the first request.
func fixed(contentType string, body []byte) entry {
	tag := etagOf(body)
	return entry{
		contentType: contentType,
		body:        func() []byte { return body },
		etag:        func() string { return tag },
	}
}

// registry is yomihon's entire static-asset name space, built once at
// package init (see buildRegistry) and never mutated afterward.
var registry = buildRegistry()

// buildRegistry assembles the fixed asset set: the explicitly named product
// modules, canonical brand mark, and whole vendored mermaid/ subtree from
// assets.Files (see that package's doc comment for why the mermaid tree is
// more than one file), plus render.ChromaCSS's computed stylesheet, which has
// no embedded file backing it at all.
func buildRegistry() map[string]entry {
	chroma := sync.OnceValue(func() []byte { return []byte(render.ChromaCSS()) })
	reg := map[string]entry{
		"chroma.css": {
			contentType: cssContentType,
			body:        chroma,
			etag:        sync.OnceValue(func() string { return etagOf(chroma()) }),
		},
	}
	for _, name := range []string{
		"yomihon.js",
		"preferences.js",
		"drawer.js",
		"sidebar.js",
		"contents.js",
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
	reg[name] = fixed(contentType, b)
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
		reg[p] = fixed(woff2ContentType, b)
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
		reg[name] = fixed(jsContentType, b)
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
	// These names carry no build fingerprint, so a stored copy has to be asked
	// about rather than trusted for a period: no-cache keeps the copy and
	// revalidates every time, and the strong tag turns that question into a
	// bodiless 304. Without this the reader re-downloads every stylesheet,
	// module and font on each navigation, and two of those block painting.
	// The modification time is deliberately zero — bytes baked into the binary
	// have none, and inventing one would put a second, weaker validator beside
	// the tag. http.ServeContent owns the conditional request, the byte range
	// and the content length from here; hand-rolling that comparison gets the
	// multi-tag and weak-tag forms wrong.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", e.etag())
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(e.body()))
}
