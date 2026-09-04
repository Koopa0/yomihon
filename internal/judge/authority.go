package judge

import (
	"context"
	"errors"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// The two refusals a caller has to be able to tell apart. Both mean the same
// thing to the engine — no vocabulary to judge notes against — and different
// things to whoever pointed the command at this folder, who is owed a different
// paragraph for each. Distinguishing them is the caller's job, so they are
// exported; the paragraphs are the binary's, written for a person at a terminal.
var (
	// ErrPrivacyAuthorityUnavailable reports a contract that exists and could
	// not be honoured. The reason is deliberately not carried: a decoder's
	// account of an existing contract's keys would quote vault content back out
	// under exactly the policy that is missing.
	ErrPrivacyAuthorityUnavailable = errors.New("privacy authority unavailable; agent-facing output disabled")

	// ErrNoVaultContract is the other half of the same silence. A folder that
	// carries no contract asserted nothing: it is not broken, it simply has no
	// vocabulary for these commands to judge in, and saying so is a different
	// sentence from saying a declaration could not be honoured.
	ErrNoVaultContract = errors.New("this folder has no vault contract, so there is nothing to judge notes against")
)

// scanAuthority is the contract state that permits one agent-facing judge
// action. Every command loads it before reading vault notes and validates the
// same source again after constructing its payload.
type scanAuthority struct {
	contract *schema.Contract
	privacy  schema.PrivacyPolicy
}

// domainRoots are the folders the contract declares a note's own folder names
// its knowledge domain under. The reports group by them, and the frontmatter
// rule enforces the same declaration, so both read one key rather than each
// carrying a copy of this vault's layout.
func (a scanAuthority) domainRoots() domainRoots {
	if a.contract == nil {
		return nil
	}
	return a.contract.Definition().Rules.DomainEqualsFolderUnder
}

func loadScanAuthority(ctx context.Context, reader *vaultfs.Reader) (scanAuthority, error) {
	contract, err := schema.LoadReader(ctx, reader)
	if err != nil {
		if schema.ContractAbsent(err) {
			return scanAuthority{}, ErrNoVaultContract
		}
		// The decoder's own words are deliberately dropped. A parse failure
		// names keys from the contract, and this face exists to hand its output
		// to an agent, so explaining the fault here would send vault content
		// out under exactly the policy that is missing. The operator reads the
		// cause where reading is the point: the server states it on the page
		// and logs it at startup.
		return scanAuthority{}, ErrPrivacyAuthorityUnavailable
	}
	authority := scanAuthority{
		contract: contract,
		privacy:  contract.PrivacyPolicy(),
	}
	if err := authority.validate(); err != nil {
		return scanAuthority{}, err
	}
	return authority, nil
}

// roles answers which note types take part in path and map behaviour. The
// judge asks the contract rather than naming a type, so a vault that calls
// its courses something else is read the same way the reader reads it.
func (a scanAuthority) roles() schema.NavigationRoles {
	return a.contract.NavigationRoles()
}

func (a scanAuthority) validate() error {
	privacy := a.privacy.ValidateSource()
	if !privacy.Available() {
		return ErrPrivacyAuthorityUnavailable
	}
	return nil
}

func (a scanAuthority) egressAllowed(relPath string) bool {
	return a.privacy.EgressAllowed(relPath)
}

// conceptType is the note type this vault files as its distilled ideas, and
// whether the contract declares it. The name comes from the contract layer for
// the same reason the map and path roles do: a vault names its own vocabulary,
// and a judge that spelled the word itself would judge a vault that never uses
// it against a corpus it does not have.
func (a scanAuthority) conceptType() (name string, declared bool) {
	return a.contract.ConceptType()
}

// declaresType reports whether the contract lists noteType at all.
func (a scanAuthority) declaresType(noteType string) bool {
	return a.contract.DeclaresType(noteType)
}
