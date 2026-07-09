package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCrossOriginResourcePolicy pins the server-wide embed refusal. The listener
// is loopback, but a browser will carry a request to it on behalf of any page
// the reader visits; the header denies that page the embed, so it cannot read a
// vault file's existence or size from a load event or run a servable script file
// in its own origin. It rides on every response, whatever the inner handler does
// or omits.
func TestCrossOriginResourcePolicy(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := crossOriginResourcePolicy(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/anything", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Errorf("Cross-Origin-Resource-Policy = %q, want %q", got, "same-origin")
	}
}
