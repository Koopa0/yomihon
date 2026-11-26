package pages

import (
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// ContinueRow is the desk's one way back to where the reader left off.
//
// It carries sentences rather than the mark itself: which of them applies is
// decided where the mark and the generation behind the page are both in hand,
// so the row and the rest of the desk describe one reading of the vault.
type ContinueRow struct {
	// Show is whether the reader has left a place at all. A desk with no mark
	// draws no row and says nothing about marks: an empty row would be an
	// advertisement for a control that lives on another page.
	Show bool
	// Title is the note to return to, or its path where the note is gone and
	// there is no title left to read. It is the largest text on the desk.
	Title string
	// Language is the tag the note declared, so the title is read in the
	// language its author wrote it in.
	Language string
	// Kicker says where the note sits: the course and the part of it, or the
	// folder for a note no course walks. Empty where the note is gone.
	Kicker         string
	KickerLanguage string
	// Section is the heading the mark was kept under, empty when the mark names
	// the note as a whole or the heading is gone.
	Section RowPlace
	// Age is how long ago the bookmark was added, already in words, and AgeAt
	// the moment itself in RFC 3339. Both are empty for a bookmark that carries
	// no time.
	Age   string
	AgeAt string
	// Href is where the row leads: the note, the anchor the mark named, and
	// the distance below it. It is empty where the note is no longer in the
	// vault, and the row is then a sentence rather than a link.
	Href string
	// Notice is what the row says beyond the title — that the note has changed
	// since the mark was set, or that it is gone. Empty where neither is true,
	// which is the ordinary case and renders nothing.
	Notice string
}

// around is the words a phrase sets either side of the one value it takes, so
// a page can set that value apart — in its author's language, or in a face of
// its own — and still say the sentence the dictionary wrote.
func around(phrase wording.Phrase, lang wording.Lang) (before, after string) {
	before, after, _ = strings.Cut(phrase.In(lang), "%s")
	return before, after
}
