package note_test

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestOversizeMarkdownExplainsItsLimit follows the information page's own
// health link, including a note beyond the first page and URL-sensitive names.
func TestOversizeMarkdownExplainsItsLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "Notes"))
	data := bytes.Repeat([]byte("x"), 2_102_802)
	for i := range 26 {
		write(t, filepath.Join(root, fmt.Sprintf("Notes/A%02d.md", i)), data)
	}
	paths := []string{"Notes/Z huge.md", "Notes/Z 字 # % &.md"}
	for _, rel := range paths {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), data)
	}
	write(t, filepath.Join(root, "blob"), []byte{0, 1, 2})
	write(t, filepath.Join(root, "big.txt"), data)
	write(t, filepath.Join(root, "big.MD"), data)
	write(t, filepath.Join(root, "Notes", "at-limit.md"), bytes.Repeat([]byte("x"), 1<<20))
	srv := newServer(t, root)
	for _, tt := range []struct{ lang, want, generic string }{
		{lang: "zh-Hant", want: "這篇筆記的大小是 2.0 MiB（2,102,802 位元組），超過 1 MiB 的閱讀上限；原始檔仍可下載。", generic: "這裡沒有這種檔案的閱讀器"},
		{lang: "en", want: "This note is 2.0 MiB (2,102,802 bytes), over the 1 MiB reading limit; the original file can still be downloaded.", generic: "There is no reader here for this kind of file"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()
			client := srv.Client()
			for _, rel := range paths {
				t.Run(rel, func(t *testing.T) {
					t.Parallel()
					body := sizeLimitPage(t, client, srv.URL+"/notes/"+url.PathEscape(rel), tt.lang)
					if !strings.Contains(html.UnescapeString(body), tt.want) || strings.Contains(body, tt.generic) {
						t.Errorf("size-limit explanation missing or generic: want %q", tt.want)
					}
					var healthLink *url.URL
					for _, match := range regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(body, -1) {
						link, err := url.Parse(html.UnescapeString(match[1]))
						if err != nil {
							t.Fatalf("parse page link: %v", err)
						}
						if link.Path == "/health" && link.Fragment != "" {
							healthLink = link
						}
					}
					if healthLink == nil {
						t.Fatal("size-limit health row link missing")
					}
					if healthLink.RawQuery != "page=all" {
						t.Fatalf("health link query = %q, want whole report", healthLink.RawQuery)
					}
					page := sizeLimitPage(t, client, srv.URL+healthLink.String(), tt.lang)
					row := regexp.MustCompile(`(?s)<tr\b[^>]*\bid="` + regexp.QuoteMeta(healthLink.Fragment) + `"[^>]*>(.*?)</tr>`).FindStringSubmatch(page)
					if len(row) != 2 || !strings.Contains(html.UnescapeString(row[1]), rel) || !strings.Contains(row[1], "over the source bound") {
						t.Errorf("size-limit health link does not reach its own row: %s", healthLink)
					}
					first := sizeLimitPage(t, client, srv.URL+"/health", tt.lang)
					if strings.Contains(first, `id="`+healthLink.Fragment+`"`) {
						t.Error("pagination control did not place the target beyond the first page")
					}
					code, _, raw := fetch(t, client, srv.URL+"/raw/"+url.PathEscape(rel))
					if code != http.StatusOK || raw != string(data) {
						t.Error("raw note bytes changed")
					}
				})
			}
			for _, rel := range []string{"blob", "big.txt", "big.MD"} {
				body := sizeLimitPage(t, client, srv.URL+"/notes/"+rel, tt.lang)
				if !strings.Contains(body, tt.generic) || strings.Contains(body, tt.want) {
					t.Errorf("generic file %q explanation changed", rel)
				}
			}
			body := sizeLimitPage(t, client, srv.URL+"/notes/Notes/at-limit.md", tt.lang)
			if strings.Contains(body, "y-fileinfo__lead") || !strings.Contains(body, "y-prose") {
				t.Error("note exactly at the limit is not readable")
			}
		})
	}
}

// TestExcludedOversizeMarkdownHasNoHealthRow keeps a document the contract
// excludes from diagnostics from linking to a finding the report does not have.
func TestExcludedOversizeMarkdownHasNoHealthRow(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, skippedDocumentVault(t), loadContract(t))
	// The contract leaves this file out of the library, so below the bound it
	// opens as a document; over the bound it must not be called a note.
	for _, tt := range []struct {
		lang       wording.Lang
		want, note string
	}{
		{lang: wording.ZhHant, want: "這份文件的大小是 1.0 MiB（1,048,577 位元組），超過 1 MiB 的閱讀上限；原始檔仍可下載。", note: "這篇筆記"},
		{lang: wording.En, want: "This document is 1.0 MiB (1,048,577 bytes), over the 1 MiB reading limit; the original file can still be downloaded.", note: "This note is"},
	} {
		body := sizeLimitPage(t, srv.Client(), srv.URL+"/notes/Elsewhere/README.md", string(tt.lang))
		if strings.Contains(body, "health-source-bound-") {
			t.Error("excluded Markdown links to an absent health finding")
		}
		if text := html.UnescapeString(body); !strings.Contains(text, tt.want) || strings.Contains(text, tt.note) {
			t.Errorf("caught: excluded oversize Markdown in %s is not named a document: want %q and no %q", tt.lang, tt.want, tt.note)
		}
	}
}

func sizeLimitPage(t *testing.T, client *http.Client, address, lang string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	if err != nil {
		t.Fatalf("size-limit request: %v", err)
	}
	req.Header.Set("Cookie", "yomihon_lang="+lang)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("size-limit GET: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close size-limit response: %v", closeErr)
		}
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read size-limit response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("size-limit GET = %d, want 200", resp.StatusCode)
	}
	return string(data)
}
