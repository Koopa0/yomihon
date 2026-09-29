package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// NotesOfType returns captured readable notes of one declared type, including
// notes outside the knowledge layer. It grants no role or lifecycle meaning;
// the caller supplies the type obtained from its contract authority.
func (g *Generation) NotesOfType(noteType string) []Reading {
	if g == nil || noteType == "" {
		return nil
	}
	var notes []Reading
	for _, reading := range g.notes {
		if schema.NormalizeWord(reading.Type) == noteType {
			notes = append(notes, reading)
		}
	}
	slices.SortFunc(notes, func(a, b Reading) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
	return notes
}

// BasedOnDeclarations returns the source words the note authored, including
// any section or block fragments. The detached list grants no link target.
func (g *Generation) BasedOnDeclarations(rel string) []string {
	if g == nil {
		return nil
	}
	return slices.Clone(basedOnValues(g.parsed[rel]))
}
