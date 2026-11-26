package mark_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
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

// wantOldQuestions is the questions of oldQuestions as a machine file takes
// them in, which is how they are read now. Their ids are derived from the place
// each marked and are not written here.
func wantOldQuestions() []machinemarks.Mark {
	return []machinemarks.Mark{
		{Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "section-one", At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)},
		{Kind: machinemarks.Question, Path: "Notes/other.md", At: time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)},
	}
}

// aRealVault is the folder of a vault that is really there, and the
// configuration directory and the store of its marks, which a test of the marks
// a machine file takes in needs.
func aRealVault(t *testing.T) (store *mark.Store, root, config string) {
	t.Helper()
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config = t.TempDir()
	return newStoreFor(t, config, root), root, config
}

func TestOldQuestionsAreReadAsQuestionsWithoutWritingAnything(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	held, err := store.MachineMarks()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantOldQuestions(), held, withoutIDs); diff != "" {
		t.Errorf("converted questions (-want +got):\n%s", diff)
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
	store, root, config := aRealVault(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	writeFile(t, oldBookmarkPath(store), oldBookmark)

	added := machinemarks.Mark{ID: markID(9), Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "new-section", At: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	m := machineMarksOver(t, store, root, config)
	if _, err := m.Put(&added, nil, nil, allowed); err != nil {
		t.Fatal(err)
	}
	course := aPlace()
	course.RelPath = "Courses/go/G01.md"
	if err := moveBookmark(store, "Courses/go.md", course); err != nil {
		t.Fatal(err)
	}

	v, err := m.View()
	if err != nil {
		t.Fatal(err)
	}
	var held []machinemarks.Mark
	for _, e := range v.Entries(allowed) {
		held = append(held, e.Mark())
	}
	slices.SortFunc(held, func(a, b machinemarks.Mark) int { return a.At.Compare(b.At) })
	if diff := cmp.Diff(append(wantOldQuestions(), added), held, withoutIDs); diff != "" {
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
	first, root, config := aRealVault(t)
	writeFile(t, oldQuestionsPath(first), oldQuestions)
	writeFile(t, oldBookmarkPath(first), oldBookmark)

	// The reader clears the first old question; that is the first write.
	cleared := wantOldQuestions()[0]
	cleared.ID = markID(1)
	same := func(m *machinemarks.Mark) bool {
		return m.Kind == cleared.Kind && m.Path == cleared.Path && m.Anchor == cleared.Anchor
	}
	out, err := machineMarksOver(t, first, root, config).Toggle(&cleared, same, allowedPlace, allowed)
	if err != nil || !out.Mark.Deleted {
		t.Fatalf("clearing an old question = %+v, %v; want it removed", out, err)
	}
	// And sets a bookmark of their own over the old one.
	moved := aPlace()
	moved.Offset = 7
	if err = moveBookmark(first, mark.NoCourse, moved); err != nil {
		t.Fatal(err)
	}

	second := newStoreFor(t, config, root)
	m := machineMarksOver(t, second, root, config)
	v, err := m.View()
	if err != nil {
		t.Fatal(err)
	}
	if got := liveQuestions(v); !slices.Equal(got, []string{"Notes/other.md#"}) {
		t.Errorf("after a second start the questions are %v, want only the one that was not cleared", got)
	}
	if got, _ := heldBookmark(t, second); got.Offset != 7 {
		t.Errorf("a second start read the old bookmark (offset %d), not the one kept after it", got.Offset)
	}
	// Writing again does not convert again either.
	highlight := machinemarks.Mark{ID: markID(9), Kind: machinemarks.Highlight, Path: "Notes/source.md", Quote: "words", At: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if _, err = m.Put(&highlight, nil, nil, allowed); err != nil {
		t.Fatal(err)
	}
	if v, err = m.View(); err != nil {
		t.Fatal(err)
	}
	if got := liveQuestions(v); !slices.Equal(got, []string{"Notes/other.md#"}) {
		t.Errorf("a write after a second start left the questions %v, want the one", got)
	}
}

// allowedPlace admits any place for a mark that is new.
func allowedPlace(*machinemarks.Mark) bool { return true }

// liveQuestions is where each question that is not deleted is, as the note and
// the anchor, in the order they were made.
func liveQuestions(v *machinemarks.View) []string {
	var held []machinemarks.Mark
	for _, e := range v.Entries(allowed) {
		if m := e.Mark(); !e.Deleted() && m.Kind == machinemarks.Question {
			held = append(held, m)
		}
	}
	slices.SortFunc(held, func(a, b machinemarks.Mark) int { return a.At.Compare(b.At) })
	places := make([]string, len(held))
	for i := range held {
		places[i] = held[i].Path + "#" + held[i].Anchor
	}
	return places
}

func TestConvertingTheSameOldFileTwiceNamesTheSameMarks(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	writeFile(t, oldQuestionsPath(store), oldQuestions)
	first, err := store.MachineMarks()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newStoreFor(t, filepath.Dir(filepath.Dir(store.Dir())), "/vaults/notes").MachineMarks()
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
	// a write of a mark is the first write of a machine file, which takes the
	// earlier file in.
	writeMark := func(t *testing.T, s *mark.Store, root, config string) error {
		t.Helper()
		put := machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "a.md", Body: "x", At: time.Now()}
		_, err := machineMarksOver(t, s, root, config).Put(&put, nil, nil, allowed)
		return err
	}
	writeBookmark := func(t *testing.T, s *mark.Store, _, _ string) error {
		t.Helper()
		return moveBookmark(s, mark.NoCourse, aPlace())
	}
	readBookmark := func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err }
	readMarks := func(s *mark.Store) error { _, err := s.MachineMarks(); return err }
	for _, tt := range []struct {
		name, file, body string
		read             func(*mark.Store) error
		write            func(*testing.T, *mark.Store, string, string) error
	}{
		{"a bookmark in another version", "reader.json", `{"version":2,"vault":"/v","continuation":{"path":"a.md"}}`, readBookmark, writeBookmark},
		{"a bookmark that is not JSON", "reader.json", "not json", readBookmark, writeBookmark},
		{
			"a bookmark the shape refuses", "reader.json", `{"version":1,"continuation":{"path":"../out.md","offset":0,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}`,
			readBookmark, writeBookmark,
		},
		{"questions that are not an array", "uncertainty.json", `{"version":1}`, readMarks, writeMark},
		{
			"questions with a field it does not know", "uncertainty.json", `[{"path":"a.md","anchor":"","time":"2026-09-22T10:00:00Z","content":"private"}]`,
			readMarks, writeMark,
		},
		{
			"a question twice", "uncertainty.json", `[{"path":"a.md","anchor":"","time":"2026-09-22T10:00:00Z"},{"path":"a.md","anchor":"","time":"2026-09-23T10:00:00Z"}]`,
			readMarks, writeMark,
		},
		{"a question the shape refuses", "uncertainty.json", `[{"path":"Source.md\nForged","anchor":"","time":"2026-09-22T10:00:00Z"}]`, readMarks, writeMark},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, root, config := aRealVault(t)
			path := filepath.Join(store.Dir(), tt.file)
			writeFile(t, path, tt.body)
			if err := tt.read(store); !errors.Is(err, mark.ErrUnreadable) {
				t.Errorf("reading %s = %v, want ErrUnreadable", tt.name, err)
			}
			if err := tt.write(t, store, root, config); err == nil || errors.Is(err, mark.ErrInvalid) {
				t.Errorf("writing over %s = %v, want a refusal that is not ErrInvalid", tt.name, err)
			}
			if bytesOf(t, path) != tt.body {
				t.Errorf("%s was changed", tt.file)
			}
			for _, written := range []string{"marks.json", "progress.json"} {
				if _, err := os.Stat(filepath.Join(store.Dir(), written)); !os.IsNotExist(err) {
					t.Errorf("a refused write created %s", written)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "reader-notes")); !os.IsNotExist(err) {
				t.Errorf("a refused write made the reader folder: %v", err)
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
	if err := moveBookmark(store, mark.NoCourse, aPlace()); err != nil {
		t.Errorf("MoveBookmark over it = %v", err)
	}
}
