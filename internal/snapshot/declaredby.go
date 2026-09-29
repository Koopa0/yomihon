package snapshot

import (
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// DeclaredPlace is an authored location in the source a note names. Heading
// and Block retain the fragment text; both empty means the whole source file.
// Neither field asserts that the source still contains the named location.
type DeclaredPlace struct {
	Heading string
	Block   string
}

// DeclaringNote is a note that names a source in based_on, together with the
// distinct places it names in that source. Locations belong to the source,
// while Note is the separate note that made the declaration.
type DeclaringNote struct {
	Note      nav.NoteRef
	Locations []DeclaredPlace
}

// DeclaredBy returns the notes that uniquely name relPath in based_on, ordered
// by their displayed filename. Each note's locations retain declaration order;
// a repeated fragment or whole-file declaration appears only the first time.
// It reads only the reverse index's candidates and their captured frontmatter,
// so every answer belongs to this generation without scanning the vault again.
// The returned slice and every Locations slice belong to the caller.
func (g *Generation) DeclaredBy(relPath string) []DeclaringNote {
	if g == nil {
		return nil
	}
	var out []DeclaringNote
	for _, ref := range g.BasedOnBy(relPath) {
		if ref.RelPath == relPath {
			continue
		}
		places := declaredPlaces(g.parsed[ref.RelPath], g.graph, relPath)
		if len(places) > 0 {
			out = append(out, DeclaringNote{Note: ref, Locations: places})
		}
	}
	return out
}

func declaredPlaces(note *vault.Note, index *graph.Index, source string) []DeclaredPlace {
	var places []DeclaredPlace
	seen := make(map[DeclaredPlace]bool)
	for _, value := range basedOnValues(note) {
		ref, ok := declaredSource(index, value)
		if !ok || ref.RelPath != source {
			continue
		}
		inner := strings.TrimSpace(value)
		if rest, wrapped := strings.CutPrefix(inner, "[["); wrapped {
			if stripped, closed := strings.CutSuffix(rest, "]]"); closed {
				inner = stripped
			}
		}
		link, _ := graph.ParseWikilink(inner)
		place := DeclaredPlace{Heading: link.Heading, Block: link.Block}
		if !seen[place] {
			seen[place] = true
			places = append(places, place)
		}
	}
	return places
}
