package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// dlBlock finds the one <dl class="y-notefacts">…</dl> region so a count taken
// inside it says something about that list specifically, not about the whole
// page — a dt or dd anywhere else on a reading page (the keyboard-help panel
// carries its own) must not inflate or deflate what this counts.
var dlBlock = regexp.MustCompile(`(?s)<dl class="y-notefacts">.*?</dl>`)

func renderNoteView(t *testing.T, view *NoteView) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Note(*view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestNoteFactsAreAllDtDdPairs is the acceptance line itself: every fact the
// note shows is a dt/dd pair in one list. Rather than enumerating the fields by
// name — a table that silently stops proving anything the day another fact is
// added beside them instead of inside noteFacts — it derives how many pairs are
// owed from how many of NoteView's own fact fields are populated, and checks
// the rendered list against that count. A fact added outside noteFacts (a stray
// span beside the title, say) would grow the visible facts without growing this
// count, which is exactly the regression this test exists to catch.
func TestNoteFactsAreAllDtDdPairs(t *testing.T) {
	t.Parallel()
	full := NoteView{
		Title:         "L01",
		RelPath:       "Writing/lessons/go/L01.md",
		Language:      "ja",
		Type:          "lesson",
		Draft:         true,
		VerifiedOn:    "2026-07-10",
		VerifiedAt:    "2026-07-10",
		Prerequisites: []nav.NoteRef{{Name: "L00", RelPath: "Writing/lessons/go/L00.md"}},
		ObsidianHref:  "obsidian://open?path=x",
	}
	tests := []struct {
		name string
		view NoteView
		want int
	}{
		{name: "every fact declared", view: full, want: 5},
		{name: "no language declared", view: func() NoteView { v := full; v.Language = ""; return v }(), want: 4},
		{name: "no day declared", view: func() NoteView { v := full; v.VerifiedOn, v.VerifiedAt = "", ""; return v }(), want: 4},
		{name: "no prerequisites declared", view: func() NoteView { v := full; v.Prerequisites = nil; return v }(), want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			html := renderNoteView(t, &tt.view)
			blocks := dlBlock.FindAllString(html, -1)
			if len(blocks) != 1 {
				t.Fatalf("found %d y-notefacts lists, want exactly one", len(blocks))
			}
			dts, dds := strings.Count(blocks[0], "<dt"), strings.Count(blocks[0], "<dd")
			if dts != tt.want || dds != tt.want {
				t.Errorf("%d <dt> and %d <dd>, want %d of each for %d populated facts", dts, dds, tt.want, tt.want)
			}
		})
	}
}

// TestNoteFactValues locks which dt names which dd: a reordering or a
// mismatched pair would pass a bare count and fail this. The day is the author's
// own and is labelled as theirs; the file's change time is nowhere on the page.
func TestNoteFactValues(t *testing.T) {
	t.Parallel()
	view := NoteView{
		Title:         "L01",
		RelPath:       "Writing/lessons/go/L01.md",
		Language:      "ja",
		Type:          "lesson",
		Draft:         true,
		VerifiedOn:    "2026-07-10",
		VerifiedAt:    "2026-07-10",
		Prerequisites: []nav.NoteRef{{Name: "L00", RelPath: "Writing/lessons/go/L00.md"}, {Name: "not written yet"}},
	}
	html := renderNoteView(t, &view)
	for _, want := range []string{
		`<dt lang="zh-Hant">類型</dt><dd>lesson</dd>`,
		`<dt lang="zh-Hant">語言</dt><dd>ja</dd>`,
		`<dt lang="zh-Hant">` + wording.VerifiedOn.In(wording.ZhHant) + `</dt><dd><time datetime="2026-07-10">2026-07-10</time></dd>`,
		`<dt lang="zh-Hant">原始檔</dt><dd><a class="y-notefacts__path" href="/raw/Writing/lessons/go/L01.md">Writing/lessons/go/L01.md</a></dd>`,
	} {
		if strings.Count(html, want) != 1 {
			t.Errorf("want %q exactly once, found %d", want, strings.Count(html, want))
		}
	}
	prerequisites := `<dt lang="zh-Hant">` + wording.Prerequisites.In(wording.ZhHant) + `</dt><dd><ul class="y-notefacts__links" role="list"><li><a href="/notes/Writing/lessons/go/L00.md">L00</a></li><li><span>not written yet</span></li></ul></dd>`
	if !strings.Contains(html, prerequisites) {
		t.Errorf("the prerequisites are not a list of links with the unplaced one left as text; want %q", prerequisites)
	}
	for _, retired := range []string{"檔案變更於", "File changed"} {
		if strings.Contains(html, retired) {
			t.Errorf("the page still says %q, the file's change time, which a checkout stamps on every file at once", retired)
		}
	}
	// The draft flag is not a fact of the list: it is one quiet word above it,
	// and no control. A draft row, or a control, inside the facts would be a
	// second place that states it or a door to changing it.
	for _, block := range dlBlock.FindAllString(html, -1) {
		for _, banned := range []string{wording.Draft.In(wording.ZhHant), "draft", "<form", "<button"} {
			if strings.Contains(block, banned) {
				t.Errorf("the facts' list carries %q; the draft mark stands above the list", banned)
			}
		}
	}
}

