package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
)

var (
	headingTag = regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
	tagStrip   = regexp.MustCompile(`<[^>]+>`)

	// offscreenNote matches the explanation this renderer attaches out of sight
	// to a link whose target is unwritten. It is dropped with its contents rather
	// than unwrapped, since those words are never part of what the section
	// holding the link is called. The element is renderer-owned and never nests
	// another, so the shortest close is its own.
	offscreenNote = regexp.MustCompile(`(?s)<span class="` + offscreenNoteClass + `">.*?</span>`)

	// nestedHeadingOpen detects a raw inline <h1-6> inside a heading's own inner
	// HTML — the one input that makes headingTag's non-greedy match stop at the
	// wrong close and truncate the heading. The `[>\s]` guard keeps it off
	// <hr>, <header> and <hgroup>.
	nestedHeadingOpen = regexp.MustCompile(`<h[1-6][>\s]`)
)

// headingInnerText reduces a heading's inner markup to the words the reader sees:
// a ruby reading and the offscreen explanation of an unwritten link are dropped
// with their contents before the remaining tags are, so the text keeps the base
// characters once and names a section by what is on screen. Character references
// then resolve. Both removals run before the tags, and so before any reference
// resolves, since authored markup arrives escaped and stays the text it is.
func headingInnerText(inner string) string {
	inner = offscreenNote.ReplaceAllString(inner, "")
	inner = rubyReading.ReplaceAllString(inner, "")
	return strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(inner, "")))
}

// The two halves of a place inside a document, as this package writes them:
// every attribute value is double-quoted and every quote an author wrote inside
// one arrived escaped. Deliberately not general HTML rules — they read bytes
// this package wrote.
var (
	anchorAttribute = regexp.MustCompile(` id="[^"]*"`)
	anchorAddress   = regexp.MustCompile(` href="#[^"]*"`)
)

// StripAnchors returns rendered HTML with every place inside it removed: the
// names, and the addresses that reach them. It is for an excerpt shown beside
// the page it was fetched for rather than as a page of its own. Each name it
// carried would be a second element answering to one the page's own headings
// and blocks already have, so a fragment naming it would reach whichever came
// first; and each address left behind afterwards would be a footnote number
// that looks live, does nothing, and drops a fragment in the address bar.
//
// The addresses are dropped rather than pointed at the note's own page: an
// excerpt's footnote ids are qualified for the region it was rendered in, so
// the name the address carries is one the note's page does not have either. The
// footnote marker stays visible and stops being a control.
func StripAnchors(htmlOut string) string {
	return anchorAddress.ReplaceAllString(anchorAttribute.ReplaceAllString(htmlOut, ""), "")
}

// assignHeadingIDs walks the final rendered HTML, gives each h1-h6 the id its
// name folds to, and collects the table of contents in document order. A name
// that folds to an id already taken bumps a numeric suffix until it lands on one
// assigned nowhere earlier. Every heading gets an id, a transcluded excerpt's
// included, while the contents list takes fewer; reserved is an id already
// spoken for elsewhere on the page. It reads the HTML this package just wrote
// rather than a parsed tree, because no single tree ever holds the page's
// headings together.
func assignHeadingIDs(htmlOut, reserved string) (string, []TOCEntry) {
	var toc []TOCEntry
	seen := map[string]bool{}
	if reserved != "" {
		seen[reserved] = true
	}
	transcluded := embedSpans(htmlOut)
	var out strings.Builder
	rest := 0
	for _, m := range headingTag.FindAllStringSubmatchIndex(htmlOut, -1) {
		out.WriteString(htmlOut[rest:m[0]])
		rest = m[1]
		level := int(htmlOut[m[2]] - '0') // headingTag guarantees a single 1-6 digit
		inner := htmlOut[m[4]:m[5]]

		// A raw inline <hN> here would make the non-greedy match stop at the
		// inner close, truncating the heading, so it is left byte-identical with
		// no id and no entry: degrade rather than corrupt.
		if nestedHeadingOpen.MatchString(inner) {
			out.WriteString(htmlOut[m[0]:m[1]])
			continue
		}
		// A course branch declares its part in the order at the end of the
		// heading that opens it. The declaration is grammar the course parser
		// consumes, as the '#' marks are, so the page calls the section what the
		// course calls it: the words shown, the contents entry and the id all
		// come from the heading with the role taken off. Level is the whole test
		// applied here, because this pass walks assembled markup, which no longer
		// says what contained a heading, while a course also asks that the
		// heading stand on its own. So a role written on a heading nested inside
		// a quote or a list item comes off the page's copy although the course
		// opens no branch there. The course does report that one, quoting the
		// line as the author wrote it, which leaves a report quoting words the
		// page has stopped showing.
		inner = sequence.HeadingName(inner, level)
		text := headingInnerText(inner)

		id := graph.SectionID(text)
		if seen[id] {
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s-%d", id, n)
				if !seen[cand] {
					id = cand
					break
				}
			}
		}
		seen[id] = true

		if !withinAny(transcluded, m[0], m[1]) {
			toc = append(toc, TOCEntry{Level: level, Text: text, ID: id})
		}
		fmt.Fprintf(&out, `<h%d id="%s">%s</h%d>`, level, id, inner, level)
	}
	out.WriteString(htmlOut[rest:])
	return out.String(), toc
}

// embedOpeners are the exact container openings a transcluded excerpt is
// wrapped in, spelled from the same function that writes them so the scan and
// the writer cannot drift apart.
var embedOpeners = []string{
	`<div class="` + embedClass(false) + `">`,
	`<div class="` + embedClass(true) + `">`,
}

// embedSpans reports the byte ranges of transcluded excerpts in fully assembled
// page HTML, each running from its container's opening tag to the close that
// balances it. Counting div tags is sound because authored markup never reaches
// this scan as markup, so every literal div is this renderer's own and balanced.
// Excerpts do not nest, so the spans are disjoint.
func embedSpans(htmlOut string) [][2]int {
	var spans [][2]int
	// Each opener keeps a cursor to its own next occurrence and only a cursor a
	// chosen span has passed is re-aimed, so the scan prices the page at its
	// length rather than at its length times its excerpt count.
	next := make([]int, len(embedOpeners))
	for i, opener := range embedOpeners {
		next[i] = nextMark(htmlOut, opener, 0)
	}
	for {
		start, width := -1, 0
		for i, at := range next {
			if at >= 0 && (start < 0 || at < start) {
				start, width = at, len(embedOpeners[i])
			}
		}
		if start < 0 {
			return spans
		}
		end := divEnd(htmlOut, start+width)
		spans = append(spans, [2]int{start, end})
		for i, at := range next {
			if at >= 0 && at < end {
				next[i] = nextMark(htmlOut, embedOpeners[i], end)
			}
		}
	}
}

// divEnd walks from just inside one open div to just past the close that
// balances it. A tail with no balancing close — nothing this renderer writes —
// ends the span at the end of the document rather than walking off it.
func divEnd(htmlOut string, from int) int {
	depth := 1
	for depth > 0 {
		nextOpen := strings.Index(htmlOut[from:], "<div")
		nextClose := strings.Index(htmlOut[from:], "</div>")
		if nextClose < 0 {
			return len(htmlOut)
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			depth++
			from += nextOpen + len("<div")
			continue
		}
		depth--
		from += nextClose + len("</div>")
	}
	return from
}
