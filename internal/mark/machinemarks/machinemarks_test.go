package machinemarks

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

func TestAWriteIsInTheLedgerAndInTheVault(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	res := a.put(thought(1, "why does vacuum wait?"), nil)
	if res.Rev == 0 || res.Bytes == 0 {
		t.Fatalf("Put = %+v, want a rev and the bytes written", res)
	}
	id := a.id()
	file := readFile(t, filepath.Join(a.marksDir(), id+".jsonl"))
	if got := len(lines(file)); got != 1 {
		t.Fatalf("the machine file holds %d lines, want 1", got)
	}
	l, v := decode(lines(file)[0])
	if v != readable || l.M != id || l.Mark.Body != "why does vacuum wait?" || l.S == "" {
		t.Fatalf("the line reads as %+v (%d)", l, v)
	}
	if !bytes.Contains(a.ledgerBytes(), []byte("why does vacuum wait?")) {
		t.Fatal("the ledger does not hold the line")
	}
	if got := a.words(1); got != "why does vacuum wait?" {
		t.Fatalf("read back %q", got)
	}
}

// A page that reads the marks while a change waits for the ledger's lock, which
// another process holds, is answered at once and not when the change is made.
func TestAReadIsNotHeldBehindAChangeThatWaitsForTheLedger(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "first")
	// The ledger's lock, held as another process would hold it.
	other, err := confdir.Acquire(t.Context(), a.dir, "ledger.lock", 0)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			if err := other.Release(); err != nil {
				t.Errorf("Release: %v", err)
			}
		}
	}
	t.Cleanup(release)

	changed := make(chan error, 1)
	base := a.entry(mid(1)).Seen
	go func() {
		next := thought(1, "second")
		_, err := a.m.Put(t.Context(), &next, base, nil, open)
		changed <- err
	}()
	for until := time.Now().Add(200 * time.Millisecond); time.Now().Before(until); {
		read := make(chan string, 1)
		go func() {
			v, err := a.m.View()
			if err != nil {
				read <- err.Error()
				return
			}
			e, _ := v.Entry(mid(1), open)
			read <- e.Mark().Body
		}()
		select {
		case got := <-read:
			if got != "first" {
				t.Fatalf("a read while the change waits says %q, want %q", got, "first")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a read waited 5s behind a change that waits for the ledger's lock")
		}
	}
	release()
	if err := <-changed; err != nil {
		t.Fatalf("the change once the lock came free: %v", err)
	}
	if got := a.words(1); got != "second" {
		t.Errorf("after the change the mark says %q, want %q", got, "second")
	}
}

// A ledger that cannot be read fails the read and the write alike, and the
// vault is left as it was: a view made without the ledger would lack the marks
// only the ledger holds, and a write made from it would leave them out of this
// machine's file.
func TestALedgerThatCannotBeReadFailsTheReadAndTheWrite(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "first")
	file := filepath.Join(a.marksDir(), a.id()+".jsonl")
	before := readFile(t, file)
	ledgerFile := a.dir.Join("ledger.jsonl")
	if err := os.Remove(ledgerFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ledgerFile, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.m.View(); !errors.Is(err, ErrLedgerUnreadable) {
		t.Errorf("View with a ledger that cannot be read = %v, want ErrLedgerUnreadable", err)
	}
	next := thought(2, "second")
	if _, err := a.m.Put(t.Context(), &next, nil, nil, open); !errors.Is(err, ErrLedgerUnreadable) {
		t.Errorf("Put with a ledger that cannot be read = %v, want ErrLedgerUnreadable", err)
	}
	if after := readFile(t, file); !bytes.Equal(after, before) {
		t.Errorf("the machine file changed to %q, want it as it was, %q", after, before)
	}
}

func TestAReadMakesNothing(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	vaultBefore, confBefore := tree(t, a.root), tree(t, a.conf)
	v := a.view()
	if got := v.Entries(open); len(got) != 0 || v.MachineID() != "" || v.Missing() != 0 {
		t.Fatalf("a vault nobody marked reads as %d marks, id %q, %d missing", len(got), v.MachineID(), v.Missing())
	}
	for _, step := range []func(){
		func() { a.view() },
		func() {
			if err := a.m.NoticeNewID(t.Context()); err != nil {
				t.Error(err)
			}
		},
	} {
		step()
	}
	if diff := changedPaths(vaultBefore, tree(t, a.root)); diff != "" {
		t.Errorf("a read changed the vault: %s", diff)
	}
	if diff := changedPaths(confBefore, tree(t, a.conf)); diff != "" {
		t.Errorf("a read changed the configuration directory: %s", diff)
	}
}

