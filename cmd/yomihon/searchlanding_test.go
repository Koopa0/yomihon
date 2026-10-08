package main

import (
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// TestHeadingHitsRespectDecodedReadingContext preserves ambiguity controls
// while decoded prose and clipped link labels supply useful section context.
func TestHeadingHitsRespectDecodedReadingContext(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, body, query, fragment string }{
		{"an entity reference opens the section", "## Where the inkwell waits\n\nTom &amp; Jerry run.\n", "inkwell", "#:~:text=inkwell%20waits,-Tom%20%26%20Jerry"},
		{"a backslash escape opens the section", "## Where the inkwell waits\n\nThe snake\\_case name.\n", "inkwell", "#:~:text=inkwell%20waits,-The%20snake_case%20name."},
		{"a link opens the section", "## Where the inkwell waits\n\n[Go docs](https://go.dev) explains it.\n", "inkwell", "#:~:text=inkwell%20waits,-Go%20docs"},
		{"a bare address opens the section", "## Where the inkwell waits\n\nhttps://go.dev explains it.\n", "inkwell", "#:~:text=inkwell%20waits,-https%3A%2F%2Fgo.dev"},
		{"the heading holds an entity reference", "## X &amp; place\n\nThe shelf keeps it dry.\n", "place", "#:~:text=place,-The%20shelf%20keeps"},
		{"an entity reference closes the block before", "## First\n\nTom &amp; Jerry run.\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", "#:~:text=%26%20Jerry%20run.-,Where%20the%20inkwell%20waits"},
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

// TestSearchDirectivesNameAdjacentReadingText follows production result links
// into the article, including the localized words external links insert.
func TestSearchDirectivesNameAdjacentReadingText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, query string
		context           bool
	}{
		{"entity", "## Where the inkwell waits\n\nTom &amp; Jerry run.\n", "inkwell", true},
		{"escape", "## Where the inkwell waits\n\nThe snake\\_case name.\n", "inkwell", true},
		{"external", "## Where the inkwell waits\n\n[Go docs](https://go.dev) explains it.\n", "inkwell", true},
		{"explicit-autolink", "## Where the inkwell waits\n\n<https://go.dev> explains it.\n", "inkwell", true},
		{"bare-https", "## Where the inkwell waits\n\nhttps://go.dev explains it.\n", "inkwell", true},
		{"bare-www", "## Where the inkwell waits\n\nwww.example.com explains it.\n", "inkwell", true},
		{"cjk-link", "甲[乙](https://go.dev)丙丁搜尋戊己。\n", "搜尋", true},
		{"nfd-before-link", "cafe\u0301 [乙](https://go.dev)丙搜尋尾。\n", "搜尋", true},
		{"multiple-links", "[甲](https://a.test)乙搜尋丙[丁](https://b.test)戊。\n", "搜尋", true},
		{"body-end-link", "搜尋[尾](https://go.dev)\n", "搜尋", true},
		{"prior-block-link-end", "## Earlier\n\nPrior words [x](https://go.dev)\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", false},
		{"prior-block-link-inside", "## Earlier\n\nPrior [x](https://go.dev) words here.\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", true},
		{"heading-link-end", "## Where the inkwell [waits](https://go.dev)\n\nThe shelf keeps it dry.\n", "inkwell", false},
		{"heading-link-after-prior", "## Earlier\n\nPrior words here.\n\n## Where the inkwell [waits](https://go.dev)\n\n## Next\n", "inkwell", false},
		{"missing-local-link", "## Where the inkwell waits\n\n[notes](Missing.md) explains it.\n", "inkwell", true},
		{"decoded-entity", "Tom &amp; Jerry run.\n", `"Tom & Jerry"`, false},
		{"decoded-escape", "The snake\\_case name.\n", "snake_case", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := "Notes/" + tt.name + ".md"
			site := homeSite(t, map[string]string{path: tt.body})
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					page := readingPageIn(t, site, route+url.Values{"q": {tt.query}}.Encode(), lang)
					rows := resultLink.FindAllStringSubmatch(page, -1)
					if len(rows) != 1 {
						t.Fatalf("caught: display search case=%s lang=%s route=%s rows=%d, want one", tt.name, lang, route, len(rows))
					}
					href := html.UnescapeString(rows[0][1])
					address, err := url.Parse(href)
					if err != nil {
						t.Fatal(err)
					}
					if address.Path != "/notes/"+path {
						t.Fatalf("result target = %q, want %q", address.Path, "/notes/"+path)
					}
					address.Fragment, address.RawFragment = "", ""
					article := articleSearchText(t, readingPageIn(t, site, address.String(), lang))
					_, directive, ok := strings.Cut(href, ":~:text=")
					if !ok {
						t.Fatalf("caught: result %q has no text directive", href)
					}
					assertAdjacentDirective(t, directive, article, tt.name, tt.context, lang, route)
				}
			}
		})
	}
}
func articleSearchText(t *testing.T, page string) string {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	articles := 0
	var walk func(*nethtml.Node, bool)
	walk = func(n *nethtml.Node, article bool) {
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				for class := range strings.FieldsSeq(attr.Val) {
					if class == "y-article" {
						articles++
						article = true
					}
				}
			}
		}
		if article && n.Type == nethtml.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, article)
		}
	}
	walk(doc, false)
	if articles != 1 {
		t.Fatalf("article count = %d, want one", articles)
	}
	return strings.Join(strings.Fields(text.String()), " ")
}

