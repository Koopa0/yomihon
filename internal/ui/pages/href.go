// Package pages holds one templ component per served page, and the small
// hand-written helpers those components call. The doc lives in a hand-written
// file because a generated one carries a header the linters skip.
package pages

import (
	"cmp"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
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

// notesHref builds the reading-page URL for a vault-relative path. A
// daily-briefing HTML opens at the report surface rather than as a /notes/
// source dump, so a folder row, a rail twin and a search hit land where the
// shelf already sends the reader.
func notesHref(p string) string {
	if name, ok := nav.BriefingName(p); ok {
		return reportHref(name)
	}
	return VaultHref("/notes/", p)
}

// rawHref builds the unchanged-bytes URL for a vault-relative path. It is its
// own route rather than a suffix on the note URL, which a vault directory named
// "raw" would make ambiguous.
func rawHref(p string) string { return VaultHref("/raw/", p) }

// ResumeHref is where a reader goes back to the place they kept: the note, the
// anchor the mark named, and how far below it they were.
//
// The anchor is the fragment, so a browser running nothing lands on the heading
// or block the reader stopped under — which is the whole of the promise a kept
// place can make without a script. The distance rides as a query the reading
// page's own module spends and then removes from the address; a fragment
// carrying it would name no element and drop the reader at the top.
//
// It takes the three values rather than the record holding them because two
// surfaces send a reader back — the desk's row and a course's own verb — and
// the address they send them to has to be one address. Neither of them has any
// use for the rest of a kept place.
func ResumeHref(relPath, anchor string, offset int) string {
	address := VaultHref("/notes/", relPath)
	if offset > 0 {
		address += "?" + url.Values{resumeOffsetParam: {strconv.Itoa(offset)}}.Encode()
	}
	if anchor != "" {
		address += "#" + url.PathEscape(anchor)
	}
	return address
}

// resumeOffsetParam carries the distance below the anchor. The reading page
// that spends it is drawn by this package too, so the name is written once and
// stamped into every address that carries one.
const resumeOffsetParam = "at"

// hitFragment encodes the destination selected by the search layer. A crossing
// match names one term in each end block; a normal hit names the first marked
// excerpt span, whose source context and word edges are known by the index.
func hitFragment(r *SearchResult) string {
	if r.BlockCrossing {
		prefix, start := landingTerm(r)
		end := strings.TrimSpace(r.LandingEnd)
		switch {
		case start != "" && end != "":
			return textDirective(prefix, start, end)
		case start != "":
			return textDirective(prefix, start, "")
		case end != "":
			// Those words sit in the first block and this stretch in the
			// last, and a prefix is only read as one beside a term from the
			// same block, so this end travels alone.
			return textDirective("", end, "")
		default:
			return ""
		}
	}
	prefix, start := landingTerm(r)
	if start == "" {
		return ""
	}
	return textDirective(prefix, start, "")
}

// landingTerm is the opening of a directive that names the landing match: the
// run of words it follows, and the stretch to arrive at. The two are chosen
// together because the browser asks less of a stretch a run introduces — it
// takes that one wherever the run leaves off, and the other only where a word
// begins — so the stretch that can stand alone is wanted exactly where there
// is no run to put in front of it. Both empty leaves the row with no term of
// its own, which for a crossing match is what hands the far end the directive.
func landingTerm(r *SearchResult) (prefix, start string) {
	if prefix = strings.TrimSpace(r.LandingPrefix); prefix != "" {
		return prefix, strings.TrimSpace(r.Landing)
	}
	return "", strings.TrimSpace(r.LandingBare)
}

// textDirective assembles the fragment for one hit. The "-" that marks the
// leading term as words to search ahead of the match is written after the
// escaping rather than through it: escapeTextDirective spends that character
// on the reader's own words, so a marker passed through it would come out
// encoded and be read as part of the term.
func textDirective(prefix, start, end string) string {
	var b strings.Builder
	b.WriteString("#:~:text=")
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		b.WriteString(escapeTextDirective(prefix))
		b.WriteString("-,")
	}
	b.WriteString(escapeTextDirective(start))
	if end != "" {
		b.WriteString(",")
		b.WriteString(escapeTextDirective(end))
	}
	return b.String()
}

