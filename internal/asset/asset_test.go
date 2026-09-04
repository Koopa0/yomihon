package asset_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	projectassets "github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/wording"
)

// newServer wires asset.Register the same way cmd/yomihon/main.go does and
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

// TestRepresentativeAssetsServe200 exercises each public asset class through
// the package boundary. The internal registry test enumerates every exact key.
func TestRepresentativeAssetsServe200(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name       string
		path       string
		wantType   string
		minBodyLen int
	}{
		{name: "yomihon.js", path: "/static/yomihon.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "preferences.js", path: "/static/preferences.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "drawer.js", path: "/static/drawer.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "sidebar.js", path: "/static/sidebar.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "contents.js", path: "/static/contents.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "search.js", path: "/static/search.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "shortcuts.js", path: "/static/shortcuts.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "diagrams.js", path: "/static/diagrams.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "lesson.js", path: "/static/lesson.js", wantType: "text/javascript; charset=utf-8", minBodyLen: 100},
		{name: "application stylesheet", path: "/static/app.css", wantType: "text/css; charset=utf-8", minBodyLen: 1000},
		{name: "chroma.css", path: "/static/chroma.css", wantType: "text/css; charset=utf-8", minBodyLen: 100},
		{name: "proportional font", path: "/static/fonts/Geist-Variable.woff2", wantType: "font/woff2", minBodyLen: 1000},
		{name: "monospace font", path: "/static/fonts/GeistMono-Variable.woff2", wantType: "font/woff2", minBodyLen: 1000},
		{name: "reading font", path: "/static/fonts/Newsreader-Variable.woff2", wantType: "font/woff2", minBodyLen: 1000},
		{name: "italic reading font", path: "/static/fonts/Newsreader-Italic-Variable.woff2", wantType: "font/woff2", minBodyLen: 1000},
		{name: "mermaid entry module", path: "/static/mermaid.esm.min.mjs", wantType: "text/javascript; charset=utf-8", minBodyLen: 1000},
		{name: "brand mark", path: "/static/yomihon-mark.svg", wantType: "image/svg+xml", minBodyLen: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := srv.Client().Get(srv.URL + tt.path) //nolint:noctx // fixed, test-controlled URL; test scope, no context needed
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Errorf("close response body: %v", closeErr)
				}
			}()

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

func TestBrandMarkServesExactEmbeddedBytes(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	want, err := projectassets.Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read embedded brand mark: %v", err)
	}
	resp, err := srv.Client().Get(srv.URL + "/static/yomihon-mark.svg") //nolint:noctx // fixed, test-controlled URL
	if err != nil {
		t.Fatalf("GET brand mark: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read brand mark response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/yomihon-mark.svg status = %d, want %d; body:\n%s", resp.StatusCode, http.StatusOK, got)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "image/svg+xml" {
		t.Errorf("GET /static/yomihon-mark.svg Content-Type = %q, want %q", contentType, "image/svg+xml")
	}
	if contentTypeOptions := resp.Header.Get("X-Content-Type-Options"); contentTypeOptions != "nosniff" {
		t.Errorf("GET /static/yomihon-mark.svg X-Content-Type-Options = %q, want %q", contentTypeOptions, "nosniff")
	}
	if !bytes.Equal(got, want) {
		t.Errorf("GET /static/yomihon-mark.svg body differs from the canonical embedded bytes")
	}
}

