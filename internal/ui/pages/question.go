package pages

import (
	"strconv"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// questionAttrs gives the enhancement its endpoint and translated words.
// A page with no local storage offers no control.
func questionAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if v.QuestionAddress == "" || v.RelPath == "" {
		return nil
	}
	return templ.Attributes{
		"data-uncertainty-endpoint":    v.QuestionAddress,
		"data-uncertainty-path":        v.RelPath,
		"data-uncertainty-prefix":      v.IDPrefix,
		"data-uncertainty-label":       wording.QuestionControl.In(lang),
		"data-uncertainty-saved":       wording.QuestionSaved.In(lang),
		"data-uncertainty-cleared":     wording.QuestionCleared.In(lang),
		"data-uncertainty-failed":      wording.QuestionNotStored.In(lang),
		"data-uncertainty-unavailable": wording.QuestionUnavailable.In(lang),
		"data-uncertainty-scope":       wording.QuestionPlaceOnly.In(lang),
		"data-uncertainty-lang":        lang.Tag(),
	}
}

// SlotPatternID is the id of the slot-machine card at a zero-based position.
// It comes from position, not from the pattern's authored id, which nothing
// checks for being present or distinct.
func SlotPatternID(index int) string {
	return "slot-pattern-" + strconv.Itoa(index+1)
}
