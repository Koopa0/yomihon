package render

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// An opener has a blockquote prefix: four leading spaces or a tab show code.
var (
	calloutStartPattern = regexp.MustCompile(`^ {0,3}>\s*\[!([A-Za-z0-9_-]+)\]([+-]?)\s?(.*)$`)
	quotePrefix         = graph.QuotePrefix
)

// calloutStart reports whether line opens an Obsidian callout block, optionally
// with the "-"/"+" that makes it foldable, and if so its lowercased type, fold
// suffix, and any inline title text.
func calloutStart(line string) (typ, fold, title string, ok bool) {
	m := calloutStartPattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", false
	}
	return strings.ToLower(m[1]), m[2], strings.TrimSpace(m[3]), true
}

// calloutBucket is one of the visual/semantic groups every known callout
// type sorts into.
type calloutBucket int

const (
	bucketUnknown calloutBucket = iota
	bucketNote
	bucketWarning
	bucketQuote
)

// calloutGroup holds types that share a tint and icon. An untitled callout
// takes its own type's name, independently of that shared look.
type calloutGroup struct {
	bucket calloutBucket
	types  []string
}

// calloutVocabulary names the built-in callout types this renderer recognizes.
// A type outside it produces a diagnostic and remains a plain blockquote.
// Quotations carry their own icon because they hold someone else's words.
var calloutVocabulary = []calloutGroup{
	{bucketNote, []string{"info", "note", "tip", "important", "hint", "abstract", "tldr", "summary", "todo"}},
	{bucketNote, []string{"success", "check", "done"}},
	{bucketNote, []string{"question", "help", "faq"}},
	{bucketNote, []string{"example"}},
	{bucketQuote, []string{"quote", "cite"}},
	{bucketWarning, []string{"warning", "caution", "attention"}},
	{bucketWarning, []string{"danger", "error", "bug", "fail", "failure", "missing"}},
}

// calloutBucketOf maps a lowercased callout type to its bucket and default
// title. bucketUnknown means the type is unrecognized and the caller falls
// back to a plain blockquote.
func calloutBucketOf(typ string) (bucket calloutBucket, defaultTitle string) {
	for _, group := range calloutVocabulary {
		if slices.Contains(group.types, typ) {
			return group.bucket, strings.ToUpper(typ[:1]) + typ[1:]
		}
	}
	return bucketUnknown, ""
}

// String names a bucket for a message about a bucket nobody gave a look to. A
// value outside the constants is named by its number, which is all there is to
// say about one nothing declared.
func (b calloutBucket) String() string {
	switch b {
	case bucketUnknown:
		return "unknown"
	case bucketNote:
		return "note"
	case bucketWarning:
		return "warning"
	case bucketQuote:
		return "quote"
	default:
		return strconv.Itoa(int(b))
	}
}

// calloutIcon is the mark a bucket's title row opens with: a line drawing at
// the size and stroke of the interface's own icons, written inline so it needs
// no icon font and no asset of its own, and drawn in the title's colour. A text
// glyph took whatever shape and weight the reader's fonts gave it, which set an
// emoji beside one title and a hairline beside the next. A type this renderer
// does not recognize never reaches here — it is turned back into a plain
// blockquote before a look is chosen — so bucketUnknown shares the note's mark
// for the caller that stops recognizing that, and a bucket nobody wrote a look
// for stops rather than quietly borrowing one.
func calloutIcon(bucket calloutBucket) string {
	switch bucket {
	case bucketWarning:
		return calloutSVG(`<path d="M12 3.5 2.5 20h19z"></path><path d="M12 10v4"></path><path d="M12 17h.01"></path>`)
	case bucketQuote:
		return calloutSVG(`<path d="M6 17v-3.5C6 10 7.5 8 10 7"></path><path d="M6 13.5h3.5V17H6"></path><path d="M14 17v-3.5c0-3.5 1.5-5.5 4-6.5"></path><path d="M14 13.5h3.5V17H14"></path>`)
	case bucketNote, bucketUnknown:
		return calloutSVG(`<circle cx="12" cy="12" r="9"></circle><path d="M12 11v5"></path><path d="M12 8h.01"></path>`)
	default:
		panic("render: unknown calloutBucket: " + bucket.String())
	}
}

// calloutSVG wraps one mark's strokes in the frame every callout mark shares.
// It is hidden from assistive technology because the title beside it already
// says what kind of callout this is.
func calloutSVG(strokes string) string {
	return `<svg class="callout-icon" aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + strokes + `</svg>`
}

// calloutClass is the bucket's class-name suffix, paired with calloutIcon so
// a bucket's look is decided in one place beside its glyph, including what
// happens to a bucket neither of them was taught.
func calloutClass(bucket calloutBucket) string {
	switch bucket {
	case bucketWarning:
		return "warning"
	case bucketQuote:
		return "quote"
	case bucketNote, bucketUnknown:
		return "note"
	default:
		panic("render: unknown calloutBucket: " + bucket.String())
	}
}

// calloutShell spells one already-classified callout's markup as the two halves
// that enclose its body: a fold suffix becomes a native <details>, closed or
// open, and no suffix a static tinted div.
//
// The halves are returned apart rather than wrapped around finished HTML
// because the body is left in the note's own source between them. A callout
// rendered as its own document was its own footnote scope, so a reference and
// the definition it names could not see each other across the boundary: each
// side reached nothing and stayed on the page as the characters the author
// typed. One note is one document, so one note is one set of footnotes, one
// numbering, and one endnote list standing where the reader can reach it.
func calloutShell(bucket calloutBucket, defaultTitle, fold, title string) (open, closing string) {
	if title == "" {
		title = defaultTitle
	}
	bucketClass := calloutClass(bucket)
	header := calloutIcon(bucket) + html.EscapeString(title)

	if fold == "-" || fold == "+" {
		openAttr := ""
		if fold == "+" {
			openAttr = " open"
		}
		return fmt.Sprintf(
				`<details class="callout callout-%s"%s><summary class="callout-title">%s</summary>`+
					`<div class="callout-body">`, bucketClass, openAttr, header),
			`</div></details>`
	}
	return fmt.Sprintf(
			`<div class="callout callout-%s"><p class="callout-title">%s</p>`+
				`<div class="callout-body">`, bucketClass, header),
		`</div></div>`
}
