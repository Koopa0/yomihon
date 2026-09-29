package note

import (
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// HasPlace reports whether rel is a note this snapshot lets a reader open and
// anchor is a place that note renders: one of its headings, the title standing
// in for an opening heading, or a slot-machine card of its drill. An empty
// anchor names the note as a whole. Heading ids do not depend on the interface
// language, so any one renders the same set.
func (h *Handler) HasPlace(rel, anchor string) bool {
	snap := h.sources.Snapshot().Capture()
	n, ok := readableNote(snap, rel)
	if !ok {
		return false
	}
	if anchor == "" {
		return true
	}
	result := snap.Render(rel, n.Body, wording.ZhHant)
	if anchor == result.TitleAnchor {
		return true
	}
	for _, heading := range result.TOC {
		if heading.ID == anchor {
			return true
		}
	}
	if sidecar, found := snap.Slots().Lookup(n.Slug); found {
		for i := range sidecar.Patterns {
			if anchor == pages.SlotPatternID(i) {
				return true
			}
		}
	}
	return false
}
