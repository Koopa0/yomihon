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

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// ShellData is the snapshot-derived state shared by the topbar and sidebar.
// A handler receives it as one value so navigation and the pending count cannot
// come from different atomic snapshot reads.
type ShellData struct {
	Nav          *nav.Model
	Pending      int
	PendingKnown bool
}

// Chrome builds the request-cookie state around this snapshot-derived shell.
func (s ShellData) Chrome(r *http.Request, title string) layouts.Chrome {
	return ChromeFromRequest(r, title, s.Pending, s.PendingKnown)
}

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

// rawHref builds the unchanged-bytes URL for a vault-relative path, escaping
// each segment exactly as notesHref does, so a file's page and the bytes it
// points at name the same file whatever its spaces, CJK, or missing extension.
// It is its own route rather than a suffix on the note URL: a vault directory
// named "raw" would otherwise make "/notes/raw/x" ambiguous.
func rawHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/raw/" + strings.Join(segments, "/")
}

// syllabusHref builds the study-path page URL for a vault-relative path,
// percent-escaping each segment exactly as notesHref does — so a study-path
// whose path carries spaces or CJK (e.g. "Maps/Go 課綱.md") round-trips through
// GET /syllabus/{path...} and matches the switcher link byte-for-byte.
func syllabusHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/syllabus/" + strings.Join(segments, "/")
}

// statusHref builds the pure-filter browse URL for a status: Home's Lifecycle
// block links each status to the search results filtered to it. url.Values escapes
// the colon, yielding e.g. /search?q=status%3Adraft.
func statusHref(status string) string {
	return "/search?" + url.Values{"q": {"status:" + status}}.Encode()
}

// reportHref builds the report shell URL for a briefing's filename. Unlike a
// note, a report is addressed by its bare filename (unique within
// System/reports/daily-briefing/), never a vault path: the handler resolves the
// name against the snapshot's report allowlist, so request input never reaches a
// filesystem join. The name is a single path segment, so one PathEscape suffices
// (any "/" in it would escape to %2F and stay one segment).
func reportHref(name string) string {
	return "/reports/" + url.PathEscape(name)
}

// reportRawHref builds the verbatim-serving endpoint the report iframe points
// at: the same name-keyed allowlist lookup as reportHref, plus the /raw suffix
// that writes the briefing HTML byte-for-byte inside the sandboxed frame.
func reportRawHref(name string) string {
	return "/reports/" + url.PathEscape(name) + "/raw"
}

// LifecycleItem is one row of Home's Lifecycle block: a schema status
// (the vocabulary comes from the toml contract), its live snapshot
// count, whether it is the current note's status, and whether it is the
// schema-owned seal target.
type LifecycleItem struct {
	Name   string
	Count  int
	Active bool
	Sealed bool
}

// ChromeFromRequest builds the shell Chrome from the request: the page title
// plus the persisted theme and furigana cookies (default light / on), so the
// root element renders the correct state on the first byte (no FOUC). Only the
// two known cookie values are honored; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
func ChromeFromRequest(r *http.Request, title string, pending int, pendingKnown bool) layouts.Chrome {
	theme := "light"
	if c, err := r.Cookie("yomihon_theme"); err == nil && c.Value == "dark" {
		theme = "dark"
	}
	ruby := "on"
	if c, err := r.Cookie("yomihon_ruby"); err == nil && c.Value == "off" {
		ruby = "off"
	}
	return layouts.Chrome{Title: title, Theme: theme, Ruby: ruby, Pending: pending, PendingKnown: pendingKnown}
}
