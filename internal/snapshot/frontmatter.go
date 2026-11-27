package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// NotesOfType returns captured readable notes of one declared type, including
// notes outside the knowledge layer but never a declared non-instance, such as
// a template. It grants no role meaning; the caller supplies the
// type obtained from its rules authority.
func (g *Generation) NotesOfType(noteType string) []Reading {
	if noteType == "" {
		return nil
	}
	var notes []Reading
	policy := g.ArtifactPolicy()
	for rel := range g.notes {
		reading := g.notes[rel]
		if schema.NormalizeWord(reading.Type) == noteType && !policy.IsNonInstance(reading.RelPath) {
			notes = append(notes, reading)
		}
	}
	slices.SortFunc(notes, func(a, b Reading) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
	return notes
}

// NotText reports whether rel's frontmatter writes key with a value that is
// not text — a list, a mapping, a number or a date — so that nothing can be
// read from it as that field. An absent or empty key wrote nothing to misread
// and answers false.
func (g *Generation) NotText(rel, key string) bool {
	if g.parsed[rel] == nil {
		return false
	}
	value, present := g.parsed[rel].Frontmatter[key]
	if !present || value == nil {
		return false
	}
	_, text := value.(string)
	return !text
}

// NotBoolean reports whether rel's frontmatter writes key with a value that is
// not a boolean — text, a number, a list — so that whether the flag is set
// cannot be read from it. An absent or empty key wrote nothing to misread and
// answers false.
func (g *Generation) NotBoolean(rel, key string) bool {
	if g.parsed[rel] == nil {
		return false
	}
	value, present := g.parsed[rel].Frontmatter[key]
	if !present || value == nil {
		return false
	}
	_, boolean := value.(bool)
	return !boolean
}
