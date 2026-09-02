package pages

import (
	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// Title is the page's one title, used for both the document title and the
// heading so a reader switching between the tab and the page reads the same
// sentence.
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

// StatusRecovery renders the write face's recovery state in the ordinary
// reading shell. It intentionally exposes only GET links: a failed POST must
// never be repeated by a recovery control.
// recoveryFreshnessAttrs marks the recovery column as one the client may watch.
// Both facts have to be there: which note to ask about, and the version the
// refused write bound itself to. Without either, the page keeps the plain
// invitation it carried before there was anything to hold it for.
//
// The hold sentences travel with the watch, under the same names and from the
// same dictionary entries the reading page's column carries them: the words
// are the server's, and the client shows a held link exactly what it reads
// here. A column that named the note and the version but carried no words
// would leave that link saying nothing a reader can use.
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
