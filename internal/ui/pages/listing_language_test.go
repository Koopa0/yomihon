package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestSearchListingLanguageComesOnlyFromAuthority holds what a search result row
// is allowed to say about the language a note is written in. A declared tag is
// stamped on the title and snippet; a note that declared nothing leaves both
// without a language of their own, so the page language stands for them.
func TestSearchListingLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want []string
	}{
		{
			name: "undeclared note states no language on title or snippet",
			want: []string{
				`<span class="y-result__title">Alpha</span>`,
				`<span class="y-result__snippet">needle in prose</span>`,
			},
		},
		{
			name: "Japanese note stamps title and snippet",
			lang: "ja",
			want: []string{
				`<span class="y-result__title" lang="ja">L01 わたしは学生です</span>`,
				`<span class="y-result__snippet" lang="ja">`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := SearchView{
				Query: "needle",
				Results: []SearchResult{{
					RelPath:  "Notes/alpha.md",
					Title:    "Alpha",
					Language: tt.lang,
					Snippet:  "needle in prose",
				}},
				Total: 1,
			}
			if tt.lang == "ja" {
				view.Results[0].RelPath = "Writing/lessons/japanese/L01.md"
				view.Results[0].Title = "L01 わたしは学生です"
				view.Results[0].Snippet = "わたしは学生です"
			}
			var buf bytes.Buffer
			if err := SearchResults(view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("SearchResults() missing %q in %q", want, html)
				}
			}
			if tt.lang == "" && strings.Contains(html, `y-result__title" lang=`) {
				t.Errorf("undeclared search row stamped a title language: %q", html)
			}
			if tt.lang == "" && strings.Contains(html, `y-result__snippet" lang=`) {
				t.Errorf("undeclared search row stamped a snippet language: %q", html)
			}
		})
	}
}

// TestFolderListingLanguageComesOnlyFromAuthority holds the folder shelf row
// title the same way: declared language on the span, absent otherwise.
func TestFolderListingLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "undeclared note states no language", want: `<span class="y-row__title">Alpha</span>`},
		{name: "Japanese note stamps title", lang: "ja", want: `<span class="y-row__title" lang="ja">L01 わたしは学生です</span>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := Row{Text: "Alpha", Href: "/notes/Notes/alpha.md", Language: tt.lang}
			if tt.lang == "ja" {
				row = Row{Text: "L01 わたしは学生です", Href: "/notes/Writing/lessons/japanese/L01.md", Language: tt.lang}
			}
			var buf bytes.Buffer
			if err := shelfPageRow(row).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if html := buf.String(); !strings.Contains(html, tt.want) {
				t.Errorf("shelf row missing %q in %q", tt.want, html)
			}
		})
	}
}

// TestSearchUnlocatedNoteUsesNoArticleLanguage holds the interface sentence a
// block-crossing row carries when the page cannot locate the hit: it is chosen
// by the chrome language and must not inherit the note's declared tag.
func TestSearchUnlocatedNoteUsesNoArticleLanguage(t *testing.T) {
	t.Parallel()
	view := SearchView{
		Query: "needle",
		Results: []SearchResult{{
			RelPath:       "Notes/alpha.md",
			Title:         "Alpha",
			Language:      "ja",
			BlockCrossing: true,
			Landing:       "",
			LandingEnd:    "",
		}},
		Total: 1,
	}
	var buf bytes.Buffer
	if err := SearchResults(view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, wording.SearchHitUnlocated.In(wording.ZhHant)) {
		t.Fatalf("SearchResults() missing unlocated note sentence in %q", html)
	}
	if strings.Contains(html, `y-result__snippet" lang=`) {
		t.Errorf("unlocated search row stamped snippet language: %q", html)
	}
}

// TestRailListingLanguageComesOnlyFromAuthority holds the left-rail folder shelf
// and map rows the same way: declared language on the row span, absent otherwise.
func TestRailListingLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "undeclared note states no language", want: `<a class="ui-navitem" href="/notes/Notes/alpha.md"><span>Alpha</span></a>`},
		{name: "Japanese note stamps title", lang: "ja", want: `<a class="ui-navitem" href="/notes/Writing/lessons/japanese/L01.md"><span lang="ja">L01</span></a>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := Row{Text: "Alpha", Href: "/notes/Notes/alpha.md"}
			if tt.lang == "ja" {
				row = Row{Text: "L01", Href: "/notes/Writing/lessons/japanese/L01.md", Language: tt.lang}
			}
			shelf := Shelf{Title: "here", Href: "/folders/here", Rows: []Row{row}}
			var buf bytes.Buffer
			if err := ShelfRail(shelf, 24, "here", "另外 %d 篇 →").Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if html := buf.String(); !strings.Contains(html, tt.want) {
				t.Errorf("ShelfRail() missing %q in %q", tt.want, html)
			}
		})
	}

	entry := nav.MapEntry{Kind: nav.EntryResolved, RelPath: "Writing/lessons/japanese/L01.md", Text: "L01", Language: "ja"}
	var buf bytes.Buffer
	if err := entryLink(Sidebar{}, layouts.Chrome{}, entry).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render map row: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<span lang="ja">L01</span>`) {
		t.Errorf("entryLink missing Japanese language stamp in %q", html)
	}
}

