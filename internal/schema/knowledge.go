package schema

import (
	"slices"
	"strings"
)

// KnowledgeScope is the set of top-level directories a vault calls its
// knowledge layer, separating the notes its owner reads from generated
// reports, templates and machinery. A folder that declares nothing has no
// scope, and then nothing is outside it.
type KnowledgeScope struct {
	dirs  []string
	claim Claim
}

// Claim reports how far the scan declaration got.
func (s KnowledgeScope) Claim() Claim {
	return s.claim
}

// Available reports whether the contract declared a knowledge layer.
func (s KnowledgeScope) Available() bool {
	return s.claim.held()
}

// Includes reports whether a vault-relative path is inside the knowledge layer,
// and true for everything when no scope was declared: an undeclared set
// excludes nothing. Only the first path segment is consulted, folded the way
// every other vault directory scope folds one.
func (s KnowledgeScope) Includes(relPath string) bool {
	if !s.Available() {
		return true
	}
	top, _, _ := strings.Cut(relPath, "/")
	return slices.ContainsFunc(s.dirs, func(dir string) bool { return SameDirName(top, dir) })
}

// deriveKnowledgeScope reads the declared knowledge layer. A contract that
// declares none leaves the scope unresolved rather than empty, because an empty
// one would match no directory and hide the whole vault; an unresolved scope
// includes everything until a contract narrows it.
//
// Unclaimed is this package's word for the opposite state — nothing was ever
// asserted — and it is not what happens here.
func deriveKnowledgeScope(dirs []string) KnowledgeScope {
	if len(dirs) == 0 {
		return KnowledgeScope{claim: Rejected("")}
	}
	return KnowledgeScope{dirs: slices.Clone(dirs), claim: heldClaim()}
}
