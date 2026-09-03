package pages

// PreviewView is one hover card's contents: an excerpt of the note under the
// reader's pointer, cut at the section or block that link addressed. It is
// fetched and shown without leaving the page the reader is on, so it carries no
// chrome of its own beyond the two sentences below.
//
// RelPath empty is the one case the card speaks in the interface's own voice
// rather than showing another note's words: the address named no note this
// generation holds. Everything else is an excerpt, and Truncated says whether
// it reaches the end of what that address named.
type PreviewView struct {
	RelPath   string
	Language  string
	BodyHTML  string
	Truncated bool
}
