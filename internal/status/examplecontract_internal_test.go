package status

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// exampleContractPath is the tracked starting-point contract that the install
// instructions tell a new operator to copy into their own vault.
var exampleContractPath = filepath.Join("..", "..", "examples", "vault-schema.toml")

// TestShippedExampleContractLoadsAndOwnsEveryStatusAsTheWriteActor holds the
// tracked starting-point contract to the two promises made about it: it decodes
// under the same strict reader a real vault contract goes through, and every
// status it declares is owned by the single actor this binary writes as.
//
// Both promises are load-bearing because the file is copied, not imported. A
// starting point that no longer parses hands a new operator a vault that cannot
// be judged, and one naming any other owner hands them a write face that
// refuses every transition while naming an actor they never chose.
//
// Every status the example declares is also required to be settable. That is a
// property of this file rather than of contracts in general: a mature contract
// may legally scope a status to a subset of the types sharing its status group,
// and the vault this tool was built for does exactly that. A starting point
// that offers a status no lifecycle row can reach is a different thing — it
// teaches the shape wrong on the first read. Scoping a status here later is a
// deliberate choice, and this test is where that choice has to be made.
func TestShippedExampleContractLoadsAndOwnsEveryStatusAsTheWriteActor(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(exampleContractPath)
	if err != nil {
		t.Fatalf("schema.LoadFile(%q) error = %v, want the tracked example to decode", exampleContractPath, err)
	}

	noteTypes := contract.Definition().Enums.Type
	if len(noteTypes) == 0 {
		t.Fatal("the tracked example declares no note types, so no owner would be examined")
	}

	owners := 0
	for _, noteType := range noteTypes {
		statuses := contract.Statuses(noteType)
		if len(statuses) == 0 {
			t.Errorf("Statuses(%q) is empty, so that type carries no status to own", noteType)
			continue
		}
		for _, status := range statuses {
			stage, ok := contract.Stage(noteType, status)
			if !ok {
				t.Errorf("Stage(%q, %q) is absent: the example declares a status no lifecycle row can set", noteType, status)
				continue
			}
			if want := []string{actor}; !slices.Equal(stage.Owner, want) {
				t.Errorf("Stage(%q, %q).Owner = %v, want %v", noteType, status, stage.Owner, want)
			}
			owners++
		}
	}

	if owners == 0 {
		t.Fatal("no lifecycle owner was examined, so this test asserts nothing")
	}

	// Owning a status is not the same as being able to reach it. A ready row
	// whose predecessor list is emptied still decodes, still resolves, and
	// still names the right owner, while every certification a new operator
	// attempts is refused; a lifecycle bent into a cycle with no entry point
	// passes those same checks and lets no note be created at all. Walking
	// the journey pins the topology the rows cannot.
	walk := []struct{ from, to string }{
		{"", "draft"},
		{"draft", "ready"},
		{"ready", "archived"},
	}
	walked := make(map[string]bool, len(walk))
	for _, step := range walk {
		walked[step.to] = true
	}

	for _, noteType := range noteTypes {
		for _, step := range walk {
			if err := contract.Transition(noteType, step.from, step.to, actor); err != nil {
				t.Errorf("Transition(%q, %q→%q, %q) = %v, want allowed: the starting point promises this walk",
					noteType, step.from, step.to, actor, err)
			}
		}
		// The walk is written out rather than derived, so it would stop
		// covering a status the moment one is added. Saying so here turns
		// that into a failure instead of a quiet gap.
		for _, status := range contract.Statuses(noteType) {
			if !walked[status] {
				t.Errorf("the example declares status %q for type %q but the walk never reaches it: extend the walk or drop the status",
					status, noteType)
			}
		}
	}
}
