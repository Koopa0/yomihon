package origin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const wirePermissionsPolicy = "camera=(), microphone=(), geolocation=(), usb=(), serial=(), payment=(), display-capture=()"

func TestProtectPinsPermissionsPolicyOnTheWire(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		commit func(http.ResponseWriter, *http.Request)
		status int
	}{
		{"WriteHeader", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, http.StatusNoContent},
		{"Write", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("body")) //nolint:errcheck // the client checks the response below
		}, http.StatusOK},
		{"ReadFrom", func(w http.ResponseWriter, _ *http.Request) {
			rf, ok := w.(io.ReaderFrom)
			if !ok {
				t.Error("Protect hid ReadFrom")
				return
			}
			if _, err := rf.ReadFrom(strings.NewReader("bytes")); err != nil {
				t.Errorf("ReadFrom: %v", err)
			}
		}, http.StatusOK},
		{"Flush", func(w http.ResponseWriter, _ *http.Request) {
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush: %v", err)
			}
		}, http.StatusOK},
		{"silent", func(http.ResponseWriter, *http.Request) {}, http.StatusOK},
		{"early hints", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			w.Header().Del("Permissions-Policy")
			w.WriteHeader(http.StatusNoContent)
		}, http.StatusNoContent},
		{"not found", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "missing", http.StatusNotFound) }, http.StatusNotFound},
		{"server error", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "failed", http.StatusInternalServerError) }, http.StatusInternalServerError},
		{"raw sandbox", func(w http.ResponseWriter, r *http.Request) {
			if !SetContentSecurityPolicy(r.Context(), w, "sandbox; default-src 'none'; frame-ancestors 'self'") {
				t.Error("raw sandbox policy was not accepted")
			}
			w.WriteHeader(http.StatusOK)
		}, http.StatusOK},
	}
	for _, tt := range tests {
		for _, value := range []string{"", "camera=*"} {
			t.Run(tt.name+"/"+value, func(t *testing.T) {
				t.Parallel()
				server := httptest.NewServer(Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Permissions-Policy", value)
					tt.commit(w, r)
				})))
				t.Cleanup(server.Close)
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
				if err != nil {
					t.Fatalf("NewRequestWithContext: %v", err)
				}
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatalf("GET: %v", err)
				}
				if _, err := io.Copy(io.Discard, response.Body); err != nil {
					t.Errorf("read response: %v", err)
				}
				if err := response.Body.Close(); err != nil {
					t.Errorf("close response: %v", err)
				}
				if response.StatusCode != tt.status {
					t.Fatalf("GET status = %d, want %d", response.StatusCode, tt.status)
				}
				t.Log("invoked: committed permissions policy")
				if got := response.Header.Get("Permissions-Policy"); got != wirePermissionsPolicy {
					t.Errorf("caught: committed Permissions-Policy = %q, want %q", got, wirePermissionsPolicy)
				}
				if tt.name == "raw sandbox" && response.Header.Get("Content-Security-Policy") != "sandbox; default-src 'none'; frame-ancestors 'self'" {
					t.Error("explicit raw sandbox policy changed")
				}
			})
		}
	}
}

func TestProtectKeepsCallerCancellation(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	cancelled := make(chan error, 1)
	server := httptest.NewServer(Protect(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
			cancelled <- r.Context().Err()
		case <-time.After(3 * time.Second):
			cancelled <- errors.New("request cancellation did not reach the handler")
		}
	})))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	finished := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(request)
		if response != nil {
			err = errors.Join(err, response.Body.Close())
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach Protect's handler")
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled client error = %v, want context cancellation", err)
	}
	t.Log("invoked: protected request cancellation")
	if err := <-cancelled; !errors.Is(err, context.Canceled) {
		t.Errorf("caught: handler cancellation = %v, want context cancellation", err)
	}
}
