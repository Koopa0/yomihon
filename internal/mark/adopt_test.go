package mark_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/ledger"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
)

func admitEverything(string) bool { return true }

// aMovedVault is one reader's configuration directory holding what was kept for
// a vault at its old path, and a store for the same vault at the path it has
// now, which has kept nothing yet.
type aMovedVault struct {
	config, oldRoot, newRoot string
	old, now                 *mark.Store
}

func movedVault(t *testing.T) aMovedVault {
	t.Helper()
	root := t.TempDir()
	v := aMovedVault{
		config:  filepath.Join(root, "config"),
		oldRoot: filepath.Join(root, "before", "notes"),
		newRoot: filepath.Join(root, "after", "notes"),
	}
	v.old = newStoreFor(t, v.config, v.oldRoot)
	v.now = newStoreFor(t, v.config, v.newRoot)
	if err := os.MkdirAll(v.newRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	return v
}

// adopt is what the marks command does when it is asked to adopt: Adopt, and
// the vault's marks taking in the marks it answers.
func (v aMovedVault) adopt(t *testing.T, admit func(string) bool) error {
	t.Helper()
	marks, err := v.now.Adopt(v.oldRoot, admit)
	if err != nil {
		return err
	}
	kept := v.kept(t)
	return errors.Join(kept.TakeIn(t.Context(), marks), kept.Close())
}

// kept is the marks of the vault at the path it has now.
func (v aMovedVault) kept(t *testing.T) *mark.Kept {
	t.Helper()
	reader, err := vault.Open(v.newRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // the test is over
	return mark.OpenKept(v.now, reader, slog.New(slog.DiscardHandler))
}

// inVault is the marks the reader has in the vault at the path it has now, the
// ones not deleted, by id: what its machine files hold and what its earlier file
// still would give the first write.
func (v aMovedVault) inVault(t *testing.T) []machinemarks.Mark {
	t.Helper()
	kept := v.kept(t)
	defer func() { _ = kept.Close() }() //nolint:errcheck // the test is over
	view, err := kept.View()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := kept.Pending(view)
	if err != nil {
		t.Fatal(err)
	}
	held := pending
	for _, e := range view.Entries(mark.AllMarks) {
		if !e.Deleted() {
			held = append(held, e.Mark())
		}
	}
	slices.SortFunc(held, func(a, b machinemarks.Mark) int { return strings.Compare(a.ID, b.ID) })
	return held
}

// keepEverything leaves one of each thing in the old vault's directory: a
// highlight, a question and a thought in marks.json, a bookmark for no course and
// one for a course, and two finished notes.
func (v aMovedVault) keepEverything(t *testing.T) {
	t.Helper()
	thought := aHighlight()
	thought.Kind, thought.Body, thought.Quote, thought.Anchor = machinemarks.Thought, "this is why", "second", "section-two"
	// Nothing writes marks.json any more, so the marks of the earlier version
	// are left as it left them, by hand.
	writeFile(t, marksPath(v.old), marksJSON(t, v.oldRoot, *aHighlight(), *aQuestion(), *thought))
	if err := moveBookmark(v.old, mark.NoCourse, aPlace()); err != nil {
		t.Fatal(err)
	}
	course := aPlace()
	course.RelPath = "Courses/go/two.md"
	if err := moveBookmark(v.old, "Courses/go/index.md", course); err != nil {
		t.Fatal(err)
	}
	for _, note := range []string{"Notes/source.md", "Courses/go/one.md"} {
		if err := v.old.SetFinished(note, true); err != nil {
			t.Fatal(err)
		}
	}
}

// directoryBytes is every file of dir by name, for telling that nothing in it
// changed or appeared. The lock is left out: the first change makes it, empty,
// and it stays whether the change writes anything or not.
func directoryBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}
	}
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]string{}
	for _, entry := range entries {
		if entry.Name() == "marks.lock" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- a file of this test's own temporary directory
		if err != nil {
			t.Fatal(err)
		}
		held[entry.Name()] = string(data)
	}
	return held
}

