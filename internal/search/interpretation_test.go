package search

import (
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestEmptySearchExplainsLiteralOperators(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Notes/go/worker.md", Title: "Worker", PlainText: "goroutine channel"},
		{RelPath: "Notes/or.md", Title: "Ordinary word", PlainText: "OR"},
		{RelPath: "Notes/flags.md", Title: "Literal flags", PlainText: "goroutine -flag"},
	}, validArtifactPolicy(t))
	tests := []struct {
		name, query string
		hint        bool
		term        string
	}{
		{name: "minus", query: "goroutine -channel", hint: true, term: "-channel"},
		{name: "disjunction", query: "goroutine OR missing", hint: true, term: "OR"},
		{name: "quoted flag", query: `goroutine "-channel"`},
		{name: "quoted disjunction", query: `goroutine "OR" missing`},
		{name: "ordinary lower case", query: "goroutine or missing"},
		{name: "genuine miss", query: "absent"},
		{name: "literal success", query: "goroutine -flag"},
		{name: "filter value", query: "topic:-flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					body := interpretationPage(t, idx, route+url.Values{"q": {tt.query}}.Encode(), lang)
					got := strings.Contains(body, "data-literal-operator")
					if got != tt.hint {
						t.Errorf("GET %s %q literal hint = %v, want %v", route, tt.query, got, tt.hint)
					}
					if tt.hint {
						sentence := "已當一般文字搜尋"
						if lang == wording.En {
							sentence = "was searched for as ordinary text"
						}
						if !strings.Contains(body, sentence) || !strings.Contains(body, html.EscapeString(tt.term)) {
							t.Errorf("GET %s %q missing localized literal explanation", route, tt.query)
						}
					}
					if tt.name == "disjunction" && strings.Contains(body, `href="/search?q=OR"`) {
						t.Errorf("GET %s offered OR alone", route)
					}
					if tt.name == "literal success" && !strings.Contains(body, "Literal flags") {
						t.Errorf("GET %s reinterpreted a literal flag", route)
					}
				}
			}
		})
	}
}

func TestEmptySearchOffersFullFolderPaths(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Notes/go/one.md", Title: "One", PlainText: "goroutine"},
		{RelPath: "Lessons/go/two.md", Title: "Two", PlainText: "goroutine"},
		{RelPath: "Notes/go/deep/three.md", Title: "Three", PlainText: "other"},
		{RelPath: "Areas/Databases/four.md", Title: "Four", PlainText: "sql"},
	}, validArtifactPolicy(t))
	tests := []struct {
		name, query string
		links       []string
		counts      []string
	}{
		{name: "ambiguous basename", query: "folder:go", links: []string{"folder:Lessons/go", "folder:Notes/go"}, counts: []string{"1", "2"}},
		{name: "preserves words", query: "goroutine folder:go", links: []string{"goroutine folder:Lessons/go", "goroutine folder:Notes/go"}, counts: []string{"1", "1"}},
		{name: "case folded", query: "folder:GO", links: []string{"folder:Lessons/go", "folder:Notes/go"}, counts: []string{"1", "2"}},
		{name: "exact path", query: "folder:Notes/go"},
		{name: "no such basename", query: "folder:missing"},
		{name: "unmatched word", query: "absent folder:go"},
	}
	link := regexp.MustCompile(`<a[^>]*href="(/search\?q=[^"]*)"[^>]*>`)
	paragraph := regexp.MustCompile(`(?s)<p[^>]*data-folder-suggestion[^>]*>(.*?)</p>`)
	count := regexp.MustCompile(`(?:（(\d+) 筆）| \((\d+)\))`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					body := interpretationPage(t, idx, route+url.Values{"q": {tt.query}}.Encode(), lang)
					if got := strings.Contains(body, "data-folder-suggestion"); got != (len(tt.links) > 0) {
						t.Errorf("GET %s %q folder hint = %v, want %v", route, tt.query, got, len(tt.links) > 0)
					}
					var got []string
					for _, m := range link.FindAllStringSubmatch(body, -1) {
						u, err := url.Parse(html.UnescapeString(m[1]))
						if err != nil {
							t.Fatal(err)
						}
						q := u.Query().Get("q")
						if strings.Contains(q, "folder:Lessons/go") || strings.Contains(q, "folder:Notes/go") {
							got = append(got, q)
						}
					}
					var counts []string
					if p := paragraph.FindStringSubmatch(body); len(p) > 0 {
						for _, m := range count.FindAllStringSubmatch(p[1], -1) {
							counts = append(counts, m[1]+m[2])
						}
					}
					want := struct{ Queries, Counts []string }{tt.links, tt.counts}
					observed := struct{ Queries, Counts []string }{got, counts}
					if diff := cmp.Diff(want, observed); diff != "" {
						t.Errorf("GET %s %q folder offers (-want +got):\n%s", route, tt.query, diff)
					}
					if len(tt.links) > 0 {
						sentence := "從資料庫根目錄起算的路徑"
						if lang == wording.En {
							sentence = "path from the vault root"
						}
						if !strings.Contains(body, sentence) {
							t.Errorf("GET %s missing localized folder explanation", route)
						}
						// Every offered URL is exercised through the same public route.
						for _, q := range got {
							landed := interpretationPage(t, idx, "/search/results?"+url.Values{"q": {q}}.Encode(), lang)
							if !strings.Contains(landed, `class="y-result"`) {
								t.Errorf("follow suggestion %q found no result", q)
							}
						}
					}
				}
			}
		})
	}
}

func interpretationPage(t *testing.T, idx *lexical.Index, target string, lang wording.Lang) string {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot { return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}}} }, slog.New(slog.DiscardHandler)).Register(mux)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s status %d, want 200", target, w.Code)
	}
	return w.Body.String()
}
