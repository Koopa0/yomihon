package render

import (
	"regexp"
	"slices"
	"strings"
)

// idReference matches an attribute whose value names other elements rather than
// describing this one. The list is every such attribute HTML and WAI-ARIA
// define, not the ones this tree happens to write today: an attribute left out
// would keep pointing at the other note's copy of a name, and nothing about the
// element carrying it says which kind it is. "form" is written before "for" so
// the longer name is tried first, and both are pinned anyway by the space in
// front and the "=\"" behind.
var idReference = regexp.MustCompile(
	` (form|for|list|headers|itemref|popovertarget|` +
		`aria-activedescendant|aria-controls|aria-describedby|aria-details|` +
		`aria-errormessage|aria-flowto|aria-labelledby|aria-owns)="([^"]*)"`)

// Qualify names every place inside one rendered note under prefix, and rewrites
// every address and reference that reaches one of them. It is the counterpart of
// StripAnchors: an excerpt shown beside a page drops its places, while a whole
// note shown beside another keeps them under a name of its own.
//
// A page holding two notes is the reason. Each note is rendered on its own and
// numbers its headings, footnotes and block addresses from its own text, so two
// of them on one page would hand the same name to two elements — and a reader
// following the right-hand note's contents entry, footnote marker, or an address
// its author wrote at one of its own sections would land in the left-hand note's
// copy. Under a prefix each note answers only to its own names, and every
// address inside it still reaches what its author meant.
//
// Three shapes are rewritten, which are the three ways one element reaches
// another inside a document: the name an element carries, the "#" address a link
// follows, and the attributes that describe one element by naming others. An
// empty address is left alone — it scrolls to the top of the document and names
// nothing, so a prefix would only make it dead. An empty prefix returns the
// result unchanged, which is the page showing one note.
//
// Attribute values are read as the bytes this package writes: every value
// double-quoted, every quote inside one already escaped. That is what lets one
// pass cover a heading, a footnote, a block address, a link written at a section
// of the note's own text, and a control spliced into the body afterwards,
// without any of them keeping a rule of its own.
//
// It renames the result it is given rather than handing back another: what a
// render came to is one value a page holds once, and copying it to rename three
// of its fields would leave the caller deciding which of the two is the page's.
func Qualify(prefix string, res *Result) {
	if prefix == "" || res == nil {
		return
	}
	res.HTML = qualifyPlaces(prefix, res.HTML)
	if res.TitleAnchor != "" {
		res.TitleAnchor = prefix + res.TitleAnchor
	}
	// The contents list is built from the same headings the body carries, so it
	// is renamed with them. The entries are copied first: the slice arrives from
	// the pass that stamped the headings, and renaming through it would reach
	// anything else already holding the same entries.
	if len(res.TOC) > 0 {
		toc := slices.Clone(res.TOC)
		for i := range toc {
			toc[i].ID = prefix + toc[i].ID
		}
		res.TOC = toc
	}
}

// qualifyPlaces renames the places one body's HTML carries and the references
// that reach them.
func qualifyPlaces(prefix, htmlOut string) string {
	under := func(name string) string { return prefix + name }
	htmlOut = renameGroup(anchorAttribute, 1, htmlOut, under)
	htmlOut = renameGroup(anchorAddress, 1, htmlOut, func(address string) string {
		if address == "" {
			return ""
		}
		return under(address)
	})
	return renameGroup(idReference, 2, htmlOut, func(names string) string {
		return qualifyNames(prefix, names)
	})
}

// qualifyNames renames each element an attribute points at. Several of these
// attributes take a whitespace-separated list, and renaming the value whole
// would turn a list of names into one name nothing has. The separators the
// author of the markup chose are kept, since this rewrites what the values are
// and not how they were spelled.
func qualifyNames(prefix, names string) string {
	fields := strings.Fields(names)
	if len(fields) == 0 {
		return names
	}
	for i, name := range fields {
		fields[i] = prefix + name
	}
	return strings.Join(fields, " ")
}

// renameGroup returns htmlOut with the given capture group of every match
// replaced by what rename makes of it. Everything outside that group — the
// attribute name, the quotes around the value, and the bytes between matches —
// is copied exactly, so a pass renames what an attribute holds without
// rewriting the attribute. It walks the assembled markup rather than a parsed
// tree for the same reason the heading pass does: no single tree holds
// everything one page assembles.
func renameGroup(re *regexp.Regexp, group int, htmlOut string, rename func(string) string) string {
	matches := re.FindAllStringSubmatchIndex(htmlOut, -1)
	if len(matches) == 0 {
		return htmlOut
	}
	var out strings.Builder
	out.Grow(len(htmlOut))
	rest := 0
	for _, m := range matches {
		start, end := m[2*group], m[2*group+1]
		out.WriteString(htmlOut[rest:start])
		out.WriteString(rename(htmlOut[start:end]))
		rest = end
	}
	out.WriteString(htmlOut[rest:])
	return out.String()
}
