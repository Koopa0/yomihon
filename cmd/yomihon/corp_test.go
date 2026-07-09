package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The refusal exactly as it must appear on the wire. These are spelled out
// rather than taken from the middleware's own constants: a check that compares a
// constant against itself proves only that Go finds equal values equal. It would
// stay green while the server shipped a header named something else, or shipped
// "cross-origin" — which is not a weaker refusal but an invitation.
const (
	wireCORPHeader = "Cross-Origin-Resource-Policy"
	wireCORPValue  = "same-origin"
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
				w.Header().Del(wireCORPHeader)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "a handler that rewrites the header to something weaker",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(wireCORPHeader, "cross-origin")
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
				w.Header().Del(wireCORPHeader)
			},
		},
		{
			name: "a handler that weakens the header and never writes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(wireCORPHeader, "cross-origin")
			},
		},
		{
			// A flush commits the response. Reached through a controller, it
			// would otherwise walk past the wrapper to the writer underneath.
			name: "a handler that deletes the header and flushes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
				_ = http.NewResponseController(w).Flush()
			},
		},
		{
			name: "a handler that deletes the header and flushes by assertion",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/anything", http.NoBody)
			rec := httptest.NewRecorder()
			crossOriginResourcePolicy(tt.inner).ServeHTTP(rec, req)

			// Result reports the headers as they were committed. The recorder's
			// live map would happily show a value that never left, which is how
			// a bypass hides from a test that trusts it.
			if got := rec.Result().Header.Get(wireCORPHeader); got != wireCORPValue {
				t.Errorf("committed %s = %q, want %q", wireCORPHeader, got, wireCORPValue)
			}
		})
	}
}

// TestCorpWriterKeepsTheWriterUnderneathReachable pins that the wrapper hides
// none of the abilities the real writer has. Flushing and hijacking are not
// named on the wrapper itself, so a caller reaches them through an
// http.ResponseController, which follows Unwrap; and the copy that serves a
// file's bytes must still find the writer's own io.ReaderFrom rather than fall
// back to a buffer.
func TestCorpWriterKeepsTheWriterUnderneathReachable(t *testing.T) {
	t.Parallel()

	t.Run("a flush reaches the writer underneath", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		w := &corpWriter{ResponseWriter: rec}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through the wrapper = %v, want it to reach the writer", err)
		}
		if !rec.Flushed {
			t.Error("the writer underneath was never flushed")
		}
	})

	t.Run("the copy fast path is offered, and commits the refusal", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		w := &corpWriter{ResponseWriter: rec}

		// io.Copy reaches for the destination's io.ReaderFrom; hiding it would
		// send every served file through a buffer instead.
		rf, ok := any(w).(io.ReaderFrom)
		if !ok {
			t.Fatal("the wrapper hides the writer's io.ReaderFrom")
		}
		// Called directly, not through io.Copy: io.Copy prefers a source's
		// io.WriterTo, so a copy from a strings.Reader would never reach here
		// and would prove nothing about this path.
		if _, err := rf.ReadFrom(strings.NewReader("bytes")); err != nil {
			t.Fatalf("ReadFrom = %v", err)
		}
		if got := rec.Result().Header.Get(wireCORPHeader); got != wireCORPValue {
			t.Errorf("after a copy, %s = %q, want %q", wireCORPHeader, got, wireCORPValue)
		}
		if rec.Body.String() != "bytes" {
			t.Errorf("body = %q, want %q", rec.Body.String(), "bytes")
		}
	})
}
