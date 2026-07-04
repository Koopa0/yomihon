package asset_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/koopa0/kurodo/internal/asset"
)

// newServer wires asset.Register the same way cmd/kurodo/main.go does and
// starts a real HTTP server, so the tests below exercise net/http's own
// path canonicalization (redirects, ".." collapsing) exactly as production
// traffic would — not a hand-simulated approximation of it.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	asset.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestKnownAssetsServe200 covers every name asset.Register can answer: a
// GET must succeed with the correct Content-Type and a non-empty body.
func TestKnownAssetsServe200(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name       string
		path       string
		wantType   string
		minBodyLen int
	}{
		{name: "kurodo.js", path: "/static/kurodo.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "chroma.css", path: "/static/chroma.css", wantType: "text/css; charset=utf-8", minBodyLen: 100},
		{name: "mermaid entry module", path: "/static/mermaid.esm.min.mjs", wantType: "text/javascript; charset=utf-8", minBodyLen: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(srv.URL + tt.path) //nolint:noctx // fixed, test-controlled URL; test scope, no context needed
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET %s status = %d, want %d", tt.path, resp.StatusCode, http.StatusOK)
			}
			if ct := resp.Header.Get("Content-Type"); ct != tt.wantType {
				t.Errorf("GET %s Content-Type = %q, want %q", tt.path, ct, tt.wantType)
			}
			if ct := resp.Header.Get("X-Content-Type-Options"); ct != "nosniff" {
				t.Errorf("GET %s X-Content-Type-Options = %q, want %q", tt.path, ct, "nosniff")
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body of %s: %v", tt.path, err)
			}
			if len(body) < tt.minBodyLen {
				t.Errorf("GET %s body length = %d, want at least %d", tt.path, len(body), tt.minBodyLen)
			}
		})
	}
}

// TestMermaidEntryModuleChunkResolves proves the vendored mermaid tree's
// directory structure survived intact: one of the entry module's own
// dynamic import() targets (a diagram-type chunk) must also be servable at
// exactly the relative path the browser will request it at.
func TestMermaidEntryModuleChunkResolves(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	resp, err := http.Get(srv.URL + "/static/mermaid.esm.min.mjs") //nolint:noctx // fixed, test-controlled URL
	if err != nil {
		t.Fatalf("GET mermaid entry module: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	entry, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading mermaid entry module: %v", err)
	}

	// Every mermaid.esm.min.mjs release version references its own
	// generated chunk filenames — this fixture doesn't hardcode one, it
	// finds a real one from the file we're serving and confirms the
	// server also answers it, at exactly the path the browser's relative
	// import() resolution would compute.
	loc := indexOfImportChunk(entry)
	if loc == "" {
		t.Fatal("did not find a `./chunks/mermaid.esm.min/*.mjs` dynamic import in the served entry module")
	}

	chunkResp, err := http.Get(srv.URL + "/static/" + loc) //nolint:noctx // fixed, test-controlled URL
	if err != nil {
		t.Fatalf("GET chunk %s: %v", loc, err)
	}
	defer func() { _ = chunkResp.Body.Close() }()
	if chunkResp.StatusCode != http.StatusOK {
		t.Errorf("GET /static/%s status = %d, want %d (this chunk is what the entry module's own dynamic import() would fetch)", loc, chunkResp.StatusCode, http.StatusOK)
	}
}

// indexOfImportChunk extracts one `./chunks/mermaid.esm.min/NAME.mjs`
// reference (without the leading "./") from raw mermaid.esm.min.mjs
// source, or "" if none is found.
func indexOfImportChunk(src []byte) string {
	const marker = "./chunks/mermaid.esm.min/"
	i := bytes.Index(src, []byte(marker))
	if i < 0 {
		return ""
	}
	rest := src[i+2:] // drop "./"
	before, _, found := bytes.Cut(rest, []byte{'"'})
	if !found {
		return ""
	}
	return string(before)
}

// TestUnknownAssetsAre404 is the single most important test in this
// package: this route's whole security property is that it cannot serve
// anything outside its fixed, compile-time-known set. Every name below —
// a plain nonexistent name, path-traversal-shaped names (both raw and
// percent-encoded), and an absolute path — must come back 404 with no
// file content, proving there is no filesystem access on this path at
// all (not just that these particular attempts happen to fail).
func TestUnknownAssetsAre404(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "plain nonexistent name", path: "/static/does-not-exist.js"},
		{name: "nonexistent name shaped like a real one", path: "/static/kurodo.js.bak"},
		{name: "raw dot-dot traversal", path: "/static/../../../../etc/passwd"},
		{name: "dot-dot traversal inside one path segment", path: "/static/..%2F..%2F..%2Fetc%2Fpasswd"},
		{name: "double-encoded dot-dot", path: "/static/%252e%252e%252fetc%252fpasswd"},
		{name: "absolute path as the asset name", path: "/static/%2Fetc%2Fpasswd"},
		{name: "vault content is not on this route", path: "/static/Writing/lessons/golang/Middleware.md"},
		{name: "a real embedded chunk filename with a wrong prefix", path: "/static/mermaid/chunks/mermaid.esm.min/does-not-exist.mjs"},
		// The mermaid LICENSE is embedded (for MIT provenance) but must NOT
		// be servable: embedTree registers only .mjs runtime files, so a
		// non-runtime file in the same tree can't leak onto /static/.
		{name: "vendored LICENSE is embedded but not served", path: "/static/LICENSE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(srv.URL + tt.path) //nolint:noctx // fixed, test-controlled URL
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body of %s: %v", tt.path, err)
			}

			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("GET %s status = %d %s, want %d; body:\n%s", tt.path, resp.StatusCode, resp.Status, http.StatusNotFound, body)
			}
			// Belt-and-braces: even a 404 must never carry real file
			// content (e.g. an "root:" line from an actual /etc/passwd
			// this handler somehow reached).
			if bytes.Contains(body, []byte("root:")) {
				t.Errorf("GET %s returned content that looks like a real file, not a 404 page:\n%s", tt.path, body)
			}
		})
	}
}
