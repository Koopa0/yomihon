package machinemarks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/ledger"
)

// copyTree copies the configuration directory of one clone to another, which is
// what moving a home directory to a new computer does.
func copyConfig(t *testing.T, from, to *clone) {
	t.Helper()
	entries, err := os.ReadDir(from.dir.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(to.dir.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "machine-file.lock" {
			continue
		}
		if err = os.WriteFile(to.dir.Join(e.Name()), readFile(t, from.dir.Join(e.Name())), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// The configuration directory of one machine is copied to another, so two
// computers write under one machine id. Each notices, by a line under its id
// that none of its processes wrote, and the one that finds it leaves the id:
// nothing is lost, and the view says it took a new one.
func TestACopiedConfigurationDirectoryIsTwoMachines(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "before the copy")
	copyConfig(t, a, b)
	pull(a, b)
	b.restart()
	// Both are the one id now, and each writes on its own.
	idA := a.id()
	if b.id() != idA {
		t.Fatalf("the copy has id %q, want %q", b.id(), idA)
	}
	a.restart() // a new process of A: a code B's ledger has never heard of
	a.say(2, "A's, after the copy")
	b.say(3, "B's, after the copy")
	// B pulls A's file over its own, which is the same name: git would
	// conflict, or take A's.
	if err := os.WriteFile(filepath.Join(b.marksDir(), idA+".jsonl"), readFile(t, filepath.Join(a.marksDir(), idA+".jsonl")), 0o600); err != nil {
		t.Fatal(err)
	}
	if b.view().IDRetired() {
		t.Fatal("B left its id before it wrote")
	}
	// A reading alone does not retire anything; the write does.
	b.say(4, "B writes after the pull")
	if b.id() == idA {
		t.Fatal("B went on under A's id after it found A's lines under it")
	}
	for n, want := range map[int]string{1: "before the copy", 2: "A's, after the copy", 3: "B's, after the copy", 4: "B writes after the pull"} {
		if got := b.words(n); got != want {
			t.Errorf("B reads mark %d as %q, want %q", n, got, want)
		}
	}
	if !b.view().IDRetired() {
		t.Fatal("the view does not say B left its id")
	}
	// A's own file is not rewritten by B: it is the old id's, which is A's now.
	if got := string(readFile(t, filepath.Join(b.marksDir(), idA+".jsonl"))); got != string(readFile(t, filepath.Join(a.marksDir(), idA+".jsonl"))) {
		t.Fatal("B rewrote the file of the id it left")
	}
}

// A machine that leaves an id keeps the lines it wrote under it as its own, so
// they are not found to be another machine's when it writes again.
func TestALeftIdKeepsItsOwnLines(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "before the copy")
	copyConfig(t, a, b)
	b.restart()
	b.say(2, "B under the shared id")
	pull(a, b)
	a.restart()
	a.say(3, "A under the shared id")
	idA := a.id()
	if err := os.WriteFile(filepath.Join(b.marksDir(), idA+".jsonl"), readFile(t, filepath.Join(a.marksDir(), idA+".jsonl")), 0o600); err != nil {
		t.Fatal(err)
	}
	b.say(4, "B leaves")
	left := b.id()
	for range 3 {
		b.restart()
		b.say(5, "again")
		if b.id() != left {
			t.Fatalf("B changed its id again: %q after %q", b.id(), left)
		}
	}
}

// A crash between leaving an id and taking the next is mended by the next
// write, which leaves again.
func TestALeaveStoppedHalfWayIsFinished(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "before")
	copyConfig(t, a, b)
	b.restart()
	b.say(2, "B")
	idA := a.id()
	a.restart()
	a.say(3, "A")
	if err := os.WriteFile(filepath.Join(b.marksDir(), idA+".jsonl"), readFile(t, filepath.Join(a.marksDir(), idA+".jsonl")), 0o600); err != nil {
		t.Fatal(err)
	}
	// B recorded that it left the id and was killed before it took another.
	if _, err := b.m.ledger.Record(t.Context(), &ledger.Event{Kind: kindRetire, ID: idA}); err != nil {
		t.Fatal(err)
	}
	b.restart()
	b.say(4, "B after the crash")
	if b.id() == idA {
		t.Fatal("B went on under the id it had left")
	}
	for n := 1; n <= 4; n++ {
		if b.words(n) == "(none)" {
			t.Errorf("B reads no mark %d", n)
		}
	}
}

// The configuration directory is copied and the line of the other machine is
// not in the working tree: no line is under the id that this machine did not
// write, so nothing happens. This is what a copy that has not diverged is.
func TestACopyThatHasNotDivergedIsOneMachine(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "before")
	copyConfig(t, a, b)
	pull(a, b)
	b.restart()
	id := b.id()
	b.say(2, "B")
	if b.id() != id || strings.Contains(string(b.ledgerBytes()), `"kind":"retire"`) {
		t.Fatal("a machine left an id that no one else wrote under")
	}
}

