package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReturnableAddressRefusesANonGETTarget holds the language form's return
// address to places a GET can revisit: a POST-rendered page names a target,
// not a page, so it falls back to Home.
func TestReturnableAddressRefusesANonGETTarget(t *testing.T) {
	t.Parallel()
	get := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/Notes/a.md?from=draft", http.NoBody)
	if got := returnableAddress(get); got != "/notes/Notes/a.md?from=draft" {
		t.Fatalf("a GET's own address = %q, want it kept", got)
	}
	post := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/status", http.NoBody)
	if got := returnableAddress(post); got != "/" {
		t.Fatalf("a POST target = %q, want the Home fallback", got)
	}
}
