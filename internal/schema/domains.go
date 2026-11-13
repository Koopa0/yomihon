package schema

import "slices"

// Domains is where a vault's notes get their domain: the first folder below a
// root the contract names under folders.domain_from. A note never writes its
// domain, and a note outside every root has none. The zero value names no root,
// so no note has a domain.
type Domains struct {
	roots []string
}

// Domains returns the contract's domain roots as a capability a face can hold
// without holding the contract.
func (c *Contract) Domains() Domains {
	if c == nil {
		return Domains{}
	}
	return Domains{roots: slices.Clone(c.folders.DomainFrom)}
}

// Of returns the domain of the note at relPath, empty when the path sits
// outside every root or directly under one.
func (d Domains) Of(relPath string) string {
	folder, _ := DomainFolder(d.roots, relPath)
	return folder
}

// Roots returns the declared roots, cloned so a caller cannot change them.
func (d Domains) Roots() []string {
	return slices.Clone(d.roots)
}
