package mark_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

// The files an earlier version wrote, as bytes written out by hand rather than
// taken from the code's constants: a constant changed together with the code it
// guards proves nothing about what is already on a reader's disk.
const (
	oldBookmark = `{
  "version": 1,
  "vault": "/vaults/notes",
  "continuation": {
    "path": "Writing/lessons/japanese/L01.md",
    "anchor": "the-topic-particle",
    "offset": 420,
    "identity": "` + anIdentity + `",
    "at": "2026-09-17T10:00:00Z"
  }
}
`
	oldQuestions = `[
  {"path": "Notes/source.md", "anchor": "section-one", "time": "2026-09-22T10:00:00Z"},
  {"path": "Notes/other.md", "anchor": "", "time": "2026-09-23T09:00:00+08:00"}
]
`
)

func oldBookmarkPath(store *mark.Store) string { return filepath.Join(store.Dir(), "reader.json") }

func oldQuestionsPath(store *mark.Store) string {
	return filepath.Join(store.Dir(), "uncertainty.json")
}

// bytesOf reads a file for comparing before and after.
func bytesOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a file a test wrote under its own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func wantOldQuestions() []mark.Mark {
	return []mark.Mark{
		{Kind: mark.Question, RelPath: "Notes/source.md", Anchor: "section-one", At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)},
		{Kind: mark.Question, RelPath: "Notes/other.md", At: time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)},
	}
}

func TestOldQuestionsAreReadAsQuestionsWithoutWritingAnything(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	held, err := store.List(mark.Question)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantOldQuestions(), held, withoutIDs); diff != "" {
		t.Errorf("converted questions (-want +got):\n%s", diff)
	}
	if !store.Stored() {
		t.Error("a store with an old questions file reports no marks file")
	}
	entries, err := os.ReadDir(store.Dir())
	if err != nil || len(entries) != 1 || entries[0].Name() != "uncertainty.json" {
		t.Errorf("reading wrote beside the old file: %v, %v", entries, err)
	}
	if bytesOf(t, oldQuestionsPath(store)) != oldQuestions {
		t.Error("reading changed the old file")
	}
}

func TestOldBookmarkIsTheBookmarkOutsideAnyCourse(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldBookmarkPath(store), oldBookmark)
	got, found, err := store.Bookmark(mark.NoCourse)
	if err != nil || !found {
		t.Fatalf("Bookmark = %v, %v", found, err)
	}
	if diff := cmp.Diff(*aPlace(), got); diff != "" {
		t.Errorf("converted bookmark (-want +got):\n%s", diff)
	}
	if _, found, err = store.Bookmark("Courses/go.md"); err != nil || found {
		t.Errorf("the old bookmark belongs to a course: %v, %v", found, err)
	}
	if _, err = os.Stat(progressPath(store)); !os.IsNotExist(err) {
		t.Errorf("reading wrote %s", progressPath(store))
	}
}

// The first write converts what is there into the new file, with the new mark
// beside it; the old files are never touched, so a reader who goes back to the
// version that wrote them finds them as they left them.
func TestTheFirstWriteConvertsAndTheOldFilesStayAsTheyWere(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	writeFile(t, oldBookmarkPath(store), oldBookmark)

	added := aQuestion()
	added.Anchor = "new-section"
	if _, err := store.Toggle(added, admitAny); err != nil {
		t.Fatal(err)
	}
	course := aPlace()
	course.RelPath = "Courses/go/G01.md"
	if err := store.SetBookmark("Courses/go.md", course); err != nil {
		t.Fatal(err)
	}

	held, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(append(wantOldQuestions(), *added), held, withoutIDs); diff != "" {
		t.Errorf("marks after the first write (-want +got):\n%s", diff)
	}
	if got, found, bookmarkErr := store.Bookmark(mark.NoCourse); bookmarkErr != nil || !found || got.RelPath != aPlace().RelPath {
		t.Errorf("the converted bookmark = %v, %v, %v; want the old place kept for no course", got, found, bookmarkErr)
	}
	if got, found, bookmarkErr := store.Bookmark("Courses/go.md"); bookmarkErr != nil || !found || got.RelPath != course.RelPath {
		t.Errorf("the new bookmark = %v, %v, %v", got, found, bookmarkErr)
	}
	if bytesOf(t, oldQuestionsPath(store)) != oldQuestions || bytesOf(t, oldBookmarkPath(store)) != oldBookmark {
		t.Error("a write changed an old file")
	}
}

