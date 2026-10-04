package lexical

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
)

func TestBriefingExcerpts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, query, text, landing string
		marks                              []string
	}{
		{"quoted tags", `<P a="quoted > needle">visible</P>`, "needle", "visible", "", nil},
		{"raw text", `<script>needle</scripture>still hidden</SCRIPT ><STYLE>hidden</style><p>visible</p>`, "needle", "visible", "", nil},
		{"comments", `<p>visible</p><!--needle-->`, "needle", "visible", "", nil},
		{"unclosed comment", `visible<!--needle`, "needle", "visible", "", nil},
		{"unclosed script", `visible<script>needle`, "needle", "visible", "", nil},
		{"entities", `<p>needle &amp; &lt;x&gt; &#65; &#x42;</p>`, "needle", "needle & <x> A B", "needle", []string{"needle"}},
		{"source-only entity hit", `<p>needle &amp;</p>`, "amp", "needle &", "", nil},
		{"visible entity hit", `<p>needle &amp;</p>`, "&", "needle &", "", []string{"&"}},
		{"semicolon-less entity", `<p>needle &notit &amp text</p>`, "needle", "needle ¬it & text", "needle", []string{"needle"}},
		{"inline and block boundaries", `<p>nee<b>dle</b></p><p>next</p>`, "next", "needle next", "next", []string{"next"}},
		{"pre code text", `<pre><code>needle &lt;tag&gt; ==literal== ~~literal~~</code></pre>`, "needle", "needle <tag> ==literal== ~~literal~~", "needle", []string{"needle"}},
		{"unfinished tag is literal", `needle <p a="unfinished`, "needle", `needle <p a="unfinished`, "needle", []string{"needle"}},
		{"short numeric context", `<p>needle&#09x &#65tail &#x41tail</p>`, "needle", "needle x Atail Atail", "needle", []string{"needle"}},
		{"one digit decoder control", `<p>needle&#9x &#9;</p>`, "needle", "needle&#9x", "needle", []string{"needle"}},
		{"digit start is displayed text", `needle <3>`, "needle", "needle <3>", "needle", []string{"needle"}},
		{"punctuation start is displayed text", `needle <-p> <:p>`, "needle", "needle <-p> <:p>", "needle", []string{"needle"}},
		{"bogus end tag is hidden", `needle </3>`, "needle", "needle", "needle", []string{"needle"}},
		{"unclosed quoted markup stays literal", `needle <p a="unfinished <b>literal`, "needle", `needle <p a="unfinished <b>literal`, "needle", []string{"needle"}},
		{"decoded occurrence needs its own source evidence", `<p>&NotEqualTilde; ≂</p>`, "≂", "≂̸ ≂", "≂", []string{"≂"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := DocumentFromBriefing("System/reports/daily-briefing/fixture.html", []byte(tt.source))
			got := displayedAnswer(t, &document, tt.query)
			want := excerptAnswer{Text: tt.text, Marks: tt.marks, Landing: tt.landing, Bare: tt.landing}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("briefing excerpt (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBriefingRawClosingCandidates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, element string
		want                  bool
	}{
		{"opening tag", "<script>", "script", false},
		{"own name at opening offset", "<xscript>", "script", false},
		{"foreign opening with unfinished quote", "<p a='", "script", false},
		{"foreign closing with unfinished quote", "</other a='", "script", false},
		{"foreign equal width closing name", "</foobar>", "script", false},
		{"name prefix", "</scripture>", "script", false},
		{"name suffix", "</script-other>", "script", false},
		{"spaced slash", "< /script>", "script", false},
		{"missing name", "</", "script", false},
		{"invalid name boundary", "</script!>", "script", false},
		{"case folded close", "</SCRIPT>", "script", true},
		{"whitespace boundary", "</script \t>", "script", true},
		{"slash boundary", "</style/>", "style", true},
		{"quoted greater than", "</script a='>'>", "script", true},
		{"unfinished own closing quote", "</script a='", "script", true},
		{"own name at eof", "</style", "style", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := briefingOwnClose(tt.source, 0, tt.element); got != tt.want {
				t.Errorf("briefingOwnClose(%q, %q) = %t, want %t", tt.source, tt.element, got, tt.want)
			}
		})
	}
}

func TestBriefingRawTextSourceCeiling(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, text string
	}{
		{
			name: "foreign openings", source: "<script>" + strings.Repeat("<p a='", (render.MaxSourceBytes-64)/len("<p a='")) + "</SCRIPT a='>'><p>visible</p>",
			text: "…visible",
		},
		{
			name: "foreign closings", source: "<script>" + strings.Repeat("</other a='", (render.MaxSourceBytes-64)/len("</other a='")) + "</SCRIPT a='>'><p>visible</p>",
			text: "…visible",
		},
		{
			name: "unfinished own closings", source: "visible<script>" + strings.Repeat("</script a='", (render.MaxSourceBytes-32)/len("</script a='")),
			text: "visible…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := DocumentFromBriefing("System/reports/daily-briefing/fixture.html", []byte(tt.source))
			got := displayedAnswer(t, &document, "visible")
			want := excerptAnswer{Text: tt.text, Marks: []string{"visible"}, Landing: "visible", Bare: "visible"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("raw text ceiling excerpt (-want +got):\n%s", diff)
			}
		})
	}
}

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
			name: "hidden overlap suppresses whole replacement", text: "needle visible", query: "visible",
			spans: []render.DisplaySpan{{Start: 0, End: 6, Replacement: "needle"}, {Start: 2, End: 4, Hidden: true}},
			want:  excerptAnswer{Text: "visible", Marks: []string{"visible"}, Landing: "visible", Bare: "visible"},
		},
		{
			name: "invalid replacement bytes cannot panic", text: "hide needle", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Replacement: "\xff"}},
			want:  excerptAnswer{Text: "\xff needle", Marks: []string{"needle"}, Landing: "needle", Bare: "needle"},
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
		{
			name: "overlapping replacement emits once", text: "abcdef needle", query: "needle",
			spans: []render.DisplaySpan{{Start: 0, End: 4, Replacement: "X"}, {Start: 2, End: 6, Replacement: "Y"}},
			want:  excerptAnswer{Text: "X needle", Marks: []string{"needle"}, Landing: "needle", Bare: "needle"},
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
			name: "complete entity at both cut edges", text: "aa &amp; bb", start: 3, end: 8,
			spans: []render.DisplaySpan{{Start: 3, End: 8, Replacement: "&"}},
			want:  []ExcerptRun{{Text: "…&…"}},
		},
		{
			name: "partial entity at right cut", text: "aa &amp; bb", start: 3, end: 7,
			spans: []render.DisplaySpan{{Start: 3, End: 8, Replacement: "&"}},
			want:  []ExcerptRun{{Text: "……"}},
		},
		{
			name: "partial entity at left cut", text: "aa &amp; bb", start: 4, end: 8,
			spans: []render.DisplaySpan{{Start: 3, End: 8, Replacement: "&"}},
			want:  []ExcerptRun{{Text: "……"}},
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

func FuzzBriefingDisplay(f *testing.F) {
	for _, source := range []string{`<p title="a > b">needle &amp;</p>`, `<!--`, `<script>needle</SCRIPT>`, `&#x31;`, "e\u0301 &#65;", strings.Repeat("<p a='", 100)} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		document := DocumentFromBriefing("fixture.html", []byte(source))
		index := NewIndex([]Document{document}, schema.ArtifactPolicy{})
		answer, err := index.Search(Parse("needle"), 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for i := range answer.Results {
			result := &answer.Results[i]
			if utf8.ValidString(source) && !utf8.ValidString(result.Snippet) {
				t.Fatalf("display corrupted UTF-8: %q", result.Snippet)
			}
		}
	})
}

func FuzzDisplayRanges(f *testing.F) {
	for _, seed := range []struct {
		text, replacement string
		start, end        int
	}{
		{"hide needle", "\xff", 0, 4},
		{"e\u0301 needle", "", 0, 1},
		{"needle", "", -1, 7},
		{"漢\n字 needle", "&", 0, 4},
	} {
		f.Add(seed.text, seed.replacement, seed.start, seed.end)
	}
	f.Fuzz(func(t *testing.T, text, replacement string, start, end int) {
		if len(text)+len(replacement) > 32768 {
			t.Skip()
		}
		document := Document{RelPath: "fixture.md", PlainText: text, DisplaySpans: []render.DisplaySpan{{Start: start, End: end, Hidden: replacement == "", Replacement: replacement}}}
		index := NewIndex([]Document{document}, schema.ArtifactPolicy{})
		answer, err := index.Search(Parse("needle"), 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for i := range answer.Results {
			result := &answer.Results[i]
			if utf8.ValidString(text) && utf8.ValidString(replacement) && !utf8.ValidString(result.Snippet) {
				t.Fatalf("display corrupted UTF-8: %q", result.Snippet)
			}
		}
	})
}
