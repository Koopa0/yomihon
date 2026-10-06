package search

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestSearchShowsTheAnsweringFrontmatterTag(t *testing.T) {
	t.Parallel()
	note := vault.Parse("Notes/Tagged.md", []byte("---\ntitle: Quiet\ntags: [unrelated, Haiku, 'École ＡBC', 'X <b>& fragment']\n---\nOther prose."))
	idx := lexical.NewIndex([]lexical.Document{lexical.DocumentFromNote(note)}, schema.ArtifactPolicy{})
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	for _, lang := range []struct{ code, label string }{{"en", "tag: "}, {"zh-Hant", "標籤： "}} {
		for _, route := range []string{"/search", "/search/results"} {
			for _, tt := range []struct{ query, marked string }{
				{"haiku", "<mark>Haiku</mark>"},
				{"#haiku", "<mark>Haiku</mark>"},
				{"#école abc", "<mark>École</mark> <mark>ＡBC</mark>"},
				{"<b>", "X <mark>&lt;b&gt;</mark>&amp; fragment"},
			} {
				t.Run(lang.code+route+tt.query, func(t *testing.T) {
					t.Parallel()
					body := tagResponse(t, server, route, tt.query, lang.code)
					t.Log("invoked: actual served search response")
					if route == "/search" && !strings.Contains(body, `<html lang="`+lang.code+`"`) {
						t.Fatalf("caught: search page language differs from requested %q", lang.code)
					}
					row := resultRow(t, body, "Notes/Tagged.md")
					if !strings.Contains(row, `class="y-result__tag"`) {
						t.Fatalf("caught: actual tag-only response omits answering tag markup: %q", row)
					}
					named := elementText(t, row, "y-result__tag")
					want := `class="y-result__tag">` + lang.label + tt.marked
					if named != want {
						t.Errorf("caught: answering tag HTML = %q, want %q", named, want)
					}
					if strings.Contains(row, "#:~:text=") || strings.Contains(row, "y-result__snippet") || strings.Contains(named, "<b>") || strings.Contains(named, "unrelated") {
						t.Errorf("caught: tag-only attribution invented body evidence or unsafe/unmatched markup: %q", row)
					}
				})
			}
			t.Run(lang.code+route+"title wins", func(t *testing.T) {
				t.Parallel()
				row := resultRow(t, tagResponse(t, server, route, "quiet", lang.code), "Notes/Tagged.md")
				if strings.Contains(row, "y-result__tag") {
					t.Errorf("caught: title-only result carries tag attribution: %q", row)
				}
			})
		}
	}
}

func tagResponse(t *testing.T, server *httptest.Server, route, query, lang string) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+route+"?q="+url.QueryEscape(query), http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	request.Header.Set("Cookie", "yomihon_lang="+lang)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET %q error = %v", route, err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if response.StatusCode != http.StatusOK || readErr != nil || closeErr != nil {
		t.Fatalf("GET %q = status %d, read error %v, close error %v; want 200 without errors", route, response.StatusCode, readErr, closeErr)
	}
	return string(body)
}
