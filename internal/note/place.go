package note

import (
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// HasPlace reports whether rel is a note this snapshot lets a reader open and
// anchor is a place that note renders: one of its headings, the title standing
// in for an opening heading, or a slot-machine card of its drill. An empty
// anchor names the note as a whole. Heading ids do not depend on the interface
// language, so any one renders the same set.
func (h *Handler) HasPlace(rel, anchor string) bool {
	return newPlaceResolver(h.sources.Snapshot().Capture()).hasPlace(rel, anchor)
}

// placeResolver owns a request's captured generation and lazily resolves each
// shown note once. A note's complete rendered place set is built at most once.
type placeResolver struct {
	snap  *snapshot.Generation
	notes map[string]*resolvedPlaces
}

type resolvedPlaces struct {
	reading  snapshot.Reading
	readable bool
	anchors  map[string]bool
}

func newPlaceResolver(snap *snapshot.Generation) *placeResolver {
	return &placeResolver{snap: snap, notes: make(map[string]*resolvedPlaces)}
}

func (p *placeResolver) note(rel string) *resolvedPlaces {
	if resolved, ok := p.notes[rel]; ok {
		return resolved
	}
	n, ok := readableNote(p.snap, rel)
	resolved := &resolvedPlaces{reading: n, readable: ok}
	p.notes[rel] = resolved
	return resolved
}

func (p *placeResolver) hasPlace(rel, anchor string) bool {
	n := p.note(rel)
	if !n.readable {
		return false
	}
	if anchor == "" {
		return true
	}
	if n.anchors == nil {
		n.anchors = make(map[string]bool)
		result := p.snap.Render(rel, n.reading.Body, wording.ZhHant)
		if result.TitleAnchor != "" {
			n.anchors[result.TitleAnchor] = true
		}
		for _, heading := range result.TOC {
			n.anchors[heading.ID] = true
		}
		if sidecar, found := p.snap.Slots().Lookup(n.reading.Slug); found {
			for i := range sidecar.Patterns {
				n.anchors[pages.SlotPatternID(i)] = true
			}
		}
	}
	return n.anchors[anchor]
}
