package schema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestGovernanceSeparatesSilenceFromFault is the whole point of the grant
// split: before it, a folder carrying no contract and a folder whose contract
// could not be parsed produced the same nil contract and therefore the same
// page, so the operator's real mistake was reported exactly as loudly as their
// having made none — which is to say, both were shouted about.
func TestGovernanceSeparatesSilenceFromFault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		governance      schema.Governance
		wantGoverned    bool
		wantTrustworthy bool
		wantSentence    string
	}{
		{
			name:            "no contract file",
			governance:      schema.Ungoverned(),
			wantGoverned:    false,
			wantTrustworthy: true,
		},
		{
			name:            "contract exists and cannot be read",
			governance:      schema.Unreadable(errors.New("toml: line 42: expected a key separator")),
			wantGoverned:    true,
			wantTrustworthy: false,
			wantSentence:    "line 42",
		},
		{
			name:            "contract loaded",
			governance:      loadContractText(t, "", "").Governance(),
			wantGoverned:    true,
			wantTrustworthy: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.governance.Governed(); got != tt.wantGoverned {
				t.Errorf("Governed() = %t, want %t", got, tt.wantGoverned)
			}
			if got := tt.governance.Trustworthy(); got != tt.wantTrustworthy {
				t.Errorf("Trustworthy() = %t, want %t", got, tt.wantTrustworthy)
			}
			got := tt.governance.Diagnostic()
			if tt.wantSentence == "" && got != "" {
				t.Errorf("Diagnostic() = %q, want silence", got)
			}
			if tt.wantSentence != "" && !strings.Contains(got, tt.wantSentence) {
				t.Errorf("Diagnostic() = %q, want substring %q", got, tt.wantSentence)
			}
		})
	}
}

// TestContractLeftAnInputOutStaysGoverned pins the case the rejected definition
// of "governed" got wrong. A contract that loaded and merely omitted a section
// still claimed authority over this vault, so the surfaces that depend on being
// governed stay on and the omission is reported as the fault it is — rather
// than the vault being treated as an ordinary unclaimed folder, which would
// show neither the governance surfaces nor the fault.
func TestContractLeftAnInputOutStaysGoverned(t *testing.T) {
	t.Parallel()

	contract := loadContractText(t, "", "")
	if !contract.Governance().Governed() {
		t.Fatal("a contract that omitted [artifacts] reports the vault ungoverned")
	}
	if !contract.Governance().Trustworthy() {
		t.Error("the contract itself was read cleanly; only one declaration inside it is unresolved")
	}
	if contract.ArtifactPolicy().Trustworthy() {
		t.Error("the omitted declaration is trustworthy; the operator meant to exclude directories yomihon cannot name")
	}
	if contract.ArtifactPolicy().Diagnostic() == "" {
		t.Error("the omitted declaration says nothing; a governed vault with no artifact policy is news")
	}
}

// TestRejectedClaimCanCloseButNeverOpen pins the asymmetry that makes the whole
// scheme fail-closed by construction: a caller outside this package can always
// refuse a capability and can never grant one.
func TestRejectedClaimCanCloseButNeverOpen(t *testing.T) {
	t.Parallel()

	claim := schema.Rejected("declared and rejected")
	if claim.Held() || claim.Trustworthy() {
		t.Error("Rejected() produced a usable claim")
	}
	if !claim.Claimed() {
		t.Error("Rejected() is unclaimed; something was asserted here")
	}
	if claim.Diagnostic() == "" {
		t.Error("Rejected() dropped the sentence it was given")
	}
}

// TestClosureDoesNotDependOnHavingASentence keeps the predicate and the message
// apart at the one place the difference is a type guarantee rather than an
// accident. Every production caller supplies a sentence today, so a predicate
// that secretly read the string would behave identically everywhere and no
// page-level test could tell — which is how a capability came to be answered
// from an empty string in the first place.
func TestClosureDoesNotDependOnHavingASentence(t *testing.T) {
	t.Parallel()

	silent := schema.Rejected("")
	if silent.Trustworthy() {
		t.Error("a rejected declaration with no sentence reports itself trustworthy")
	}
	if !silent.Claimed() {
		t.Error("a rejected declaration reports that nothing was ever claimed")
	}
	if silent.Held() {
		t.Error("a rejected declaration reports itself held")
	}
	if got := silent.Diagnostic(); got != "" {
		t.Errorf("Diagnostic() = %q, want empty", got)
	}
}