func TestSearchConsumesProseMarkupAndKeepsCodeLiteral(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, query string
		rows              int
		snippet           string
	}{
		{"prose-entity-display", "Tom &amp; Jerry run.\n", `"Tom & Jerry"`, 1, "Tom & Jerry run."},
		{"prose-entity-source", "Tom &amp; Jerry run.\n", "&amp;", 0, ""},
		{"prose-escape-display", "The snake\\_case name.\n", "snake_case", 1, "The snake_case name."},
		{"prose-escape-source", "The snake\\_case name.\n", `snake\_case`, 0, ""},
		{"span-entity-source", "`Tom &amp; Jerry`\n", "&amp;", 1, "Tom &amp; Jerry"},
		{"span-escape-source", "`snake\\_case`\n", `snake\_case`, 1, "snake\\_case"},
		{"fence-entity-source", "```\nTom &amp; Jerry\n```\n", "&amp;", 1, "Tom &amp; Jerry"},
		{"fence-escape-source", "```\nsnake\\_case\n```\n", `snake\_case`, 1, "snake\\_case"},
		{"wikilink-alias-entity-literal", "See [[Other|Tom &amp; Jerry]] here.\n", "&amp;", 1, "See Other Tom &amp; Jerry here."},
		{"wikilink-alias-entity-decoded", "See [[Other|Tom &amp; Jerry]] here.\n", `"Tom & Jerry"`, 0, ""},
		{"wikilink-target-escape-literal", "[[snake\\_case]]\n", `snake\_case`, 1, "snake\\_case"},
		{"wikilink-target-escape-decoded", "[[snake\\_case]]\n", "snake_case", 0, ""},
		{"callout-title-entity-literal", "> [!note] Tom &amp; Jerry\n", "&amp;", 1, "Tom &amp; Jerry"},
		{"callout-title-entity-decoded", "> [!note] Tom &amp; Jerry\n", `"Tom & Jerry"`, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			site := homeSite(t, map[string]string{"Notes/Reading.md": tt.body})
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				for _, route := range []string{"/search?", "/search/results?", "/search/results?facets=1&"} {
					page := readingPageIn(t, site, route+url.Values{"q": {tt.query}}.Encode(), lang)
					if rows := len(resultLink.FindAllStringSubmatch(page, -1)); rows != tt.rows {
						t.Errorf("caught: prose-code corpus case=%s lang=%s route=%s rows=%d, want %d", tt.name, lang, route, rows, tt.rows)
					}
					if tt.snippet != "" {
						doc, err := nethtml.Parse(strings.NewReader(page))
						if err != nil {
							t.Fatal(err)
						}
						var snippets []string
						var visit func(*nethtml.Node)
						visit = func(n *nethtml.Node) {
							for _, attr := range n.Attr {
								if attr.Key == "class" && slices.Contains(strings.Fields(attr.Val), "y-result__snippet") {
									snippets = append(snippets, searchExcerptText(n))
								}
							}
							for child := n.FirstChild; child != nil; child = child.NextSibling {
								visit(child)
							}
						}
						visit(doc)
						if diff := cmp.Diff([]string{tt.snippet}, snippets); diff != "" {
							t.Errorf("caught: literal excerpt case=%s lang=%s route=%s (-want +got):\n%s", tt.name, lang, route, diff)
						}
					}
				}
			}
		})
	}
}