// changedPaths names what differs between two trees.
func changedPaths(before, after map[string]string) string {
	var out []string
	for p, d := range after {
		if was, ok := before[p]; !ok || was != d {
			out = append(out, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			out = append(out, p+" (gone)")
		}
	}
	return strings.Join(sorted(out...), ", ")
}

// A highlight, a thought and a question are one mark: it keeps its id as it
// changes state, the words are one field, and a resolved question that becomes
// a thought and a question again is resolved still.
func TestAMarkChangesStateAndKeepsItsID(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	steps := []struct {
		name string
		edit func(m *Mark)
		want func(m Mark) bool
	}{
		{"a highlight", func(m *Mark) { *m = highlight(1, "dead tuples") }, func(m Mark) bool { return m.Kind == Highlight }},
		{"a question", func(m *Mark) { m.Kind = Question }, func(m Mark) bool { return m.Kind == Question }},
		{"words in it", func(m *Mark) { m.Body = "why not at once?" }, func(m Mark) bool { return m.Body == "why not at once?" && m.Resolved.IsZero() }},
		{"resolved", func(m *Mark) { m.Resolved = m.At.Add(1) }, func(m Mark) bool { return !m.Resolved.IsZero() }},
		{"a thought", func(m *Mark) { m.Kind = Thought }, func(m Mark) bool { return m.Kind == Thought && !m.Resolved.IsZero() }},
		{"a question again", func(m *Mark) { m.Kind = Question }, func(m Mark) bool { return m.Kind == Question && !m.Resolved.IsZero() }},
	}
	var m Mark
	for _, s := range steps {
		var base Seen
		if s.name != "a highlight" {
			base = a.entry(mid(1)).Seen
			m = a.entry(mid(1)).Mark()
		}
		s.edit(&m)
		a.put(m, base)
		if got := a.entry(mid(1)).Mark(); !s.want(got) || got.ID != mid(1) {
			t.Fatalf("after %s the mark is %+v", s.name, got)
		}
	}
	if n := len(a.view().Entries(open)); n != 1 {
		t.Fatalf("one mark in six states reads as %d marks", n)
	}
}

func TestAWholeSectionMarkIsNeverAHighlight(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "a thought about a section")
	before, ledgerBefore := tree(t, a.root), string(a.ledgerBytes())
	section := a.entry(mid(1)).Mark()
	section.Kind, section.Body, section.Quote = Highlight, "", ""
	_, err := a.m.Put(t.Context(), &section, a.entry(mid(1)).Seen, nil, open)
	if !errors.Is(err, ErrInvalidMark) {
		t.Fatalf("Put of a highlight with no quote = %v, want ErrInvalidMark", err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" || string(a.ledgerBytes()) != ledgerBefore {
		t.Fatalf("a refused highlight wrote %q to the vault or the ledger", diff)
	}
}

// Two machines each change one mark before either has read the other's change:
// neither replaces the other, both are shown, and nothing is merged.
func TestTwoMachinesEditOneMarkAtOnce(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	a.say(1, "A's words")
	b.say(1, "B's words")
	pull(a, b)
	pull(b, a)
	for _, c := range []*clone{a, b} {
		e := c.entry(mid(1))
		if !e.Conflict() || len(e.Values) != 2 {
			t.Fatalf("%s reads %d values, want 2", c.name, len(e.Values))
		}
		if got := c.words(1); got != "B's words | A's words" {
			t.Fatalf("%s reads %q, want the later of the two first", c.name, got)
		}
	}
	// A reads both and says so: the mark has one value again, on both.
	a.say(1, "settled")
	pull(a, b)
	for _, c := range []*clone{a, b} {
		if got := c.words(1); got != "settled" {
			t.Fatalf("%s reads %q after the choice was written, want one value", c.name, got)
		}
	}
}

// A line that was replaced once is not brought back by the replacing line being
// written over: A changes a mark B had changed, then changes it again, and the
// first change of B's, which A's lines both read, stays replaced.
func TestAReplacedLineStaysReplacedWhenTheNewerLineIsRewritten(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	b.say(1, "B's edit")
	pull(b, a)
	a.say(1, "A's edit on B's")
	a.say(1, "A again")
	pull(a, b)
	for _, c := range []*clone{a, b} {
		if got := c.words(1); got != "A again" {
			t.Fatalf("%s reads %q, want only A's last words", c.name, got)
		}
	}
}

// A edits a mark and B deletes it without having read the edit: the deletion
// does not hide A's words, whichever is later; and when both delete it, it is
// deleted.
func TestAConflictShowsTheNewestValueStillAlive(t *testing.T) {
	t.Parallel()
	for _, deleteLast := range []bool{true, false} {
		a, b := newClone(t, "A"), newClone(t, "B")
		a.say(1, "first")
		pull(a, b)
		if deleteLast {
			a.say(1, "A's words")
			b.delete(1)
		} else {
			b.delete(1)
			a.say(1, "A's words")
		}
		pull(b, a)
		pull(a, b)
		for _, c := range []*clone{a, b} {
			e := c.entry(mid(1))
			if e.Deleted() || e.Mark().Body != "A's words" || len(e.Values) != 2 {
				t.Fatalf("deleteLast=%v: %s reads %q, want A's words first with the deletion beside", deleteLast, c.name, c.words(1))
			}
		}
	}
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	a.delete(1)
	b.delete(1)
	pull(a, b)
	pull(b, a)
	for _, c := range []*clone{a, b} {
		if !c.entry(mid(1)).Deleted() {
			t.Fatalf("both deleted it, and %s reads %q", c.name, c.words(1))
		}
	}
}

// git takes this machine's newest lines out of the working tree: they are still
// read, a change of them is made on them, nothing is written by reading, and
// Restore writes them back.
func TestWhatGitTookAwayIsStillReadAndComesBack(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "kept before the reset")
	a.say(2, "never committed")
	file := filepath.Join(a.marksDir(), a.id()+".jsonl")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	before := tree(t, a.root)
	v := a.view()
	if v.Missing() != 2 || a.words(1) != "kept before the reset" || a.words(2) != "never committed" {
		t.Fatalf("after the file was taken away: %d missing, reads %q and %q", v.Missing(), a.words(1), a.words(2))
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" {
		t.Fatalf("reading wrote %s", diff)
	}
	if _, err := a.m.Restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	if a.view().Missing() != 0 || len(lines(readFile(t, file))) != 2 {
		t.Fatalf("after Restore: %d missing, %d lines in the file", a.view().Missing(), len(lines(readFile(t, file))))
	}
	// A second Restore has nothing to do and writes nothing.
	before = tree(t, a.root)
	if res, err := a.m.Restore(t.Context()); err != nil || res.Bytes != 0 {
		t.Fatalf("Restore of a file that is held = %+v, %v", res, err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" {
		t.Fatalf("a Restore with nothing to restore wrote %s", diff)
	}
	// A change of a mark that was not in the working tree brings both back.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	a.say(1, "edited after the reset")
	if got := len(lines(readFile(t, file))); got != 2 {
		t.Fatalf("a change after the reset left %d lines in the file, want both marks", got)
	}
}

// The ledger comes back from an old copy that knows the process, git takes the
// file back to an older commit, and this machine writes before it has pulled
// the other machine's edit: both edits are read, in a new process and in the
// same one.
func TestTheTripleFaultIsAConflict(t *testing.T) {
	t.Parallel()
	for _, sameProcess := range []bool{false, true} {
		a, b := newClone(t, "A"), newClone(t, "B")
		a.say(1, "one")
		oldFile := readFile(t, filepath.Join(a.marksDir(), a.id()+".jsonl"))
		oldLedger := a.ledgerBytes()
		a.say(1, "two")
		a.say(1, "three")
		pull(a, b)
		b.say(1, "B's edit")
		// The ledger goes back, and git takes the file back.
		if err := os.WriteFile(a.dir.Join("ledger.jsonl"), oldLedger, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.marksDir(), a.id()+".jsonl"), oldFile, 0o600); err != nil {
			t.Fatal(err)
		}
		if !sameProcess {
			a.restart()
		}
		a.clock.tick()
		e := a.entry(mid(1))
		if _, err := a.m.Put(t.Context(), new(thought(1, "A after all that")), e.Seen, nil, open); err != nil {
			t.Fatalf("sameProcess=%v: %v", sameProcess, err)
		}
		pull(a, b)
		pull(b, a)
		for _, c := range []*clone{a, b} {
			got := c.words(1)
			if !strings.Contains(got, "A after all that") || !strings.Contains(got, "B's edit") {
				t.Fatalf("sameProcess=%v: %s reads %q: one machine's edit was taken as replaced", sameProcess, c.name, got)
			}
		}
	}
}