// TestBookRailListingLanguageComesOnlyFromAuthority holds the reading page's
// book rail the way every other listing is held: a lesson's title carries the
// language that lesson declared and none where it declared nothing, while the
// rail's own words stay the interface's. The rail is read under English chrome,
// so a title that had merely inherited the document would be announced as
// English — which is what a reader of a Japanese course would hear.
func TestBookRailListingLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	chrome := layouts.Chrome{Lang: wording.En}

	rows := []struct {
		name  string
		entry PathEntryView
		want  string
	}{
		{
			name:  "a lesson that declared its language",
			entry: PathEntryView{Kind: nav.EntryResolved, Text: "日本語の課", Href: "/notes/Lessons/Language lesson.md", Language: "ja"},
			want:  `<span lang="ja">日本語の課</span>`,
		},
		{
			name:  "a lesson that declared none",
			entry: PathEntryView{Kind: nav.EntryResolved, Text: "Alpha", Href: "/notes/Notes/alpha.md"},
			want:  `<span>Alpha</span>`,
		},
		{
			name:  "a row that reached nothing still says the words it lists",
			entry: PathEntryView{Kind: nav.EntryAmbiguous, Text: "日本語の課", Language: "ja"},
			want:  `<span lang="ja">日本語の課</span>`,
		},
	}
	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := bookRailEntry(ReadingRail{}, chrome, tt.entry).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render book rail row: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, tt.want) {
				t.Errorf("book rail row missing %q in %q", tt.want, html)
			}
			// The language belongs to the title alone. On the row itself it
			// would also cover the status chip and the resolution word beside
			// it, which are the rail's words and not the lesson's.
			opening, _, _ := strings.Cut(html, ">")
			if strings.Contains(opening, "lang=") {
				t.Errorf("the whole book rail row took the lesson's language: %q", opening)
			}
		})
	}

	// The study path's own page lists the same rows from the same view, so the
	// language it now carries is stamped there too rather than only where the
	// defect was reported.
	t.Run("the study path's own row", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entry := PathEntryView{Kind: nav.EntryResolved, Text: "日本語の課", Href: "/notes/Lessons/Language lesson.md", Language: "ja"}
		if err := entryRow(entry, wording.En).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render study path row: %v", err)
		}
		want := `<span class="y-lesson__title" lang="ja">日本語の課</span>`
		if html := buf.String(); !strings.Contains(html, want) {
			t.Errorf("study path row missing %q in %q", want, html)
		}
	})

	t.Run("the steps either side", func(t *testing.T) {
		t.Parallel()
		rail := ReadingRail{
			book: &nav.Path{Title: "日本語の道", RelPath: "Maps/Language path.md"},
			neighbors: nav.Neighbors{
				PathTitle:   "日本語の道",
				PathRelPath: "Maps/Language path.md",
				Prev:        nav.NoteRef{Name: "L00 はじめに", RelPath: "Lessons/L00.md", Language: "ja"},
				Next:        nav.NoteRef{Name: "Alpha", RelPath: "Lessons/alpha.md"},
			},
		}
		var buf bytes.Buffer
		if err := bookRail(rail, chrome).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render book rail: %v", err)
		}
		html := buf.String()
		// Each expectation puts the interface's word immediately before the
		// span, which is the boundary itself: the words naming the step stay
		// English and the title they hand over to stays Japanese.
		for _, want := range []string{
			`Previous lesson: <span lang="ja">L00 はじめに</span>`,
			`Next lesson: <span>Alpha</span>`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("the book rail's steps are missing %q in %q", want, html)
			}
		}
	})
}

// TestHealthListingLanguageComesOnlyFromAuthority holds health link names the
// same way as other listings: declared language on the span, absent otherwise.
func TestHealthListingLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	ref := nav.NoteRef{Name: "L01", RelPath: "Writing/lessons/japanese/L01.md", Language: "ja"}
	view := HealthView{
		Unwritten: []snapshot.HealthLink{{From: ref, Target: "Ghost"}},
	}
	var buf bytes.Buffer
	if err := Health(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	want := `<a class="y-healthlink" href="/notes/Writing/lessons/japanese/L01.md"><span lang="ja">L01</span></a>`
	if !strings.Contains(html, want) {
		t.Errorf("Health() missing %q in %q", want, html)
	}
}
