package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	calloutStartPattern = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z]+)\]([+-]?)\s?(.*)$`)
	quotePrefix         = regexp.MustCompile(`^\s*>\s?`)
)

// calloutStart reports whether line opens an Obsidian callout block
// ("> [!type]", optionally followed by "-"/"+" for a foldable
// <details>) and, if so, its type (lowercased), fold suffix, and any
// inline title text.
func calloutStart(line string) (typ, fold, title string, ok bool) {
	m := calloutStartPattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", false
	}
	return strings.ToLower(m[1]), m[2], strings.TrimSpace(m[3]), true
}

// calloutBucket is one of the two visual/semantic groups every known
// callout type sorts into.
type calloutBucket int

const (
	bucketUnknown calloutBucket = iota
	bucketNote
	bucketWarning
)

// calloutBucketOf maps a callout type (already lowercased) to its bucket
// and default English title.
// bucketUnknown means the type is not recognized — the caller falls back
// to a plain blockquote and records a Diagnostic.
func calloutBucketOf(typ string) (bucket calloutBucket, defaultTitle string) {
	switch typ {
	case "info", "note", "tip", "hint", "abstract", "summary", "todo":
		return bucketNote, "Note"
	case "question", "help", "faq":
		return bucketNote, "Question"
	case "example", "quote", "cite":
		return bucketNote, "Example"
	case "warning", "caution", "attention":
		return bucketWarning, "Warning"
	case "danger", "error", "bug", "fail", "failure", "missing":
		return bucketWarning, "Danger"
	default:
		return bucketUnknown, ""
	}
}

// calloutIcon is a small, dependency-free (no icon font, no SVG asset)
// glyph per bucket.
func calloutIcon(bucket calloutBucket) string {
	if bucket == bucketWarning {
		return "⚠"
	}
	return "ℹ"
}

// renderCallout renders one already-classified (known-bucket) callout:
// fold == "-"/"+" becomes a native <details> (closed/open); no suffix
// becomes a static, tinted div. The body is markdown — it is rendered
// through render (the same pipeline used at the top level), so nested
// formatting and nested wikilinks work inside a callout.
func (r *Renderer) renderCallout(bucket calloutBucket, defaultTitle, fold, title, body string, allowEmbed embedPolicy, diags *[]Diagnostic) string {
	if title == "" {
		title = defaultTitle
	}
	inner := r.render(body, allowEmbed)
	*diags = append(*diags, inner.Diagnostics...)

	bucketClass := "note"
	if bucket == bucketWarning {
		bucketClass = "warning"
	}
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
