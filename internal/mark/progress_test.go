package mark_test

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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
	if err := moveBookmark(store, mark.NoCourse, want); err != nil {
		t.Fatalf("MoveBookmark: %v", err)
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
	if err := moveBookmark(store, mark.NoCourse, aPlace()); err != nil {
		t.Fatalf("keep the first place: %v", err)
	}
	second := aPlace()
	second.RelPath = "Notes/alpha.md"
	second.Anchor = "elsewhere"
	second.Offset = 12
	if err := moveBookmark(store, mark.NoCourse, second); err != nil {
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
		if err := moveBookmark(store, course, place); err != nil {
			t.Fatalf("MoveBookmark(%q): %v", course, err)
		}
	}
	moved := *inGo
	moved.RelPath = "Courses/go/G02.md"
	if err := moveBookmark(store, go1, &moved); err != nil {
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
			if err := moveBookmark(store, mark.NoCourse, refused); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("MoveBookmark(%s) = %v, want ErrInvalid", tt.name, err)
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
		if err := moveBookmark(store, course, aPlace()); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("MoveBookmark(course %q) = %v, want ErrInvalid", course, err)
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
	if err := moveBookmark(store, mark.NoCourse, want); err != nil {
		t.Fatalf("keep a place: %v", err)
	}
	refused := aPlace()
	refused.Identity = "not-a-digest"
	if err := moveBookmark(store, mark.NoCourse, refused); err == nil {
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
	finished := `{"path":"a.md","at":"2026-09-17T10:00:00Z"}`
	for _, tt := range []struct{ name, body string }{
		{"not JSON at all", "this is not a file yomihon wrote\n"},
		{"an empty file", ""},
		{"a version from another day", `{"version":99,"vault":"/v","bookmarks":[` + place + `]}`},
		{"a bookmark with no place in it", `{"version":1,"vault":"/v","bookmarks":[{"course":"Courses/go.md"}]}`},
		{"no list", `{"version":1,"vault":"/vaults/notes"}`},
		{"a field it does not know", `{"version":1,"vault":"/v","bookmarks":[],"streak":3}`},
		{"a note finished twice", `{"version":1,"vault":"/v","bookmarks":[],"finished":[` + finished + `,` + finished + `]}`},
		{"a finished note with no time", `{"version":1,"vault":"/v","bookmarks":[],"finished":[{"path":"a.md","at":"0001-01-01T00:00:00Z"}]}`},
		{"a finished path the shape refuses", `{"version":1,"vault":"/v","bookmarks":[],"finished":[{"path":"../outside.md","at":"2026-09-17T10:00:00Z"}]}`},
		{"a finished note with a field it does not know", `{"version":1,"vault":"/v","bookmarks":[],"finished":[{"path":"a.md","at":"2026-09-17T10:00:00Z","stars":5}]}`},
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
			if err := moveBookmark(store, mark.NoCourse, aPlace()); !errors.Is(err, mark.ErrUnreadable) || errors.Is(err, mark.ErrInvalid) {
				t.Errorf("MoveBookmark over %s = %v, want ErrUnreadable and not ErrInvalid", tt.name, err)
			}
			if data, err := os.ReadFile(progressPath(store)); err != nil || string(data) != tt.body {
				t.Errorf("the file changed to %q: %v", data, err)
			}
		})
	}
}

func TestAFinishedNoteComesBackWithWhenItWasFinished(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/notes")
	before := time.Now().Add(-time.Second)
	if err := store.SetFinished("Notes/a.md", true); err != nil {
		t.Fatal(err)
	}
	at, finished, err := store.FinishedAt("Notes/a.md")
	if err != nil || !finished || at.Before(before) || at.After(time.Now().Add(time.Second)) {
		t.Errorf("FinishedAt = %v, %v, %v; want finished a moment ago", at, finished, err)
	}
	if _, finished, err = store.FinishedAt("Notes/b.md"); err != nil || finished {
		t.Errorf("a note nobody finished = %v, %v", finished, err)
	}
	reopened := newStoreFor(t, config, "/vaults/notes")
	if _, finished, err = reopened.FinishedAt("Notes/a.md"); err != nil || !finished {
		t.Errorf("after a restart FinishedAt = %v, %v", finished, err)
	}
}

