package pages

import (
	"github.com/a-h/templ"
	"github.com/koopa0/yomihon/internal/wording"
)

// uncertaintyAttrs gives the enhancement its endpoint and translated words.
// A page with no local storage offers no control.
func uncertaintyAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if v.UncertaintyAddress == "" || v.RelPath == "" {
		return nil
	}
	return templ.Attributes{
		"data-uncertainty-endpoint":    v.UncertaintyAddress,
		"data-uncertainty-path":        v.RelPath,
		"data-uncertainty-prefix":      v.IDPrefix,
		"data-uncertainty-label":       wording.UncertaintyControl.In(lang),
		"data-uncertainty-saved":       wording.UncertaintySaved.In(lang),
		"data-uncertainty-cleared":     wording.UncertaintyCleared.In(lang),
		"data-uncertainty-failed":      wording.UncertaintyNotStored.In(lang),
		"data-uncertainty-unavailable": wording.UncertaintyUnavailable.In(lang),
		"data-uncertainty-scope":       wording.UncertaintyPlaceOnly.In(lang),
		"data-uncertainty-lang":        lang.Tag(),
	}
}
