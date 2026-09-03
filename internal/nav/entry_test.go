package nav

import (
	"strings"
	"testing"
)

// TestAnEntryKindNamesItself pins the words, not just the existence of a
// method: they are what a diagnostic and a rail row both carry, so an outcome
// renamed here silently renames it on a page.
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
