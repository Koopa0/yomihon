package mark_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

func TestAKeptPlaceComesBack(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if _, ok := heldBookmark(t, store); ok {
		t.Fatal("a store nobody has written to reports a kept place")
	}
	want := aPlace()
	if err := store.SetBookmark(mark.NoCourse, want); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}
	got, ok := heldBookmark(t, store)
	if !ok {
		t.Fatal("the place that was just kept is not reported")
	}
	if diff := cmp.Diff(*want, got); diff != "" {
		t.Errorf("Bookmark() mismatch (-want +got):\n%s", diff)
	}
}

// TestKeepingAnotherPlaceReplacesTheFirst holds the ruled shape: one place for
// one course. A file that accumulated them would be a reading history, which is
// a different thing and was not asked for.
func TestKeepingAnotherPlaceReplacesTheFirst(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if err := store.SetBookmark(mark.NoCourse, aPlace()); err != nil {
		t.Fatalf("keep the first place: %v", err)
	}
	second := aPlace()
	second.RelPath = "Notes/alpha.md"
	second.Anchor = "elsewhere"
	second.Offset = 12
	if err := store.SetBookmark(mark.NoCourse, second); err != nil {
		t.Fatalf("keep the second place: %v", err)
	}
	got, ok := heldBookmark(t, store)
	if !ok {
		t.Fatal("no place is reported after two were kept")
	}
	if diff := cmp.Diff(*second, got); diff != "" {
		t.Errorf("the second place did not replace the first (-want +got):\n%s", diff)
	}
	data, err := os.ReadFile(progressPath(store))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Bookmarks []json.RawMessage }
	if err = json.Unmarshal(data, &doc); err != nil || len(doc.Bookmarks) != 1 {
		t.Errorf("the file holds %d bookmarks (%v), want one", len(doc.Bookmarks), err)
	}
}

func TestEachCourseKeepsItsOwnBookmark(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	const go1, go2 = "Courses/go.md", "Courses/japanese.md"
	inGo, inJapanese := aPlace(), aPlace()
	inGo.RelPath, inJapanese.RelPath = "Courses/go/G01.md", "Courses/japanese/J01.md"
	for course, place := range map[string]*mark.Bookmark{go1: inGo, go2: inJapanese, mark.NoCourse: aPlace()} {
		if err := store.SetBookmark(course, place); err != nil {
			t.Fatalf("SetBookmark(%q): %v", course, err)
		}
	}
	moved := *inGo
	moved.RelPath = "Courses/go/G02.md"
	if err := store.SetBookmark(go1, &moved); err != nil {
		t.Fatal(err)
	}
	for course, want := range map[string]mark.Bookmark{go1: moved, go2: *inJapanese, mark.NoCourse: *aPlace()} {
		got, found, err := store.Bookmark(course)
		if err != nil || !found {
			t.Fatalf("Bookmark(%q) = %v, %v", course, found, err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Bookmark(%q) (-want +got):\n%s", course, diff)
		}
	}
	if _, found, err := store.Bookmark("Courses/other.md"); err != nil || found {
		t.Errorf("a course nobody bookmarked reports one: %v, %v", found, err)
	}
}

func TestARefusedPlaceIsNotStored(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		spoil func(*mark.Bookmark)
	}{
		{"no note", func(c *mark.Bookmark) { c.RelPath = "" }},
		{"an absolute path", func(c *mark.Bookmark) { c.RelPath = "/etc/passwd" }},
		{"a path that climbs out", func(c *mark.Bookmark) { c.RelPath = "../outside.md" }},
		{"a path that is not in NFC", func(c *mark.Bookmark) { c.RelPath = "Notes/パ.md" }},
		// fs.ValidPath lets control characters through, so each class the
		// predicate names is driven: C0, DELETE, C1 and the two Unicode
		// separators that end a line without being a newline.
		{"a path carrying a NUL", func(c *mark.Bookmark) { c.RelPath = "Notes/a\x00b.md" }},
		{"a path carrying a line feed", func(c *mark.Bookmark) { c.RelPath = "Notes/a\nb.md" }},
		{"a path carrying a NUL and a line feed", func(c *mark.Bookmark) { c.RelPath = "Notes/a\x00b\nc.md" }},
		{"a path carrying a carriage return", func(c *mark.Bookmark) { c.RelPath = "Notes/a\rb.md" }},
		{"a path carrying a tab", func(c *mark.Bookmark) { c.RelPath = "Notes/a\tb.md" }},
		{"a path carrying DELETE", func(c *mark.Bookmark) { c.RelPath = "Notes/a\x7fb.md" }},
		{"a path carrying a C1 control", func(c *mark.Bookmark) { c.RelPath = "Notes/a\u0085b.md" }},
		{"a path carrying a line separator", func(c *mark.Bookmark) { c.RelPath = "Notes/a\u2028b.md" }},
		{"a path carrying a paragraph separator", func(c *mark.Bookmark) { c.RelPath = "Notes/a\u2029b.md" }},
		{"a path that is not UTF-8", func(c *mark.Bookmark) { c.RelPath = "Notes/a\xffb.md" }},
		{"an anchor that could cut the address", func(c *mark.Bookmark) { c.Anchor = "here#elsewhere" }},
		{"an anchor carrying a control character", func(c *mark.Bookmark) { c.Anchor = "here\nthere" }},
		// The anchor had only the character check, which reads an invalid byte
		// as U+FFFD and let it through to be stored as something its sender
		// never wrote.
		{"an anchor that is not UTF-8", func(c *mark.Bookmark) { c.Anchor = "here\xff" }},
		{"a negative offset", func(c *mark.Bookmark) { c.Offset = -1 }},
		{"an offset past any document", func(c *mark.Bookmark) { c.Offset = 1 << 30 }},
		{"an identity that is not one", func(c *mark.Bookmark) { c.Identity = "not-a-digest" }},
		{"an identity in the other case", func(c *mark.Bookmark) { c.Identity = strings.ToUpper(anIdentity) }},
		{"no identity at all", func(c *mark.Bookmark) { c.Identity = "" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			refused := aPlace()
			tt.spoil(refused)
			if err := store.SetBookmark(mark.NoCourse, refused); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("SetBookmark(%s) = %v, want ErrInvalid", tt.name, err)
			}
			if _, ok := heldBookmark(t, store); ok {
				t.Error("a refused place was stored anyway")
			}
			if _, statErr := os.Stat(progressPath(store)); !os.IsNotExist(statErr) {
				t.Errorf("a refused place created %s", progressPath(store))
			}
		})
	}
	store := newStore(t)
	for _, course := range []string{"../escape.md", "/abs.md", "a\nb.md", "Courses/パ.md"} {
		if err := store.SetBookmark(course, aPlace()); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("SetBookmark(course %q) = %v, want ErrInvalid", course, err)
		}
	}
}

