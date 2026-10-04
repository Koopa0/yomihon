package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestDisplayedExcerpts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  string
		query string
		want  excerptView
	}{
		{
			name:  "heading role",
			body:  "## 啟動與交接 {sequence=primary}\n\n後記。\n",
			query: "啟動與交接",
			want:  excerptView{Text: "啟動與交接 後記。", Marks: []excerptMark{{Text: "啟動與交接"}}},
		},
		{
			name:  "role remains findable",
			body:  "## 啟動與交接 {sequence=primary}\n\n後記。\n",
			query: "sequence=primary",
			want:  excerptView{Text: "啟動與交接 後記。"},
		},
		{
			name:  "retracted claim",
			body:  "前言 ~~撤回 加一格 buffer~~ 後記。\n",
			query: "加一格",
			want:  excerptView{Text: "前言 撤回 加一格 buffer 後記。", Deleted: "撤回 加一格 buffer", Marks: []excerptMark{{Text: "加一格", Deleted: true}}},
		},
		{
			name:  "authored highlight",
			body:  "前言 ==重點 lanthanum 說明== 後記。\n",
			query: "lanthanum",
			want:  excerptView{Text: "前言 重點 lanthanum 說明 後記。", Marks: []excerptMark{{Text: "lanthanum"}}},
		},
		{
			name:  "inline code is literal and escaped",
			body:  "`~~codehit~~ ==literal== {sequence=primary} <img src=x>`\n",
			query: "codehit",
			want:  excerptView{Text: "~~codehit~~ ==literal== {sequence=primary} <img src=x>", Marks: []excerptMark{{Text: "codehit"}}},
		},
		{
			name:  "fenced code is literal",
			body:  "```text\n~~fencehit~~ ==literal== {sequence=primary}\n```\n",
			query: "fencehit",
			want:  excerptView{Text: "~~fencehit~~ ==literal== {sequence=primary}", Marks: []excerptMark{{Text: "fencehit"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			const relPath = "Notes/go/Fixture.md"
			source := "---\ntitle: Fixture\ntype: note\nstatus: draft\ndomain: go\nlang: zh-Hant\n---\n\n" + tt.body
			mux := excerptMux(t, relPath, source)
			checkDisplayedExcerpt(t, mux, tt.query, &tt.want)
		})
	}
}

func TestDisplayedExcerptsBriefing(t *testing.T) {
	t.Parallel()

	const source = `<p a="attribute-only > x">取消 &amp; &lt;x&gt;</p><!--hidden--><SCRIPT>scripthide</SCRIPT><style>stylehide</style>`
	tests := []struct {
		name    string
		relPath string
		query   string
		want    excerptView
	}{
		{
			name:    "registered briefing",
			relPath: "System/reports/daily-briefing/fixture.html",
			query:   "取消",
			want:    excerptView{Text: "取消 & <x>", Marks: []excerptMark{{Text: "取消"}}},
		},
		{
			name:    "hidden attribute remains findable",
			relPath: "System/reports/daily-briefing/fixture.html",
			query:   "attribute-only",
			want:    excerptView{Text: "取消 & <x>"},
		},
		{name: "nested source", relPath: "System/reports/daily-briefing/nested/fixture.html", query: "取消", want: excerptView{Text: source, Marks: []excerptMark{{Text: "取消"}}}},
		{name: "ordinary source", relPath: "Notes/go/fixture.html", query: "取消", want: excerptView{Text: source, Marks: []excerptMark{{Text: "取消"}}}},
		{name: "case sensitive extension", relPath: "System/reports/daily-briefing/fixture.HTML", query: "取消", want: excerptView{Text: source, Marks: []excerptMark{{Text: "取消"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := excerptMux(t, tt.relPath, source)
			checkDisplayedExcerpt(t, mux, tt.query, &tt.want)
		})
	}
}

func excerptMux(t *testing.T, relPath, source string) *http.ServeMux {
	t.Helper()
	root := t.TempDir()
	contractBytes, err := os.ReadFile(filepath.Join("..", "..", "examples", "vault", filepath.FromSlash(schema.ContractRelPath)))
	if err != nil {
		t.Fatalf("read example contract: %v", err)
	}
	for name, data := range map[string][]byte{schema.ContractRelPath: contractBytes, relPath: []byte(source)} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if mkdirErr := os.MkdirAll(filepath.Dir(file), 0o750); mkdirErr != nil {
			t.Fatalf("create fixture directory: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(file, data, 0o600); writeErr != nil { // #nosec G703 -- the files belong to this test's temporary vault.
			t.Fatalf("write fixture: %v", writeErr)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close: %v", closeErr)
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatalf("schema.LoadReader: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, err := snapshot.New(t.Context(), reader, log, contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		gen := store.Current()
		return RequestSnapshot{Index: gen.Search(), Shell: nav.Shell{Nav: gen.Navigation(), Governed: contract.Governance().Governed()}}
	}, log).Register(mux)
	return mux
}

func checkDisplayedExcerpt(t *testing.T, mux *http.ServeMux, query string, want *excerptView) {
	t.Helper()
	for _, route := range []string{"/search", "/search/results?facets=0", "/search/results?facets=1"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			sep := "?"
			if strings.Contains(route, "?") {
				sep = "&"
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, route+sep+"q="+url.QueryEscape(query), http.NoBody)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", route, rr.Code)
			}
			got := readExcerpt(t, rr.Body.String())
			if diff := cmp.Diff(*want, got); diff != "" {
				t.Errorf("GET %s excerpt (-want +got):\n%s", route, diff)
			}
		})
	}
}

type excerptView struct {
	Text          string
	OtherElements []string
	Deleted       string
	Marks         []excerptMark
}

type excerptMark struct {
	Text    string
	Deleted bool
}

func readExcerpt(t *testing.T, body string) excerptView {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse response HTML: %v", err)
	}
	var snippets []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " y-result__snippet ") {
				snippets = append(snippets, n)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if len(snippets) != 1 {
		t.Fatalf("served result snippets = %d, want one hit; body = %q", len(snippets), body)
	}
	view := excerptView{Text: strings.TrimSpace(excerptText(snippets[0]))}
	var inspect func(*html.Node, bool)
	inspect = func(n *html.Node, deleted bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "del":
				deleted = true
			case "mark":
				view.Marks = append(view.Marks, excerptMark{Text: excerptText(n), Deleted: deleted})
			default:
				view.OtherElements = append(view.OtherElements, n.Data)
			}
		}
		if n.Type == html.TextNode && deleted {
			view.Deleted += n.Data
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			inspect(child, deleted)
		}
	}
	for child := snippets[0].FirstChild; child != nil; child = child.NextSibling {
		inspect(child, false)
	}
	return view
}

func excerptText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return b.String()
}