// A rev does not come from the clock: a clock set back an hour, and a ledger
// compacted, a new process: the revs only go up.
func TestTheCounterDoesNotDependOnTheClock(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	var last uint64
	for i := range 3 {
		res := a.say(1, strings.Repeat("x", i+1))
		if res.Rev <= last {
			t.Fatalf("rev %d after %d", res.Rev, last)
		}
		last = res.Rev
	}
	a.clock.ms -= 3_600_000
	a.restart()
	if err := a.m.ledger.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	res := a.say(1, "after the clock went back, and a restart and a compaction")
	if res.Rev <= last {
		t.Fatalf("rev %d after %d", res.Rev, last)
	}
	if a.words(1) != "after the clock went back, and a restart and a compaction" {
		t.Fatalf("reads %q", a.words(1))
	}
}

func TestAChangeIsRefusedForAStalePageAndOnlyThen(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	first := a.say(1, "one")
	a.say(1, "two") // another page, or this one, wrote it since
	m := thought(1, "from the old page")

	_, err := a.m.Put(t.Context(), &m, nil, nil, open)
	if !errors.Is(err, ErrNoBase) {
		t.Fatalf("Put of a mark that exists with no base = %v, want ErrNoBase", err)
	}
	_, err = a.m.Put(t.Context(), &m, first.Seen, nil, open)
	stale, isStale := errors.AsType[*StaleError](err)
	if !errors.Is(err, ErrStale) || !isStale {
		t.Fatalf("Put on an old page = %v, want a StaleError", err)
	}
	if stale.Seen[a.writer()] != a.entry(mid(1)).Seen[a.writer()] {
		t.Fatalf("the StaleError says %v, the mark says %v", stale.Seen, a.entry(mid(1)).Seen)
	}
	if got := a.words(1); got != "two" {
		t.Fatalf("a refused change wrote: reads %q", got)
	}
	if _, err = a.m.Put(t.Context(), &m, stale.Seen, nil, open); err != nil {
		t.Fatalf("Put on what the StaleError said = %v", err)
	}
}