// bookmarkFor is the bookmark kept for course and whether there is one, failing
// the test when the file cannot be read.
func bookmarkFor(t *testing.T, store *mark.Store, course string) (mark.Bookmark, bool) {
	t.Helper()
	held, found, err := store.Bookmark(course)
	if err != nil {
		t.Fatalf("Bookmark(%q): %v", course, err)
	}
	return held, found
}

// finishedFor is every finished note, failing the test when the file cannot be
// read.
func finishedFor(t *testing.T, store *mark.Store) []mark.Finished {
	t.Helper()
	held, err := store.FinishedNotes()
	if err != nil {
		t.Fatalf("FinishedNotes: %v", err)
	}
	return held
}

// finishedAtFor is when note was finished, failing the test when it is not or
// the file cannot be read.
func finishedAtFor(t *testing.T, store *mark.Store, note string) time.Time {
	t.Helper()
	at, found, err := store.FinishedAt(note)
	if err != nil || !found {
		t.Fatalf("FinishedAt(%q) = %v, %v, %v", note, at, found, err)
	}
	return at
}

// listedMarks is the marks of marks.json, as a machine file takes them in.
func listedMarks(t *testing.T, store *mark.Store) []machinemarks.Mark {
	t.Helper()
	held, err := store.MachineMarks()
	if err != nil {
		t.Fatalf("MachineMarks: %v", err)
	}
	return held
}

func TestAdoptBringsAcrossWhatWasKeptForTheOldPath(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	want := listedMarks(t, v.old)
	slices.SortFunc(want, func(a, b machinemarks.Mark) int { return strings.Compare(a.ID, b.ID) })
	if diff := cmp.Diff(want, v.inVault(t)); diff != "" {
		t.Errorf("the marks are not the old vault's (-old +now):\n%s", diff)
	}
	for _, course := range []string{mark.NoCourse, "Courses/go/index.md"} {
		want, _ := bookmarkFor(t, v.old, course)
		got, found := bookmarkFor(t, v.now, course)
		if !found {
			t.Fatalf("the bookmark for %q is not there", course)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("the bookmark for %q (-old +now):\n%s", course, diff)
		}
	}
	if diff := cmp.Diff(finishedFor(t, v.old), finishedFor(t, v.now)); diff != "" {
		t.Errorf("the finished notes (-old +now):\n%s", diff)
	}
}

// Running it twice, as a reader who is unsure it worked would, changes nothing
// the second time.
func TestAdoptTwiceChangesNothingTheSecondTime(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("the first Adopt: %v", err)
	}
	after, vaultAfter := directoryBytes(t, v.now.Dir()), tree(t, v.newRoot)
	if len(after) == 0 {
		t.Fatal("the first Adopt wrote nothing, so the second proves nothing")
	}
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("the second Adopt: %v", err)
	}
	if diff := cmp.Diff(after, directoryBytes(t, v.now.Dir())); diff != "" {
		t.Errorf("the second Adopt changed a file (-after the first +after the second):\n%s", diff)
	}
	if diff := cmp.Diff(vaultAfter, tree(t, v.newRoot)); diff != "" {
		t.Errorf("the second Adopt changed the vault (-after the first +after the second):\n%s", diff)
	}
}

// The old directory is where the reader's marks were left, and the earlier
// version of yomihon that wrote them may still be run against it.
func TestAdoptLeavesTheOldDirectoryAsItWas(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	before := directoryBytes(t, v.old.Dir())
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if diff := cmp.Diff(before, directoryBytes(t, v.old.Dir())); diff != "" {
		t.Errorf("the old directory changed (-before +after):\n%s", diff)
	}
}

