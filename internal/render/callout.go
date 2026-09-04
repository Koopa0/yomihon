package render

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	calloutStartPattern = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z]+)\]([+-]?)\s?(.*)$`)
	quotePrefix         = regexp.MustCompile(`^\s*>\s?`)
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

// calloutGroup is one share of the callout vocabulary: the types that render
// alike, the bucket they render as, and the English title they carry when
// their author writes none.
type calloutGroup struct {
	bucket calloutBucket
	title  string
	types  []string
}

// calloutVocabulary is every callout type this renderer answers to, written
// out as data so one value holds both which types exist and what each of them
// looks like. The set is closed, as Obsidian's is: a type outside it is not a
// callout, and the block falls back to a plain blockquote. A quotation is its
// own group, again as Obsidian reads it — it carries someone's words rather
// than a remark about the text.
var calloutVocabulary = []calloutGroup{
	{bucketNote, "Note", []string{"info", "note", "tip", "hint", "abstract", "summary", "todo"}},
	{bucketNote, "Question", []string{"question", "help", "faq"}},
	{bucketNote, "Example", []string{"example"}},
	{bucketQuote, "Quote", []string{"quote", "cite"}},
	{bucketWarning, "Warning", []string{"warning", "caution", "attention"}},
	{bucketWarning, "Danger", []string{"danger", "error", "bug", "fail", "failure", "missing"}},
}

// calloutBucketOf maps a lowercased callout type to its bucket and default
// title. bucketUnknown means the type is unrecognized and the caller falls
// back to a plain blockquote.
func calloutBucketOf(typ string) (bucket calloutBucket, defaultTitle string) {
	for _, group := range calloutVocabulary {
		if slices.Contains(group.types, typ) {
			return group.bucket, group.title
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

// calloutIcon is a small, dependency-free (no icon font, no SVG asset)
// glyph per bucket. A type this renderer does not recognize never reaches here
// — it is turned back into a plain blockquote before a look is chosen — so
// bucketUnknown shares the note glyph for the caller that stops recognizing
// that, and a bucket nobody wrote a look for stops rather than quietly
// borrowing one.
func calloutIcon(bucket calloutBucket) string {
	switch bucket {
	case bucketWarning:
		return "⚠"
	case bucketQuote:
		return "❝"
	case bucketNote, bucketUnknown:
		return "ℹ"
	default:
		panic("render: unknown calloutBucket: " + bucket.String())
	}
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
	header := fmt.Sprintf(`<span class="callout-icon" aria-hidden="true">%s</span>%s`,
		calloutIcon(bucket), html.EscapeString(title))

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
