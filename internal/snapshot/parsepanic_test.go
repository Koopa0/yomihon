package snapshot

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// recordedLog is a slog.Handler that keeps every record it is handed, so a test
// can count what a build logged at one level and read the attributes on it.
type recordedLog struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *recordedLog) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature, record by value.
func (l *recordedLog) Handle(_ context.Context, record slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, record.Clone())
	return nil
}

func (l *recordedLog) WithAttrs([]slog.Attr) slog.Handler { return l }

func (l *recordedLog) WithGroup(string) slog.Handler { return l }

// at returns the records logged at exactly level, in the order they were logged.
func (l *recordedLog) at(level slog.Level) []slog.Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []slog.Record
	for i := range l.records {
		if l.records[i].Level == level {
			out = append(out, l.records[i])
		}
	}
	return out
}

// attrText returns the text of the attribute a record carries under key, empty
// when it carries none.
func attrText(record *slog.Record, key string) string {
	var text string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			text = attr.Value.String()
			return false
		}
		return true
	})
	return text
}

// parsePanicsFor makes the parse of each named note panic with value, and
// returns the function that puts the real parse back; the test's end puts it
// back too. Every other note parses as it always does.
//
// The parse is a package variable, so a test calling this must not be parallel:
// a parallel test is held until every sequential one has finished, which is
// what keeps the swap from being read by a build running beside it.
func parsePanicsFor(tb testing.TB, value any, paths ...string) (restore func()) {
	tb.Helper()
	genuine := parseNoteSource
	restore = func() { parseNoteSource = genuine }
	parseNoteSource = func(rel string, data []byte) *vault.Note {
		if slices.Contains(paths, rel) {
			panic(value)
		}
		return genuine(rel, data)
	}
	tb.Cleanup(restore)
	return restore
}

// mustNotPanic runs step and turns a panic that escapes it into the test
// failure it is, naming the step, rather than a crashed test binary.
func mustNotPanic(tb testing.TB, step string, run func()) {
	tb.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			tb.Fatalf("%s: a panic parsing one note escaped the snapshot build: %v", step, rec)
		}
	}()
	run()
}

// panicSource opens root as a recordingSource so a test can read how many times
// each file was read, and closes the capability when the test ends.
func panicSource(tb testing.TB, root string) *recordingSource {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { closeReader(tb, reader) })
	return &recordingSource{Source: reader, reads: make(map[string]int), fail: make(map[string]int)}
}

// TestAPanicParsingOneNoteAtStartupCostsThatNoteNotTheVault is the lock on the
// startup scan's containment. One note's parse panics; the build still publishes
// a generation, the other notes are in it, the file is named where a reader looks
// for what could not be read, the note page has the unreadable answer for it
// rather than a 404, and one ERROR record says which file and what the parser
// panicked with.
func TestAPanicParsingOneNoteAtStartupCostsThatNoteNotTheVault(t *testing.T) {
	const (
		poisoned = "Concepts/Poison.md"
		value    = "injected parser panic"
	)
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, poisoned, "---\ntitle: Poison\ntype: concept\n---\npoison body\n")
	writeNote(t, root, "Concepts/Beta.md", "---\ntitle: Beta\ntype: concept\n---\nbeta\n")
	contract := testContract(t, root)
	parsePanicsFor(t, value, poisoned)
	logs := &recordedLog{}

	var store *Store
	mustNotPanic(t, "startup scan", func() {
		var err error
		store, err = New(t.Context(), panicSource(t, root), slog.New(logs), contract, contract.Governance())
		if err != nil {
			t.Fatalf("New() error = %v, want a generation of the notes that parse", err)
		}
	})
	gen := store.Current()

	for _, served := range []string{"Concepts/Alpha.md", "Concepts/Beta.md"} {
		if _, ok := gen.Note(served); !ok {
			t.Errorf("Note(%q) missing: one note's panic cost the notes beside it", served)
		}
	}
	if _, ok := gen.Note(poisoned); ok {
		t.Errorf("Note(%q) present: a note whose parse panicked was served as though it had been read", poisoned)
	}
	if _, ok := gen.Entry(poisoned); !ok {
		t.Errorf("Entry(%q) missing: the file is on disk, so its page must say it could not be read rather than that nothing is there", poisoned)
	}
	if got := gen.NoteCount(); got != 3 {
		t.Errorf("NoteCount() = %d, want 3: a note that could not be parsed is still a note somebody wrote", got)
	}
	// A citation of the note still lands on its file, as it does for a note the
	// read could not open: the resolver holds a stub for it.
	if got := gen.Graph().Resolve("Poison"); got.Kind != graph.KindUnique || got.RelPath != poisoned {
		t.Errorf("Resolve(Poison) = %+v, want the file %q", got, poisoned)
	}

	fresh := gen.Freshness()
	if fresh.Complete {
		t.Error("Freshness().Complete = true for a generation that could not read one of its notes")
	}
	if len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != poisoned {
		t.Fatalf("Freshness().Blocked = %+v, want exactly %q named", fresh.Blocked, poisoned)
	}
	if !strings.Contains(fresh.Blocked[0].Reason, value) {
		t.Errorf("Blocked reason = %q, want the panic value %q in it", fresh.Blocked[0].Reason, value)
	}

	errs := logs.at(slog.LevelError)
	if len(errs) != 1 {
		t.Fatalf("logged %d ERROR records, want exactly 1", len(errs))
	}
	if path := attrText(&errs[0], "path"); path != poisoned {
		t.Errorf("ERROR path attribute = %q, want %q", path, poisoned)
	}
	if got := attrText(&errs[0], "panic"); got != value {
		t.Errorf("ERROR panic attribute = %q, want %q", got, value)
	}
}

