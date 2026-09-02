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
