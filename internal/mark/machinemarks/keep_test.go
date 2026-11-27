package machinemarks

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// A Keep of a mark that is there writes nothing and answers the one that is.
func TestKeepOfAMarkThatIsThereWritesNothing(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	first := highlight(1, "dead tuples")
	out, err := a.m.Keep(t.Context(), &first, open)
	if err != nil || !out.Written {
		t.Fatalf("first Keep = %+v, %v", out, err)
	}
	before := tree(t, a.root)
	ledger := string(a.ledgerBytes())
	again := highlight(2, "dead tuples")
	out, err = a.m.Keep(t.Context(), &again, open)
	if err != nil || out.Written || out.Mark.ID != mid(1) {
		t.Fatalf("second Keep = %+v, %v; want the first mark and no write", out, err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" || string(a.ledgerBytes()) != ledger {
		t.Fatalf("a Keep that found the mark wrote %q", diff)
	}
	if got := len(a.view().Entries(open)); got != 1 {
		t.Fatalf("%d marks, want 1", got)
	}
}

// Two Keeps of one highlight at once keep one: the look and the write are one
// step under the lock.
func TestKeepsAtOnceKeepOne(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	var wg sync.WaitGroup
	for n := 1; n <= 8; n++ {
		wg.Go(func() {
			m := highlight(n, "same passage")
			if _, err := crowded(func() (Outcome, error) { return a.m.Keep(t.Context(), &m, open) }); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := len(a.view().Entries(open)); got != 1 {
		t.Fatalf("%d marks after 8 Keeps of one place, want 1", got)
	}
}

func TestKeepRefusesAnIDThatIsTaken(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "words")
	m := highlight(1, "other")
	if _, err := a.m.Keep(t.Context(), &m, open); !errors.Is(err, ErrInvalidMark) {
		t.Fatalf("Keep with a taken id = %v, want ErrInvalidMark", err)
	}
}

// Toggle takes away what is there and makes what is not.
func TestToggleMakesAndTakesAway(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	q := Mark{ID: mid(1), Kind: Question, Path: "courses/pg/vacuum.md", Anchor: "dead-tuples", At: thought(1, "").At}
	same := func(m *Mark) bool { return m.Kind == Question && m.Quote == "" && m.Anchor == "dead-tuples" }
	out, err := a.m.Toggle(t.Context(), &q, same, open)
	if err != nil || !out.Written || out.Mark.Deleted {
		t.Fatalf("first Toggle = %+v, %v", out, err)
	}
	q2 := q
	q2.ID = mid(2)
	out, err = a.m.Toggle(t.Context(), &q2, same, open)
	if err != nil || !out.Written || !out.Mark.Deleted || out.Mark.ID != mid(1) {
		t.Fatalf("second Toggle = %+v, %v; want the first mark deleted", out, err)
	}
	if e := a.entry(mid(1)); !e.Values[0].Mark.Deleted {
		t.Fatal("the unmarked mark still reads as there")
	}
	q3 := q
	q3.ID = mid(3)
	if out, err = a.m.Toggle(t.Context(), &q3, same, open); err != nil || out.Mark.Deleted || out.Mark.ID != mid(3) {
		t.Fatalf("third Toggle = %+v, %v; want a new mark", out, err)
	}
}

// A mark with two values is not taken away blind.
func TestToggleRefusesAMarkInConflict(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	a.say(1, "A's words")
	b.say(1, "B's words")
	pull(b, a)
	q := func(n int) Mark {
		m := thought(n, "")
		m.Kind = Question
		return m
	}
	if e := a.entry(mid(1)); len(e.Values) != 2 {
		t.Fatalf("%d values, want a conflict", len(e.Values))
	}
	m := q(2)
	_, err := a.m.Toggle(t.Context(), &m, func(x *Mark) bool { return x.ID == mid(1) }, open)
	stale, ok := errors.AsType[*StaleError](err)
	if !ok || len(stale.Seen) == 0 {
		t.Fatalf("Toggle on a conflict = %v, want a StaleError with what is read", err)
	}
	if len(a.entry(mid(1)).Values) != 2 {
		t.Fatal("the conflict was written over")
	}
}

// An adoption takes in only the marks no machine has: a taken-in mark that was
// then deleted stays deleted, a mark in the same place as the reader's own is
// not taken for a second, and taking the same in again writes nothing.
func TestTakeInTakesInOnlyWhatIsNew(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	old := []Mark{thought(1, "first"), thought(2, "second")}
	a.legacy = func() ([]Mark, error) { return old, nil }
	a.restart()
	a.say(3, "the first write takes the old ones in")
	a.delete(1)
	here := highlight(5, "dead tuples")
	a.put(here, nil)
	same := here
	same.ID = mid(6)
	adopted := []Mark{thought(1, "first"), thought(4, "arrived with an adoption"), same}
	if _, err := a.m.TakeIn(t.Context(), adopted); err != nil {
		t.Fatal(err)
	}
	if a.words(4) != "arrived with an adoption" {
		t.Fatalf("the adopted mark reads %q", a.words(4))
	}
	if got := a.words(1); got != "deleted:first" {
		t.Fatalf("the deleted mark reads %q after a take-in, want it still deleted", got)
	}
	if _, ok := a.view().Entry(mid(6), open); ok {
		t.Fatal("a mark in the same place as one there was taken in beside it")
	}
	before := string(a.ledgerBytes())
	if _, err := a.m.TakeIn(t.Context(), adopted); err != nil {
		t.Fatal(err)
	}
	if string(a.ledgerBytes()) != before {
		t.Fatal("a take-in with nothing new wrote to the ledger")
	}
}

// Making a mark new needs the caller to stand behind its place, and taking one
// away does not: a question about a note that is gone can still be put down.
func TestToggleAdmitsOnlyWhatIsNewAndAlwaysTakesAway(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	same := func(m *Mark) bool { return m.Kind == Question && m.Anchor == "dead-tuples" }
	refuse := func(*Mark) bool { return false }
	q := thought(1, "")
	q.Kind = Question
	before := tree(t, a.root)
	if _, err := a.m.Toggle(t.Context(), &q, same, refuse); !errors.Is(err, ErrInvalidMark) {
		t.Fatalf("Toggle of a place that is not admitted = %v, want ErrInvalidMark", err)
	}
	if _, err := a.m.Toggle(t.Context(), &q, same, nil); !errors.Is(err, ErrInvalidMark) {
		t.Fatalf("Toggle with no admit = %v, want ErrInvalidMark", err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" {
		t.Fatalf("a refused place wrote %s", diff)
	}
	if _, err := a.m.Toggle(t.Context(), &q, same, open); err != nil {
		t.Fatal(err)
	}
	q2 := q
	q2.ID = mid(2)
	out, err := a.m.Toggle(t.Context(), &q2, same, refuse)
	if err != nil || !out.Mark.Deleted {
		t.Fatalf("taking away with a refusing admit = %+v, %v; want the mark taken away", out, err)
	}
}

// A page that is told it is stale is told what it may write on top of, and that
// is what it had read and this machine's own lines it had not: a line of another
// computer that came in since is not covered by it, so that writing the answer
// back does not hide words the page was never shown.
func TestStaleSeenDoesNotCoverALineOfAnotherComputerThePageNeverRead(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	page := a.entry(mid(1)).Seen // the page, drawn on A
	a.put(thought(1, "A edits in another tab"), page)
	b.say(1, "B's words, read A's first")
	pull(b, a)

	m := thought(1, "from the stale page")
	_, err := a.m.Put(t.Context(), &m, page, nil)
	stale, ok := errors.AsType[*StaleError](err)
	if !ok {
		t.Fatalf("Put on an old page = %v, want a StaleError", err)
	}
	for writer := range stale.Seen {
		if strings.HasPrefix(writer, b.id()+"/") {
			t.Fatalf("the stale answer covers B's line, which the page never read: %v", stale.Seen)
		}
	}
	if _, err = a.m.Put(t.Context(), &m, stale.Seen, nil); err != nil {
		t.Fatalf("writing on top of the stale answer: %v", err)
	}
	got := a.words(1)
	if !strings.Contains(got, "from the stale page") || !strings.Contains(got, "B's words, read A's first") || len(a.entry(mid(1)).Values) < 2 {
		t.Fatalf("after use my words the mark reads %q, want B's words kept beside as a conflict", got)
	}
}
