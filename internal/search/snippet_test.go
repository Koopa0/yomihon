package search

import (
	"strings"
	"testing"
	"unicode/utf8"

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

// Each boundary of the window steps clear of a run of letters it cannot keep
// whole — the opening one forward off the run's tail, the closing one back off
// its head. A match sitting deep inside one long run is stepped over by both at
// once, and until each was held at the match that produced an opening after the
// close: a reversed slice, and a search that took the page down instead of
// answering it. A few hundred letters with no break in them is the whole
// requirement.
func TestSnippetHoldsItsWindowAtTheMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		plain string
		token string
		want  string
	}{
		{
			// Neither boundary can reach a word edge on its own side, so both
			// come to rest on the match and the window closes to nothing. The
			// run is what the note contains; there is no readable line inside
			// it to show, and saying so beats crashing.
			name:  "letters on both sides of the match",
			plain: strings.Repeat("a", 300) + "z" + strings.Repeat("a", 300),
			token: "z",
			want:  "……",
		},
		{
			// The opening gives up the unreadable run and comes to rest on the
			// match rather than walking past it, so the match still opens the
			// line. The close needs no adjustment: the text ends first.
			name:  "readable words after the run",
			plain: strings.Repeat("a", 300) + "z is the match, and readable words follow it",
			token: "z",
			want:  "…z is the match, and readable words follow it",
		},
		{
			// The mirror image, and the case that holds the closing boundary to
			// giving the run up rather than dragging it in: it comes back to
			// where the run began, which here is the match itself, and the
			// window keeps the readable words in front of it.
			name:  "readable words before the run",
			plain: "readable words come first and then z" + strings.Repeat("a", 300),
			token: "z",
			want:  "readable words come first and then…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippet(tt.plain, fold(tt.plain), []string{fold(tt.token)})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("snippet() mismatch (-want +got):\n%s", diff)
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

// The snippet window was a byte budget, and a byte budget pays out by script:
// a Han character costs three bytes, so the same numbers bought a reader of
// Chinese about a third of the context they bought a reader of English. A
// diarist searching three years of her own writing hit it exactly where it
// hurts — the long entries, which are the ones a year-end reread is for.
func TestSnippetGivesEveryScriptTheSameWindow(t *testing.T) {
	t.Parallel()

	const needle = "母親"
	chinese := strings.Repeat("今天天氣很好我出門散步", 30) + needle + strings.Repeat("後來下雨了我就回家了", 30)
	english := strings.Repeat("the weather was fine so I went for a walk ", 8) + "mother" + strings.Repeat(" and then it rained and I went home", 8)

	idx := NewIndex([]Document{
		{RelPath: "a.md", Title: "中文", PlainText: chinese},
		{RelPath: "b.md", Title: "English", PlainText: english},
	}, validArtifactPolicy(t))

	zh := snippetFor(t, idx, needle)
	en := snippetFor(t, idx, "mother")

	// Both windows are measured in what the reader sees. Whole-word expansion
	// and the ellipses move the exact count a little, so the lock is on the
	// ratio: one script must not get a fraction of the other's context.
	zhRunes := len([]rune(zh))
	enRunes := len([]rune(en))
	if zhRunes*2 < enRunes {
		t.Errorf("the Chinese snippet is %d characters against English's %d — the budget still pays out by script\nzh: %q\nen: %q", zhRunes, enRunes, zh, en)
	}
	// And it must still be a window, not the whole note.
	if zhRunes > 260 {
		t.Errorf("the Chinese snippet is %d characters; the window stopped bounding anything", zhRunes)
	}
}

func snippetFor(t *testing.T, idx *Index, query string) string {
	t.Helper()
	results, err := idx.Search(Parse(query))
	if err != nil {
		t.Fatalf("Search(%q) error = %v", query, err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(%q) returned %d results, want exactly 1 to measure", query, len(results))
	}
	return results[0].Snippet
}

// The matched offset is found on a folded copy and used against the original.
// A fold that is not length-preserving can land it inside a character, and
// counting characters outward from a broken start would carry the break into
// what the reader sees. Turkish İ folds to two bytes' worth of nothing like
// itself, which is the shape of that failure.
func TestSnippetSurvivesAFoldThatMovesABoundary(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		// The İ folds one byte shorter, so every offset found after it points
		// one byte early in the original. The character before the match is
		// multi-byte on purpose: with an ASCII space there, landing a byte
		// early still lands on a boundary and the failure hides.
		// The İ folds one byte shorter, so every offset found after it points a
		// byte early in the original — inside a character, since what follows
		// is Han. The match sits far enough in that subtracting a byte budget
		// from that offset lands mid-character too; nearer the start the
		// subtraction clamps to zero and the break has nowhere to show.
		{RelPath: "a.md", Title: "t", PlainText: "İ" + strings.Repeat("今天出門散步看見一隻貓在牆上睡覺，", 6) + "天氣很好，回家以後泡了一壺茶。"},
	}, validArtifactPolicy(t))

	got := snippetFor(t, idx, "天氣")
	if !utf8.ValidString(got) {
		t.Errorf("Snippet = %q, which is not valid UTF-8", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Errorf("Snippet = %q, which carries a replacement character — a boundary broke", got)
	}
}
