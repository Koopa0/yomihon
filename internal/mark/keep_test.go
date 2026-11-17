package mark_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

// aThought is a thought beside a span of a note.
func aThought() *mark.Mark {
	return &mark.Mark{
		Kind:     mark.Thought,
		RelPath:  "Notes/source.md",
		Anchor:   "section-one",
		Quote:    "the topic particle",
		Before:   "In Japanese, ",
		After:    " marks what the sentence is about",
		Body:     "It marks the topic, not the subject.",
		Identity: anIdentity,
		At:       time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
	}
}

func TestKeepKeepsAMarkAndNamesIt(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	kept, added, err := store.Keep(aHighlight(), admitAny)
	if err != nil || !added {
		t.Fatalf("Keep = %v, %v, want a new mark", added, err)
	}
	held, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Mark{kept}, held); diff != "" {
		t.Errorf("the mark Keep answered is not the mark kept (-answered +kept):\n%s", diff)
	}
	if len(kept.ID) != 32 {
		t.Errorf("the id is %q, want the 32 hex digits the Store issues", kept.ID)
	}
}

func TestKeepAnswersTheMarkAlreadyThereAndWritesNothing(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	first, _, err := store.Keep(aHighlight(), admitAny)
	if err != nil {
		t.Fatal(err)
	}
	before := contents(t, marksPath(store))
	again := aHighlight()
	again.At = again.At.Add(time.Hour)
	second, added, err := store.Keep(again, func(*mark.Mark) bool {
		t.Error("admit was asked about a mark that is already kept")
		return true
	})
	if err != nil || added {
		t.Fatalf("Keep of what is kept = %v, %v, want the mark that is there", added, err)
	}
	if second.ID != first.ID {
		t.Errorf("Keep answered %s, want the mark kept before, %s", second.ID, first.ID)
	}
	if after := contents(t, marksPath(store)); after != before {
		t.Error("Keep of what is kept rewrote the file")
	}
}

func TestKeepTakesEveryThoughtAsNew(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	for range 2 {
		if _, added, err := store.Keep(aThought(), admitAny); err != nil || !added {
			t.Fatalf("Keep of a thought = %v, %v, want a new mark each time", added, err)
		}
	}
	if held := listed(t, store, mark.Thought); len(held) != 2 {
		t.Errorf("%d thoughts are kept, want 2", len(held))
	}
}

func TestKeepKeepsNothingAdmitDoesNotAccept(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	for name, admit := range map[string]func(*mark.Mark) bool{
		"a refusing admit": func(*mark.Mark) bool { return false },
		"no admit":         nil,
	} {
		if _, _, err := store.Keep(aHighlight(), admit); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("Keep with %s = %v, want ErrInvalid", name, err)
		}
	}
	if store.Stored() {
		t.Error("a refused mark left a file")
	}
}

func TestKeepRefusesWhatNoFileHolds(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	empty := aHighlight()
	empty.Quote = ""
	for name, m := range map[string]*mark.Mark{"nil": nil, "a highlight of nothing": empty} {
		if _, _, err := store.Keep(m, admitAny); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("Keep of %s = %v, want ErrInvalid", name, err)
		}
	}
}

func TestKeepStopsAtTheCountAndStillAnswersWhatIsKept(t *testing.T) {
	t.Parallel()

	full := make([]string, mark.MaxMarks)
	for i := range full {
		full[i] = aStoredQuestion(i)
	}
	store := newStore(t)
	writeFile(t, marksPath(store), aMarksFile(full...))
	if _, _, err := store.Keep(aHighlight(), admitAny); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("a mark past the cap = %v, want ErrInvalid", err)
	}
	here := aQuestion()
	here.Anchor = "a0"
	if _, added, err := store.Keep(here, admitAny); err != nil || added {
		t.Errorf("Keep of a question that is kept at the cap = %v, %v, want it answered", added, err)
	}
}

// TestKeepKeepsOneHighlightWhenTwoArriveTogether is what makes the check and
// the write one step: a page that sends a highlight twice at once keeps one.
func TestKeepKeepsOneHighlightWhenTwoArriveTogether(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	const arrivals = 16
	var group sync.WaitGroup
	added := make(chan bool, arrivals)
	for range arrivals {
		group.Go(func() {
			_, isNew, err := store.Keep(aHighlight(), admitAny)
			if err != nil {
				t.Error(err)
			}
			added <- isNew
		})
	}
	group.Wait()
	close(added)
	news := 0
	for isNew := range added {
		if isNew {
			news++
		}
	}
	if held := listed(t, store); len(held) != 1 || news != 1 {
		t.Errorf("%d marks are kept and %d arrivals were new, want 1 and 1", len(held), news)
	}
}
