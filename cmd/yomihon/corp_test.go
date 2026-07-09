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
// in its own origin.
//
// The refusal has to survive whatever the handler beneath it does — answer
// without writing, write without a status, or set the header to something of its
// own. No single endpoint may weaken a policy the whole server rests on.
func TestCrossOriginResourcePolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		inner http.HandlerFunc
	}{
		{
			name:  "a handler that names a status",
			inner: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		},
		{
			name:  "a handler that answers without writing anything",
			inner: func(http.ResponseWriter, *http.Request) {},
		},
		{
			name: "a handler that writes a body without a status",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("body"))
			},
		},
		{
			name: "a handler that deletes the header",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(corpHeader)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "a handler that rewrites the header to something weaker",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(corpHeader, "cross-origin")
				_, _ = w.Write([]byte("body"))
			},
		},
		{
			name: "a handler that fails",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "no", http.StatusNotFound)
			},
		},
		{
			// The quietest way to lose it: no status, no body, nothing for a
			// wrapper to intercept — the server writes the 200 itself, from a
			// header map the handler has already emptied.
			name: "a handler that deletes the header and never writes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(corpHeader)
			},
		},
		{
			name: "a handler that weakens the header and never writes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(corpHeader, "cross-origin")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/anything", http.NoBody)
			rec := httptest.NewRecorder()
			crossOriginResourcePolicy(tt.inner).ServeHTTP(rec, req)

			if got := rec.Header().Get(corpHeader); got != corpValue {
				t.Errorf("%s = %q, want %q", corpHeader, got, corpValue)
			}
		})
	}
}
