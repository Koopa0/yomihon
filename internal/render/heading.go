package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

var (
	headingTag = regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N}]+`)
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

// foldFragment is the fold the two kinds of "#fragment" share: Unicode form and
// letter case, and nothing else. Those are the two ways a name differs for
// reasons its author never chose; every other difference they did choose.
func foldFragment(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// slugify makes a CJK-safe fragment id from heading text: NFC, lowercase, keep
// only Unicode letters and digits, collapse every other run to one hyphen, trim
// hyphens, and fall back to "section" when nothing is left. Keeping every Unicode
// letter is what lets a CJK heading produce a usable id. The NFC step comes first
// because a combining mark otherwise counts as "not a letter" and becomes a
// hyphen, so か+◌゙ん and がん would stamp two anchors for one heading.
func slugify(s string) string {
	s = slugDrop.ReplaceAllString(foldFragment(s), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "section"
	}
	return s
}

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

// anchorAttribute matches one id attribute in this package's own output, where
// every attribute value is double-quoted and every quote an author wrote inside
// one arrived escaped. It is deliberately not a general HTML rule: it reads
// bytes this package wrote.
var anchorAttribute = regexp.MustCompile(` id="[^"]*"`)

// StripAnchorIDs returns rendered HTML with every id removed and the markup
// around them untouched. It is for an excerpt shown beside the page it was
// fetched for rather than as a page of its own: nothing links into such an
// excerpt, and while it is on screen each id it carried would be a second
// element answering to a name the page's own headings and blocks already have,
// so a fragment naming one would reach whichever came first.
func StripAnchorIDs(htmlOut string) string {
	return anchorAttribute.ReplaceAllString(htmlOut, "")
}

// assignHeadingSlugs walks the final rendered HTML, assigns each h1-h6 a
// slugify'd id, and collects the table of contents in document order. A repeated
// base slug bumps a numeric suffix until it lands on an id assigned nowhere
// earlier. Every heading gets an id, a transcluded excerpt's included, while the
// contents list takes fewer; reserved is an id already spoken for elsewhere on the
// page. It reads the HTML this package just wrote rather than a parsed tree,
// because no single tree ever holds the page's headings together.
func assignHeadingSlugs(htmlOut, reserved string) (string, []TOCEntry) {
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
		text := headingInnerText(inner)

		id := slugify(text)
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
	`<div class="` + embedClass("") + `">`,
	`<div class="` + embedClass("#miss") + `">`,
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
