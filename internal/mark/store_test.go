package mark_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/mark"
)

// withoutIDs compares marks by what they say: an id is made when a mark is kept
// and is not something a test can write in advance.
var withoutIDs = cmpopts.IgnoreFields(mark.Mark{}, "ID")

func marksPath(store *mark.Store) string { return filepath.Join(store.Dir(), "marks.json") }

func progressPath(store *mark.Store) string { return filepath.Join(store.Dir(), "progress.json") }

// writeFile puts bytes where a store looks, as a reader or an earlier version
// would have left them.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// aMarksFile is a marks.json holding the given marks, written by hand so that
// a test of reading does not lean on the code that writes.
func aMarksFile(marks ...string) string {
	return `{"version":1,"vault":"/vaults/notes","marks":[` + strings.Join(marks, ",") + `]}`
}

// aStoredQuestion is one question as marks.json holds it, with a distinct id.
func aStoredQuestion(i int) string {
	return fmt.Sprintf(`{"id":"%032x","kind":"question","path":"Notes/source.md","anchor":"a%d","at":"2026-09-22T10:00:00Z"}`, i+1, i)
}

func TestToggleTouchesOnlyTheChosenNoteAndAnchor(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	empty, err := store.List()
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("missing file = %v, %v; want empty non-nil list", empty, err)
	}
	first := aQuestion()
	second := *first
	second.Anchor = "section-two"
	third := *first
	third.RelPath = "Notes/other.md"
	for _, kept := range []*mark.Mark{first, &second, &third} {
		if added, toggleErr := store.Toggle(kept, admitAny); toggleErr != nil || !added {
			t.Fatalf("add %v = %v, %v", kept, added, toggleErr)
		}
	}
	// Time is not identity: revisiting the first place removes it.
	first.At = first.At.Add(time.Hour)
	if added, toggleErr := store.Toggle(first, admitAny); toggleErr != nil || added {
		t.Fatalf("remove first = %v, %v", added, toggleErr)
	}
	held, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Mark{second, third}, held, withoutIDs); diff != "" {
		t.Fatalf("unrelated marks changed (-want +got):\n%s", diff)
	}
	for _, kept := range []*mark.Mark{&second, &third} {
		if added, toggleErr := store.Toggle(kept, admitAny); toggleErr != nil || added {
			t.Fatalf("remove %v = %v, %v", kept, added, toggleErr)
		}
	}
	var doc struct{ Marks []mark.Mark }
	data, err := os.ReadFile(marksPath(store))
	if err != nil || json.Unmarshal(data, &doc) != nil || doc.Marks == nil || len(doc.Marks) != 0 {
		t.Fatalf("cleared file = %q, %v; want an empty list of marks", data, err)
	}
}

// A highlight and a question about the same words are two marks; a question
// about a section and one about words in it are two marks.
func TestToggleSeparatesKindsAndQuotes(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	section := aQuestion()
	span := aQuestion()
	span.Quote = "the topic particle"
	highlight := aHighlight()
	highlight.Anchor = section.Anchor
	span.Anchor = section.Anchor
	for _, kept := range []*mark.Mark{section, span, highlight} {
		if added, err := store.Toggle(kept, admitAny); err != nil || !added {
			t.Fatalf("add %v = %v, %v", kept, added, err)
		}
	}
	if held, err := store.List(); err != nil || len(held) != 3 {
		t.Fatalf("List() = %d marks, %v; want 3", len(held), err)
	}
	if held, err := store.List(mark.Highlight); err != nil || len(held) != 1 || held[0].Kind != mark.Highlight {
		t.Errorf("List(Highlight) = %v, %v", held, err)
	}
	if held, err := store.List(mark.Question, mark.Thought); err != nil || len(held) != 2 {
		t.Errorf("List(Question, Thought) = %d marks, %v; want 2", len(held), err)
	}
}

