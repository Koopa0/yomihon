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

// publishedNoteTitle is the example vault's hand-published note.
const publishedNoteTitle = "A published note"

// exampleReader holds what differs between the two interface languages a
// reader of the public example can choose. The note prose keeps its authored
// language; the chrome and the course the reader follows change with the
// cookie.
type exampleReader struct {
	lang      wording.Lang
	course    string
	lifecycle string
	search    string
	// explanation is what the lifecycle note has to say about published.
	explanation []string
	warning     string
	confirm     string
}

var exampleReaders = []exampleReader{
	{
		lang: wording.En, course: "Reading yomihon", lifecycle: "The status lifecycle", search: "Search",
		explanation: []string{
			"The diagram shows transitions the contract allows, including ready → published.",
			"Yomihon never sets a note to published, even when the contract permits that target",
			"the value records a publication that happened somewhere else, and a reading surface cannot attest to one.",
			"This restriction concerns the target, not a note already marked published.",
			"Its status panel offers archived because this example contract allows published → archived",
		},
		warning: "After archived, this offers no way back to the current status.",
		confirm: "Confirm archived",
	},
	{
		lang: wording.ZhHant, course: "讀懂 yomihon", lifecycle: "status 的生命週期", search: "搜尋",
		explanation: []string{
			"上圖呈現契約允許的轉換，包含 ready → published。",
			"yomihon 也不會把筆記設為 published",
			"這個值記錄的是在別處發生的發表，閱讀介面無法為它作證。",
			"這項限制針對的是轉換的目標，不是已經標為 published 的筆記。",
			"它的狀態面板仍提供 archived，因為這份範例契約允許 published → archived",
		},
		warning: "設為 archived 之後，這裡不再有回到目前狀態的路。",
		confirm: "確認設為 archived",
	},
}

// publishedNotePhrases is what the published note says about its own status
// panel and about yomihon never setting published.
var publishedNotePhrases = []string{
	"Its status panel offers archived, which this example contract permits from published.",
	"Yomihon never sets a note to published",
	"that value records a publication outside this vault, which yomihon cannot attest to.",
	"A note already marked published can still offer other transitions allowed by its contract.",
}

// deniesOnwardOperation matches the earlier explanations, which told a reader
// the published note's panel offered nothing while it does offer archived.
var deniesOnwardOperation = regexp.MustCompile(`offers nothing onward|沒有下一步`)

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

// missingFrom names the phrases text lacks, and reports when it still denies
// that the published note offers an onward operation.
func missingFrom(text string, phrases []string) []string {
	var problems []string
	seen := words(text)
	for _, phrase := range phrases {
		if !strings.Contains(seen, phrase) {
			problems = append(problems, "missing "+phrase)
		}
	}
	if deniesOnwardOperation.MatchString(text) {
		problems = append(problems, "the explanation still denies the offered onward operation")
	}
	return problems
}

func requirePhrases(t *testing.T, where, markup string, phrases []string) {
	t.Helper()
	if problems := missingFrom(markup, phrases); len(problems) > 0 {
		t.Errorf("%s: %s; text = %q", where, strings.Join(problems, "; "), words(markup))
	}
}

