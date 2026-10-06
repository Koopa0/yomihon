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
		ranges            [][2]int
	}{
		{name: "ordinary heading", body: "## 等待者應該放在哪裡？\n", plain: "等待者應該放在哪裡？", heading: true, ranges: [][2]int{{0, len("等待者應該放在哪裡？")}}},
		{name: "strike keeps the source delimiters", body: "~~取消工作。~~\n", plain: "~~取消工作。~~", ranges: [][2]int{{2, 2 + len("取消工作。")}}},
		{name: "trimming shifts coordinates", body: "\n\n~~取消工作。~~\n\n", plain: "~~取消工作。~~", ranges: [][2]int{{2, 2 + len("取消工作。")}}},
		{name: "consumed role offers no context", body: "## 等待者 {sequence=primary}\n", plain: "等待者 {sequence=primary}"},
		{name: "unknown inline node offers no context", body: "![等待者](diagram.svg)取消工作。\n", plain: "等待者取消工作。"},
		{name: "ruby offers no context", body: "<ruby>今日<rt>きょう</rt></ruby>取消工作。\n", plain: "今日取消工作。\nきょう"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plain, blocks, _ := render.PlainBlocks(tt.body)
			if diff := cmp.Diff(tt.plain, plain); diff != "" {
				t.Fatalf("PlainBlocks(%q) source (-want +got):\n%s", tt.body, diff)
			}
			want := render.Block{End: len(tt.plain), Heading: tt.heading, ContextRanges: tt.ranges}
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
			if block.Heading && (len(block.ContextRanges) != 1 || block.ContextRanges[0][1] != block.End) {
				t.Fatalf("PlainBlocks(%q) heading block %+v is not one unchanged range", body, block)
			}
			low = block.End
		}
	})
}
