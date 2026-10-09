package lexical

import "testing"

func TestWithFilterDoesNotOfferAConstraintReadAsAnotherValue(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`"east"`, "「east」", "『east』"} {
		for _, key := range []string{"domain", "folder"} {
			query, offered := WithFilter("needle", Filter{Key: key, Value: value})
			if !offered {
				continue
			}
			got := Parse(query).Filters()
			if len(got) != 1 || got[0].Key != key || !filterValuesEqual(key, got[0].Value, value) {
				t.Errorf("offered %q for %s:%q, but it reads as %v", query, key, value, got)
			}
		}
	}
	if _, offered := WithFilter("needle", Filter{Key: "domain", Value: "field notes"}); !offered {
		t.Error("ordinary space-bearing value lost its valid offer")
	}
}
