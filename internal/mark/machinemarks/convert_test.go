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
	if _, err := a.m.Put(t.Context(), &m, nil, nil, open); !errors.Is(err, ErrLegacy) {
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
	if _, err := a.m.Put(t.Context(), new(thought(6, "the write that asked")), nil, nil, open); err != nil {
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
	if _, err := b.m.Put(t.Context(), &reserved, b.entry(mid(3)).Seen, nil, open); err != nil {
		t.Fatal(err)
	}
	pull(b, a)
	if !a.entry(mid(3)).Conflict() {
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
	if _, err := b.m.Put(t.Context(), &older, b.entry(mid(4)).Seen, nil, open); err != nil {
		t.Fatal(err)
	}
	a.say(4, "A's newer side")
	pull(b, a)
	if e := a.entry(mid(4)); !e.Conflict() || e.Mark().Body != "A's newer side" {
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

// A write is asked through the gate too: a mark the gate withholds answers a
// Put, a Delete and a Choose exactly as a mark that is not there does, so that
// the answer says nothing of it, and no error carries its writers or revs.
func TestAWriteToAWithheldMarkAnswersAsIfThereWasNone(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	secret := thought(1, "an agent's note")
	secret.Path = "agent/secret.md"
	a.put(secret, nil)
	// A conflict whose second value is the reserved one.
	b.say(2, "first")
	pull(b, a)
	older := b.entry(mid(2)).Mark()
	older.Path, older.Body = "agent/y.md", "B's older side"
	b.clock.tick()
	if _, err := b.m.Put(t.Context(), &older, b.entry(mid(2)).Seen, nil, open); err != nil {
		t.Fatal(err)
	}
	a.say(2, "A's newer side")
	pull(b, a)
	notReserved := func(m *Mark) bool { return !strings.HasPrefix(m.Path, "agent/") }
	if e := a.entry(mid(2)); !e.Conflict() {
		t.Fatal("no conflict was made")
	}
	seen1, seen2 := a.entry(mid(1)).Seen, a.entry(mid(2)).Seen
	before := string(a.ledgerBytes())
	tree1 := tree(t, a.root)

	edit := thought(1, "an edit of a mark the gate withholds")
	edit.Path = "notes/x.md"
	edit2 := thought(2, "an edit of a conflict the gate withholds")
	for name, call := range map[string]func() error{
		"Put with the page's base": func() error { _, err := a.m.Put(t.Context(), &edit, seen1, nil, notReserved); return err },
		"Put with no base":         func() error { _, err := a.m.Put(t.Context(), &edit, nil, nil, notReserved); return err },
		"Put with a stale base": func() error {
			_, err := a.m.Put(t.Context(), &edit, Seen{"00000000/": 1}, nil, notReserved)
			return err
		},
		"Put on a withheld conflict":    func() error { _, err := a.m.Put(t.Context(), &edit2, seen2, nil, notReserved); return err },
		"Delete":                        func() error { _, err := a.m.Delete(t.Context(), mid(1), seen1, nil, notReserved); return err },
		"Delete of a withheld conflict": func() error { _, err := a.m.Delete(t.Context(), mid(2), seen2, nil, notReserved); return err },
		"Choose": func() error {
			_, err := a.m.Choose(t.Context(), mid(2), Pick{Writer: a.writer(), Rev: 1}, seen2, notReserved)
			return err
		},
		"Choose of a value that is not there": func() error {
			_, err := a.m.Choose(t.Context(), mid(2), Pick{Writer: "ffffffff/", Rev: 99}, seen2, notReserved)
			return err
		},
	} {
		if err := call(); !saysGone(err) {
			t.Errorf("%s = %#v, want what a mark that is not there answers, ErrGone and nothing more", name, err)
		}
	}
	// What a mark that is not there answers.
	if _, err := a.m.Delete(t.Context(), mid(99), nil, nil, notReserved); !saysGone(err) {
		t.Fatalf("Delete of nothing = %v, want ErrGone", err)
	}
	if _, err := a.m.Choose(t.Context(), mid(99), Pick{}, nil, notReserved); !saysGone(err) {
		t.Fatalf("Choose of nothing = %v, want ErrGone", err)
	}
	if string(a.ledgerBytes()) != before || changedPaths(tree1, tree(t, a.root)) != "" {
		t.Fatal("a refused write changed the ledger or the vault")
	}
	for name, call := range map[string]func(){
		"Put":    func() { _, _ = a.m.Put(t.Context(), &edit, nil, nil, nil) },        //nolint:errcheck // it panics
		"Delete": func() { _, _ = a.m.Delete(t.Context(), mid(1), nil, nil, nil) },    //nolint:errcheck // it panics
		"Choose": func() { _, _ = a.m.Choose(t.Context(), mid(1), Pick{}, nil, nil) }, //nolint:errcheck // it panics
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s with no gate did not refuse", name)
				}
			}()
			call()
		}()
	}
}

// saysGone says err is ErrGone with nothing of a mark in it: not a StaleError,
// and a message that is ErrGone's own.
func saysGone(err error) bool {
	_, stale := errors.AsType[*StaleError](err)
	return errors.Is(err, ErrGone) && !stale && err.Error() == ErrGone.Error()
}
