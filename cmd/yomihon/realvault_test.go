//go:build realvault

package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestEveryVaultFileOpensThroughProductionSite(t *testing.T) {
	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		t.Fatal("YOMIHON_ROOT is required")
	}
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal("newReadingSite failed for configured real vault")
	}
	t.Cleanup(func() {
		if err := site.close(); err != nil {
			t.Fatal("close production site failed")
		}
	})
	srv := httptest.NewUnstartedServer(site)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	t.Cleanup(srv.Close)

	// The production site starts its reconciliation loop at construction. Stop
	// and join that loop while leaving its rooted read capabilities open, so the
	// exact entry set and every request projection below come from one published
	// generation. Production reads still revalidate each captured entry; the
	// deferred check invalidates this run if the selected vault drifts.
	site.cancel()
	site.watchers.Wait()
	view := site.snapshots.Current()
	entries := view.Files()
	if len(entries) == 0 {
		t.Fatal("served snapshot contains no files")
	}
	for i, entry := range entries {
		t.Run(ordinalName(i), func(t *testing.T) {
			rel := entry.Path()
			defer func() {
				if _, err := site.source.ReadPrefix(t.Context(), entry, 0); err != nil {
					t.Error("vault changed during frozen-generation acceptance")
				}
			}()
			pagePath := (&url.URL{Path: "/notes/" + rel}).EscapedPath()
			req, reqErr := http.NewRequestWithContext( //nolint:gosec // srv is this test's loopback httptest server.
				t.Context(), http.MethodGet, srv.URL+pagePath, http.NoBody,
			)
			if reqErr != nil {
				t.Fatal("construct request failed")
			}
			resp, doErr := srv.Client().Do(req) //nolint:gosec // req is restricted to srv above.
			if doErr != nil {
				t.Fatal("request failed")
			}
			bytes, readErr := io.Copy(io.Discard, resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Error("response read failed")
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
			if bytes == 0 {
				t.Error("response body is empty")
			}
			if strings.HasSuffix(rel, ".md") {
				return
			}

			expected, readErr := site.source.ReadPrefix(t.Context(), entry, 1)
			if readErr != nil {
				t.Fatal("read captured resource prefix failed")
			}
			rawPath := (&url.URL{Path: "/raw/" + rel}).EscapedPath()
			head := requestRealVault(t, srv.Client(), http.MethodHead, srv.URL+rawPath, "")
			assertRawHeaders(t, head, entry.Size())
			if body, bodyErr := io.ReadAll(head.Body); bodyErr != nil || len(body) != 0 {
				t.Error("HEAD raw response carried a body")
			}
			if closeErr := head.Body.Close(); closeErr != nil {
				t.Error("close HEAD raw response failed")
			}

			if entry.Size() == 0 {
				empty := requestRealVault(t, srv.Client(), http.MethodGet, srv.URL+rawPath, "")
				body, bodyErr := io.ReadAll(empty.Body)
				closeErr := empty.Body.Close()
				if bodyErr != nil || closeErr != nil || empty.StatusCode != http.StatusOK || len(body) != 0 {
					t.Error("empty raw resource did not return an empty 200 response")
				}
				return
			}

			partial := requestRealVault(t, srv.Client(), http.MethodGet, srv.URL+rawPath, "bytes=0-0")
			body, bodyErr := io.ReadAll(partial.Body)
			partialCloseErr := partial.Body.Close()
			if bodyErr != nil || partialCloseErr != nil {
				t.Fatal("read raw range failed")
			}
			if partial.StatusCode != http.StatusPartialContent || len(body) != 1 || len(expected) != 1 || body[0] != expected[0] {
				t.Error("raw range did not return the captured resource's first byte")
			}
			wantRange := "bytes 0-0/" + strconv.FormatInt(entry.Size(), 10)
			if got := partial.Header.Get("Content-Range"); got != wantRange {
				t.Errorf("raw Content-Range = %q, want %q", got, wantRange)
			}
			assertRawSecurityHeaders(t, partial.Header)
		})
	}
}

func requestRealVault(t *testing.T, client *http.Client, method, urlStr, byteRange string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext( //nolint:gosec // callers pass only this test's loopback server URL.
		t.Context(), method, urlStr, http.NoBody,
	)
	if err != nil {
		t.Fatal("construct raw request failed")
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	response, err := client.Do(req) //nolint:gosec // client and URL come from one httptest server.
	if err != nil {
		t.Fatal("raw request failed")
	}
	return response
}

func assertRawHeaders(t *testing.T, response *http.Response, size int64) {
	t.Helper()
	if response.StatusCode != http.StatusOK {
		t.Errorf("HEAD raw status = %d, want 200", response.StatusCode)
	}
	if response.ContentLength != size {
		t.Errorf("HEAD raw Content-Length = %d, want %d", response.ContentLength, size)
	}
	if got := response.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("HEAD raw Accept-Ranges = %q, want %q", got, "bytes")
	}
	assertRawSecurityHeaders(t, response.Header)
}

func assertRawSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for name, want := range map[string]string{
		"Cache-Control":                "no-store",
		"Cross-Origin-Resource-Policy": "same-origin",
		"X-Content-Type-Options":       "nosniff",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("raw %s = %q, want %q", name, got, want)
		}
	}
	if header.Get("Content-Type") == "" {
		t.Error("raw Content-Type is empty")
	}
	if header.Get("Content-Security-Policy") == "" {
		t.Error("raw Content-Security-Policy is empty")
	}
}

func ordinalName(i int) string {
	return fmt.Sprintf("entry-%06d", i)
}