func TestAddKeepsAMarkWithAnIdAndRemoveHandsItBack(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	highlight := aHighlight()
	kept, err := store.Add(highlight)
	if err != nil {
		t.Fatal(err)
	}
	if highlight.ID != "" {
		t.Error("Add wrote through the pointer it was given")
	}
	if len(kept.ID) != 32 {
		t.Errorf("Add gave the mark the id %q, want 32 hex digits", kept.ID)
	}
	other, err := store.Add(highlight)
	if err != nil || other.ID == kept.ID {
		t.Fatalf("a second mark = %v, %v; want another id", other.ID, err)
	}
	removed, found, err := store.Remove(kept.ID)
	if err != nil || !found {
		t.Fatalf("Remove = %v, %v", found, err)
	}
	if diff := cmp.Diff(kept, removed); diff != "" {
		t.Errorf("Remove did not hand back what was kept (-want +got):\n%s", diff)
	}
	if again, found, removeErr := store.Remove(kept.ID); removeErr != nil || found || again.ID != "" {
		t.Errorf("Remove of nothing = %v, %v, %v", again, found, removeErr)
	}
	// What was handed back can be offered back: it keeps its id.
	if restored, addErr := store.Add(&removed); addErr != nil || restored.ID != kept.ID {
		t.Errorf("Add of a removed mark = %v, %v; want the same id", restored.ID, addErr)
	}
	if _, addErr := store.Add(&removed); !errors.Is(addErr, mark.ErrInvalid) {
		t.Errorf("Add of an id already kept = %v, want ErrInvalid", addErr)
	}
}

func TestEveryFieldOfAMarkSurvivesAWriteAndARead(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	thought := &mark.Mark{
		Kind: mark.Thought, RelPath: "Notes/source.md", Anchor: "s", Quote: "words", Before: "a ", After: " b",
		Identity: anIdentity, Body: "what I think", At: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
	}
	question := aQuestion()
	question.Reason = "why does it work?"
	question.ResolvedAt = time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	want := []mark.Mark{*aHighlight(), *question, *thought}
	for i := range want {
		if _, err := store.Add(&want[i]); err != nil {
			t.Fatal(err)
		}
	}
	got, err := newStoreFor(t, filepath.Dir(filepath.Dir(store.Dir())), "/vaults/notes").List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, withoutIDs); diff != "" {
		t.Errorf("marks after a restart (-want +got):\n%s", diff)
	}
}

func TestAFileIsOwnerOnlyAndNothingIsLeftBesideIt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX permission bits to read back")
	}
	store := newStore(t)
	for range 3 {
		if _, err := store.Add(aHighlight()); err != nil {
			t.Fatal(err)
		}
		if err := store.SetBookmark(mark.NoCourse, aPlace()); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if diff := cmp.Diff([]string{"marks.json", "progress.json"}, names); diff != "" {
		t.Errorf("the directory holds more than the two files (-want +got):\n%s", diff)
	}
	for _, path := range []string{marksPath(store), progressPath(store)} {
		if info, statErr := os.Stat(path); statErr != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, %v; want 600", filepath.Base(path), info, statErr)
		}
	}
	if info, statErr := os.Stat(store.Dir()); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the directory mode = %v, %v; want 700", info, statErr)
	}
}

// TestTheFilesNameTheirVault holds the half of the directory decision a person
// acts on. The directory is a digest and says nothing, so what tells them which
// vault they are looking at is inside the file.
func TestTheFilesNameTheirVault(t *testing.T) {
	t.Parallel()
	const root = "/vaults/notes"
	store := newStoreFor(t, t.TempDir(), root)
	if _, err := store.Add(aHighlight()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBookmark(mark.NoCourse, aPlace()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{marksPath(store), progressPath(store)} {
		data, err := os.ReadFile(path) // #nosec G304 -- a file a test wrote under its own temporary directory
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Version int    `json:"version"`
			Vault   string `json:"vault"`
		}
		if err = json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Vault != root || doc.Version != 1 {
			t.Errorf("%s names vault %q at version %d, want %q at 1", filepath.Base(path), doc.Vault, doc.Version, root)
		}
	}
}

// TestNothingIsCreatedUntilAMarkIsKept holds the footprint: a reader who keeps
// nothing leaves nothing on their machine, and asking is not keeping.
func TestNothingIsCreatedUntilAMarkIsKept(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/notes")
	if held, err := store.List(); err != nil || len(held) != 0 {
		t.Fatalf("List() = %v, %v", held, err)
	}
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || found {
		t.Fatalf("Bookmark() = %v, %v", found, err)
	}
	if store.Stored() {
		t.Error("a store nobody wrote to reports a stored file")
	}
	if _, err := os.Stat(filepath.Join(config, "yomihon")); !os.IsNotExist(err) {
		t.Errorf("asking created %s", filepath.Join(config, "yomihon"))
	}
}

func TestTwoVaultsKeepTwoSetsOfMarks(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	first := newStoreFor(t, config, "/vaults/notes")
	second := newStoreFor(t, config, "/vaults/other")
	if first.Dir() == second.Dir() {
		t.Fatalf("two vaults share one directory at %s", first.Dir())
	}
	if _, err := first.Add(aHighlight()); err != nil {
		t.Fatal(err)
	}
	if held, err := second.List(); err != nil || len(held) != 0 {
		t.Errorf("a mark kept for one vault is listed for another: %v, %v", held, err)
	}
}

func TestMarksSurviveARestartAndDeletingTheFileClearsThem(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store := newStoreFor(t, config, "/vaults/one")
	kept := aQuestion()
	kept.Anchor = ""
	if _, err := store.Toggle(kept, admitAny); err != nil {
		t.Fatal(err)
	}
	reopened := newStoreFor(t, config, "/vaults/one")
	held, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Mark{*kept}, held, withoutIDs); diff != "" {
		t.Fatalf("reopened marks (-want +got):\n%s", diff)
	}
	if err = os.Remove(marksPath(store)); err != nil {
		t.Fatal(err)
	}
	if held, err = reopened.List(); err != nil || len(held) != 0 {
		t.Fatalf("a deleted file still reports %v, %v", held, err)
	}
}

