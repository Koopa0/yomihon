package pages

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// TestAHeadingTheAuthorNumberedIsRecognised is the reading of a heading that
// already carries its author's count. Each row is a way notes are actually
// numbered; the second half is headings that open on something else and must
// not be taken for a count, or a note would lose its numbers for a word.
func TestAHeadingTheAuthorNumberedIsRecognised(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"1. Replication", true},
		{"3 Storage and Retrieval", true},
		{"1.2 Leaders and followers", true},
		{"07 Transactions", true},
		{"３　全角", true},
		{"§4 Scope", true},
		{"第三節：失約的燈", true},
		{"第2章 分區", true},
		{"一、緒論", true},
		{"貳．方法", true},
		{"IV. Results", true},
		{"II) Method", true},
		{"  2 leading space", true},
		{"The first mechanism", false},
		{"Isolation levels", false},
		{"I think, therefore", false},
		{"Version 2", false},
		{"快照隔離抓不到的寫偏斜", false},
		{"一致性與共識", false},
		{"第", false},
		{"", false},
	} {
		if got := opensOnOwnNumber(tc.text); got != tc.want {
			t.Errorf("opensOnOwnNumber(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// TestANoteIsNumberedAllOrNothing holds the page to counting a note's sections
// only where the author did not: one ## heading of the author's numbered is
// enough to leave the whole note to the author's count, a note with no ##
// heading has nothing to count, and a page holding two notes counts neither.
func TestANoteIsNumberedAllOrNothing(t *testing.T) {
	t.Parallel()
	const flag = `<div class="y-prose" data-section-numbers>`
	r := render.New(graph.BuildFromNotes(nil, nil), emptyBodies{}, noDeclaredTitles{}, vaultHolds{})
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "sections the author left uncounted", body: "intro\n\n## Alpha\n\ntext\n\n## Beta\n\ntext\n", want: true},
		{name: "a deeper heading that opens on a number", body: "intro\n\n## Alpha\n\n### 1. Step\n\n## Beta\n", want: true},
		{name: "one section the author numbered", body: "intro\n\n## Alpha\n\ntext\n\n## 2. Beta\n\ntext\n", want: false},
		{name: "every section the author numbered", body: "## 第一節\n\ntext\n\n## 第二節\n\ntext\n", want: false},
		{name: "no ## heading", body: "intro\n\n### Only deeper\n\ntext\n", want: false},
	} {
		page := renderNote(t, r, "Probe", tc.body)
		if got := strings.Contains(page, flag); got != tc.want {
			t.Errorf("%s: the page counts the sections = %v, want %v", tc.name, got, tc.want)
		}
	}

	nested := NoteView{NestedTitle: true, TOC: []render.TOCEntry{{Level: 2, Text: "Alpha"}, {Level: 2, Text: "Beta"}}}
	if nested.numbersSections() {
		t.Error("a note nested under another page's heading counts its sections; a page holding two notes counts neither")
	}
}

// TestTheRibbonHangsWhereTheBookmarkIsOffered holds the ribbon to the control
// it pictures: it is drawn where the header offers to keep a place in this
// note and nowhere else, it is hidden from assistive technology, and it holds
// nothing a pointer or a keyboard could take.
func TestTheRibbonHangsWhereTheBookmarkIsOffered(t *testing.T) {
	t.Parallel()
	offered := NoteView{Title: "Probe", RelPath: "Notes/Probe.md", ContentIdentity: "abc", MarkAddress: "/bookmark"}
	for _, tc := range []struct {
		name string
		view NoteView
		want int
	}{
		{name: "a note offering the bookmark", view: offered, want: 1},
		{name: "a note offering none", view: NoteView{Title: "Probe", RelPath: "Notes/Probe.md", ContentIdentity: "abc"}, want: 0},
	} {
		page := renderNoteView(t, &tc.view)
		if got := strings.Count(page, `class="y-ribbon"`); got != tc.want {
			t.Fatalf("%s: the page hangs %d ribbons, want %d", tc.name, got, tc.want)
		}
		if tc.want == 0 {
			continue
		}
		_, ribbon, _ := strings.Cut(page, `<span class="y-ribbon"`)
		ribbon, _, _ = strings.Cut(ribbon, "</span>")
		if !strings.HasPrefix(ribbon, ` aria-hidden="true">`) {
			t.Errorf("%s: the ribbon is not hidden from assistive technology: %q", tc.name, ribbon)
		}
		for _, taken := range []string{"<a ", "<button", "tabindex", "href="} {
			if strings.Contains(ribbon, taken) {
				t.Errorf("%s: the ribbon holds %q, which a pointer or keyboard could take: %q", tc.name, taken, ribbon)
			}
		}
	}
	nested := offered
	nested.NestedTitle = true
	if nested.carriesRibbon() {
		t.Error("a note nested in a page holding two notes hangs a ribbon; the page has one header and no one note for it")
	}
}

// TestTheRunningHeadNamesTheNotesSections holds what the header's running head
// is drawn from: the note's level-two headings and nothing deeper, on a note's
// own page, and nothing at all for a note with no such heading or for a note
// standing in a page that holds two.
func TestTheRunningHeadNamesTheNotesSections(t *testing.T) {
	t.Parallel()
	toc := []render.TOCEntry{{Level: 2, Text: "Alpha", ID: "alpha"}, {Level: 3, Text: "Inside", ID: "inside"}, {Level: 2, Text: "Beta", ID: "beta"}}
	got := (&NoteView{Language: "en", TOC: toc}).runningHead()
	want := &layouts.RunningHead{Language: "en", Sections: []layouts.RunningSection{{ID: "alpha", Text: "Alpha"}, {ID: "beta", Text: "Beta"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("runningHead() mismatch (-want +got):\n%s", diff)
	}
	if head := (&NoteView{TOC: toc[1:2]}).runningHead(); head != nil {
		t.Errorf("a note with no ## heading has a running head: %+v", head)
	}
	if head := (&NoteView{NestedTitle: true, TOC: toc}).runningHead(); head != nil {
		t.Errorf("a note in a page holding two has a running head: %+v", head)
	}
}