// The new file existing is what says the old one has been converted. A second
// start does not convert again, and a mark the reader deleted after the
// conversion is not brought back from the old file that still holds it.
func TestASecondStartDoesNotConvertAgainOrBringBackWhatWasDeleted(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	first := newStoreFor(t, config, "/vaults/notes")
	writeFile(t, oldQuestionsPath(first), oldQuestions)
	writeFile(t, oldBookmarkPath(first), oldBookmark)

	// The reader clears the first old question; that is the first write.
	cleared := wantOldQuestions()[0]
	if added, err := first.Toggle(&cleared, admitAny); err != nil || added {
		t.Fatalf("clearing an old question = %v, %v; want it removed", added, err)
	}
	// And sets a bookmark of their own over the old one.
	moved := aPlace()
	moved.Offset = 7
	if err := first.SetBookmark(mark.NoCourse, moved); err != nil {
		t.Fatal(err)
	}

	second := newStoreFor(t, config, "/vaults/notes")
	held, err := second.List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantOldQuestions()[1:], held, withoutIDs); diff != "" {
		t.Errorf("after a second start (-want +got):\n%s", diff)
	}
	if got, _ := heldBookmark(t, second); got.Offset != 7 {
		t.Errorf("a second start read the old bookmark (offset %d), not the one kept after it", got.Offset)
	}
	// Writing again does not convert again either.
	if _, err = second.Add(aHighlight()); err != nil {
		t.Fatal(err)
	}
	if held, err = second.List(mark.Question); err != nil || len(held) != 1 {
		t.Errorf("a write after a second start left %d questions (%v), want 1", len(held), err)
	}
}

func TestConvertingTheSameOldFileTwiceNamesTheSameMarks(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	first, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newStoreFor(t, filepath.Dir(filepath.Dir(store.Dir())), "/vaults/notes").List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("two reads of one old file named different marks (-first +second):\n%s", diff)
	}
	if len(first) != 2 || first[0].ID == first[1].ID {
		t.Errorf("converted marks share an id: %v", first)
	}
}

// An old file this version does not understand is the reader's own record. It
// is reported, kept, and what depends on it is refused; it is never read as
// empty and never replaced.
func TestAnOldFileThatCannotBeConvertedIsReportedAndKept(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, file, body string
		read             func(*mark.Store) error
		write            func(*mark.Store) error
	}{
		{
			"a bookmark in another version", "reader.json", `{"version":2,"vault":"/v","continuation":{"path":"a.md"}}`,
			func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err },
			func(s *mark.Store) error { return s.SetBookmark(mark.NoCourse, aPlace()) },
		},
		{
			"a bookmark that is not JSON", "reader.json", "not json",
			func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err },
			func(s *mark.Store) error { return s.SetBookmark(mark.NoCourse, aPlace()) },
		},
		{
			"a bookmark the shape refuses", "reader.json", `{"version":1,"continuation":{"path":"../out.md","offset":0,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}`,
			func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err },
			func(s *mark.Store) error { return s.SetBookmark(mark.NoCourse, aPlace()) },
		},
		{
			"questions that are not an array", "uncertainty.json", `{"version":1}`,
			func(s *mark.Store) error { _, err := s.List(); return err },
			func(s *mark.Store) error { _, err := s.Toggle(aQuestion(), admitAny); return err },
		},
		{
			"questions with a field it does not know", "uncertainty.json", `[{"path":"a.md","anchor":"","time":"2026-09-22T10:00:00Z","content":"private"}]`,
			func(s *mark.Store) error { _, err := s.List(); return err },
			func(s *mark.Store) error { _, err := s.Add(aHighlight()); return err },
		},
		{
			"a question twice", "uncertainty.json", `[{"path":"a.md","anchor":"","time":"2026-09-22T10:00:00Z"},{"path":"a.md","anchor":"","time":"2026-09-23T10:00:00Z"}]`,
			func(s *mark.Store) error { _, err := s.List(); return err },
			func(s *mark.Store) error { _, err := s.Toggle(aQuestion(), admitAny); return err },
		},
		{
			"a question the shape refuses", "uncertainty.json", `[{"path":"Source.md\nForged","anchor":"","time":"2026-09-22T10:00:00Z"}]`,
			func(s *mark.Store) error { _, err := s.List(); return err },
			func(s *mark.Store) error { _, err := s.Toggle(aQuestion(), admitAny); return err },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			path := filepath.Join(store.Dir(), tt.file)
			writeFile(t, path, tt.body)
			if err := tt.read(store); !errors.Is(err, mark.ErrUnreadable) {
				t.Errorf("reading %s = %v, want ErrUnreadable", tt.name, err)
			}
			if err := tt.write(store); !errors.Is(err, mark.ErrUnreadable) || errors.Is(err, mark.ErrInvalid) {
				t.Errorf("writing over %s = %v, want ErrUnreadable and not ErrInvalid", tt.name, err)
			}
			if bytesOf(t, path) != tt.body {
				t.Errorf("%s was changed", tt.file)
			}
			for _, written := range []string{"marks.json", "progress.json"} {
				if _, err := os.Stat(filepath.Join(store.Dir(), written)); !os.IsNotExist(err) {
					t.Errorf("a refused write created %s", written)
				}
			}
		})
	}
}

// An old bookmark file that holds no place is the file of a reader who set
// none, not a damaged one.
func TestAnOldBookmarkFileWithNoPlaceInItIsNoBookmark(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldBookmarkPath(store), `{"version":1,"vault":"/vaults/notes"}`)
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || found {
		t.Errorf("Bookmark = %v, %v; want none and no error", found, err)
	}
	if err := store.SetBookmark(mark.NoCourse, aPlace()); err != nil {
		t.Errorf("SetBookmark over it = %v", err)
	}
}
