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

// TestProductionSearchLandings follows the address each result row offers on
// every public search route. A hit inside a heading names the section's first
// words after it, because the contents list repeats the heading and stands
// above the prose on a narrow page; the list's copy is followed by the next
// entry's name, so only the body's copy answers the whole directive.
func TestProductionSearchLandings(t *testing.T) {
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
	files["Notes/Inkwell.md"] = "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry.\n"
	site := homeSite(t, files)
	link := regexp.MustCompile(`<a class="y-result" href="([^"]+)">`)
	tests := []struct{ name, query, path, fragment string }{
		{"public heading", "等待", worker, "#:~:text=%E7%AD%89%E5%BE%85%E8%80%85%E6%87%89%E8%A9%B2%E6%94%BE%E5%9C%A8%E5%93%AA%E8%A3%A1%EF%BC%9F,-%E8%8B%A5%E6%8A%8A%20wg.Wait%28%29%20%E8%88%87"},
		{"english heading", "inkwell", "Notes/Inkwell.md", "#:~:text=inkwell%20waits,-The%20shelf%20keeps"},
		{"public strike", "取消", pipeline, "#:~:text=%E5%8F%96%E6%B6%88,-%E5%B7%A5%E4%BD%9C"},
		{"heading followed by a heading names no section", "重複", "Notes/Repeated.md", "#:~:text=%E9%87%8D%E8%A4%87,-%E8%80%85"},
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
					want := []string{strings.TrimPrefix(tt.fragment, "#")}
					if diff := cmp.Diff(want, got); diff != "" {
						t.Errorf("GET %s (%s) %s landing (-want +got):\n%s", route, lang, tt.path, diff)
					}
				}
			}
		})
	}
}
