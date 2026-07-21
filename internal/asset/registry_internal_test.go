package asset

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/assets"
)

func TestEveryRegisteredAssetServesItsExactEntry(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Register(mux)
	for name, want := range registry {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/"+name, http.NoBody)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET /static/%s status = %d, want %d", name, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != want.contentType {
				t.Errorf("GET /static/%s Content-Type = %q, want %q", name, got, want.contentType)
			}
			if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("GET /static/%s X-Content-Type-Options = %q, want nosniff", name, got)
			}
			if got := response.Body.Bytes(); len(got) == 0 {
				t.Errorf("GET /static/%s body is empty", name)
			}
		})
	}
}

func TestProductScriptRegistryIsExact(t *testing.T) {
	t.Parallel()
	want := []string{
		"contents.js",
		"diagrams.js",
		"drawer.js",
		"lesson.js",
		"preferences.js",
		"search.js",
		"shortcuts.js",
		"sidebar.js",
		"status.js",
		"yomihon.js",
	}
	var got []string
	for name := range registry {
		if strings.HasSuffix(name, ".js") {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("registered product scripts = %q, want exact closed set %q", got, want)
	}
}

func TestBrandMarkRegistryIsExact(t *testing.T) {
	t.Parallel()

	wantBody, err := assets.Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read embedded brand mark: %v", err)
	}
	wantNames := []string{"yomihon-mark.svg"}
	var gotSVGNames []string
	var gotBodyNames []string
	for name, candidate := range registry {
		if strings.HasSuffix(name, ".svg") {
			gotSVGNames = append(gotSVGNames, name)
		}
		if bytes.Equal(candidate.body(), wantBody) {
			gotBodyNames = append(gotBodyNames, name)
		}
	}
	slices.Sort(gotSVGNames)
	slices.Sort(gotBodyNames)
	if !slices.Equal(gotSVGNames, wantNames) {
		t.Errorf("registered SVG assets = %q, want exact closed set %q", gotSVGNames, wantNames)
	}
	if !slices.Equal(gotBodyNames, wantNames) {
		t.Errorf("registry names serving the canonical brand bytes = %q, want exact closed set %q", gotBodyNames, wantNames)
	}

	got, ok := registry["yomihon-mark.svg"]
	if !ok {
		t.Fatal("registry has no yomihon-mark.svg entry")
	}
	if got.contentType != "image/svg+xml" {
		t.Errorf("yomihon-mark.svg content type = %q, want %q", got.contentType, "image/svg+xml")
	}
	if gotBody := got.body(); !bytes.Equal(gotBody, wantBody) {
		t.Errorf("yomihon-mark.svg registry body differs from the embedded source")
	}
}
