package render

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// An opener may stand at any indent, because a callout nested in a list item
// sits at that item's content column. Whether four spaces or a tab show the
// line as code depends on the container around it, which only a parse knows,
// so each reader asks its parser for the indented code lines before treating a
// match as a callout.
var (
	calloutStartPattern = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z0-9_-]+)\]([+-]?)\s?(.*)$`)
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

// IsCalloutOpening reports whether line has callout opening syntax, including
// unknown types rendered in a neutral shell. It does not decide whether the
// surrounding Markdown makes the line code or whether it can carry an address.
func IsCalloutOpening(line string) bool {
	_, _, _, ok := calloutStart(line)
	return ok
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

// calloutGroup holds types that share a label. An untitled callout takes its own
// type's name, independently of that shared word.
type calloutGroup struct {
	bucket calloutBucket
	types  []string
}

// calloutVocabulary names the built-in callout types this renderer recognizes.
// A type outside it produces a diagnostic and uses the neutral note shell.
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
// title. An unrecognized type keeps its classification and uses the neutral
// note shell with its own name as the default title.
func calloutBucketOf(typ string) (bucket calloutBucket, defaultTitle string) {
	for _, group := range calloutVocabulary {
		if slices.Contains(group.types, typ) {
			return group.bucket, strings.ToUpper(typ[:1]) + typ[1:]
		}
	}
	return bucketUnknown, strings.ToUpper(typ[:1]) + typ[1:]
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

// calloutClass is the bucket's class-name suffix, so a bucket's look is decided
// in one place, including what happens to a bucket nothing was taught.
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

// unlabelledCallouts are the types whose title is a question: the question is
// the title and what opens under it is its answer, so a word naming the type
// would say nothing the title has not.
var unlabelledCallouts = []string{"question", "help", "faq"}

// calloutLabel is the word a callout of this type opens its title row with, and
// false for the types that print none. A warning, danger or missing is a
// warning; a quote or cite is a quote; every other type, including one this
// renderer does not know, is a note, which is the neutral shell it is drawn in.
func calloutLabel(typ string, bucket calloutBucket) (word wording.Phrase, labelled bool) {
	switch {
	case slices.Contains(unlabelledCallouts, typ):
		return wording.Phrase{}, false
	case bucket == bucketWarning:
		return wording.CalloutWarning, true
	case bucket == bucketQuote:
		return wording.CalloutQuote, true
	case bucket == bucketNote, bucket == bucketUnknown:
		return wording.CalloutNote, true
	default:
		panic("render: unknown calloutBucket: " + bucket.String())
	}
}

// namesItsLabel reports whether a callout's own type is already the word its
// label says in English, so that printing the type as a default title beside
// the label would say the same thing twice.
func namesItsLabel(typ string) bool {
	return typ == "warning" || typ == "note" || typ == "quote"
}

// calloutOpening is the start of a callout's title row: its label, where it has
// one, in the language the interface speaks, because the article around it speaks
// the note's own.
func calloutOpening(typ string, bucket calloutBucket, lang wording.Lang) string {
	word, labelled := calloutLabel(typ, bucket)
	if !labelled {
		return ""
	}
	return `<span class="callout-type" lang="` + lang.Tag() + `">` + word.In(lang) + `</span>`
}

// calloutShell spells one already-classified callout's markup as the two halves
// that enclose its body: a fold suffix becomes a native <details>, closed or
// open, and no suffix a plain div. The type rides on the element so the page can
// tell a question's fold from any other. The caller escapes the literal title
// and any address span before passing their markup, and passes the label row's
// opening, which is already markup.
//
// The halves are returned apart rather than wrapped around finished HTML
// because the body is left in the note's own source between them. A callout
// rendered as its own document was its own footnote scope, so a reference and
// the definition it names could not see each other across the boundary: each
// side reached nothing and stayed on the page as the characters the author
// typed. One note is one document, so one note is one set of footnotes, one
// numbering, and one endnote list standing where the reader can reach it.
func calloutShell(bucket calloutBucket, typ, fold, openingHTML, titleHTML string) (open, closing string) {
	bucketClass := calloutClass(bucket)
	header := openingHTML + titleHTML

	if fold == "-" || fold == "+" {
		openAttr := ""
		if fold == "+" {
			openAttr = " open"
		}
		return fmt.Sprintf(
				`<details class="callout callout-%s" data-callout="%s"%s><summary class="callout-title">%s</summary>`+
					`<div class="callout-body">`, bucketClass, typ, openAttr, header),
			`</div></details>`
	}
	return fmt.Sprintf(
			`<div class="callout callout-%s" data-callout="%s"><p class="callout-title">%s</p>`+
				`<div class="callout-body">`, bucketClass, typ, header),
		`</div></div>`
}
