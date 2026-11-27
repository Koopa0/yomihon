package lexical

import "testing"

// TestEveryFilterKeyTheGrammarAcceptsIsAnswered ties the grammar to the code
// that answers it, which is written out one key at a time and falls through to
// "no" for a key it does not recognize. A key added to the grammar would parse,
// be offered on the page, and then quietly match nothing at all.
//
// The rows are this test's own, so adding a key to the grammar without adding
// the row and the case fails here rather than in a reader's empty result.
func TestEveryFilterKeyTheGrammarAcceptsIsAnswered(t *testing.T) {
	t.Parallel()

	e := &entry{
		relPath:      "Writing/lessons/one.md",
		pathFold:     Fold("Writing/lessons/one.md"),
		noteType:     "lesson",
		noteTypeFold: Fold("lesson"),
		draft:        true,
		domain:       "japanese",
		domainFold:   Fold("japanese"),
		slug:         "one",
		slugFold:     Fold("one"),
	}
	satisfied := map[string]string{
		"type":   "lesson",
		"draft":  "true",
		"domain": "japanese",
		"slug":   "one",
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
		if !e.matchesFilter(Filter{Key: key, Value: Fold(value)}) {
			t.Errorf("matchesFilter(%q:%q) = false on an entry that carries it; the key parses and nothing answers it", key, value)
		}
		if e.matchesFilter(Filter{Key: key, Value: Fold(value + "-not-this")}) {
			t.Errorf("matchesFilter(%q) answered true for a value this entry does not carry", key)
		}
	}
}
