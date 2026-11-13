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
	lang      wording.Lang
	course    string
	lifecycle string
	search    string
	// explanation is what the lifecycle note has to say about how a status
	// changes and what marks an unfinished note.
	explanation []string
}

var exampleReaders = []exampleReader{
	{
		lang: wording.En, course: "Reading yomihon", lifecycle: "The status lifecycle", search: "Search",
		explanation: []string{
			"The diagram shows transitions the contract allows, including ready → published.",
			"yomihon sets no status, published or any other: it never writes into the vault, and a status changes only in the author's editor.",
			"A note that writes draft: true is marked as a draft on its own page, in the sidebar and on its course.",
		},
	},
	{
		lang: wording.ZhHant, course: "讀懂 yomihon", lifecycle: "status 的生命週期", search: "搜尋",
		explanation: []string{
			"上圖呈現契約允許的轉換，包含 ready → published。",
			"yomihon 不設定任何狀態，published 或其他都一樣：它從不寫入書庫，狀態只在作者的編輯器裡改動。",
			"寫了 draft: true 的筆記，會在自己的頁面、側欄與課程上標為草稿。",
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

// TestExampleVaultExplainsTheLifecycleOnTheCourseRoute follows the public
// route a first-time reader takes, from Home through the complete path index
// into the course and its lifecycle note. Both interface languages walk it.
func TestExampleVaultExplainsTheLifecycleOnTheCourseRoute(t *testing.T) {
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
			heading := elementInner(t, deskBlockMarkup(t, home, "paths"), "<h2")
			if got := onlyLink(t, "Home paths heading", heading, "", ""); got != "/paths" {
				t.Fatalf("Home paths heading links to %q, want /paths", got)
			}
			index := readingPageIn(t, site, "/paths", reader.lang)
			courseHref := onlyLink(t, "path index", index, "data-index-row", reader.course)
			if !strings.HasPrefix(courseHref, "/syllabus/") {
				t.Fatalf("the index opens %q, want a course under /syllabus/", courseHref)
			}
			course := readingPageIn(t, site, courseHref, reader.lang)
			lifecycleHref := onlyLink(t, "course", course, "y-lesson", reader.lifecycle)
			lifecycle := readingPageIn(t, site, lifecycleHref, reader.lang)

			where := "lifecycle note in " + string(reader.lang)
			requireArticle(t, where, lifecycle, reader.lifecycle, string(reader.lang))
			requirePhrases(t, where+" prose", elementInner(t, lifecycle, `<div class="y-prose"`), reader.explanation)
			requireNoStatusControl(t, where, lifecycle)

			// The diagram is the contract the prose explains: it has to draw the
			// target and the onward transition the sentences name.
			code, ok := attr(elementStart(t, lifecycle, `<div class="mermaid-diagram"`), "data-mermaid-code")
			if !ok {
				t.Fatalf("%s: the diagram has no authored source", where)
			}
			source, err := url.QueryUnescape(code)
			if err != nil {
				t.Fatalf("%s: diagram source does not decode: %v", where, err)
			}
			for _, edge := range []string{"ready --> published", "published --> archived"} {
				if !strings.Contains(source, edge) {
					t.Errorf("%s: the diagram lost %q, which the prose explains; source = %q", where, edge, source)
				}
			}
		})
	}
}

// TestExampleVaultSearchReachesTheLifecycleNote submits the same native GET
// form Home draws and follows the result to the lifecycle note.
func TestExampleVaultSearchReachesTheLifecycleNote(t *testing.T) {
	t.Parallel()
	site := exampleSite(t)
	for _, reader := range exampleReaders {
		t.Run(string(reader.lang), func(t *testing.T) {
			t.Parallel()
			home := readingPageIn(t, site, "/", reader.lang)
			form := elementStart(t, home, `<form class="y-homesearch"`)
			if method, _ := attr(form, "method"); method != "get" {
				t.Errorf("Home search method = %q, want get", method)
			}
			if action, _ := attr(form, "action"); action != "/search" {
				t.Errorf("Home search action = %q, want /search", action)
			}
			formBody := elementInner(t, home, `<form class="y-homesearch"`)
			if !strings.Contains(formBody, `name="q"`) {
				t.Error("Home search form has no q field")
			}
			if !strings.Contains(words(formBody), reader.search) {
				t.Errorf("Home search button does not say %q; form text = %q", reader.search, words(formBody))
			}

			results := readingPageIn(t, site, "/search?q=published", reader.lang)
			list := elementInner(t, results, `<ol class="y-results"`)
			var row string
			rows := 0
			for _, chunk := range strings.Split(list, `<a class="y-result"`)[1:] {
				body, _, _ := strings.Cut(chunk, "</a>")
				if strings.Contains(body, `class="y-result__title" lang="`+string(reader.lang)+`">`+reader.lifecycle+"</") {
					rows++
					row = `<a class="y-result"` + body + "</a>"
				}
			}
			if rows != 1 {
				t.Fatalf("the search page offers %s %d times, want exactly once", reader.lifecycle, rows)
			}

			href, _ := attr(row, "href")
			path, _, _ := strings.Cut(href, "#")
			where := "lifecycle note in " + string(reader.lang)
			note := readingPageIn(t, site, path, reader.lang)
			requireArticle(t, where, note, reader.lifecycle, string(reader.lang))
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
