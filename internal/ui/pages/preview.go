package pages

// PreviewView is one hover card's contents: an excerpt of the note under the
// reader's pointer, cut at the section or block that link addressed. It is
// fetched and shown without leaving the page the reader is on, so the only
// chrome it carries is a source line naming the note — and the section within
// it, where the address named one — above whatever else the card holds.
//
// The three fields are independent, and each combination the card can be in is
// a real one: an excerpt alone is the ordinary case; an excerpt with a notice
// is one that stops short of the end; a notice with a note but no excerpt is an
// address naming a place that note does not have; and a notice with neither is
// an address with no note behind it at all. The way on to the note is that
// source line, offered whenever there is a note at all rather than only when
// something was withheld: a reader who wants the whole of what they are looking
// at has the same reason to leave the card as one who was shown part of it.
type PreviewView struct {
	// RelPath is the note the card is about, empty when the address named none.
	RelPath string
	// Language is that note's own declared language, which need not be the
	// language of the page showing the card.
	Language string
	// Title is that note's own name, which the card shows above the excerpt: a
	// link written at an alias, or at a section, shows the reader words that
	// are not the note's, and adjacency to the link is the only thing that
	// says which note they are reading.
	Title string
	// Section is the name of the place inside the note the excerpt was cut at,
	// empty for a whole note or a block.
	Section string
	// BodyHTML is the rendered excerpt, empty when there is none to show.
	BodyHTML string
	// Notice is the card's own sentence, already in the reader's language.
	Notice string
}
