package pages

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/wording"
)

// resultCount names how many hits a search returned. Where the list holds only
// the opening stretch of a larger answer it says both numbers, so the count
// never claims the page shows more than it does.
func resultCount(shown, total int, lang wording.Lang) string {
	if total > shown {
		return fmt.Sprintf(wording.ResultCountShownFmt.In(lang), total, shown)
	}
	return plural(total, wording.ResultCountOne, wording.ResultCountMany, lang)
}
