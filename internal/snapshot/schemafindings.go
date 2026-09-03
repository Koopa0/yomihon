package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/judge"
)

// SchemaFindings returns what the schema said about relPath's frontmatter when
// this generation read it — the same verdict the check command reaches over the
// same bytes. A note the schema is content with, and a path this generation does
// not hold, both answer with nothing. The slice is the caller's own; the
// findings in it point into what the generation holds and are read-only.
func (v *Generation) SchemaFindings(relPath string) []judge.Finding {
	if v == nil {
		return nil
	}
	return slices.Clone(v.schemaFindings[relPath])
}
