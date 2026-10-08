package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// SchemaFindings returns what the schema said about relPath's frontmatter when
// this generation read it — the same verdict the check command reaches over the
// same bytes. A note the schema is content with, and a path this generation does
// not hold, both answer with nothing. The slice is the caller's own; the
// findings in it point into what the generation holds and are read-only.
func (g *Generation) SchemaFindings(relPath string) []judge.Finding {
	return g.SchemaResult(relPath).Findings
}

// SchemaResult returns the captured frontmatter verdict and its enum selector
// together. Absent, clean and unavailable notes return the zero result. The
// Findings slice belongs to the caller; its pointed-to fields remain read-only.
func (g *Generation) SchemaResult(relPath string) judge.FrontmatterResult {
	if g == nil {
		return judge.FrontmatterResult{}
	}
	result := g.schemaResults[relPath]
	result.Findings = slices.Clone(result.Findings)
	return result
}

// DomainFolder returns the first folder below a declared domain root for a
// canonical vault-relative file path, using the declaration that produced this
// generation's schema findings. A nil generation, an unmatched path, and a file
// directly below a root return ("", false).
func (g *Generation) DomainFolder(relPath string) (string, bool) {
	if g == nil {
		return "", false
	}
	return schema.DomainFolder(g.domainRoots, relPath)
}
