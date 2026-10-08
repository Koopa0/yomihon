package sequence

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestARowKeepsTheWordsItsAuthorWroteAfterTheLink holds what a course cover
// prints beside a lesson's title. The gloss is the author's own sentence, so
// the dash they joined it with and the punctuation inside it survive exactly,
// while the marks that only tell the browser how to draw a word — emphasis, a
// link's brackets, a code span's backticks, an Obsidian comment — are not
// words and do not reach the page as if they were.
func TestARowKeepsTheWordsItsAuthorWroteAfterTheLink(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  string
		want string
	}{
		{name: "an em dash and a sentence", row: "- [[G01]] — 啟動工作，等它完成\n", want: "— 啟動工作，等它完成"},
		{name: "the author's other joiner is kept as typed", row: "- [[G01]]：啟動工作\n", want: "：啟動工作"},
		{name: "no words after the link", row: "- [[G01]]\n", want: ""},
		{name: "only space after the link", row: "- [[G01]]   \n", want: ""},
		{name: "an alias is the title, not the gloss", row: "- [[G01|Start]] — go\n", want: "— go"},
		{name: "emphasis falls away around its words", row: "- [[L01]] · **L1** 〜は〜です — 名詞句\n", want: "· L1 〜は〜です — 名詞句"},
		{name: "a code span keeps its characters", row: "- [[G06]] — 等待 `select` 或計時器\n", want: "— 等待 select 或計時器"},
		{name: "a link keeps its words and loses its syntax", row: "- [[G06]] — 見 [Go 規格](https://go.dev/ref/spec) 的說明\n", want: "— 見 Go 規格 的說明"},
		{name: "an Obsidian comment is cut out", row: "- [[G01]] — 啟動 %%待改寫%% 工作\n", want: "— 啟動  工作"},
		{name: "a comment inside a code span is text", row: "- [[G01]] — 寫 `%%` 兩個字\n", want: "— 寫 %% 兩個字"},
		{name: "a break inside the block reads as a space", row: "- [[G01]] — 啟動\n  工作\n", want: "— 啟動 工作"},
		{name: "inline HTML has no words", row: "- [[G01]] — 啟動<br>工作\n", want: "— 啟動工作"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.row, 1)
			entries := doc.Groups[0].entries()
			if len(entries) != 1 {
				t.Fatalf("row %q produced %d entries, want 1", tt.row, len(entries))
			}
			if entries[0].State != EntryAccepted {
				t.Fatalf("row %q was not accepted (%v), so the gloss below proves nothing", tt.row, entries[0].State)
			}
			if got := entries[0].Gloss; got != tt.want {
				t.Errorf("row %q gloss = %q, want %q", tt.row, got, tt.want)
			}
		})
	}
}

// TestAGlossIsOnlyTheRowsOwnWords names what a gloss must not borrow: the
// words of a nested list the row opens, which belong to the rows beneath it,
// and anything on a row the grammar did not accept as a lesson.
func TestAGlossIsOnlyTheRowsOwnWords(t *testing.T) {
	t.Parallel()

	doc := Parse("## P {sequence=primary}\n\n"+
		"- [[A]] — 自己的話\n"+
		"\t- 支線 {sequence=local}\n"+
		"\t\t- [[B]] — 支線的話\n"+
		"- 前面有字 [[C]] — 不是課\n"+
		"- [[D]] — 提到 [[E]] 的課\n", 1)

	got := map[string]string{}
	for _, e := range allEntries(doc) {
		got[e.Target+"/"+e.State.String()] = e.Gloss
	}
	want := map[string]string{
		"A/accepted":     "— 自己的話",
		"B/accepted":     "— 支線的話",
		"C/noncanonical": "",
		"/multi-target":  "",
	}
	for key, w := range want {
		if g, ok := got[key]; !ok || g != w {
			t.Errorf("gloss of %q = %q (present %t), want %q", key, g, ok, w)
		}
	}
}

// TestARowOutsideTheCourseKeepsItsGlossToo is what lets a reference section be
// listed with its sentences: the grammar leaves such a row out of the course
// and still reads the words the author wrote beside it.
func TestARowOutsideTheCourseKeepsItsGlossToo(t *testing.T) {
	t.Parallel()

	doc := Parse("## 查閱與對照 {sequence=none}\n\n- [[Go 並行地圖]] — 等待、交接與收尾的關係\n", 1)
	entries := doc.Groups[0].entries()
	if len(entries) != 1 || entries[0].Gloss != "— 等待、交接與收尾的關係" {
		t.Errorf("entries = %+v, want one row carrying its gloss", entries)
	}
}

func TestFirstBlockPartsKeepTheWholeDocument(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		body   string
		span   Span
		target Span
		gloss  string
	}{
		{
			name: "soft break", body: "## P {sequence=primary}\n\n- [[A]] one\n  two\n",
			span: Span{Start: 27, Stop: 42}, target: Span{Start: 27, Stop: 32},
			gloss: "one two",
		},
		{
			name: "hard break", body: "## P {sequence=primary}\n\n- [[A]] one  \n  two\n",
			span: Span{Start: 27, Stop: 44}, target: Span{Start: 27, Stop: 32},
			gloss: "one two",
		},
		{
			name: "CRLF soft break", body: "## P {sequence=primary}\r\n\r\n- [[A]] one\r\n  two\r\n",
			span: Span{Start: 29, Stop: 45}, target: Span{Start: 29, Stop: 34},
			gloss: "one two",
		},
		{
			name: "raw multiline code", body: "## P {sequence=primary}\n\n- [[A]] `one\n  two`\n",
			span: Span{Start: 27, Stop: 44}, target: Span{Start: 27, Stop: 32},
			gloss: "one\n  two",
		},
		{
			name: "raw comment text parts", body: "## P {sequence=primary}\n\n- [[A]] one %%hidden%% two\n",
			span: Span{Start: 27, Stop: 51}, target: Span{Start: 27, Stop: 32},
			gloss: "one  two",
		},
		{
			name: "first container has no own lines", body: "## P {sequence=primary}\n\n- > [[A]] word\n",
			span: Span{Start: 29, Stop: 39}, target: Span{Start: 29, Stop: 34},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want := Document{Groups: []*Group{{
				Name: "P", Level: 2, Line: 7, Role: RolePrimary,
				Items: []Item{{Entry: &Candidate{
					Text: "A", Target: "A", Line: 9, Span: tt.span,
					Gloss: tt.gloss, TargetSpan: tt.target, State: EntryAccepted,
				}}},
			}}}
			if diff := cmp.Diff(want, Parse(tt.body, 7)); diff != "" {
				t.Errorf("caught: first-block provenance changed Document (-want +got):\n%s", diff)
			}
		})
	}
}
