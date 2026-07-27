package judge

import (
	"context"
	"errors"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

var (
	errPrivacyAuthorityUnavailable = errors.New("privacy authority unavailable; agent-facing output disabled")

	// errNoVaultContract is the other half of the same silence. A folder that
	// carries no contract asserted nothing: it is not broken, it simply has no
	// vocabulary for these commands to judge in, and saying so is a different
	// sentence from saying a declaration could not be honoured.
	errNoVaultContract = errors.New("this folder has no vault contract, so there is nothing to judge notes against")
)

// scanAuthority is the contract state that permits one agent-facing judge
// action. Every command loads it before reading vault notes and validates the
// same source again after constructing its payload.
type scanAuthority struct {
	contract *schema.Contract
	privacy  schema.PrivacyPolicy
}

func loadScanAuthority(ctx context.Context, reader *vault.Reader) (scanAuthority, error) {
	contract, err := schema.LoadReader(ctx, reader)
	if err != nil {
		if schema.ContractAbsent(err) {
			return scanAuthority{}, errNoVaultContract
		}
		// The decoder's own words are deliberately dropped. A parse failure
		// names keys from the contract, and this face exists to hand its output
		// to an agent, so explaining the fault here would send vault content
		// out under exactly the policy that is missing. The operator reads the
		// cause where reading is the point: the server states it on the page
		// and logs it at startup.
		return scanAuthority{}, errPrivacyAuthorityUnavailable
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

func (a scanAuthority) validate() error {
	privacy := a.privacy.ValidateSource()
	if !privacy.Available() {
		return errPrivacyAuthorityUnavailable
	}
	return nil
}

func (a scanAuthority) egressAllowed(relPath string) bool {
	return a.privacy.EgressAllowed(relPath)
}
