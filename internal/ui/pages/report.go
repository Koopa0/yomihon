package pages

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/wording"
)

// ReportReadingLimitMessage is shared by the shell and the plain-text refusal
// so neither surface rounds away the byte count that exceeded the bound.
func ReportReadingLimitMessage(size, limit int64, lang wording.Lang) string {
	return fmt.Sprintf(wording.ReportOverReadingLimit.In(lang), humanSize(size, lang), readingLimit(limit))
}
