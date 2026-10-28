package render_test

import (
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
)

func TestPlainBlockLocalContexts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, plain string
		heading           bool
		verbatim          bool
		ranges            [][2]int
		// end is the first block's end where it is not the whole text.
		end int
	}{
		// A heading's own words are what the contents list repeats, so it
		// offers no range even when it is shown as written.
		{name: "ordinary heading", body: "## 等待者應該放在哪裡？\n", plain: "等待者應該放在哪裡？", heading: true},
		{name: "strike keeps the source delimiters", body: "~~取消工作。~~\n", plain: "~~取消工作。~~", ranges: [][2]int{{2, 2 + len("取消工作。")}}},
		{name: "trimming shifts coordinates", body: "\n\n~~取消工作。~~\n\n", plain: "~~取消工作。~~", ranges: [][2]int{{2, 2 + len("取消工作。")}}},
		{name: "consumed role offers no context", body: "## 等待者 {sequence=primary}\n", plain: "等待者 {sequence=primary}"},
		{name: "unknown inline node offers no context", body: "![等待者](diagram.svg)取消工作。\n", plain: "等待者取消工作。"},
		{name: "ruby offers no context", body: "<ruby>今日<rt>きょう</rt></ruby>取消工作。\n", plain: "今日取消工作。\nきょう"},
		// Entities and escapes contribute the characters the page shows.
		// External link insertions are reported separately from block text.
		{name: "an entity reference", body: "Tom &amp; Jerry run.\n", plain: "Tom & Jerry run.", verbatim: true},
		{name: "a numeric character reference", body: "Tom &#38; Jerry run.\n", plain: "Tom & Jerry run.", verbatim: true},
		{name: "a backslash escape", body: "The snake\\_case name.\n", plain: "The snake_case name.", verbatim: true},
		{name: "a link", body: "[Go docs](https://go.dev) explains it.\n", plain: "Go docs explains it.", verbatim: true},
		{name: "an autolink", body: "<https://go.dev> explains it.\n", plain: "https://go.dev explains it.", verbatim: true},
		{name: "a bare address", body: "https://go.dev explains it.\n", plain: "https://go.dev explains it.", verbatim: true},
		{name: "a bare www address", body: "See www.example.com now.\n", plain: "See www.example.com now.", verbatim: true},
		{name: "a footnote reference", body: "Text[^1] here.\n\n[^1]: The note.\n", plain: "Text here.\nThe note.", end: len("Text here.")},
		{name: "inline html", body: "Some <b>bold</b> words.\n", plain: "Some bold words."},
		{name: "a heading holding an entity reference", body: "## X &amp; place\n", plain: "X & place", heading: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plain, blocks, _ := render.PlainBlocks(tt.body)
			if diff := cmp.Diff(tt.plain, plain); diff != "" {
				t.Fatalf("PlainBlocks(%q) source (-want +got):\n%s", tt.body, diff)
			}
			end := len(tt.plain)
			if tt.end != 0 {
				end = tt.end
			}
			want := render.Block{End: end, Verbatim: tt.verbatim, Heading: tt.heading, ContextRanges: tt.ranges}
			// Ruby readings form their own block after the visible sentence.
			if tt.name == "ruby offers no context" {
				want.End = len("今日取消工作。")
			}
			if len(blocks) == 0 {
				t.Fatal("PlainBlocks() returned no blocks")
			}
			if diff := cmp.Diff(want, blocks[0]); diff != "" {
				t.Errorf("PlainBlocks(%q) first block (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

func FuzzPlainBlockContexts(f *testing.F) {
	for _, body := range []string{"## 等待者？\n", "~~取消工作。~~\n", "## cafe\u0301等待者。\n", "~~咖~~啡色。\n", "<ruby>今日<rt>きょう</rt></ruby>。\n", "## {sequence=primary}\n", "\x00\xff"} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		plain, blocks, _ := render.PlainBlocks(body)
		low := 0
		for _, block := range blocks {
			for _, span := range block.ContextRanges {
				if span[0] < low || span[0] >= span[1] || span[1] > block.End || block.End > len(plain) {
					t.Fatalf("PlainBlocks(%q) context %v lies outside block [%d,%d)", body, span, low, block.End)
				}
				if utf8.ValidString(plain) && !utf8.ValidString(plain[span[0]:span[1]]) {
					t.Fatalf("PlainBlocks(%q) context %v splits a rune", body, span)
				}
			}
			if block.Heading && len(block.ContextRanges) != 0 {
				t.Fatalf("PlainBlocks(%q) heading block %+v offers context", body, block)
			}
			low = block.End
		}
	})
}
