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

// TestRailListingLanguageComesOnlyFromAuthority holds the left-rail folder shelf
// the same way: declared language on the row span, absent otherwise.
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
