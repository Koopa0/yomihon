package mark_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// withoutIDs compares marks by what they say: the id of a mark converted from
// the first of the earlier files is derived, and is not something a test needs
// to write in advance.
var withoutIDs = cmpopts.IgnoreFields(machinemarks.Mark{}, "ID")

// aMarksFile is a marks.json holding the given marks, written by hand so that
// a test of reading does not lean on the code that writes.
func aMarksFile(marks ...string) string {
	return `{"version":1,"vault":"/vaults/notes","marks":[` + strings.Join(marks, ",") + `]}`
}

// marksJSON is a marks.json for the vault at vaultRoot holding marks, as the
// earlier version wrote it, where a question's words were its reason. A mark
// with no id is given one by its place in the list. It is what a test of
// reading or taking in leaves for the code to find, since nothing writes
// marks.json any more.
func marksJSON(t *testing.T, vaultRoot string, marks ...machinemarks.Mark) string {
	t.Helper()
	type earlier struct {
		machinemarks.Mark

		Reason string `json:"reason,omitempty"`
	}
	entries := make([]string, len(marks))
	for i := range marks {
		m := earlier{Mark: marks[i]}
		if m.ID == "" {
			m.ID = fmt.Sprintf("%032x", i+1)
		}
		if m.Kind == machinemarks.Question {
			m.Reason, m.Body = m.Body, ""
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = string(data)
	}
	return `{"version":1,"vault":` + strconv.Quote(vaultRoot) + `,"marks":[` + strings.Join(entries, ",") + `]}`
}

// aStoredQuestion is one question as marks.json holds it, with a distinct id.
func aStoredQuestion(i int) string {
	return fmt.Sprintf(`{"id":"%032x","kind":"question","path":"Notes/source.md","anchor":"a%d","at":"2026-09-22T10:00:00Z"}`, i+1, i)
}

// TestMachineMarksCarriesEveryFieldOfAStoredMark holds what the first write of a
// machine file takes in: every field of a mark of marks.json, with a question's
// reason as its words, in the order the file has them. The file is written out
// by hand, as the earlier version left it.
func TestMachineMarksCarriesEveryFieldOfAStoredMark(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	resolved := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	writeFile(t, marksPath(store), marksJSON(t, "/vaults/notes",
		*aHighlight(),
		machinemarks.Mark{
			Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "section-one", Quote: "words", Before: "a ", After: " b",
			Identity: anIdentity, Body: "why does it work?", Resolved: resolved, At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
		},
		machinemarks.Mark{
			Kind: machinemarks.Thought, Path: "Notes/source.md", Anchor: "s", Quote: "words", Before: "a ", After: " b",
			Identity: anIdentity, Body: "what I think", At: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		},
	))
	want := []machinemarks.Mark{
		{
			Kind: machinemarks.Highlight, Path: "Notes/source.md", Anchor: "section-one", Quote: "the topic particle",
			Before: "In Japanese, ", After: " marks what the sentence is about", Identity: anIdentity,
			At: time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC),
		},
		{
			Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "section-one", Quote: "words", Before: "a ", After: " b",
			Identity: anIdentity, Body: "why does it work?", Resolved: resolved, At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			Kind: machinemarks.Thought, Path: "Notes/source.md", Anchor: "s", Quote: "words", Before: "a ", After: " b",
			Identity: anIdentity, Body: "what I think", At: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		},
	}
	for i := range want {
		want[i].ID = fmt.Sprintf("%032x", i+1)
	}
	got, err := newStoreFor(t, filepath.Dir(filepath.Dir(store.Dir())), "/vaults/notes").MachineMarks()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("marks as a machine file takes them in (-want +got):\n%s", diff)
	}
}