// TestNoteFactsStandAtTheEndAndNothingStandsUnderTheTitle holds where the facts
// are: with what ends the text, after the prose, and not between the title and
// the first sentence. The title is the head and has nothing after it but the
// words of the note.
func TestNoteFactsStandAtTheEndAndNothingStandsUnderTheTitle(t *testing.T) {
	t.Parallel()
	view := NoteView{
		Title:        "L01",
		RelPath:      "Writing/lessons/go/L01.md",
		Language:     "ja",
		Type:         "lesson",
		Draft:        true,
		VerifiedOn:   "2026-07-10",
		VerifiedAt:   "2026-07-10",
		BodyHTML:     "<p>The prose.</p>",
		ObsidianHref: "obsidian://open?path=x",
	}
	html := renderNoteView(t, &view)
	title, prose, facts, end := strings.Index(html, `<h1 class="y-title">`), strings.Index(html, "<p>The prose.</p>"), strings.Index(html, `<dl class="y-notefacts">`), strings.Index(html, `class="y-related"`)
	if title < 0 || prose < title || end < prose || facts < end {
		t.Fatalf("title@%d prose@%d end@%d facts@%d, want them in that order", title, prose, end, facts)
	}
	if between := html[title:prose]; strings.Contains(between, "y-notefacts") || strings.Contains(between, "data-note-draft") || strings.Contains(between, "y-metarow") || strings.Contains(between, "y-headmeta") {
		t.Errorf("something of the note's facts stands between its title and its prose:\n%s", between)
	}
	for _, door := range []string{`href="obsidian://open?path=x"`} {
		if at := strings.Index(html, door); at < facts {
			t.Errorf("the door %q stands at %d, before the facts at %d", door, at, facts)
		}
	}
}

// TestNoteWithNoFactsDrawsNoFactsAndADraftIsOneWord is the other side of
// factsShown: a note with none of the facts draws no list for either reader to
// learn is empty, and a draft flag alone is the one quiet word with no list
// beneath it.
func TestNoteWithNoFactsDrawsNoFactsAndADraftIsOneWord(t *testing.T) {
	t.Parallel()
	bare := renderNoteView(t, &NoteView{Title: "Bare"})
	for _, kept := range []string{"y-metarow", "y-headmeta", "y-notefacts", "y-facts", `class="y-related"`} {
		if strings.Contains(bare, kept) {
			t.Errorf("a note declaring no fact still drew %q:\n%s", kept, bare)
		}
	}
	draft := renderNoteView(t, &NoteView{Title: "Bare", Draft: true})
	if !strings.Contains(draft, `<span class="ui-status" lang="zh-Hant" data-note-draft>草稿</span>`) {
		t.Errorf("a draft with no other fact lost its one word:\n%s", draft)
	}
	if strings.Contains(draft, "y-notefacts") {
		t.Errorf("a draft with no other fact drew an empty list:\n%s", draft)
	}
}