// What this vault has kept since it moved is the reader's newer word, and
// stays; only what it does not have comes across.
func TestAdoptKeepsWhatTheVaultAlreadyHas(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)

	ownHighlight := *aHighlight()
	ownHighlight.ID = markID(10)
	writeFile(t, marksPath(v.now), marksJSON(t, v.newRoot, ownHighlight))
	ownBookmark := aPlace()
	ownBookmark.Anchor, ownBookmark.Offset = "a-newer-place", 7
	if err := moveBookmark(v.now, mark.NoCourse, ownBookmark); err != nil {
		t.Fatal(err)
	}
	if err := v.now.SetFinished("Notes/source.md", true); err != nil {
		t.Fatal(err)
	}
	ownFinishedAt := finishedAtFor(t, v.now, "Notes/source.md")

	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	held := v.inVault(t)
	if len(held) != 3 || !slices.ContainsFunc(held, func(m machinemarks.Mark) bool { return m.ID == ownHighlight.ID }) {
		t.Errorf("the marks are %+v, want 3: the highlight this vault has, and the question and thought it has not", held)
	}
	if got, _ := bookmarkFor(t, v.now, mark.NoCourse); got.Anchor != "a-newer-place" {
		t.Errorf("the bookmark this vault kept was replaced by %+v", got)
	}
	if got, _ := bookmarkFor(t, v.now, "Courses/go/index.md"); got.RelPath != "Courses/go/two.md" {
		t.Errorf("the bookmark for the course it had none for did not come across: %+v", got)
	}
	if at := finishedAtFor(t, v.now, "Notes/source.md"); !at.Equal(ownFinishedAt) {
		t.Errorf("a note finished here is finished at %v, want the time it was finished here, %v", at, ownFinishedAt)
	}
	if finished := finishedFor(t, v.now); len(finished) != 2 {
		t.Errorf("%d notes are finished, want 2", len(finished))
	}
}

// Two thoughts in one place say two things.
func TestAdoptDoesNotTakeAThoughtForTheOneAlreadyThere(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	thought := func(body string) *machinemarks.Mark {
		m := aHighlight()
		m.Kind, m.Body = machinemarks.Thought, body
		return m
	}
	older, newer := thought("what the old one said"), thought("what this one says")
	older.ID, newer.ID = markID(1), markID(2)
	writeFile(t, marksPath(v.old), marksJSON(t, v.oldRoot, *older))
	writeFile(t, marksPath(v.now), marksJSON(t, v.newRoot, *newer))
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if held := v.inVault(t); len(held) != 2 {
		t.Errorf("%d thoughts are kept, want both", len(held))
	}
}

// The same words marked at two places of one section are two marks, as the
// route that keeps a mark holds them: both are brought across, and neither is
// taken for the other.
func TestAdoptBringsTheSameWordsAtTwoPlacesAsTwoMarks(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	first, second := aHighlight(), aHighlight()
	first.ID, second.ID = markID(1), markID(2)
	second.Before, second.After = "Elsewhere, ", " is said again"
	writeFile(t, marksPath(v.old), marksJSON(t, v.oldRoot, *first, *second))
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if held := v.inVault(t); len(held) != 2 {
		t.Errorf("%d highlights were brought across, want both", len(held))
	}
}

// What a vault does not hold, or does not let be read, stays where it was: a
// mark on it has nowhere to be drawn here, and a list that was never asked about
// it must not carry it.
func TestAdoptBringsAcrossOnlyWhatIsAdmitted(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	admit := func(rel string) bool { return rel == "Notes/source.md" || rel == "Courses/go/index.md" }
	if err := v.adopt(t, admit); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	held := v.inVault(t)
	for _, m := range held {
		if m.Path != "Notes/source.md" {
			t.Errorf("a mark on %q came across", m.Path)
		}
	}
	if got := len(held); got != 3 {
		t.Errorf("%d marks came across, want the 3 on the admitted note", got)
	}
	// Its course is admitted, but the note it points at is not, and a bookmark
	// that points at ground this vault withholds is not one to bring over.
	if _, found := bookmarkFor(t, v.now, "Courses/go/index.md"); found {
		t.Error("the bookmark for a course came across although its note is refused")
	}
	if _, found := bookmarkFor(t, v.now, mark.NoCourse); found {
		t.Error("the bookmark for no course came across although its note is refused")
	}
	finished := finishedFor(t, v.now)
	if len(finished) != 1 || finished[0].RelPath != "Notes/source.md" {
		t.Errorf("finished notes %+v came across, want only Notes/source.md", finished)
	}
}

