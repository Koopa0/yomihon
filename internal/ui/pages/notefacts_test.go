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
// the head shows is a dt/dd pair. Rather than enumerating the four fields by
// name — a table that silently stops proving anything the day a fifth fact is
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
		{name: "every fact declared", view: full, want: 4},
		{name: "no language declared", view: func() NoteView { v := full; v.Language = ""; return v }(), want: 3},
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
	// Each pair is asserted as an exact string between its own <dt> and </dd>,
	// so a reordering or a value wrapped in something else stops matching.
	for _, want := range []string{
		`<dt lang="zh-Hant">更新於</dt><dd><time datetime="2026-07-10">2026-07-10</time></dd>`,
		`<dt lang="zh-Hant">類型</dt><dd>lesson</dd>`,
		`<dt lang="zh-Hant">語言</dt><dd>ja</dd>`,
		`<dt lang="zh-Hant">原始檔</dt><dd><a href="/raw/Writing/lessons/go/L01.md">Writing/lessons/go/L01.md</a></dd>`,
	} {
		if strings.Count(html, want) != 2 {
			t.Errorf("want %q exactly twice (narrow and wide copies), found %d", want, strings.Count(html, want))
		}
	}
	// The status is not a fact of the head: the status face states it, live,
	// wherever it can be changed. A status row, or a control, inside the facts
	// would be a second place that states it or a second door to changing it.
	for i, block := range dlBlock.FindAllString(html, -1) {
		for _, banned := range []string{"狀態", "draft", "<form", "<button"} {
			if strings.Contains(block, banned) {
				t.Errorf("copy %d of the head facts carries %q; the status is stated by the status face alone", i, banned)
			}
		}
	}
}

// TestNoteHeadWithNoFactsDrawsNeitherCopy is the other side of headFactsShown:
// a note with none of the four facts draws no disclosure and no open list,
// rather than an empty dl either reader would have to open to learn is empty.
// A status alone is not a fact of the head, so a note declaring only that is
// bare too.
func TestNoteHeadWithNoFactsDrawsNeitherCopy(t *testing.T) {
	t.Parallel()
	for name, view := range map[string]NoteView{
		"nothing declared": {Title: "Bare"},
		"a status alone":   {Title: "Bare", Status: "draft"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if strings.Contains(html, "y-metarow") || strings.Contains(html, "y-headmeta") || strings.Contains(html, "y-notefacts") {
				t.Errorf("a note declaring none of the four facts still drew head-fact markup:\n%s", html)
			}
		})
	}
}
