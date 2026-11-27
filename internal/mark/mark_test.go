package mark_test

import (
	"errors"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// A marks.json whose mark carries a word the machine files have no place for is
// not one the earlier version wrote: it wrote a reason only on a question and a
// body only on another kind. It is damaged, left as it is and reported, and no
// word in it is dropped by a conversion.
func TestAMarkTheEarlierVersionCouldNotHaveWrittenIsUnreadable(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, mark string }{
		{"a question with a body", `{"id":"` + markID(1) + `","kind":"question","path":"Notes/a.md","body":"an answer","at":"2026-09-22T10:00:00Z"}`},
		{"a highlight with a reason", `{"id":"` + markID(1) + `","kind":"highlight","path":"Notes/a.md","quote":"q","reason":"why","at":"2026-09-22T10:00:00Z"}`},
		{"a thought with a reason", `{"id":"` + markID(1) + `","kind":"thought","path":"Notes/a.md","body":"x","reason":"y","at":"2026-09-22T10:00:00Z"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			written := aMarksFile(tt.mark)
			writeFile(t, marksPath(store), written)
			if held, err := store.MachineMarks(); !errors.Is(err, mark.ErrUnreadable) {
				t.Fatalf("MachineMarks over %s = %v, %v; want ErrUnreadable", tt.name, held, err)
			}
			if contents(t, marksPath(store)) != written {
				t.Error("the file was changed")
			}
		})
	}
}
