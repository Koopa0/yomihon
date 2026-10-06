package snapshot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// headSource is the vault a generation reads, with the reads of a briefing's
// head recorded and refusable. It holds the reader beside the embedded ObservedSource
// so the test says what it asks of the reader and not of the interface.
type headSource struct {
	ObservedSource

	reader *vault.Reader
	asked  map[string]int64
	refuse error
}

func (s *headSource) ReadPrefix(ctx context.Context, entry vault.Entry, n int64) ([]byte, error) {
	s.asked[entry.Path()] = n
	if s.refuse != nil {
		return nil, s.refuse
	}
	return s.reader.ReadPrefix(ctx, entry, n)
}

func headStore(t *testing.T, root string, refuse error) (*Store, *headSource) {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &headSource{ObservedSource: reader, reader: reader, asked: map[string]int64{}, refuse: refuse}
	contract := testContract(t, root)
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store, source
}

// TestABriefingIsNamedByTheTitleAtItsHead pins what a generation does to learn
// a briefing's name. It asks for the first 64 KiB of every file the shelf calls
// a briefing and for no other file, so the answer costs a bounded read however
// large the briefing is: a briefing carries its fonts and pictures inline and
// the whole file may be several times the size at which the generation stops
// reading a file as text. That is the case the last briefing below stands for.
func TestABriefingIsNamedByTheTitleAtItsHead(t *testing.T) {
	t.Parallel()

	const dir = "System/reports/daily-briefing/"
	root := t.TempDir()
	writeNote(t, root, dir+"2026-09-22.html", "<!doctype html><title>What the receiver leaves behind</title><p>body</p>")
	writeNote(t, root, dir+"2026-09-21.html", "<p>a briefing that names nothing</p>")
	writeNote(t, root, dir+"2026-09-20.html", "<title>A title ahead of a very large file</title><!--"+strings.Repeat("x", (1<<20)+4096)+"-->")
	writeNote(t, root, dir+"2026-09-19.html", "<!--"+strings.Repeat("x", 70<<10)+"--><title>Past the bound</title>")
	writeNote(t, root, "Notes/page.html", "<title>Not a briefing</title>")

	store, source := headStore(t, root, nil)

	got := map[string]string{}
	for _, report := range store.Current().Navigation().Reports() {
		got[report.RelPath] = report.Title
	}
	want := map[string]string{
		dir + "2026-09-22.html": "What the receiver leaves behind",
		dir + "2026-09-21.html": "2026-09-21.html",
		dir + "2026-09-20.html": "A title ahead of a very large file",
		dir + "2026-09-19.html": "2026-09-19.html",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("briefing titles (-want +got):\n%s", diff)
	}

	wantAsked := map[string]int64{
		dir + "2026-09-22.html": 65536,
		dir + "2026-09-21.html": 65536,
		dir + "2026-09-20.html": 65536,
		dir + "2026-09-19.html": 65536,
	}
	if diff := cmp.Diff(wantAsked, source.asked); diff != "" {
		t.Errorf("head reads asked of the source (-want +got):\n%s", diff)
	}
}

// TestABriefingHeadThatCannotBeReadDoesNotHoldBackTheGeneration pins what a
// refused head read costs: that briefing is shown under its file name and
// nothing else changes. A name is a courtesy to the reader, so holding the
// whole folder back for one file that will not open would stop every later
// change from ever being published.
func TestABriefingHeadThatCannotBeReadDoesNotHoldBackTheGeneration(t *testing.T) {
	t.Parallel()

	const path = "System/reports/daily-briefing/2026-09-22.html"
	root := t.TempDir()
	writeNote(t, root, path, "<title>Unreachable</title>")
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")

	store, _ := headStore(t, root, errors.New("injected head read failure"))

	gen := store.Current()
	if !gen.Freshness().Complete {
		t.Error("the generation is incomplete; a refused head read held it back")
	}
	reports := gen.Navigation().Reports()
	if len(reports) != 1 || reports[0].Title != "2026-09-22.html" {
		t.Errorf("Reports() = %+v, want the briefing under its file name", reports)
	}
	if _, ok := gen.Note("Concepts/Alpha.md"); !ok {
		t.Error("a note beside the briefing was lost")
	}
}
