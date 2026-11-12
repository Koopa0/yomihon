// Package figure decides what an interactive HTML figure in a vault may do in
// the reader's browser.
//
// A figure is a single self-contained .html file a note embeds with
// ![[name.html]]. It is shown in a frame of its own, and it reaches nothing:
// no domain is trusted, so nothing is loaded from any of them, and a domain
// that changes hands, is compromised, or redirects somewhere else has nothing
// to act on. Two things decide how far a figure may go, and both are read off
// the same bytes: what the markup asks the browser to do when the figure
// opens, and whether the owner approved those exact bytes to run their inline
// scripts.
//
// Inspect answers the first question. A figure whose markup can make the
// browser fetch, frame, or connect on its own — a nested frame, a link
// element, a base address, a refresh, a ping, an external script — is refused
// and not served at all, because the frame's content policy closes every
// request those make but not, in every browser measured, the connection a
// nested frame opens before the policy is consulted. The parse is a real HTML
// tree build, run the way the browser will run it, rather than a pattern over
// the text: a pattern and a browser disagree about exactly the markup someone
// would write to slip past one.
//
// Approval answers the second, and lives outside this package: the bytes'
// digest is pinned by a person at a terminal, and any change to the file
// lapses it. Without it, a figure's scripts never run.
package figure

import (
	"path"
	"strings"
)

// extension is the one name a figure is recognised by. It is matched in any
// case and never inferred from the bytes: a file is a figure because its
// author named it one.
const extension = ".html"

// Is reports whether relPath names a figure: a file whose final extension is
// .html, in any case.
func Is(relPath string) bool {
	return strings.EqualFold(path.Ext(relPath), extension)
}
