package main

import (
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// exampleReader holds what differs between the two interface languages a
// reader of the public example can choose. The note prose keeps its authored
// language; the chrome and the course the reader follows change with the
// cookie.
type exampleReader struct {
	lang   wording.Lang
	course string
	drafts string
	search string
	// query is what a reader types to find the drafts note.
	query string
	// explanation is what the drafts note has to say about what marks an
	// unfinished note and who changes it.
	explanation []string
	// edges are the transitions the note's diagram has to draw: both ways
	// between a note that writes the flag and one that does not.
	edges []string
}

var exampleReaders = []exampleReader{
	{
		lang: wording.En, course: "Reading yomihon", drafts: "Drafts", search: "Search", query: "drafts",
		explanation: []string{
			"A note with no draft field is finished.",
			"It sets no field, draft or any other, and offers no button that would: the flag changes only in the author's editor.",
			"A note that writes draft: true is marked as a draft, quietly, on its own page, in the sidebar and on its course.",
		},
		edges: []string{
			"started[draft: true] -->|the author deletes the line| finished",
			"finished -->|the author writes it again| started",
		},
	},
	{
		lang: wording.ZhHant, course: "讀懂 yomihon", drafts: "草稿", search: "搜尋", query: "草稿",
		explanation: []string{
			"沒有 draft 欄位的筆記就是寫完了。",
			"它不設定任何欄位，draft 也一樣，也沒有任何按鈕會這麼做：這個旗標只在作者的編輯器裡改動。",
			"寫了 draft: true 的筆記，會在自己的頁面、側欄與課程上安靜地標為草稿。",
		},
		edges: []string{
			"started[draft: true] -->|作者刪掉這一行| finished",
			"finished -->|作者再寫回去| started",
		},
	},
}

// offersAControl matches the earlier explanations, which told a reader a page
// offered a control that changes a note's status.
var offersAControl = regexp.MustCompile(`status panel|status control|狀態面板|狀態控制`)

// exampleSite composes the production site over the tracked public example.
// Every request in this file is a GET, so the vault is only read.
func exampleSite(t *testing.T) http.Handler {
	t.Helper()
	site, err := newReadingSite(t.Context(), "../../examples/vault", t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite over examples/vault: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	return site
}

var (
	htmlTag    = regexp.MustCompile(`<[^>]*>`)
	whitespace = regexp.MustCompile(`\s+`)
	anchorTag  = regexp.MustCompile(`<a\s[^>]*>`)
)

// words is the text a reader sees: tags dropped, entities decoded, runs of
// white space collapsed.
func words(markup string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(html.UnescapeString(htmlTag.ReplaceAllString(markup, "")), " "))
}

// attr returns the value of one attribute of a start tag.
func attr(tag, name string) (string, bool) {
	_, rest, found := strings.Cut(tag, " "+name+`="`)
	if !found {
		return "", false
	}
	value, _, closed := strings.Cut(rest, `"`)
	return html.UnescapeString(value), closed
}

// elementInner returns the markup inside the first element whose start tag
// begins with open, balancing nested elements of the same name.
func elementInner(t *testing.T, page, open string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, open)
	if !found {
		t.Fatalf("the page carries no %s", open)
	}
	_, rest, _ = strings.Cut(rest, ">")
	name := strings.TrimPrefix(strings.SplitN(open, " ", 2)[0], "<")
	depth, offset := 1, 0
	for depth > 0 {
		next := strings.Index(rest[offset:], "<")
		if next < 0 {
			t.Fatalf("%s is never closed", open)
		}
		offset += next
		switch {
		case strings.HasPrefix(rest[offset:], "</"+name):
			depth--
			if depth == 0 {
				return rest[:offset]
			}
		case strings.HasPrefix(rest[offset:], "<"+name+" ") || strings.HasPrefix(rest[offset:], "<"+name+">"):
			depth++
		}
		offset++
	}
	return ""
}

// elementStart returns the start tag of the first element beginning with open.
func elementStart(t *testing.T, page, open string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, open)
	if !found {
		t.Fatalf("the page carries no %s", open)
	}
	tag, _, _ := strings.Cut(rest, ">")
	return open + tag + ">"
}

// anchors returns the start tag and text of every link in markup.
func anchors(markup string) (tags, texts []string) {
	for _, loc := range anchorTag.FindAllStringIndex(markup, -1) {
		tag := markup[loc[0]:loc[1]]
		inner, _, _ := strings.Cut(markup[loc[1]:], "</a>")
		tags = append(tags, tag)
		texts = append(texts, words(inner))
	}
	return tags, texts
}

