package judge

import (
	"context"
	"errors"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

var errPrivacyAuthorityUnavailable = errors.New("privacy authority unavailable; agent-facing output disabled")

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
