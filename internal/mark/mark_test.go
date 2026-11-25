package mark_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
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
func aQuestion() *mark.Mark {
	return &mark.Mark{
		Kind:    mark.Question,
		RelPath: "Notes/source.md",
		Anchor:  "section-one",
		At:      time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// aHighlight is a highlight of a span of text, with what stood around it.
func aHighlight() *mark.Mark {
	return &mark.Mark{
		Kind:     mark.Highlight,
		RelPath:  "Notes/source.md",
		Anchor:   "section-one",
		Quote:    "the topic particle",
		Before:   "In Japanese, ",
		After:    " marks what the sentence is about",
		Identity: anIdentity,
		At:       time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC),
	}
}

func admitAny(*mark.Mark) bool { return true }

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

// TestAStoreWithNothingNamingItIsRefused holds the one thing the constructor
// checks. No configuration directory builds a relative path, which would
// put the reader's marks beside whatever folder the process started in and look
// like it had worked.
func TestAStoreWithNothingNamingItIsRefused(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		configDir confdir.ConfigDir
		vaultRoot string
	}{
		{"no configuration directory", confdir.ConfigDir{}, "/vaults/notes"},
		{"no vault root", confdir.At(t.TempDir()), ""},
		{"neither", confdir.ConfigDir{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, err := mark.Open(tt.configDir, tt.vaultRoot)
			if err == nil {
				t.Fatalf("mark.Open(%v, %q) built a store at %s", tt.configDir, tt.vaultRoot, store.Dir())
			}
			if !errors.Is(err, mark.ErrNotNamed) {
				t.Errorf("mark.Open(%v, %q) = %v, want it to be ErrNotNamed", tt.configDir, tt.vaultRoot, err)
			}
		})
	}
}

// TestAMarkIsRefusedWhenItIsNotAShapeTheFilesHold: what reaches a store came in
// over a route anyone on this machine can post to, and a value kept unchecked
// is handed back to a later page as though a reader had chosen it.
func TestAMarkIsRefusedWhenItIsNotAShapeTheFilesHold(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		spoil func(*mark.Mark)
	}{
		{"a kind that is none", func(m *mark.Mark) { m.Kind = "bookmark" }},
		{"no kind", func(m *mark.Mark) { m.Kind = "" }},
		{"a highlight of nothing", func(m *mark.Mark) { *m = *aHighlight(); m.Quote = ""; m.Before = ""; m.After = "" }},
		{"a highlight that says something", func(m *mark.Mark) { *m = *aHighlight(); m.Body = "I think so" }},
		{"a highlight with a reason", func(m *mark.Mark) { *m = *aHighlight(); m.Reason = "why" }},
		{"a question that says something", func(m *mark.Mark) { m.Body = "an answer" }},
		{"a thought that says nothing", func(m *mark.Mark) { m.Kind = mark.Thought }},
		{"a thought with a reason", func(m *mark.Mark) { m.Kind = mark.Thought; m.Body = "x"; m.Reason = "y" }},
		{"a thought that is resolved", func(m *mark.Mark) { m.Kind = mark.Thought; m.Body = "x"; m.ResolvedAt = m.At }},
		{"words around nothing", func(m *mark.Mark) { m.Before = "before" }},
		{"a quote that is not text", func(m *mark.Mark) { m.Quote = "a\xffb" }},
		{"a quote carrying a NUL", func(m *mark.Mark) { m.Quote = "a\x00b" }},
		{"a quote too long", func(m *mark.Mark) { m.Quote = strings.Repeat("a", 8<<10+1) }},
		{"context too long", func(m *mark.Mark) { m.Quote = "q"; m.After = strings.Repeat("a", 513) }},
		{"a reason too long", func(m *mark.Mark) { m.Reason = strings.Repeat("a", 4<<10+1) }},
		{"a body too long", func(m *mark.Mark) { m.Kind = mark.Thought; m.Body = strings.Repeat("a", 32<<10+1) }},
		{"an identity that is not one", func(m *mark.Mark) { m.Identity = "not-a-digest" }},
		{"an identity in the other case", func(m *mark.Mark) { m.Identity = strings.ToUpper(anIdentity) }},
		{"no time", func(m *mark.Mark) { m.At = time.Time{} }},
		{"no note", func(m *mark.Mark) { m.RelPath = "" }},
		{"an absolute path", func(m *mark.Mark) { m.RelPath = "/etc/passwd" }},
		{"a path that climbs out", func(m *mark.Mark) { m.RelPath = "../outside.md" }},
		{"a path that is not in NFC", func(m *mark.Mark) { m.RelPath = "Notes/パ.md" }},
		{"a path carrying a line feed", func(m *mark.Mark) { m.RelPath = "Notes/a\nb.md" }},
		{"a path carrying an escape", func(m *mark.Mark) { m.RelPath = "Notes/\x1b[2J.md" }},
		{"a path carrying a line separator", func(m *mark.Mark) { m.RelPath = "Notes/a\u2028b.md" }},
		{"a path that is not UTF-8", func(m *mark.Mark) { m.RelPath = "Notes/a\xffb.md" }},
		{"a path too long", func(m *mark.Mark) { m.RelPath = strings.Repeat("a", 1025) }},
		{"an anchor that could cut the address", func(m *mark.Mark) { m.Anchor = "here#elsewhere" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			refused := aQuestion()
			tt.spoil(refused)
			if _, err := store.Add(refused); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("Add(%s) = %v, want ErrInvalid", tt.name, err)
			}
			if _, err := store.Toggle(refused, admitAny); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("Toggle(%s) = %v, want ErrInvalid", tt.name, err)
			}
			if held, err := store.List(); err != nil || len(held) != 0 {
				t.Errorf("a refused mark was stored: %v, %v", held, err)
			}
		})
	}
}

