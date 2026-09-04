package nav

import (
	"strings"
	"testing"
)

// TestAnEntryKindNamesItself pins the words, not just the existence of a
// method: they are what a diagnostic and a rail row both carry, so renaming an
// outcome here renames it on every page that draws one. The pages say so
// themselves — they ask this package for the word instead of keeping a copy, so
// a rename that reaches them without reaching their expectations turns them red
// rather than shipping two spellings of the same outcome.
func TestAnEntryKindNamesItself(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind EntryKind
		want string
	}{
		{EntryResolved, "resolved"},
		{EntryUnresolved, "unresolved"},
		{EntryAmbiguous, "ambiguous"},
		{EntryNonInstance, "non-instance"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := tc.kind.String(); got != tc.want {
				t.Errorf("EntryKind.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAnEntryKindRefusesToNameAValueItDoesNotDeclare holds the answer for the
// member somebody adds to this block later: a name invented for it would report
// a resolution that never happened, so the value itself is all there is to say.
func TestAnEntryKindRefusesToNameAValueItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		text, isText := recovered.(string)
		if !isText || !strings.Contains(text, "200") {
			t.Errorf("panic = %v, want a message naming the value 200", recovered)
		}
	}()
	_ = EntryKind(200).String()
	t.Error("EntryKind(200).String() returned instead of panicking")
}

// TestAnUnnamedEntryKindCanStillBeAskedAbout holds the half of the naming a
// page depends on. A rail draws every row it was given, so the surface asking
// what an outcome is called cannot be the one that stops the request: asking
// for the token reports that the value is unclassified, and the row carries the
// number instead of taking the page down with it.
func TestAnUnnamedEntryKindCanStillBeAskedAbout(t *testing.T) {
	t.Parallel()

	token, known := EntryKind(200).Token()
	if known {
		t.Errorf("Token() called EntryKind(200) one of ours and named it %q", token)
	}
	if token != "" {
		t.Errorf("Token() = %q for a kind nothing classified; an outcome nobody reached has no name to carry", token)
	}
}
