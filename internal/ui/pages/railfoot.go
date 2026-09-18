package pages

import (
	"github.com/koopa0/yomihon/internal/wording"
)

// The rail's foot answers in sentences, never in blanks. Both answers are
// worked out here rather than inside the template, because Go written inside a
// template reaches the compiler only as generated output, which every linter in
// this repository is told to skip.

// libraryName is what the foot calls the folder being read. A folder whose path
// yielded no name to take is named as the one in front of the reader, which is
// true and is the only thing left to say about it.
func libraryName(name string, lang wording.Lang) string {
	if name == "" {
		return wording.ThisLibrary.In(lang)
	}
	return name
}

// libraryFindings is what the foot says about the health page's number. Nothing
// found is said in words rather than as a zero: a row of digits ending in 0
// reads as a count that has not been taken yet, and this one has.
func libraryFindings(findings int, lang wording.Lang) string {
	if findings == 0 {
		return wording.LibraryNoFindings.In(lang)
	}
	return plural(findings, wording.LibraryFindingOne, wording.LibraryFindingMany, lang)
}

// libraryHealthState is the dot's state, which the stylesheet colours. It is the
// only place the two states are named, and the words beside the dot carry the
// same answer, so the colour is never the only thing saying it.
func libraryHealthState(findings int) string {
	if findings == 0 {
		return "clear"
	}
	return "findings"
}