func TestConcurrentTogglesKeepEveryDistinctPlace(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	var writers sync.WaitGroup
	for _, anchor := range []string{"one", "two", "three", "four"} {
		writers.Go(func() {
			kept := aQuestion()
			kept.Anchor = anchor
			if _, err := store.Toggle(kept, admitAny); err != nil {
				t.Errorf("toggle %s: %v", anchor, err)
			}
		})
	}
	writers.Wait()
	if held, err := store.List(); err != nil || len(held) != 4 {
		t.Fatalf("concurrent writes retained %v, %v; want all four", held, err)
	}
}

// TestTwoStoresOnOneDirectoryKeepEveryMark is the command line and the running
// server: two values over the same files, each with a lock of its own, whose
// changes must not write one another away.
func TestTwoStoresOnOneDirectoryKeepEveryMark(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	const each = 100
	var writers sync.WaitGroup
	for w := range 2 {
		store := newStoreFor(t, config, "/vaults/notes")
		writers.Go(func() {
			for i := range each {
				kept := aQuestion()
				kept.Anchor = fmt.Sprintf("w%d-%d", w, i)
				if _, err := store.Add(kept); err != nil {
					t.Errorf("writer %d add %d: %v", w, i, err)
					return
				}
			}
		})
	}
	writers.Wait()
	held, err := newStoreFor(t, config, "/vaults/notes").List()
	if err != nil || len(held) != 2*each {
		t.Fatalf("two stores kept %d marks, %v; want %d", len(held), err, 2*each)
	}
}

// TestTwoStoresOnOneDirectoryKeepEveryFinishedNote is the same loss in the file
// of what a reader has finished, which a mark's change never touches.
func TestTwoStoresOnOneDirectoryKeepEveryFinishedNote(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	const each = 100
	var writers sync.WaitGroup
	for w := range 2 {
		store := newStoreFor(t, config, "/vaults/notes")
		writers.Go(func() {
			for i := range each {
				if err := store.SetFinished(fmt.Sprintf("Notes/w%d-%d.md", w, i), true); err != nil {
					t.Errorf("writer %d finish %d: %v", w, i, err)
					return
				}
			}
		})
	}
	writers.Wait()
	finished, err := newStoreFor(t, config, "/vaults/notes").FinishedNotes()
	if err != nil || len(finished) != 2*each {
		t.Fatalf("two stores kept %d finished notes, %v; want %d", len(finished), err, 2*each)
	}
}

func TestToggleAdmitsOnlyWhenAddingAndWithoutAnAdmitterRefuses(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	if _, err := store.Toggle(aQuestion(), nil); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("no admitter error = %v, want ErrInvalid", err)
	}
	if _, err := store.Toggle(aQuestion(), admitAny); err != nil {
		t.Fatal(err)
	}
	// The place has since gone; clearing the mark must still work.
	refuse := func(*mark.Mark) bool { return false }
	if added, err := store.Toggle(aQuestion(), refuse); err != nil || added {
		t.Fatalf("clearing a stale mark = %v, %v", added, err)
	}
}

func TestAnInvalidMarkIsRefusedBeforeStorageIsRead(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	// A directory where the file should be makes any read fail with another
	// error, so only a refusal that came first can be ErrInvalid.
	if err := os.MkdirAll(marksPath(store), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []*mark.Mark{nil, {Kind: mark.Question, RelPath: "", At: time.Now()}} {
		if _, err := store.Toggle(kept, admitAny); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("Toggle(%v) = %v, want ErrInvalid", kept, err)
		}
		if _, err := store.Add(kept); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("Add(%v) = %v, want ErrInvalid", kept, err)
		}
	}
}