func TestAdoptWithNoAdmitterBringsNothingAndWritesNothing(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	marks, err := v.now.Adopt(v.oldRoot, nil)
	if err != nil || len(marks) != 0 {
		t.Fatalf("Adopt = %v, %v; want no marks", marks, err)
	}
	if held := directoryBytes(t, v.now.Dir()); len(held) != 0 {
		t.Errorf("a refused adoption left %d files", len(held))
	}
}

func TestAdoptRefusesWhatCannotBeAdoptedAndNamesNoPath(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)

	roots := t.TempDir()
	elsewhereRoot := filepath.Join(roots, "elsewhere")
	other := newStoreFor(t, v.config, elsewhereRoot)
	writeFile(t, marksPath(other), marksJSON(t, elsewhereRoot, *aQuestion()))
	// A directory copied by hand to the name another path gives records the path
	// it was made for, and says so.
	copiedRoot := filepath.Join(roots, "copied")
	for name, data := range directoryBytes(t, other.Dir()) {
		writeFile(t, filepath.Join(newStoreFor(t, v.config, copiedRoot).Dir(), name), data)
	}
	// A directory that holds a progress file with nothing in it holds nothing.
	emptiedRoot := filepath.Join(roots, "emptied")
	writeFile(t, progressPath(newStoreFor(t, v.config, emptiedRoot)),
		`{"version":1,"vault":`+strconv.Quote(emptiedRoot)+`,"bookmarks":[]}`)

	tests := []struct {
		name  string
		store *mark.Store
		path  string
		want  error
	}{
		{"a relative path", v.now, filepath.Join("before", "notes"), mark.ErrInvalid},
		{"this vault's own path", v.old, v.oldRoot, mark.ErrOwnPath},
		{"a path nothing is kept for", v.now, filepath.Join(roots, "never"), mark.ErrNothingKept},
		{"a directory that holds nothing", v.now, emptiedRoot, mark.ErrNothingKept},
		{"a directory that records another folder", v.now, copiedRoot, mark.ErrOtherVault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := tt.store.Adopt(tt.path, admitEverything)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Adopt(%q) = %v, want %v", tt.path, err, tt.want)
			}
			if strings.Contains(err.Error(), filepath.Base(tt.path)) {
				t.Errorf("the refusal %q names the path it was given", err)
			}
		})
	}
}

// A path that is another spelling of the old one, with a trailing separator or
// a step that comes back, is the same folder and finds the same directory.
func TestAdoptFindsTheDirectoryOfAnyCleanSpellingOfThePath(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	spelled := v.oldRoot + string(filepath.Separator)
	spelled = filepath.Join(spelled, "..", filepath.Base(v.oldRoot)) + string(filepath.Separator)
	if marks, err := v.now.Adopt(spelled, admitEverything); err != nil || len(marks) == 0 {
		t.Fatalf("Adopt(%q) = %v, %v; want the marks kept there", spelled, marks, err)
	}
}

// An earlier version's files record no vault, so the directory is taken by its
// name, converted the way a read converts them, and left as they were.
func TestAdoptTakesADirectoryThatOnlyAnEarlierVersionWrote(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	writeFile(t, filepath.Join(v.old.Dir(), "uncertainty.json"), `[{"path":"Notes/source.md","anchor":"section-one","time":"2026-09-22T10:00:00Z"}]`)
	writeFile(t, filepath.Join(v.old.Dir(), "reader.json"), `{"version":1,"continuation":{"path":"Notes/source.md","anchor":"section-one","offset":3,"identity":"`+anIdentity+`","at":"2026-09-17T10:00:00Z"}}`)
	before := directoryBytes(t, v.old.Dir())
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if held := v.inVault(t); len(held) != 1 || held[0].Kind != machinemarks.Question {
		t.Errorf("marks %+v came across, want the one question", held)
	}
	if got, found, err := v.now.Bookmark(mark.NoCourse); err != nil || !found || got.Offset != 3 {
		t.Errorf("the bookmark %+v (found %v, %v) did not come across", got, found, err)
	}
	if diff := cmp.Diff(before, directoryBytes(t, v.old.Dir())); diff != "" {
		t.Errorf("the earlier version's files changed (-before +after):\n%s", diff)
	}
}

