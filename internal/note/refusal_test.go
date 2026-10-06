package note

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

// TestLocalizedRouterKeepsRegisteredResponses observes the response on a real
// HTTP connection: wildcard values, raw ranges, HEAD, redirects and a feature's
// own 405 all retain the meaning assigned by that registered handler.
func TestLocalizedRouterKeepsRegisteredResponses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /echo/{path...}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := io.WriteString(w, r.PathValue("path")+"|"+r.Pattern); err != nil { // #nosec G705 -- plain-text routing fixture, never HTML
			t.Error(err)
		}
	})
	mux.HandleFunc("GET /raw/{name}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, r.PathValue("name"), time.Time{}, strings.NewReader("abcdef"))
	})
	mux.HandleFunc("GET /owned", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "PATCH")
		w.WriteHeader(http.StatusMethodNotAllowed)
		if _, err := io.WriteString(w, "feature refusal"); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("GET /silent", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /flush", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Flushed", "yes")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		if _, err := io.WriteString(w, "stream"); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("GET /tree/", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, "tree"); err != nil {
			t.Error(err)
		}
	})
	h := &Handler{}
	server := httptest.NewServer(h.LocalizedRouter(mux))
	t.Cleanup(server.Close)
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	tests := []struct {
		name, method, path, rangeValue, body, allow, location string
		code                                                  int
	}{
		{name: "wildcard", method: "GET", path: "/echo/a%20b/c", body: "a b/c|GET /echo/{path...}", code: 200},
		{name: "HEAD", method: "HEAD", path: "/echo/a", code: 200},
		{name: "range", method: "GET", path: "/raw/value.txt", rangeValue: "bytes=1-3", body: "bcd", code: 206},
		{name: "registered refusal", method: "GET", path: "/owned", body: "feature refusal", allow: "PATCH", code: 405},
		{name: "silent", method: "GET", path: "/silent", code: 200},
		{name: "flush", method: "GET", path: "/flush", body: "stream", code: 200},
		{name: "trailing slash", method: "GET", path: "/tree", location: "/tree/", code: 307},
		{name: "cleaned path", method: "PUT", path: "/unmatched/../missing", location: "/missing", code: 307},
		{name: "unmatched", method: "GET", path: "/missing", body: "404 page not found\n", code: 404},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, server.URL+tc.path, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Range", tc.rangeValue)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if resp.StatusCode != tc.code {
				t.Errorf("code=%d, want%d", resp.StatusCode, tc.code)
			}
			if tc.location == "" && string(body) != tc.body {
				t.Errorf("body=%q, want%q", body, tc.body)
			}
			if got := resp.Header.Get("Allow"); got != tc.allow {
				t.Errorf("Allow=%q,want%q", got, tc.allow)
			}
			if got := resp.Header.Get("Location"); got != tc.location {
				t.Errorf("Location=%q,want%q", got, tc.location)
			}
			if tc.name == "flush" && resp.Header.Get("X-Flushed") != "yes" {
				t.Error("flush header missing")
			}
			if tc.name == "range" && resp.Header.Get("Content-Range") != "bytes 1-3/6" {
				t.Errorf("Content-Range=%q", resp.Header.Get("Content-Range"))
			}
		})
	}
}

// TestLocalizedRouterKeepsRequestCancellation proves that registered handlers
// receive the caller-owned request lifetime rather than a detached request.
func TestLocalizedRouterKeepsRequestCancellation(t *testing.T) {
	t.Parallel()
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	entered := make(chan struct{})
	observed := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /wait", func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
			observed <- r.Context().Err()
		case <-deadline.Done():
			observed <- deadline.Err()
		}
	})
	h := &Handler{}
	server := httptest.NewServer(h.LocalizedRouter(mux))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/wait", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		resp, doErr := server.Client().Do(req)
		if resp != nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		result <- doErr
	}()
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("handler never entered")
	}
	cancel()
	if err = <-result; !errors.Is(err, context.Canceled) {
		t.Errorf("client cancellation=%v", err)
	}
	select {
	case err = <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("server cancellation=%v", err)
		}
	case <-t.Context().Done():
		t.Fatal("server did not observe cancellation")
	}
}
