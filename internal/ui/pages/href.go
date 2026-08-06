// Package pages holds one templ component per served page (and the small
// hand-written helpers those components call). The package doc lives in
// this hand-written file rather than in a .templ: the generated *_templ.go
// carries a "Code generated" header that the linters skip, so a
// non-generated file must own the package comment.
package pages

import (
	"cmp"
	"net/http"
	"net/url"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// Shell is the snapshot-derived state shared by the topbar and sidebar.
// A handler receives it as one value so navigation and the advanceable count
// cannot come from different atomic snapshot reads.
//
// Governed says whether anything claimed authority over this vault. It gates
// every surface that would otherwise present a lifecycle vocabulary — a status
// chip, an advanceable count, the write face — because naming a status the
// vault never declared invents a vocabulary rather than reporting one.
type Shell struct {
	Nav              *nav.Model
	Governed         bool
	Advanceable      int
	AdvanceableKnown bool
}

// Chrome builds the request-cookie state around this snapshot-derived shell.
func (s Shell) Chrome(r *http.Request, title string) layouts.Chrome {
	return chromeFromRequest(r, title, s.Advanceable, s.AdvanceableKnown)
}

// WithoutInstanceProjections returns a shell whose navigation and topbar carry
// no instance-derived state. Direct file and folder navigation remain in the
// model; the supplied claim records why instance projections closed and, when
// it carries one, the sentence to show.
func (s Shell) WithoutInstanceProjections(claim schema.Claim) Shell {
	s.Nav = s.Nav.WithoutInstanceProjections(nav.Close(claim))
	s.Advanceable = 0
	s.AdvanceableKnown = false
	return s
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

// statusChipLabel names one square of the lifecycle block. A note whose
// frontmatter carries no status still has somewhere to go, so it is counted
// and listed like any other; saying so in words leaves the reader with what
// the file is missing rather than a link with nothing written on it.
func statusChipLabel(status string) string {
	return cmp.Or(status, "未標示狀態")
}

// folderHref builds the browse URL for a folder, escaping each segment the
// way notesHref does so a folder and a note under it agree byte for byte about
// how their shared path is written.
func folderHref(dir string) string {
	segments := strings.Split(dir, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return "/folders/" + strings.Join(segments, "/")
}

// searchHref builds the URL for one search query, escaping it the same way
// the form submission would, so an offered search and a typed one land on the
// same page.
func searchHref(q string) string {
	return "/search?" + url.Values{"q": {q}}.Encode()
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

// chromeFromRequest builds the shell Chrome from the request: the page title
// plus the persisted theme, furigana, and single-key-shortcut cookies (default
// light / on / on), so the root element renders the correct state on the first
// byte (no FOUC). Each
// cookie honors only its known values; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
func chromeFromRequest(r *http.Request, title string, advanceable int, advanceableKnown bool) layouts.Chrome {
	theme := "light"
	if c, err := r.Cookie("yomihon_theme"); err == nil && c.Value == "dark" {
		theme = "dark"
	}
	ruby := "on"
	if c, err := r.Cookie("yomihon_ruby"); err == nil && c.Value == "off" {
		ruby = "off"
	}
	textSize := "m"
	if c, err := r.Cookie("yomihon_textsize"); err == nil && (c.Value == "l" || c.Value == "xl") {
		textSize = c.Value
	}
	singleKeyShortcutsEnabled := true
	if c, err := r.Cookie("yomihon_shortcuts"); err == nil && c.Value == "off" {
		singleKeyShortcutsEnabled = false
	}
	return layouts.Chrome{
		Title:                     title,
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     theme,
		Ruby:                      ruby,
		TextSize:                  textSize,
		SingleKeyShortcutsEnabled: singleKeyShortcutsEnabled,
		Advanceable:               advanceable,
		AdvanceableKnown:          advanceableKnown,
	}
}
