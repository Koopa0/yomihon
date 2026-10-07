package layouts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	projectassets "github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/asset"
)

func TestBaseAndStylesheetUseCompleteServedByteAddresses(t *testing.T) {
	t.Parallel()
	var rendered bytes.Buffer
	if err := Base(Chrome{Title: "identity", Assets: asset.Versions{}}).Render(t.Context(), &rendered); err != nil {
		t.Fatalf("render Base: %v", err)
	}
	mux := http.NewServeMux()
	asset.Register(mux)
	get := func(address string) []byte {
		t.Helper()
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("GET %s status = %d, body bytes = %d, want 200 and nonempty body", address, response.Code, response.Body.Len())
		}
		return response.Body.Bytes()
	}
	check := func(address string) string {
		t.Helper()
		parsed, err := url.Parse(address)
		if err != nil {
			t.Fatalf("parse asset address %q: %v", address, err)
		}
		if parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/static/") || parsed.EscapedPath() != parsed.Path {
			t.Fatalf("caught: noncanonical resource address %q, want a local /static/ path", address)
		}
		body := get(address)
		sum := sha256.Sum256(body)
		want := hex.EncodeToString(sum[:])[:12]
		if parsed.RawQuery != "v="+want {
			t.Errorf("caught: served-byte address %q, want %s?v=%s", address, parsed.Path, want)
		}
		return parsed.Path
	}
	paths := make(map[string]int)
	declarations := 0
	// Extract every resource address before decoding or classifying it. A
	// character reference can spell /static/ without those literal bytes.
	for _, tag := range regexp.MustCompile(`<(?:link|img|script)\b[^>]*>`).FindAllString(rendered.String(), -1) {
		for _, attr := range regexp.MustCompile(`\b(?:href|src)="([^"<>]*)"`).FindAllStringSubmatch(tag, -1) {
			address := html.UnescapeString(attr[1])
			// The render-blocking expect link identifies a landmark, not a
			// fetched resource. It is Base's only non-resource link element.
			if strings.HasPrefix(tag, "<link ") && strings.Contains(tag, `rel="expect"`) && address == "#_y-main" {
				continue
			}
			declarations++
			paths[check(address)]++
		}
	}
	if declarations != 8 {
		t.Errorf("Base static link/img/script declarations = %d, want 8", declarations)
	}
	wantPaths := map[string]int{
		"/static/app.css":                               1,
		"/static/chroma.css":                            1,
		"/static/yomihon.js":                            1,
		"/static/yomihon-mark.svg":                      2,
		"/static/fonts/Geist-Variable.woff2":            1,
		"/static/fonts/GeistMono-Variable.woff2":        1,
		"/static/fonts/Newsreader-Latin-Variable.woff2": 1,
	}
	if diff := cmp.Diff(wantPaths, paths); diff != "" {
		t.Errorf("Base complete static declarations (-want +got):\n%s", diff)
	}
	maps := regexp.MustCompile(`(?s)<script[^>]*type="importmap"[^>]*>(.*?)</script>`).FindAllStringSubmatch(rendered.String(), -1)
	if len(maps) != 1 {
		t.Fatalf("Base import maps = %d, want 1", len(maps))
	}
	var imports struct {
		Imports map[string]string `json:"imports"`
	}
	if err := json.Unmarshal([]byte(maps[0][1]), &imports); err != nil {
		t.Fatalf("decode Base import map: %v", err)
	}
	wantImports := make(map[string]string)
	err := fs.WalkDir(projectassets.Files, "js", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || name != "js/mermaid/mermaid.esm.min.mjs" && (strings.Count(name, "/") != 1 || !strings.HasSuffix(name, ".js")) {
			return nil
		}
		key := strings.TrimPrefix(name, "js/")
		if name == "js/mermaid/mermaid.esm.min.mjs" {
			key = "mermaid.esm.min.mjs"
		}
		path := "/static/" + key
		sum := sha256.Sum256(get(path))
		wantImports[path] = path + "?v=" + hex.EncodeToString(sum[:])[:12]
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded Base modules: %v", err)
	}
	if len(wantImports) != 19 {
		t.Fatalf("embedded Base module set = %d, want 19", len(wantImports))
	}
	if diff := cmp.Diff(wantImports, imports.Imports); diff != "" {
		t.Errorf("caught: Base complete module addresses (-want +got):\n%s", diff)
	}
	for key, address := range imports.Imports {
		if got := check(address); got != key {
			t.Errorf("import key %q resolves to path %q", key, got)
		}
	}
	if _, ok := imports.Imports["/static/mermaid.esm.min.mjs"]; !ok {
		t.Error("caught: Base import map omits Mermaid facade")
	}
	css := string(get(asset.Versions{}.URL("app.css")))
	cssURLs := regexp.MustCompile(`url\(['"]?(/static/[^)'"\s]+)['"]?\)`).FindAllStringSubmatch(css, -1)
	fontPaths := make(map[string]int)
	fontAddresses := make(map[string]string)
	for _, match := range cssURLs {
		path := check(match[1])
		fontPaths[path]++
		fontAddresses[path] = match[1]
	}
	files, err := projectassets.Files.ReadDir("fonts")
	if err != nil {
		t.Fatalf("read embedded fonts: %v", err)
	}
	wantFonts := make(map[string]int)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".woff2") {
			wantFonts["/static/fonts/"+file.Name()] = 1
		}
	}
	if len(wantFonts) != 6 {
		t.Fatalf("embedded font set = %d, want 6", len(wantFonts))
	}
	if diff := cmp.Diff(wantFonts, fontPaths); diff != "" {
		t.Errorf("served CSS complete font set (-want +got):\n%s", diff)
	}
	// Normal basic-Latin faces are first-paint fonts; italic and extended
	// subsets are demand-loaded. Derive the set from the actual declarations.
	firstPaint := make(map[string]int)
	for _, block := range regexp.MustCompile(`(?s)@font-face\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		if !strings.Contains(block[1], "font-style: normal;") || strings.Contains(block[1], "unicode-range:") && !strings.Contains(block[1], "unicode-range: U+0000-") {
			continue
		}
		urls := regexp.MustCompile(`url\(['"]?(/static/[^)'"\s]+)['"]?\)`).FindAllStringSubmatch(block[1], -1)
		if len(urls) != 1 {
			t.Fatalf("first-paint font URLs = %d, want 1: %s", len(urls), block[1])
		}
		firstPaint[urls[0][1]]++
	}
	preloads := make(map[string]int)
	for _, link := range regexp.MustCompile(`<link\b[^>]*>`).FindAllString(rendered.String(), -1) {
		if !strings.Contains(link, `rel="preload"`) || !strings.Contains(link, `as="font"`) {
			continue
		}
		address := regexp.MustCompile(`href="([^"]+)"`).FindStringSubmatch(link)
		if len(address) != 2 {
			t.Fatalf("font preload has no address: %s", link)
		}
		decoded := html.UnescapeString(address[1])
		preloads[decoded]++
		path := check(decoded)
		if decoded != fontAddresses[path] {
			t.Errorf("caught: preload %q differs from CSS font address %q", decoded, fontAddresses[path])
		}
	}
	if len(firstPaint) != 3 {
		t.Fatalf("normal basic-Latin first-paint font set = %d, want 3", len(firstPaint))
	}
	if diff := cmp.Diff(firstPaint, preloads); diff != "" {
		t.Errorf("caught: complete first-paint preload addresses (-want +got):\n%s", diff)
	}
}
