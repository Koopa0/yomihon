package pages

import "github.com/a-h/templ"

// FolderNotice is one standing fact about how these pages relate to the folder
// on disk, already worded in the reader's language: a state the pages are in
// that no unreadable file explains. The desk and the health page draw the same
// three parts, so the two cannot tell one fact two ways.
type FolderNotice struct {
	// Title names the state.
	Title string
	// Summary says why, and what to do about it.
	Summary string
	// Detail is the machine's own account of what the notice is about — the
	// files, as they were reported — set beside the summary the way every other
	// diagnostic is. It is empty where there is nothing to name.
	Detail string
}

// healthNotices draws every notice in the order it was given, as one component.
// The health page joins it to the call that follows it rather than making a
// call of its own, so that a folder with no notice draws the bytes it drew
// before there were any: templ writes a space between two calls on separate
// lines, and one more call is one more space in every recorded page.
func healthNotices(notices []FolderNotice) templ.Component {
	drawn := make([]templ.Component, len(notices))
	for i, notice := range notices {
		drawn[i] = healthNotice(notice)
	}
	return templ.Join(drawn...)
}
