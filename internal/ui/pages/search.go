package pages

import (
	"fmt"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// unlocatedNote is the sentence a row carries when the index found a match
// the opened page cannot locate. Empty for every other row, including a
// crossing match that still has a first-block landing, so a result that
// already points at the page says nothing extra.
func unlocatedNote(r *SearchResult, lang wording.Lang) string {
	if !r.BlockCrossing || strings.TrimSpace(r.Landing) != "" {
		return ""
	}
	return wording.SearchHitUnlocated.In(lang)
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
