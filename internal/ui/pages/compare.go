package pages

import (
	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// compareReadAloudAttrs carries the read-aloud bar's words to a page that will
// grow one. The page grows at most one bar and both columns reach the same
// speech owner, so the first column with anything to read aloud supplies the
// words and the other needs none of its own.
func compareReadAloudAttrs(v *CompareView, lang wording.Lang) templ.Attributes {
	if attrs := readAloudAttrs(v.A.BodyHTML, lang); attrs != nil {
		return attrs
	}
	return readAloudAttrs(v.B.BodyHTML, lang)
}
