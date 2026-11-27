package machinemarks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

func legacyOf(marks ...Mark) func() ([]Mark, error) {
	return func() ([]Mark, error) { return marks, nil }
}

// The marks of the earlier file are taken in by the first write of a machine
// that has no file of marks, once, as one commit; the earlier file is only read;
// a file git takes away does not make them be taken in again.
func TestTheEarlierMarksAreTakenInOnce(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.legacy = legacyOf(thought(1, "from the old file"), highlight(2, "a passage"), thought(3, "another"))
	a.restart()

	before := tree(t, a.root)
	if got := len(a.view().Entries(open)); got != 0 {
		t.Fatalf("a read took in %d marks", got)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" {
		t.Fatalf("a read wrote %s", diff)
	}
	a.say(4, "the first write")
	if got := len(a.view().Entries(open)); got != 4 {
		t.Fatalf("after the first write %d marks are read, want the 3 converted and 1 new", got)
	}
	file := filepath.Join(a.marksDir(), a.id()+".jsonl")
	if got := len(sealedline.Lines(readFile(t, file))); got != 4 {
		t.Fatalf("the file holds %d lines, want 4", got)
	}
	if n := strings.Count(string(a.ledgerBytes()), `"kind":"converted"`); n != 1 {
		t.Fatalf("the ledger holds %d conversions, want 1", n)
	}

	// The file is taken away, and the earlier marks are not taken in again:
	// they come back from the ledger.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	a.legacy = legacyOf(thought(1, "from the old file"), thought(9, "added to the old file since"))
	a.restart()
	if v := a.view(); v.Missing() == 0 || a.words(9) != "(none)" {
		t.Fatalf("%d missing, mark 9 reads %q", v.Missing(), a.words(9))
	}
	a.say(5, "after the file was taken away")
	if a.words(9) != "(none)" || a.words(2) == "(none)" || a.words(5) == "(none)" {
		t.Fatalf("the old file was taken in again, or the converted marks were lost: %q %q %q", a.words(9), a.words(2), a.words(5))
	}
}

// A conversion is not made over a file that is there: a machine that has a file
// of marks has already begun.
func TestAMachineWithAFileTakesInNothing(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "a file already")
	a.legacy = legacyOf(thought(2, "old"))
	a.restart()
	a.say(3, "second write")
	if a.words(2) != "(none)" {
		t.Fatal("the earlier marks were taken in beside a file that was there")
	}
}

// An earlier file that cannot be read stops the write, and nothing is written
// for it.
func TestAnEarlierFileThatCannotBeReadStopsTheWrite(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.legacy = func() ([]Mark, error) { return nil, errors.New("damaged") }
	a.restart()
	m := thought(1, "x")
	if _, err := a.m.Put(t.Context(), &m, nil, nil); !errors.Is(err, ErrLegacy) {
		t.Fatalf("Put = %v, want ErrLegacy", err)
	}
	if _, err := os.Stat(a.marksDir()); err == nil {
		t.Fatal("a refused write made the folders")
	}
	// Mended, the same write goes through and takes the marks in.
	a.legacy = legacyOf(thought(2, "old"))
	a.restart()
	a.say(1, "x")
	if a.words(2) != "old" {
		t.Fatal("the mended earlier file was not taken in")
	}
}

// A mark of the earlier file that does not fit a line, whatever the reason, is
// skipped and counted, and does not keep every later write from going through:
// the others are taken in and written, and the skipped mark stays in the
// earlier file, which is never changed.
func TestAnEarlierMarkThatDoesNotFitIsSkippedAndTheRestAreTakenIn(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	wrongKind := thought(2, "x")
	wrongKind.Kind = "idea"
	// Quotes are escaped when written, so words that fit the bound as text do
	// not fit as a line.
	tooLong := thought(3, strings.Repeat(`"`, 40<<10))
	noPath := thought(4, "no note to be about")
	noPath.Path = ""
	a.legacy = legacyOf(thought(1, "fine"), wrongKind, tooLong, noPath, thought(5, "also fine"))
	a.restart()
	if _, err := a.m.Put(t.Context(), new(thought(6, "the write that asked")), nil, nil); err != nil {
		t.Fatalf("Put = %v, want the write to go through beside marks that cannot be taken in", err)
	}
	if v := a.view(); v.LegacySkipped() != 3 {
		t.Fatalf("%d skipped, want 3", v.LegacySkipped())
	}
	for n, want := range map[int]string{1: "fine", 5: "also fine", 6: "the write that asked", 2: "(none)", 3: "(none)", 4: "(none)"} {
		if got := a.words(n); got != want {
			t.Errorf("mark %d reads %q, want %q", n, got, want)
		}
	}
	// It is not taken in again, nor counted again, by a later write.
	a.restart()
	a.say(7, "a later write")
	if v := a.view(); v.LegacySkipped() != 3 || a.words(2) != "(none)" {
		t.Fatalf("%d skipped after a later write, and mark 2 reads %q", v.LegacySkipped(), a.words(2))
	}
}

