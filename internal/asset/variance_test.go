package asset_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestStaticCookieVariance(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	asset.Register(mux)
	handler := origin.Protect(mux)
	url := asset.Versions{}.URL("app.css")
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody))
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("stylesheet has no validator")
	}
	for _, tc := range []struct {
		name, method, url, validator, byteRange, vary string
		status                                        int
	}{
		{"versioned", "GET", url, "", "", "", 200},
		{"bare", "GET", "/static/app.css", "", "", "", 200},
		{"head", "HEAD", url, "", "", "", 200},
		{"conditional", "GET", url, tag, "", "", 304},
		{"range", "GET", url, "", "bytes=0-9", "", 206},
		{"missing", "GET", "/static/missing.js", "", "", "Cookie", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []string{"en", "zh-Hant"} {
				r := httptest.NewRequestWithContext(t.Context(), tc.method, tc.url, http.NoBody)
				r.AddCookie(&http.Cookie{Name: wording.CookieName, Value: lang, HttpOnly: true, SameSite: http.SameSiteStrictMode}) // #nosec G124 -- loopback HTTP fixture cookie, not a production cookie
				r.Header.Set("If-None-Match", tc.validator)
				r.Header.Set("Range", tc.byteRange)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code != tc.status {
					t.Fatalf("status = %d, want %d", w.Code, tc.status)
				}
				if got := w.Result().Header.Get("Vary"); got != tc.vary {
					t.Errorf("%s Vary = %q, want %q", lang, got, tc.vary)
				}
				if tc.status == 404 && !strings.Contains(w.Body.String(), wording.AssetNotFound.In(wording.FromCookieValue(lang))) {
					t.Error("missing asset lost its localized sentence")
				}
			}
		})
	}
}