func assertAdjacentDirective(t *testing.T, directive, article, name string, context bool, lang wording.Lang, route string) {
	t.Helper()
	parts := strings.Split(directive, ",")
	prefix, suffix := "", ""
	if value, ok := strings.CutSuffix(parts[0], "-"); ok {
		prefix = value
		parts = parts[1:]
	}
	if len(parts) > 1 && strings.HasPrefix(parts[len(parts)-1], "-") {
		suffix = strings.TrimPrefix(parts[len(parts)-1], "-")
		parts = parts[:len(parts)-1]
	}
	decode := func(s string) string {
		value, err := url.PathUnescape(s)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(value), " ")
	}
	prefix, suffix = decode(prefix), decode(suffix)
	if len(parts) < 1 || len(parts) > 2 || decode(parts[0]) == "" {
		t.Fatalf("invalid directive %q", directive)
	}
	start := decode(parts[0])
	end := ""
	if len(parts) == 2 {
		end = decode(parts[1])
		if end == "" {
			t.Fatalf("caught: empty directive end %q", directive)
		}
	}
	if context && prefix == "" && suffix == "" {
		t.Errorf("caught: nonempty display context case=%s lang=%s route=%s directive=%q", name, lang, route, directive)
	}
	found := false
	for at := 0; at <= len(article); {
		i := strings.Index(article[at:], start)
		if i < 0 {
			break
		}
		i += at
		stop := i + len(start)
		if end != "" {
			j := strings.Index(article[stop:], end)
			if j < 0 {
				break
			}
			stop += j + len(end)
		}
		before := strings.TrimRight(article[:i], " ")
		after := strings.TrimLeft(article[stop:], " ")
		if strings.HasSuffix(before, prefix) && strings.HasPrefix(after, suffix) {
			found = true
			break
		}
		at = i + len(start)
	}
	if !found {
		t.Errorf("caught: adjacent reading context case=%s lang=%s route=%s directive=%q article=%q", name, lang, route, directive, article)
	}
}

func TestDroppedTitleLandsWithDecodedOpening(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, body, query, href string }{
		{"comment-before-title", "%% prefatory comment %%\n# comment-before-title\n\nOpening words here.\n", "comment-before-title", "/notes/Notes/comment-before-title.md#:~:text=comment%2Dbefore%2Dtitle"},
		{"decoded-opening", "## Earlier\n\nPrior words here.\n\n## Where the inkwell waits\n\nTom &amp; Jerry run.\n", "inkwell", "/notes/Notes/decoded-opening.md#:~:text=inkwell%20waits,-Tom%20%26%20Jerry"},
		{"prefix-budget-control", "## Earlier\n\none two three four five.\n\n## Where the inkwell waits\n\n## Next\n", "inkwell", "/notes/Notes/prefix-budget-control.md#:~:text=three%20four%20five.-,Where%20the%20inkwell%20waits"},
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
		for class := range strings.FieldsSeq(classes) {
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

func searchExcerptText(n *nethtml.Node) string {
	var text strings.Builder
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(text.String()), " ")
}