// The lines a machine wrote under an id it has left are its own, and the file
// of that id is another machine's now: they are written into the file of the id
// it has now, so that git carries them, they are not counted missing for ever,
// and Restore brings them back.
func TestTheLinesOfALeftIdAreKeptInTheFileOfTheNewId(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "before the copy")
	copyConfig(t, a, b)
	b.restart()
	b.say(2, "B under the shared id")
	pull(a, b)
	a.restart()
	a.say(3, "A under the shared id")
	idA := a.id()
	// The file of the shared id is A's in B's working tree now.
	if err := os.WriteFile(filepath.Join(b.marksDir(), idA+".jsonl"), readFile(t, filepath.Join(a.marksDir(), idA+".jsonl")), 0o600); err != nil {
		t.Fatal(err)
	}
	// A line under the shared id with no process in it, which no process of B
	// wrote: it is not B's, whatever the id says.
	foreign, err := (&Line{Fmt: Format, M: idA, Rev: 900, T: 1, Mark: thought(9, "no process wrote this under B's old id")}).encode()
	if err != nil {
		t.Fatal(err)
	}
	fileA := filepath.Join(b.marksDir(), idA+".jsonl")
	if err = os.WriteFile(fileA, append(readFile(t, fileA), append(foreign, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	b.say(4, "B leaves")
	if b.id() == idA {
		t.Fatal("B did not leave the id")
	}
	if strings.Contains(string(readFile(t, filepath.Join(b.marksDir(), b.id()+".jsonl"))), "no process wrote this") {
		t.Fatal("a line under the id B left, that none of B's processes wrote, was written into B's own file")
	}
	if v := b.view(); v.Missing() != 0 {
		t.Fatalf("%d lines are counted missing after B left the id: the lines it wrote under it are in no file of its own", v.Missing())
	}
	own := filepath.Join(b.marksDir(), b.id()+".jsonl")
	if !strings.Contains(string(readFile(t, own)), `"m":"`+idA+`"`) {
		t.Fatal("the file of B's new id holds none of the lines B wrote under the id it left")
	}
	// git takes the file away, and Restore puts everything back.
	if err := os.Remove(own); err != nil {
		t.Fatal(err)
	}
	if v := b.view(); v.Missing() == 0 {
		t.Fatal("nothing is missing after B's file was taken away")
	}
	if _, err := b.m.Restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	if v := b.view(); v.Missing() != 0 {
		t.Fatalf("%d lines are still missing after Restore", v.Missing())
	}
	if !strings.Contains(string(readFile(t, own)), `"m":"`+idA+`"`) {
		t.Fatal("Restore did not bring back the lines B wrote under the id it left")
	}
	for n, want := range map[int]string{2: "B under the shared id", 4: "B leaves"} {
		if got := b.words(n); got != want {
			t.Errorf("B reads mark %d as %q, want %q", n, got, want)
		}
	}
}