// Marks the vault could not hold beside its own are refused whole.
func TestAdoptOfMarksThatWouldPassTheBoundTakesNoneIn(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	// An id that none of the full file's marks has, so that this mark is the one
	// that does not fit.
	question := aQuestion()
	question.ID = markID(machinemarks.MaxLive + 1)
	writeFile(t, marksPath(v.old), marksJSON(t, v.oldRoot, *question))
	if err := moveBookmark(v.old, mark.NoCourse, aPlace()); err != nil {
		t.Fatal(err)
	}
	full := make([]string, machinemarks.MaxLive)
	for i := range full {
		full[i] = aStoredQuestion(i)
	}
	writeFile(t, marksPath(v.now), aMarksFile(full...))
	if err := v.adopt(t, admitEverything); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("Adopt = %v, want ErrInvalid", err)
	}
	if held := v.inVault(t); slices.ContainsFunc(held, func(m machinemarks.Mark) bool { return m.ID == question.ID }) {
		t.Error("the mark that does not fit was taken in")
	}
}

// A file that cannot be read is reported and left, on either side: a damaged
// file is the reader's to recover, and nothing is built over it.
func TestAdoptWithAnUnreadableFileReportsItAndWritesNothing(t *testing.T) {
	t.Parallel()
	t.Run("in the old directory", func(t *testing.T) {
		t.Parallel()
		v := movedVault(t)
		v.keepEverything(t)
		writeFile(t, marksPath(v.old), `{"version":9,"vault":"x","marks":[]}`)
		if _, err := v.now.Adopt(v.oldRoot, admitEverything); !errors.Is(err, mark.ErrUnreadable) {
			t.Fatalf("Adopt = %v, want ErrUnreadable", err)
		}
		if held := directoryBytes(t, v.now.Dir()); len(held) != 0 {
			t.Errorf("%d files were written beside an unreadable one", len(held))
		}
	})
	t.Run("in this vault's directory", func(t *testing.T) {
		t.Parallel()
		v := movedVault(t)
		v.keepEverything(t)
		writeFile(t, progressPath(v.now), `not a file this wrote`)
		before := directoryBytes(t, v.now.Dir())
		if _, err := v.now.Adopt(v.oldRoot, admitEverything); !errors.Is(err, mark.ErrUnreadable) {
			t.Fatalf("Adopt = %v, want ErrUnreadable", err)
		}
		if diff := cmp.Diff(before, directoryBytes(t, v.now.Dir())); diff != "" {
			t.Errorf("an unreadable file was built over (-before +after):\n%s", diff)
		}
	})
}

// The files it writes are the vault's own: they name this vault and carry the
// time the reader made each thing, not the time it was brought across.
func TestAdoptWritesFilesThatNameThisVaultAndKeepTheirTimes(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	if err := v.adopt(t, admitEverything); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	data, err := os.ReadFile(progressPath(v.now))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Vault string `json:"vault"`
	}
	if err = json.Unmarshal(data, &head); err != nil || head.Vault != v.newRoot {
		t.Errorf("progress.json records the vault %q (%v), want %q", head.Vault, err, v.newRoot)
	}
	if _, err = os.Stat(marksPath(v.now)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an adoption wrote marks.json, which is only read: %v", err)
	}
	for _, m := range v.inVault(t) {
		if before := listedMarks(t, v.old); !slices.ContainsFunc(before, func(o machinemarks.Mark) bool { return o.ID == m.ID && o.At.Equal(m.At) }) {
			t.Errorf("the mark %s came across made at %v, not when it was made", m.ID, m.At)
		}
	}
	got, want := finishedFor(t, v.now), finishedFor(t, v.old)
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("%d finished notes came across, want %d", len(got), len(want))
	}
	for i := range got {
		if !got[i].At.Equal(want[i].At) {
			t.Errorf("the finished note %q is finished at %v, want when it was finished before, %v", got[i].RelPath, got[i].At, want[i].At)
		}
	}
}

