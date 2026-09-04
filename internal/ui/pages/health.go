package pages

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/wording"
)

// clean reports whether the folder has nothing to answer for.
func (v *HealthView) clean() bool {
	return len(v.Unwritten) == 0 && len(v.TitleOnly) == 0 && v.IslandCount == 0 &&
		len(v.Collisions) == 0 && len(v.Blocked) == 0 && len(v.Skipped) == 0 &&
		len(v.StatusOutsideEnum) == 0 &&
		len(v.FrontmatterUnreadable) == 0 && len(v.SchemaFaults) == 0 &&
		v.InstanceScopeUnknown == "" && v.SchemaScopeUnknown == ""
}

// blockedLede states what the blocked list means for the reader, and how
// current the page behind it is.
func (v *HealthView) blockedLede(lang wording.Lang) string {
	lede := wording.BlockedLede.In(lang)
	if v.LastComplete == "" {
		return lede + wording.BlockedNeverComplete.In(lang)
	}
	return lede + fmt.Sprintf(wording.BlockedLastCompleteFmt.In(lang), v.LastComplete)
}
