package pages

import (
	"strconv"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// questionAttrs gives the enhancement its endpoint and translated words.
// A page with no place to keep a question, or whose reader folder cannot be
// written to (Writable is then why), offers no control: the marks pages say why.
func questionAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if v.QuestionAddress == "" || v.RelPath == "" || v.Writable != "" {
		return nil
	}
	return templ.Attributes{
		"data-question-endpoint":    v.QuestionAddress,
		"data-question-path":        v.RelPath,
		"data-question-prefix":      v.IDPrefix,
		"data-question-label":       wording.QuestionControl.In(lang),
		"data-question-saved":       wording.QuestionSaved.In(lang),
		"data-question-cleared":     wording.QuestionCleared.In(lang),
		"data-question-failed":      wording.QuestionNotStored.In(lang),
		"data-question-unavailable": wording.QuestionUnavailable.In(lang),
		"data-question-lang":        lang.Tag(),
	}
}

// SlotPatternID is the id of the slot-machine card at a zero-based position.
// It comes from position, not from the pattern's authored id, which nothing
// checks for being present or distinct.
func SlotPatternID(index int) string {
	return "slot-pattern-" + strconv.Itoa(index+1)
}
