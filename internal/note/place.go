package note

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/lesson"
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
	anchors  map[string]resolvedPlace
}

type resolvedPlace struct {
	label pages.RowPlace
	card  int
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
		n.anchors = make(map[string]resolvedPlace)
		result := p.snap.Render(rel, n.reading.Body, wording.ZhHant)
		if result.TitleAnchor != "" {
			n.anchors[result.TitleAnchor] = resolvedPlace{}
		}
		for _, heading := range result.TOC {
			n.anchors[heading.ID] = resolvedPlace{label: pages.RowPlace{Text: heading.Text, Language: n.reading.Language}}
		}
		if sidecar, found := p.snap.Slots().Lookup(n.reading.Slug); found {
			for i, pattern := range sidecar.Patterns {
				n.anchors[pages.SlotPatternID(i)] = resolvedPlace{
					label: pages.RowPlace{Text: lesson.AbstractTemplate(pattern.Template), Language: "ja"}, card: i + 1,
				}
			}
		}
	}
	_, found := n.anchors[anchor]
	return found
}

func (p *placeResolver) label(rel, anchor string, lang wording.Lang) pages.RowPlace {
	if !p.hasPlace(rel, anchor) {
		return pages.RowPlace{}
	}
	place := p.note(rel).anchors[anchor]
	if place.card > 0 && place.label.Text == "" {
		return pages.RowPlace{Text: fmt.Sprintf(wording.PracticeCardLabelFmt.In(lang), place.card), Language: lang.Tag()}
	}
	return place.label
}