// onlyLink returns the href of the one link in markup that carries marker as an
// attribute (or every link when marker is empty) and whose text is want, or the
// only link at all when want is empty.
func onlyLink(t *testing.T, where, markup, marker, want string) string {
	t.Helper()
	tags, texts := anchors(markup)
	var hrefs []string
	for i, tag := range tags {
		if marker != "" && !strings.Contains(tag, " "+marker) && !strings.Contains(tag, `class="`+marker+`"`) {
			continue
		}
		if want != "" && !strings.Contains(texts[i], want) {
			continue
		}
		href, ok := attr(tag, "href")
		if !ok {
			t.Fatalf("%s: a matching link has no href: %s", where, tag)
		}
		hrefs = append(hrefs, href)
	}
	if len(hrefs) != 1 {
		t.Fatalf("%s: %d links match (%q, %q), want exactly one", where, len(hrefs), marker, want)
	}
	return hrefs[0]
}

// missingFrom names the phrases text lacks, and reports when it still speaks
// of a control that changes a status.
func missingFrom(text string, phrases []string) []string {
	var problems []string
	seen := words(text)
	for _, phrase := range phrases {
		if !strings.Contains(seen, phrase) {
			problems = append(problems, "missing "+phrase)
		}
	}
	if offersAControl.MatchString(text) {
		problems = append(problems, "the explanation still speaks of a status control")
	}
	return problems
}

func requirePhrases(t *testing.T, where, markup string, phrases []string) {
	t.Helper()
	if problems := missingFrom(markup, phrases); len(problems) > 0 {
		t.Errorf("%s: %s; text = %q", where, strings.Join(problems, "; "), words(markup))
	}
}

// requireNoStatusControl holds a page to offering no way to change a note: the
// reading room writes nothing into the vault.
func requireNoStatusControl(t *testing.T, where, page string) {
	t.Helper()
	if strings.Contains(page, `action="/status"`) || strings.Contains(page, `name="to"`) {
		t.Errorf("%s offers a status control", where)
	}
}

// requireArticle checks the title and authored language every note page carries.
func requireArticle(t *testing.T, where, page, title, lang string) {
	t.Helper()
	tag := `<article class="y-article"`
	_, rest, found := strings.Cut(page, tag)
	if !found {
		t.Fatalf("%s: the page draws no article", where)
	}
	start, _, _ := strings.Cut(rest, ">")
	if got, _ := attr(tag+start, "lang"); got != lang {
		t.Errorf("%s: article lang = %q, want %q", where, got, lang)
	}
	if got := words(elementInner(t, page, `<h1 class="y-title"`)); got != title {
		t.Errorf("%s: article title = %q, want %q", where, got, title)
	}
}

// TestExampleVaultExplainsDraftsOnTheCourseRoute follows the public route a
// first-time reader takes, from Home through the complete path index into the
// course and its note on drafts. Both interface languages walk it.
func TestExampleVaultExplainsDraftsOnTheCourseRoute(t *testing.T) {
	t.Parallel()
	site := exampleSite(t)
	for _, reader := range exampleReaders {
		t.Run(string(reader.lang), func(t *testing.T) {
			t.Parallel()
			home := readingPageIn(t, site, "/", reader.lang)
			if !strings.Contains(home, `<html lang="`+string(reader.lang)+`"`) {
				t.Errorf("Home is not drawn in %s", reader.lang)
			}
			// Home previews a few paths; the index is the complete public route.
			heading := elementInner(t, deskBlockMarkup(t, home, "courses"), "<h2")
			if got := onlyLink(t, "Home paths heading", heading, "", ""); got != "/courses" {
				t.Fatalf("Home paths heading links to %q, want /courses", got)
			}
			index := readingPageIn(t, site, "/courses", reader.lang)
			courseHref := onlyLink(t, "path index", index, "data-index-row", reader.course)
			if !strings.HasPrefix(courseHref, "/courses/") {
				t.Fatalf("the index opens %q, want a course under /courses/", courseHref)
			}
			course := readingPageIn(t, site, courseHref, reader.lang)
			draftsHref := onlyLink(t, "course", course, "y-lesson", reader.drafts)
			drafts := readingPageIn(t, site, draftsHref, reader.lang)

			where := "drafts note in " + string(reader.lang)
			requireArticle(t, where, drafts, reader.drafts, string(reader.lang))
			requirePhrases(t, where+" prose", elementInner(t, drafts, `<div class="y-prose"`), reader.explanation)
			requireNoStatusControl(t, where, drafts)

			// The diagram is what the prose explains: it has to draw the flag
			// being taken out and written back, both by the author.
			code, ok := attr(elementStart(t, drafts, `<div class="mermaid-diagram"`), "data-mermaid-code")
			if !ok {
				t.Fatalf("%s: the diagram has no authored source", where)
			}
			source, err := url.QueryUnescape(code)
			if err != nil {
				t.Fatalf("%s: diagram source does not decode: %v", where, err)
			}
			for _, edge := range reader.edges {
				if !strings.Contains(source, edge) {
					t.Errorf("%s: the diagram lost %q, which the prose explains; source = %q", where, edge, source)
				}
			}
		})
	}
}

