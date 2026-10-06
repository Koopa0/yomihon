package main

import (
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	nethtml "golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestProductionSearchLandings follows the address each result row offers on
// every public search route. A hit inside a heading names the section's first
// words after it, because the contents list repeats the heading and stands
// above the prose on a narrow page; the list's copy is followed by the next
// entry's name, so only the body's copy answers the whole directive. Where no
// prose follows, the previous block's last words go ahead of the heading
// instead, which the list's copy is preceded by the previous entry's name.
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
	files["Notes/Inkwell.md"] = "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry.\n"
	files["Notes/Before.md"] = "## First\n\nIntro words here.\n\n## Where the quill waits\n\n## Next\n"
	site := homeSite(t, files)
	tests := []struct{ name, query, path, fragment string }{
		{"public heading", "等待", worker, "#:~:text=%E7%AD%89%E5%BE%85%E8%80%85%E6%87%89%E8%A9%B2%E6%94%BE%E5%9C%A8%E5%93%AA%E8%A3%A1%EF%BC%9F,-%E8%8B%A5%E6%8A%8A%20wg.Wait%28%29%20%E8%88%87"},
		{"english heading", "inkwell", "Notes/Inkwell.md", "#:~:text=inkwell%20waits,-The%20shelf%20keeps"},
		{"public strike", "取消", pipeline, "#:~:text=%E5%8F%96%E6%B6%88,-%E5%B7%A5%E4%BD%9C"},
		{"heading followed by a heading names the prose before it", "quill", "Notes/Before.md", "#:~:text=Intro%20words%20here.-,Where%20the%20quill%20waits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLandingOnEveryRoute(t, site, tt.query, tt.path, tt.fragment)
		})
	}
}

// TestGatedHeadingHitsKeepTheDirectiveMainEmits pins the heading hits that
// get no section context, each to the directive the index gave them before
// section context existed, byte for byte. A block whose source holds, among
// others, an entity reference, a backslash escape, a link or a bare address
// is shown by the page in other characters, so its words would name a
// run the page does not have, and the directive would find nothing. The
// same holds for a heading written that way. And a run the contents list's
// copy could answer as well — an opening that also starts the body, a closing
// that also ends the previous entry's name, or one ahead of the first entry,
// which follows the list's own label — tells the two copies apart no
// better than the heading alone. Each want was read off the index before
// section context, by the same requests against the same notes.
func TestGatedHeadingHitsKeepTheDirectiveMainEmits(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, body, query, fragment string }{
		{"an entity reference opens the section", "## Where the inkwell waits\n\nTom &amp; Jerry run.\n", "inkwell", "#:~:text=inkwell"},
		{"a backslash escape opens the section", "## Where the inkwell waits\n\nThe snake\\_case name.\n", "inkwell", "#:~:text=inkwell"},
		{"a link opens the section", "## Where the inkwell waits\n\n[Go docs](https://go.dev) explains it.\n", "inkwell", "#:~:text=inkwell"},
		{"a bare address opens the section", "## Where the inkwell waits\n\nhttps://go.dev explains it.\n", "inkwell", "#:~:text=inkwell"},
		{"the heading holds an entity reference", "## X &amp; place\n\nThe shelf keeps it dry.\n", "place", "#:~:text=place"},
		{"an entity reference closes the block before", "## First\n\nTom &amp; Jerry run.\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", "#:~:text=inkwell"},
		{"the opening also starts the body", "Shared opening words.\n\n## Zebra place\n\nShared opening words again.\n", "zebra", "#:~:text=Zebra"},
		{"the heading is the first entry", "Intro words here.\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", "#:~:text=inkwell"},
		{"the closing also ends the previous entry", "## Alpha\n\nIntro Alpha\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", "#:~:text=inkwell"},
	}
	files := map[string]string{}
	for _, tt := range tests {
		files["Notes/"+tt.name+".md"] = tt.body
	}
	site := homeSite(t, files)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLandingOnEveryRoute(t, site, tt.query, "Notes/"+tt.name+".md", tt.fragment)
		})
	}
}

var resultLink = regexp.MustCompile(`<a class="y-result" href="([^"]+)">`)

