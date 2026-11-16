package note

import (
	"github.com/koopa0/yomihon/internal/schema"
)

// enumContract withholds vocabulary after the captured authority has closed;
// an old declaration must not read as advice about a changed contract.
func (h *Handler) enumContract(authority schema.Authority) *schema.Contract {
	if authority.Closed() {
		return nil
	}
	return h.sources.Contract
}

// enumValues reads the declaration that judged this field.
func enumValues(contract *schema.Contract, field string) []string {
	return contract.EnumValues(field)
}
