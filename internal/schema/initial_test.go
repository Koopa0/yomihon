package schema

import (
	"errors"
	"strings"
	"testing"
)

// explicitInitialContract declares initial on every lifecycle row, and is built
// so that two rows disagree with what the older reading would have concluded
// from from alone: article draft is an initial status while naming a
// predecessor, and archived is not one while accepting any predecessor. A
// fixture where the two readings agree would pass under either and prove
// nothing.
const explicitInitialContract = `schema_version = "1"

[enums]
type = ["article", "lesson", "system"]

[enums.status]
note = ["draft", "review", "archived"]
lesson = ["draft", "ready", "published", "archived"]
system = ["active", "archived"]

[fields]
known = ["related"]
lesson_only = ["predecessor", "successors"]

[fields.status_group]
lesson = ["lesson"]
system = ["system"]

[rules]
slug_pattern = "^[a-z]+$"

[[lifecycle]]
status = "draft"
applies_to = ["article"]
initial = true
from = ["review"]
owner = ["editor"]

[[lifecycle]]
status = "review"
applies_to = ["article"]
initial = false
from = ["draft"]
owner = ["editor"]

[[lifecycle]]
status = "draft"
applies_to = ["lesson"]
initial = true
from = ["ready"]
owner = ["editor"]

[[lifecycle]]
status = "ready"
applies_to = ["lesson"]
initial = false
from = ["draft"]
owner = ["koopa"]

[[lifecycle]]
status = "published"
applies_to = ["lesson"]
initial = false
from = ["ready"]
owner = ["koopa"]

[[lifecycle]]
status = "active"
applies_to = ["system"]
initial = true
from = []
owner = ["system"]

[[lifecycle]]
status = "archived"
applies_to = ["*"]
initial = false
from = ["*"]
owner = []
`

// TestInitialIsDeclaredNotInferred locks the whole point of the key: a row says
// whether a note may start there, and the predecessor list says nothing about
// it either way. Both cases below are ones the older reading got backwards, so
// each fails on a contract that still infers.
func TestInitialIsDeclaredNotInferred(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, explicitInitialContract)

	if err := contract.Transition("article", "", "draft"); err != nil {
		t.Errorf("Transition(article, initial, draft) = %v, want nil: the row declares initial = true, and naming a predecessor does not take that away", err)
	}
	if err := contract.Transition("article", "", "archived"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("Transition(article, initial, archived) = %v, want %v: the row declares initial = false, and accepting any predecessor does not make it a starting point", err, ErrIllegalTransition)
	}
	if err := contract.Transition("system", "", "active"); err != nil {
		t.Errorf("Transition(system, initial, active) = %v, want nil", err)
	}
	if err := contract.Transition("article", "", "review"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("Transition(article, initial, review) = %v, want %v", err, ErrIllegalTransition)
	}
}

// TestNarrowedLessonDraftRefusesPublished locks the edge the migration removes.
// A lesson may return to draft from ready, and from nowhere else; the older
// wildcard let a published lesson walk back, which contradicts what publishing
// means.
//
// It reads the predecessor list and never the initial flag, so it says nothing
// about whether a row's starting-point declaration is honoured: with the flag
// computed by the older inference instead of read from the row, this passes
// unchanged while the case above fails. Counting it among the locks on the
// declared reading would overstate what is covered.
func TestNarrowedLessonDraftRefusesPublished(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, explicitInitialContract)

	if err := contract.Transition("lesson", "ready", "draft"); err != nil {
		t.Errorf("Transition(lesson, ready, draft) = %v, want nil", err)
	}
	if err := contract.Transition("lesson", "published", "draft"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("Transition(lesson, published, draft) = %v, want %v", err, ErrIllegalTransition)
	}
}

// TestInitialKeyMustBeAllRowsOrNone locks the transition between the two
// readings. A contract that declares the key on some rows and not others has
// two meanings at once, and the half without it would be read by inference
// while the half with it would be read as written.
func TestInitialKeyMustBeAllRowsOrNone(t *testing.T) {
	t.Parallel()

	mixed := strings.Replace(explicitInitialContract, "applies_to = [\"article\"]\ninitial = false\nfrom = [\"draft\"]", "applies_to = [\"article\"]\nfrom = [\"draft\"]", 1)
	if mixed == explicitInitialContract {
		t.Fatal("the fixture edit matched nothing, so this case would pass without ever mixing the key")
	}
	_, err := decodeContract([]byte(mixed), policySource{})
	if err == nil {
		t.Fatal("decodeContract() with the key on some rows only = nil, want an error naming the mix")
	}
	// The rule, not the key. A contract that names initial nowhere is rejected
	// by a decoder that has never heard of the key at all, and that refusal
	// says `unknown core keys: "lifecycle.initial"` — which contains the word
	// and would satisfy any assertion looking for it, letting this case pass
	// against a build where the whole feature is missing.
	if !strings.Contains(err.Error(), "on every row or on none") {
		t.Errorf("decodeContract() error = %v, want the one that states the all-or-none rule; any other error means this case is passing for a reason it does not name", err)
	}
}

// TestInitialFalseWithNoPredecessorIsRefused locks the loader gate. A status
// that may not be a starting point and names no predecessor cannot be reached
// by any route, so a contract declaring one is stating something it cannot
// mean.
func TestInitialFalseWithNoPredecessorIsRefused(t *testing.T) {
	t.Parallel()

	unreachable := strings.Replace(explicitInitialContract, "status = \"review\"\napplies_to = [\"article\"]\ninitial = false\nfrom = [\"draft\"]", "status = \"review\"\napplies_to = [\"article\"]\ninitial = false\nfrom = []", 1)
	if unreachable == explicitInitialContract {
		t.Fatal("the fixture edit matched nothing, so this case would pass without ever declaring an unreachable status")
	}
	_, err := decodeContract([]byte(unreachable), policySource{})
	if err == nil {
		t.Fatal("decodeContract() with a status that is neither initial nor reachable = nil, want an error")
	}
	// The condition, not the status name. Naming only the status would be
	// satisfied by any refusal that happens to mention it, including one about
	// a different rule entirely.
	if !strings.Contains(err.Error(), "nothing could ever reach it") {
		t.Errorf("decodeContract() error = %v, want the one that states the status is unreachable", err)
	}
	if !strings.Contains(err.Error(), "review") {
		t.Errorf("decodeContract() error = %v, does not say which status it is about", err)
	}
}

// TestContractWithoutInitialKeysKeepsTheOlderReading locks the transition path:
// a contract that names the key nowhere is read exactly as it was before, so a
// vault can be migrated after the code that understands the key is in place
// rather than at the same moment.
func TestContractWithoutInitialKeysKeepsTheOlderReading(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, lifecycleContract)

	if err := contract.Transition("article", "", "draft"); err != nil {
		t.Errorf("Transition(article, initial, draft) = %v, want nil: an empty predecessor list still means a starting point here", err)
	}
	if err := contract.Transition("article", "", "archived"); err != nil {
		t.Errorf("Transition(article, initial, archived) = %v, want nil: a wildcard predecessor list still means a starting point here", err)
	}
	if err := contract.Transition("article", "", "review"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("Transition(article, initial, review) = %v, want %v", err, ErrIllegalTransition)
	}
}
