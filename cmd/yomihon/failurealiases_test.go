package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

const failureVaultRoot = "../../examples/vault"

const deadlockExplanation = "這個版本會以 fatal error: all goroutines are asleep - deadlock! 結束；runtime 只在所有 goroutine 都阻塞、又沒有能讓它們繼續的事件時報死結，仍有其他工作能執行的伺服器則可能只讓這個 goroutine 一直等著。"

func TestShippedFailureAliasesHaveOneOwner(t *testing.T) {
	t.Parallel()
	reader, openErr := vault.Open(failureVaultRoot)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	owners := make(map[string][]string)
	aliases := make(map[string][]string)
	for _, entry := range scan.Files() {
		if !strings.EqualFold(path.Ext(entry.Path()), ".md") {
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatal(readErr)
		}
		n := vault.Parse(entry.Path(), data)
		if n.FMDiagnostic != "" {
			t.Fatalf("parse %q: %v", entry.Path(), n.FMDiagnostic)
		}
		aliases[entry.Path()] = n.Aliases()
		for _, alias := range n.Aliases() {
			for _, word := range []string{"deadlock", "死結", "goroutine leak", "洩漏"} {
				if strings.EqualFold(alias, word) {
					owners[word] = append(owners[word], entry.Path())
				}
			}
		}
	}
	t.Log("invoked: whole-vault failure alias ownership")
	want := map[string][]string{
		"deadlock": {"Concepts/go/取消與收尾.md"}, "死結": {"Concepts/go/取消與收尾.md"},
		"goroutine leak": {"Concepts/go/goroutine 的生命週期.md"}, "洩漏": {"Concepts/go/goroutine 的生命週期.md"},
	}
	if diff := cmp.Diff(want, owners); diff != "" {
		t.Errorf("caught: whole-vault alias owners (-want +got):\n%s", diff)
	}
	for _, tc := range []struct {
		rel  string
		want []string
	}{
		{"Concepts/go/取消與收尾.md", []string{"context cancellation", "deadlock", "死結"}},
		{"Concepts/go/goroutine 的生命週期.md", []string{"goroutine lifetime", "goroutine leak", "洩漏"}},
	} {
		t.Run(tc.rel, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, aliases[tc.rel]); diff != "" {
				t.Errorf("caught: retained concept aliases (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShippedFailureWordsReachTheConcepts(t *testing.T) {
	t.Parallel()
	site, err := newReadingSite(t.Context(), failureVaultRoot, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	srv := httptest.NewServer(site)
	t.Cleanup(srv.Close)
	const lesson = "Lessons/go/G02 channel 的交接.md"
	cases := []struct {
		query string
		want  []string
	}{
		{"deadlock", []string{"Concepts/go/取消與收尾.md", lesson}},
		{"死結", []string{"Concepts/go/取消與收尾.md", lesson}},
		{"goroutine leak", []string{"Concepts/go/goroutine 的生命週期.md"}},
		{"洩漏", []string{"Concepts/go/goroutine 的生命週期.md"}},
	}
	for _, lang := range []string{"zh-Hant", "en"} {
		for _, tc := range cases {
			t.Run(lang+"/"+tc.query, func(t *testing.T) {
				t.Parallel()
				t.Log("invoked: actual failure-word search")
				doc := failureWordPage(t, srv.Client(), srv.URL+"/search?q="+url.QueryEscape(tc.query), lang)
				var got []string
				var aliasText []string
				for node := range doc.Descendants() {
					if node.Type != html.ElementNode {
						continue
					}
					if node.Data == "a" && failureWordClass(node, "y-result") {
						for _, attr := range node.Attr {
							if attr.Key == "href" {
								parsed, err := url.Parse(attr.Val)
								if err != nil {
									t.Fatal(err)
								}
								got = append(got, strings.TrimPrefix(parsed.Path, "/notes/"))
							}
						}
					}
					if node.Data == "span" && failureWordClass(node, "y-result__alias") {
						aliasText = append(aliasText, failureWordText(node))
					}
				}
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("caught: failure-word result paths (-want +got):\n%s", diff)
				}
				if len(aliasText) != 1 || !strings.Contains(aliasText[0], tc.query) {
					t.Errorf("caught: visible alias %q, want one containing %q", aliasText, tc.query)
				}
				concept := failureWordPage(t, srv.Client(), srv.URL+pages.VaultHref("/notes/", tc.want[0]), lang)
				found := false
				for node := range concept.Descendants() {
					if node.Type == html.ElementNode && node.Data == "article" {
						found = true
					}
				}
				if !found {
					t.Error("caught: search concept destination has no reading article")
				}
			})
		}
		t.Run(lang+"/runtime explanation", func(t *testing.T) {
			t.Parallel()
			t.Log("invoked: actual lesson runtime explanation")
			doc := failureWordPage(t, srv.Client(), srv.URL+pages.VaultHref("/notes/", lesson), lang)
			found := 0
			for node := range doc.Descendants() {
				if node.Type == html.ElementNode && node.Data == "p" && failureWordText(node) == deadlockExplanation {
					found++
				}
			}
			if found != 1 {
				t.Errorf("caught: rendered deadlock explanation count=%d, want1 complete paragraph", found)
			}
		})
	}
}

func failureWordPage(t *testing.T, client *http.Client, address, lang string) *html.Node {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "yomihon_lang="+lang)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read/close: %v / %v", readErr, closeErr)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GETstatus=%d,type=%q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	rootLang := ""
	for node := range doc.Descendants() {
		if node.Type == html.ElementNode && node.Data == "html" {
			for _, attr := range node.Attr {
				if attr.Key == "lang" {
					rootLang = attr.Val
				}
			}
		}
	}
	if rootLang != lang {
		t.Fatalf("html lang=%q,want%q", rootLang, lang)
	}
	return doc
}

func failureWordClass(node *html.Node, class string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " "+class+" ") {
			return true
		}
	}
	return false
}

func failureWordText(node *html.Node) string {
	var text strings.Builder
	for child := range node.Descendants() {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	}
	return strings.Join(strings.Fields(text.String()), " ")
}
