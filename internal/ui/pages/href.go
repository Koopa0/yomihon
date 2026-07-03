// Package pages holds one templ component per served page (and the small
// hand-written helpers those components call). The package doc lives in
// this hand-written file rather than in a .templ: the generated *_templ.go
// carries a "Code generated" header that the linters skip, so a
// non-generated file must own the package comment.
package pages

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/koopa0/kurodo/internal/ui/layouts"
)

// notesHref builds the reading-page URL for a vault-relative path,
// percent-escaping each segment individually (so a literal "/" in an
// escaped segment can never be read as a separator) while keeping "/" as
// the separator between segments — exactly how GET /notes/{path...} expects
// to receive it back, and byte-identical to the href the rendered wikilinks
// use, so a sidebar link and an in-body link to the same note match.
//
// It mirrors internal/render's own unexported notesHref rather than sharing
// one helper: exporting render's copy (or routing this through render)
// would couple the UI layer to the renderer for a five-line URL builder,
// and hoisting a shared "how to link to a note" helper is a change to
// render's public surface out of scope for the navigation work.
func notesHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/notes/" + strings.Join(segments, "/")
}

// statusHref builds the pure-filter browse URL for a status: the Lifecycle rail
// links each status to the search results filtered to it. url.Values escapes
// the colon, yielding e.g. /search?q=status%3Adraft.
func statusHref(status string) string {
	return "/search?" + url.Values{"q": {"status:" + status}}.Encode()
}

// LifecycleItem is one row of the status-first Lifecycle rail: a schema status
// (the vocabulary comes from the toml contract, wall 3), its live snapshot
// count, and whether it is the current note's status.
type LifecycleItem struct {
	Name   string
	Count  int
	Active bool
}

// ChromeFromRequest builds the shell Chrome from the request: the page title
// plus the persisted theme and furigana cookies (default light / on), so the
// root element renders the correct state on the first byte (no FOUC). Only the
// two known cookie values are honored; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
func ChromeFromRequest(r *http.Request, title string) layouts.Chrome {
	theme := "light"
	if c, err := r.Cookie("kurodo_theme"); err == nil && c.Value == "dark" {
		theme = "dark"
	}
	ruby := "on"
	if c, err := r.Cookie("kurodo_ruby"); err == nil && c.Value == "off" {
		ruby = "off"
	}
	return layouts.Chrome{Title: title, Theme: theme, Ruby: ruby}
}
