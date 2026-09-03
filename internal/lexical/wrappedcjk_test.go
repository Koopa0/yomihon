package lexical

import (
	"strings"
	"testing"
)

// TestAPhraseFoundAcrossAWrappedLine covers the sentence a reader can see and
// cannot find. Prose written without spaces between its words carries no
// separator for a line break to stand in for, so a phrase split across one is
// split in the index too — while the same sentence in English is found,
// because its words were already parted by spaces and a break is just more
// whitespace.
func TestAPhraseFoundAcrossAWrappedLine(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "Notes/Dose.md", Title: "Dose", PlainText: "本品不建議用於\n兒童使用。"},
		{RelPath: "Notes/En.md", Title: "En", PlainText: "this is not recommended for\nchildren at all"},
		// A wrap with one script on each side. Only one of them writes
		// without spaces, so the break is still a separator and joining
		// across it would invent a word.
		{RelPath: "Notes/Mixed.md", Title: "Mixed", PlainText: "用於\nchildren 與\n兒童"},
	}, validArtifactPolicy(t))

	for _, c := range []struct {
		name  string
		query string
		want  int
	}{
		{"a phrase split by the wrap", "不建議用於兒童", 1},
		{"either half on its own", "不建議用於", 1},
		{"the English phrase, unchanged", `"not recommended for children"`, 1},
		// The break stood between two words that a space already parted. Were
		// it dropped rather than kept as whitespace, the two would fuse into a
		// word nobody wrote, and a search for that word would answer.
		{"English words are not fused by the wrap", "forchildren", 0},
		// The join needs both sides, not just the one before the break: a
		// character that writes without spaces standing before a word that
		// does not is still parted from it.
		{"a wrap between two different scripts is left alone", "用於children", 0},
		// And the same wrap where both sides do write without spaces is closed.
		{"a wrap between two spaceless characters is closed", "與兒童", 1},
		// Nothing may be found that is not in the text: the join closes a gap
		// between two characters, it does not reorder them.
		{"characters are not resequenced", "兒童不建議", 0},
	} {
		results, _, err := idx.SearchN(Parse(c.query), -1)
		if err != nil {
			t.Fatalf("%s: Search: %v", c.name, err)
		}
		if len(results) != c.want {
			t.Errorf("%s: query %q returned %d results, want %d", c.name, c.query, len(results), c.want)
		}
	}
}

// TestTheJoinLeavesTheSnippetPointingAtTheRightCharacters is the half a fold
// change can break silently. The marks a reader sees are cut in the stored
// text using offsets found in the folded copy, so a fold that drops a byte
// moves every offset after it: the words would still be found and the
// highlight would land somewhere else.
func TestTheJoinLeavesTheSnippetPointingAtTheRightCharacters(t *testing.T) {
	t.Parallel()

	const body = "本品不建議用於\n兒童使用。"
	runs := MarkHits(body, Parse("不建議用於兒童").Tokens())
	if len(runs) == 0 {
		t.Fatal("the phrase was not marked at all, so nothing below checks where the marks landed")
	}
	var marked strings.Builder
	for _, r := range runs {
		if r.Hit {
			marked.WriteString(r.Text)
		}
	}
	// The marked stretch is the phrase as the file holds it — the wrap
	// included, because that is what those characters are on disk.
	if got := marked.String(); got != "不建議用於\n兒童" {
		t.Errorf("the marks cover %q, which is not where the phrase sits in the text", got)
	}
}
