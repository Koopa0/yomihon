package mark_test

import (
	"errors"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// anIdentity is a content identity in the one spelling a reading page stamps.
const anIdentity = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func aPlace() *mark.Bookmark {
	return &mark.Bookmark{
		RelPath:  "Writing/lessons/japanese/L01.md",
		Anchor:   "the-topic-particle",
		Offset:   420,
		Identity: anIdentity,
		At:       time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
	}
}

// aQuestion is a question about a whole section.
func aQuestion() *machinemarks.Mark {
	return &machinemarks.Mark{
		Kind:   machinemarks.Question,
		Path:   "Notes/source.md",
		Anchor: "section-one",
		At:     time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// aHighlight is a highlight of a span of text, with what stood around it.
func aHighlight() *machinemarks.Mark {
	return &machinemarks.Mark{
		Kind:     machinemarks.Highlight,
		Path:     "Notes/source.md",
		Anchor:   "section-one",
		Quote:    "the topic particle",
		Before:   "In Japanese, ",
		After:    " marks what the sentence is about",
		Identity: anIdentity,
		At:       time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC),
	}
}

func newStore(t *testing.T) *mark.Store {
	t.Helper()
	return newStoreFor(t, t.TempDir(), "/vaults/notes")
}

func newStoreFor(t *testing.T, configDir, vaultRoot string) *mark.Store {
	t.Helper()
	store, err := mark.Open(confdir.At(configDir), vaultRoot)
	if err != nil {
		t.Fatalf("mark.Open(%q, %q): %v", configDir, vaultRoot, err)
	}
	return store
}

// heldBookmark is the bookmark for reading outside any course, failing the test
// when the file cannot be read.
func heldBookmark(t *testing.T, store *mark.Store) (mark.Bookmark, bool) {
	t.Helper()
	held, found, err := store.Bookmark(mark.NoCourse)
	if err != nil {
		t.Fatalf("Bookmark: %v", err)
	}
	return held, found
}

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

// moveBookmark sets the bookmark of a course and asks nothing of what it
// replaced, for the tests that are about something else.
func moveBookmark(store *mark.Store, course string, b *mark.Bookmark) error {
	_, _, err := store.MoveBookmark(course, b, nil)
	return err
}
