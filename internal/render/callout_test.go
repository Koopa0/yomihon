package render

import (
	"strings"
	"testing"
)

// TestTheCalloutVocabularyNamesEachTypeOnce replaces a check the compiler used
// to make. The vocabulary was a switch, and a type written into two case lists
// was a compile error; as a table it is a silent first-match-wins, so moving
// "summary" into the Danger group would build, vet clean, and quietly keep
// rendering it as a note. The coverage lock beside it cannot see that either,
// because it ranges over buckets rather than types.
//
// There is deliberately no assertion on how many types there are. A count is a
// number the next person bumps without reading it, and it would fail for the
// one edit that is always legitimate — adding a type Obsidian added.
func TestTheCalloutVocabularyNamesEachTypeOnce(t *testing.T) {
	t.Parallel()

	group := map[string]string{}
	for _, vocabulary := range calloutVocabulary {
		if vocabulary.title == "" {
			t.Errorf("the %q group carries no default title, so a callout written with no title of its own renders headless", vocabulary.types)
		}
		if len(vocabulary.types) == 0 {
			t.Error("a callout group names no types, so nothing can ever render as it")
		}
		for _, typ := range vocabulary.types {
			if previous, taken := group[typ]; taken {
				t.Errorf("[!%s] is named by both the %q and the %q groups; the first one wins silently, "+
					"so the second is dead and that callout renders as something its author did not ask for",
					typ, previous, vocabulary.title)
				continue
			}
			group[typ] = vocabulary.title
			// calloutStart lowercases what the author wrote before anything
			// looks it up, so an entry carrying a capital is unreachable: it
			// sits in the table and no callout can ever match it.
			if typ != strings.ToLower(typ) {
				t.Errorf("[!%s] is spelled with a capital in the vocabulary, and a callout's type is "+
					"lowercased before the lookup, so nothing will ever match this entry", typ)
			}
			if bucket, _ := calloutBucketOf(typ); bucket == bucketUnknown {
				t.Errorf("[!%s] is in the vocabulary and classifies as unknown, so it renders as a plain blockquote", typ)
			}
		}
	}
}
