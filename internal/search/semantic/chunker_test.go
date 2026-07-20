package semantic_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/search/semantic"
)

func TestProxyTokensUsesPinnedUnicodeBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "four ASCII runes share one proxy token", text: "abcd", want: 1},
		{name: "Han", text: strings.Repeat("漢", 4), want: 4},
		{name: "Hiragana", text: strings.Repeat("あ", 4), want: 4},
		{name: "Katakana", text: strings.Repeat("ア", 4), want: 4},
		{name: "before CJK symbols", text: strings.Repeat(string(rune(0x2fff)), 4), want: 1},
		{name: "first CJK symbol", text: strings.Repeat(string(rune(0x3000)), 4), want: 4},
		{name: "last CJK symbol", text: strings.Repeat(string(rune(0x303f)), 4), want: 4},
		{name: "after CJK symbols", text: strings.Repeat(string(rune(0x3040)), 4), want: 1},
		{name: "before fullwidth", text: strings.Repeat(string(rune(0xfeff)), 4), want: 1},
		{name: "first fullwidth", text: strings.Repeat(string(rune(0xff00)), 4), want: 4},
		{name: "last fullwidth", text: strings.Repeat(string(rune(0xffef)), 4), want: 4},
		{name: "after fullwidth", text: strings.Repeat(string(rune(0xfff0)), 4), want: 1},
		{name: "mixed counters", text: "漢abcd", want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := semantic.ProxyTokens(tt.text); got != tt.want {
				t.Errorf("ProxyTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestChunkDocumentUsesHeadingPathAndDropsEmptySections(t *testing.T) {
	t.Parallel()

	body := `preamble

## 課

本文

### 例

example

## Empty

---
`
	got := semantic.ChunkNote("日本語", body, 100)
	if len(got.Failures) != 0 {
		t.Fatalf("ChunkDocument() failures = %+v, want none", got.Failures)
	}
	if len(got.Chunks) != 3 {
		t.Fatalf("ChunkDocument() chunks = %+v, want 3 kept sections", got.Chunks)
	}
	wantSubmitted := []string{
		"title: 日本語 | text: preamble",
		"title: 日本語 › 課 | text: 本文",
		"title: 日本語 › 課 › 例 | text: example",
	}
	for i, chunk := range got.Chunks {
		if chunk.Ordinal != i {
			t.Errorf("chunk %d ordinal = %d, want %d", i, chunk.Ordinal, i)
		}
		if chunk.Submitted != wantSubmitted[i] {
			t.Errorf("chunk %d Submitted = %q, want %q", i, chunk.Submitted, wantSubmitted[i])
		}
		if chunk.ProxyTokens != semantic.ProxyTokens(chunk.Submitted) {
			t.Errorf("chunk %d ProxyTokens = %d, recomputed %d", i, chunk.ProxyTokens, semantic.ProxyTokens(chunk.Submitted))
		}
	}
}

func TestChunkDocumentSplitsOversizeSectionsAtParagraphs(t *testing.T) {
	t.Parallel()

	const first = "first paragraph carries enough ASCII text"
	const second = "second paragraph carries enough ASCII text"
	body := "## H\n\n" + first + "\n\n" + second + "\n"
	tokenCap := max(
		semantic.ProxyTokens("title: T › H — part 9/9 | text: "+first),
		semantic.ProxyTokens("title: T › H — part 9/9 | text: "+second),
	)
	got := semantic.ChunkNote("T", body, tokenCap)
	if len(got.Failures) != 0 {
		t.Fatalf("ChunkDocument() failures = %+v, want none", got.Failures)
	}
	if len(got.Chunks) != 2 {
		t.Fatalf("ChunkDocument() chunks = %+v, want one per paragraph", got.Chunks)
	}
	for i, chunk := range got.Chunks {
		if chunk.Part != i+1 || chunk.Parts != 2 {
			t.Errorf("chunk %d part = %d/%d, want %d/2", i, chunk.Part, chunk.Parts, i+1)
		}
		if chunk.ProxyTokens > tokenCap {
			t.Errorf("chunk %d proxy tokens = %d, cap %d", i, chunk.ProxyTokens, tokenCap)
		}
		if !strings.Contains(chunk.Submitted, "— part ") {
			t.Errorf("chunk %d Submitted = %q, missing continuation label", i, chunk.Submitted)
		}
	}
	if got.Chunks[0].Body != first || got.Chunks[1].Body != second {
		t.Errorf("paragraph bodies = %q / %q, want intact paragraphs", got.Chunks[0].Body, got.Chunks[1].Body)
	}
}

func TestChunkDocumentSplitsCodeAtLinesBeforeRunes(t *testing.T) {
	t.Parallel()

	const line1 = "const firstValue = 1111111111"
	const line2 = "\tconst secondValue = 2222222222"
	body := "## Code\n\n```go\n" + line1 + "\n" + line2 + "\n```\n"
	tokenCap := semantic.ProxyTokens("title: T › Code — part 9/9 | text: " + line2)
	got := semantic.ChunkNote("T", body, tokenCap)
	if len(got.Failures) != 0 {
		t.Fatalf("ChunkDocument() failures = %+v, want none", got.Failures)
	}
	if len(got.Chunks) != 2 {
		t.Fatalf("ChunkDocument() chunks = %+v, want two code-line chunks", got.Chunks)
	}
	if got.Chunks[0].Body != line1 || got.Chunks[1].Body != line2 {
		t.Errorf("code bodies = %q / %q, want line boundaries preserved", got.Chunks[0].Body, got.Chunks[1].Body)
	}
}

func TestChunkDocumentHardSplitsOneOversizeLineOnRuneBoundaries(t *testing.T) {
	t.Parallel()

	line := strings.Repeat("界", 20)
	tokenCap := semantic.ProxyTokens("title: T › H — part 99/99 | text: " + strings.Repeat("界", 4))
	got := semantic.ChunkNote("T", "## H\n\n"+line+"\n", tokenCap)
	if len(got.Failures) != 0 {
		t.Fatalf("ChunkDocument() failures = %+v, want none", got.Failures)
	}
	if len(got.Chunks) < 2 {
		t.Fatalf("ChunkDocument() returned %d chunk, want hard split", len(got.Chunks))
	}
	var joined strings.Builder
	for _, chunk := range got.Chunks {
		if !utf8.ValidString(chunk.Body) {
			t.Errorf("chunk body is not valid UTF-8: %x", chunk.Body)
		}
		if chunk.ProxyTokens > tokenCap {
			t.Errorf("chunk proxy tokens = %d, cap %d", chunk.ProxyTokens, tokenCap)
		}
		joined.WriteString(chunk.Body)
	}
	if joined.String() != line {
		t.Errorf("joined hard splits = %q, want original %q", joined.String(), line)
	}
}

func TestChunkDocumentReportsPrefixThatConsumesCap(t *testing.T) {
	t.Parallel()

	const title = "Extremely Long Title"
	const heading = "Extremely Long Heading"
	tokenCap := semantic.ProxyTokens("title: " + title + " › " + heading + " | text: ")
	got := semantic.ChunkNote(title, "## "+heading+"\n\nbody\n", tokenCap)
	if len(got.Chunks) != 0 {
		t.Errorf("ChunkDocument() chunks = %+v, want none", got.Chunks)
	}
	if len(got.Failures) != 1 || got.Failures[0].Reason != semantic.ChunkFailurePrefixConsumesCap {
		t.Fatalf("ChunkDocument() failures = %+v, want prefix-consumes-cap", got.Failures)
	}
}

func TestChunkDocumentUsesProviderNoneForAnEmptyTitle(t *testing.T) {
	t.Parallel()

	got := semantic.ChunkNote("", "body\n", 100)
	if len(got.Failures) != 0 || len(got.Chunks) != 1 {
		t.Fatalf("ChunkDocument() = %+v, want one successful chunk", got)
	}
	if got.Chunks[0].Submitted != "title: none | text: body" {
		t.Errorf("Submitted = %q, want provider's empty-title sentinel", got.Chunks[0].Submitted)
	}
}
