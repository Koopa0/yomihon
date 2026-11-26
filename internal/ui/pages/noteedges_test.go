package pages

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheHeadSaysWhatItKnowsAboutTheFileUnderTheTitle pins where the page says
// what it knows about the file rather than read from it: inside the head, after
// the title and before the controls, so the double rule under the head parts
// all of it from the text and a reader meets it on arrival at every width. The
// order is the reading kept from before, the fence that never closes, then the
// schema's findings, each led by the warning triangle; a field the rules list
// the values of folds them under its sentence, closed.
func TestTheHeadSaysWhatItKnowsAboutTheFileUnderTheTitle(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:               "A",
		RelPath:             "Notes/a.md",
		BodyHTML:            "<p>The prose.</p>",
		Stale:               true,
		FrontmatterUnclosed: true,
		SchemaNotices: []SchemaNotice{
			{Says: []wording.SchemaPart{{Text: "type", Code: true}, {Text: " is field-note."}}, Field: "type", Known: []string{"concept", "guide"}},
			{Says: []wording.SchemaPart{{Text: "mystery_key", Code: true}, {Text: " is unknown."}}},
		},
	}
	html := renderComponent(t, Note(view, layouts.Chrome{Lang: wording.En}))

	at := strings.Index(html, `<header class="y-frontis">`)
	if at < 0 {
		t.Fatalf("no head in %s", html)
	}
	head, _, ok := strings.Cut(html[at:], "</header>")
	if !ok {
		t.Fatalf("no closed head in %s", html)
	}
	title := strings.Index(head, `<h1 class="y-title">`)
	stale := strings.Index(head, `<p class="y-headnote" data-note-stale>`)
	fence := strings.Index(head, `<p class="y-headnote" data-frontmatter-unclosed>`)
	schemaAt := strings.Index(head, `<p class="y-headnote" data-schema-notice>`)
	if title < 0 || stale < title || fence < stale || schemaAt < fence {
		t.Errorf("head order: title@%d stale@%d fence@%d schema@%d, want them in that order:\n%s", title, stale, fence, schemaAt, head)
	}
	if got := strings.Count(head, `data-icon="warning"`); got != 4 {
		t.Errorf("the head draws %d warning triangles, want one per note (4):\n%s", got, head)
	}
	fold := `<details class="y-headnote__known"><summary>Allowed <code>type</code> values`
	if got := strings.Count(head, fold); got != 1 {
		t.Errorf("the head folds %d lists of values under the type's sentence, want 1 and closed:\n%s", got, head)
	}
	if !strings.Contains(head, `<code>concept</code>, <code>guide</code>`) {
		t.Errorf("the folded list does not name the rules' values in order:\n%s", head)
	}
	for _, gone := range []string{"y-statusflag", "y-fileinfo__note", "<details class=\"y-headnote__known\" open"} {
		if strings.Contains(html, gone) {
			t.Errorf("the page still draws %q", gone)
		}
	}
	body := strings.Index(html, "<p>The prose.</p>")
	if body < strings.Index(html, "</header>") {
		t.Errorf("the prose stands before the head closes")
	}
}

// TestAFoldOfValuesSaysWhenTheRulesListNone keeps the fold honest about an
// empty list: the rules gave the field a list with nothing in it, which the fold
// says, while a field the rules give no list has no fold at all.
func TestAFoldOfValuesSaysWhenTheRulesListNone(t *testing.T) {
	t.Parallel()

	notice := func(known []string) string {
		v := NoteView{Title: "A", SchemaNotices: []SchemaNotice{{Says: []wording.SchemaPart{{Text: "x"}}, Field: "status", Known: known}}}
		return renderComponent(t, headNotes(v, wording.ZhHant))
	}
	if html := notice([]string{}); !strings.Contains(html, "<p>無。</p>") {
		t.Errorf("an empty list does not say it is empty: %s", html)
	}
	if html := notice(nil); strings.Contains(html, "<details") {
		t.Errorf("a field with no list folds one: %s", html)
	}
}

// TestTheHeadEndsOnWhatTheNoteIs holds the word the row above the title ends
// on: an index and a course say so before a draft says it is one, a report says
// so, and a template is never called a draft, because the draft it writes is a
// field of the notes made from it.
func TestTheHeadEndsOnWhatTheNoteIs(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		view  NoteView
		words []string
		kind  bool
		draft bool
	}{
		{name: "an index draft", view: NoteView{Kind: KindIndex, Draft: true}, words: []string{"索引", "草稿"}, kind: true, draft: true},
		{name: "a course", view: NoteView{Kind: KindCourse}, words: []string{"課程"}, kind: true},
		{name: "a report", view: NoteView{Kind: KindReport}, words: []string{"報告"}, kind: true},
		{name: "a template that writes draft", view: NoteView{Kind: KindTemplate, Draft: true}},
		{name: "a note", view: NoteView{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.view.Title, tt.view.RelPath = "A", "Notes/a.md"
			row := renderComponent(t, noteWhere(tt.view, wording.ZhHant))
			last := -1
			for _, word := range tt.words {
				at := strings.Index(row, word)
				if at <= last {
					t.Errorf("%q stands at %d, after %d: %s", word, at, last, row)
				}
				last = at
			}
			if got := strings.Contains(row, "data-note-draft"); got != tt.draft {
				t.Errorf("the row marks a draft = %v, want %v: %s", got, tt.draft, row)
			}
			if got := strings.Contains(row, "data-note-kind"); got != tt.kind {
				t.Errorf("the row says a kind = %v, want %v: %s", got, tt.kind, row)
			}
		})
	}
}

// TestAFileThatCouldNotBeReadIsAskedForAgainWhereItIs pins the way out of the
// page for a note that is there and could not be read: a read of the same
// address, escaped, in place of a search for something that is not lost, and
// never a link off this server.
func TestAFileThatCouldNotBeReadIsAskedForAgainWhereItIs(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		view NotFoundView
		want string
	}{
		{name: "unreadable", view: NotFoundView{Asked: "/notes/Notes/a b#1.md", Unreadable: true}, want: "/notes/Notes/a%20b%231.md"},
		{name: "parse failed", view: NotFoundView{Asked: "/notes/讀.md", Unreadable: true, ParsePanic: true}, want: "/notes/%E8%AE%80.md"},
		{name: "nothing there", view: NotFoundView{Asked: "/notes/nope.md"}},
		{name: "another host", view: NotFoundView{Asked: "//evil.example/x", Unreadable: true}},
		{name: "not a path", view: NotFoundView{Asked: "notes/a.md", Unreadable: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.view.againHref(); got != tt.want {
				t.Errorf("againHref(%q) = %q, want %q", tt.view.Asked, got, tt.want)
			}
			page := renderComponent(t, NotFound(tt.view, layouts.Chrome{Lang: wording.ZhHant}))
			again := strings.Contains(page, "data-read-again")
			search := strings.Contains(page, `<a class="y-recovery__action" href="/search">`)
			if again != (tt.want != "") || search == again {
				t.Errorf("read again = %v and search = %v, want exactly one, the read where there is an address: %s", again, search, page)
			}
			// What to do about an unreadable file is in its lede; looking for
			// it in the folders is not.
			if tt.view.Unreadable && strings.Contains(page, wording.NotFoundNext.In(wording.ZhHant)) {
				t.Errorf("an unreadable file is told to look for itself in the folders")
			}
		})
	}
}