// TestTheFilesNameTheirVault holds the half of the directory decision a person
// acts on. The directory is a digest and says nothing, so what tells them which
// vault they are looking at is inside the file.
func TestTheFilesNameTheirVault(t *testing.T) {
	t.Parallel()
	const root = "/vaults/notes"
	store := newStoreFor(t, t.TempDir(), root)
	if err := moveBookmark(store, mark.NoCourse, aPlace()); err != nil {
		t.Fatal(err)
	}
	data := contents(t, progressPath(store))
	var doc struct {
		Version int    `json:"version"`
		Vault   string `json:"vault"`
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Vault != root || doc.Version != 1 {
		t.Errorf("progress.json names vault %q at version %d, want %q at 1", doc.Vault, doc.Version, root)
	}
}

// TestNothingIsCreatedUntilSomethingIsKept holds the footprint: a reader who
// keeps nothing leaves nothing on their machine, and asking is not keeping.
func TestNothingIsCreatedUntilSomethingIsKept(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/notes")
	if held, err := store.MachineMarks(); err != nil || len(held) != 0 {
		t.Fatalf("MachineMarks() = %v, %v", held, err)
	}
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || found {
		t.Fatalf("Bookmark() = %v, %v", found, err)
	}
	if held, err := store.FinishedNotes(); err != nil || len(held) != 0 {
		t.Fatalf("FinishedNotes() = %v, %v", held, err)
	}
	if _, err := os.Stat(filepath.Join(config, "yomihon")); !os.IsNotExist(err) {
		t.Errorf("asking created %s", filepath.Join(config, "yomihon"))
	}
}

func TestTwoVaultsKeepTwoSetsOfProgress(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	first := newStoreFor(t, config, "/vaults/notes")
	second := newStoreFor(t, config, "/vaults/other")
	if first.Dir() == second.Dir() {
		t.Fatalf("two vaults share one directory at %s", first.Dir())
	}
	if err := moveBookmark(first, mark.NoCourse, aPlace()); err != nil {
		t.Fatal(err)
	}
	if err := first.SetFinished("Notes/a.md", true); err != nil {
		t.Fatal(err)
	}
	if held, found, err := second.Bookmark(mark.NoCourse); err != nil || found {
		t.Errorf("a bookmark kept for one vault is found for another: %v, %v, %v", held, found, err)
	}
	if held, err := second.FinishedNotes(); err != nil || len(held) != 0 {
		t.Errorf("a note finished in one vault is listed for another: %v, %v", held, err)
	}
}

func TestProgressSurvivesARestartAndDeletingTheFileClearsIt(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/one")
	kept := aPlace()
	if err := moveBookmark(store, mark.NoCourse, kept); err != nil {
		t.Fatal(err)
	}
	reopened := newStoreFor(t, config, "/vaults/one")
	held, found := bookmarkFor(t, reopened, mark.NoCourse)
	if !found {
		t.Fatal("a reopened store has no bookmark")
	}
	if diff := cmp.Diff(*kept, held); diff != "" {
		t.Fatalf("reopened bookmark (-want +got):\n%s", diff)
	}
	if err := os.Remove(progressPath(store)); err != nil {
		t.Fatal(err)
	}
	if held, found = bookmarkFor(t, reopened, mark.NoCourse); found {
		t.Fatalf("a deleted file still reports %v", held)
	}
}

func TestConcurrentMovesKeepEveryCourseBookmark(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	var writers sync.WaitGroup
	courses := []string{"Courses/one.md", "Courses/two.md", "Courses/three.md", "Courses/four.md"}
	for _, course := range courses {
		writers.Go(func() {
			if err := moveBookmark(store, course, aPlace()); err != nil {
				t.Errorf("move the bookmark of %s: %v", course, err)
			}
		})
	}
	writers.Wait()
	for _, course := range courses {
		if _, found := bookmarkFor(t, store, course); !found {
			t.Errorf("concurrent writes lost the bookmark of %s", course)
		}
	}
}

// TestAskingToTakeAwayWhatIsNotThereLeavesNoDirectory holds the footprint
// against the lock: clearing or unfinishing on a store that holds nothing is not
// keeping, and a directory made to hold the lock would be.
func TestAskingToTakeAwayWhatIsNotThereLeavesNoDirectory(t *testing.T) {
	t.Parallel()
	for name, ask := range map[string]func(*mark.Store) error{
		"clear a bookmark": func(s *mark.Store) error {
			_, cleared, err := s.ClearBookmark(mark.NoCourse)
			if cleared {
				return errors.New("cleared a bookmark in a store that holds none")
			}
			return err
		},
		"say a note is not finished": func(s *mark.Store) error { return s.SetFinished("Notes/a.md", false) },
		"move a bookmark that expects one kept": func(s *mark.Store) error {
			at := time.Now()
			_, _, err := s.MoveBookmark(mark.NoCourse, aPlace(), &mark.Expectation{At: &at})
			if !errors.Is(err, mark.ErrChanged) {
				return errors.Join(errors.New("a move that expects a bookmark in a store that holds none was not refused as ErrChanged"), err)
			}
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config := t.TempDir()
			if err := ask(newStoreFor(t, config, "/vaults/notes")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(config, "yomihon")); !os.IsNotExist(err) {
				t.Errorf("asking created %s", filepath.Join(config, "yomihon"))
			}
		})
	}
}

