package pages

import "github.com/a-h/templ"

// nothingNotice is what one surface shows when it has nothing to show: a search
// that matched no note, an address that names none, a folder with no fault left
// to report, a shelf a declaration has not filled, and the two notices the desk
// carries when the reading itself came up short. Each of those moments used to
// be drawn in a shape of its own, so the same kind of news met the reader as a
// different layout every time.
//
// It holds no words. What the surface says arrives as the component's children
// and in Title, already in the reader's language, because what is absent and
// what a reader can do about it is the surface's knowledge rather than this
// one's.
type nothingNotice struct {
	// Kind names the surface, written out as data-nothing so a check can ask
	// which notice it is looking at without matching the prose inside it.
	Kind string
	// Mark is the short word above the heading, where the surface has one. The
	// chrome draws no icon that means "there is nothing here", so what stands
	// in that place is a word rather than a picture.
	Mark string
	// Title is the heading. It is empty where the page's own title already
	// stands above the notice and a second heading would only repeat it.
	Title string
	// TitleID names the heading for whatever points at it: a section's
	// aria-labelledby, an address ending in a fragment.
	TitleID string
	// Page says the heading is the page's own, so it is written as the
	// document's first-level heading rather than as a section's.
	Page bool
	// Fault says the news is a fault rather than an absence — something could
	// not be read, or could not be honoured. An absence stays quiet; a fault
	// takes the warning colour.
	Fault bool
}

// titleAttrs names the heading only where the surface gave it a name, so a
// notice nothing points at does not carry an empty id.
func (n nothingNotice) titleAttrs() templ.Attributes {
	if n.TitleID == "" {
		return nil
	}
	return templ.Attributes{"id": n.TitleID}
}
