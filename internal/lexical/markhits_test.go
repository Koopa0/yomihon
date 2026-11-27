package lexical

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// A result that does not say why it matched leaves the reader scanning a grey
// block for the word they just typed. The runs carry slices of the reader's own
// text, so nothing is re-cased and nothing becomes markup.
func TestMarkHitsMarkWhatMatched(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		text   string
		tokens []string
		want   []HitRun
	}{
		{
			name:   "the matched stretch is marked and the rest is not",
			text:   "2026-07-30 錄音回聽",
			tokens: []string{"錄音"},
			want: []HitRun{
				{Text: "2026-07-30 "},
				{Text: "錄音", Hit: true},
				{Text: "回聽"},
			},
		},
		{
			name:   "a match found by folded case keeps the text the note wrote",
			text:   "B-tree vs GIN",
			tokens: []string{"gin"},
			want: []HitRun{
				{Text: "B-tree vs "},
				{Text: "GIN", Hit: true},
			},
		},
		{
			name:   "two tokens covering the same words produce one mark, not nested ones",
			text:   "索引 index 說明",
			tokens: []string{"索引 index", "index"},
			want: []HitRun{
				{Text: "索引 index", Hit: true},
				{Text: " 說明"},
			},
		},
		{
			name:   "a text nothing matched is left alone for the plain rendering",
			text:   "沒有相符",
			tokens: []string{"別的"},
			want:   nil,
		},
		{
			// The query's spacing is whatever the reader typed — a full-width
			// space between two words of Japanese, two spaces after a full
			// stop — and the mark has to land on the words either way.
			name:   "a phrase spaced differently in the query still marks the words",
			text:   "daily semantic retrieval log",
			tokens: []string{"semantic　retrieval"},
			want: []HitRun{
				{Text: "daily "},
				{Text: "semantic retrieval", Hit: true},
				{Text: " log"},
			},
		},
		{
			name:   "a phrase whose words are not adjacent marks nothing",
			text:   "semantic log retrieval",
			tokens: []string{"semantic retrieval"},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MarkHits(tt.text, tt.tokens)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MarkHits(%q, %v) mismatch (-want +got):\n%s", tt.text, tt.tokens, diff)
			}
			// Whatever the runs are, reassembling them must give back exactly
			// the text: a mark that drops or duplicates text is worse than no
			// mark at all.
			var rebuilt strings.Builder
			for _, r := range got {
				rebuilt.WriteString(r.Text)
			}
			if len(got) > 0 && rebuilt.String() != tt.text {
				t.Errorf("MarkHits() runs rebuild to %q, want %q", rebuilt.String(), tt.text)
			}
		})
	}
}

// MarkHits locates matches on a simple-case-folded copy of the text, and
// case folding does not preserve byte length: Ⱥ grows from two bytes to three, a
// byte that is not valid UTF-8 becomes the three-byte replacement character,
// and long s ſ shrinks from two bytes to one. An offset found on the folded
// copy therefore cannot index the original directly: a growing fold pushes a
// match past the end of the text, and a shrinking one marks the wrong bytes
// and cuts characters in half. The runs must land on the note's own bytes in
// every one of those shapes.
func TestMarkHitsSurviveAFoldThatChangesLength(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		text   string
		tokens []string
		want   []HitRun
	}{
		{
			name:   "a letter whose lowercase is longer does not push the match off the end",
			text:   "Ⱥ zap",
			tokens: []string{"zap"},
			want: []HitRun{
				{Text: "Ⱥ "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:   "a byte that is not valid UTF-8 does not push the match off the end",
			text:   "\x80 zap",
			tokens: []string{"zap"},
			want: []HitRun{
				{Text: "\x80 "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:   "a letter whose simple fold is shorter does not drag the mark onto the wrong bytes",
			text:   "ſſſſ zap",
			tokens: []string{"zap"},
			want: []HitRun{
				{Text: "ſſſſ "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:   "a match on the shrinking letter itself marks the whole character",
			text:   "ſ 筆記",
			tokens: []string{"s"},
			want: []HitRun{
				{Text: "ſ", Hit: true},
				{Text: " 筆記"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MarkHits(tt.text, tt.tokens)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MarkHits(%q, %v) mismatch (-want +got):\n%s", tt.text, tt.tokens, diff)
			}
			var rebuilt strings.Builder
			for _, r := range got {
				rebuilt.WriteString(r.Text)
			}
			if rebuilt.String() != tt.text {
				t.Errorf("MarkHits() runs rebuild to %q, want %q", rebuilt.String(), tt.text)
			}
		})
	}
}

// TestTheTwoFoldMappingsAgree holds the two shapes of one rule to the same
// answer. foldWithSourceOffsets tabulates every byte position; the landing
// walks to the positions it needs instead, because it measures a whole note.
func TestTheTwoFoldMappingsAgree(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"",
		"plain ascii",
		"Ⱥ grows and ſ shrinks",
		strings.Repeat("Ⱥſ", 8),
		"ǰ ﬁ ﬄ ǅ Ǆ",             // folds that change width in either direction
		"甲乙丙丁 mixed with ASCII", // multi-byte characters that fold to themselves
		"\xff\xfe not utf-8",    // bytes that decode as the replacement character
		"ſ",
		"Ⱥ",
		// A line break the fold drops, which moves every offset after it.
		"本品不建議用於\n兒童使用。",
		// The same break where the fold keeps it, because English parts its
		// words with the whitespace the break is made of.
		"not recommended for\nchildren",
		// A break with a character of each kind on either side: kept, because
		// only one side is a script that writes without spaces.
		"用於\nchildren and 兒童\n使用",
		// Consecutive breaks, and one at each end.
		"\n甲\n\n乙\n",
		// Fullwidth ASCII folding to a shorter encoding, the mapping the
		// width fold added: three source bytes become one folded byte.
		"３つ（たまご）Ｇｏ",
	} {
		t.Run(strconv.Quote(s), func(t *testing.T) {
			t.Parallel()
			fold, src := foldWithSourceOffsets(s)
			for k := range len(fold) + 1 {
				if got, want := sourceOffsetOfFold(s, k), src[k]; got != want {
					t.Fatalf("sourceOffsetOfFold(%q, %d) = %d, want %d (the tabulated answer)", s, k, got, want)
				}
				if k == 0 {
					continue
				}
				last := src[k-1]
				_, size := utf8.DecodeRuneInString(s[last:])
				if got, want := sourceEndOfFold(s, k), last+size; got != want {
					t.Fatalf("sourceEndOfFold(%q, %d) = %d, want %d", s, k, got, want)
				}
			}
		})
	}
}
