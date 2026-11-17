package pages

import (
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// unlocatedNote is the sentence a row carries when the index found a match
// the opened page cannot locate. Empty for every other row, including a
// crossing match that still has a first-block or last-block landing, so a
// result that already points at the page says nothing extra.
func unlocatedNote(r *SearchResult, lang wording.Lang) string {
	if !r.BlockCrossing || strings.TrimSpace(r.Landing) != "" || strings.TrimSpace(r.LandingEnd) != "" {
		return ""
	}
	return wording.SearchHitUnlocated.In(lang)
}

// searchCount is what the answer says of its own extent: how many notes it
// holds in all, whatever part of them the list has read so far.
func searchCount(v *SearchView, lang wording.Lang) string {
	return noteCountFact(v.Total, lang)
}
