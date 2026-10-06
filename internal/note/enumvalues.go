package note

import (
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
)

// enumContract withholds vocabulary after the captured authority has closed;
// an old declaration must not read as advice about a changed contract.
func (h *Handler) enumContract(authority status.Authority) *schema.Contract {
	if authority.Closed() {
		return nil
	}
	return h.sources.Contract
}

// enumValues reads the declaration that judged this field. An unknown type
// uses the general note group for the schema finding, as the checker does;
// the lifecycle panel separately withholds claims about that unknown type.
func enumValues(contract *schema.Contract, field, noteType string) []string {
	if contract == nil {
		return nil
	}
	switch field {
	case "status":
		return contract.StatusesInGroup(contract.JudgedStatusGroup(noteType))
	case "type":
		return contract.Definition().Enums.Type
	case "domain":
		return contract.Definition().Enums.Domain
	}
	return nil
}
