package pages

import (
	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// Title is the page's one title, used for both the document title and the
// heading.
func (v *StatusRecoveryView) Title(lang wording.Lang) string {
	if v.Changed {
		return wording.RecoveryChangedTitle.In(lang)
	}
	return wording.RecoveryUnchangedTitle.In(lang)
}
func (v *StatusRecoveryView) mutationState(lang wording.Lang) string {
	if v.Changed {
		return wording.RecoveryChangedState.In(lang)
	}
	return wording.RecoveryUnchangedState.In(lang)
}

// recoveryFreshnessAttrs marks the recovery column as one the client may watch.
// Both the note to ask about and the version the refused write bound itself to
// have to be there; without either the column carries no watch. The hold
// sentences travel with it, from the dictionary rather than from the script.
func recoveryFreshnessAttrs(v *StatusRecoveryView, lang wording.Lang) templ.Attributes {
	if v.NotePath == "" || v.NoteIdentity == "" {
		return nil
	}
	return templ.Attributes{
		"data-freshness-path":       v.NotePath,
		"data-freshness-identity":   v.NoteIdentity,
		"data-freshness-holdtitle":  wording.FreshnessHoldPreparingTitle.In(lang),
		"data-freshness-holddetail": wording.FreshnessHoldPreparingDetail.In(lang),
		"data-freshness-gonetitle":  wording.FreshnessHoldGoneTitle.In(lang),
		"data-freshness-gone":       wording.FreshnessGone.In(lang),
	}
}
