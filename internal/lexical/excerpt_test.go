package lexical

import (
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
)

func TestDisplayNFCAndLanding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text, query string
		spans             []render.DisplaySpan
		// blocks, where given, replace the one block the text is otherwise:
		// a prefix needs a genuine unchanged block, and a hidden span before
		// it cannot become its locatable prefix.
		blocks []render.Block
		want   excerptAnswer
	}{
		{
			name: "composed prefix keeps original visible landing", text: "cafe\u0301 {hidden} visible", query: "visible",
			spans: []render.DisplaySpan{{Start: 7, End: 15, Hidden: true}},
			want:  excerptAnswer{Landing: "visible", Bare: "visible"},
		},
		{
			name: "unrepresentable composed split remains literal", text: "e\u0301 visible", query: "visible",
			spans: []render.DisplaySpan{{Start: 0, End: 1, Hidden: true}},
			want:  excerptAnswer{Landing: "visible", Bare: "visible"},
		},
		{
			name: "hidden first hit gives way to shown occurrence", text: "needle before real needle tail", query: "needle",
			spans:  []render.DisplaySpan{{Start: 0, End: 6, Hidden: true}},
			blocks: []render.Block{{End: 6}, {End: len("needle before real needle tail"), Verbatim: true}},
			want:   excerptAnswer{Landing: "needle", Bare: "needle", Prefix: "before real"},
		},
		{
			name: "wholly hidden hit stays findable with bare link", text: "needle before real words", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 6, Hidden: true}},
			want:  excerptAnswer{},
		},
		{
			name: "cleanup cannot manufacture a query hit", text: "aHIDEb ab", query: "ab",
			spans: []render.DisplaySpan{{Start: 1, End: 5, Hidden: true}},
			want:  excerptAnswer{Landing: "ab", Bare: "ab"},
		},
		{
			name: "quoted phrase carries collapsed space coordinates", text: "hide hello \t world end", query: `"hello world"`,
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Landing: "hello world", Bare: "hello world"},
		},
		{
			name: "CJK wrap keeps original match", text: "hide 漢\n字 end", query: "漢字",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Landing: "漢 字", Bare: "漢 字"},
		},
		{
			name: "reordered combining prefix maps whole string", text: "a\u0301\u0323 {hidden} needle", query: "needle",
			spans: []render.DisplaySpan{{Start: 6, End: 14, Hidden: true}},
			want:  excerptAnswer{Landing: "needle", Bare: "needle"},
		},
		{
			name: "width-folded visible hit keeps original bytes", text: "hide ＮＥＥＤＬＥ tail", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Landing: "ＮＥＥＤＬＥ", Bare: "ＮＥＥＤＬＥ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			blocks := tt.blocks
			if blocks == nil {
				blocks = []render.Block{{End: len(tt.text)}}
			}
			document := Document{RelPath: "Notes/fixture.md", PlainText: tt.text, Blocks: blocks, DisplaySpans: tt.spans}
			got := displayedAnswer(t, &document, tt.query)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("display landing (-want +got):\n%s", diff)
			}
		})
	}
}

type excerptAnswer struct {
	Landing, Bare, Prefix string
}

func displayedAnswer(t *testing.T, document *Document, query string) excerptAnswer {
	t.Helper()
	result := soleResult(t, NewIndex([]Document{*document}, schema.ArtifactPolicy{}), query)
	return excerptAnswer{Landing: result.Landing, Bare: result.LandingBare, Prefix: result.LandingPrefix}
}

func FuzzDisplayRanges(f *testing.F) {
	for _, seed := range []struct {
		text       string
		start, end int
		deleted    bool
	}{
		{"hide \xff needle", 0, 4, false},
		{"e\u0301 needle", 0, 1, false},
		{"needle", -1, 7, false},
		{"漢\n字 needle", 0, 4, true},
	} {
		f.Add(seed.text, seed.start, seed.end, seed.deleted)
	}
	f.Fuzz(func(t *testing.T, text string, start, end int, deleted bool) {
		if len(text) > 32768 {
			t.Skip()
		}
		document := Document{RelPath: "fixture.md", PlainText: text, DisplaySpans: []render.DisplaySpan{{Start: start, End: end, Hidden: !deleted, Deleted: deleted}}}
		index := NewIndex([]Document{document}, schema.ArtifactPolicy{})
		answer, err := index.Search(Parse("needle"), 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for i := range answer.Results {
			result := &answer.Results[i]
			if utf8.ValidString(text) && (!utf8.ValidString(result.Landing) || !utf8.ValidString(plain(result.Lead.Runs))) {
				t.Fatalf("display corrupted UTF-8: landing %q, lead %q", result.Landing, plain(result.Lead.Runs))
			}
		}
	})
}
