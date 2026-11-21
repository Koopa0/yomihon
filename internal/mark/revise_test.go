package mark_test

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
)

// listed is every mark kept, failing the test when the file cannot be read.
func listed(t *testing.T, store *mark.Store, kinds ...mark.Kind) []mark.Mark {
	t.Helper()
	held, err := store.List(kinds...)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return held
}

// contents is what a file holds, failing the test when it cannot be read.
func contents(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a file under a directory this test made
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func keptMark(t *testing.T, store *mark.Store, m *mark.Mark) mark.Mark {
	t.Helper()
	kept, err := store.Add(m)
	if err != nil {
		t.Fatalf("Add(%v): %v", m.Kind, err)
	}
	return kept
}

func TestReviseResolvesAndReopensAQuestion(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	question := keptMark(t, store, aQuestion())
	first := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

	resolved, found, err := store.Revise(question.ID, mark.Revision{Resolved: new(true)}, first)
	if err != nil || !found || !resolved.ResolvedAt.Equal(first) {
		t.Fatalf("resolving = %v at %v, found %v, %v; want it resolved at %v", resolved.Kind, resolved.ResolvedAt, found, err, first)
	}
	// Resolving what is resolved leaves the day it was resolved.
	again, _, err := store.Revise(question.ID, mark.Revision{Resolved: new(true)}, first.Add(24*time.Hour))
	if err != nil || !again.ResolvedAt.Equal(first) {
		t.Errorf("resolving twice moved the time to %v, %v; want %v", again.ResolvedAt, err, first)
	}
	reopened, _, err := store.Revise(question.ID, mark.Revision{Resolved: new(false)}, first)
	if err != nil || !reopened.ResolvedAt.IsZero() {
		t.Errorf("reopening left %v, %v; want it open", reopened.ResolvedAt, err)
	}
	if held := listed(t, store); len(held) != 1 || !held[0].ResolvedAt.IsZero() {
		t.Errorf("what is kept after reopening is %+v", held)
	}
}

func TestReviseChangesAReasonAndAThoughtsWords(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	question := keptMark(t, store, aQuestion())
	thought := keptMark(t, store, aThought())
	now := time.Now()

	if got, _, err := store.Revise(question.ID, mark.Revision{Reason: new("why does it tie?")}, now); err != nil || got.Reason != "why does it tie?" {
		t.Errorf("a new reason = %q, %v", got.Reason, err)
	}
	if got, _, err := store.Revise(question.ID, mark.Revision{Reason: new("")}, now); err != nil || got.Reason != "" {
		t.Errorf("clearing the reason = %q, %v", got.Reason, err)
	}
	if got, _, err := store.Revise(thought.ID, mark.Revision{Body: new("The topic, and nothing more.")}, now); err != nil || got.Body != "The topic, and nothing more." {
		t.Errorf("a new body = %q, %v", got.Body, err)
	}
	held := listed(t, store)
	if len(held) != 2 || held[0].Reason != "" || held[1].Body != "The topic, and nothing more." {
		t.Errorf("what is kept is %+v", held)
	}
}

func TestReviseRefusesWhatTheKindDoesNotHaveAndWritesNothing(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	highlight := keptMark(t, store, aHighlight())
	question := keptMark(t, store, aQuestion())
	thought := keptMark(t, store, aThought())
	before := contents(t, marksPath(store))
	for name, tt := range map[string]struct {
		id       string
		revision mark.Revision
	}{
		"a reason on a highlight":      {highlight.ID, mark.Revision{Reason: new("x")}},
		"an empty reason on a thought": {thought.ID, mark.Revision{Reason: new("")}},
		"resolving a highlight":        {highlight.ID, mark.Revision{Resolved: new(true)}},
		"resolving a thought":          {thought.ID, mark.Revision{Resolved: new(false)}},
		"a body on a question":         {question.ID, mark.Revision{Body: new("x")}},
		"a body on a highlight":        {highlight.ID, mark.Revision{Body: new("x")}},
		"a thought with no words":      {thought.ID, mark.Revision{Body: new("")}},
		"a body too long to keep":      {thought.ID, mark.Revision{Body: new(strings.Repeat("a", 33<<10))}},
		"no change at all":             {question.ID, mark.Revision{}},
	} {
		if _, found, err := store.Revise(tt.id, tt.revision, time.Now()); !errors.Is(err, mark.ErrInvalid) || found {
			t.Errorf("%s = found %v, %v; want ErrInvalid", name, found, err)
		}
	}
	if after := contents(t, marksPath(store)); after != before {
		t.Error("a refused revision changed the file")
	}
}

func TestReviseFindsOnlyTheIdsTheStoreIssues(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	question := keptMark(t, store, aQuestion())
	for name, id := range map[string]string{
		"an id nothing holds":   strings.Repeat("0", 32),
		"a real id in capitals": strings.ToUpper(question.ID),
		"a real id cut short":   question.ID[:31],
		"nothing":               "",
		"a path":                "../marks",
	} {
		if _, found, err := store.Revise(id, mark.Revision{Resolved: new(true)}, time.Now()); found || err != nil {
			t.Errorf("Revise(%s) = found %v, %v; want nothing found and no error", name, found, err)
		}
	}
	if held := listed(t, store); len(held) != 1 || !held[0].ResolvedAt.IsZero() {
		t.Errorf("a revision of what is not there changed %+v", held)
	}
}

func TestReviseLeavesAFileItCannotReadAsItIs(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	writeFile(t, marksPath(store), "{")
	if _, _, err := store.Revise(strings.Repeat("a", 32), mark.Revision{Resolved: new(true)}, time.Now()); !errors.Is(err, mark.ErrUnreadable) {
		t.Errorf("Revise over a damaged file = %v, want ErrUnreadable", err)
	}
	if data := contents(t, marksPath(store)); data != "{" {
		t.Errorf("the damaged file became %q", data)
	}
}

// TestReviseCanBeAskedTwiceAtOnce holds that a change reads the file and
// replaces it under the lock Add and Remove take, so two changes to different
// marks are both kept.
func TestReviseCanBeAskedTwiceAtOnce(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	question := keptMark(t, store, aQuestion())
	thought := keptMark(t, store, aThought())
	var group sync.WaitGroup
	group.Go(func() {
		if _, _, err := store.Revise(question.ID, mark.Revision{Resolved: new(true)}, time.Now()); err != nil {
			t.Error(err)
		}
	})
	group.Go(func() {
		if _, _, err := store.Revise(thought.ID, mark.Revision{Body: new("changed")}, time.Now()); err != nil {
			t.Error(err)
		}
	})
	group.Wait()
	held := listed(t, store)
	if len(held) != 2 || held[0].ResolvedAt.IsZero() || held[1].Body != "changed" {
		t.Errorf("two changes at once left %+v, want both", held)
	}
}