// A damaged file is kept as it is and reported, whatever the next mark is: the
// reader can recover what is in it, and no mark is a way to lose it.
func TestAMarksFileThatCannotBeReadIsReportedAndNeverReplaced(t *testing.T) {
	t.Parallel()
	good := aStoredQuestion(1)
	for _, damaged := range []string{
		"", "[", "null", "{}", `{"version":1}`, `{"version":1,"vault":"/v","marks":null}`,
		`{"version":2,"vault":"/v","marks":[]}`,
		`{"version":1,"vault":"/v","marks":[]} {}`,
		`{"version":1,"vault":"/v","marks":[],"extra":1}`,
		aMarksFile(good, good),
		aMarksFile(`{"id":"1","kind":"question","path":"Notes/source.md","at":"2026-09-22T10:00:00Z"}`),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"../source.md","at":"2026-09-22T10:00:00Z"}`),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"note","path":"a.md","at":"2026-09-22T10:00:00Z"}`),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"a.md","at":"invalid"}`),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"Source.md\n2026-12-31  thought   Fake.md  Forged  (started)","at":"2026-09-22T10:00:00Z"}`),
		aMarksFile(`{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"Notes/\u001b]0;pwned\u0007.md","at":"2026-09-22T10:00:00Z"}`),
	} {
		t.Run(damaged, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			writeFile(t, marksPath(store), damaged)
			if _, err := store.List(); !errors.Is(err, mark.ErrUnreadable) {
				t.Fatalf("List over a damaged file = %v; want ErrUnreadable", err)
			}
			if !store.Stored() {
				t.Error("a damaged file is not stored")
			}
			for name, err := range map[string]error{
				"Toggle": func() error { _, err := store.Toggle(aQuestion(), admitAny); return err }(),
				"Add":    func() error { _, err := store.Add(aHighlight()); return err }(),
				"Remove": func() error { _, _, err := store.Remove(strings.Repeat("a", 32)); return err }(),
			} {
				if !errors.Is(err, mark.ErrUnreadable) || errors.Is(err, mark.ErrInvalid) {
					t.Errorf("%s over a damaged file = %v; want ErrUnreadable and not ErrInvalid", name, err)
				}
			}
			data, err := os.ReadFile(marksPath(store))
			if err != nil || string(data) != damaged {
				t.Fatalf("the damaged file changed to %q: %v", data, err)
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
	if _, err := store.List(); !errors.Is(err, mark.ErrUnreadable) {
		t.Fatalf("List over a directory = %v; want ErrUnreadable", err)
	}
	if _, err := store.Toggle(aQuestion(), admitAny); !errors.Is(err, mark.ErrUnreadable) {
		t.Fatalf("Toggle over a directory = %v; want ErrUnreadable", err)
	}
}

func TestAnInvalidUTF8MarkIsRefusedEveryTimeAndTheFileStaysReadable(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	// JSON stores an invalid byte as U+FFFD, so two identical requests used to
	// write the same pair twice and make every later read fail as a duplicate.
	for range 2 {
		kept := aQuestion()
		kept.Anchor = "x\xff"
		if _, err := store.Toggle(kept, admitAny); !errors.Is(err, mark.ErrInvalid) {
			t.Fatalf("Toggle(invalid UTF-8 anchor) = %v, want ErrInvalid", err)
		}
	}
	if held, err := store.List(); err != nil || len(held) != 0 {
		t.Fatalf("List() = %v, %v; want a readable empty list", held, err)
	}
	if _, err := store.Toggle(aQuestion(), admitAny); err != nil {
		t.Fatalf("a later valid toggle failed: %v", err)
	}
}

// The count is held on write and on read: a file that holds more marks than the
// writer keeps was not written by it.
func TestMarksBeyondTheCountAreRefusedWhenAddedAndDamagedWhenStored(t *testing.T) {
	t.Parallel()
	full := make([]string, mark.MaxMarks)
	for i := range full {
		full[i] = aStoredQuestion(i)
	}
	store := newStore(t)
	writeFile(t, marksPath(store), aMarksFile(full...))
	if held, err := store.List(); err != nil || len(held) != mark.MaxMarks {
		t.Fatalf("a file at the cap read as %d marks, %v", len(held), err)
	}
	over := aQuestion()
	over.Anchor = "over"
	if _, err := store.Toggle(over, admitAny); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("a mark past the cap = %v, want ErrInvalid", err)
	}
	if _, err := store.Add(aHighlight()); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("a highlight past the cap = %v, want ErrInvalid", err)
	}
	// Clearing at the cap still works, and makes room.
	first := aQuestion()
	first.Anchor = "a0"
	if added, err := store.Toggle(first, admitAny); err != nil || added {
		t.Fatalf("clearing at the cap = %v, %v", added, err)
	}
	if added, err := store.Toggle(over, admitAny); err != nil || !added {
		t.Fatalf("adding after room was made = %v, %v", added, err)
	}

	tooMany := newStore(t)
	writeFile(t, marksPath(tooMany), aMarksFile(slices.Concat(full, []string{aStoredQuestion(mark.MaxMarks)})...))
	if held, err := tooMany.List(); !errors.Is(err, mark.ErrUnreadable) {
		t.Errorf("one mark past the cap read as %d marks, %v; want ErrUnreadable", len(held), err)
	}
}

