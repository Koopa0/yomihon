package note_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

func TestShippedCancellationLessonShowsTheProgramItsExcerptExplains(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, readingLibraryRoot, readingLibraryContract(t))

	for _, lang := range []string{"zh-Hant", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+pages.VaultHref("/notes/", "Lessons/go/G04 取消不再需要的工作.md"), http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Cookie", "yomihon_lang="+lang)
			response, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("read/close G04 response: %v / %v", readErr, closeErr)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("G04 status = %d, want 200: %s", response.StatusCode, body)
			}
			doc, err := html.Parse(strings.NewReader(string(body)))
			if err != nil {
				t.Fatal(err)
			}
			var embeds []*html.Node
			var rootLang string
			var find func(*html.Node)
			find = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "html" {
					for _, attr := range node.Attr {
						if attr.Key == "lang" {
							rootLang = attr.Val
						}
					}
				}
				if node.Type == html.ElementNode && node.Data == "template" {
					return
				}
				if node.Type == html.ElementNode && node.Data == "div" {
					for _, attr := range node.Attr {
						if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " embed ") {
							embeds = append(embeds, node)
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					find(child)
				}
			}
			find(doc)
			if rootLang != lang {
				t.Errorf("caught: G04 interface language = %q, want %q", rootLang, lang)
			}
			if len(embeds) != 1 {
				t.Fatalf("caught: G04 visible excerpts = %d, want 1", len(embeds))
			}
			var codes []string
			var sourceHref string
			var read func(*html.Node)
			read = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "pre" {
					codes = append(codes, strings.TrimSpace(shippedExcerptText(node)))
				}
				if node.Type == html.ElementNode && node.Data == "a" && node.Parent != nil && node.Parent.Data == "p" {
					for _, attr := range node.Parent.Attr {
						if attr.Key == "class" && attr.Val == "embed__source" {
							for _, link := range node.Attr {
								if link.Key == "href" {
									sourceHref = link.Val
								}
							}
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					read(child)
				}
			}
			read(embeds[0])
			t.Log("invoked: shipped G04 section excerpt")
			if len(codes) != 2 {
				t.Fatalf("caught: G04 embedded code blocks = %d, want program and output", len(codes))
			}
			for _, want := range []string{"done := make(chan struct{})", "defer close(done)"} {
				if !strings.Contains(codes[0], want) {
					t.Errorf("caught: G04 embedded program lost %q", want)
				}
			}
			if diff := cmp.Diff("1\nstopped", codes[1]); diff != "" {
				t.Errorf("caught: G04 embedded output (-want +got):\n%s", diff)
			}
			decoded, err := url.PathUnescape(sourceHref)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != "/notes/Notes/go/可以取消的管線.md" {
				t.Errorf("caught: excerpt source = %q, want pipeline note", decoded)
			}
			text := shippedExcerptText(embeds[0])
			for _, want := range []string{
				"ctx.Done() 通知 producer 取消；done 關閉則表示 producer 已完成收尾。cancel() 本身不等待工作結束。",
				"消費者不再接收時，send 無法繼續。取消訊號讓 select 走到 return，接著執行 defer：先關閉 out，再關閉 done。",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("caught: excerpt lost explanation %q", want)
				}
			}
			for _, outside := range []string{"producer 是 out 的唯一發送者", "取消沒有優先權"} {
				if strings.Contains(text, outside) {
					t.Errorf("caught: section excerpt swallowed later section %q", outside)
				}
			}
		})
	}
}

func shippedExcerptText(node *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(at *html.Node) {
		if at.Type == html.TextNode {
			text.WriteString(at.Data)
		}
		for child := at.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return text.String()
}
