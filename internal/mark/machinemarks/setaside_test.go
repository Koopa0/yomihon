package machinemarks

import (
	"slices"
	"strings"
	"testing"
)

// aSetAsideValue leaves A with a value of its own that a change of A's replaced
// while B's newer value was the one shown: A wrote "A's words", B changed the
// mark from "first" and wrote the newer "B's words", and A, which read both,
// changed the mark again.
func aSetAsideValue(t *testing.T) *clone {
	t.Helper()
	a, b := newClone(t, "A"), newClone(t, "B")
	a.say(1, "first")
	pull(a, b)
	a.say(1, "A's words")
	b.say(1, "B's words")
	pull(b, a)
	if got := a.entry(mid(1)); got.Mark().Body != "B's words" || len(got.Values) != 2 {
		t.Fatalf("A reads %q, want B's newer words first and A's beside", a.words(1))
	}
	a.say(1, "B's words, changed by A")
	return a
}

func (c *clone) setAside() []string {
	c.t.Helper()
	var words []string
	for _, e := range c.view().Entries(open) {
		for i := range e.Aside {
			words = append(words, e.Aside[i].Mark.Body)
		}
	}
	return words
}

// The value of this machine's own that the change replaces is kept whole in the
// ledger, is not a value of the mark, and is not in the file.
func TestAnOwnValueNobodyWasShownIsSetAsideInTheLedger(t *testing.T) {
	t.Parallel()
	a := aSetAsideValue(t)
	if got := a.setAside(); !slices.Equal(got, []string{"A's words"}) {
		t.Fatalf("set aside = %q, want A's words", got)
	}
	// The gate is asked of it as of any value: one it refuses hides the mark.
	if _, ok := a.view().Entry(mid(1), func(m *Mark) bool { return m.Body != "A's words" }); ok {
		t.Error("a mark whose value set aside the gate refuses was given")
	}
	if strings.Contains(string(readFile(t, a.marksDir()+"/"+a.id()+".jsonl")), "A's words") {
		t.Error("the test did not reach the case: A's file still holds the value")
	}
	if got := a.words(1); strings.Contains(got, "A's words") {
		t.Errorf("the mark reads %q: a value set aside is not a value of the mark", got)
	}
}

// A compaction keeps every set-aside line, in a process that began after, and
// the ledger is read the same afterwards.
func TestACompactionKeepsWhatWasSetAside(t *testing.T) {
	t.Parallel()
	a := aSetAsideValue(t)
	for range 2 {
		if err := a.m.ledger.Compact(t.Context()); err != nil {
			t.Fatal(err)
		}
		a.say(1, "words after a compaction")
		a.restart()
		if got := a.setAside(); !slices.Equal(got, []string{"A's words"}) {
			t.Fatalf("set aside after a compaction = %q, want A's words", got)
		}
	}
	// What the ledger reads as the newest line of the mark is not changed by it.
	events, _, err := a.m.ledger.Events()
	if err != nil {
		t.Fatal(err)
	}
	latest := latestLines(events, a.id())
	if got := string(latest[mid(1)].Content); strings.Contains(got, "A's words") && !strings.Contains(got, "after a compaction") {
		t.Errorf("the newest line of the mark is the set-aside one: %s", got)
	}
}