// TestARefusalLeavesTheKeptPlaceAlone is the half the test above cannot see.
// That one starts from nothing, so "nothing was stored" is also what a refusal
// that wiped the file would look like. A refusal must not be a way to clear a
// place the reader meant to keep, and only a store that already holds one can
// show it.
func TestARefusalLeavesTheKeptPlaceAlone(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	want := aPlace()
	if err := store.SetBookmark(mark.NoCourse, want); err != nil {
		t.Fatalf("keep a place: %v", err)
	}
	refused := aPlace()
	refused.Identity = "not-a-digest"
	if err := store.SetBookmark(mark.NoCourse, refused); err == nil {
		t.Fatal("a place with no identity was accepted")
	}
	got, ok := heldBookmark(t, store)
	if !ok {
		t.Fatal("the kept place is gone after a refusal")
	}
	if diff := cmp.Diff(*want, got); diff != "" {
		t.Errorf("a refusal changed the kept place (-want +got):\n%s", diff)
	}
}

// A progress file this version does not recognise is the reader's only record
// of where they were. It is reported and kept as it is, and what the reader
// keeps next is refused until they have looked at it: nothing here can say what
// it meant, and replacing it would be the one way to lose it.
func TestAProgressFileThatCannotBeReadIsReportedAndNeverReplaced(t *testing.T) {
	t.Parallel()
	place := `{"bookmark":{"path":"a.md","offset":0,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}`
	for _, tt := range []struct{ name, body string }{
		{"not JSON at all", "this is not a file yomihon wrote\n"},
		{"an empty file", ""},
		{"a version from another day", `{"version":99,"vault":"/v","bookmarks":[` + place + `]}`},
		{"a bookmark with no place in it", `{"version":1,"vault":"/v","bookmarks":[{"course":"Courses/go.md"}]}`},
		{"no list", `{"version":1,"vault":"/vaults/notes"}`},
		{"a field it does not know", `{"version":1,"vault":"/v","bookmarks":[],"finished":[]}`},
		{"a place the shape refuses", `{"version":1,"vault":"/v","bookmarks":[{"bookmark":{"path":"../outside.md","offset":0,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}]}`},
		{"a path carrying a control character", `{"version":1,"vault":"/v","bookmarks":[{"bookmark":{"path":"Notes/a\u0000b\nc.md","offset":1,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}]}`},
		{"two bookmarks for one course", `{"version":1,"vault":"/v","bookmarks":[` + place + `,` + place + `]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			writeFile(t, progressPath(store), tt.body)
			if _, _, err := store.Bookmark(mark.NoCourse); !errors.Is(err, mark.ErrUnreadable) {
				t.Errorf("Bookmark over %s = %v, want ErrUnreadable", tt.name, err)
			}
			if err := store.SetBookmark(mark.NoCourse, aPlace()); !errors.Is(err, mark.ErrUnreadable) || errors.Is(err, mark.ErrInvalid) {
				t.Errorf("SetBookmark over %s = %v, want ErrUnreadable and not ErrInvalid", tt.name, err)
			}
			if data, err := os.ReadFile(progressPath(store)); err != nil || string(data) != tt.body {
				t.Errorf("the file changed to %q: %v", data, err)
			}
		})
	}
}