// ledgerOf is the ledger of the vault at root in config.
func ledgerOf(t *testing.T, config, root string) (l *ledger.Ledger, file string) {
	t.Helper()
	dir, err := confdir.VaultDir(confdir.At(config), root)
	if err != nil {
		t.Fatal(err)
	}
	return ledger.Open(dir, slog.New(slog.DiscardHandler)), dir.Join("ledger.jsonl")
}

func readOrEmpty(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a file of this test's own temporary directory
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(data)
}

// A vault that moved carries the ledger of what this machine wrote at the
// earlier path, byte for byte, so this machine goes on under the machine id it
// had there. A ledger is something kept: an earlier path that has only a
// ledger is not one with nothing to adopt. Adopting again changes nothing, and
// the earlier ledger is left as it was.
func TestAdoptCarriesTheLedgerOfTheEarlierPath(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	earlier, earlierFile := ledgerOf(t, v.config, v.oldRoot)
	for _, e := range []*ledger.Event{
		{Kind: "header", ID: "1f3a9c07", Process: "7a1f03d2"},
		{Kind: ledger.KindMark, Machine: "1f3a9c07", Process: "7a1f03d2", Mark: "m1", Content: []byte(`{"mark":{"id":"m1","body":"words"}}`)},
	} {
		if _, err := earlier.Record(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	before := readOrEmpty(t, earlierFile)
	now, nowFile := ledgerOf(t, v.config, v.newRoot)
	for i := range 2 {
		if _, err := v.now.Adopt(v.oldRoot, admitEverything); err != nil {
			t.Fatalf("Adopt %d of an earlier path with only a ledger: %v", i+1, err)
		}
		if got := readOrEmpty(t, nowFile); got != before {
			t.Fatalf("after Adopt %d this vault's ledger is %q, want the earlier one byte for byte", i+1, got)
		}
	}
	if carried, _, err := now.Events(); err != nil || len(carried) != 2 || carried[1].Mark != "m1" {
		t.Errorf("the carried ledger reads %v, %v; want its two lines", carried, err)
	}
	if got := readOrEmpty(t, earlierFile); got != before {
		t.Error("Adopt changed the earlier ledger")
	}
}

// A vault that has written a ledger of its own since it moved keeps it: the
// earlier one is not merged into it, and stays where it was.
func TestAdoptKeepsTheLedgerThisVaultHas(t *testing.T) {
	t.Parallel()
	v := movedVault(t)
	v.keepEverything(t)
	earlier, _ := ledgerOf(t, v.config, v.oldRoot)
	if _, err := earlier.Record(t.Context(), &ledger.Event{Kind: "header", ID: "1f3a9c07"}); err != nil {
		t.Fatal(err)
	}
	now, nowFile := ledgerOf(t, v.config, v.newRoot)
	if _, err := now.Record(t.Context(), &ledger.Event{Kind: "header", ID: "2b4d6f80"}); err != nil {
		t.Fatal(err)
	}
	mine := readOrEmpty(t, nowFile)
	marks, err := v.now.Adopt(v.oldRoot, admitEverything)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if got := readOrEmpty(t, nowFile); got != mine {
		t.Errorf("Adopt changed the ledger this vault has to %q, want %q", got, mine)
	}
	if len(marks) != 3 {
		t.Errorf("Adopt answered %d marks beside the kept ledger, want 3", len(marks))
	}
}