// Every read has a cap. A file longer than anything the writer installs is read
// as damaged, left as it is, and never taken in whole.
func TestAFileBeyondItsReadCapIsUnreadableAndKept(t *testing.T) {
	t.Parallel()
	pad := strings.Repeat("x", 17<<20)
	for _, tt := range []struct {
		name string
		path func(*mark.Store) string
		data string
		read func(*mark.Store) error
	}{
		{"the marks", marksPath, `{"version":1,"vault":"/v","marks":[],"pad":"` + pad + `"}`, func(s *mark.Store) error { _, err := s.List(); return err }},
		{"the progress", progressPath, `{"version":1,"vault":"/v","bookmarks":[],"pad":"` + pad[:2<<20] + `"}`, func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err }},
		{"the old bookmark", func(s *mark.Store) string { return filepath.Join(s.Dir(), "reader.json") }, `{"version":1,"pad":"` + pad[:200<<10] + `"}`, func(s *mark.Store) error { _, _, err := s.Bookmark(mark.NoCourse); return err }},
		{"the old questions", func(s *mark.Store) string { return filepath.Join(s.Dir(), "uncertainty.json") }, "[" + strings.Repeat(" ", 5<<20) + "]", func(s *mark.Store) error { _, err := s.List(); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			writeFile(t, tt.path(store), tt.data)
			if err := tt.read(store); !errors.Is(err, mark.ErrUnreadable) {
				t.Errorf("reading %s over its cap = %v, want ErrUnreadable", tt.name, err)
			}
			if after, err := os.ReadFile(tt.path(store)); err != nil || !bytes.Equal(after, []byte(tt.data)) {
				t.Errorf("%s was changed: %v", tt.name, err)
			}
		})
	}
}

// What is written is what the next read takes: a mark that would make the file
// longer than the read cap is refused, rather than written and then unreadable.
func TestAWriteThatTheNextReadWouldRefuseIsRefused(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	// The marks file's read cap. A change to it makes this test fail rather than
	// pass quietly: a file over the cap is not one the store starts from, and one
	// far under it is never refused.
	const readCap = 16 << 20
	body := strings.Repeat("b", 32<<10)
	entry := func(i int) string {
		return fmt.Sprintf(`{"id":"%032x","kind":"thought","path":"Notes/source.md","anchor":"a%d","body":%q,"at":"2026-09-22T10:00:00Z"}`, i+1, i, body)
	}
	// Start about two entries short of the cap, so the refusal comes within a few
	// writes of a file this size.
	held := make([]string, (readCap-3*len(entry(0)))/(len(entry(0))+1))
	for i := range held {
		held[i] = entry(i)
	}
	writeFile(t, marksPath(store), aMarksFile(held...))
	if _, err := store.List(); err != nil {
		t.Fatalf("the starting file is not one the store reads: %v", err)
	}
	var refusal error
	for i := range 6 {
		_, refusal = store.Add(&mark.Mark{Kind: mark.Thought, RelPath: "Notes/source.md", Anchor: "z" + strconv.Itoa(i), Body: body, At: time.Now()})
		if refusal != nil {
			break
		}
	}
	if !errors.Is(refusal, mark.ErrInvalid) {
		t.Fatalf("filling the file past its cap ended with %v, want ErrInvalid", refusal)
	}
	if _, err := store.List(); err != nil {
		t.Errorf("after the refusal the file cannot be read: %v", err)
	}
}

func TestAHighlightIsKeptWhereItIsToggledOnlyByItsWords(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, quote := range []string{"one", "two"} {
		highlight := aHighlight()
		highlight.Quote = quote
		if added, err := store.Toggle(highlight, admitAny); err != nil || !added {
			t.Fatalf("Toggle(%q) = %v, %v", quote, added, err)
		}
	}
	if held, err := store.List(mark.Highlight); err != nil || len(held) != 2 {
		t.Fatalf("two highlights in one section = %d, %v; want 2", len(held), err)
	}
}