func requireNoPublishedTarget(t *testing.T, where, page string) {
	t.Helper()
	if strings.Contains(page, `name="to" value="published"`) {
		t.Errorf("%s offered published as a status target", where)
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

// requirePublishedNote holds the hand-published example note to what it says
// and to the one operation its status panel offers.
func requirePublishedNote(t *testing.T, page string, reader *exampleReader) {
	t.Helper()
	where := "A published note in " + string(reader.lang)
	requireArticle(t, where, page, publishedNoteTitle, "en")
	requirePhrases(t, where+" prose", elementInner(t, page, `<div class="y-prose"`), publishedNotePhrases)
	requireNoPublishedTarget(t, where, page)

	forms := statusForms(page)
	if len(forms) == 0 {
		t.Fatalf("%s offers no status form", where)
	}
	for _, form := range forms {
		if got := hiddenInput(form, "from"); got != "published" {
			t.Errorf("%s: status form from = %q, want published", where, got)
		}
		if got := hiddenInput(form, "to"); got != "archived" {
			t.Errorf("%s: status form to = %q, want archived", where, got)
		}
		details := elementInner(t, form, "<details")
		if strings.Contains(form, "<details class=\"y-statusconfirm\" open") {
			t.Errorf("%s: the confirmation starts open", where)
		}
		if got := words(elementInner(t, details, `<p class="y-statusconfirm__note"`)); got != reader.warning {
			t.Errorf("%s: no-return consequence = %q, want %q", where, got, reader.warning)
		}
		if got := words(elementInner(t, details, `<button class="y-xbtn y-statusconfirm__submit"`)); got != reader.confirm {
			t.Errorf("%s: confirmation button = %q, want %q", where, got, reader.confirm)
		}
	}
}

var statusFormStart = regexp.MustCompile(`<form[^>]* action="/status"[^>]*>`)

// statusForms returns the markup of each status form on a page. A note draws
// one per status face, and both must offer the same single operation.
func statusForms(page string) []string {
	var forms []string
	for _, loc := range statusFormStart.FindAllStringIndex(page, -1) {
		form, _, _ := strings.Cut(page[loc[0]:], "</form>")
		forms = append(forms, form)
	}
	return forms
}

func hiddenInput(form, name string) string {
	_, rest, found := strings.Cut(form, `name="`+name+`" value="`)
	if !found {
		return ""
	}
	value, _, _ := strings.Cut(rest, `"`)
	return html.UnescapeString(value)
}

// TestExampleVaultExplainsPublicationOnTheCourseRoute follows the public route
// a first-time reader takes, from Home through the complete path index into
// the course and its lifecycle note, and from there to the published note the
// lifecycle note links. Both interface languages walk it.
func TestExampleVaultExplainsPublicationOnTheCourseRoute(t *testing.T) {
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
			requireNoPublishedTarget(t, where, lifecycle)

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

			prose := elementInner(t, lifecycle, `<div class="y-prose"`)
			publishedHref := onlyLink(t, where+" prose", prose, "", publishedNoteTitle)
			requirePublishedNote(t, readingPageIn(t, site, publishedHref, reader.lang), &reader)
		})
	}
}

// TestExampleVaultSearchReachesThePublishedNote submits the same native GET
// form Home draws and follows the result to the published note.
func TestExampleVaultSearchReachesThePublishedNote(t *testing.T) {
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
				if strings.Contains(body, `class="y-result__title" lang="en">`+publishedNoteTitle+"</") {
					rows++
					row = `<a class="y-result"` + body + "</a>"
				}
			}
			if rows != 1 {
				t.Fatalf("the search page offers %s %d times, want exactly once", publishedNoteTitle, rows)
			}
			requirePhrases(t, "search excerpt in "+string(reader.lang),
				elementInner(t, row, `<span class="y-result__snippet"`), []string{"Its status panel offers archived"})

			href, _ := attr(row, "href")
			path, _, _ := strings.Cut(href, "#")
			requirePublishedNote(t, readingPageIn(t, site, path, reader.lang), &reader)
		})
	}
}

// TestExampleAssertionsRejectTheOldExplanations proves the content checks can
// fail: each passage below is what the example said before it explained that
// a published note still offers archived, and the check has to name it.
func TestExampleAssertionsRejectTheOldExplanations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		text    string
		phrases []string
	}{
		{
			name:    "published note prose",
			text:    "This note carries status: published, written by hand. Its status panel names the status and offers nothing onward: the contract allows ready → published, and yomihon still does not make that move, because the value records something that happened outside this vault.",
			phrases: publishedNotePhrases,
		},
		{
			name:    "English lifecycle prose",
			text:    "The contract allows ready → published and yomihon still does not make that move. The value records a publication that happened somewhere else, and a reading surface cannot attest to one. A published note carries it, written by hand, and its status panel offers nothing onward.",
			phrases: exampleReaders[0].explanation,
		},
		{
			name:    "Traditional Chinese lifecycle prose",
			text:    "契約允許 ready → published，yomihon 仍然不走這一步。這個值記錄的是在別處發生的發表，閱讀介面無法為它作證。這個知識庫裡有一篇手寫上這個值的筆記，它的狀態面板沒有下一步。",
			phrases: exampleReaders[1].explanation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems := missingFrom(tt.text, tt.phrases)
			if len(problems) == 0 {
				t.Fatal("the old explanation passed the content check")
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.HasPrefix(p, "missing ") }) {
				t.Errorf("problems = %v, want a missing-phrase report", problems)
			}
		})
	}
	if got := missingFrom("a note whose panel offers nothing onward", nil); !slices.Contains(got, "the explanation still denies the offered onward operation") {
		t.Errorf("the denial check did not fire on old wording: %v", got)
	}
}
