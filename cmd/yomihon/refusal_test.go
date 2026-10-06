package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoutingRefusalsKeepTheReadingShell checks requests refused before any
// write handler can run, including both browser cross-origin signals.
func TestRoutingRefusalsKeepTheReadingShell(t *testing.T) {
	root := t.TempDir()
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	before, err := os.ReadFile(filepath.Join(root, "Concepts", "alpha.md")) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, method, target, signal, allow string
		code                                int
	}{
		{"note method", "POST", "/notes/Concepts/alpha.md", "", "GET, HEAD", 405},
		{"raw method", "DELETE", "/raw/Concepts/alpha.md", "", "GET, HEAD", 405},
		{"options method", "OPTIONS", "/notes/Concepts/alpha.md", "", "GET, HEAD", 405},
		{"unknown path method", "POST", "/unknown", "", "GET, HEAD", 405},
		{"write method", "PUT", "/status", "", "GET, HEAD, POST", 405},
		{"fetch status", "POST", "/status", "fetch", "", 403},
		{"origin status", "POST", "/status", "origin", "", 403},
		{"same site status", "POST", "/status", "same-site", "", 403},
		{"malformed origin", "POST", "/status", "malformed", "", 403},
		{"fetch language", "POST", "/lang", "fetch", "", 403},
		{"fetch marks", "POST", "/marks", "fetch", "", 403},
	}
	for _, language := range []string{"zh-Hant", "en"} {
		for _, tc := range cases {
			t.Run(language+"/"+tc.name, func(t *testing.T) {
				req := siteRequest(t, tc.method, tc.target, strings.NewReader("path=Concepts%2Falpha.md&to=ready"))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: language, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
				if tc.signal == "fetch" {
					req.Header.Set("Sec-Fetch-Site", "cross-site")
				}
				if tc.signal == "origin" {
					req.Header.Set("Origin", "https://outside.invalid")
				}
				if tc.signal == "same-site" {
					req.Header.Set("Sec-Fetch-Site", "same-site")
				}
				if tc.signal == "malformed" {
					req.Header.Set("Origin", ":")
				}
				rec := httptest.NewRecorder()
				site.ServeHTTP(rec, req)
				response := rec.Result()
				data, readErr := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				if response.StatusCode != tc.code {
					t.Errorf("%s %s code = %d, want %d", tc.method, tc.target, response.StatusCode, tc.code)
				}
				if got := response.Header.Get("Allow"); got != tc.allow {
					t.Errorf("Allow = %q, want %q", got, tc.allow)
				}
				if got := response.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
					t.Errorf("Content-Type = %q, want HTML", got)
				}
				body := string(data)
				title, lede := "這個地址不接受這種請求", "請用下方連結重新開啟頁面。"
				if tc.code == 403 {
					title, lede = "這個請求未通過來源檢查", "請從閱讀頁面重新操作。"
				}
				if language == "en" {
					title, lede = "This address does not accept this request", "Open the page again using a link below."
					if tc.code == 403 {
						title, lede = "This request did not pass the origin check", "Try again from the reading page."
					}
				}
				for _, fragment := range []string{`<html lang="` + language + `"`, `id="main-content"`, `class="y-shell2"`, `id="request-refused-title"`, title, lede, `href="/search"`, `href="/"`} {
					if !strings.Contains(body, fragment) {
						t.Errorf("refused response lacks %q", fragment)
					}
				}
				for _, bare := range []string{"Method Not Allowed", "cross-origin request detected"} {
					if strings.Contains(body, bare) {
						t.Errorf("bare router response remains: %q", bare)
					}
				}
			})
		}
	}
	after, err := os.ReadFile(filepath.Join(root, "Concepts", "alpha.md")) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Error("refused requests changed the note")
	}
}

// TestRefusalReplaysKeepTheWireReply observes refusals over a real connection
// to an isolated fixture site. Replaying either request cannot enter a write
// face, and the committed header is read by the HTTP client rather than from
// the handler's mutable header map.
func TestRefusalReplaysKeepTheWireReply(t *testing.T) {
	root := t.TempDir()
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	server := httptest.NewServer(site)
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		method, path, lang, allow string
		cross                     bool
		code                      int
	}{
		{method: "POST", path: "/notes/Concepts/alpha.md", lang: "zh-Hant", allow: "GET, HEAD", code: 405},
		{method: "POST", path: "/notes/Concepts/alpha.md", lang: "en", allow: "GET, HEAD", code: 405},
		{method: "POST", path: "/status", lang: "zh-Hant", cross: true, code: 403},
		{method: "POST", path: "/status", lang: "en", cross: true, code: 403},
	} {
		t.Run(tc.lang+tc.path, func(t *testing.T) {
			for range 3 {
				req, requestErr := http.NewRequestWithContext(t.Context(), tc.method, server.URL+tc.path, http.NoBody)
				if requestErr != nil {
					t.Fatal(requestErr)
				}
				req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: tc.lang, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
				if tc.cross {
					req.Header.Set("Sec-Fetch-Site", "cross-site")
				}
				resp, responseErr := server.Client().Do(req)
				if responseErr != nil {
					t.Fatal(responseErr)
				}
				data, readErr := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				if resp.StatusCode != tc.code || resp.Header.Get("Allow") != tc.allow || resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
					t.Errorf("wire reply=%d Allow=%q Content-Type=%q, want%d %q HTML", resp.StatusCode, resp.Header.Get("Allow"), resp.Header.Get("Content-Type"), tc.code, tc.allow)
				}
				if !strings.Contains(string(data), `<html lang="`+tc.lang+`"`) || !strings.Contains(string(data), `id="request-refused-title"`) {
					t.Error("wire HTML lacks localized recovery shell")
				}
			}
		})
	}
}

// TestRefusalRenderingKeepsCancellation checks the refusal component itself:
// a cancelled reader receives no rendered document, while the already chosen
// status and method header retain their meaning.
func TestRefusalRenderingKeepsCancellation(t *testing.T) {
	root := t.TempDir()
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	for _, tc := range []struct {
		path, allow string
		cross       bool
		code        int
	}{
		{path: "/notes/Concepts/alpha.md", allow: "GET, HEAD", code: 405},
		{path: "/status", cross: true, code: 403},
	} {
		t.Run(tc.path, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			req := siteRequest(t, http.MethodPost, tc.path, http.NoBody).WithContext(ctx)
			if tc.cross {
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			rec := httptest.NewRecorder()
			site.ServeHTTP(rec, req)
			response := rec.Result()
			data, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if response.StatusCode != tc.code || response.Header.Get("Allow") != tc.allow {
				t.Errorf("cancelled refusal=%d Allow=%q, want%d %q", response.StatusCode, response.Header.Get("Allow"), tc.code, tc.allow)
			}
			if len(data) != 0 {
				t.Error("cancelled refusal rendered a document")
			}
		})
	}
}