func TestDroppedTitleKeepsMainLanding(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, body, query, href string }{
		{"comment-before-title", "%% prefatory comment %%\n# comment-before-title\n\nOpening words here.\n", "comment-before-title", "/notes/Notes/comment-before-title.md#:~:text=comment%2Dbefore%2Dtitle"},
		{"unsafe-next-known-prior", "## Earlier\n\nPrior words here.\n\n## Where the inkwell waits\n\nTom &amp; Jerry run.\n", "inkwell", "/notes/Notes/unsafe-next-known-prior.md#:~:text=Prior%20words%20here.-,Where%20the%20inkwell%20waits"},
		{"safe-plain-control", "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry.\n", "inkwell", "/notes/Notes/safe-plain-control.md#:~:text=inkwell%20waits,-The%20shelf%20keeps"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := "Notes/" + tt.name + ".md"
			site := homeSite(t, map[string]string{path: tt.body})
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				if tt.name == "comment-before-title" {
					page := readingPageIn(t, site, "/notes/"+path, lang)
					assertDroppedTitlePage(t, page, tt.name)
					t.Logf("invoked: dropped title page lang=%s status=200 title=1 bodyH1=0 toc=0", lang)
				}
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					page := readingPageIn(t, site, route+url.Values{"q": {tt.query}}.Encode(), lang)
					matches := resultLink.FindAllStringSubmatch(page, -1)
					if len(matches) != 1 {
						t.Fatalf("GET %s case=%s (%s) has %d result rows, want one", route, tt.name, lang, len(matches))
					}
					got := html.UnescapeString(matches[0][1])
					address, _, _ := strings.Cut(got, "#")
					decoded, err := url.PathUnescape(address)
					if err != nil {
						t.Fatal(err)
					}
					if decoded != "/notes/"+path {
						t.Fatalf("GET %s case=%s (%s) path = %q, want %q", route, tt.name, lang, decoded, "/notes/"+path)
					}
					t.Logf("invoked: dropped title GET case=%s lang=%s route=%s status=200", tt.name, lang, route)
					if diff := cmp.Diff(tt.href, got); diff != "" {
						t.Errorf("caught: dropped-title href case=%s lang=%s route=%s (-want +got):\n%s", tt.name, lang, route, diff)
					}
				}
			}
		})
	}
}

func assertDroppedTitlePage(t *testing.T, page, title string) {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var text func(*nethtml.Node) string
	text = func(n *nethtml.Node) string {
		if n.Type == nethtml.TextNode {
			return n.Data
		}
		var out strings.Builder
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			out.WriteString(text(child))
		}
		return out.String()
	}
	prose, titleMatches, bodyH1, tocLinks := 0, 0, 0, 0
	opening := false
	var walk func(*nethtml.Node, bool)
	walk = func(n *nethtml.Node, inProse bool) {
		classes, href := "", ""
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classes = attr.Val
			}
			if attr.Key == "href" {
				href = attr.Val
			}
		}
		for _, class := range strings.Fields(classes) {
			if class == "y-prose" {
				prose++
				inProse = true
				opening = strings.Contains(text(n), "Opening words here.")
			}
			if class == "y-title" && strings.TrimSpace(text(n)) == title {
				titleMatches++
			}
		}
		if inProse && n.Type == nethtml.ElementNode && n.Data == "h1" {
			bodyH1++
		}
		if n.Type == nethtml.ElementNode && n.Data == "a" && href == "#"+title {
			tocLinks++
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inProse)
		}
	}
	walk(doc, false)
	if prose != 1 || titleMatches != 1 || bodyH1 != 0 || tocLinks != 0 || !opening {
		t.Fatalf("page title/prose premise: prose=%d title=%d bodyH1=%d tocLinks=%d opening=%v", prose, titleMatches, bodyH1, tocLinks, opening)
	}
}

// assertLandingOnEveryRoute checks the one fragment path's row offers for
// query, in both interface languages, on the full search page, the dialog's
// results and the faceted results.
func assertLandingOnEveryRoute(t *testing.T, site http.Handler, query, path, fragment string) {
	t.Helper()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
			body := readingPageIn(t, site, route+url.Values{"q": {query}}.Encode(), lang)
			var got []string
			for _, m := range resultLink.FindAllStringSubmatch(body, -1) {
				href := html.UnescapeString(m[1])
				parts := strings.SplitN(href, "#", 2)
				decoded, err := url.PathUnescape(parts[0])
				if err != nil {
					t.Fatal(err)
				}
				if decoded == "/notes/"+path {
					if len(parts) != 2 {
						t.Errorf("GET %s %s has no landing fragment", route, path)
						continue
					}
					got = append(got, parts[1])
				}
			}
			want := []string{strings.TrimPrefix(fragment, "#")}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("GET %s (%s) %s landing (-want +got):\n%s", route, lang, path, diff)
			}
		}
	}
}