// writer is this machine's writer name in the current process.
func (c *clone) writer() string { return c.id() + "/" + c.m.currentSession() }

// The last words of an editing session go through after an earlier send of it
// was written; an earlier send that arrives after a later one is superseded;
// and another page writing wins over both.
func TestTheSendsOfOneEdit(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	base := a.say(1, "start").Seen
	send := func(body string, seq uint64, seen Seen, editID string) error {
		m := thought(1, body)
		a.clock.tick()
		_, err := a.m.Put(t.Context(), &m, seen, &Edit{ID: editID, Seq: seq}, open)
		return err
	}
	if err := send("first send", 1, base, "tab1"); err != nil {
		t.Fatal(err)
	}
	// The page never heard the answer to the first send, and goes on from the
	// version it had: the second send of the same edit goes through.
	if err := send("second send", 2, base, "tab1"); err != nil {
		t.Fatalf("a later send of the same edit = %v, want it written", err)
	}
	// The first send arrives again after the second: the words are in already.
	if err := send("first send", 1, base, "tab1"); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("an earlier send after a later = %v, want ErrSuperseded", err)
	}
	// Another page writes over the same base: it is stale, not superseded.
	if err := send("other tab", 1, base, "tab2"); !errors.Is(err, ErrStale) {
		t.Fatalf("another page on an old base = %v, want ErrStale", err)
	}
	// An old send of a session beside a newer other page: the stale wins.
	if err := send("first send", 1, base, "tab1"); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("got %v", err)
	}
	if got := a.words(1); got != "second send" {
		t.Fatalf("reads %q", got)
	}
}

