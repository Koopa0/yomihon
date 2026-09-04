package render

import (
	"fmt"
	"html"
	"regexp"
	"slices"
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

// calloutIcon is a small, dependency-free (no icon font, no SVG asset)
// glyph per bucket.
func calloutIcon(bucket calloutBucket) string {
	switch bucket {
	case bucketWarning:
		return "⚠"
	case bucketQuote:
		return "❝"
	default:
		return "ℹ"
	}
}

// calloutClass is the bucket's class-name suffix, paired with calloutIcon so
// a bucket's look is decided in one place beside its glyph.
func calloutClass(bucket calloutBucket) string {
	switch bucket {
	case bucketWarning:
		return "warning"
	case bucketQuote:
		return "quote"
	default:
		return "note"
	}
}

// renderCallout renders one already-classified callout: a fold suffix becomes a
// native <details>, closed or open, and no suffix a static tinted div. The body
// goes through the same pipeline as the top level, so nesting works inside it.
func (r *Pipeline) renderCallout(bucket calloutBucket, defaultTitle, fold, title, body string, allowEmbed embedPolicy, col *collector) string {
	if title == "" {
		title = defaultTitle
	}
	inner := r.render(body, allowEmbed, col.page)
	col.diags = append(col.diags, inner.Diagnostics...)

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
				`<div class="callout-body">%s</div></details>`,
			bucketClass, openAttr, header, inner.HTML)
	}
	return fmt.Sprintf(
		`<div class="callout callout-%s"><p class="callout-title">%s</p>`+
			`<div class="callout-body">%s</div></div>`,
		bucketClass, header, inner.HTML)
}
