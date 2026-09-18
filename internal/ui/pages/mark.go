package pages

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
	// there is no title left to read.
	Title string
	// Href is where the row leads: the note, the anchor the mark named, and
	// the distance below it. It is empty where the note is no longer in the
	// vault, and the row is then a sentence rather than a link.
	Href string
	// Notice is what the row says beyond the title — that the note has changed
	// since the mark was set, or that it is gone. Empty where neither is true,
	// which is the ordinary case and renders nothing.
	Notice string
}
