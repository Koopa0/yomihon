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
		want              excerptAnswer
	}{
		{
			name: "composed prefix keeps original visible landing", text: "cafe\u0301 {hidden} visible", query: "visible",
			spans: []render.DisplaySpan{{Start: 7, End: 15, Hidden: true}},
			want:  excerptAnswer{Text: "café visible", Marks: []string{"visible"}, Landing: "visible", Bare: "visible"},
		},
		{
			name: "unrepresentable composed split remains literal", text: "e\u0301 visible", query: "visible",
			spans: []render.DisplaySpan{{Start: 0, End: 1, Hidden: true}},
			want:  excerptAnswer{Text: "é visible", Marks: []string{"visible"}, Landing: "visible", Bare: "visible"},
		},
		{
			name: "hidden first hit gives way to shown occurrence", text: "needle before real needle tail", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 6, Hidden: true}},
			want:  excerptAnswer{Text: "before real needle tail", Marks: []string{"needle"}, Landing: "needle", Bare: "needle", Prefix: "before real"},
		},
		{
			name: "wholly hidden hit stays findable with bare link", text: "needle before real words", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 6, Hidden: true}},
			want:  excerptAnswer{Text: "before real words"},
		},
		{
			name: "cleanup cannot manufacture a query hit", text: "aHIDEb ab", query: "ab",
			spans: []render.DisplaySpan{{Start: 1, End: 5, Hidden: true}},
			want:  excerptAnswer{Text: "ab ab", Marks: []string{"ab"}, Landing: "ab", Bare: "ab"},
		},
		{
			name: "quoted phrase carries collapsed space coordinates", text: "hide hello \t world end", query: `"hello world"`,
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Text: "hello world end", Marks: []string{"hello world"}, Landing: "hello world", Bare: "hello world"},
		},
		{
			name: "CJK wrap keeps original match", text: "hide 漢\n字 end", query: "漢字",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Text: "漢 字 end", Marks: []string{"漢 字"}, Landing: "漢 字", Bare: "漢 字"},
		},
		{
			name: "reordered combining prefix maps whole string", text: "a\u0301\u0323 {hidden} needle", query: "needle",
			spans: []render.DisplaySpan{{Start: 6, End: 14, Hidden: true}},
			want:  excerptAnswer{Text: "ạ́ needle", Marks: []string{"needle"}, Landing: "needle", Bare: "needle"},
		},
		{
			name: "width-folded visible hit keeps original bytes", text: "hide ＮＥＥＤＬＥ tail", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Hidden: true}},
			want:  excerptAnswer{Text: "ＮＥＥＤＬＥ tail", Marks: []string{"ＮＥＥＤＬＥ"}, Landing: "ＮＥＥＤＬＥ", Bare: "ＮＥＥＤＬＥ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := Document{RelPath: "Notes/fixture.md", PlainText: tt.text, Blocks: []render.Block{{End: len(tt.text), Verbatim: false}}, DisplaySpans: tt.spans}
			// Prefix cases require a genuine unchanged block. The hidden span
			// precedes this block and cannot become its locatable prefix.
			if tt.name == "hidden first hit gives way to shown occurrence" {
				document.Blocks = []render.Block{{End: 6}, {End: len(tt.text), Verbatim: true}}
			}
			got := displayedAnswer(t, &document, tt.query)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("display landing (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExcerptWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text string
		start, end int
		spans      []render.DisplaySpan
		want       []ExcerptRun
	}{
		{
			name: "retraction opened before cut", text: "~~needle tail~~", start: 4, end: 11,
			spans: []render.DisplaySpan{{Start: 0, End: 2, Hidden: true}, {Start: 2, End: 13, Deleted: true}, {Start: 13, End: 15, Hidden: true}},
			want:  []ExcerptRun{{Text: "…"}, {Text: "edle ta", Deleted: true}, {Text: "…"}},
		},
		{
			name: "hidden span crosses opening", text: "abcXdef", start: 3, end: 6,
			spans: []render.DisplaySpan{{Start: 2, End: 5, Hidden: true}},
			want:  []ExcerptRun{{Text: "…e…"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			indexed := entry{PlainText: tt.text, displaySpans: tt.spans}
			got, _, _ := indexed.displayedExcerpt(tt.start, tt.end, []string{"needle"})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("excerpt window (-want +got):\n%s", diff)
			}
		})
	}
}

type excerptAnswer struct {
	Text, Landing, Bare, Prefix string
	Marks                       []string
}

func displayedAnswer(t *testing.T, document *Document, query string) excerptAnswer {
	t.Helper()
	index := NewIndex([]Document{*document}, schema.ArtifactPolicy{})
	answer, err := index.Search(Parse(query), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if answer.Total != 1 || len(answer.Results) != 1 {
		t.Fatalf("Search total/rows = %d/%d, want 1/1", answer.Total, len(answer.Results))
	}
	result := answer.Results[0]
	got := excerptAnswer{Text: result.Snippet, Landing: result.Landing, Bare: result.LandingBare, Prefix: result.LandingPrefix}
	for _, run := range result.SnippetRuns {
		if run.Hit {
			got.Marks = append(got.Marks, run.Text)
		}
	}
	if result.SnippetRuns == nil {
		for _, run := range MarkHits(result.Snippet, Parse(query).Tokens()) {
			if run.Hit {
				got.Marks = append(got.Marks, run.Text)
			}
		}
	}
	return got
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
			if utf8.ValidString(text) && !utf8.ValidString(result.Snippet) {
				t.Fatalf("display corrupted UTF-8: %q", result.Snippet)
			}
		}
	})
}
