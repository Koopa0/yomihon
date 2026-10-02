package schema

import (
	"errors"
	"strings"
	"testing"
)

// settledContract writes settled on some rows, leaves it off one, and repeats
// a status across two rows, so each rule that could be dropped gives a
// different answer. ready is settled outright. archived says nothing, and
// draft appears for two types with the two rows disagreeing, which is the case
// a first-row-wins or last-row-wins reading would get wrong in one direction
// each.
const settledContract = `schema_version = "1"

[enums]
type = ["article", "lesson"]

[enums.status]
note = ["draft", "ready", "archived"]
lesson = ["draft", "ready", "archived"]

[fields.status_group]
lesson = ["lesson"]

[rules]
slug_pattern = "^[a-z]+$"

[[lifecycle]]
status = "draft"
applies_to = ["article"]
initial = true
from = ["ready"]
owner = ["author"]
settled = false

[[lifecycle]]
status = "draft"
applies_to = ["lesson"]
initial = true
from = ["ready"]
owner = ["author"]
settled = true

[[lifecycle]]
status = "ready"
applies_to = ["*"]
initial = false
from = ["draft"]
owner = ["author"]
settled = true

[[lifecycle]]
status = "archived"
applies_to = ["*"]
initial = false
from = ["*"]
owner = ["author"]
`

func TestASettledStatusIsReadOffTheLifecycleRow(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, settledContract)

	if !contract.DeclaresSettled() {
		t.Error("DeclaresSettled() = false, want true: three rows write the key")
	}
	for _, tc := range []struct {
		status string
		want   bool
		why    string
	}{
		{"ready", true, "its row says settled = true"},
		{"archived", false, "its row never mentions the key, and silence is not settled"},
		{"nonesuch", false, "no row names it"},
		{"", false, "an empty status names no row"},
	} {
		if got := contract.Settled(tc.status); got != tc.want {
			t.Errorf("Settled(%q) = %v, want %v: %s", tc.status, got, tc.want, tc.why)
		}
	}
}

// TestARepeatedStatusIsSettledWhenAnyRowSaysSo pins the rule for a status that
// several lifecycle rows name. The fixture has draft settled on the second of
// its two rows and unsettled on the first; the reversed fixture swaps them. A
// reading that took the first row, or the last, answers wrongly for one of the
// two.
func TestARepeatedStatusIsSettledWhenAnyRowSaysSo(t *testing.T) {
	t.Parallel()

	swapped := strings.Replace(settledContract, "settled = false", "settled = SWAP", 1)
	swapped = strings.Replace(swapped, "settled = true", "settled = false", 1)
	swapped = strings.Replace(swapped, "settled = SWAP", "settled = true", 1)
	if swapped == settledContract {
		t.Fatal("the swap matched nothing, so both orders would be the same fixture")
	}

	for name, source := range map[string]string{"settled on the later row": settledContract, "settled on the earlier row": swapped} {
		contract := decodeLifecycleFixture(t, source)
		if !contract.Settled("draft") {
			t.Errorf("%s: Settled(draft) = false, want true: one of the two rows naming it declares it settled", name)
		}
	}
}

func TestAContractThatNeverWritesSettledSettlesNothing(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, explicitInitialContract)

	if contract.DeclaresSettled() {
		t.Error("DeclaresSettled() = true, want false: no row writes the key")
	}
	for group, statuses := range contract.definition.Enums.Status {
		for _, status := range statuses {
			if contract.Settled(status) {
				t.Errorf("Settled(%q) in group %q = true, want false: a contract that declares nothing settles nothing, so every status stays marked", status, group)
			}
		}
	}
}

// TestSettledFalseEverywhereStillDeclares keeps "wrote nothing" apart from
// "wrote that nothing is settled". The second is a statement a contract's
// author made, and a face that counts the notes not yet settled counts by it.
func TestSettledFalseEverywhereStillDeclares(t *testing.T) {
	t.Parallel()

	allFalse := strings.ReplaceAll(settledContract, "settled = true", "settled = false")
	if allFalse == settledContract {
		t.Fatal("the edit matched nothing, so this is the original contract")
	}
	contract := decodeLifecycleFixture(t, allFalse)

	if !contract.DeclaresSettled() {
		t.Error("DeclaresSettled() = false, want true: rows write the key, as false")
	}
	for _, status := range []string{"draft", "ready", "archived"} {
		if contract.Settled(status) {
			t.Errorf("Settled(%q) = true, want false: every row that writes the key writes false", status)
		}
	}
}

func TestSettledFoldsTheStatusItIsAsked(t *testing.T) {
	t.Parallel()

	// The contract spells the word composed; a filename-derived value arrives
	// decomposed. Both are built from numeric escapes so the source stays ASCII.
	composed, decomposed := "café", "café"
	if composed == decomposed {
		t.Fatal("the two spellings are the same bytes, so folding is not what this tests")
	}
	source := strings.NewReplacer("ready", composed).Replace(settledContract)
	contract := decodeLifecycleFixture(t, source)

	if !contract.Settled(composed) {
		t.Fatalf("Settled(%q) = false, want true: the contract's own spelling", composed)
	}
	if !contract.Settled(decomposed) {
		t.Errorf("Settled(%q) = false, want true: the same word spelled the way the filesystem hands it over", decomposed)
	}
}

func TestSettledAcceptsNoOtherLifecycleKeyAndOnlyABoolean(t *testing.T) {
	t.Parallel()

	t.Run("a typo beside it is still refused", func(t *testing.T) {
		t.Parallel()
		data := replaceLifecycleRowText(t, settledContract, 3, "settled = true", "settled = true\nsettledd = true")
		assertContractError(t, data, `unknown core keys: "lifecycle.settledd"`)
	})

	t.Run("a word is not a boolean", func(t *testing.T) {
		t.Parallel()
		data := replaceLifecycleRowText(t, settledContract, 3, "settled = true", `settled = "yes"`)
		_, err := decodeContract([]byte(data), policySource{})
		if err == nil {
			t.Fatal("decodeContract(settled = \"yes\") error = nil, want a hard error: the contract would otherwise load with the key silently ignored")
		}
		if ContractAbsent(err) {
			t.Errorf("decodeContract(settled = \"yes\") = %v, which reads as an absent contract rather than a malformed one", err)
		}
	})
}

func TestSettlementTravelsWithTheCapabilitiesOfATrustedContract(t *testing.T) {
	t.Parallel()
	contract := decodeLifecycleFixture(t, settledContract)

	trusted := contract.Capabilities(contract.Governance()).Settlement
	if !trusted.Declared() || !trusted.Settled("ready") {
		t.Errorf("trusted capabilities Settlement = declared %v, ready settled %v, want both true", trusted.Declared(), trusted.Settled("ready"))
	}
	withheld := contract.Capabilities(Unreadable(errors.New("unparsable"))).Settlement
	if withheld.Declared() || withheld.Settled("ready") {
		t.Errorf("withheld capabilities Settlement = declared %v, ready settled %v, want neither: a contract that could not be read declares nothing", withheld.Declared(), withheld.Settled("ready"))
	}
}

func TestANilContractDeclaresNothingSettled(t *testing.T) {
	t.Parallel()

	var contract *Contract
	if contract.DeclaresSettled() || contract.Settled("ready") || contract.Settlement().Declared() {
		t.Error("a nil contract declares or settles something, want nothing")
	}
}
