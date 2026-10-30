package pages

import (
	"github.com/koopa0/yomihon/internal/wording"
)

// The rail's foot answers in sentences, never in blanks. Its answer is worked
// out here rather than inside the template, because Go written inside a
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
