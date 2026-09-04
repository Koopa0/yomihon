// Package pages holds one templ component per served page, and the small
// hand-written helpers those components call. The doc lives in a hand-written
// file because a generated one carries a header the linters skip.
package pages

import (
	"cmp"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// VaultHref builds a URL for a vault-relative path under prefix, escaping each
// segment on its own so a literal "/" inside one is never read as a separator.
// It is byte-identical to the href a rendered wikilink carries, so a rail link
// and an in-body link to the same note match.
//
// It is exported because a face outside these templates sends a reader to one of
// these addresses too, and the escaping is the half that is easy to get subtly
// wrong twice: a segment escaped whole rather than one at a time turns a name
// carrying a slash into a path.
func VaultHref(prefix, p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return prefix + strings.Join(segments, "/")
}

// notesHref builds the reading-page URL for a vault-relative path.
func notesHref(p string) string { return VaultHref("/notes/", p) }

// rawHref builds the unchanged-bytes URL for a vault-relative path. It is its
// own route rather than a suffix on the note URL, which a vault directory named
// "raw" would make ambiguous.
func rawHref(p string) string { return VaultHref("/raw/", p) }

// ObsidianHref builds the obsidian://open URI for the note at rel inside the
// vault rooted at root. Every segment is escaped on its own, and "&" besides,
// so a name carrying "?", "#", "%" or "&" cannot cut the single query parameter
// short or start a second one; spaces become %20, never "+", which Obsidian
// reads as itself. Containment is the calling route's, already bounded. Either
// argument empty yields "", and the caller then renders no link.
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

// syllabusHref builds the study-path page URL for a vault-relative path.
func syllabusHref(p string) string { return VaultHref("/syllabus/", p) }

// statusHref builds the search URL filtered to one status, with url.Values
// escaping the colon: /search?q=status%3Adraft.
func statusHref(status string) string {
	return "/search?" + url.Values{"q": {"status:" + status}}.Encode()
}

// plural picks the phrase that agrees with n. Chinese does not inflect a noun
// for number, so both sides of such a pair carry the same words there.
func plural(n int, one, many wording.Phrase, lang wording.Lang) string {
	if n == 1 {
		return fmt.Sprintf(one.In(lang), n)
	}
	return fmt.Sprintf(many.In(lang), n)
}

// statusChipLabel names one square of the lifecycle block, saying in words that
// a note carries no status rather than leaving the square blank.
func statusChipLabel(status string, lang wording.Lang) string {
	return cmp.Or(status, wording.NoStatusStated.In(lang))
}

// folderHref builds the browse URL for a folder.
func folderHref(dir string) string { return VaultHref("/folders/", dir) }

// searchHref builds the URL for one search query, escaping it as the form
// submission would, so an offered search and a typed one land on the same page.
func searchHref(q string) string {
	return "/search?" + url.Values{"q": {q}}.Encode()
}

// reportHref builds the report shell URL for a briefing's bare filename, never a
// vault path: the handler resolves the name against the snapshot's report
// allowlist, so request input never reaches a filesystem join.
func reportHref(name string) string {
	return "/reports/" + url.PathEscape(name)
}

// reportRawHref builds the endpoint the report iframe points at: reportHref's
// allowlist lookup plus the suffix that writes the briefing HTML unchanged.
func reportRawHref(name string) string {
	return "/reports/" + url.PathEscape(name) + "/raw"
}

// LifecycleItem is one row of the folder index's status distribution: a status
// the contract declares, its live snapshot count, and whether it is the ready
// accent's.
type LifecycleItem struct {
	Name   string
	Count  int
	Sealed bool
	// Unknown is set where no type carrying this status declares it, so the
	// chip carries the note page's amber flag and stays a link.
	Unknown bool
	// Href is where the chip leads, empty for a chip standing for notes with
	// no status to query — a link to a query no note answers is an offer the
	// folder cannot keep.
	Href string
	// Label is what the chip reads when it stands for something other than a
	// declared status.
	Label string
}