// TestExampleVaultSearchReachesTheDraftsNote submits the native GET form the
// search page draws and follows the result to the note on drafts.
func TestExampleVaultSearchReachesTheDraftsNote(t *testing.T) {
	t.Parallel()
	site := exampleSite(t)
	for _, reader := range exampleReaders {
		t.Run(string(reader.lang), func(t *testing.T) {
			t.Parallel()
			asked := readingPageIn(t, site, "/search", reader.lang)
			form := elementStart(t, asked, `<form class="y-searchpage__form"`)
			if method, _ := attr(form, "method"); method != "get" {
				t.Errorf("search page method = %q, want get", method)
			}
			if action, _ := attr(form, "action"); action != "/search" {
				t.Errorf("search page action = %q, want /search", action)
			}
			formBody := elementInner(t, asked, `<form class="y-searchpage__form"`)
			if !strings.Contains(formBody, `name="q"`) {
				t.Error("search page form has no q field")
			}
			if !strings.Contains(words(formBody), reader.search) {
				t.Errorf("search page button does not say %q; form text = %q", reader.search, words(formBody))
			}
			if home := readingPageIn(t, site, "/", reader.lang); strings.Contains(elementInner(t, home, `<main`), `type="search"`) {
				t.Error("Home draws a search field of its own")
			}

			results := readingPageIn(t, site, "/search?q="+url.QueryEscape(reader.query), reader.lang)
			list := elementInner(t, results, `<ol class="y-results"`)
			var row string
			rows := 0
			for _, chunk := range strings.Split(list, `<a class="y-result"`)[1:] {
				body, _, _ := strings.Cut(chunk, "</a>")
				if strings.Contains(body, `class="y-result__title" lang="`+string(reader.lang)+`">`+reader.drafts+"</") {
					rows++
					row = `<a class="y-result"` + body + "</a>"
				}
			}
			if rows != 1 {
				t.Fatalf("the search page offers %s %d times, want exactly once", reader.drafts, rows)
			}

			href, _ := attr(row, "href")
			path, _, _ := strings.Cut(href, "#")
			where := "drafts note in " + string(reader.lang)
			note := readingPageIn(t, site, path, reader.lang)
			requireArticle(t, where, note, reader.drafts, string(reader.lang))
			requirePhrases(t, where+" prose", elementInner(t, note, `<div class="y-prose"`), reader.explanation)
			requireNoStatusControl(t, where, note)
		})
	}
}

// TestExampleAssertionsRejectTheOldExplanations proves the content checks can
// fail: each passage below is what the example said while a page still
// offered a status control, and the check has to name it.
func TestExampleAssertionsRejectTheOldExplanations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		text    string
		phrases []string
	}{
		{
			name:    "English lifecycle prose",
			text:    "The diagram shows transitions the contract allows, including ready → published. Yomihon never sets a note to published, even when the contract permits that target. A published note carries that value, written by hand. Its status panel offers archived because this example contract allows published → archived.",
			phrases: exampleReaders[0].explanation,
		},
		{
			name:    "Traditional Chinese lifecycle prose",
			text:    "上圖呈現契約允許的轉換，包含 ready → published。即使契約允許這個目標，yomihon 也不會把筆記設為 published。它的狀態面板仍提供 archived，因為這份範例契約允許 published → archived。",
			phrases: exampleReaders[1].explanation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems := missingFrom(tt.text, tt.phrases)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.HasPrefix(p, "missing ") }) {
				t.Errorf("problems = %v, want a missing-phrase report", problems)
			}
			if !slices.Contains(problems, "the explanation still speaks of a status control") {
				t.Errorf("problems = %v, want the status control named", problems)
			}
		})
	}
}
