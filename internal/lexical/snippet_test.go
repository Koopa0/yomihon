package lexical

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
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
		want    []HitRun
	}{
		{
			name:    "the matched stretch is marked and the rest is not",
			snippet: "2026-07-30 錄音回聽",
			tokens:  []string{"錄音"},
			want: []HitRun{
				{Text: "2026-07-30 "},
				{Text: "錄音", Hit: true},
				{Text: "回聽"},
			},
		},
		{
			name:    "a match found by folded case keeps the text the note wrote",
			snippet: "B-tree vs GIN",
			tokens:  []string{"gin"},
			want: []HitRun{
				{Text: "B-tree vs "},
				{Text: "GIN", Hit: true},
			},
		},
		{
			name:    "two tokens covering the same words produce one mark, not nested ones",
			snippet: "索引 index 說明",
			tokens:  []string{"索引 index", "index"},
			want: []HitRun{
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
		{
			// A snippet is one line, so whatever whitespace stood between the
			// phrase's words in the note arrives here as a single space. The
			// query's spacing is whatever the reader typed — a full-width space
			// between two words of Japanese, two spaces after a full stop — and
			// the mark has to land on the words either way.
			name:    "a phrase spaced differently in the query still marks the words",
			snippet: "daily semantic retrieval log",
			tokens:  []string{"semantic　retrieval"},
			want: []HitRun{
				{Text: "daily "},
				{Text: "semantic retrieval", Hit: true},
				{Text: " log"},
			},
		},
		{
			name:    "a phrase whose words are not adjacent marks nothing",
			snippet: "semantic log retrieval",
			tokens:  []string{"semantic retrieval"},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MarkHits(tt.snippet, tt.tokens)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MarkHits(%q, %v) mismatch (-want +got):\n%s", tt.snippet, tt.tokens, diff)
			}
			// Whatever the runs are, reassembling them must give back exactly
			// the snippet: a mark that drops or duplicates text is worse than
			// no mark at all.
			var rebuilt strings.Builder
			for _, r := range got {
				rebuilt.WriteString(r.Text)
			}
			if len(got) > 0 && rebuilt.String() != tt.snippet {
				t.Errorf("MarkHits() runs rebuild to %q, want %q", rebuilt.String(), tt.snippet)
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

// MarkHits locates matches on a lowercased copy of the snippet, and
// lowercasing does not preserve length: Ⱥ grows from two bytes to three, a
// byte that is not valid UTF-8 becomes the three-byte replacement character,
// and Turkish İ shrinks from two bytes to one. An offset found on the folded
// copy therefore cannot index the original directly — growing folds pushed a
// match past the end of the snippet and the page panicked; shrinking folds
// marked the wrong bytes and cut characters in half. The runs must land on
// the note's own bytes in every one of those shapes.
func TestMarkHitsSurviveAFoldThatChangesLength(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		snippet string
		tokens  []string
		want    []HitRun
	}{
		{
			name:    "a letter whose lowercase is longer does not push the match off the end",
			snippet: "Ⱥ zap",
			tokens:  []string{"zap"},
			want: []HitRun{
				{Text: "Ⱥ "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:    "a byte that is not valid UTF-8 does not push the match off the end",
			snippet: "\x80 zap",
			tokens:  []string{"zap"},
			want: []HitRun{
				{Text: "\x80 "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:    "a letter whose lowercase is shorter does not drag the mark onto the wrong bytes",
			snippet: "İİİİ zap",
			tokens:  []string{"zap"},
			want: []HitRun{
				{Text: "İİİİ "},
				{Text: "zap", Hit: true},
			},
		},
		{
			name:    "a match on the shrinking letter itself marks the whole character",
			snippet: "İ 筆記",
			tokens:  []string{"i"},
			want: []HitRun{
				{Text: "İ", Hit: true},
				{Text: " 筆記"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MarkHits(tt.snippet, tt.tokens)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MarkHits(%q, %v) mismatch (-want +got):\n%s", tt.snippet, tt.tokens, diff)
			}
			var rebuilt strings.Builder
			for _, r := range got {
				rebuilt.WriteString(r.Text)
			}
			if rebuilt.String() != tt.snippet {
				t.Errorf("MarkHits() runs rebuild to %q, want %q", rebuilt.String(), tt.snippet)
			}
		})
	}
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

// TestSnippetKeepsTheWordsThatGovernTheMatch holds the one truncation a
// reader cannot recover from. A window placed by byte count opens wherever it
// lands, and where the sentence began with the word that reverses it —
// 不得, "not recommended", a leading 本段僅限 — the reader is shown the
// predicate on its own and reads it as the instruction. The leading ellipsis
// says bytes were cut; it does not say the cut ones inverted the sentence, and
// nobody scanning a result list can tell the difference.
//
// Each case puts the governing words further from the match than the plain
// window reaches, which is the only arrangement that can fail: a qualifier
// inside the window was never at risk. The window therefore opens at the start
// of the sentence it landed in, when that start is within reach.
func TestSnippetKeepsTheWordsThatGovernTheMatch(t *testing.T) {
	t.Parallel()
	// Longer than snippetBefore, so the plain window opens after the words
	// under test rather than before them.
	longClause := strings.Repeat("在某些特定情況下", 8)
	longEnglish := strings.Repeat("under a number of particular circumstances, ", 2)
	tests := []struct {
		name    string
		plain   string
		query   string
		wantHas string
	}{
		{
			name:    "a negation before the match survives",
			plain:   "前面一段無關的敘述。不得" + longClause + "在正式環境使用這個設定。",
			query:   "正式環境",
			wantHas: "不得",
		},
		{
			name:    "an english qualifier before the match survives",
			plain:   "An earlier sentence sits here. This is not recommended " + longEnglish + "for production clusters.",
			query:   "production",
			wantHas: "not recommended",
		},
		{
			name:    "a scope limiter before the match survives",
			plain:   "前面一段無關的敘述。本段僅限" + longClause + "舊版設定使用。",
			query:   "舊版設定",
			wantHas: "本段僅限",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippet(tt.plain, strings.ToLower(tt.plain), []string{strings.ToLower(tt.query)})
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("snippet() = %q, dropped the words that govern the match (%q)", got, tt.wantHas)
			}
			// The control: the sentence before it is not dragged in as well.
			if strings.Contains(got, "前面一段無關") || strings.Contains(got, "An earlier sentence") {
				t.Errorf("snippet() = %q, reached back past the sentence the match is in", got)
			}
		})
	}
}

// TestSnippetWillNotWalkBackThroughAWholeParagraph is the ceiling on the rule
// above, and it needs a sentence break that is genuinely out of reach — not an
// absent one. With no terminator anywhere the boundary cannot move whatever
// the ceiling says, so a page of unbroken filler tests nothing: the first
// version of this check passed with the ceiling raised to a hundred thousand.
// The break here sits hundreds of runes back, so only the ceiling stops the
// window opening there and dragging the paragraph into one result row.
func TestSnippetWillNotWalkBackThroughAWholeParagraph(t *testing.T) {
	t.Parallel()
	plain := "開頭的一句話。" + strings.Repeat("甲乙丙丁戊己庚辛壬癸", 40) + "目標詞在這裡"
	got := snippet(plain, strings.ToLower(plain), []string{"目標詞"})
	if strings.Contains(got, "開頭的一句話") {
		t.Errorf("snippet() reached back past the ceiling to a sentence hundreds of runes away:\n%s", got)
	}
	if n := len([]rune(got)); n > 320 {
		t.Errorf("snippet() ran to %d runes; the opening side is unbounded", n)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("snippet() = %q, want it to still open with an ellipsis", got)
	}
}

// TestSnippetDoesNotTreatADecimalPointAsTheEndOfASentence holds the reach-back
// off the punctuation that is not punctuation. A full stop ends a sentence
// only when something follows it that is not more of the same word: inside
// 3.14159 or vault-schema.toml it is part of the token, and a reach that
// stopped there opened the window in the middle of a number and presented the
// tail of it as the start of a sentence — worse than the plain byte window it
// replaced, which at least stepped clear of the run.
func TestSnippetDoesNotTreatADecimalPointAsTheEndOfASentence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		plain     string
		query     string
		forbidden string
		wantHas   string
	}{
		{
			name:      "a decimal point is not a sentence end",
			plain:     "An earlier sentence. Some filler words that pad this out a little 3.14159265358979 TARGETWORD tail",
			query:     "targetword",
			forbidden: "…14159265358979",
			wantHas:   "Some filler words",
		},
		{
			name:      "a dot inside a filename is not a sentence end",
			plain:     "An earlier sentence. Some filler words that pad this out a little vault-schema.toml TARGETWORD tail",
			query:     "targetword",
			forbidden: "…toml",
			wantHas:   "Some filler words",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippet(tt.plain, strings.ToLower(tt.plain), []string{tt.query})
			if strings.HasPrefix(got, tt.forbidden) {
				t.Errorf("snippet() opened inside a token at a dot that ends no sentence: %q", got)
			}
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("snippet() = %q, want it to reach back to the real sentence start (%q)", got, tt.wantHas)
			}
		})
	}
}

// TestADotInsideATokenEndsNoSentence holds the reach-back off the punctuation
// that only looks like punctuation. The scan takes the LAST terminator before
// the window, so a full stop inside vault-schema.toml or go1.26.4 beat the
// real 。 further back — and the words that govern the sentence, which this
// reach exists to keep, were dropped on exactly the sentences this vault's
// technical prose is full of. The window then opened inside the token: "…4 這
// 個版本" presents the tail of a version number as the start of a thought.
//
// A full-width terminator is never inside a token and is not followed by a
// space in Chinese, so it counts wherever it stands; an ASCII one counts only
// where something other than more of the same word follows it.
func TestADotInsideATokenEndsNoSentence(t *testing.T) {
	t.Parallel()
	filler := strings.Repeat("接著這一段說明文字", 6)
	tests := []struct {
		name  string
		plain string
	}{
		{name: "a filename", plain: "前面一句。不得使用 vault-schema.toml 這個檔案" + filler + "目標詞在這裡"},
		{name: "a version number", plain: "前面一句。不得升級到 go1.26.4 這個版本" + filler + "目標詞在這裡"},
		{name: "a decimal", plain: "前面一句。不得超過 3.14159265358979 這個數字" + filler + "目標詞在這裡"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := snippet(tt.plain, strings.ToLower(tt.plain), []string{"目標詞"})
			if !strings.Contains(got, "不得") {
				t.Errorf("snippet() = %q, dropped the word that governs the sentence because a dot inside a token beat the real sentence end", got)
			}
		})
	}

	// The control: a sentence with no token-internal dot already worked, so
	// the cases above must fail for the reason named and not because the
	// reach-back is broken outright.
	clean := "前面一句。不得使用這個設定檔" + filler + "目標詞在這裡"
	if got := snippet(clean, strings.ToLower(clean), []string{"目標詞"}); !strings.Contains(got, "不得") {
		t.Errorf("the control lost 不得 too; the fixture proves nothing about dots: %q", got)
	}
}

// TestSnippetHoldsTheMatchWhenTheFoldChangesLength holds the window on the
// hit rather than merely on a valid boundary.
//
// The offset is found on the folded copy and lowercasing does not preserve
// length: Ⱥ grows from two bytes to three and İ shrinks from two to one. Used
// as an index into the note's own bytes, that offset drifts one byte per such
// character, and once the drift exceeds the window's own radius the snippet
// slides clear of the term the reader searched for — a result whose evidence
// shows none of what matched. The sibling test beside this one asks only that
// the snippet stay valid UTF-8, which a window that has slid off the match
// still is.
func TestSnippetHoldsTheMatchWhenTheFoldChangesLength(t *testing.T) {
	t.Parallel()

	const (
		grows   = "Ⱥ" // Ⱥ, three bytes once lowercased
		shrinks = "İ" // İ, one byte once lowercased
	)
	tail := strings.Repeat("x", 400)

	for _, tc := range []struct {
		name  string
		plain string
	}{
		{
			// Enough of them that the offset lands past the window's reach
			// back: the window would open beyond the term and never show it.
			name:  "a fold that grows pushes the window past the match",
			plain: strings.Repeat(grows, 120) + "needle " + tail,
		},
		{
			// The mirror: the offset points short of the term by more than the
			// window reaches forward.
			name:  "a fold that shrinks leaves the window short of the match",
			plain: strings.Repeat(shrinks, 400) + "needle " + tail,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{RelPath: "a.md", Title: "t", PlainText: tc.plain}}, validArtifactPolicy(t))

			got := snippetFor(t, idx, "needle")
			if !strings.Contains(got, "needle") {
				t.Errorf("Snippet = %q, want the window to hold the term that matched", got)
			}
			if !utf8.ValidString(got) {
				t.Errorf("Snippet = %q, which is not valid UTF-8", got)
			}
			var marked bool
			for _, run := range MarkHits(got, []string{"needle"}) {
				if run.Hit && strings.Contains(run.Text, "needle") {
					marked = true
				}
			}
			if !marked {
				t.Errorf("MarkHits(%q) marked no run holding the term", got)
			}
		})
	}
}

// TestTheTwoFoldMappingsAgree holds the two shapes of one rule to the same
// answer. foldWithSourceOffsets tabulates every byte position; the snippet
// path walks to one position instead, because it measures a whole note. A rule
// written twice is a rule that drifts, and this repository has already paid
// for that once: two byte-identical copies of a directory check, only one of
// them ever corrected.
func TestTheTwoFoldMappingsAgree(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"",
		"plain ascii",
		"Ⱥ grows and İ shrinks",
		strings.Repeat("Ⱥİ", 8),
		"ǰ ﬁ ﬄ ǅ Ǆ",             // folds that change width in either direction
		"甲乙丙丁 mixed with ASCII", // multi-byte characters that fold to themselves
		"\xff\xfe not utf-8",    // bytes that decode as the replacement character
		"İ",
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
	} {
		t.Run(strconv.Quote(s), func(t *testing.T) {
			t.Parallel()
			fold, src := foldWithSourceOffsets(s)
			for k := range len(fold) + 1 {
				if got, want := sourceOffsetOfFold(s, k), src[k]; got != want {
					t.Fatalf("sourceOffsetOfFold(%q, %d) = %d, want %d (the tabulated answer)", s, k, got, want)
				}
			}
		})
	}
}
