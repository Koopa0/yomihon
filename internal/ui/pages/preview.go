package pages

import "github.com/a-h/templ"

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

// previewEndpointAttrs marks the reading column as one whose links may be
// previewed, and says where to ask. The address travels with the page rather
// than living in the script, for the same reason the freshness watch's does:
// the routes are the server's to name. It is withheld from a page whose prose
// carries no link a card could be opened on, so the module finds nothing to
// bind and the page below is exactly what it was.
func previewEndpointAttrs(v *NoteView) templ.Attributes {
	if !v.hasPreviewableLinks() {
		return nil
	}
	return templ.Attributes{"data-preview-endpoint": "/preview/"}
}