// TestMermaidEntryModuleChunkResolves proves the vendored mermaid tree's
// directory structure survived intact: one of the entry module's own
// dynamic import() targets (a diagram-type chunk) must also be servable at
// exactly the relative path the browser will request it at.
func TestMermaidEntryModuleChunkResolves(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	resp, err := srv.Client().Get(srv.URL + "/static/mermaid.esm.min.mjs") //nolint:noctx // fixed, test-controlled URL
	if err != nil {
		t.Fatalf("GET mermaid entry module: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
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

	chunkResp, err := srv.Client().Get(srv.URL + "/static/" + loc) //nolint:noctx // fixed, test-controlled URL
	if err != nil {
		t.Fatalf("GET chunk %s: %v", loc, err)
	}
	defer func() {
		if closeErr := chunkResp.Body.Close(); closeErr != nil {
			t.Errorf("close chunk response body: %v", closeErr)
		}
	}()
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
		{name: "nonexistent name shaped like a real one", path: "/static/yomihon.js.bak"},
		{name: "unregistered generic utility", path: "/static/utils.js"},
		{name: "unregistered generic helpers", path: "/static/helpers.js"},
		{name: "registered module backup", path: "/static/preferences.js.bak"},
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
		{name: "font LICENSE is embedded but not served", path: "/static/fonts/LICENSE.txt"},
		{name: "brand directory alias", path: "/static/brand/yomihon-mark.svg"},
		{name: "brand mark backup alias", path: "/static/yomihon-mark.svg.bak"},
		{name: "brand mark wrong-case alias", path: "/static/Yomihon-mark.svg"},
		{name: "static favicon alias", path: "/static/favicon.svg"},
		{name: "root favicon alias", path: "/favicon.svg"},
		{name: "legacy ico alias", path: "/favicon.ico"},
		{name: "apple touch alias", path: "/apple-touch-icon.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := srv.Client().Get(srv.URL + tt.path) //nolint:noctx // fixed, test-controlled URL
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Errorf("close response body: %v", closeErr)
				}
			}()

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

// TestConditionalRequestAnswers304 locks the revalidation contract: a stored
// copy is confirmed with a bodiless 304, and a copy that does not match is
// replaced with the real bytes. The mismatch case is the load-bearing one —
// without it a handler that answered 304 unconditionally would pass, and so
// would one whose tag is unquoted, since http.ServeContent ignores a tag it
// cannot parse and falls through to 200 for every request.
func TestConditionalRequestAnswers304(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "embedded stylesheet", path: "/static/app.css"},
		{name: "computed stylesheet", path: "/static/chroma.css"},
		{name: "embedded module", path: "/static/yomihon.js"},
		{name: "embedded font", path: "/static/fonts/Geist-Variable.woff2"},
		{name: "vendored module", path: "/static/mermaid.esm.min.mjs"},
		{name: "brand mark", path: "/static/yomihon-mark.svg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			first := get(t, srv.Client(), srv.URL+tt.path, "")
			if first.status != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, first.status, http.StatusOK)
			}
			tag := first.header.Get("ETag")
			if tag == "" {
				t.Fatalf("GET %s served no ETag, so no stored copy can ever be revalidated", tt.path)
			}
			if !strings.HasPrefix(tag, `"`) || !strings.HasSuffix(tag, `"`) {
				t.Fatalf("GET %s ETag = %s, want a quoted strong tag; an unquoted tag is ignored and never answers a conditional request", tt.path, tag)
			}
			if got := first.header.Get("Cache-Control"); got != "no-cache" {
				t.Errorf("GET %s Cache-Control = %q, want %q", tt.path, got, "no-cache")
			}
			if len(first.body) == 0 {
				t.Fatalf("GET %s served an empty body", tt.path)
			}

			// The tag a browser would quote back confirms the stored copy.
			match := get(t, srv.Client(), srv.URL+tt.path, tag)
			if match.status != http.StatusNotModified {
				t.Errorf("GET %s with If-None-Match %s status = %d, want %d", tt.path, tag, match.status, http.StatusNotModified)
			}
			if len(match.body) != 0 {
				t.Errorf("GET %s answered %d with a %d-byte body, want none", tt.path, match.status, len(match.body))
			}

			// A copy of different bytes is replaced, not confirmed.
			stale := get(t, srv.Client(), srv.URL+tt.path, `"not-the-bytes-you-have"`)
			if stale.status != http.StatusOK {
				t.Errorf("GET %s with a non-matching If-None-Match status = %d, want %d", tt.path, stale.status, http.StatusOK)
			}
			if !bytes.Equal(stale.body, first.body) {
				t.Errorf("GET %s with a non-matching If-None-Match served %d bytes, want the same %d as the unconditional request", tt.path, len(stale.body), len(first.body))
			}

			// The validator names the bytes, so it does not drift between
			// requests; a tag that changed every time would revalidate to a
			// 200 forever and cache nothing.
			if again := first.header.Get("ETag"); again != get(t, srv.Client(), srv.URL+tt.path, "").header.Get("ETag") {
				t.Errorf("GET %s served two different ETags for the same bytes", tt.path)
			}
		})
	}
}

// response is the part of an HTTP answer these tests judge.
type response struct {
	status int
	header http.Header
	body   []byte
}

// get issues one request, optionally quoting a stored validator back at the
// server, and reads the answer to completion.
func get(t *testing.T, client *http.Client, url, ifNoneMatch string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("building request for %s: %v", url, err)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", url, err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: body}
}

// TestAMissingAssetSpeaksTheReadersLanguage covers the one sentence this
// package writes for a person. It used to resolve to the default language at
// the call site, before any request existed to ask — so a reader working in
// the other language met one untranslated line, on the response that already
// told them nothing was there.
func TestAMissingAssetSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cookie string
		want   wording.Lang
	}{
		{name: "no choice means the default", want: wording.ZhHant},
		{name: "a reader who chose English", cookie: string(wording.En), want: wording.En},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			asset.Register(mux)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/nothing-here.js", http.NoBody)
			if tt.cookie != "" {
				request.Header.Set("Cookie", wording.CookieName+"="+tt.cookie)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			want := wording.AssetNotFound.In(tt.want)
			if got := strings.TrimSpace(response.Body.String()); got != want {
				t.Errorf("refusal = %q, want %q", got, want)
			}
		})
	}
}