// escapeTextDirective percent-encodes one term of a text directive. Everything
// outside the unreserved set is encoded, and "-" joins it rather than being
// left alone: the grammar spends that character on the marks that introduce a
// prefix and a suffix, so a hyphen among the reader's own words would be read
// as one of those and the rest of the term thrown away.
func escapeTextDirective(term string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(term))
	for i := range len(term) {
		switch c := term[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '_', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

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

// CompareWithParam names the second of the two notes a side-by-side address
// holds. The link is written here and read by the route that answers it, so the
// two ends share one spelling rather than agreeing by hand.
const CompareWithParam = "with"

// compareHref builds the address that opens two notes at once. The first note
// is a path segment and the second a query value, so each is escaped by the
// rule its own half of a URL follows: the segment through the one escaper every
// vault address already uses, the value through the same encoder a search link
// spends on a reader's query.
func compareHref(a, b string) string {
	return VaultHref("/compare/", a) + "?" + url.Values{CompareWithParam: {b}}.Encode()
}

// syllabusHref builds the study-path page URL for a vault-relative path.
func syllabusHref(p string) string { return VaultHref("/syllabus/", p) }

// SyllabusFromParam names the note a reader opened a course from. The link is
// written here and read by the route that answers it, so the two ends share one
// spelling rather than agreeing by hand.
const SyllabusFromParam = "from"

// syllabusHrefFrom is the way into a course from a note being read, naming that
// note so the course can mark where in it the reader is standing. "from" is
// already this interface's word for the value a reader arrived with, and the
// note is named by its vault path rather than by its reading address: the page
// compares it against the vault paths the course lists, and two spellings of
// one address would have to be reconciled before they could be compared. An
// empty note leaves the plain address, which marks nothing.
//
// Nothing follows the value: it is compared with what the course already holds
// and is never rendered as a link or an address, so a value naming anything
// else falls through to marking no row at all.
func syllabusHrefFrom(pathRel, noteRel string) string {
	if noteRel == "" {
		return syllabusHref(pathRel)
	}
	return syllabusHref(pathRel) + "?" + url.Values{SyllabusFromParam: {noteRel}}.Encode()
}

// statusHref builds the search URL filtered to one status, with url.Values
// escaping the colon: /search?q=status%3Adraft. The key comes from the package
// that owns the filter grammar, so a link this page draws cannot outlive the
// filter it names. The ":" between key and value is that package's grammar
// too and is still written here; it is left because a separator has no name to
// ask for, and moving the whole query-building over would put the page's own
// escaping decisions inside the parser.
func statusHref(status string) string {
	return "/search?" + url.Values{"q": {lexical.StatusFilterKey + ":" + status}}.Encode()
}

// plural picks the phrase that agrees with n. Chinese does not inflect a noun
// for number, so both sides of such a pair carry the same words there.
func plural(n int, one, many wording.Phrase, lang wording.Lang) string {
	if n == 1 {
		return fmt.Sprintf(one.In(lang), n)
	}
	return fmt.Sprintf(many.In(lang), n)
}

// countUnit is the plural phrase after its digits, so a visible tally can
// keep the number on screen and put the unit in the accessibility tree
// without an aria-label on a role-less span.
func countUnit(n int, one, many wording.Phrase, lang wording.Lang) string {
	_, unit, _ := strings.Cut(plural(n, one, many, lang), strconv.Itoa(n))
	return unit
}

// statusChipLabel names one square of the lifecycle block, saying in words that
// a note carries no status rather than leaving the square blank.
func statusChipLabel(status string, lang wording.Lang) string {
	return cmp.Or(status, wording.NoStatusStated.In(lang))
}

// facetRowLabel names what following the row does, since the row itself shows
// only a value and a number: a value already in the query leads out of it, and
// every other one leads further in. Which field is being narrowed is in the
// heading above, which a reader moving link by link never hears.
func facetRowLabel(heading string, row SearchFacetRow, lang wording.Lang) string {
	if row.Active {
		return fmt.Sprintf(wording.FacetRemoveFmt.In(lang), row.Label)
	}
	return fmt.Sprintf(wording.FacetNarrowFmt.In(lang), heading, row.Label)
}

// folderHref builds the browse URL for a folder. The vault root is not a folder
// under the tree — it is the mode's own listing — and the route for one level
// refuses an empty path, so the root answers at the mode index rather than at a
// trailing slash that returns nothing.
func folderHref(dir string) string {
	if dir == "" {
		return indexHref(folderMode)
	}
	return VaultHref("/folders/", dir)
}

// JournalMonthParam names the month a reader is reading the journal at. The
// links this page draws and the request a reader arrives with have to agree
// about that word, and a second spelling of it is how they stop agreeing.
const JournalMonthParam = "month"

// journalHref builds the journal's URL at one month. Every month is written
// out, the current one included, so a link a reader copies stays pointing at
// the month they were reading rather than at whichever month it is opened in.
func journalHref(m Month) string {
	return indexHref(journalMode) + "?" + url.Values{JournalMonthParam: {m.String()}}.Encode()
}

// searchHref builds the URL for one search query, escaping it as the form
// submission would, so an offered search and a typed one land on the same page.
func searchHref(q string) string {
	return "/search?" + url.Values{"q": {q}}.Encode()
}

// SearchPageHref builds the URL for one stretch of a search answer. It is
// exported because the face that puts the query to the index is the only one
// that can say how many hits there were, and so the only one that can build
// the strip — while how a search address is spelled stays here, beside the
// address every other search link is written with.
func SearchPageHref(q string, n PageNumber) string {
	return "/search?" + url.Values{"q": {q}, "page": {n.String()}}.Encode()
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
