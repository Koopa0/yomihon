package schema

import (
	"slices"
	"strings"
)

// KnowledgeScope is the set of top-level directories a vault calls its
// knowledge layer. It exists because a folder holds more than the notes its
// owner reads: generated reports, templates, machinery. Every face that shows
// "what is in this vault" needs the same answer to that, and the contract is
// where the vault already gives it.
//
// A folder that declares nothing has no scope, and then nothing is out of it —
// an ordinary directory is all knowledge, which is the only true answer
// available when nobody said otherwise.
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

// Includes reports whether a vault-relative path is inside the knowledge layer.
// It answers true for everything when no scope was declared: an undeclared set
// excludes nothing, and a reading surface that hid files on a folder which
// never claimed a layout would be inventing a rule its owner did not write.
//
// Only the first path segment is consulted, and it is compared with the same
// fold every other vault directory scope uses: on a case-insensitive
// filesystem one directory answers to every case spelling of its name, so a
// membership that turned on the spelling would place the same note inside the
// layer or outside it by coincidence.
func (s KnowledgeScope) Includes(relPath string) bool {
	if !s.Available() {
		return true
	}
	top, _, _ := strings.Cut(relPath, "/")
	return slices.ContainsFunc(s.dirs, func(dir string) bool { return SameDirName(top, dir) })
}

// deriveKnowledgeScope reads the declared knowledge layer. A contract that
// exists and declares none leaves the scope unclaimed rather than empty: an
// empty scope would hide the whole vault, which is never what silence means.
func deriveKnowledgeScope(dirs []string) KnowledgeScope {
	if len(dirs) == 0 {
		return KnowledgeScope{claim: Rejected("")}
	}
	return KnowledgeScope{dirs: slices.Clone(dirs), claim: heldClaim()}
}
