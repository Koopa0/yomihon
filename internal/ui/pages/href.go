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
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// Shell is the snapshot-derived state shared by the topbar and sidebar, and
// it belongs to the navigation model it is made of rather than to this
// package: a component receives one, and nothing here builds or decides
// anything about it. The name stays reachable as pages.Shell while the call
// sites still spell it that way.
type Shell = nav.Shell

// vaultHref builds a URL for a vault-relative path under prefix,
// percent-escaping each segment individually — so a literal "/" inside an
// escaped segment can never be read as a separator — while keeping "/" as the
// separator between them. That is exactly how each {path...} route expects to
// receive a path back, and it is byte-identical to the href the rendered
// wikilinks use, so a rail link and an in-body link to the same note match.
//
// It mirrors the renderer's own unexported builder rather than sharing one:
// exporting that copy would widen the renderer's surface for the sake of a
// three-line loop, and the loop is written once here instead of at each route.
func vaultHref(prefix, p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return prefix + strings.Join(segments, "/")
}

// notesHref builds the reading-page URL for a vault-relative path.
func notesHref(p string) string { return vaultHref("/notes/", p) }

// rawHref builds the unchanged-bytes URL for a vault-relative path, so a file's
// page and the bytes it points at name the same file whatever its spaces, CJK,
// or missing extension. It is its own route rather than a suffix on the note
// URL: a vault directory named "raw" would otherwise make "/notes/raw/x"
// ambiguous.
func rawHref(p string) string { return vaultHref("/raw/", p) }

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

// syllabusHref builds the study-path page URL for a vault-relative path, so a
// study path whose path carries spaces or CJK (for example "Maps/Go 課綱.md")
// round-trips through its route and matches the switcher link byte for byte.
func syllabusHref(p string) string { return vaultHref("/syllabus/", p) }

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

// folderHref builds the browse URL for a folder, so a folder and a note under
// it agree byte for byte about how their shared path is written.
func folderHref(dir string) string { return vaultHref("/folders/", dir) }

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

// LanguageFromRequest and ChromeFromRequest read one request's presentation
// state. Both live with the type they fill, beside the document they are
// stamped onto; these two names stay reachable here while the call sites still
// spell them this way.
func LanguageFromRequest(r *http.Request) wording.Lang { return layouts.LanguageFromRequest(r) }

func ChromeFromRequest(r *http.Request, title string) layouts.Chrome {
	return layouts.ChromeFromRequest(r, title)
}
