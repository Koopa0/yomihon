package main

import (
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/wording"
)

func TestProductionCJKSearchLandings(t *testing.T) {
	t.Parallel()

	const worker = "Lessons/go/G05 固定數量的 worker.md"
	const pipeline = "Notes/go/提早返回的管線.md"
	files := map[string]string{}
	for _, path := range []string{worker, pipeline} {
		body, err := os.ReadFile(filepath.Join("..", "..", "examples", "vault", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = string(body)
	}
	files["Notes/Repeated.md"] = "## 重複者。\n\n## 重複者。\n"
	files["Notes/Composed.md"] = "## cafe\u0301等待者。\n"
	files["Notes/Host.md"] = "![[Embedded]]\n\n## 同名等待者。\n"
	files["Notes/Embedded.md"] = "## 同名等待者。\n"
	site := homeSite(t, files)
	link := regexp.MustCompile(`<a class="y-result" href="([^"]+)">`)
	tests := []struct{ name, query, path, fragment string }{
		{"public heading", "等待", worker, "#等待者應該放在哪裡"},
		{"public strike", "取消", pipeline, "#:~:text=%E5%8F%96%E6%B6%88,-%E5%B7%A5%E4%BD%9C"},
		{"repeated heading does not invent an anchor", "重複", "Notes/Repeated.md", "#:~:text=%E9%87%8D%E8%A4%87,-%E8%80%85"},
		{"normalized heading", "café等待", "Notes/Composed.md", "#café等待者"},
		{"actual rendered id after an embed", "同名等待", "Notes/Host.md", "#同名等待者-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					body := readingPageIn(t, site, route+url.Values{"q": {tt.query}}.Encode(), lang)
					var got []string
					for _, m := range link.FindAllStringSubmatch(body, -1) {
						href := html.UnescapeString(m[1])
						parts := strings.SplitN(href, "#", 2)
						decoded, err := url.PathUnescape(parts[0])
						if err != nil {
							t.Fatal(err)
						}
						if decoded == "/notes/"+tt.path {
							if len(parts) != 2 {
								t.Errorf("GET %s %s has no landing fragment", route, tt.path)
								continue
							}
							got = append(got, parts[1])
						}
					}
					fragment := strings.TrimPrefix(tt.fragment, "#")
					want := []string{fragment}
					if strings.HasPrefix(tt.fragment, "#") && !strings.HasPrefix(tt.fragment, "#:~:") {
						want = []string{url.PathEscape(fragment)}
					}
					if diff := cmp.Diff(want, got); diff != "" {
						t.Errorf("GET %s (%s) %s landing (-want +got):\n%s", route, lang, tt.path, diff)
					}
				}
			}
		})
	}
}