// TestOrdinaryNoteNamesAreKept is the half the refusals above cannot see: a
// rule on control characters that grew too wide would show here rather than on
// someone's lesson.
func TestOrdinaryNoteNamesAreKept(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, path string }{
		{"a space", "Notes/A published note.md"},
		{"Chinese", "Notes/go/提早返回的管線.md"},
		{"an ideographic space", "Notes/日本　語.md"},
		{"a dotted name", "Notes/v1.2.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			question := aQuestion()
			question.RelPath = tt.path
			if _, err := newStore(t).Add(question); err != nil {
				t.Errorf("Add(%q) = %v, want it kept", tt.path, err)
			}
			place := aPlace()
			place.RelPath = tt.path
			if err := moveBookmark(newStore(t), mark.NoCourse, place); err != nil {
				t.Errorf("MoveBookmark(%q) = %v, want it kept", tt.path, err)
			}
		})
	}
}

func TestAnchorShapeSharedByReaderMarks(t *testing.T) {
	t.Parallel()
	type anchorCase struct {
		name, anchor string
		accepted     bool
	}
	cases := []anchorCase{
		{"empty fallback", "", true},
		{"heading", "topic-one", true},
		{"block", "^safe", true},
		{"ampersand", "a&b", true},
		{"non-ASCII control remains accepted", "a" + string(rune(0x85)) + "b", true},
		{"ASCII boundary", strings.Repeat("a", 256), true},
		{"ASCII over boundary", strings.Repeat("a", 257), false},
		{"CJK boundary", strings.Repeat("漢", 85) + "a", true},
		{"CJK over boundary", strings.Repeat("漢", 86), false},
		{"invalid UTF-8", string([]byte{0xff}), false},
		{"DEL", "a" + string(rune(0x7f)) + "b", false},
	}
	for _, r := range "#?/\\\"'<>" {
		cases = append(cases, anchorCase{fmt.Sprintf("delimiter %U", r), "a" + string(r) + "b", false})
	}
	for r := rune(0); r <= 0x20; r++ {
		cases = append(cases, anchorCase{fmt.Sprintf("control or space %U", r), "a" + string(r) + "b", false})
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			place := aPlace()
			place.Anchor = tt.anchor
			question := aQuestion()
			question.Anchor = tt.anchor
			_, questionErr := store.Toggle(question, admitAny)
			for name, err := range map[string]error{
				"shape":    mark.ValidateAnchor(tt.anchor),
				"bookmark": moveBookmark(store, mark.NoCourse, place),
				"question": questionErr,
			} {
				if tt.accepted && err != nil {
					t.Errorf("%s rejected %q: %v", name, tt.anchor, err)
				}
				if !tt.accepted && !errors.Is(err, mark.ErrInvalid) {
					t.Errorf("%s accepted %q or returned %v; want ErrInvalid", name, tt.anchor, err)
				}
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