// Saying what is already so changes nothing, so the first time stays the time
// and a store nobody finished anything in is not given a file.
func TestFinishedIsSaidOnceAndUndoneByNotFinished(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if err := store.SetFinished("Notes/a.md", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(progressPath(store)); !os.IsNotExist(err) {
		t.Errorf("saying a note is not finished created %s", progressPath(store))
	}
	if err := store.SetFinished("Notes/a.md", true); err != nil {
		t.Fatal(err)
	}
	first, _, err := store.FinishedAt("Notes/a.md")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err = store.SetFinished("Notes/a.md", true); err != nil {
		t.Fatal(err)
	}
	if again, _, readErr := store.FinishedAt("Notes/a.md"); readErr != nil || !again.Equal(first) {
		t.Errorf("finishing a finished note moved its time from %v to %v (%v)", first, again, readErr)
	}
	if err := store.SetFinished("Notes/a.md", false); err != nil {
		t.Fatal(err)
	}
	if _, finished, err := store.FinishedAt("Notes/a.md"); err != nil || finished {
		t.Errorf("after not finished, FinishedAt = %v, %v", finished, err)
	}
}

func TestFinishedNotesAreListedInTheOrderTheyWereFinished(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, note := range []string{"Notes/c.md", "Notes/a.md", "Notes/b.md"} {
		if err := store.SetFinished(note, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetFinished("Notes/a.md", false); err != nil {
		t.Fatal(err)
	}
	held, err := store.FinishedNotes()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range held {
		got = append(got, f.RelPath)
	}
	if diff := cmp.Diff([]string{"Notes/c.md", "Notes/b.md"}, got); diff != "" {
		t.Errorf("finished notes (-want +got):\n%s", diff)
	}
}

func TestAnInvalidNoteCannotBeFinished(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, note := range []string{"", "/abs.md", "../outside.md", "Notes/a\nb.md", "Notes/\u30cf\u309a.md", "Notes/a\xffb.md"} {
		if err := store.SetFinished(note, true); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("SetFinished(%q) = %v, want ErrInvalid", note, err)
		}
	}
	if _, err := os.Stat(progressPath(store)); !os.IsNotExist(err) {
		t.Errorf("a refused note created %s", progressPath(store))
	}
}

// What is finished and where the reader left off are separate things in one
// file, and neither replaces the other.
func TestFinishedAndBookmarksShareAFileAndKeepEachOther(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if err := moveBookmark(store, "Courses/go.md", aPlace()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFinished("Notes/a.md", true); err != nil {
		t.Fatal(err)
	}
	other := aPlace()
	other.RelPath = "Notes/z.md"
	if err := moveBookmark(store, "Courses/go.md", other); err != nil {
		t.Fatal(err)
	}
	if got, found, err := store.Bookmark("Courses/go.md"); err != nil || !found || got.RelPath != "Notes/z.md" {
		t.Errorf("the bookmark = %v, %v, %v", got, found, err)
	}
	if _, finished, err := store.FinishedAt("Notes/a.md"); err != nil || !finished {
		t.Errorf("setting a bookmark undid a finished note: %v, %v", finished, err)
	}
	if _, _, err := store.ClearBookmark("Courses/go.md"); err != nil {
		t.Fatal(err)
	}
	if _, finished, err := store.FinishedAt("Notes/a.md"); err != nil || !finished {
		t.Errorf("clearing a bookmark undid a finished note: %v, %v", finished, err)
	}
}

func TestMoreFinishedNotesThanTheBoundAreRefusedWhenAddedAndKeptWhenStored(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	var entries []string
	for i := range 10000 {
		entries = append(entries, `{"path":"Notes/n`+strconv.Itoa(i)+`.md","at":"2026-09-17T10:00:00Z"}`)
	}
	writeFile(t, progressPath(store), `{"version":1,"vault":"/v","bookmarks":[],"finished":[`+strings.Join(entries, ",")+`]}`)
	if err := store.SetFinished("Notes/one-more.md", true); !errors.Is(err, mark.ErrInvalid) {
		t.Errorf("finishing one more = %v, want ErrInvalid", err)
	}
	if err := store.SetFinished("Notes/n0.md", false); err != nil {
		t.Errorf("clearing one at the bound = %v, want nil", err)
	}
	if err := store.SetFinished("Notes/one-more.md", true); err != nil {
		t.Errorf("finishing one after clearing one = %v, want nil", err)
	}
	writeFile(t, progressPath(store), `{"version":1,"vault":"/v","bookmarks":[],"finished":[`+strings.Join(append(entries, `{"path":"Notes/over.md","at":"2026-09-17T10:00:00Z"}`), ",")+`]}`)
	if _, err := store.FinishedNotes(); !errors.Is(err, mark.ErrUnreadable) {
		t.Errorf("a file beyond the bound = %v, want ErrUnreadable", err)
	}
}

func TestClearingABookmarkRemovesOnlyThatCoursesAndWritesNothingWhereThereIsNone(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if _, removed, err := store.ClearBookmark("Courses/go.md"); err != nil || removed {
		t.Fatalf("ClearBookmark on an empty store = %v, %v", removed, err)
	}
	if _, err := os.Stat(progressPath(store)); !os.IsNotExist(err) {
		t.Errorf("clearing nothing created %s", progressPath(store))
	}
	for _, course := range []string{"Courses/go.md", "Courses/rust.md", mark.NoCourse} {
		if err := moveBookmark(store, course, aPlace()); err != nil {
			t.Fatal(err)
		}
	}
	if got, removed, err := store.ClearBookmark("Courses/go.md"); err != nil || !removed || got.RelPath != aPlace().RelPath {
		t.Fatalf("ClearBookmark = %v, %v, %v, want the bookmark that was kept", got, removed, err)
	}
	for course, want := range map[string]bool{"Courses/go.md": false, "Courses/rust.md": true, mark.NoCourse: true} {
		if _, found, err := store.Bookmark(course); err != nil || found != want {
			t.Errorf("Bookmark(%q) found = %v, %v; want %v", course, found, err, want)
		}
	}
	if _, removed, err := store.ClearBookmark("Courses/go.md"); err != nil || removed {
		t.Errorf("clearing a bookmark twice = %v, %v", removed, err)
	}
	if _, _, err := store.ClearBookmark("../escape.md"); !errors.Is(err, mark.ErrInvalid) {
		t.Errorf("ClearBookmark of a path that climbs out = %v, want ErrInvalid", err)
	}
}

// The last bookmark cleared leaves a file the next read takes, and the old
// reader.json it was converted from does not bring it back.
func TestAClearedConvertedBookmarkStaysCleared(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldBookmarkPath(store), oldBookmark)
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || !found {
		t.Fatalf("the converted bookmark = %v, %v", found, err)
	}
	if _, removed, err := store.ClearBookmark(mark.NoCourse); err != nil || !removed {
		t.Fatalf("ClearBookmark = %v, %v", removed, err)
	}
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || found {
		t.Errorf("after clearing, the old bookmark came back: %v, %v", found, err)
	}
	if bytesOf(t, oldBookmarkPath(store)) != oldBookmark {
		t.Error("clearing changed the old file")
	}
}

// A page that moves the bookmark says where it was and offers to put it back,
// so the store has to hand back what it replaced exactly, and setting that
// back has to give the file it had.
func TestMovingABookmarkAnswersTheOneItReplacedAndSettingItBackRestoresTheFile(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	course := "Courses/go.md"

	first := aPlace()
	if previous, had, err := store.MoveBookmark(course, first, nil); err != nil || had || previous != (mark.Bookmark{}) {
		t.Fatalf("the first MoveBookmark = %v, %v, %v; want nothing replaced", previous, had, err)
	}
	before := contents(t, progressPath(store))

	later := aPlace()
	later.RelPath, later.Anchor, later.Offset = "Notes/z.md", "", 7
	later.At = first.At.Add(time.Hour)
	previous, had, err := store.MoveBookmark(course, later, nil)
	if err != nil || !had || previous != *first {
		t.Fatalf("MoveBookmark = %v, %v, %v; want the first bookmark back", previous, had, err)
	}
	if got, found, err := store.Bookmark(course); err != nil || !found || got != *later {
		t.Errorf("the bookmark after moving = %v, %v, %v; want the later one", got, found, err)
	}
	if _, _, err := store.MoveBookmark(course, &previous, nil); err != nil {
		t.Fatal(err)
	}
	if after := contents(t, progressPath(store)); after != before {
		t.Errorf("setting the old bookmark back changed the file:\nbefore %s\nafter  %s", before, after)
	}
}

func TestAMovedBookmarkLeavesOtherCoursesAndTheFileAloneWhenRefused(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, course := range []string{"Courses/go.md", mark.NoCourse} {
		if err := moveBookmark(store, course, aPlace()); err != nil {
			t.Fatal(err)
		}
	}
	before := contents(t, progressPath(store))

	bad := aPlace()
	bad.Identity = "not an identity"
	if _, _, err := store.MoveBookmark("Courses/go.md", bad, nil); !errors.Is(err, mark.ErrInvalid) {
		t.Errorf("a refused bookmark = %v, want ErrInvalid", err)
	}
	if _, _, err := store.MoveBookmark("../escape.md", aPlace(), nil); !errors.Is(err, mark.ErrInvalid) {
		t.Errorf("a course that climbs out = %v, want ErrInvalid", err)
	}
	if after := contents(t, progressPath(store)); after != before {
		t.Errorf("a refusal changed the file:\nbefore %s\nafter  %s", before, after)
	}
}

// A bookmark cleared and set back leaves the file as it was, however many other
// courses have one: the order they are written in does not depend on the order
// they were set in.
func TestClearingAndSettingBackABookmarkRestoresTheFileWithSeveralKept(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, course := range []string{"Courses/go.md", "Courses/rust.md", mark.NoCourse} {
		if err := moveBookmark(store, course, aPlace()); err != nil {
			t.Fatal(err)
		}
	}
	before := contents(t, progressPath(store))

	removed, had, err := store.ClearBookmark("Courses/go.md")
	if err != nil || !had {
		t.Fatalf("ClearBookmark = %v, %v", had, err)
	}
	if _, _, err = store.MoveBookmark("Courses/go.md", &removed, nil); err != nil {
		t.Fatal(err)
	}
	if after := contents(t, progressPath(store)); after != before {
		t.Errorf("clearing a bookmark and setting it back changed the file:\nbefore %s\nafter  %s", before, after)
	}
}

// An undo that says what it expects to replace is refused where something else
// is kept, and changes nothing, so another page's move is not lost.
func TestAMoveThatExpectsAnotherBookmarkIsRefusedAndChangesNothing(t *testing.T) {
	t.Parallel()
	const course = "Courses/go.md"
	first := aPlace()
	later := aPlace()
	later.RelPath, later.At = "Notes/z.md", first.At.Add(time.Hour)

	for name, tt := range map[string]struct {
		kept    bool
		expect  *mark.Expectation
		changed bool
	}{
		"the bookmark kept":              {true, &mark.Expectation{At: &first.At}, false},
		"no condition":                   {true, nil, false},
		"another bookmark":               {true, &mark.Expectation{At: &later.At}, true},
		"none, where one is kept":        {true, &mark.Expectation{}, true},
		"none, where none is kept":       {false, &mark.Expectation{}, false},
		"a bookmark, where none is kept": {false, &mark.Expectation{At: &first.At}, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			var held string
			if tt.kept {
				if err := moveBookmark(store, course, first); err != nil {
					t.Fatal(err)
				}
				held = contents(t, progressPath(store))
			}

			_, _, err := store.MoveBookmark(course, later, tt.expect)
			if errors.Is(err, mark.ErrChanged) != tt.changed || (err != nil) != tt.changed {
				t.Fatalf("MoveBookmark = %v, want refused = %v", err, tt.changed)
			}
			kept, found, readErr := store.Bookmark(course)
			if readErr != nil {
				t.Fatal(readErr)
			}
			switch {
			case !tt.changed:
				if !found || kept.RelPath != later.RelPath || !kept.At.Equal(later.At) {
					t.Errorf("the bookmark kept = %+v, %v after a move taken, want %+v", kept, found, *later)
				}
			case tt.kept:
				if got := contents(t, progressPath(store)); got != held {
					t.Errorf("a refused move changed the file:\nbefore %s\nafter  %s", held, got)
				}
			case found:
				t.Errorf("a refused move kept the bookmark %+v", kept)
			}
		})
	}
}
