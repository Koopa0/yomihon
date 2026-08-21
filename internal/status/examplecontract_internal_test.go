package status

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// exampleContractPath is the tracked starting-point contract that the install
// instructions tell a new operator to copy into their own vault.
var exampleContractPath = filepath.Join("..", "..", "examples", "vault-schema.toml")

// TestShippedExampleContractLoadsAndReachesEveryStatus holds the tracked
// starting-point contract to the two promises made about it: it decodes under
// the same strict reader a real vault contract goes through, and every status
// it declares is reachable through the lifecycle it teaches.
//
// Both promises are load-bearing because the file is copied, not imported. A
// starting point that no longer parses hands a new operator a vault that
// cannot be judged. Every status also has a lifecycle row and a non-empty
// owner list: the lists are declarative data — they gate nothing — but they
// are the vocabulary a waiting derivation intersects, and a starting point
// whose rows name nobody teaches the shape wrong on the first read.
//
// Declaring a status is not the same as being able to reach it. A ready row
// whose predecessor list is emptied still decodes and still resolves, while
// every certification a new operator attempts is refused; a lifecycle bent
// into a cycle with no entry point passes those same checks and lets no note
// be created at all. Walking the journey pins the topology the rows cannot.
//
// The example also has to declare its human owners so the home waiting
// panel it teaches actually renders: at least one status must be waiting on
// a person under that declaration, and at least one must not be, or the
// derivation it demonstrates distinguishes nothing.
func TestShippedExampleContractLoadsAndReachesEveryStatus(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(exampleContractPath)
	if err != nil {
		t.Fatalf("schema.LoadFile(%q) error = %v, want the tracked example to decode", exampleContractPath, err)
	}

	noteTypes := contract.Definition().Enums.Type
	if len(noteTypes) == 0 {
		t.Fatal("the tracked example declares no note types, so nothing would be examined")
	}

	rows := 0
	for _, noteType := range noteTypes {
		statuses := contract.Statuses(noteType)
		if len(statuses) == 0 {
			t.Errorf("Statuses(%q) is empty, so that type carries no status to reach", noteType)
			continue
		}
		for _, status := range statuses {
			stage, ok := contract.Stage(noteType, status)
			if !ok {
				t.Errorf("Stage(%q, %q) is absent: the example declares a status no lifecycle row can set", noteType, status)
				continue
			}
			if len(stage.Owner) == 0 {
				t.Errorf("Stage(%q, %q).Owner is empty: the starting point should name an owner for every row", noteType, status)
			}
			rows++
		}
	}

	if rows == 0 {
		t.Fatal("no lifecycle row was examined, so this test asserts nothing")
	}

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
			if err := contract.Transition(noteType, step.from, step.to); err != nil {
				t.Errorf("Transition(%q, %q→%q) = %v, want allowed: the starting point promises this walk",
					noteType, step.from, step.to, err)
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

	for _, noteType := range noteTypes {
		if !contract.AwaitsHuman(noteType, "draft") {
			t.Errorf("AwaitsHuman(%q, %q) = false, want a declared human owning the onward step; the example's waiting panel would show nothing", noteType, "draft")
		}
		if contract.AwaitsHuman(noteType, "archived") {
			t.Errorf("AwaitsHuman(%q, %q) = true, want false: nothing moves on from retirement, so a row here would invent a queue", noteType, "archived")
		}
	}
}
