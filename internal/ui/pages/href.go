// Package pages holds one templ component per served page (and the small
// hand-written helpers those components call). The package doc lives in
// this hand-written file rather than in a .templ: the generated *_templ.go
// carries a "Code generated" header that the linters skip, so a
// non-generated file must own the package comment.
package pages

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// Shell is the snapshot-derived state shared by the topbar and sidebar.
// A handler receives it as one value so navigation and the governed flag
// cannot come from different atomic snapshot reads.
//
// Governed says whether anything claimed authority over this vault. It gates
// every surface that would otherwise present a lifecycle vocabulary — a status
// chip, the write face — because naming a status the vault never declared
// invents a vocabulary rather than reporting one.
type Shell struct {
	Nav      *nav.Model
	Governed bool
}

// WithoutInstanceProjections returns a shell whose navigation and topbar carry
// no instance-derived state. Direct file and folder navigation remain in the
// model; the supplied claim records why instance projections closed and, when
// it carries one, the sentence to show.
func (s Shell) WithoutInstanceProjections(claim schema.Claim) Shell {
	s.Nav = s.Nav.WithoutInstanceProjections(nav.Close(claim))
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

// ObsidianHref builds the obsidian://open URI for the note at rel inside the
// vault rooted at root. The absolute path rides as the URI's single query
// parameter with every segment percent-escaped on its own and "&" escaped
// besides, so a name carrying "?", "#", "%", or "&" cannot cut the parameter
// short or start a second one. Spaces become %20, never "+": Obsidian reads
// a literal "+" as itself. Containment needs no second check here — every
// caller passes a rel the serving route has already bounded to the vault,
// and root is the reader's own resolved vault path. Either argument empty
// yields "", and the caller then renders no link.
func ObsidianHref(root, rel string) string {
	if root == "" || rel == "" {
		return ""
	}
	segments := strings.Split(filepath.ToSlash(root)+"/"+rel, "/")
	for i, s := range segments {
		segments[i] = strings.ReplaceAll(url.PathEscape(s), "&", "%26")
	}
	return "obsidian://open?path=" + strings.Join(segments, "/")
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

// plural picks the phrase that agrees with n. Chinese does not inflect a noun
// for number, so both sides of such a pair carry the same words there; English
// does, and a tally that reads "1 notes" is the first thing a reader notices
// about a page.
func plural(n int, one, many wording.Phrase, lang wording.Lang) string {
	if n == 1 {
		return fmt.Sprintf(one.In(lang), n)
	}
	return fmt.Sprintf(many.In(lang), n)
}

// statusChipLabel names one square of the lifecycle block. A note whose
// frontmatter carries no status still has somewhere to go, so it is counted
// and listed like any other; saying so in words leaves the reader with what
// the file is missing rather than a link with nothing written on it.
func statusChipLabel(status string, lang wording.Lang) string {
	return cmp.Or(status, wording.NoStatusStated.In(lang))
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
// count, and whether it is the ready accent's schema-owned status.
type LifecycleItem struct {
	Name   string
	Count  int
	Sealed bool
	// Unknown is set when no type carrying this status declares it: the
	// value is outside every relevant enum, so the chip carries the same
	// amber flag the note page shows, and stays a link — flagged, not
	// hidden.
	Unknown bool
	// Href is where the chip leads. Every chip standing for a status leads to
	// that status's own query; the two that stand for notes carrying no
	// readable status name no status to query, so one leads to the page
	// listing them and the other leads nowhere and is not a link at all. A
	// chip that looked like a link to a query no note answers would be an
	// offer the folder cannot keep.
	Href string
	// Label is what the chip reads when it stands for something other than a
	// status the folder declares, so the status name and the words shown do
	// not have to be the same string.
	Label string
}

// LanguageFromRequest reads which language this request asked the interface to
// speak. It sits beside the other preference reads rather than in the
// dictionary, so the dictionary stays a set of sentences with no capability of
// its own and no import at all.
func LanguageFromRequest(r *http.Request) wording.Lang {
	c, err := r.Cookie(wording.CookieName)
	if err != nil {
		return wording.ZhHant
	}
	return wording.FromCookieValue(c.Value)
}

// ChromeFromRequest builds the page chrome from the request: the page title
// plus the persisted theme, furigana, and single-key-shortcut cookies, so the
// root element renders the correct state on the first byte (no FOUC). Each
// cookie honors only its known values; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
//
// The theme's default is deliberately empty rather than light: a reader who
// never chose has expressed no preference here, and the stylesheet answers an
// unstamped root with the system's own preference. Both stored values are
// honored, because an explicit light choice must keep beating a dark system.
//
// It takes no shell: what the chrome is built from is the request and nothing
// else, and a snapshot projection passed alongside would say the two were
// related when they never were.
func ChromeFromRequest(r *http.Request, title string) layouts.Chrome {
	theme := ""
	if c, err := r.Cookie("yomihon_theme"); err == nil && (c.Value == "dark" || c.Value == "light") {
		theme = c.Value
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
		Lang:                      LanguageFromRequest(r),
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     theme,
		Ruby:                      ruby,
		TextSize:                  textSize,
		SingleKeyShortcutsEnabled: singleKeyShortcutsEnabled,
		// The request's own address, so the language form can bring the reader
		// back to this page after the switch.
		ReturnTo: r.URL.RequestURI(),
	}
}
