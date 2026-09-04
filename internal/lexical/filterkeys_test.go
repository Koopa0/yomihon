package lexical

import (
	"strings"
	"testing"
)

// TestTheFilterKeysOfferedAreTheOnesTheGrammarAccepts holds the two halves
// together. A list written for the page is a list that drifts from the parser,
// and this repository has already seen that happen: the batch ledger describing
// this work names four filter keys where the grammar accepts six.
func TestTheFilterKeysOfferedAreTheOnesTheGrammarAccepts(t *testing.T) {
	t.Parallel()

	offered := FilterKeys()
	if len(offered) == 0 {
		t.Fatal("no filter keys are offered at all, so nothing below checks anything")
	}
	for _, key := range offered {
		if !isFilterKey(key) {
			t.Errorf("the page offers %q, which the parser does not accept", key)
		}
	}
	for _, key := range []string{"type", "status", "domain", "topic", "slug", "folder"} {
		if !isFilterKey(key) {
			t.Errorf("%q stopped being a filter key; this test's own list is stale", key)
		}
		if !strings.Contains(strings.Join(offered, " "), key) {
			t.Errorf("the parser accepts %q and the page never offers it", key)
		}
	}
}

// TestEveryFilterKeyTheGrammarAcceptsIsAnswered holds the third side of the
// same triangle. The sibling above ties the keys the page offers to the keys
// the parser accepts; nothing tied either to the code that answers them, which
// is written out one key at a time and falls through to "no" for a key it does
// not recognize. A seventh key added to the grammar would parse, be offered on
// the page, and then quietly match nothing at all.
//
// The rows are this test's own, so adding a key to the grammar without adding
// the row and the case fails here rather than in a reader's empty result.
func TestEveryFilterKeyTheGrammarAcceptsIsAnswered(t *testing.T) {
	t.Parallel()

	e := &entry{
		RelPath:  "Writing/lessons/one.md",
		NoteType: "lesson",
		Status:   "ready",
		Domain:   "japanese",
		Slug:     "one",
		Topics:   []string{"grammar", "kanji"},
	}
	satisfied := map[string]string{
		"type":   "lesson",
		"status": "ready",
		"domain": "japanese",
		"slug":   "one",
		"topic":  "kanji",
		"folder": "Writing/lessons",
	}
	if len(filterKeys) == 0 {
		t.Fatal("the grammar accepts no filter keys at all, so the loop below checks nothing")
	}
	for key := range filterKeys {
		value, ok := satisfied[key]
		if !ok {
			t.Errorf("the grammar accepts %q and nothing here satisfies it; a key needs a case in matchesFilter and a row here", key)
			continue
		}
		if !e.matchesFilter(Filter{Key: key, Value: value}) {
			t.Errorf("matchesFilter(%q:%q) = false on an entry that carries it; the key parses and nothing answers it", key, value)
		}
		if e.matchesFilter(Filter{Key: key, Value: value + "-not-this"}) {
			t.Errorf("matchesFilter(%q) answered true for a value this entry does not carry", key)
		}
	}
}
