package layouts

import (
	"github.com/koopa0/yomihon/internal/wording"
)

// The three answers the header's own controls need in words: the state a
// checkbox is in, the language the switch leads to, and the measure the
// reading column is set at. They are here rather than beside the markup
// because Go written inside a template reaches the compiler only as
// generated output, which every linter in this repository is told to skip.

func singleKeyShortcutsState(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

// languageMark labels the language control with the language it switches to
// rather than the one in force. A control that names the current state reads as
// a status line, and a reader who cannot read the interface they are looking at
// needs to recognise the way out of it.
func languageMark(lang wording.Lang) string {
	if lang == wording.En {
		return "中"
	}
	return "EN"
}

// textSizeLabel is the text-size control's accessible name, which carries the
// size the reader is at rather than the action the press performs. Its two
// neighbours in the header are two-state and say where they stand through
// aria-pressed; this one cycles through three, which no boolean carries, so
// the name is where its state has to live. The script keeps it current on
// every press, and a reader arriving on a reload finds the same answer.
func textSizeLabel(size string, lang wording.Lang) string {
	switch size {
	case "l":
		return wording.TextSizeLarge.In(lang)
	case "xl":
		return wording.TextSizeExtraLarge.In(lang)
	default:
		return wording.TextSizeMedium.In(lang)
	}
}
