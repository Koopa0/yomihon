package schema_test

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"strconv"
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
	if claim.Trustworthy() {
		t.Error("Rejected() produced a claim a projection may be answered over")
	}
	// The narrower question — is there a declaration here to read — is asked
	// through a capability, in the words of what that capability governs. A
	// rejection is not a reading, whichever of the two a capability gates on.
	withheld := (*schema.Contract)(nil).Capabilities(schema.Unreadable(errors.New("unparsable")))
	if withheld.Navigation.Available() || withheld.Knowledge.Available() || withheld.Artifacts.Available() {
		t.Error("a rejected declaration left a capability reporting itself available")
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
	if got := silent.Diagnostic(); got != "" {
		t.Errorf("Diagnostic() = %q, want empty", got)
	}
}

// TestTheContractSpeaksNoInterfaceLanguage keeps the vocabulary the interface
// says things in out of the package that reads the contract. A contract is
// loaded once, before any reader has asked for anything, so a sentence built
// here would be built in whichever language was guessed at startup — and the
// reader who reads in the other one would meet it sitting under a label in
// theirs. The dependency is what makes that possible, so the dependency is
// what this test forbids.
func TestTheContractSpeaksNoInterfaceLanguage(t *testing.T) {
	t.Parallel()

	const forbidden = "github.com/koopa0/yomihon/internal/wording"
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("list the package source: %v", err)
	}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		scanned++
		for _, spec := range parsed.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("%s: %v", name, unquoteErr)
			}
			if path == forbidden {
				t.Errorf("%s imports %s: a sentence built here is built before any reader has asked for one", name, path)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no package source was read; the scan would prove nothing")
	}
}

// TestTheFourCapabilitiesAgreeAboutSilence states, in one place, what each
// capability answers for a folder that never declared it. Two questions sit
// under every capability — "was this read" and "may a projection over it be
// answered" — and they give opposite answers to silence, so which one a
// capability gates on decides what it says about a folder with nothing wrong
// with it. Nothing in the type system says which; four separate doc paragraphs
// did, and a fifth capability copied from a sibling would inherit whichever
// polarity was copied without a call site looking any different.
//
// The membership answers below are therefore the design, not an observation.
// Each is the true answer to silence for the thing that capability governs,
// and a change to one of them is a change to what an ungoverned folder is.
func TestTheFourCapabilitiesAgreeAboutSilence(t *testing.T) {
	t.Parallel()

	var (
		roles    schema.NavigationRoles
		scope    schema.KnowledgeScope
		artifact schema.ArtifactPolicy
		privacy  schema.PrivacyPolicy
	)

	// The shared half: silence is silence, whichever capability is asked.
	for _, c := range []struct {
		name  string
		claim schema.Claim
	}{
		{"NavigationRoles", roles.Claim()},
		{"KnowledgeScope", scope.Claim()},
		{"ArtifactPolicy", artifact.Claim()},
		{"PrivacyPolicy", privacy.Claim()},
	} {
		if c.claim.Claimed() {
			t.Errorf("%s reports that something asserted it; nothing did", c.name)
		}
		if !c.claim.Trustworthy() {
			t.Errorf("%s refuses a projection over an undeclared set; empty is the true answer, not unanswerable", c.name)
		}
		if c.claim.Diagnostic() != "" {
			t.Errorf("%s has news about a folder that declared nothing: %q", c.name, c.claim.Diagnostic())
		}
		if c.claim.Reason() != schema.ReasonUnstated {
			t.Errorf("%s names a reason for a rejection that never happened", c.name)
		}
	}

	// Available is the narrow question, and all four answer it the same way.
	if roles.Available() || scope.Available() || artifact.Available() || privacy.Available() {
		t.Error("a capability reports a declaration was read where none was written")
	}

	// The differing half: which question each capability's own membership
	// gates on, and why that is the true answer to silence for it.
	if !scope.Includes("System/reports/one.md") {
		t.Error("an undeclared knowledge layer excludes a file: hiding files on a folder that never claimed a layout invents a rule its owner did not write")
	}
	if artifact.IsNonInstance("System/templates/Card.md") {
		t.Error("an undeclared artifact policy excludes a directory: an exclusion nobody wrote excludes nothing")
	}
	// Silence alone cannot pin the artifact policy's gate: it also refuses
	// when it holds no derived state, which is every unclaimed policy a
	// contract can produce. The state where its gate is the only thing
	// answering is a policy that was read cleanly and whose source then
	// changed, and the staleness test is where that is held. What silence can
	// say is the fail-closed direction, which is what is pinned here.
	//
	// The navigation roles are gated too, and that gate is unobservable
	// everywhere: the role sets are empty in every state but a clean reading,
	// so the membership answer is already false before the gate is asked.
	if withheld := (*schema.Contract)(nil).Capabilities(schema.Unreadable(errors.New("unparsable"))).Artifacts; withheld.IsNonInstance("System/templates/Card.md") {
		t.Error("a rejected artifact policy classifies a file: the set the operator intended is unknown, not empty")
	}
	if privacy.EgressAllowed("Notes/a.md") {
		t.Error("an undeclared privacy policy permits egress: permission is positive authority, never the absence of a deny match")
	}
	if roles.IsPathType("study-path") || roles.IsMapType("moc") {
		t.Error("undeclared navigation roles classify a type: a role nobody declared belongs to no type")
	}
}