// TestAPanicParsingOneNoteDuringARebuildCostsThatNoteNotTheServer is the same
// lock on the path a running server takes: a note that read fine is saved into a
// state its parse panics on, and the next rebuild must neither end the process
// nor take the notes beside it. Until the folder degrades the last whole
// generation is retained; after, the published one carries the last good copy of
// the note marked stale and names it blocked. Fixing the note recovers it.
func TestAPanicParsingOneNoteDuringARebuildCostsThatNoteNotTheServer(t *testing.T) {
	const (
		poisoned = "Concepts/Poison.md"
		value    = "injected parser panic"
	)
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, poisoned, "---\ntitle: Poison\ntype: concept\n---\npoison as first read\n")
	contract := testContract(t, root)
	logs := &recordedLog{}
	source := panicSource(t, root)
	store, err := New(t.Context(), source, slog.New(logs), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	whole := store.Current()
	if !whole.Freshness().Complete {
		t.Fatal("the first generation, built with the real parse, is not whole; the rebuild below proves nothing")
	}
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note is saved into the state its parse panics on, and a new note is
	// written beside it, so what the containment costs the reader is visible.
	// The size change is what makes the next rescan rebuild.
	restore := parsePanicsFor(t, value, poisoned)
	writeNote(t, root, poisoned, "---\ntitle: Poison\ntype: concept\n---\npoison, saved into a state that panics\n")
	const added = "Concepts/Beta.md"
	writeNote(t, root, added, "---\ntitle: Beta\ntype: concept\n---\nbeta\n")
	readsBefore := source.reads[poisoned]

	// One rescan per scan interval, as the ticker drives it. The first three
	// attempts land on ticks 0, 1 and 3, and the third is where retention ends.
	mustNotPanic(t, "rebuild", func() {
		for tick := range 4 {
			clock = base.Add(time.Duration(tick) * scanInterval)
			store.rescan(t.Context())
		}
	})
	degraded := store.Current()
	if degraded == whole {
		t.Fatal("the rebuild never published: one note's panic held the whole folder at its old generation")
	}
	if _, ok := degraded.Note(added); !ok {
		t.Error("a note written beside the one whose parse panics never reached a published generation")
	}
	if _, ok := degraded.Note("Concepts/Alpha.md"); !ok {
		t.Error("a note whose parse did not panic was lost from the rebuilt generation")
	}
	carried, ok := degraded.Note(poisoned)
	if !ok {
		t.Fatalf("Note(%q) missing: the last good copy of the note was dropped instead of carried", poisoned)
	}
	if !carried.Stale || !strings.Contains(carried.Body, "poison as first read") {
		t.Errorf("carried note = {Stale:%t Body:%q}, want the last good copy marked as one that could not be re-read", carried.Stale, carried.Body)
	}
	fresh := degraded.Freshness()
	if fresh.Complete || len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != poisoned {
		t.Errorf("Freshness() = %+v, want an incomplete generation naming only %q", fresh, poisoned)
	}

	// One ERROR for each build attempt that met the panic, each naming the file
	// and the value; the attempts are counted by the reads of the file.
	errs := logs.at(slog.LevelError)
	if want := source.reads[poisoned] - readsBefore; want == 0 || len(errs) != want {
		t.Fatalf("logged %d ERROR records for %d build attempts, want one for each", len(errs), want)
	}
	for i := range errs {
		if path := attrText(&errs[i], "path"); path != poisoned {
			t.Errorf("ERROR %d path attribute = %q, want %q", i, path, poisoned)
		}
		if got := attrText(&errs[i], "panic"); got != value {
			t.Errorf("ERROR %d panic attribute = %q, want %q", i, got, value)
		}
	}

	// Fixing the note is the ordinary path: it parses again, the next attempt
	// publishes a whole generation, and every degraded fact clears.
	restore()
	clock = base.Add(20 * scanInterval)
	store.rescan(t.Context())
	recovered := store.Current()
	if recovered == degraded {
		t.Fatal("a note that parses again did not publish a new generation")
	}
	if fresh := recovered.Freshness(); !fresh.Complete || len(fresh.Blocked) != 0 {
		t.Errorf("recovered Freshness() = %+v, want whole with nothing blocked", fresh)
	}
	note, ok := recovered.Note(poisoned)
	if !ok || note.Stale || !strings.Contains(note.Body, "saved into a state that panics") {
		t.Errorf("recovered note = %+v, present %t; want the edited body read afresh", note, ok)
	}
}
