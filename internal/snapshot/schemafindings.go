package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/judge"
)

// SchemaFindings returns what the schema said about relPath's frontmatter when
// this generation read it — the same verdict the check command reaches over
// the same bytes, so a reader and a command are never told different things
// about one file. A note the schema is content with, and a path this
// generation does not hold, both answer with nothing.
//
// The returned slice is the caller's own. The findings in it are read-only:
// they carry pointers into what this generation still holds, so a slice that
// is safe to keep is not a licence to write through what it points at.
func (v *View) SchemaFindings(relPath string) []judge.Finding {
	if v == nil {
		return nil
	}
	return slices.Clone(v.schemaFindings[relPath])
}
