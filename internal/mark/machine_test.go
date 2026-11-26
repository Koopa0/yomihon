package mark_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
)

func allowed(*machinemarks.Mark) bool { return true }

func markID(n int) string { return fmt.Sprintf("%032x", n) }

// machineMarksOver is the machine file's marks of a real vault, taking in what
// the store holds on the first write.
func machineMarksOver(t *testing.T, store *mark.Store, root, config string) *machinemarks.Marks {
	t.Helper()
	r, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() }) //nolint:errcheck // the test is over
	f, err := machinefile.Open(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() }) //nolint:errcheck // the test is over
	dir, err := confdir.VaultDir(confdir.At(config), root)
	if err != nil {
		t.Fatal(err)
	}
	return machinemarks.Open(f, dir, slog.New(slog.DiscardHandler), machinemarks.Options{Legacy: store.MachineMarks})
}

// marks.json is taken in by the first write of the machine file, a question's
// reason becomes its words, and marks.json is not changed, renamed or removed:
// not by the conversion, not by a later write, and not when the machine file the
// marks were taken into is gone and written again.
func TestMarksJSONIsTakenInAndNeverTouched(t *testing.T) {
	t.Parallel()
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	store := newStoreFor(t, config, root)
	writeFile(t, marksPath(store), aMarksFile(
		`{"id":"`+markID(1)+`","kind":"question","path":"Notes/a.md","anchor":"x","reason":"why not at once?","resolved":"2026-09-23T10:00:00Z","at":"2026-09-22T10:00:00Z"}`,
		`{"id":"`+markID(2)+`","kind":"thought","path":"Notes/a.md","body":"a thought","at":"2026-09-22T11:00:00Z"}`,
		`{"id":"`+markID(3)+`","kind":"highlight","path":"Notes/a.md","quote":"a passage","before":"b","after":"a","at":"2026-09-22T12:00:00Z"}`,
	))
	before, err := os.ReadFile(marksPath(store))
	if err != nil {
		t.Fatal(err)
	}
	m := machineMarksOver(t, store, root, config)
	first := machinemarks.Mark{ID: markID(4), Kind: machinemarks.Thought, Path: "Notes/b.md", Body: "new", At: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if _, err = m.Put(t.Context(), &first, nil, nil, allowed); err != nil {
		t.Fatal(err)
	}
	v, err := m.View()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]machinemarks.Mark{}
	for _, e := range v.Entries(allowed) {
		got[e.Mark().ID] = e.Mark()
	}
	want := map[string]machinemarks.Mark{
		markID(1): {ID: markID(1), Kind: machinemarks.Question, Path: "Notes/a.md", Anchor: "x", Body: "why not at once?",
			Resolved: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC), At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)},
		markID(2): {ID: markID(2), Kind: machinemarks.Thought, Path: "Notes/a.md", Body: "a thought", At: time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC)},
		markID(3): {ID: markID(3), Kind: machinemarks.Highlight, Path: "Notes/a.md", Quote: "a passage", Before: "b", After: "a", At: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)},
		markID(4): first,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the marks read after the conversion differ (-want +got):\n%s", diff)
	}

	// The machine file goes, and is written again: the converted marks come
	// from the ledger, once, and marks.json is not asked again.
	files, err := filepath.Glob(filepath.Join(root, "reader-notes", ".yomihon", "marks", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("machine files %v, %v", files, err)
	}
	if err = os.Remove(files[0]); err != nil {
		t.Fatal(err)
	}
	writeFile(t, marksPath(store), strings.Replace(string(before), "a thought", "a thought, changed since", 1))
	before, err = os.ReadFile(marksPath(store))
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = markID(5)
	if _, err = m.Put(t.Context(), &second, nil, nil, allowed); err != nil {
		t.Fatal(err)
	}
	if v, err = m.View(); err != nil {
		t.Fatal(err)
	}
	if e, ok := v.Entry(markID(2), allowed); !ok || e.Mark().Body != "a thought" {
		t.Fatalf("mark 2 reads %+v, %v after the file was written again: marks.json was taken in a second time", e, ok)
	}
	if len(v.Entries(allowed)) != 5 {
		t.Fatalf("%d marks after a second write, want 5", len(v.Entries(allowed)))
	}
	after, err := os.ReadFile(marksPath(store))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("marks.json changed: %v", err)
	}
}

// One mark of marks.json that does not fit a line (a body of quotes that is
// within the store's bound and past the line's once escaped) does not keep every
// later write from going through: it is skipped and counted, the rest are taken
// in, and marks.json, which still holds it, is not touched.
func TestOneMarkThatDoesNotFitDoesNotLockTheWrites(t *testing.T) {
	t.Parallel()
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	store := newStoreFor(t, config, root)
	big := strings.Repeat(`\"`, 32<<10) // 32 KiB of quote characters, as JSON text
	writeFile(t, marksPath(store), aMarksFile(
		`{"id":"`+markID(1)+`","kind":"thought","path":"Notes/a.md","body":"`+big+`","at":"2026-09-22T11:00:00Z"}`,
		`{"id":"`+markID(2)+`","kind":"thought","path":"Notes/a.md","body":"fine","at":"2026-09-22T12:00:00Z"}`,
	))
	before, err := os.ReadFile(marksPath(store))
	if err != nil {
		t.Fatal(err)
	}
	m := machineMarksOver(t, store, root, config)
	first := machinemarks.Mark{ID: markID(3), Kind: machinemarks.Thought, Path: "Notes/b.md", Body: "new", At: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if _, err = m.Put(t.Context(), &first, nil, nil, allowed); err != nil {
		t.Fatalf("Put = %v, want the write to go through", err)
	}
	v, err := m.View()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Entry(markID(1), allowed); ok || v.LegacySkipped() != 1 {
		t.Fatalf("the mark that does not fit is read=%v and %d are counted skipped, want it left out and 1 counted", ok, v.LegacySkipped())
	}
	if e, ok := v.Entry(markID(2), allowed); !ok || e.Mark().Body != "fine" {
		t.Fatalf("the other mark of marks.json reads %+v, %v", e, ok)
	}
	after, err := os.ReadFile(marksPath(store))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("marks.json changed: %v", err)
	}
}

// A marks.json that cannot be read stops the first write, which is told what it
// is, and nothing is made in the vault.
func TestAMarksFileThatCannotBeReadStopsTheFirstWrite(t *testing.T) {
	t.Parallel()
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	store := newStoreFor(t, config, root)
	writeFile(t, marksPath(store), `{"version":1,"marks":[`)
	m := machineMarksOver(t, store, root, config)
	mk := machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "a.md", Body: "x", At: time.Now()}
	if _, err = m.Put(t.Context(), &mk, nil, nil, allowed); err == nil {
		t.Fatal("a write went through over a marks.json that cannot be read")
	}
	if _, err = os.Stat(filepath.Join(root, "reader-notes")); err == nil {
		t.Fatal("a refused write made the reader folder")
	}
}

// The machine file's bound on the marks that are not deleted is the one the
// marks file has.
func TestTheMachineFileHasTheMarksFilesBound(t *testing.T) {
	t.Parallel()
	if machinemarks.MaxLive != mark.MaxMarks {
		t.Fatalf("MaxLive = %d, MaxMarks = %d", machinemarks.MaxLive, mark.MaxMarks)
	}
}