func TestChooseTakesOnlyAValueInTheConflict(t *testing.T) {
	t.Parallel()
	a, b, c := newClone(t, "A"), newClone(t, "B"), newClone(t, "C")
	a.say(1, "first")
	pull(a, b)
	pull(a, c)
	a.say(1, "A's words")
	b.say(1, "B's words")
	pull(b, a)
	e := a.entry(mid(1))
	var aValue, bValue Value
	for _, v := range e.Values {
		if v.Mark.Body == "A's words" {
			aValue = v
		} else {
			bValue = v
		}
	}
	// A value that is not in the conflict is stale, and nothing is written.
	before := tree(t, a.root)
	if _, err := a.m.Choose(t.Context(), mid(1), Pick{Writer: aValue.Writer, Rev: aValue.Rev + 99}, e.Seen, open); !errors.Is(err, ErrStale) {
		t.Fatalf("Choose of a value that is not there = %v, want ErrStale", err)
	}
	if _, err := a.m.Choose(t.Context(), mid(99), Pick{}, e.Seen, open); !errors.Is(err, ErrGone) {
		t.Fatalf("Choose of a mark that is not there = %v, want ErrGone", err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" {
		t.Fatalf("a refused choice wrote %s", diff)
	}
	// While the page is open a third machine writes over what it had read of
	// the mark and not over this page's: its line is one the choice never read,
	// so it stays beside the one chosen.
	c.say(1, "C, unseen")
	pull(c, a)
	if _, err := a.m.Choose(t.Context(), mid(1), Pick{Writer: bValue.Writer, Rev: bValue.Rev}, e.Seen, open); err != nil {
		t.Fatalf("Choose of B's value = %v", err)
	}
	if got := a.words(1); got != "C, unseen | B's words" && got != "B's words | C, unseen" {
		t.Fatalf("after the choice the mark reads %q, want B's words beside C's, which was not read", got)
	}
	// What was chosen is read by the other machines as the one value: after a
	// pull B reads B's words with C's beside it, and not A's.
	pull(a, b)
	if got := b.words(1); strings.Contains(got, "A's words") {
		t.Fatalf("B still reads A's words after A chose: %q", got)
	}
}

// A page that claims to have read more than there is covers nothing it did not
// read: a line written afterwards by the same writer is not replaced by it.
func TestAPageCannotClaimToHaveReadALineThatIsNotThereYet(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	forged := b.entry(mid(1)).Seen
	for w := range forged {
		forged[w] += 1000
	}
	b.clock.tick()
	if _, err := b.m.Put(t.Context(), new(thought(1, "B's words")), forged, nil, open); err != nil {
		t.Fatal(err)
	}
	a.say(1, "A's later words, which B had not read")
	pull(a, b)
	pull(b, a)
	for _, c := range []*clone{a, b} {
		if got := c.words(1); !strings.Contains(got, "A's later words") || !strings.Contains(got, "B's words") {
			t.Fatalf("%s reads %q: a claim to have read the future replaced a line", c.name, got)
		}
	}
}

// The counter has four floors. Each is shown here by a loss that only it covers:
// the ledger's lines lost and this machine's file gone, once with another
// machine having read a line this machine wrote (its read is the floor), and
// once with no one having read it (what the ledger still holds is).
func TestTheCounterHasFloorsForWhatWasLost(t *testing.T) {
	t.Parallel()
	loseTheLedgerLines := func(a *clone, keep func(line string) bool) {
		t.Helper()
		var kept []string
		for l := range strings.SplitSeq(strings.TrimSpace(string(a.ledgerBytes())), "\n") {
			if keep(l) {
				kept = append(kept, l)
			}
		}
		if err := os.WriteFile(a.dir.Join("ledger.jsonl"), []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(a.marksDir(), a.id()+".jsonl")); err != nil {
			t.Fatal(err)
		}
		a.restart()
	}
	notAMarkLine := func(l string) bool { return !strings.Contains(l, `"kind":"mark"`) }

	t.Run("what another machine read", func(t *testing.T) {
		t.Parallel()
		a, b := newClone(t, "A"), newClone(t, "B")
		a.say(1, "one")
		a.say(1, "two")
		last := a.say(1, "three").Rev
		pull(a, b)
		b.say(1, "B read three")
		loseTheLedgerLines(a, notAMarkLine)
		pull(b, a)
		if got := a.say(1, "A again").Rev; got <= last {
			t.Fatalf("rev %d after a line of rev %d that B had read: B's reading would cover it", got, last)
		}
		if got := a.words(1); got != "A again" {
			t.Fatalf("mark 1 reads %q, want A's new words and no conflict", got)
		}
	})
	t.Run("what the ledger says", func(t *testing.T) {
		t.Parallel()
		a := newClone(t, "A")
		a.say(1, "one")
		// An agent changes the file, so the next write keeps what was there:
		// an event of the ledger that is not a line.
		file := filepath.Join(a.marksDir(), a.id()+".jsonl")
		if err := os.WriteFile(file, append(readFile(t, file), []byte("an agent's note\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
		a.say(2, "two")
		loseTheLedgerLines(a, notAMarkLine)
		var top int
		for _, l := range lines(a.ledgerBytes()) {
			var e struct {
				Seq int `json:"seq"`
			}
			if err := json.Unmarshal(l, &e); err != nil {
				t.Fatal(err)
			}
			top = max(top, e.Seq)
		}
		if top < 3 {
			t.Fatalf("the ledger's highest seq is %d: the fault does not leave an event beyond the lines", top)
		}
		if got := a.say(1, "again").Rev; got <= uint64(top) {
			t.Fatalf("rev %d is not above seq %d, which the ledger still says", got, top)
		}
	})
}

// A process that is restarted writes over the line of the one before it, and a
// later change of the mark, a deletion, must still cover that line: when git
// puts the old file back, the old line must not be a value of the mark again.
func TestADeletionAfterARestartCoversTheLineOfTheProcessBefore(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "written by the first process")
	file := filepath.Join(a.marksDir(), a.id()+".jsonl")
	old := readFile(t, file)
	a.restart()
	a.say(1, "written by the second process")
	a.delete(1)
	// git puts the file of the first process back.
	if err := os.WriteFile(file, old, 0o600); err != nil {
		t.Fatal(err)
	}
	a.restart()
	e := a.entry(mid(1))
	if !e.Deleted() || e.Conflict() {
		t.Fatalf("after the old file came back the mark is deleted=%v with %d values, want deleted and no conflict: %s", e.Deleted(), len(e.Values), a.words(1))
	}
}

// Lines that say they had read what had not been written leave no line that
// nothing replaces. Then each writer's newest line stands, and not the older
// lines of a writer that its own newer line replaced.
func TestWhenEveryLineIsReplacedEachWritersNewestStands(t *testing.T) {
	t.Parallel()
	const (
		m1, m2 = "11111111", "22222222"
		w2     = m2 + "/"
	)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	line := func(m string, rev uint64, seen Seen, body string) *kept {
		mark := thought(1, body)
		mark.At = at
		l := Line{Fmt: Format, M: m, Rev: rev, T: 1, Mark: mark, Seen: seen}
		raw, err := l.encode()
		if err != nil {
			t.Fatal(err)
		}
		return &kept{Line: l, raw: raw}
	}
	// 1's second line says it had read 2's first; 2's first says it had read 1's
	// second: each replaces the other, and 1's first line is replaced by its own
	// second.
	got := resolve([]*kept{
		line(m1, 1, nil, "one, first"),
		line(m1, 2, Seen{w2: 1}, "one, second"),
		line(m2, 1, Seen{m1 + "/": 2}, "two, first"),
	})
	var bodies []string
	for _, h := range got.heads {
		bodies = append(bodies, h.Mark.Body)
	}
	slices.Sort(bodies)
	if want := []string{"one, second", "two, first"}; !slices.Equal(bodies, want) {
		t.Fatalf("the lines that stand are %q, want %q", bodies, want)
	}
}

// A page that sends again and again, each send on top of what the last one
// answered, has read only what it was shown: a line another machine's edit
// brought in while the page was open is not in what it answers, so the next
// send does not cover it and the edit is not hidden.
func TestAnAnswerSaysOnlyWhatThePageHadRead(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	page := a.entry(mid(1)).Seen // the page A's reader has open
	b.say(1, "B's edit")
	pull(b, a) // a git pull brings it in while the page is open
	e := Edit{ID: "page-1", Seq: 1}
	first, err := a.m.Put(t.Context(), new(thought(1, "A, typing")), page, &e, open)
	if err != nil {
		t.Fatal(err)
	}
	e.Seq = 2
	if _, err = a.m.Put(t.Context(), new(thought(1, "A, typing on")), first.Seen, &e, open); err != nil {
		t.Fatalf("the second send: %v", err)
	}
	got := a.entry(mid(1))
	if !got.Conflict() {
		t.Fatalf("B's edit was covered by a page that had not read it: %s", a.words(1))
	}
	if words := a.words(1); !strings.Contains(words, "B's edit") || !strings.Contains(words, "A, typing on") || strings.Contains(words, "A, typing |") {
		t.Fatalf("the mark reads %q, want A's last words and B's edit as its two values", words)
	}
}
