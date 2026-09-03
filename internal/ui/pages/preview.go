package pages

// PreviewView is one hover card's contents: an excerpt of the note under the
// reader's pointer, cut at the section or block that link addressed. It is
// fetched and shown without leaving the page the reader is on, so it carries no
// chrome of its own beyond the one sentence below.
//
// The three fields are independent, and each combination the card can be in is
// a real one: an excerpt alone is the ordinary case; an excerpt with a notice
// is one that stops short of the end; a notice with a note but no excerpt is an
// address naming a place that note does not have; and a notice with neither is
// an address with no note behind it at all. The way on to the note is offered
// whenever there is a note and something was withheld, which is exactly when a
// reader has a reason to leave the card.
type PreviewView struct {
	// RelPath is the note the card is about, empty when the address named none.
	RelPath string
	// Language is that note's own declared language, which need not be the
	// language of the page showing the card.
	Language string
	// BodyHTML is the rendered excerpt, empty when there is none to show.
	BodyHTML string
	// Notice is the card's own sentence, already in the reader's language.
	Notice string
}
