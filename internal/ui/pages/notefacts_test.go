package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// dlBlock finds one <dl class="y-notefacts">…</dl> region so a count taken
// inside it says something about that list specifically, not about the whole
// page — a dt or dd anywhere else on a reading page (the keyboard-help panel
// carries its own) must not inflate or deflate what this counts.
var dlBlock = regexp.MustCompile(`(?s)<dl class="y-notefacts">.*?</dl>`)

// TestNoteHeadFactsAreAllDtDdPairs is the acceptance line itself: every fact
// the head shows is a dt/dd pair. Rather than enumerating the five fields by
// name — a table that silently stops proving anything the day a sixth fact is
// added beside them instead of inside noteFacts — it derives how many pairs
// are owed from how many of NoteView's own fact fields are populated, and
// checks the rendered list against that count. A fact added to the head
// outside noteFacts (a stray span in .y-metarow__detail, say) would grow the
// visible facts without growing this count, which is exactly the regression
// this test exists to catch.
func TestNoteHeadFactsAreAllDtDdPairs(t *testing.T) {
	t.Parallel()
	full := NoteView{
		Title:        "L01",
		RelPath:      "Writing/lessons/go/L01.md",
		Language:     "ja",
		Type:         "lesson",
		Status:       "draft",
		Updated:      "2026-07-10",
		UpdatedAt:    "2026-07-10",
		ObsidianHref: "obsidian://open?path=x",
	}
	tests := []struct {
		name string
		view NoteView
		want int
	}{
		{name: "every fact declared", view: full, want: 5},
		{name: "no language declared", view: func() NoteView { v := full; v.Language = ""; return v }(), want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			blocks := dlBlock.FindAllString(html, -1)
			if len(blocks) != 2 {
				t.Fatalf("found %d y-notefacts lists, want 2 (the folded narrow copy and the open wide copy)", len(blocks))
			}
			if blocks[0] != blocks[1] {
				t.Errorf("the narrow and wide copies disagree, so the two placements are not drawing the one shared component:\nnarrow: %s\nwide:   %s", blocks[0], blocks[1])
			}
			for i, block := range blocks {
				dts := strings.Count(block, "<dt")
				dds := strings.Count(block, "<dd")
				if dts != tt.want || dds != tt.want {
					t.Errorf("copy %d: %d <dt> and %d <dd>, want %d of each for %d populated facts", i, dts, dds, tt.want, tt.want)
				}
			}
		})
	}
}

// TestNoteHeadFactValues locks which dt names which dd: a reordering or a
// mismatched pair would pass a bare count and fail this.
func TestNoteHeadFactValues(t *testing.T) {
	t.Parallel()
	view := NoteView{
		Title:     "L01",
		RelPath:   "Writing/lessons/go/L01.md",
		Language:  "ja",
		Type:      "lesson",
		Status:    "draft",
		Updated:   "2026-07-10",
		UpdatedAt: "2026-07-10",
	}
	var buf bytes.Buffer
	if err := Note(view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	// The status pair is asserted as an exact, untagged "draft" between its own
	// <dd> and </dd> — the live control elsewhere on the page is the only door
	// to changing it, so a status value wrapped in a link or a form here would
	// open a second one, and the literal match below would stop matching.
	for _, want := range []string{
		`<dt lang="zh-Hant">類型</dt><dd>lesson</dd>`,
		`<dt lang="zh-Hant">狀態</dt><dd>draft</dd>`,
		`<dt lang="zh-Hant">語言</dt><dd>ja</dd>`,
		`<dt lang="zh-Hant">原始檔</dt><dd><a href="/raw/Writing/lessons/go/L01.md">Writing/lessons/go/L01.md</a></dd>`,
	} {
		if strings.Count(html, want) != 2 {
			t.Errorf("want %q exactly twice (narrow and wide copies), found %d", want, strings.Count(html, want))
		}
	}
}

// TestNoteHeadWithNoFactsDrawsNeitherCopy is the other side of headFactsShown:
// a note with none of the five facts draws no disclosure and no open list,
// rather than an empty dl either reader would have to open to learn is empty.
func TestNoteHeadWithNoFactsDrawsNeitherCopy(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Note(NoteView{Title: "Bare"}, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, "y-metarow") || strings.Contains(html, "y-headmeta") || strings.Contains(html, "y-notefacts") {
		t.Errorf("a note declaring none of the five facts still drew head-fact markup:\n%s", html)
	}
}
