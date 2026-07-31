package search

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

// A snippet window lands wherever the byte count puts it, and where that was
// inside a word the reader got a fragment that reads as a typo: a practice log
// opened "…026-07-30", a database note severed B-tree into "…ee vs GIN". CJK
// prose has no word boundary and is expected to be cut between characters; runs
// of letters and digits are not.
func TestSnippetDoesNotOpenOrCloseInsideAWord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		plain     string
		token     string
		forbidden string
		wantWhole string
	}{
		{
			// Sized so the window opens one byte into the year: eleven bytes of
			// date plus thirty of filler puts the match forty-one bytes in.
			name:      "a year keeps all four digits",
			plain:     "2026-07-30 甲乙丙丁戊己庚辛壬癸錄音回聽",
			token:     "錄音",
			forbidden: "026-07-30",
			wantWhole: "2026-07-30",
		},
		{
			// Same arithmetic, opening two bytes into B-tree.
			name:      "a hyphenated term is not severed at the window edge",
			plain:     "B-tree 甲乙丙丁戊己庚辛壬癸甲乙GIN 的取捨",
			token:     "gin",
			forbidden: "ree 甲",
			wantWhole: "B-tree",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippet(tt.plain, strings.ToLower(tt.plain), []string{tt.token})
			// The defect's shape is the opening: an ellipsis followed straight
			// by the tail of a word. Asking merely whether the fragment occurs
			// anywhere cannot fail, since the whole word contains it.
			if strings.HasPrefix(got, "…"+tt.forbidden) {
				t.Errorf("snippet() opened inside a word: %q begins with the tail %q", got, tt.forbidden)
			}
			if !strings.Contains(got, tt.wantWhole) {
				t.Errorf("snippet() = %q, want it to contain the whole %q", got, tt.wantWhole)
			}
		})
	}
}

// A result that does not say why it matched leaves the reader scanning a grey
// block for the word they just typed. The runs carry slices of the reader's own
// text, so nothing is re-cased and nothing becomes markup.
func TestSnippetRunsMarkWhatMatched(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		snippet string
		tokens  []string
		want    []pages.SnippetRun
	}{
		{
			name:    "the matched stretch is marked and the rest is not",
			snippet: "2026-07-30 錄音回聽",
			tokens:  []string{"錄音"},
			want: []pages.SnippetRun{
				{Text: "2026-07-30 "},
				{Text: "錄音", Hit: true},
				{Text: "回聽"},
			},
		},
		{
			name:    "a match found by folded case keeps the text the note wrote",
			snippet: "B-tree vs GIN",
			tokens:  []string{"gin"},
			want: []pages.SnippetRun{
				{Text: "B-tree vs "},
				{Text: "GIN", Hit: true},
			},
		},
		{
			name:    "two tokens covering the same words produce one mark, not nested ones",
			snippet: "索引 index 說明",
			tokens:  []string{"索引 index", "index"},
			want: []pages.SnippetRun{
				{Text: "索引 index", Hit: true},
				{Text: " 說明"},
			},
		},
		{
			name:    "a snippet nothing matched is left alone for the plain rendering",
			snippet: "沒有相符",
			tokens:  []string{"別的"},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := markHits(tt.snippet, tt.tokens)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("markHits(%q, %v) mismatch (-want +got):\n%s", tt.snippet, tt.tokens, diff)
			}
			// Whatever the runs are, reassembling them must give back exactly
			// the snippet: a mark that drops or duplicates text is worse than
			// no mark at all.
			var rebuilt strings.Builder
			for _, r := range got {
				rebuilt.WriteString(r.Text)
			}
			if len(got) > 0 && rebuilt.String() != tt.snippet {
				t.Errorf("markHits() runs rebuild to %q, want %q", rebuilt.String(), tt.snippet)
			}
		})
	}
}
