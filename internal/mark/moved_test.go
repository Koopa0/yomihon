package mark_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// TestRetiredAddressesRedirectAndKeepTheMethod holds the compatibility of the
// two addresses a page drawn before the rename still posts to. A 308 keeps the
// method and the body, so a bookmark or a question sent from such a page still
// arrives; a 301 would turn the POST into a GET and lose it.
func TestRetiredAddressesRedirectAndKeepTheMethod(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.DiscardHandler)
	file := newFile(t)
	mux := http.NewServeMux()
	mark.NewHandler(file, vaultPlaces(nil), log).Register(mux)
	mark.NewQuestionHandler(file, vaultPlaces(nil), log).Register(mux)

	// The requests run in order on one mux: a redirect target that remembered
	// an earlier request's query would show on the one after it.
	for _, tt := range []struct {
		name, method, target, want string
	}{
		{"the old bookmark address", http.MethodPost, "/marks", "/bookmark"},
		{"the old bookmark address with a query", http.MethodPost, "/marks?a=1", "/bookmark?a=1"},
		{"the old bookmark address again without one", http.MethodPost, "/marks", "/bookmark"},
		{"the old question list", http.MethodGet, "/uncertainties", "/questions"},
		{"the old question toggle", http.MethodPost, "/uncertainties", "/questions"},
		{"the old question toggle with a query", http.MethodPost, "/uncertainties?b=2", "/questions?b=2"},
	} {
		request := httptest.NewRequestWithContext(t.Context(), tt.method, tt.target, strings.NewReader("path=a.md"))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusPermanentRedirect {
			t.Errorf("%s: %s %s = %d, want 308", tt.name, tt.method, tt.target, response.Code)
		}
		if got := response.Header().Get("Location"); got != tt.want {
			t.Errorf("%s: %s %s Location = %q, want %q", tt.name, tt.method, tt.target, got, tt.want)
		}
	}
}
