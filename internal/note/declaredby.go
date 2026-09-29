package note

import (
	"html"
	"net/url"
	"regexp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// declaredBlockSpan reads only the span shape the renderer gives a block
// address. Arbitrary element ids, escaped code examples and unanchored marker
// spans do not establish a source location.
var declaredBlockSpan = regexp.MustCompile(`<span id="([^"]+)">([^<]*)</span>`)

// declaredBy keeps the declaring note and the places it names separate: the
// note opens its own file, while each location returns to this rendered source.
// The supplied result is the page's already-qualified render; a missing place
// stays named without an address rather than being guessed from source text.
func declaredBy(snap *snapshot.Generation, rel string, result *render.Result, idPrefix string, lang wording.Lang) []pages.DeclaringNoteView {
	declarations := snap.DeclaredBy(rel)
	if len(declarations) == 0 {
		return nil
	}
	anchors := declaredSourceAnchors(result, idPrefix)
	whole := pages.ResumeHref(rel, "", 0)
	if result != nil && result.TitleAnchor != "" {
		whole = "#" + url.PathEscape(result.TitleAnchor)
	}
	out := make([]pages.DeclaringNoteView, 0, len(declarations))
	for _, declaration := range declarations {
		view := pages.DeclaringNoteView{Note: declaration.Note}
		for _, place := range declaration.Locations {
			location := pages.DeclaredPlaceView{Label: wording.WholeSource.In(lang), Href: whole}
			var id string
			switch {
			case place.Block != "":
				location.Label = "#^" + place.Block
				id = idPrefix + graph.FoldFragment("^"+place.Block)
			case place.Heading != "":
				location.Label = "#" + place.Heading
				id = idPrefix + graph.SectionID(place.Heading)
			}
			if id != "" {
				location.Href = ""
				if anchors[id] {
					location.Href = "#" + url.PathEscape(id)
				}
			}
			view.Locations = append(view.Locations, location)
		}
		out = append(out, view)
	}
	return out
}

// declaredSourceAnchors takes headings from the rendered outline and blocks
// from their generated spans. The visible marker must name the same folded
// address as its id, so an unrelated span cannot stand in for a named block.
func declaredSourceAnchors(result *render.Result, idPrefix string) map[string]bool {
	anchors := make(map[string]bool)
	if result == nil {
		return anchors
	}
	if result.TitleAnchor != "" {
		anchors[result.TitleAnchor] = true
	}
	for _, heading := range result.TOC {
		anchors[heading.ID] = true
	}
	for _, match := range declaredBlockSpan.FindAllStringSubmatch(result.HTML, -1) {
		id := html.UnescapeString(match[1])
		marker := html.UnescapeString(match[2])
		if len(marker) > 1 && marker[0] == '^' && id == idPrefix+graph.FoldFragment(marker) {
			anchors[id] = true
		}
	}
	return anchors
}
