package pages

import (
	"fmt"
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

// searchCount is the one sentence a list of hits says about its own extent. A
// divided answer names which of the hits are on this page and how many there
// are in all: the tally alone would be true and useless on the second page,
// and "the first twenty" would be false there.
//
// Every other face says the tally, and says how far the list was cut where
// something cut it — the palette floats over a page with no strip under its
// rows, so nothing else there would say the list stops short of the answer.
func searchCount(v *SearchView, lang wording.Lang) string {
	if v.Pager.Number > 0 {
		return fmt.Sprintf(wording.ResultRangeFmt.In(lang), v.Pager.First+1, v.Pager.Last, v.Pager.Total)
	}
	return resultCount(len(v.Results), v.Total, lang)
}

// resultCount names how many hits a search returned. Where the list holds only
// the opening stretch of a larger answer it says both numbers, so the count
// never claims the page shows more than it does.
func resultCount(shown, total int, lang wording.Lang) string {
	if total > shown {
		return fmt.Sprintf(wording.ResultCountShownFmt.In(lang), total, shown)
	}
	return plural(total, wording.ResultCountOne, wording.ResultCountMany, lang)
}