// pressedAgain makes a write the way a reader does while another process holds
// the lock: a refusal with ErrLocked is one the reader is shown, and they press
// the control again. Any other answer is the write's own.
func pressedAgain(write func() error) error {
	for {
		if err := write(); !errors.Is(err, confdir.ErrLocked) {
			return err
		}
	}
}

// missing is what of kept, the writes that answered nil, list does not hold.
func missing(kept [][]string, list []string) []string {
	held := map[string]bool{}
	for _, one := range list {
		held[one] = true
	}
	var absent []string
	for _, one := range slices.Concat(kept...) {
		if !held[one] {
			absent = append(absent, one)
		}
	}
	return absent
}

// TestTwoStoresOnOneDirectoryKeepEveryBookmark is the command line and the
// running server: two values over the same files, each with a lock of its own,
// whose changes must not write one another away. A write that answered nil is in
// the file; how long the other store kept the lock is not what this holds.
func TestTwoStoresOnOneDirectoryKeepEveryBookmark(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	const each = 100
	kept := make([][]string, 2)
	var writers sync.WaitGroup
	for w := range 2 {
		store := newStoreFor(t, config, "/vaults/notes")
		writers.Go(func() {
			for i := range each {
				course := fmt.Sprintf("Courses/w%d-%d.md", w, i)
				if err := pressedAgain(func() error { return moveBookmark(store, course, aPlace()) }); err != nil {
					t.Errorf("writer %d bookmark %d: %v", w, i, err)
					return
				}
				kept[w] = append(kept[w], course)
			}
		})
	}
	writers.Wait()
	reader := newStoreFor(t, config, "/vaults/notes")
	var courses []string
	for _, one := range slices.Concat(kept...) {
		if _, found := bookmarkFor(t, reader, one); found {
			courses = append(courses, one)
		}
	}
	if absent := missing(kept, courses); len(absent) > 0 {
		t.Errorf("%d bookmarks answered kept are not in the file, first %s", len(absent), absent[0])
	}
	if len(courses) != 2*each {
		t.Errorf("two stores kept %d bookmarks; want %d", len(courses), 2*each)
	}
}

