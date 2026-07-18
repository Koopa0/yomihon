package asset

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
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