// The gate is asked of every value of every mark, ledger lines included, and a
// mark with one value the gate refuses is not given, nor said to be held.
func TestTheGateHoldsBackWhatTheLedgerHoldsToo(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	secret := thought(1, "an agent's note")
	secret.Path = "agent/secret.md"
	a.put(secret, nil)
	a.say(2, "an ordinary note")
	// git takes A's file away: both marks are now the ledger's alone.
	if err := os.Remove(filepath.Join(a.marksDir(), a.id()+".jsonl")); err != nil {
		t.Fatal(err)
	}
	notReserved := func(m *Mark) bool { return !strings.HasPrefix(m.Path, "agent/") }
	v := a.view()
	if v.Missing() != 2 {
		t.Fatalf("%d missing, want 2", v.Missing())
	}
	if got := v.Entries(notReserved); len(got) != 1 || got[0].Mark().ID != mid(2) {
		t.Fatalf("the gate let %d marks through, want only the ordinary one", len(got))
	}
	if _, ok := v.Entry(mid(1), notReserved); ok {
		t.Fatal("Entry gave a mark the gate refuses, from the ledger")
	}
	// A conflict of which one side is reserved is held back whole.
	b.say(3, "first")
	pull(b, a)
	a.say(3, "A's side")
	reserved := a.entry(mid(3)).Mark()
	reserved.Path, reserved.Body = "agent/x.md", "B's side, in a note an agent holds"
	b.clock.tick()
	if _, err := b.m.Put(t.Context(), &reserved, b.entry(mid(3)).Seen, nil); err != nil {
		t.Fatal(err)
	}
	pull(b, a)
	if len(a.entry(mid(3)).Values) < 2 {
		t.Fatal("no conflict was made")
	}
	if _, ok := a.view().Entry(mid(3), notReserved); ok {
		t.Fatal("a conflict with a value the gate refuses was given")
	}
	for _, e := range a.view().Entries(notReserved) {
		if e.Mark().ID == mid(3) {
			t.Fatal("Entries gave a conflict with a value the gate refuses")
		}
	}
	// The same with the refused value the older one, shown after the first: the
	// gate is asked of every value, not of the one that counts.
	b.say(4, "first")
	pull(b, a)
	a.view()
	older := b.entry(mid(4)).Mark()
	older.Path, older.Body = "agent/y.md", "B's older side, in a note an agent holds"
	b.clock.tick()
	if _, err := b.m.Put(t.Context(), &older, b.entry(mid(4)).Seen, nil); err != nil {
		t.Fatal(err)
	}
	a.say(4, "A's newer side")
	pull(b, a)
	if e := a.entry(mid(4)); len(e.Values) < 2 || e.Mark().Body != "A's newer side" {
		t.Fatalf("mark 4 is %+v, want a conflict that shows A's newer side first", e.Values)
	}
	if _, ok := a.view().Entry(mid(4), notReserved); ok {
		t.Fatal("Entry gave a conflict whose second value the gate refuses")
	}
	for _, e := range a.view().Entries(notReserved) {
		if e.Mark().ID == mid(4) {
			t.Fatal("Entries gave a conflict whose second value the gate refuses")
		}
	}
	// No gate is a mistake, not everything.
	func() {
		defer func() {
			if recover() == nil {
				t.Error("Entries(nil) did not refuse")
			}
		}()
		v.Entries(nil)
	}()
}