// TestAChangeThatCannotTakeTheLockIsRefusedAndChangesNothing is the other half
// of pressedAgain: another process holds the lock past the wait, and the change
// answers ErrLocked, which the reader is shown, and leaves the file as it was.
func TestAChangeThatCannotTakeTheLockIsRefusedAndChangesNothing(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/notes")
	if err := store.SetFinished("Notes/one.md", true); err != nil {
		t.Fatal(err)
	}
	before := directoryBytes(t, store.Dir())
	dir, err := confdir.VaultDir(confdir.At(config), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	held, err := confdir.Acquire(t.Context(), dir, "marks.lock", 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() }) //nolint:errcheck // the temporary directory goes with it

	if err := store.SetFinished("Notes/another.md", true); !errors.Is(err, confdir.ErrLocked) {
		t.Fatalf("SetFinished while another process holds the lock past the wait = %v, want ErrLocked", err)
	}
	if diff := cmp.Diff(before, directoryBytes(t, store.Dir())); diff != "" {
		t.Errorf("a refused change changed a file (-before +after):\n%s", diff)
	}
}

// A damaged file is kept as it is and reported, whatever is done next: the
// reader can recover what is in it, and nothing built over it. The file is read
// for the marks the first write of a machine file takes in, and what cannot be
// read is not read as nothing.
func TestAMarksFileThatCannotBeReadIsReportedAndLeftAsItIs(t *testing.T) {
	t.Parallel()
	good := aStoredQuestion(1)
	for _, damaged := range []string{
		"", "[", "null", "{}", `{"version":1}`, `{"version":1,"vault":"/v","marks":null}`,
		`{"version":2,"vault":"/v","marks":[]}`,
		`{"version":1,"vault":"/v","marks":[]} {}`,
		`{"version":1,"vault":"/v","marks":[],"extra":1}`,
		aMarksFile(good, good),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"a.md","at":"invalid"}`),
	} {
		t.Run(damaged, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			writeFile(t, marksPath(store), damaged)
			if _, err := store.MachineMarks(); !errors.Is(err, mark.ErrUnreadable) {
				t.Fatalf("MachineMarks over a damaged file = %v; want ErrUnreadable", err)
			}
			if data := contents(t, marksPath(store)); data != damaged {
				t.Fatalf("the damaged file changed to %q", data)
			}
		})
	}
}

func TestAMarksFileThatIsNotAFileIsReportedNotReadAsEmpty(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if err := os.MkdirAll(marksPath(store), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MachineMarks(); !errors.Is(err, mark.ErrUnreadable) {
		t.Fatalf("MachineMarks over a directory = %v; want ErrUnreadable", err)
	}
}

// The count is held on read: a file that holds more marks than the writer kept
// was not written by it.
func TestMarksBeyondTheCountAreDamagedWhenStored(t *testing.T) {
	t.Parallel()
	full := make([]string, machinemarks.MaxLive)
	for i := range full {
		full[i] = aStoredQuestion(i)
	}
	store := newStore(t)
	writeFile(t, marksPath(store), aMarksFile(full...))
	if held, err := store.MachineMarks(); err != nil || len(held) != machinemarks.MaxLive {
		t.Fatalf("a file at the cap read as %d marks, %v", len(held), err)
	}

	tooMany := newStore(t)
	writeFile(t, marksPath(tooMany), aMarksFile(slices.Concat(full, []string{aStoredQuestion(machinemarks.MaxLive)})...))
	if held, err := tooMany.MachineMarks(); !errors.Is(err, mark.ErrUnreadable) {
		t.Errorf("one mark past the cap read as %d marks, %v; want ErrUnreadable", len(held), err)
	}
}

// A file longer than anything the writer installs is read as damaged, left as
// it is, and never taken in whole. Every file is read through confdir's cap,
// which confdir's tests hold; this is the one the store reads with the least.
func TestAFileBeyondItsReadCapIsUnreadableAndKept(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	path := filepath.Join(store.Dir(), "reader.json")
	data := `{"version":1,"pad":"` + strings.Repeat("x", 65<<10) + `"}`
	writeFile(t, path, data)
	if _, _, err := store.Bookmark(mark.NoCourse); !errors.Is(err, mark.ErrUnreadable) {
		t.Errorf("reading reader.json over its cap = %v, want ErrUnreadable", err)
	}
	if contents(t, path) != data {
		t.Error("reader.json was changed")
	}
}
